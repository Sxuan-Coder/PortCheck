package main

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// 资源历史：monitor 采样聚合成 5 秒一帧，按 UTC 日落盘为紧凑二进制文件，
// 查询按时间范围分桶汇总。所有磁盘 I/O 都在独立 writer goroutine 上，
// 采样路径只向带缓冲 channel 投递，磁盘慢/报错不拖累 monitor:tick 推送。
const (
	historyMagic         = "PCHK-H1\n"
	historyFrameSize     = 32
	historyFrameInterval = 5 * time.Second
	historyKeepDays      = 7
	historyChannelCap    = 64
	historyBucketCount   = 300
)

// historySample 是 monitor 单次采样的历史输入。
type historySample struct {
	ts         time.Time
	cpu        float64 // 整机 CPU 百分比
	memGB      float64
	commitGB   float64
	memTotalGB float64
	tcp        uint32
	udp        uint32
}

// historyFrame 是落盘的 5 秒聚合帧（小端，32 字节）。
type historyFrame struct {
	ts         int64 // Unix 毫秒（帧起始，按 5s 对齐）
	cpuAvg     uint16 // 百分比 ×100
	cpuPeak    uint16 // 百分比 ×100
	memMB      uint32
	commitMB   uint32
	memTotalMB uint32
	tcp        uint32
	udp        uint32
}

// encodeFrame 按 ts,cpuAvg,cpuPeak,memMB,commitMB,memTotalMB,tcp,udp 顺序小端编码。
func encodeFrame(f historyFrame) []byte {
	buf := make([]byte, historyFrameSize)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(f.ts))
	binary.LittleEndian.PutUint16(buf[8:10], f.cpuAvg)
	binary.LittleEndian.PutUint16(buf[10:12], f.cpuPeak)
	binary.LittleEndian.PutUint32(buf[12:16], f.memMB)
	binary.LittleEndian.PutUint32(buf[16:20], f.commitMB)
	binary.LittleEndian.PutUint32(buf[20:24], f.memTotalMB)
	binary.LittleEndian.PutUint32(buf[24:28], f.tcp)
	binary.LittleEndian.PutUint32(buf[28:32], f.udp)
	return buf
}

// decodeFrame 是 encodeFrame 的逆操作。
func decodeFrame(buf []byte) historyFrame {
	return historyFrame{
		ts:         int64(binary.LittleEndian.Uint64(buf[0:8])),
		cpuAvg:     binary.LittleEndian.Uint16(buf[8:10]),
		cpuPeak:    binary.LittleEndian.Uint16(buf[10:12]),
		memMB:      binary.LittleEndian.Uint32(buf[12:16]),
		commitMB:   binary.LittleEndian.Uint32(buf[16:20]),
		memTotalMB: binary.LittleEndian.Uint32(buf[20:24]),
		tcp:        binary.LittleEndian.Uint32(buf[24:28]),
		udp:        binary.LittleEndian.Uint32(buf[28:32]),
	}
}

// gbToMB 换算并截断到 uint32 范围（约 4TB 上限，饱和不回绕）。
func gbToMB(gb float64) uint32 {
	const maxUint32 = float64(^uint32(0))
	mb := gb * 1024
	if mb >= maxUint32 {
		return uint32(maxUint32)
	}
	if mb <= 0 {
		return 0
	}
	return uint32(mb)
}

// percentToU16 把百分比编码为 ×100 的 uint16，超界饱和。
func percentToU16(p float64) uint16 {
	if p <= 0 {
		return 0
	}
	if p >= 655.35 {
		return 65535
	}
	return uint16(p * 100)
}

// historyRecorder 聚合采样为帧并由独立 goroutine 落盘。
type historyRecorder struct {
	mu      sync.Mutex
	started bool
	ch      chan historySample
	done    chan struct{} // writeLoop 退出信号，stop 时等待排空

	dir       string
	day       string // 当前写入的 UTC 日（YYYY-MM-DD）
	file      *os.File
	frameTs   time.Time // 当前聚合窗的 5s 对齐起点
	winCpuSum float64
	winCpuMax float64
	winCount  int
	last      historySample // 窗口内最后一个样本（内存/端口计数取末值）
}

// historyRec 是包级单例：monitor emitTick 投递样本，HistoryService 管理生命周期。
var historyRec = &historyRecorder{ch: make(chan historySample, historyChannelCap)}

// historyAdd 投递一个采样样本；未启动或缓冲满时丢弃，绝不阻塞采样路径。
// 无有效数据（非 Windows 占位采样）时跳过，避免写入全零帧。
func historyAdd(s historySample) {
	if !s.valid() {
		return
	}
	historyRec.mu.Lock()
	started := historyRec.started
	historyRec.mu.Unlock()
	if !started {
		return
	}
	select {
	case historyRec.ch <- s:
	default:
		// 缓冲满说明磁盘慢于采样，丢样本保采样路径，历史曲线上表现为稀疏。
	}
}

