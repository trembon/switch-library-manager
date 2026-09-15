package app

import (
	"path/filepath"

	"github.com/trembon/switch-library-manager/backend/db"
)

func buildLocalLibraryData(localDB *db.LocalSwitchFilesDB, switchDB *db.SwitchTitlesDB) LocalLibraryData {
	response := LocalLibraryData{}
	for key, value := range localDB.TitlesMap {
		if value.BaseExist {
			version := ""
			name := ""
			if value.File.Metadata.Ncap != nil {
				version = value.File.Metadata.Ncap.DisplayVersion
				name = value.File.Metadata.Ncap.TitleName["AmericanEnglish"].Title
			}

			if len(value.Updates) != 0 {
				if value.Updates[value.LatestUpdate].Metadata.Ncap != nil {
					version = value.Updates[value.LatestUpdate].Metadata.Ncap.DisplayVersion
				} else {
					version = ""
				}
			}

			if title, ok := switchDB.TitlesMap[key]; ok {
				name = getLibraryTitleName(title, name, value.File.ExtendedInfo.FileName)
				response.LibraryData = append(response.LibraryData, LibraryTemplateData{
					Icon:    title.Attributes.IconUrl,
					Name:    name,
					TitleId: title.Attributes.Id,
					Update:  value.LatestUpdate,
					Version: version,
					Region:  title.Attributes.Region,
					Type:    getType(value),
					Path:    filepath.Join(value.File.ExtendedInfo.BaseFolder, value.File.ExtendedInfo.FileName),
				})
			} else {
				if name == "" {
					name = db.ParseTitleNameFromFileName(value.File.ExtendedInfo.FileName)
				}
				response.LibraryData = append(response.LibraryData, LibraryTemplateData{
					Name:    name,
					Update:  value.LatestUpdate,
					Version: version,
					Type:    getType(value),
					TitleId: value.File.Metadata.TitleId,
					Path:    filepath.Join(value.File.ExtendedInfo.BaseFolder, value.File.ExtendedInfo.FileName),
				})
			}
		} else {
			for _, update := range value.Updates {
				response.Issues = append(response.Issues, Pair{
					Key:   filepath.Join(update.ExtendedInfo.BaseFolder, update.ExtendedInfo.FileName),
					Value: "base file is missing",
				})
			}
			for _, dlc := range value.Dlc {
				response.Issues = append(response.Issues, Pair{
					Key:   filepath.Join(dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName),
					Value: "base file is missing",
				})
			}
		}
	}
	for key, skipped := range localDB.Skipped {
		response.Issues = append(response.Issues, Pair{
			Key:   filepath.Join(key.BaseFolder, key.FileName),
			Value: skipped.ReasonText,
		})
	}
	response.NumFiles = localDB.NumFiles
	return response
}

func getType(gameFile *db.SwitchGameFiles) string {
	if gameFile.IsSplit {
		return "split"
	}
	if gameFile.MultiContent {
		return "multi-content"
	}
	ext := filepath.Ext(gameFile.File.ExtendedInfo.FileName)
	if len(ext) > 1 {
		return ext[1:]
	}
	return ""
}

func getLibraryTitleName(title *db.SwitchTitle, name, fileName string) string {
	if title != nil && title.Attributes.Name != "" {
		return title.Attributes.Name
	}
	if name != "" {
		return name
	}
	return db.ParseTitleNameFromFileName(fileName)
}
