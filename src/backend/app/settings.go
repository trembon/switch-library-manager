package app

import (
	"fmt"
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
	// Table layouts are saved on their own by SaveTableLayout. Keep the stored
	// ones so a settings form built from an older copy cannot overwrite them.
	current, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return fmt.Errorf("read current table layouts: %w", err)
	}
	value.GUI.TableLayouts = current.GUI.TableLayouts
	value = normalizeSettings(value)
	if err := settings.SaveSettingsWithError(&value, a.baseFolder); err != nil {
		return err
	}
	a.rememberWindowState = value.GUI.RememberWindowState
	return nil
}

// SaveTableLayout saves the column layout of one table (order, visibility and
// user-set widths). An empty layout resets the table to its default columns.
// Only the table layouts change; other settings are saved as currently loaded.
func (a *App) SaveTableLayout(table string, layout settings.TableLayout) error {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	current, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return fmt.Errorf("save %s table layout: %w", table, err)
	}
	updated, err := settings.WithTableLayout(current, table, layout)
	if err != nil {
		return err
	}
	if err := settings.SaveSettingsWithError(updated, a.baseFolder); err != nil {
		return fmt.Errorf("save %s table layout: %w", table, err)
	}
	return nil
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
