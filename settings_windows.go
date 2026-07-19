package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// SetAutostart 设置/取消 PortCheck 开机自启（HKCU 注册表 Run 键）。
func (s *SettingsService) SetAutostart(enabled bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("无法打开注册表 Run 键：%w", err)
	}
	defer key.Close()

	if enabled {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		return key.SetStringValue("PortCheck", `"`+exe+`"`)
	}
	if err := key.DeleteValue("PortCheck"); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}
