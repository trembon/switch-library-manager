package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func (a *App) UpdateDB() error {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.state.switchDB != nil {
		return nil
	}
	switchDB, err := a.buildSwitchDB()
	if err != nil {
		return err
	}
	a.state.switchDB = switchDB
	return nil
}

func (a *App) UpdateLocalLibrary(ignoreCache bool) (LocalLibraryData, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()
	if a.state.switchDB == nil {
		return LocalLibraryData{}, errors.New("title database is not loaded")
	}

	localDB, err := a.buildLocalDB(ignoreCache)
	if err != nil {
		return LocalLibraryData{}, err
	}
	return buildLocalLibraryData(localDB, a.state.switchDB), nil
}

func (a *App) buildSwitchDB() (*db.SwitchTitlesDB, error) {
	settingsObj := settings.ReadSettings(a.baseFolder)
	a.updateProgress(1, 4, "Downloading titles.json")

	filename := filepath.Join(a.baseFolder, settings.TITLE_JSON_FILENAME)
	titleFile, titlesETag, err := db.LoadAndUpdateFile(settingsObj.TitlesJsonUrl, filename, settingsObj.TitlesEtag)
	if err != nil {
		return nil, errors.New("failed to download switch titles [reason:" + err.Error() + "]")
	}
	settingsObj.TitlesEtag = titlesETag

	a.updateProgress(2, 4, "Downloading versions.json")
	filename = filepath.Join(a.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsETag, err := db.LoadAndUpdateFile(settingsObj.VersionsJsonUrl, filename, settingsObj.VersionsEtag)
	if err != nil {
		return nil, errors.New("failed to download switch updates [reason:" + err.Error() + "]")
	}
	settingsObj.VersionsEtag = versionsETag
	if err := settings.SaveSettingsWithError(settingsObj, a.baseFolder); err != nil {
		return nil, fmt.Errorf("save title database settings: %w", err)
	}

	a.updateProgress(3, 4, "Processing switch titles and updates ...")
	switchTitleDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)
	a.updateProgress(4, 4, "Finishing up...")
	return switchTitleDB, err
}

func (a *App) buildLocalDB(ignoreCache bool) (*db.LocalSwitchFilesDB, error) {
	settingsObj := settings.ReadSettings(a.baseFolder)
	scanFolders := append([]string{}, settingsObj.ScanFolders...)
	scanFolders = append(scanFolders, settingsObj.Folder)
	localDB, err := a.localDbManager.CreateLocalSwitchFilesDB(
		scanFolders,
		progressReporter{app: a},
		settingsObj.ScanRecursively,
		ignoreCache,
	)
	a.state.localDB = localDB
	return localDB, err
}
