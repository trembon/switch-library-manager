package app

import (
	"strings"

	"github.com/trembon/switch-library-manager/backend/settings"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) LoadSettings() *settings.AppSettings {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	result := settings.ReadSettings(a.baseFolder)
	if a.ctx != nil {
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
	}
	return result
}

func (a *App) SaveSettings(value settings.AppSettings) error {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	value = normalizeSettings(value)
	return settings.SaveSettingsWithError(&value, a.baseFolder)
}

func normalizeSettings(value settings.AppSettings) settings.AppSettings {
	if value.ScanFolders == nil {
		value.ScanFolders = []string{}
	}
	if value.IgnoreDLCTitleIds == nil {
		value.IgnoreDLCTitleIds = []string{}
	}
	if value.IgnoreUpdateTitleIds == nil {
		value.IgnoreUpdateTitleIds = []string{}
	}
	if value.IgnoreFileTypes == nil {
		value.IgnoreFileTypes = []string{}
	}
	return value
}

func (a *App) IsKeysFileAvailable() bool {
	keys, err := settings.SwitchKeys()
	return err == nil && keys != nil && keys.GetKey("header_key") != ""
}

func (a *App) CheckUpdate() (bool, error) {
	newUpdate, err := settings.CheckForUpdates()
	if err != nil && strings.Contains(err.Error(), "dial tcp") {
		return false, nil
	}
	return newUpdate, err
}
