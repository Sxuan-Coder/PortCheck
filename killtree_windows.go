//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// errTreeEnumerate 表示无法构建可靠的进程快照，调用方应回退单进程结束。
var errTreeEnumerate = errors.New("进程树枚举失败")

// errPIDReused 表示进程身份校验失败（PID 已被其他进程复用），拒绝操作。
var errPIDReused = errors.New("进程身份校验失败（PID 已被复用），已跳过")

// treeKillWaitTimeout 是单个进程 TerminateProcess 后等待退出的时限。
const treeKillWaitTimeout = 3000

// TreeKillFailure 记录树结束中单个进程的失败原因。
type TreeKillFailure struct {
	PID uint32
	Err string
}

// TreeKillReport 是一次进程树结束的逐项结果。
type TreeKillReport struct {
	Killed  []uint32          // 已确认死亡的 PID，叶子在前、根在后
	Failed  []TreeKillFailure // 终止失败的 PID 与原因
	Pruned  int               // 命中保护名单被整棵剪枝的节点数
	Skipped int               // 打不开或身份无法验证而跳过的进程数
}

// procSnapshotEntry 是父子关系构建所需的最小进程条目。
type procSnapshotEntry struct {
	PID  uint32
	PPID uint32
}

// planTreeKill 由进程快照构建叶子优先的终止计划（纯函数，可单测）。
// 规则：
//   - 子进程创建时间必须晚于父进程，否则视为 PID 复用，从该边断链；
//   - 打不开（取不到创建时间）的进程跳过，且不再向下钻（无法验证其后代身份）；
//   - 受保护 PID（0/4/当前应用自身）命中时整棵子树剪枝，防止误杀界面进程；
//   - visited 防御快照中的环。
//
// 返回的 order 叶子在前、根在后，可直接按序终止。
func planTreeKill(entries []procSnapshotEntry, root uint32, creation map[uint32]int64, protected func(uint32) bool) (order []uint32, report TreeKillReport) {
	children := map[uint32][]uint32{}
	for _, e := range entries {
		if e.PID == e.PPID {
			continue // 快照自环防御
		}
		children[e.PPID] = append(children[e.PPID], e.PID)
	}

	visited := make(map[uint32]bool)
	var walk func(pid, parentPID uint32)
	walk = func(pid, parentPID uint32) {
		if visited[pid] {
			return
		}
		visited[pid] = true
		if protected(pid) {
			report.Pruned++
			return // 整棵子树剪枝：不再下钻
		}
		ct, ok := creation[pid]
		if !ok {
			report.Skipped++
			return
		}
		if parentPID != 0 {
			pct, ok := creation[parentPID]
			if !ok || ct <= pct {
				return // 子进程不可能早于父进程：PID 复用，断链
			}
		}
		for _, c := range children[pid] {
			walk(c, pid)
		}
		order = append(order, pid)
	}
	walk(root, 0)
	return order, report
}

