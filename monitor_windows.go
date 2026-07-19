package main

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// 通过 lazyproc 调用 golang.org/x/sys/windows 未直接导出的性能 API。
var (
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	psapi                      = windows.NewLazySystemDLL("psapi.dll")
	procGetSystemTimes         = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx   = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetProcessMemoryInfo   = psapi.NewProc("GetProcessMemoryInfo")
	procGetActiveProcessorCount = kernel32.NewProc("GetActiveProcessorCount")
)

// allProcessorGroups 用于 GetActiveProcessorCount 取全部逻辑核心。
const allProcessorGroups uint16 = 0xFFFF

// memoryStatusEx 对应 Win32 MEMORYSTATUSEX，字段顺序与字宽必须精确匹配。
type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// processMemoryCounters 对应 Win32 PROCESS_MEMORY_COUNTERS（x64 下 72 字节）。
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// filetimeTo100ns 把 Filetime 转为以 100ns 为单位的累计刻度（CPU 时间基础单位）。
func filetimeTo100ns(ft windows.Filetime) int64 {
	return int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
}

// readCPUInfo 读取 CPU 型号名与逻辑核心数，仅在启动时调用一次。
func readCPUInfo() (int, string) {
	cores := runtime.NumCPU()
	if n, err := getActiveProcessorCount(); err == nil && n > 0 {
		cores = int(n)
	}

	name := ""
	if key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE); err == nil {
		if v, _, err := key.GetStringValue("ProcessorNameString"); err == nil {
			name = v
		}
		key.Close()
	}
	return cores, name
}

func getActiveProcessorCount() (uint32, error) {
	r1, _, e := procGetActiveProcessorCount.Call(uintptr(allProcessorGroups))
	if r1 == 0 {
		return 0, e
	}
	return uint32(r1), nil
}

// sampleProcessesAndPerf 是每秒采样的核心：遍历进程快照、计算 CPU% 差值、读取内存，
// 同时读取整机 CPU 与内存，更新 MonitorService 内的"上一次"状态。
//
// elapsed 为距离上次采样的秒数，用于把进程 CPU 时间增量换算为百分比。
func sampleProcessesAndPerf(s *MonitorService, elapsed float64) ([]ProcessInfo, PerfSnapshot) {
	out := make([]ProcessInfo, 0, 256)

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return out, samplePerfOnly(s)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return out, samplePerfOnly(s)
	}

	seen := make(map[uint32]int64, len(s.prevProcTimes))
	infoCache := make(map[uint32]processInfo, 256)
	for {
		pid := entry.ProcessID
		if cpu, mem, ok := queryProcessLoad(pid); ok {
			seen[pid] = cpu
			prev := s.prevProcTimes[pid]
			info := cachedProcessInfo(infoCache, pid)
			out = append(out, ProcessInfo{
				PID:         pid,
				Name:        info.name,
				Path:        info.path,
				CPU:         cpuPercent(cpu, prev, elapsed),
				MemBytes:    mem,
				IconDataURL: iconDataURLForEmit(info.path),
			})
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	s.prevProcTimes = seen

	return out, samplePerfOnly(s)
}

// samplePerfOnly 仅采样整机 CPU/内存并更新系统 CPU 的差值状态。
func samplePerfOnly(s *MonitorService) PerfSnapshot {
	var idle, kernel, user windows.Filetime
	perf := PerfSnapshot{CPUName: s.cpuName, NumCores: s.numCores}

	if r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	); r != 0 {
		curIdle, curKernel, curUser := filetimeTo100ns(idle), filetimeTo100ns(kernel), filetimeTo100ns(user)
		total := (curKernel + curUser) - (s.prevKernel + s.prevUser)
		idleDelta := curIdle - s.prevIdle
		if total > 0 && s.prevKernel > 0 {
			busy := total - idleDelta
			if busy < 0 {
				busy = 0
			}
			perf.CPUPercent = float64(busy) / float64(total) * 100
		}
		s.prevIdle, s.prevKernel, s.prevUser = curIdle, curKernel, curUser
	}

	var mem memoryStatusEx
	mem.dwLength = uint32(unsafe.Sizeof(mem))
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem))); r != 0 && mem.ullTotalPhys > 0 {
		perf.MemTotalGB = bytesToGB(mem.ullTotalPhys)
		perf.MemUsedGB = bytesToGB(mem.ullTotalPhys - mem.ullAvailPhys)
	}

	return perf
}

