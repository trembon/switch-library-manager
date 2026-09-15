package app

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startup(ctx context.Context) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	a.ctx = ctx
}

func (a *App) shutdown(context.Context) {}

func (a *App) applicationMenu() *menu.Menu {
	appMenu := menu.NewMenu()
	appMenu.Append(menu.EditMenu())

	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Rescan", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(a.ctx, "rescan", false)
	})
	fileMenu.AddText("Hard rescan", nil, func(_ *menu.CallbackData) {
		a.state.mu.Lock()
		err := a.localDbManager.ClearScanData()
		a.state.mu.Unlock()
		if err != nil {
			a.emitError(err)
			return
		}
		wailsruntime.EventsEmit(a.ctx, "rescan", true)
	})

	return appMenu
}

func (a *App) emitError(err error) {
	if err == nil {
		return
	}
	a.sugarLogger.Error(err)
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "error", err.Error())
	}
}
