package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"voyagergate/internal/tray"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed all:build/windows/icon.ico
var iconData []byte

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "VoyagerGate 渡桥",
		Width:     1280,
		Height:    750,
		MinWidth:  1100,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 20, B: 27, A: 1},
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			tray.StartTray(ctx, iconData)
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatalf("VoyagerGate 启动失败: %v", err)
	}
}
