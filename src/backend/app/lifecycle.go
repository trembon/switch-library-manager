package app

import (
	"context"
	"fmt"

	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startup(ctx context.Context) {
	a.state.mu.Lock()
	a.ctx = ctx
	migrationInfo := a.migrationInfo
	a.migrationInfo = nil
	a.state.mu.Unlock()

	if migrationInfo == nil {
		return
	}

	_, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.InfoDialog,
		Title:   "Settings migrated to v2",
		Message: settingsMigrationMessage(migrationInfo),
	})
	if err != nil {
		a.sugarLogger.Error("show settings migration notice", err)
	}
}

func settingsMigrationMessage(migrationInfo *settings.MigrationInfo) string {
	return fmt.Sprintf("Your legacy settings.json was preserved as:\n\n%s\n\nA new v2 settings.json was created with default values, which are now active. Manually copy the settings you want into the new file using docs/settings-v2.md, then restart the application.", migrationInfo.BackupPath)
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
