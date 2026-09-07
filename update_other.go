//go:build !windows

package main

import "fmt"

// openInstaller 非 Windows 平台占位：仅返回不支持的错误，与仓库跨平台约定一致。
func openInstaller(path string) error {
	return fmt.Errorf("当前平台暂不支持自动启动安装包：%s", path)
}
