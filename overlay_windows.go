package main

import (
	"syscall"
	"time"

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

// resetMainWindowScroll 复位主窗口根视口的滚动偏移。
// 实测根因（CDP 诊断复现）：悬浮窗创建/销毁瞬间，主窗口 WebView2 controller bounds
// 被瞬时置为悬浮窗的小尺寸，浏览器为让刚聚焦的设置项保持可见而滚动根视口；bounds
// 恢复后该偏移卡死——内容整体上移、窗口下方留黑、标题栏拖拽命中错位（可拖区随内容
// 一起上移）。此状态下 scrollTop/scrollTo 均读为 0 / 无效（overflow:hidden 视口），
// 唯一验证有效的复位是 scrollIntoView 把根滚回顶部。
// 前端已在 Apply 前.blur() 移除焦点（useSettings.ts），从源头避免该滚动；此处兜底
// 立即执行一次 + 150ms 后补一次，覆盖偏移晚于 Apply 返回的时序。
// 非 Windows 平台无此问题（见 overlay_other.go 占位）。
func resetMainWindowScroll() {
	if mainWindow == nil {
		return
	}
	const js = `(function(){var t=document.querySelector('.titlebar');(t||document.body).scrollIntoView();})()`
	mainWindow.ExecJS(js)
	time.AfterFunc(150*time.Millisecond, func() {
		if mainWindow != nil {
			mainWindow.ExecJS(js)
		}
	})
}
