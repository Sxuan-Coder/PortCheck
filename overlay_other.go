//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// fixOverlayExStyle 非 Windows 平台占位：悬浮窗仅 Windows 有完整实现。
func fixOverlayExStyle(_ application.Window) {}
