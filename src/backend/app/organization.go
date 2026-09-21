package app

import (
	"errors"

	"github.com/trembon/switch-library-manager/backend/process"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func (a *App) GetOrganizationPreview() (OrganizationPreview, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()

	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return OrganizationPreview{}, err
	}
	if !process.IsOptionsValid(settingsObj.Organization) {
		return OrganizationPreview{}, errors.New("the organize options in settings.json are not valid, please check that the template contains file/folder name")
	}

	entries := process.BuildOrganizationPreview(
		settingsObj.Paths.LibraryFolder,
		settingsObj.Organization,
		a.state.localDB,
		a.state.switchDB,
	)
	previewEntries := make([]OrganizationPreviewEntry, 0, len(entries))
	for _, entry := range entries {
		previewEntries = append(previewEntries, OrganizationPreviewEntry{Kind: entry.Kind, Path: entry.Path})
	}
	return OrganizationPreview{RootFolder: settingsObj.Paths.LibraryFolder, Entries: previewEntries}, nil
}

func (a *App) OrganizeLibrary() error {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.state.switchDB == nil || a.state.localDB == nil {
		return errors.New("local and title databases must be loaded")
	}

	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return err
	}
	if !process.IsOptionsValid(settingsObj.Organization) {
		return errors.New("the organize options in settings.json are not valid, please check that the template contains file/folder name")
	}
	progress := progressReporter{app: a}
	process.OrganizeByFolders(settingsObj.Paths.LibraryFolder, a.state.localDB, a.state.switchDB, progress)
	if settingsObj.Organization.DeleteOldUpdateFiles {
		process.DeleteOldUpdates(a.baseFolder, a.state.localDB, progress)
	}
	return nil
}
