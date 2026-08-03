//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// elevatedResult 是提权子进程写回结果文件的 JSON 结构。
type elevatedResult struct {
	OK      bool   `json:"ok"`
	Action  string `json:"action"`
	Target  string `json:"target"`
	Message string `json:"message"`
}

const (
	// winErrorAccessDenied 是 Win32 ERROR_ACCESS_DENIED。
	winErrorAccessDenied = syscall.Errno(5)
	// elevatedWaitTimeout 等待提权子进程最长时长。
	elevatedWaitTimeout = 60 * time.Second
	// relaunchExitDelay 提权重启后旧实例退出前的延时，保证前端渲染出提示。
	relaunchExitDelay = 800 * time.Millisecond
)

// isElevated 判断当前进程令牌是否已提权（管理员）。
func isElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	var elevated uint32
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevated)), uint32(unsafe.Sizeof(elevated)), &size); err != nil {
		return false
	}
	return elevated != 0
}

// relaunchElevated 以管理员身份重启应用本体：ShellExecuteEx(runas) 启动新实例（正常窗口，不等待），
// 随后延时退出旧实例，避免重复实例冲突。用于切换到"整个系统"进程范围。
func relaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行路径失败: %w", err)
	}
	if err := shellExecuteRunAsNoWait(exe); err != nil {
		return err
	}
	time.AfterFunc(relaunchExitDelay, func() {
		application.Get().Quit()
	})
	return nil
}

// ensureProcessScopeElevation 在启动早期（GUI 初始化前）调用：
// 当设置要求"整个系统"进程范围但当前进程非管理员时，自动 runas 提权重启自身并退出当前实例，
// 避免重启后退回普通权限导致 SYSTEM 进程再次不可见。用户取消 UAC 或提权失败时继续以当前权限启动。
func ensureProcessScopeElevation() {
	s, err := (&SettingsService{}).GetSettings()
	if err != nil || s.ProcessScope != processScopeSystem || isElevated() {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if err := shellExecuteRunAsNoWait(exe); err != nil {
		return
	}
	os.Exit(0)
}

// isAccessDenied 判断错误是否为权限不足。
func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, winErrorAccessDenied) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == winErrorAccessDenied {
		return true
	}
	// 包装后的错误文案兜底。
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "拒绝访问") ||
		strings.Contains(msg, "access denied")
}

// tryDirectOrElevate 先直接执行 op，权限不足时再 runas 提权重试。
// directFn 为进程内纯函数；elevateOp 为 CLI --op 值。
func tryDirectOrElevate(directFn func() error, elevateOp, name, location string) error {
	err := directFn()
	if err == nil {
		return nil
	}
	if !isAccessDenied(err) {
		return err
	}
	return runElevatedOp(elevateOp, name, location)
}

// runElevatedOp 以 runas 启动自身隐藏子进程执行单次写操作，并读取结果文件。
func runElevatedOp(op, name, location string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行路径失败: %w", err)
	}

	outFile, err := os.CreateTemp("", "portcheck-elevated-*.json")
	if err != nil {
		return fmt.Errorf("创建结果文件失败: %w", err)
	}
	outPath := outFile.Name()
	_ = outFile.Close()
	defer os.Remove(outPath)

	args := []string{
		"--elevated",
		"--out", outPath,
		"--op", op,
		"--name", name,
	}
	if location != "" {
		args = append(args, "--location", location)
	}

	if err := shellExecuteRunAs(exe, args); err != nil {
		return err
	}

	result, err := readElevatedResult(outPath)
	if err != nil {
		return err
	}
	if !result.OK {
		if result.Message == "" {
			return errors.New("提权操作失败")
		}
		return errors.New(result.Message)
	}
	return nil
}

