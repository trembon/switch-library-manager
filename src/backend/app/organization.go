package app

import (
	"errors"

	"github.com/trembon/switch-library-manager/backend/process"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func (a *App) OrganizeLibrary() error {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.state.switchDB == nil || a.state.localDB == nil {
		return errors.New("local and title databases must be loaded")
	}

	settingsObj := settings.ReadSettings(a.baseFolder)
	if !process.IsOptionsValid(settingsObj.OrganizeOptions) {
		return errors.New("the organize options in settings.json are not valid, please check that the template contains file/folder name")
	}
	progress := progressReporter{app: a}
	process.OrganizeByFolders(settingsObj.Folder, a.state.localDB, a.state.switchDB, progress)
	if settingsObj.OrganizeOptions.DeleteOldUpdateFiles {
		process.DeleteOldUpdates(a.baseFolder, a.state.localDB, progress)
	}
	return nil
}
