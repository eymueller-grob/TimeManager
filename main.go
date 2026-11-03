package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "TimeManager",
		Width:     1280,
		Height:    860,
		MinWidth:  960,
		MinHeight: 650,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 251, A: 255},
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "TimeManager-6a8f05b3-bfca-45e0-872a-27bfcb35035d",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.restoreWindow()
			},
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal("Starting TimeManager: ", err)
	}
}
