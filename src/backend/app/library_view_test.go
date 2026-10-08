package app

import (
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestBuildLocalLibraryDataFallsBackToBaseDisplayVersion(t *testing.T) {
	tests := []struct {
		name         string
		latestUpdate int
		updates      map[int]db.SwitchFileInfo
		wantVersion  string
	}{
		{
			name:        "base only",
			wantVersion: "1.0",
		},
		{
			name:         "latest update missing",
			latestUpdate: 2,
			wantVersion:  "1.0",
		},
		{
			name:         "latest update without metadata",
			latestUpdate: 2,
			updates:      map[int]db.SwitchFileInfo{2: {}},
			wantVersion:  "1.0",
		},
		{
			name:         "latest update without NACP",
			latestUpdate: 2,
			updates: map[int]db.SwitchFileInfo{
				2: {Metadata: &switchfs.ContentMetaAttributes{}},
			},
			wantVersion: "1.0",
		},
		{
			name:         "latest update with empty display version",
			latestUpdate: 2,
			updates: map[int]db.SwitchFileInfo{
				2: {Metadata: &switchfs.ContentMetaAttributes{Ncap: &switchfs.Nacp{}}},
			},
			wantVersion: "1.0",
		},
		{
			name:         "latest update with display version",
			latestUpdate: 2,
			updates: map[int]db.SwitchFileInfo{
				2: {Metadata: &switchfs.ContentMetaAttributes{Ncap: &switchfs.Nacp{DisplayVersion: "2.0"}}},
			},
			wantVersion: "2.0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			localDB := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
				"0100000000010": {
					BaseExist:    true,
					LatestUpdate: test.latestUpdate,
					Updates:      test.updates,
					File: db.SwitchFileInfo{
						ExtendedInfo: db.ExtendedFileInfo{FileName: "Game.nsp"},
						Metadata: &switchfs.ContentMetaAttributes{TitleId: "0100000000010000", Ncap: &switchfs.Nacp{
							DisplayVersion: "1.0",
							TitleName:      map[string]switchfs.NacpTitle{},
						}},
					},
				},
			}}

			library := buildLocalLibraryData(localDB, &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}})
			if len(library.LibraryData) != 1 {
				t.Fatalf("library data rows = %d, want 1", len(library.LibraryData))
			}
			if got := library.LibraryData[0].Version; got != test.wantVersion {
				t.Errorf("version = %q, want %q", got, test.wantVersion)
			}
		})
	}
}
