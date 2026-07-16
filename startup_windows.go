package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// runKey 是注册表中"开机启动"Run 键的统一路径。
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// ListStartup 以只读方式枚举开机启动项：
// 覆盖 HKCU / HKLM 的 Run 键，以及当前用户的 Startup 文件夹（v1 不做启用/禁用）。
func (s *StartupService) ListStartup() ([]StartupEntry, error) {
	out := make([]StartupEntry, 0, 16)

	out = appendRunKey(out, registry.CURRENT_USER, "HKCU")
	out = appendRunKey(out, registry.LOCAL_MACHINE, "HKLM")
	out = appendStartupFolder(out)

	return out, nil
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
