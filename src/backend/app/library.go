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
	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return nil, err
	}
	cache, err := settings.ReadCache(a.baseFolder)
	if err != nil {
		return nil, err
	}
	a.updateProgress(1, 4, "Downloading titles.json")

	filename := filepath.Join(a.baseFolder, settings.TITLE_JSON_FILENAME)
	titleFile, titlesETag, err := db.LoadAndUpdateFile(settingsObj.DataSources.TitlesURL, filename, cache.TitlesETag)
	if err != nil {
		return nil, errors.New("failed to download switch titles [reason:" + err.Error() + "]")
	}
	cache.TitlesETag = titlesETag

	a.updateProgress(2, 4, "Downloading versions.json")
	filename = filepath.Join(a.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsETag, err := db.LoadAndUpdateFile(settingsObj.DataSources.VersionsURL, filename, cache.VersionsETag)
	if err != nil {
		return nil, errors.New("failed to download switch updates [reason:" + err.Error() + "]")
	}
	cache.VersionsETag = versionsETag
	if err := settings.SaveCacheWithError(cache, a.baseFolder); err != nil {
		return nil, fmt.Errorf("save title database cache: %w", err)
	}

	a.updateProgress(3, 4, "Processing switch titles and updates ...")
	switchTitleDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)
	a.updateProgress(4, 4, "Finishing up...")
	return switchTitleDB, err
}

func (a *App) buildLocalDB(ignoreCache bool) (*db.LocalSwitchFilesDB, error) {
	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return nil, err
	}
	scanFolders := append([]string{}, settingsObj.Paths.ScanFolders...)
	scanFolders = append(scanFolders, settingsObj.Paths.LibraryFolder)
	localDB, err := a.localDbManager.CreateLocalSwitchFilesDB(
		scanFolders,
		progressReporter{app: a},
		settingsObj.Scan.Recursive,
		ignoreCache,
	)
	a.state.localDB = localDB
	return localDB, err
}
