package app

import (
	"context"

	"github.com/trembon/switch-library-manager/backend/settings"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

const (
	defaultWindowWidth  = 1200
	defaultWindowHeight = 600
)

func loadWindowState(baseFolder string, enabled bool, logger *zap.SugaredLogger) *settings.WindowState {
	if !enabled {
		return nil
	}
	state, err := settings.ReadWindowState(baseFolder)
	if err != nil {
		if logger != nil {
			logger.Warnf("failed to load saved window bounds; using default window size and position: %v", err)
		}
		return nil
	}
	return state
}

func windowSizeForStart(enabled bool, state *settings.WindowState) (int, int) {
	if enabled && state != nil {
		return state.Width, state.Height
	}
	return defaultWindowWidth, defaultWindowHeight
}

func saveWindowStateIfNeeded(baseFolder string, enabled, normal bool, state *settings.WindowState) error {
	if !enabled || !normal {
		return nil
	}
	return settings.SaveWindowState(state, baseFolder)
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()

	if !a.rememberWindowState || !wailsruntime.WindowIsNormal(ctx) {
		return false
	}
	width, height := wailsruntime.WindowGetSize(ctx)
	x, y := wailsruntime.WindowGetPosition(ctx)
	state := &settings.WindowState{Width: width, Height: height, X: x, Y: y}
	if err := saveWindowStateIfNeeded(a.baseFolder, true, true, state); err != nil {
		a.sugarLogger.Errorf("failed to save window bounds: %v", err)
	}
	return false
}
