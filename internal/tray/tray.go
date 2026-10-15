package tray

import (
	"context"
	"runtime"

	"github.com/getlantern/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"voyagergate/internal/version"
)

var trayCtx context.Context

func StartTray(c context.Context, iconData []byte) {
	trayCtx = c
	go runSystray(iconData)
}

func runSystray(iconData []byte) {
	// systray 的 Windows 实现要求：创建托盘窗口和 GetMessage 消息泵必须在同一个
	// OS 线程上执行。若不加锁，Go 调度器可能在两次 syscall 之间把本 goroutine
	// 迁移到别的线程，托盘消息会永远收不到，表现为点击托盘无响应、菜单打不开。
	runtime.LockOSThread()
	systray.Run(func() {
		if len(iconData) > 0 {
			systray.SetIcon(iconData)
		}
		systray.SetTitle("VoyagerGate 渡桥")
		systray.SetTooltip("微服务本地联调网关")
		mShow := systray.AddMenuItem("主界面", "恢复显示主窗口")
		mAbout := systray.AddMenuItem("关于", "关于 VoyagerGate")
		mQuit := systray.AddMenuItem("退出", "退出程序")
		for {
			select {
			case <-mShow.ClickedCh:
				wailsRuntime.WindowShow(trayCtx)
			case <-mAbout.ClickedCh:
				_, _ = wailsRuntime.MessageDialog(trayCtx, wailsRuntime.MessageDialogOptions{
					Title:   "关于 VoyagerGate 渡桥",
					Message: "微服务本地联调网关 —— 本地与远端，一桥相连；本地离线，自动渡回。\n\n版本 " + version.Display() + "\n作者：rabbit boy_0214@sina.com",
				})
			case <-mQuit.ClickedCh:
				systray.Quit()
				wailsRuntime.Quit(trayCtx)
				return
			}
		}
	}, func() {})
}
