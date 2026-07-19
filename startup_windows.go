package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// runKey 是注册表中"开机启动"Run 键的统一路径。
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// StartupApproved 路径（与任务管理器禁用态对齐）。
const (
	approvedRunHKCU           = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
	approvedStartupFolderHKCU = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder`
	approvedRunHKLM           = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
)

// StartupApproved 状态字节（与 Autoruns/任务管理器约定一致）。
const (
	startupApprovedDisabled byte = 0x03
	startupApprovedEnabled  byte = 0x02 // 部分系统为 0x02/0x06；读时 bit0=1 视为禁用
)

// ListStartup 枚举开机启动项：
// 覆盖 HKCU / HKLM 的 Run 键，以及当前用户的 Startup 文件夹；并回填禁用状态。
func (s *StartupService) ListStartup() ([]StartupEntry, error) {
	out := make([]StartupEntry, 0, 16)

	out = appendRunKey(out, registry.CURRENT_USER, "HKCU")
	out = appendRunKey(out, registry.LOCAL_MACHINE, "HKLM")
	out = appendStartupFolder(out)

	// 回填禁用状态与应用图标（同步提取，列表一次返回）。
	for i := range out {
		out[i].Disabled = isStartupDisabled(out[i].Name, out[i].Location)
		exe := resolveStartupExePath(out[i].Command)
		out[i].IconDataURL = iconDataURLForPathSync(exe)
	}
	return out, nil
}

// DeleteStartup 删除启动项；权限不足时自动 UAC 提权重试。
func (s *StartupService) DeleteStartup(name, location string) (StartupOpResult, error) {
	if err := validateStartupArgs(name, location); err != nil {
		return StartupOpResult{Name: name, Action: "delete"}, err
	}
	err := tryDirectOrElevate(func() error {
		return deleteStartupDirect(name, location)
	}, "startup-delete", name, location)
	if err != nil {
		return StartupOpResult{Name: name, Action: "delete"}, err
	}
	return StartupOpResult{
		Name:    name,
		Action:  "delete",
		Message: fmt.Sprintf("已删除启动项：%s", name),
	}, nil
}

// DisableStartup 禁用启动项（可恢复）；权限不足时自动 UAC 提权重试。
func (s *StartupService) DisableStartup(name, location string) (StartupOpResult, error) {
	if err := validateStartupArgs(name, location); err != nil {
		return StartupOpResult{Name: name, Action: "disable"}, err
	}
	err := tryDirectOrElevate(func() error {
		return disableStartupDirect(name, location)
	}, "startup-disable", name, location)
	if err != nil {
		return StartupOpResult{Name: name, Action: "disable"}, err
	}
	return StartupOpResult{
		Name:    name,
		Action:  "disable",
		Message: fmt.Sprintf("已禁用启动项：%s", name),
	}, nil
}

// EnableStartup 启用已禁用的启动项；权限不足时自动 UAC 提权重试。
func (s *StartupService) EnableStartup(name, location string) (StartupOpResult, error) {
	if err := validateStartupArgs(name, location); err != nil {
		return StartupOpResult{Name: name, Action: "enable"}, err
	}
	err := tryDirectOrElevate(func() error {
		return enableStartupDirect(name, location)
	}, "startup-enable", name, location)
	if err != nil {
		return StartupOpResult{Name: name, Action: "enable"}, err
	}
	return StartupOpResult{
		Name:    name,
		Action:  "enable",
		Message: fmt.Sprintf("已启用启动项：%s", name),
	}, nil
}

func validateStartupArgs(name, location string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("启动项名称不能为空")
	}
	switch location {
	case "HKCU", "HKLM", "StartupFolder":
		return nil
	default:
		return fmt.Errorf("未知启动项来源：%s", location)
	}
}

// appendRunKey 读取某个根键下的 Run 键全部字符串值。
func appendRunKey(out []StartupEntry, root registry.Key, location string) []StartupEntry {
	key, err := registry.OpenKey(root, runKeyPath, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return out
	}
	defer key.Close()

	valueNames, err := key.ReadValueNames(-1)
	if err != nil {
		return out
	}
	for _, name := range valueNames {
		val, _, err := key.GetStringValue(name)
		if err != nil {
			continue
		}
		out = append(out, StartupEntry{Name: name, Command: val, Location: location})
	}
	return out
}

// appendStartupFolder 枚举当前用户的"启动"文件夹下的快捷方式/可执行文件。
func appendStartupFolder(out []StartupEntry) []StartupEntry {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return out
	}
	folder := filepath.Join(configDir, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")

	entries, err := os.ReadDir(folder)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		full := filepath.Join(folder, entry.Name())
		out = append(out, StartupEntry{Name: entry.Name(), Command: full, Location: "StartupFolder"})
	}
	return out
}

// deleteStartupDirect 直接删除启动项（注册表值或文件夹文件）。
func deleteStartupDirect(name, location string) error {
	switch location {
	case "HKCU":
		return deleteRunValue(registry.CURRENT_USER, name)
	case "HKLM":
		return deleteRunValue(registry.LOCAL_MACHINE, name)
	case "StartupFolder":
		return deleteStartupFolderFile(name)
	default:
		return fmt.Errorf("未知启动项来源：%s", location)
	}
}

func deleteRunValue(root registry.Key, name string) error {
	key, err := registry.OpenKey(root, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.DeleteValue(name); err != nil {
		return err
	}
	// 同步清理 StartupApproved 中的对应值（忽略失败）。
	_ = deleteApprovedValue(root, name, "Run")
	return nil
}

func deleteStartupFolderFile(name string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	folder := filepath.Join(configDir, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	full := filepath.Join(folder, name)
	// 防止路径穿越：必须在 Startup 文件夹内。
	if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(folder)+string(os.PathSeparator)) &&
		filepath.Clean(full) != filepath.Clean(folder) {
		return fmt.Errorf("非法启动项路径")
	}
	if err := os.Remove(full); err != nil {
		return err
	}
	_ = deleteApprovedValue(registry.CURRENT_USER, name, "StartupFolder")
	return nil
}

// disableStartupDirect 写入 StartupApproved 为禁用态。
func disableStartupDirect(name, location string) error {
	return writeStartupApproved(name, location, true)
}

// enableStartupDirect 写入 StartupApproved 为启用态。
func enableStartupDirect(name, location string) error {
	return writeStartupApproved(name, location, false)
}

// isStartupDisabled 读取 StartupApproved 判断是否禁用。
// 约定：存在值且第 0 字节最低位为 1（0x03/0x01 等）视为禁用；无值视为启用。
func isStartupDisabled(name, location string) bool {
	root, subkey, ok := approvedKeyFor(location)
	if !ok {
		return false
	}
	key, err := registry.OpenKey(root, subkey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	data, _, err := key.GetBinaryValue(name)
	if err != nil || len(data) == 0 {
		return false
	}
	// bit0 == 1 → 禁用（0x03 禁用，0x02/0x06 启用）。
	return data[0]&0x01 != 0
}

func writeStartupApproved(name, location string, disabled bool) error {
	root, subkey, ok := approvedKeyFor(location)
	if !ok {
		return fmt.Errorf("未知启动项来源：%s", location)
	}
	key, _, err := registry.CreateKey(root, subkey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	buf := make([]byte, 12)
	if disabled {
		buf[0] = startupApprovedDisabled
		// FILETIME = 100-ns intervals since 1601-01-01 UTC
		ft := uint64(time.Now().UTC().UnixNano()/100 + 116444736000000000)
		*(*uint64)(unsafe.Pointer(&buf[4])) = ft
	} else {
		buf[0] = startupApprovedEnabled
		// 时间戳填 0
	}
	return key.SetBinaryValue(name, buf)
}

func deleteApprovedValue(root registry.Key, name, kind string) error {
	var subkey string
	switch kind {
	case "Run":
		subkey = approvedRunHKCU // HKCU/HKLM 同路径
	case "StartupFolder":
		subkey = approvedStartupFolderHKCU
	default:
		return nil
	}
	key, err := registry.OpenKey(root, subkey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.DeleteValue(name)
}

func approvedKeyFor(location string) (registry.Key, string, bool) {
	switch location {
	case "HKCU":
		return registry.CURRENT_USER, approvedRunHKCU, true
	case "HKLM":
		return registry.LOCAL_MACHINE, approvedRunHKLM, true
	case "StartupFolder":
		return registry.CURRENT_USER, approvedStartupFolderHKCU, true
	default:
		return 0, "", false
	}
}

// 避免未使用常量告警（HKLM 与 HKCU 路径相同，常量保留语义）。
var _ = approvedRunHKLM