// shellExecuteInfoW 对应 Win32 SHELLEXECUTEINFOW。
type shellExecuteInfoW struct {
	CbSize         uint32
	FMask          uint32
	Hwnd           windows.Handle
	LpVerb         *uint16
	LpFile         *uint16
	LpParameters   *uint16
	LpDirectory    *uint16
	NShow          int32
	HInstApp       windows.Handle
	LpIDList       uintptr
	LpClass        *uint16
	HKeyClass      windows.Handle
	DwHotKey       uint32
	HIconOrMonitor windows.Handle
	HProcess       windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x00000040
	swHide                = 0
	swShownormal          = 1 // SW_SHOWNORMAL：GUI 提权重启时正常显示窗口
	errorCancelled        = syscall.Errno(1223)
)

var (
	modShell32         = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteEx = modShell32.NewProc("ShellExecuteExW")
)

// shellExecuteRunAs 通过 ShellExecuteEx(verb=runas) 启动提权子进程并等待退出。
func shellExecuteRunAs(exe string, args []string) error {
	// 拼参数字符串：每个参数若含空格则加引号。
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		if strings.ContainsAny(a, " \t\"") {
			quoted = append(quoted, `"`+strings.ReplaceAll(a, `"`, `\"`)+`"`)
		} else {
			quoted = append(quoted, a)
		}
	}
	params := strings.Join(quoted, " ")

	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	paramPtr, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}

	info := &shellExecuteInfoW{
		CbSize:       uint32(unsafe.Sizeof(shellExecuteInfoW{})),
		FMask:        seeMaskNoCloseProcess,
		LpVerb:       verb,
		LpFile:       file,
		LpParameters: paramPtr,
		LpDirectory:  dir,
		NShow:        swHide,
	}
	r1, _, callErr := procShellExecuteEx.Call(uintptr(unsafe.Pointer(info)))
	if r1 == 0 {
		// 用户取消 UAC 时通常返回 ERROR_CANCELLED (1223)。
		if errors.Is(callErr, errorCancelled) {
			return errors.New("已取消管理员授权")
		}
		return fmt.Errorf("提权启动失败: %w", callErr)
	}
	if info.HProcess == 0 {
		return errors.New("提权进程句柄为空")
	}
	defer windows.CloseHandle(info.HProcess)

	// WaitForSingleObject 超时（毫秒）。
	status, err := windows.WaitForSingleObject(info.HProcess, uint32(elevatedWaitTimeout/time.Millisecond))
	if err != nil {
		return fmt.Errorf("等待提权进程失败: %w", err)
	}
	if status == uint32(windows.WAIT_TIMEOUT) {
		_ = windows.TerminateProcess(info.HProcess, 1)
		return errors.New("提权操作超时")
	}
	return nil
}

// shellExecuteRunAsNoWait 通过 ShellExecuteEx(verb=runas) 以管理员身份启动自身 GUI（正常窗口）后立即返回，
// 不等待子进程退出。用于提权重启应用本体。
func shellExecuteRunAsNoWait(exe string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}

	info := &shellExecuteInfoW{
		CbSize:      uint32(unsafe.Sizeof(shellExecuteInfoW{})),
		LpVerb:      verb,
		LpFile:      file,
		LpDirectory: dir,
		NShow:       swShownormal,
	}
	r1, _, callErr := procShellExecuteEx.Call(uintptr(unsafe.Pointer(info)))
	if r1 == 0 {
		if errors.Is(callErr, errorCancelled) {
			return errors.New("已取消管理员授权")
		}
		return fmt.Errorf("提权启动失败: %w", callErr)
	}
	return nil
}

// readElevatedResult 读取并解析结果文件。
func readElevatedResult(path string) (elevatedResult, error) {
	// 提权子进程退出后文件应已写好；短暂重试应对磁盘延迟。
	var lastErr error
	for i := 0; i < 10; i++ {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if len(data) == 0 {
			lastErr = errors.New("结果文件为空")
			time.Sleep(50 * time.Millisecond)
			continue
		}
		var r elevatedResult
		if err := json.Unmarshal(data, &r); err != nil {
			return elevatedResult{}, fmt.Errorf("解析结果文件失败: %w", err)
		}
		return r, nil
	}
	return elevatedResult{}, fmt.Errorf("读取提权结果失败: %v", lastErr)
}

