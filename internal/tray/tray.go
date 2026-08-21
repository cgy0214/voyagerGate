package tray

import (
	"context"
	"syscall"
	"time"
	"unsafe"

	"github.com/getlantern/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"voyagergate/internal/version"
)

var (
	trayCtx     context.Context
	origWndProc uintptr

	user32             = syscall.NewLazyDLL("user32.dll")
	pFindWindowW       = user32.NewProc("FindWindowW")
	pSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcW   = user32.NewProc("CallWindowProcW")
	pShowWindow        = user32.NewProc("ShowWindow")
)

const (
	_SC_MINIMIZE   = 0xF020
	_SW_HIDE       = 0
	_GWL_WNDPROC   = -4
	_WM_SYSCOMMAND = 0x0112
)

func StartTray(c context.Context, iconData []byte) {
	trayCtx = c
	go runSystray(iconData)
	go hookMinimize()
}

func runSystray(iconData []byte) {
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

func hookMinimize() {
	time.Sleep(500 * time.Millisecond)
	title, _ := syscall.UTF16PtrFromString("VoyagerGate 渡桥")
	hwnd, _, _ := pFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		return
	}
	newProc := syscall.NewCallback(wndProc)
	ret, _, _ := pSetWindowLongPtrW.Call(hwnd, ^uintptr(4-1), newProc)
	if ret != 0 {
		origWndProc = ret
	}
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == _WM_SYSCOMMAND && (wParam&0xFFF0) == _SC_MINIMIZE && origWndProc != 0 {
		pShowWindow.Call(hwnd, _SW_HIDE)
		return 0
	}
	if origWndProc != 0 {
		r, _, _ := pCallWindowProcW.Call(origWndProc, hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	return 0
}
