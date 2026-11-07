package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryFrameRoundTrip(t *testing.T) {
	f := historyFrame{
		ts: 1700000000123, cpuAvg: 4321, cpuPeak: 65535,
		memMB: 8192, commitMB: 12288, memTotalMB: 32768, tcp: 65535, udp: 100,
	}
	got := decodeFrame(encodeFrame(f))
	if got != f {
		t.Fatalf("round trip mismatch: %+v vs %+v", got, f)
	}
}

func TestGbToMBAndPercentSaturate(t *testing.T) {
	if got := gbToMB(8.5); got != 8704 {
		t.Fatalf("gbToMB(8.5) = %d, want 8704", got)
	}
	if got := gbToMB(-1); got != 0 {
		t.Fatalf("gbToMB(-1) = %d, want 0", got)
	}
	if got := gbToMB(1 << 23); got != 4294967295 { // 8TB 饱和
		t.Fatalf("gbToMB saturate = %d", got)
	}
	if got := percentToU16(12.34); got != 1234 {
		t.Fatalf("percentToU16(12.34) = %d, want 1234", got)
	}
	if got := percentToU16(1e6); got != 65535 {
		t.Fatalf("percentToU16 overflow = %d, want 65535", got)
	}
	if got := percentToU16(-5); got != 0 {
		t.Fatalf("percentToU16(-5) = %d, want 0", got)
	}
}

// TestHistoryRecorderAggregationAndTruncation 用临时目录走完整写读链路：
// 三个样本跨两个 5s 窗口 → 两帧落盘；再手工追加半帧验证打开时截断。
func TestHistoryRecorderAggregationAndTruncation(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Truncate(historyFrameInterval)

	if err := historyRec.start(dir); err != nil {
		t.Fatal(err)
	}
	// 同一窗口两个样本：CPU 均值 (10+30)/2=20，峰值 30。
	historyAdd(historySample{ts: base, cpu: 10, memTotalGB: 32, memGB: 8, commitGB: 12, tcp: 100, udp: 50})
	historyAdd(historySample{ts: base.Add(time.Second), cpu: 30, memTotalGB: 32, memGB: 9, commitGB: 13, tcp: 110, udp: 51})
	// 下一窗口一个样本。
	historyAdd(historySample{ts: base.Add(5 * time.Second), cpu: 50, memTotalGB: 32, memGB: 10, commitGB: 14, tcp: 120, udp: 52})
	// 无效样本（非 Windows 占位）必须被丢弃。
	historyAdd(historySample{ts: base.Add(10 * time.Second), cpu: 99})

	// 停止触发残窗冲刷。
	historyRec.stop()
	historyRec.mu.Lock()
	started := historyRec.started
	day := historyRec.day
	historyRec.mu.Unlock()
	if started {
		t.Fatal("stop 后 recorder 应处于未启动状态")
	}

	data, err := os.ReadFile(filepath.Join(dir, day+".bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:len(historyMagic)]) != historyMagic {
		t.Fatalf("magic mismatch: %q", string(data[:len(historyMagic)]))
	}
	body := data[len(historyMagic):]
	if len(body) != 2*historyFrameSize {
		t.Fatalf("帧数错误：body=%d bytes, want %d", len(body), 2*historyFrameSize)
	}

	f0 := decodeFrame(body[:historyFrameSize])
	if f0.cpuAvg != 2000 || f0.cpuPeak != 3000 {
		t.Fatalf("第一帧 CPU 均值/峰值错误: %d/%d", f0.cpuAvg, f0.cpuPeak)
	}
	if f0.memMB != 9*1024 || f0.tcp != 110 { // 窗口取末值
		t.Fatalf("第一帧末值错误: memMB=%d tcp=%d", f0.memMB, f0.tcp)
	}
	f1 := decodeFrame(body[historyFrameSize : 2*historyFrameSize])
	if f1.cpuAvg != 5000 || f1.ts != base.Add(5*time.Second).UnixMilli() {
		t.Fatalf("第二帧错误: %+v", f1)
	}

	// 追加半帧后重开：残尾应被截断，帧数不变。
	// （recorder 生命周期一次 start/stop，这里直接复用 openDayLocked 验证打开时截断逻辑。）
	fh, err := os.OpenFile(filepath.Join(dir, day+".bin"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fh.Write(make([]byte, 10))
	fh.Close()

	historyRec.mu.Lock()
	err = historyRec.openDayLocked(day)
	historyRec.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	historyRec.mu.Lock()
	file := historyRec.file
	historyRec.file = nil // 手动接管，避免后续 stop 二次关闭
	historyRec.mu.Unlock()
	if file == nil {
		t.Fatal("openDayLocked 未打开文件")
	}
	info, _ := file.Stat()
	if rem := (info.Size() - int64(len(historyMagic))) % historyFrameSize; rem != 0 {
		t.Fatalf("残帧未截断: size=%d rem=%d", info.Size(), rem)
	}
	file.Close()
}

func TestBucketHistoryFrames(t *testing.T) {
	now := time.Now().UnixMilli()
	frames := []historyFrame{
		{ts: now - 1000, cpuAvg: 2000, cpuPeak: 2500, memMB: 100, memTotalMB: 1000},
		{ts: now - 900, cpuAvg: 4000, cpuPeak: 4500, memMB: 200, memTotalMB: 1000},
	}
	points := bucketHistoryFrames(frames, 1)
	if len(points) != historyBucketCount {
		t.Fatalf("桶数 = %d, want %d", len(points), historyBucketCount)
	}
	// 最后一个桶应聚合两帧：均值 30%，峰值 45%。
	last := points[len(points)-1]
	if !last.HasData || last.Cpu != 30 || last.CpuPeak != 45 {
		t.Fatalf("末桶错误: %+v", last)
	}
	if last.MemMB != 200 { // 末值
		t.Fatalf("末桶 memMB = %d, want 200", last.MemMB)
	}
	// 首桶无数据 → 断档标记。
	if points[0].HasData {
		t.Fatal("首桶不应有数据")
	}
}
