//go:build !windows

package main

import "encoding/base64"

// iconDataURLForPathSync 非 Windows：回退应用 logo。
func iconDataURLForPathSync(path string) string {
	return fallbackAppIconDataURL()
}

// iconDataURLForEmit 非 Windows 占位。
func iconDataURLForEmit(path string) string {
	return ""
}

// resolveStartupExePath 非 Windows 占位。
func resolveStartupExePath(command string) string {
	return ""
}

// fallbackAppIconDataURL 使用嵌入的 PortCheck 应用图标。
func fallbackAppIconDataURL() string {
	if len(appIcon) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(appIcon)
}
