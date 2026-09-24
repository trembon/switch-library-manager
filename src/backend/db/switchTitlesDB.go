package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type TitleAttributes struct {
	Id                string      `json:"id"`
	Name              string      `json:"name,omitempty"`
	Version           json.Number `json:"version,omitempty"`
	Region            string      `json:"region,omitempty"`
	ReleaseDate       int         `json:"releaseDate,omitempty"`
	ParsedReleaseDate string
	Publisher         string   `json:"publisher,omitempty"`
	IconUrl           string   `json:"iconUrl,omitempty"`
	Screenshots       []string `json:"screenshots,omitempty"`
	BannerUrl         string   `json:"bannerUrl,omitempty"`
	Description       string   `json:"description,omitempty"`
	Size              int      `json:"size,omitempty"`
	IsDemo            bool     `json:"isDemo,omitempty"`
}

type SwitchTitle struct {
	Attributes TitleAttributes
	Updates    map[int]string
	Dlc        map[string]TitleAttributes
}

type SwitchTitlesDB struct {
	TitlesMap map[string]*SwitchTitle
}

func decodeToJsonObject(reader io.Reader, target interface{}) error {
	return json.NewDecoder(reader).Decode(target)
}

// ValidateTitlesJSON rejects malformed or empty title data before a download
// can replace the last usable local copy.
func ValidateTitlesJSON(data []byte) error {
	var titles map[string]TitleAttributes
	if err := json.Unmarshal(data, &titles); err != nil {
		return fmt.Errorf("decode titles JSON: %w", err)
	}
	if len(titles) == 0 {
		return errors.New("titles JSON must contain at least one title")
	}
	return nil
}

// ValidateVersionsJSON rejects malformed or empty version data before a
// download can replace the last usable local copy.
func ValidateVersionsJSON(data []byte) error {
	var versions map[string]map[int]string
	if err := json.Unmarshal(data, &versions); err != nil {
		return fmt.Errorf("decode versions JSON: %w", err)
	}
	if len(versions) == 0 {
		return errors.New("versions JSON must contain at least one title")
	}
	for id, titleVersions := range versions {
		for version := range titleVersions {
			if version < 0 {
				return fmt.Errorf("version %d for title %q must not be negative", version, id)
			}
		}
	}
	return nil
}

func CreateSwitchTitleDB(titlesFile, versionsFile io.Reader) (*SwitchTitlesDB, error) {
	//parse the titles objects
	var titles = map[string]TitleAttributes{}
	err := decodeToJsonObject(titlesFile, &titles)
	if err != nil {
		return nil, err
	}

	//parse the titles objects
	//titleID -> versionId-> release date
	var versions = map[string]map[int]string{}
	err = decodeToJsonObject(versionsFile, &versions)
	if err != nil {
		return nil, err
	}

	result := SwitchTitlesDB{TitlesMap: map[string]*SwitchTitle{}}
	for id, attr := range titles {
		id = strings.ToLower(id)
		idPrefix, err := titleIDPrefix(id)
		if err != nil {
			zap.S().Warnf("skipping unsupported title ID %q: %v", id, err)
			continue
		}

		//TitleAttributes id rules:
		//main TitleAttributes ends with 000
		//Updates ends with 800
		//Dlc adds 1 to 4th char starting from the right (always odd) and
		//have a running counter (starting with 001) in the 3 last chars
		switchTitle := &SwitchTitle{Dlc: map[string]TitleAttributes{}}

		if t, ok := result.TitlesMap[idPrefix]; ok {
			switchTitle = t
		}
		result.TitlesMap[idPrefix] = switchTitle

		// parse the release date to a date string
		prd := strconv.Itoa(attr.ReleaseDate)
		if len(prd) == 8 {
			attr.ParsedReleaseDate = prd[0:4] + "-" + prd[4:6] + "-" + prd[6:8]
		} else {
			attr.ParsedReleaseDate = prd
		}

		//process Updates
		if strings.HasSuffix(id, "800") {
			updates := versions[id[0:len(id)-3]+"000"]
			switchTitle.Updates = updates
			continue
		}

		//process main TitleAttributes
		if strings.HasSuffix(id, "000") {
			switchTitle.Attributes = attr
			continue
		}

		//not an update, and not main TitleAttributes, so treat it as a DLC
		switchTitle.Dlc[id] = attr

	}

	return &result, nil
}

// titleIDPrefix returns the normalized title group key used by remote and local data.
func titleIDPrefix(id string) (string, error) {
	id = strings.ToLower(id)
	if len(id) != 16 {
		return "", errors.New("title ID must contain 16 hexadecimal characters")
	}
	if _, err := strconv.ParseUint(id, 16, 64); err != nil {
		return "", errors.New("title ID must contain 16 hexadecimal characters")
	}
	if strings.HasSuffix(id, "000") || strings.HasSuffix(id, "800") {
		return id[:len(id)-3], nil
	}
	value, _ := strconv.ParseUint(id[len(id)-4:len(id)-3], 16, 4)
	if value == 0 {
		return "", errors.New("DLC title ID has an invalid group nibble")
	}
	return id[:len(id)-4] + strconv.FormatUint(value-1, 16), nil
}