// queryProcessLoad 返回进程累计 CPU 时间（user+kernel，100ns 刻度）与工作集字节，失败时 ok=false。
func queryProcessLoad(pid uint32) (cpu int64, mem uint64, ok bool) {
	if pid == 0 {
		return 0, 0, false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, 0, false
	}
	defer windows.CloseHandle(handle)

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return 0, 0, false
	}
	cpu = filetimeTo100ns(kernel) + filetimeTo100ns(user)

	var counters processMemoryCounters
	counters.cb = uint32(unsafe.Sizeof(counters))
	if r, _, _ := procGetProcessMemoryInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.cb)); r == 0 {
		return cpu, 0, true
	}
	return cpu, uint64(counters.workingSetSize), true
}

// processNameByPID 复用端口服务中已实现的最小进程信息查询，避免逻辑重复。
func processNameByPID(pid uint32) string {
	return queryProcessInfo(pid).name
}

// cpuPercent 把两次采样的累计 CPU 时间增量换算为单核基准百分比（原始值，可超过 100%）。
// 单核基准能直观反映"吃了几个核"，前端再除以核心数得到整机基准（0-100%）。
func cpuPercent(cur, prev int64, elapsed float64) float64 {
	if elapsed <= 0 || prev <= 0 || cur <= prev {
		return 0
	}
	// 1 秒 = 1e7 个 100ns 单位
	percent := float64(cur-prev) / (elapsed * 1e7) * 100
	if percent < 0 {
		return 0
	}
	return percent
}

// portCountsLight 仅统计 TCP/UDP 连接数量与唯一进程数，不解析进程名/路径，
// 供 MonitorService 每秒推送 PortStats 时使用，避免对每个端口行做 OpenProcess。
func portCountsLight() PortListResult {
	result := PortListResult{}
	uniquePids := map[uint32]struct{}{}

	countTCP := func(family uint32) {
		buf, err := queryTable(procGetExtendedTCPTable, family, tcpTableOwnerPIDAll)
		if err != nil || len(buf) < 4 {
			return
		}
		count := *(*uint32)(unsafe.Pointer(&buf[0]))
		for i := uint32(0); i < count; i++ {
			rowSize := uintptr(unsafe.Sizeof(mibTCPRowOwnerPID{}))
			if family == afInet6 {
				rowSize = unsafe.Sizeof(mibTCP6RowOwnerPID{})
			}
			offset := uintptr(4) + uintptr(i)*rowSize
			if int(offset)+int(rowSize) > len(buf) {
				break
			}
			var pid uint32
			var state uint32
			if family == afInet {
				row := (*mibTCPRowOwnerPID)(unsafe.Pointer(&buf[offset]))
				pid, state = row.OwningPID, row.State
			} else {
				row := (*mibTCP6RowOwnerPID)(unsafe.Pointer(&buf[offset]))
				pid, state = row.OwningPID, row.State
			}
			result.TCPCount++
			if state == 2 { // LISTENING
				result.ListeningCount++
			}
			uniquePids[pid] = struct{}{}
		}
	}

	countUDP := func(family uint32) {
		buf, err := queryTable(procGetExtendedUDPTable, family, udpTableOwnerPID)
		if err != nil || len(buf) < 4 {
			return
		}
		count := *(*uint32)(unsafe.Pointer(&buf[0]))
		for i := uint32(0); i < count; i++ {
			rowSize := uintptr(unsafe.Sizeof(mibUDPRowOwnerPID{}))
			if family == afInet6 {
				rowSize = unsafe.Sizeof(mibUDP6RowOwnerPID{})
			}
			offset := uintptr(4) + uintptr(i)*rowSize
			if int(offset)+int(rowSize) > len(buf) {
				break
			}
			if family == afInet {
				uniquePids[(*mibUDPRowOwnerPID)(unsafe.Pointer(&buf[offset])).OwningPID] = struct{}{}
			} else {
				uniquePids[(*mibUDP6RowOwnerPID)(unsafe.Pointer(&buf[offset])).OwningPID] = struct{}{}
			}
			result.UDPCount++
			result.ListeningCount++
		}
	}

	countTCP(afInet)
	countTCP(afInet6)
	countUDP(afInet)
	countUDP(afInet6)
	result.ProcessCount = len(uniquePids)
	return result
}

func bytesToGB(b uint64) float64 {
	const gb = 1024 * 1024 * 1024
	return float64(b) / gb
}
