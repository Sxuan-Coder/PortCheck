package main

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// processInfo 是进程名 + 可执行路径的最小描述，端口采样与进程采样共用，
// 避免在两处重复实现 QueryFullProcessImageName 查询逻辑。
type processInfo struct {
	name string
	path string
}

// cachedProcessInfo 按 PID 缓存查询结果，单次采样内对同一 PID 只查询一次系统。
func cachedProcessInfo(cache map[uint32]processInfo, pid uint32) processInfo {
	if info, ok := cache[pid]; ok {
		return info
	}
	info := queryProcessInfo(pid)
	cache[pid] = info
	return info
}

// queryProcessInfo 通过 PROCESS_QUERY_LIMITED_INFORMATION 打开进程并取完整镜像路径，
// 权限要求低、对受保护进程也能成功，是进程信息查询的首选路径。
func queryProcessInfo(pid uint32) processInfo {
	if pid == 0 {
		return processInfo{name: "System Idle Process"}
	}
	if pid == 4 {
		return processInfo{name: "System"}
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return processInfo{name: fmt.Sprintf("PID %d", pid)}
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return processInfo{name: fmt.Sprintf("PID %d", pid)}
	}

	path := windows.UTF16ToString(buffer[:size])
	return processInfo{
		name: filepath.Base(path),
		path: path,
	}
}
