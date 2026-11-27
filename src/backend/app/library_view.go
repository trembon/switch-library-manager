package app

import (
	"path/filepath"

	"github.com/trembon/switch-library-manager/backend/db"
)

func buildLocalLibraryData(localDB *db.LocalSwitchFilesDB, switchDB *db.SwitchTitlesDB, allowUnicode bool) LocalLibraryData {
	response := LocalLibraryData{
		LibraryData: make([]LibraryTemplateData, 0),
		Issues:      make([]Pair, 0),
	}
	for key, value := range localDB.TitlesMap {
		if value.BaseExist {
			version := ""
			titleID := ""
			if value.File.Metadata != nil {
				titleID = value.File.Metadata.TitleId
			}
			var title *db.SwitchTitle
			if switchDB != nil {
				title = switchDB.TitlesMap[key]
			}
			name := db.ResolveGameTitleName(title, value, allowUnicode)
			if value.File.Metadata != nil && value.File.Metadata.Ncap != nil {
				version = value.File.Metadata.Ncap.DisplayVersion
			}

			if latest, ok := value.Updates[value.LatestUpdate]; ok && latest.Metadata != nil && latest.Metadata.Ncap != nil && latest.Metadata.Ncap.DisplayVersion != "" {
				version = latest.Metadata.Ncap.DisplayVersion
			}

			row := LibraryTemplateData{
				Name:     name,
				Update:   value.LatestUpdate,
				Version:  version,
				Type:     getType(value),
				TitleId:  titleID,
				Path:     filepath.Join(value.File.ExtendedInfo.BaseFolder, value.File.ExtendedInfo.FileName),
				FileSize: baseFileSize(value),
			}
			if title != nil {
				row.Icon = title.Attributes.IconUrl
				row.TitleId = title.Attributes.Id
				row.Region = title.Attributes.Region
				row.ReleaseDate = catalogReleaseDate(title.Attributes)
				row.TitleMetadata = titleMetadata(title.Attributes)
			}
			row.UpdateID = updateTitleID(row.TitleId, value.LatestUpdate)
			response.LibraryData = append(response.LibraryData, row)
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

// titleMetadata copies the optional catalog fields shown as extra columns.
func titleMetadata(attributes db.TitleAttributes) TitleMetadata {
	return TitleMetadata{
		Publisher:     attributes.Publisher,
		Genres:        attributes.Category,
		Languages:     attributes.Languages,
		Players:       int(attributes.NumberOfPlayers),
		AgeRating:     int(attributes.Rating),
		RatingContent: attributes.RatingContent,
	}
}

// catalogReleaseDate returns the parsed catalog release date, or an empty
// string when the catalog has none (it parses a missing date as "0").
func catalogReleaseDate(attributes db.TitleAttributes) string {
	if attributes.ReleaseDate <= 0 {
		return ""
	}
	return attributes.ParsedReleaseDate
}

// baseFileSize reports the base file's size, or zero when a single file size
// would be misleading: split games are stored as several parts.
func baseFileSize(files *db.SwitchGameFiles) int64 {
	if files == nil || files.IsSplit || files.File.ExtendedInfo.IsDir || files.File.ExtendedInfo.Size < 0 {
		return 0
	}
	return files.File.ExtendedInfo.Size
}

// updateTitleID derives the update title ID (ending in 800) for a base title
// that has an installed update.
func updateTitleID(baseTitleID string, latestUpdate int) string {
	if latestUpdate <= 0 || len(baseTitleID) != 16 {
		return ""
	}
	return baseTitleID[:13] + "800"
}