// terminateTree 结束 root 及其全部可验证后代。
// 语义约定：
//   - 快照枚举不可靠 → 返回 (nil, errTreeEnumerate)，调用方回退单进程结束；
//   - 根进程打不开 → 返回 (report, err)，由调用方分类（权限不足等）；
//   - 部分后代失败 → 返回 (report, err)，Killed 保留已结束项，调用方不得再单杀根。
func terminateTree(root uint32) (*TreeKillReport, error) {
	if err := validateKillPID(root); err != nil {
		return nil, err
	}

	// 根进程一次性以 终止+同步+查询 权限打开，校验与终止共用同一句柄，
	// 句柄本身保证对象身份，杜绝校验与终止之间 PID 复用。
	rootHandle, err := windows.OpenProcess(
		windows.PROCESS_TERMINATE|windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, root)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(rootHandle)
	rootCreate, ok := processCreationTime(rootHandle)
	if !ok {
		return nil, errors.New("无法读取目标进程启动时间")
	}

	entries, entriesOK := snapshotProcEntries()
	if !entriesOK {
		return nil, errTreeEnumerate
	}

	self := uint32(os.Getpid())
	creation := map[uint32]int64{root: rootCreate}
	for _, e := range entries {
		if e.PID == root || creation[e.PID] != 0 {
			continue
		}
		if ct, ok := queryCreationTimeByPID(e.PID); ok {
			creation[e.PID] = ct
		}
	}

	order, report := planTreeKill(entries, root, creation, func(pid uint32) bool {
		return pid == 0 || pid == 4 || pid == self
	})
	if len(order) == 0 {
		return &report, errors.New("目标进程无法安全结束（身份无法验证或命中保护名单）")
	}

	for _, pid := range order {
		if pid == root {
			if err := terminateHandleValidated(rootHandle, rootCreate); err != nil {
				report.Failed = append(report.Failed, TreeKillFailure{PID: pid, Err: err.Error()})
			} else {
				report.Killed = append(report.Killed, pid)
			}
			continue
		}
		if err := terminateChildByPID(pid, creation[pid]); err != nil {
			if errors.Is(err, errPIDReused) {
				report.Skipped++
				continue
			}
			report.Failed = append(report.Failed, TreeKillFailure{PID: pid, Err: err.Error()})
			continue
		}
		report.Killed = append(report.Killed, pid)
	}

	var firstErr error
	if len(report.Failed) > 0 {
		firstErr = fmt.Errorf("%d 个进程结束失败（首个：%s）", len(report.Failed), report.Failed[0].Err)
	}
	return &report, firstErr
}

// snapshotProcEntries 枚举进程快照。Process32Next 的非正常错误会使本次枚举
// 不可信，整体重试最多 3 次；仍失败返回 false，调用方回退单进程结束。
func snapshotProcEntries() ([]procSnapshotEntry, bool) {
	for attempt := 0; attempt < 3; attempt++ {
		snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
		if err != nil {
			continue
		}
		var entries []procSnapshotEntry
		var entry windows.ProcessEntry32
		entry.Size = uint32(unsafe.Sizeof(entry))
		err = windows.Process32First(snapshot, &entry)
		for err == nil {
			entries = append(entries, procSnapshotEntry{PID: entry.ProcessID, PPID: entry.ParentProcessID})
			err = windows.Process32Next(snapshot, &entry)
		}
		windows.CloseHandle(snapshot)
		// 仅 ERROR_NO_MORE_FILES 是正常的枚举终点；其他错误视为快照不可信，重试。
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return entries, true
		}
	}
	return nil, false
}

// terminateChildByPID 打开子进程、复验创建时间后终止，限时等待退出。
func terminateChildByPID(pid uint32, createTime int64) error {
	handle, err := windows.OpenProcess(
		windows.PROCESS_TERMINATE|windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return fmt.Errorf("打开进程 %d 失败：%w", pid, err)
	}
	defer windows.CloseHandle(handle)
	return terminateHandleValidated(handle, createTime)
}

// terminateHandleValidated 在终止前复验句柄对应的创建时间与记录一致
// （防收集与终止间隙 PID 复用），随后 TerminateProcess 并限时等待。
func terminateHandleValidated(handle windows.Handle, createTime int64) error {
	ct, ok := processCreationTime(handle)
	if !ok || ct != createTime {
		return errPIDReused
	}
	if err := windows.TerminateProcess(handle, 1); err != nil {
		return err
	}
	status, err := windows.WaitForSingleObject(handle, treeKillWaitTimeout)
	if err != nil {
		return err
	}
	if status == uint32(windows.WAIT_TIMEOUT) {
		return errTerminateTimeout
	}
	return nil
}

// processCreationTime 从已打开的进程句柄读取创建时间（100ns 刻度）。
func processCreationTime(handle windows.Handle) (int64, bool) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return 0, false
	}
	return filetimeTo100ns(creation), true
}

// queryCreationTimeByPID 打开进程（最低查询权限）读取创建时间，打不开返回 false。
func queryCreationTimeByPID(pid uint32) (int64, bool) {
	if pid == 0 || pid == 4 {
		return 0, false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(handle)
	return processCreationTime(handle)
}
