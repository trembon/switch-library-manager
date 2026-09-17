package app

import (
	"strings"

	"github.com/trembon/switch-library-manager/backend/settings"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) LoadSettings() (*settings.AppSettings, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	result, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return nil, err
	}
	if a.ctx != nil {
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
	}
	return result, nil
}

func (a *App) SaveSettings(value settings.AppSettings) error {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	value = normalizeSettings(value)
	return settings.SaveSettingsWithError(&value, a.baseFolder)
}

func normalizeSettings(value settings.AppSettings) settings.AppSettings {
	if value.Paths.ScanFolders == nil {
		value.Paths.ScanFolders = []string{}
	}
	if value.MissingContent.IgnoreDLCTitleIDs == nil {
		value.MissingContent.IgnoreDLCTitleIDs = []string{}
	}
	if value.MissingContent.IgnoreUpdateIDs == nil {
		value.MissingContent.IgnoreUpdateIDs = []string{}
	}
	if value.Scan.IgnoreFileTypes == nil {
		value.Scan.IgnoreFileTypes = []string{}
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
