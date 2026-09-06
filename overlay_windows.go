package main

import (
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	user32DLL         = syscall.NewLazyDLL("user32.dll")
	procGetWindowLong = user32DLL.NewProc("GetWindowLongPtrW")
	procSetWindowLong = user32DLL.NewProc("SetWindowLongPtrW")
	procSetWindowPos  = user32DLL.NewProc("SetWindowPos")
)

// fixOverlayExStyle 修正悬浮窗的扩展窗口样式。
// wails v3 alpha.78 把 HiddenOnTaskbar 实现为 WS_EX_NOACTIVATE（见上游
// webview_window_windows.go，WS_EX_TOOLWINDOW 被注释掉）。悬浮窗整个表面都是
// 拖拽区（--wails-draggable），点击时 Wails 通过 WM_NCLBUTTONDOWN(HTCAPTION)
// 进入系统模态移动循环；WS_EX_NOACTIVATE 窗口无法正常激活，该循环会卡住并吞掉
// 全局鼠标输入，表现为「点击悬浮窗后所有窗口都点不动」（上游 discussion #5001）。
// 这里改回 WS_EX_TOOLWINDOW：同样不在任务栏占位，但保留正常激活/点击行为。
// 必须在窗口已创建（Show 之后）调用，否则拿不到 HWND。
func fixOverlayExStyle(win application.Window) {
	if win == nil {
		return
	}
	hwnd := uintptr(win.NativeWindow())
	if hwnd == 0 {
		return
	}
	const (
		wsExToolWindow  = 0x00000080
		wsExNoActivate  = 0x08000000
		swpNoSize       = 0x0001
		swpNoMove       = 0x0002
		swpNoZOrder     = 0x0004
		swpFrameChanged = 0x0020
	)
	const gwlExStyle = ^uintptr(19) // GWL_EXSTYLE = -20
	style, _, _ := procGetWindowLong.Call(hwnd, gwlExStyle)
	newStyle := (style &^ uintptr(wsExNoActivate)) | wsExToolWindow
	procSetWindowLong.Call(hwnd, gwlExStyle, newStyle)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		uintptr(swpNoSize|swpNoMove|swpNoZOrder|swpFrameChanged))
}
