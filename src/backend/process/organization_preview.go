package process

import (
	"path/filepath"
	"sort"
	"strconv"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

// OrganizationPreviewEntry describes one resulting path inside the library
// root. The preview intentionally exposes only the content kind and path; it
// does not expose the source file or any local metadata.
type OrganizationPreviewEntry struct {
	Kind string
	Path string
}

// BuildOrganizationPreview returns one example for each supported content
// kind. Entries from the scanned library are preferred, with deterministic
// examples filling any missing kinds.
func BuildOrganizationPreview(baseFolder string, options settings.OrganizeOptions, localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB) []OrganizationPreviewEntry {
	byKind := map[string]OrganizationPreviewEntry{}

	if localDB != nil {
		keys := make([]string, 0, len(localDB.TitlesMap))
		for key := range localDB.TitlesMap {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			game := localDB.TitlesMap[key]
			if game == nil {
				continue
			}
			var title *db.SwitchTitle
			if titlesDB != nil {
				title = titlesDB.TitlesMap[key]
			}
			for _, entry := range organizationPreviewForGame(baseFolder, options, game, title) {
				if _, exists := byKind[entry.Kind]; !exists {
					byKind[entry.Kind] = entry
				}
			}
			if len(byKind) == 3 {
				break
			}
		}
	}

	if len(byKind) < 3 {
		fallback := fallbackOrganizationPreview(baseFolder, options)
		for _, entry := range fallback {
			if _, exists := byKind[entry.Kind]; !exists {
				byKind[entry.Kind] = entry
			}
		}
	}

	result := make([]OrganizationPreviewEntry, 0, 3)
	for _, kind := range []string{"game", "update", "dlc"} {
		if entry, ok := byKind[kind]; ok {
			result = append(result, entry)
		}
	}
	return result
}

func organizationPreviewForGame(baseFolder string, options settings.OrganizeOptions, game *db.SwitchGameFiles, title *db.SwitchTitle) []OrganizationPreviewEntry {
	if !game.BaseExist && !options.ProcessWhenMissingBaseGame {
		return nil
	}

	titleName := getTitleName(title, game)
	templateData := map[string]string{
		settings.TEMPLATE_TITLE_NAME:  titleName,
		settings.TEMPLATE_VERSION_TXT: "",
		settings.TEMPLATE_VERSION:     "0",
	}
	if title != nil {
		templateData[settings.TEMPLATE_REGION] = title.Attributes.Region
		templateData[settings.TEMPLATE_TITLE_ID] = title.Attributes.Id
	}
	if templateData[settings.TEMPLATE_TITLE_ID] == "" && game.File.Metadata != nil {
		templateData[settings.TEMPLATE_TITLE_ID] = game.File.Metadata.TitleId
	}

	destinationPath := game.File.ExtendedInfo.BaseFolder
	if options.CreateFolderPerGame {
		destinationPath = filepath.Join(baseFolder, getFolderName(options, templateData))
	}

	result := make([]OrganizationPreviewEntry, 0, 3)
	if game.BaseExist {
		templateData[settings.TEMPLATE_TYPE] = "BASE"
		if game.File.Metadata != nil && game.File.Metadata.Ncap != nil {
			templateData[settings.TEMPLATE_VERSION_TXT] = game.File.Metadata.Ncap.DisplayVersion
		}
		result = append(result, OrganizationPreviewEntry{
			Kind: "game",
			Path: organizationPreviewPath(baseFolder, organizationTargetPath(
				destinationPath,
				game.File.ExtendedInfo.BaseFolder,
				game.File.ExtendedInfo.FileName,
				options,
				"base",
				templateData,
				0,
			)),
		})
	}

	updateVersions := make([]int, 0, len(game.Updates))
	for version := range game.Updates {
		updateVersions = append(updateVersions, version)
	}
	sort.Ints(updateVersions)
	if len(updateVersions) > 0 {
		version := updateVersions[len(updateVersions)-1]
		update := game.Updates[version]
		if !(game.MultiContent && game.BaseExist && game.File.ExtendedInfo == update.ExtendedInfo) {
			if update.Metadata != nil {
				templateData[settings.TEMPLATE_TITLE_ID] = update.Metadata.TitleId
			}
			templateData[settings.TEMPLATE_VERSION] = strconv.Itoa(version)
			templateData[settings.TEMPLATE_VERSION_TXT] = ""
			if update.Metadata != nil && update.Metadata.Ncap != nil {
				templateData[settings.TEMPLATE_VERSION_TXT] = update.Metadata.Ncap.DisplayVersion
			}
			templateData[settings.TEMPLATE_TYPE] = "UPD"
			result = append(result, OrganizationPreviewEntry{
				Kind: "update",
				Path: organizationPreviewPath(baseFolder, organizationTargetPath(
					destinationPath,
					update.ExtendedInfo.BaseFolder,
					update.ExtendedInfo.FileName,
					options,
					"update",
					templateData,
					0,
				)),
			})
		}
	}

	dlcIDs := make([]string, 0, len(game.Dlc))
	for id := range game.Dlc {
		dlcIDs = append(dlcIDs, id)
	}
	sort.Strings(dlcIDs)
	if len(dlcIDs) > 0 {
		dlc := game.Dlc[dlcIDs[0]]
		if !(game.MultiContent && game.BaseExist && game.File.ExtendedInfo == dlc.ExtendedInfo) {
			templateData[settings.TEMPLATE_VERSION] = "0"
			templateData[settings.TEMPLATE_VERSION_TXT] = ""
			templateData[settings.TEMPLATE_TITLE_ID] = dlcIDs[0]
			if dlc.Metadata != nil {
				templateData[settings.TEMPLATE_VERSION] = strconv.Itoa(dlc.Metadata.Version)
			}
			templateData[settings.TEMPLATE_TYPE] = "DLC"
			templateData[settings.TEMPLATE_DLC_NAME] = getDlcName(title, dlc)
			result = append(result, OrganizationPreviewEntry{
				Kind: "dlc",
				Path: organizationPreviewPath(baseFolder, organizationTargetPath(
					destinationPath,
					dlc.ExtendedInfo.BaseFolder,
					dlc.ExtendedInfo.FileName,
					options,
					"dlc",
					templateData,
					0,
				)),
			})
		}
	}

	return result
}

func fallbackOrganizationPreview(baseFolder string, options settings.OrganizeOptions) []OrganizationPreviewEntry {
	baseID := "0100E95004039000"
	updateID := "0100E95004039800"
	dlcID := "0100E9500403A001"
	sampleFolder := baseFolder
	if sampleFolder == "" {
		sampleFolder = "."
	}

	game := &db.SwitchGameFiles{
		BaseExist: true,
		File: db.SwitchFileInfo{
			ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sampleFolder, FileName: "example-adventure.nsp"},
			Metadata:     previewMetadata(baseID, 0, "1.0.0"),
		},
		Updates: map[int]db.SwitchFileInfo{
			5: {
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sampleFolder, FileName: "example-adventure-update.nsp"},
				Metadata:     previewMetadata(updateID, 5, "5.0.0"),
			},
		},
		Dlc: map[string]db.SwitchFileInfo{
			dlcID: {
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sampleFolder, FileName: "example-adventure-dlc.nsp"},
				Metadata:     previewMetadata(dlcID, 1, "1.0.0"),
			},
		},
	}
	title := &db.SwitchTitle{
		Attributes: db.TitleAttributes{Id: baseID, Name: "Example Adventure", Region: "US"},
		Dlc: map[string]db.TitleAttributes{
			dlcID: {Id: dlcID, Name: "Example Adventure - Expansion Pack"},
		},
	}
	return organizationPreviewForGame(baseFolder, options, game, title)
}

func previewMetadata(id string, version int, displayVersion string) *switchfs.ContentMetaAttributes {
	return &switchfs.ContentMetaAttributes{
		TitleId: id,
		Version: version,
		Ncap:    &switchfs.Nacp{DisplayVersion: displayVersion},
	}
}

func organizationPreviewPath(baseFolder, target string) string {
	if baseFolder == "" || !filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	relative, err := filepath.Rel(baseFolder, target)
	if err != nil {
		return filepath.Clean(target)
	}
	return filepath.Clean(relative)
}
