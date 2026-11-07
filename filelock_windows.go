//go:build windows

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	rstrtmgr               = windows.NewLazySystemDLL("rstrtmgr.dll")
	procRmStartSession     = rstrtmgr.NewProc("RmStartSession")
	procRmEndSession       = rstrtmgr.NewProc("RmEndSession")
	procRmRegisterResources = rstrtmgr.NewProc("RmRegisterResources")
	procRmGetList          = rstrtmgr.NewProc("RmGetList")
)

// Restart Manager 常量（RestartManager.h）。
const (
	// rmSessionKeyChars = CCH_RM_SESSION_KEY + 1：RmStartSession 要求的会话键缓冲长度。
	rmSessionKeyChars = 33
	// rmMaxFilesPerQuery 是目录展开为文件清单的上限，防止对系统目录失控。
	// 配合产物/依赖目录跳过，真实源码目录很少触顶。
	rmMaxFilesPerQuery = 2000
	// rmSplitBudget 是单次查询允许的 RM 会话总数（含整批与二分子批）。
	rmSplitBudget = 64
	// rmBadFileWarnLimit 是逐条罗列问题文件的上限，超过则合并为一条汇总。
	rmBadFileWarnLimit = 5
	// errorMoreData 是 RmGetList 的"缓冲不足，需按 Needed 重新分配"信号。
	errorMoreData = syscall.Errno(234)
	// rmGetListMaxRetries 限制 ERROR_MORE_DATA 重试次数，防进程抖动导致的无限循环。
	rmGetListMaxRetries = 3
)

// rmUniqueProcess 对应 Win32 RM_UNIQUE_PROCESS（12 字节）。
type rmUniqueProcess struct {
	processID        uint32
	processStartTime windows.Filetime
}

// rmProcessInfo 对应 Win32 RM_PROCESS_INFO，x86/x64 均为 668 字节：
// 12 (Process) + 512 (strAppName[256]) + 128 (strServiceShortName[64]) + 16 (尾部 4 字段)。
// 数组长度必须带 CCH+1，错一个字节 RmGetList 就会按 668 步进写坏缓冲。
type rmProcessInfo struct {
	process             rmUniqueProcess
	strAppName          [256]uint16
	strServiceShortName [64]uint16
	applicationType     uint32
	appStatus           uint32
	tsSessionID         uint32
	bRestartable        uint32
}

// FileLockEntry 描述一个占用文件的进程。
type FileLockEntry struct {
	PID       uint32 `json:"pid"`
	Process   string `json:"process"`
	ExePath   string `json:"exePath"`
	AppType   string `json:"appType"`
	StartTime int64  `json:"startTime"` // 进程创建时间，Unix 毫秒；0 表示未知
}

// FileLockResult 是文件占用查询结果。
type FileLockResult struct {
	Entries  []FileLockEntry `json:"entries"`
	Scanned  int             `json:"scanned"`  // 实际参与查询的文件数
	Warnings []string        `json:"warnings"` // 被隔离的问题文件等提示
}

// FileLockService 提供文件占用检测（哪个进程锁住了这个文件/目录）。
type FileLockService struct{}

// rmAppTypeNames 把 RM_APP_TYPE 枚举映射为中文描述。
func rmAppTypeNames(t uint32) string {
	switch t {
	case 1:
		return "主窗口应用"
	case 2:
		return "窗口应用"
	case 3:
		return "Windows 服务"
	case 4:
		return "资源管理器"
	case 5:
		return "控制台应用"
	case 6:
		return "关键进程"
	default:
		return "未知类型"
	}
}

