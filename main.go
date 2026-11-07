package main

import (
	"embed"
	"log"
	"os"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// overlaySvc 作为包级变量，便于在主窗口导航完成后调用 RestoreIfEnabled 恢复悬浮窗。
var overlaySvc OverlayService

// mainWindow 供 OverlayService 在悬浮窗创建/关闭后强制主窗口重绘，
// 清除透明置顶窗口增删扰动 DWM 合成留下的旧帧残影（见 invalidateMainWindow）。
var mainWindow application.Window

func main() {
	// 提权子进程模式：不启动 GUI，执行单次写操作后立即退出。
	if isElevatedHelper(os.Args) {
		runElevatedHelper(os.Args)
		return
	}

	// 启动早期：设置要求"整个系统"进程范围且当前非管理员时，自动提权重启自身，
	// 否则重启后普通权限下 SYSTEM 进程不可见（与 dev 或打包运行方式无关）。
	ensureProcessScopeElevation()

	app := application.New(application.Options{
		Name:        "PortCheck",
		Description: "Windows local task manager & port watcher built with Wails",
		Services: []application.Service{
			application.NewService(&PortService{}),
			application.NewService(monitorSvc),
			application.NewService(&ServicesService{}),
			application.NewService(&StartupService{}),
			application.NewService(&FileLockService{}),
			application.NewService(&CodingPlanService{}),
			application.NewService(&UpdateService{}),
			application.NewService(&SettingsService{}),
			application.NewService(&overlaySvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "PortCheck",
		Width:            1000,
		Height:           640,
		MinWidth:         900,
		MinHeight:        600,
		EnableFileDrop:   true, // 文件占用页依赖系统拖放解析完整文件路径（data-file-drop-target）
		Frameless:        true, // 无边框：标题栏由前端自绘，通过 --wails-draggable: drag 拖动
		BackgroundColour: application.NewRGB(11, 12, 16),
		URL:              "/",
	})
	mainWindow = window

	// 关闭按钮（前端标题栏）拦截为"最小化到托盘"，保持后台常驻。
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		window.Hide()
		event.Cancel()
	})

	// 主窗口 WebView 导航完成后再恢复悬浮窗，避免启动期主窗口与悬浮窗两个 WebView2
	// 同时冷启动争抢资源导致主窗口白屏。WebView 每次导航都会触发，用 Once 兜底只恢复一次。
	var overlayRestored sync.Once
	window.RegisterHook(events.Windows.WebViewNavigationCompleted, func(event *application.WindowEvent) {
		overlayRestored.Do(func() {
			overlaySvc.RestoreIfEnabled()
		})
	})

	// 文件拖放桥接：框架的 WindowFilesDropped 只分发给 Go 侧监听者，不会广播到前端
	// 事件通道，前端 Events.On("common:WindowFilesDropped") 永远收不到；
	// 这里在 Go 侧接管后手动以同名事件转发（载荷仅含 filenames，够文件占用页使用）。
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(ev *application.WindowEvent) {
		files := ev.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		if app := application.Get(); app != nil {
			app.Event.EmitEvent(&application.CustomEvent{
				Name: "common:WindowFilesDropped",
				Data: map[string]any{"filenames": files},
			})
		}
	})

	// 系统托盘：双击唤起主面板，右键菜单提供显示/退出。
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon).SetTooltip("PortCheck")
	tray.OnDoubleClick(func() {
		window.Show()
	})

	menu := app.NewMenu()
	menu.Add("显示主面板").OnClick(func(*application.Context) { window.Show() })
	menu.AddSeparator()
	menu.Add("退出 PortCheck").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)
	tray.AttachWindow(window).WindowOffset(6)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
