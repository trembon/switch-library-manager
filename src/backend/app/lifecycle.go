package app

import (
	"context"
	"fmt"

	"github.com/trembon/switch-library-manager/backend/settings"
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
		Title:   "Settings migration required",
		Message: settingsMigrationMessage(migrationInfo),
	})
	if err != nil {
		a.sugarLogger.Error("show settings migration notice", err)
	}
}

func settingsMigrationMessage(migrationInfo *settings.MigrationInfo) string {
	return fmt.Sprintf("Your older settings.json was preserved as:\n\n%s\n\nA new settings.json was created with current default values, which are now active. Manually copy the settings you want into the new file using docs/settings.md, then restart the application.", migrationInfo.BackupPath)
}

func (a *App) shutdown(context.Context) {}