// FindLockers 查询占用指定文件或目录（递归展开）的进程列表。
// 无占用返回空列表（不是错误）；批量查询失败时二分隔离问题文件后继续。
func (s *FileLockService) FindLockers(path string) (FileLockResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return FileLockResult{}, errors.New("请输入文件或目录路径")
	}
	path = filepath.Clean(path)
	st, err := os.Stat(path)
	if err != nil {
		return FileLockResult{}, fmt.Errorf("路径不存在或不可访问：%w", err)
	}

	var files []string
	result := FileLockResult{}
	if st.IsDir() {
		files, err = walkDirectoryFiles(path, rmMaxFilesPerQuery)
		if err != nil {
			return FileLockResult{}, err
		}
		if len(files) == 0 {
			return result, nil
		}
		if len(files) >= rmMaxFilesPerQuery {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("目录文件数达到 %d 上限，超出部分未查询；建议缩小到具体子目录", rmMaxFilesPerQuery))
		}
	} else {
		files = []string{path}
	}
	result.Scanned = len(files)

	entries, badFiles := findLockersWithSplit(files, rmSplitBudget, rmQueryLockers)
	result.Entries = entries
	if len(badFiles) > rmBadFileWarnLimit {
		// 坏文件很多时逐条罗列只会刷屏：合并为一条汇总提示。
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d 个文件无法查询占用（多为构建产物或权限受限），建议直接查询具体文件或子目录", len(badFiles)))
	} else {
		for _, f := range badFiles {
			result.Warnings = append(result.Warnings, "无法查询该文件的占用："+f)
		}
	}

	// 去重（同一进程可能锁定多个文件，保留首个条目）并按 PID 排序，保证输出稳定。
	dedup := map[uint32]FileLockEntry{}
	pids := make([]uint32, 0, len(entries))
	for _, e := range entries {
		if _, ok := dedup[e.PID]; !ok {
			dedup[e.PID] = e
			pids = append(pids, e.PID)
		}
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	result.Entries = result.Entries[:0]
	for _, pid := range pids {
		result.Entries = append(result.Entries, dedup[pid])
	}
	return result, nil
}

// skipDirNames 是目录展开时默认跳过的构建产物/依赖目录（大小写不敏感）：
// 占用排查的目标几乎总是源码/配置，这些目录只会耗尽文件上限并产生无意义的警告。
var skipDirNames = map[string]bool{
	"node_modules": true, ".git": true, "build": true, "dist": true,
	"out": true, "obj": true, "bin": true, "target": true,
	".dart_tool": true, "__pycache__": true, ".gradle": true,
	".next": true, ".nuxt": true, ".venv": true, "venv": true,
}

// walkDirectoryFiles 递归展开目录下的普通文件，跳过产物/依赖目录，超过上限即停。
func walkDirectoryFiles(dir string, limit int) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过不可访问的子项
		}
		if d.IsDir() {
			if p != dir && skipDirNames[strings.ToLower(d.Name())] {
				return fs.SkipDir
			}
			return nil
		}
		files = append(files, p)
		if len(files) >= limit {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("遍历目录失败：%w", err)
	}
	return files, nil
}

// rmQueryFunc 是一次批量占用查询的签名，注入以便单测二分/预算逻辑。
type rmQueryFunc func(files []string) ([]FileLockEntry, error)

// findLockersWithSplit 对整批查询失败做二分隔离，返回占用条目与被隔离的问题文件。
// budget 限制 RM 会话总次数（共享计数）：大量文件批量失败时避免会话数指数爆炸，
// 预算耗尽后剩余文件整批隔离（进警告汇总）而非继续下钻。
func findLockersWithSplit(files []string, budget int, query rmQueryFunc) ([]FileLockEntry, []string) {
	st := &splitState{budget: budget, query: query}
	entries := st.recurse(files)
	return entries, st.bad
}

// splitState 携带跨递归层共享的会话预算与隔离结果。
type splitState struct {
	budget int
	bad    []string
	query  rmQueryFunc
}

func (st *splitState) recurse(files []string) []FileLockEntry {
	if len(files) == 0 {
		return nil
	}
	if st.budget <= 0 {
		st.bad = append(st.bad, files...)
		return nil
	}
	entries, err := st.query(files)
	st.budget--
	if err == nil {
		return entries
	}
	if len(files) == 1 {
		// 单个文件仍失败：隔离之，不让一个坏文件毁掉整批结果。
		st.bad = append(st.bad, files[0])
		return nil
	}
	mid := len(files) / 2
	left := st.recurse(files[:mid])
	right := st.recurse(files[mid:])
	return append(left, right...)
}

