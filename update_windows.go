package main

import (
	"fmt"
	"os/exec"
)

// openInstaller 启动已下载的安装包（Windows：cmd /c start 打开 exe）。
// 安装器启动后由用户点击安装；应用本体是否退出交给 NSIS 安装脚本处理。
func openInstaller(path string) error {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w", err)
	}
	// start 立即返回，安装器由资源管理器拉起，无需等待退出。
	return cmd.Process.Release()
}