// valid 报告样本是否包含真实采集数据。
func (s historySample) valid() bool { return s.memTotalGB > 0 }

// start 初始化目录并启动 writer goroutine。
func (r *historyRecorder) start(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	r.mu.Lock()
	r.dir = dir
	r.started = true
	r.done = make(chan struct{})
	r.mu.Unlock()
	go r.writeLoop()
	return nil
}

// stop 停止 writer，冲刷残窗并等待 goroutine 退出。
func (r *historyRecorder) stop() {
	r.mu.Lock()
	started := r.started
	r.started = false
	done := r.done
	r.mu.Unlock()
	if !started {
		return
	}
	close(r.ch)
	<-done
}

// writeLoop 消费样本、聚合 5s 帧、按日追加落盘；所有文件 I/O 都在这里。
func (r *historyRecorder) writeLoop() {
	defer close(r.done)
	for s := range r.ch {
		r.ingest(s)
	}
	// 停止时冲刷未满 5s 的残窗，避免丢掉最后一段数据。
	r.mu.Lock()
	r.flushLocked()
	if r.file != nil {
		_ = r.file.Close()
		r.file = nil
	}
	r.mu.Unlock()
}

// ingest 把样本聚入当前 5s 窗口，窗口翻转时落盘上一帧。
func (r *historyRecorder) ingest(s historySample) {
	frameTs := s.ts.Truncate(historyFrameInterval)
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.frameTs.IsZero() && !frameTs.Equal(r.frameTs) {
		r.flushLocked()
		r.winCpuSum, r.winCpuMax, r.winCount = 0, 0, 0
	}
	if r.frameTs.IsZero() || !frameTs.Equal(r.frameTs) {
		r.frameTs = frameTs
	}
	r.winCpuSum += s.cpu
	if s.cpu > r.winCpuMax {
		r.winCpuMax = s.cpu
	}
	r.winCount++
	r.last = s
}

// flushLocked 把当前窗口写成帧并追加到对应日文件；窗口状态复位由 ingest 负责。
func (r *historyRecorder) flushLocked() {
	if r.winCount == 0 {
		return
	}
	day := r.frameTs.UTC().Format("2006-01-02")
	if day != r.day || r.file == nil {
		if err := r.openDayLocked(day); err != nil {
			return // 打不开就放弃本帧，绝不阻塞
		}
	}
	avg := r.winCpuSum / float64(r.winCount)
	frame := historyFrame{
		ts:         r.frameTs.UnixMilli(),
		cpuAvg:     percentToU16(avg),
		cpuPeak:    percentToU16(r.winCpuMax),
		memMB:      gbToMB(r.last.memGB),
		commitMB:   gbToMB(r.last.commitGB),
		memTotalMB: gbToMB(r.last.memTotalGB),
		tcp:        r.last.tcp,
		udp:        r.last.udp,
	}
	_, _ = r.file.Write(encodeFrame(frame))
}

// openDayLocked 打开（或创建）某 UTC 日的帧文件；打开时把不足一帧的残尾截掉。
func (r *historyRecorder) openDayLocked(day string) error {
	if r.file != nil {
		_ = r.file.Close()
		r.file = nil
	}
	path := filepath.Join(r.dir, day+".bin")
	// 不用 O_APPEND：需要 WriteAt 修写 magic 头，追加位置由末尾 Seek 决定。
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	info, serr := f.Stat()
	if serr != nil {
		f.Close()
		return serr
	}
	size := info.Size()
	if size < int64(len(historyMagic)) {
		// 空文件/损坏文件：重写 magic 头。
		if terr := f.Truncate(0); terr != nil {
			f.Close()
			return terr
		}
		if _, werr := f.WriteAt([]byte(historyMagic), 0); werr != nil {
			f.Close()
			return werr
		}
	} else if rem := (size - int64(len(historyMagic))) % historyFrameSize; rem != 0 {
		_ = f.Truncate(size - rem) // 崩溃残帧截断
	}
	if _, serr = f.Seek(0, io.SeekEnd); serr != nil {
		f.Close()
		return serr
	}
	r.file = f
	r.day = day
	return nil
}

// pruneLocked 删除超过保留期的历史文件（按文件名日期判断，尽力而为）。
func (r *historyRecorder) pruneLocked(now time.Time) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return
	}
	cutoff := now.UTC().AddDate(0, 0, -historyKeepDays).Format("2006-01-02")
	for _, e := range entries {
		name := e.Name()
		if len(name) != 14 || name[10:] != ".bin" || name > cutoff || name[:4] < "2000" {
			continue
		}
		_ = os.Remove(filepath.Join(r.dir, name))
	}
}

// HistoryService 提供历史记录生命周期管理与分桶查询 RPC。
type HistoryService struct{}