// rmQueryLockers 用一个 Restart Manager 会话查询一批文件的占用者。
func rmQueryLockers(files []string) ([]FileLockEntry, error) {
	if len(files) == 0 {
		return nil, nil
	}
	paths := make([]*uint16, len(files))
	for i, f := range files {
		ptr, err := windows.UTF16PtrFromString(f)
		if err != nil {
			return nil, err
		}
		paths[i] = ptr
	}

	var session uint32
	// 会话键缓冲必须 CCH_RM_SESSION_KEY+1 个 WCHAR，不能省。
	// 签名：RmStartSession(pSessionHandle, dwSessionFlags, strSessionKey)。
	sessionKey := make([]uint16, rmSessionKeyChars)
	if r1, _, _ := procRmStartSession.Call(
		uintptr(unsafe.Pointer(&session)),
		0, // dwSessionFlags
		uintptr(unsafe.Pointer(&sessionKey[0])),
	); r1 != 0 {
		return nil, windows.Errno(r1)
	}
	// 每用户会话上限 64，成功后无论后续成败都必须 RmEndSession。
	defer procRmEndSession.Call(uintptr(session), 0)

	if r1, _, _ := procRmRegisterResources.Call(
		uintptr(session),
		uintptr(len(paths)),
		uintptr(unsafe.Pointer(&paths[0])),
		0, // nApplications：由文件反查占用者时为 0
		0, // rgApplications
		0, // nServiceRestart
		0, // rgsRestartKeys
	); r1 != 0 {
		return nil, windows.Errno(r1)
	}

	var needed uint32
	count := uint32(0)
	var rebootReasons uint32
	for retry := 0; retry < rmGetListMaxRetries; retry++ {
		needed = 0
		count = 0
		rebootReasons = 0
		// 第一次调用传 NULL 列表只取所需数量：无占用时返回 0，缓冲不足返回 234。
		// lpdwRebootReasons 必须是有效指针，传 NULL 会得到 ERROR_BAD_ARGUMENTS。
		r1, _, _ := procRmGetList.Call(
			uintptr(session),
			uintptr(unsafe.Pointer(&needed)),
			uintptr(unsafe.Pointer(&count)),
			0,
			uintptr(unsafe.Pointer(&rebootReasons)),
		)
		if r1 == 0 {
			return nil, nil // 无占用
		}
		if !errors.Is(windows.Errno(r1), errorMoreData) {
			return nil, windows.Errno(r1)
		}

		// 两次调用之间进程数可能增长，按最新 Needed 分配后重试。
		infos := make([]rmProcessInfo, needed)
		count = needed
		r1, _, _ = procRmGetList.Call(
			uintptr(session),
			uintptr(unsafe.Pointer(&needed)),
			uintptr(unsafe.Pointer(&count)),
			uintptr(unsafe.Pointer(&infos[0])),
			uintptr(unsafe.Pointer(&rebootReasons)),
		)
		if r1 == 0 {
			return rmEntriesFromInfos(infos[:count]), nil
		}
		if !errors.Is(windows.Errno(r1), errorMoreData) {
			return nil, windows.Errno(r1)
		}
	}
	return nil, errorMoreData
}

// rmEntriesFromInfos 把 RM_PROCESS_INFO 数组转为条目，补查进程可执行路径。
func rmEntriesFromInfos(infos []rmProcessInfo) []FileLockEntry {
	entries := make([]FileLockEntry, 0, len(infos))
	for i := range infos {
		info := &infos[i]
		name := windows.UTF16ToString(info.strAppName[:])
		entry := FileLockEntry{
			PID:       info.process.processID,
			Process:   name,
			AppType:   rmAppTypeNames(info.applicationType),
			StartTime: filetimeToUnixMilli(info.process.processStartTime),
		}
		if pi := queryProcessInfo(info.process.processID); pi.path != "" {
			entry.ExePath = pi.path
			if entry.Process == "" {
				entry.Process = pi.name
			}
		}
		if entry.Process == "" {
			entry.Process = fmt.Sprintf("PID %d", entry.PID)
		}
		entries = append(entries, entry)
	}
	return entries
}

// filetimeToUnixMilli 转换 FILETIME 到 Unix 毫秒，零值返回 0。
func filetimeToUnixMilli(ft windows.Filetime) int64 {
	if ft.HighDateTime == 0 && ft.LowDateTime == 0 {
		return 0
	}
	nsec := ft.Nanoseconds()
	if nsec <= 0 {
		return 0
	}
	return nsec / int64(1e6)
}