// writeElevatedResult 将结果写入指定文件（供提权子进程调用）。
func writeElevatedResult(path string, r elevatedResult) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// runElevatedHelper 解析 CLI 参数并执行对应写操作（提权子进程入口）。
// 由 main.go 在启动 GUI 前调用。
func runElevatedHelper(args []string) {
	outPath, op, name, location, parseErr := parseElevatedArgs(args)
	result := elevatedResult{Action: op, Target: name}

	if parseErr != nil {
		result.OK = false
		result.Message = parseErr.Error()
		if outPath != "" {
			_ = writeElevatedResult(outPath, result)
		}
		os.Exit(2)
	}

	var opErr error
	switch op {
	case "svc-stop":
		opErr = stopServiceDirect(name)
		result.Message = fmt.Sprintf("已发送停止请求：%s", name)
	case "svc-start":
		opErr = startServiceDirect(name)
		result.Message = fmt.Sprintf("已启动服务：%s", name)
	case "startup-delete":
		opErr = deleteStartupDirect(name, location)
		result.Message = fmt.Sprintf("已删除启动项：%s", name)
	case "startup-disable":
		opErr = disableStartupDirect(name, location)
		result.Message = fmt.Sprintf("已禁用启动项：%s", name)
	case "startup-enable":
		opErr = enableStartupDirect(name, location)
		result.Message = fmt.Sprintf("已启用启动项：%s", name)
	case "ping":
		// 协议探针：不执行任何写操作。
		result.Message = "pong"
	default:
		opErr = fmt.Errorf("未知操作：%s", op)
	}

	if opErr != nil {
		result.OK = false
		result.Message = opErr.Error()
		_ = writeElevatedResult(outPath, result)
		os.Exit(1)
	}
	result.OK = true
	if err := writeElevatedResult(outPath, result); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// parseElevatedArgs 解析 --elevated --out --op --name [--location]。
func parseElevatedArgs(args []string) (out, op, name, location string, err error) {
	// args[0] 是程序路径，从 args[1:] 开始。
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--elevated":
			// 标记位，无值。
		case "--out":
			if i+1 >= len(args) {
				return "", "", "", "", errors.New("缺少 --out 参数值")
			}
			i++
			out = args[i]
		case "--op":
			if i+1 >= len(args) {
				return "", "", "", "", errors.New("缺少 --op 参数值")
			}
			i++
			op = args[i]
		case "--name":
			if i+1 >= len(args) {
				return "", "", "", "", errors.New("缺少 --name 参数值")
			}
			i++
			name = args[i]
		case "--location":
			if i+1 >= len(args) {
				return "", "", "", "", errors.New("缺少 --location 参数值")
			}
			i++
			location = args[i]
		default:
			// 忽略未知参数，保持向前兼容。
		}
	}
	if out == "" {
		return "", "", "", "", errors.New("参数错误：需要 --out")
	}
	if op == "" {
		return out, "", "", "", errors.New("参数错误：需要 --op")
	}
	if op != "ping" && name == "" {
		return out, op, "", "", errors.New("参数错误：需要 --name")
	}
	if strings.HasPrefix(op, "startup-") && location == "" {
		return out, op, name, "", errors.New("参数错误：启动项操作需要 --location")
	}
	return out, op, name, location, nil
}

// isElevatedHelper 判断当前进程是否为提权子任务模式。
func isElevatedHelper(args []string) bool {
	for _, a := range args[1:] {
		if a == "--elevated" {
			return true
		}
	}
	return false
}

// ensureElevatedHelperLinked 保留对 exec 的引用，避免某些构建标签下被裁剪。
var _ = exec.Command
