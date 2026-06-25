package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "PortCheck",
		Description: "Windows local port watcher built with Wails",
		Services: []application.Service{
			application.NewService(&PortService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "PortCheck",
		Width:            1180,
		Height:           760,
		MinWidth:         980,
		MinHeight:        640,
		BackgroundColour: application.NewRGB(246, 248, 251),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
