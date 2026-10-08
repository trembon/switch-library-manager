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

type windowStartState struct {
	Width           int
	Height          int
	X               int
	Y               int
	RestorePosition bool
	Center          bool
}

func resolveWindowStartState(state *settings.WindowState, screens []wailsruntime.Screen, screenErr error) windowStartState {
	start := windowStartState{Width: defaultWindowWidth, Height: defaultWindowHeight, Center: true}
	if state == nil || screenErr != nil {
		return start
	}

	currentScreen, found := startupScreen(screens)
	if !found {
		return start
	}

	if state.ScreenWidth > 0 && state.ScreenHeight > 0 {
		for _, screen := range screens {
			if screen.Size.Width != state.ScreenWidth || screen.Size.Height != state.ScreenHeight {
				continue
			}
			if state.Width <= screen.Size.Width && state.Height <= screen.Size.Height {
				return windowStartState{
					Width:           state.Width,
					Height:          state.Height,
					X:               state.X,
					Y:               state.Y,
					RestorePosition: true,
				}
			}
			break
		}
	}

	start.Width = min(state.Width, currentScreen.Size.Width)
	start.Height = min(state.Height, currentScreen.Size.Height)
	return start
}

func startupScreen(screens []wailsruntime.Screen) (wailsruntime.Screen, bool) {
	for _, screen := range screens {
		if screen.IsCurrent && screen.Size.Width > 0 && screen.Size.Height > 0 {
			return screen, true
		}
	}
	return wailsruntime.Screen{}, false
}

func currentScreenSize(screens []wailsruntime.Screen) (int, int, bool) {
	for _, screen := range screens {
		if screen.IsCurrent && screen.Size.Width > 0 && screen.Size.Height > 0 {
			return screen.Size.Width, screen.Size.Height, true
		}
	}
	return 0, 0, false
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()

	if !a.rememberWindowState || !wailsruntime.WindowIsNormal(ctx) {
		return false
	}
	width, height := wailsruntime.WindowGetSize(ctx)
	x, y := wailsruntime.WindowGetPosition(ctx)
	screens, err := wailsruntime.ScreenGetAll(ctx)
	if err != nil {
		a.sugarLogger.Warnf("failed to read screen bounds; keeping previously saved window state: %v", err)
		return false
	}
	screenWidth, screenHeight, found := currentScreenSize(screens)
	if !found {
		a.sugarLogger.Warn("failed to identify current screen; keeping previously saved window state")
		return false
	}
	state := &settings.WindowState{
		Width:        width,
		Height:       height,
		X:            x,
		Y:            y,
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
	}
	if err := saveWindowStateIfNeeded(a.baseFolder, true, true, state); err != nil {
		a.sugarLogger.Errorf("failed to save window bounds: %v", err)
	}
	return false
}
