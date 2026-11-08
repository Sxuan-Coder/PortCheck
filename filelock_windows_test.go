//go:build windows

package main

import (
	"errors"
	"path/filepath"
	"strings"
	"os"
	"testing"
	"unsafe"
)

// RM_PROCESS_INFO 的布局断言：RmGetList 按 668 字节步进写入，
// 数组长度必须带 CCH+1（256/64），错一个字节就会越界写坏堆内存。
func TestRmProcessInfoLayout(t *testing.T) {
	if got := unsafe.Sizeof(rmProcessInfo{}); got != 668 {
		t.Fatalf("sizeof(rmProcessInfo) = %d, want 668", got)
	}
	if got := unsafe.Sizeof(rmUniqueProcess{}); got != 12 {
		t.Fatalf("sizeof(rmUniqueProcess) = %d, want 12", got)
	}
	var info rmProcessInfo
	appTypeOffset := unsafe.Offsetof(info.applicationType)
	if appTypeOffset != 652 {
		t.Fatalf("offsetof(applicationType) = %d, want 652", appTypeOffset)
	}
	if got := unsafe.Offsetof(info.tsSessionID); got != 660 {
		t.Fatalf("offsetof(tsSessionID) = %d, want 660", got)
	}
}

func TestRmAppTypeNames(t *testing.T) {
	if got := rmAppTypeNames(3); got != "Windows 服务" {
		t.Fatalf("rmAppTypeNames(3) = %q", got)
	}
	if got := rmAppTypeNames(42); got != "未知类型" {
		t.Fatalf("rmAppTypeNames(42) = %q", got)
	}
}

// TestFindLockersSelfHeldFile 用测试进程自己持有句柄的文件做冒烟：
// Restart Manager 应能完成查询且不报错；是否上报自身占用随系统而异，空结果跳过。
func TestFindLockersSelfHeldFile(t *testing.T) {
	f, err := os.CreateTemp("", "portcheck-lock-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	res, err := (&FileLockService{}).FindLockers(f.Name())
	if err != nil {
		t.Fatalf("FindLockers: %v", err)
	}
	if res.Scanned != 1 {
		t.Fatalf("scanned = %d, want 1", res.Scanned)
	}
	if len(res.Entries) == 0 {
		t.Skip("当前系统未上报本进程的文件占用，跳过断言")
	}
	found := false
	for _, e := range res.Entries {
		if e.PID == uint32(os.Getpid()) {
			found = true
		}
	}
	if !found {
		t.Fatalf("未在结果中找到自身 PID，entries=%+v", res.Entries)
	}
}

// TestWalkDirectorySkipsArtifacts 目录展开必须跳过构建产物/依赖目录，
// 且上限截断后文件数不超过 limit。
func TestWalkDirectorySkipsArtifacts(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel string, isDir bool) {
		p := filepath.Join(dir, rel)
		if isDir {
			if err := os.MkdirAll(p, 0755); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mk(`main.dart`, false)
	mk(`lib\app.dart`, false)
	mk(`build\app.so`, false)          // 产物目录：跳过
	mk(`.dart_tool\cache.bin`, false)  // 依赖缓存：跳过
	mk(`node_modules\pkg\index.js`, false) // 依赖：跳过
	mk(`BUILD NOTE.txt`, false)        // 名字含 build 的文件不跳过

	files, err := walkDirectoryFiles(dir, rmMaxFilesPerQuery)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("应只剩 3 个源码文件，得到 %d: %v", len(files), files)
	}
	for _, f := range files {
		if strings.Contains(strings.ToLower(f), `\build\`) || strings.Contains(f, ".dart_tool") || strings.Contains(f, "node_modules") {
			t.Fatalf("产物目录未被跳过: %s", f)
		}
	}
}

// TestSplitBudgetExhausted 预算耗尽后剩余文件整批隔离，不继续开子会话。
func TestSplitBudgetExhausted(t *testing.T) {
	calls := 0
	query := func(files []string) ([]FileLockEntry, error) {
		calls++
		return nil, errors.New("boom")
	}
	files := []string{"a", "b", "c", "d"}
	entries, bad := findLockersWithSplit(files, 3, query)
	if len(entries) != 0 {
		t.Fatalf("不应有占用条目")
	}
	if len(bad) != 4 {
		t.Fatalf("预算耗尽后 4 个文件应全部隔离，got %d", len(bad))
	}
	if calls != 3 {
		t.Fatalf("RM 会话调用应受预算限制为 3 次，got %d", calls)
	}
}

// TestSplitBadFileIsolated 单个坏文件被隔离，好文件的占用结果不受影响。
func TestSplitBadFileIsolated(t *testing.T) {
	badFile := "locked-by-broken"
	query := func(files []string) ([]FileLockEntry, error) {
		for _, f := range files {
			if f == badFile {
				return nil, errors.New("access failed") // 批内含坏文件即整批失败
			}
		}
		return []FileLockEntry{{PID: 1234, Process: "app.exe"}}, nil
	}
	entries, bad := findLockersWithSplit([]string{"good1.exe", badFile, "good2.exe"}, 64, query)
	if len(entries) != 2 {
		t.Fatalf("应返回 2 个占用条目，got %d", len(entries))
	}
	if len(bad) != 1 || bad[0] != badFile {
		t.Fatalf("应只隔离 %s，got %v", badFile, bad)
	}
}
