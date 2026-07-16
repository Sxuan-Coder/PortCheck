package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	app := application.New(application.Options{
		Name:        "PortCheck",
		Description: "Windows local task manager & port watcher built with Wails",
		Services: []application.Service{
			application.NewService(&PortService{}),
			application.NewService(&MonitorService{}),
			application.NewService(&ServicesService{}),
			application.NewService(&StartupService{}),
			application.NewService(&UpdateService{}),
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
		Frameless:        true, // 无边框：标题栏由前端自绘，通过 --wails-draggable: drag 拖动
		BackgroundColour: application.NewRGB(11, 12, 16),
		URL:              "/",
	})

	// 关闭按钮（前端标题栏）拦截为"最小化到托盘"，保持后台常驻。
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		window.Hide()
		event.Cancel()
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
