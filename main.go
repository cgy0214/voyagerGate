package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"voyagergate/internal/tray"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed all:build/windows/icon.ico
var iconData []byte

// appCtx 保存应用运行时上下文，供单实例唤起等回调使用
var appCtx context.Context

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
		// 点关闭按钮（X）时隐藏窗口到托盘而不是退出程序
		HideWindowOnClose: true,
		// 单实例：再次双击启动时不再新开进程，而是唤起已运行实例的窗口
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "voyagergate",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				runtime.WindowShow(appCtx)
			},
		},
		OnStartup: func(ctx context.Context) {
			appCtx = ctx
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