// ServiceStartup 启动历史落盘并清理过期文件。
func (s *HistoryService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	dir, err := historyDir()
	if err != nil {
		return nil // 拿不到配置目录就静默放弃，不影响应用
	}
	historyRec.mu.Lock()
	historyRec.pruneLocked(time.Now())
	historyRec.mu.Unlock()
	return historyRec.start(dir)
}

// ServiceShutdown 停止落盘。
func (s *HistoryService) ServiceShutdown() error {
	historyRec.stop()
	return nil
}

// historyDir 返回 %APPDATA%/PortCheck/history（跨平台取用户配置目录）。
func historyDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "PortCheck", "history"), nil
}

// HistoryPoint 是分桶后的一个数据点；HasData=false 表示该桶无采样（断档）。
type HistoryPoint struct {
	Ts         int64   `json:"ts"` // Unix 毫秒
	Cpu        float64 `json:"cpu"`        // 均值百分比 0-100
	CpuPeak    float64 `json:"cpuPeak"`
	MemMB      uint32  `json:"memMB"`
	CommitMB   uint32  `json:"commitMB"`
	MemTotalMB uint32  `json:"memTotalMB"`
	Tcp        uint32  `json:"tcp"`
	Udp        uint32  `json:"udp"`
	HasData    bool    `json:"hasData"`
}

// HistorySeries 是一次历史查询结果。
type HistorySeries struct {
	RangeHours int           `json:"rangeHours"`
	Points     []HistoryPoint `json:"points"`
}

// historyHoursWhitelist 是允许查询的时间范围。
var historyHoursWhitelist = map[int]bool{1: true, 6: true, 24: true, 168: true}

// QuerySeries 按小时数（1/6/24/168）查询分桶后的历史曲线。
func (s *HistoryService) QuerySeries(hours int) (HistorySeries, error) {
	if !historyHoursWhitelist[hours] {
		hours = 1
	}
	frames, err := readHistoryFrames(time.Now().Add(-time.Duration(hours)*time.Hour), time.Now())
	if err != nil {
		return HistorySeries{}, err
	}
	return HistorySeries{RangeHours: hours, Points: bucketHistoryFrames(frames, hours)}, nil
}

// readHistoryFrames 读取覆盖 [from, to] 的各日文件并返回范围内的帧。
func readHistoryFrames(from, to time.Time) ([]historyFrame, error) {
	dir, err := historyDir()
	if err != nil {
		return nil, err
	}
	var frames []historyFrame
	startDay := from.UTC().Truncate(24 * time.Hour)
	for day := startDay; !day.After(to); day = day.Add(24 * time.Hour) {
		name := day.UTC().Format("2006-01-02") + ".bin"
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue // 缺失/未写入的日子按断档处理
		}
		if len(data) < len(historyMagic) || string(data[:len(historyMagic)]) != historyMagic {
			continue
		}
		body := data[len(historyMagic):]
		for off := 0; off+historyFrameSize <= len(body); off += historyFrameSize {
			f := decodeFrame(body[off : off+historyFrameSize])
			ts := time.UnixMilli(f.ts)
			if ts.Before(from) || ts.After(to) {
				continue
			}
			frames = append(frames, f)
		}
	}
	sort.Slice(frames, func(i, j int) bool { return frames[i].ts < frames[j].ts })
	return frames, nil
}

// bucketHistoryFrames 把帧按最多 historyBucketCount 个桶做均值/末值聚合，
// 空桶保留时间戳并标记 HasData=false 供前端断档显示。
func bucketHistoryFrames(frames []historyFrame, hours int) []HistoryPoint {
	rangeMs := int64(hours) * 3600_000
	end := time.Now().UnixMilli()
	start := end - rangeMs
	bucketMs := rangeMs / historyBucketCount
	if bucketMs < 1 {
		bucketMs = 1
	}

	points := make([]HistoryPoint, 0, historyBucketCount)
	idx := 0
	for i := 0; i < historyBucketCount; i++ {
		lo := start + int64(i)*bucketMs
		hi := lo + bucketMs
		if i == historyBucketCount-1 {
			hi = end // 末桶收口到当前时刻
		}
		pt := HistoryPoint{Ts: lo}
		count := 0
		var cpuSum float64
		var cpuPeak uint16
		for idx < len(frames) && int64(frames[idx].ts) < hi {
			f := frames[idx]
			if int64(f.ts) >= lo {
				cpuSum += float64(f.cpuAvg)
				if f.cpuPeak > cpuPeak {
					cpuPeak = f.cpuPeak
				}
				pt.MemMB, pt.CommitMB, pt.MemTotalMB = f.memMB, f.commitMB, f.memTotalMB
				pt.Tcp, pt.Udp = f.tcp, f.udp
				count++
			}
			idx++
		}
		if count > 0 {
			pt.HasData = true
			pt.Cpu = cpuSum / float64(count) / 100
			pt.CpuPeak = float64(cpuPeak) / 100
		}
		points = append(points, pt)
	}
	return points
}
