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
	windowState := a.windowState
	rememberWindowState := a.rememberWindowState
	a.state.mu.Unlock()

	if rememberWindowState && windowState != nil {
		screens, screenErr := wailsruntime.ScreenGetAll(ctx)
		start := resolveWindowStartState(windowState, screens, screenErr)
		if screenErr != nil {
			a.sugarLogger.Warnf("failed to read screens while restoring window state; using default window bounds: %v", screenErr)
		} else if _, found := startupScreen(screens); !found {
			a.sugarLogger.Warn("no usable screen information while restoring window state; using default window bounds")
		} else if !start.RestorePosition {
			a.sugarLogger.Infof("saved window display is unavailable or too small; resizing and centering on the current screen")
		}

		wailsruntime.WindowSetSize(ctx, start.Width, start.Height)
		if start.RestorePosition {
			wailsruntime.WindowSetPosition(ctx, start.X, start.Y)
		} else if start.Center {
			wailsruntime.WindowCenter(ctx)
		}
	}

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
