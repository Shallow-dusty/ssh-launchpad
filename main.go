package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--elevated-helper" {
		os.Exit(runElevatedHelper(os.Args[2:]))
	}
	app := NewApp()
	err := wails.Run(&options.App{
		Title:            "SSH Launchpad",
		Width:            1240,
		Height:           780,
		MinWidth:         640,
		MinHeight:        560,
		Frameless:        false,
		DisableResize:    false,
		BackgroundColour: &options.RGBA{R: 244, G: 246, B: 250, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		Windows:          &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "9f5bd590-a7c4-4da4-9dce-9ac41020f200",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				if app.ctx != nil {
					wailsruntime.WindowUnminimise(app.ctx)
					wailsruntime.WindowShow(app.ctx)
					wailsruntime.EventsEmit(app.ctx, "launchpad:second-instance")
				}
			},
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
