//go:build windows

package main

import (
	"context"
	goruntime "runtime"
	"sync/atomic"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var trayExitRequested atomic.Bool

func (a *App) startTray() {
	go func() {
		goruntime.LockOSThread()
		defer goruntime.UnlockOSThread()
		systray.Run(func() {
			systray.SetIcon(appIcon)
			systray.SetTooltip("PrinterOne — Network Print Server")
			systray.SetOnTapped(func() { runtime.WindowShow(a.ctx) })

			show := systray.AddMenuItem("Открыть PrinterOne", "Показать окно приложения")
			systray.AddSeparator()
			exit := systray.AddMenuItem("Выход", "Остановить сервер и закрыть приложение")
			go func() {
				for {
					select {
					case <-show.ClickedCh:
						runtime.WindowShow(a.ctx)
					case <-exit.ClickedCh:
						trayExitRequested.Store(true)
						runtime.Quit(a.ctx)
						return
					}
				}
			}()
		}, nil)
	}()
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.config.MinimizeToTray && !trayExitRequested.Load() {
		runtime.WindowHide(ctx)
		return true
	}
	return false
}

func (a *App) shutdown(_ context.Context) {
	if a.server != nil {
		a.server.Stop()
	}
	systray.Quit()
}
