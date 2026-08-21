// Package main VoyagerGate 渡桥 — 本地开发专用智能代理网关。
// 作者：rabbit boy_0214@sina.com

package main

import (
	"context"

	"voyagergate/internal/core"
)

// App Wails 绑定入口
type App struct {
	*core.Core
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{Core: core.NewCore()}
}

// startup Wails 启动回调：注入运行时上下文并同步代理状态
func (a *App) startup(ctx context.Context) {
	a.Core.Startup(ctx)
}
