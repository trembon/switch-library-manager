package app

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestBuildLocalLibraryDataInitializesListsForEmptyResults(t *testing.T) {
	tests := []struct {
		name       string
		localDB    *db.LocalSwitchFilesDB
		wantRows   int
		wantIssues int
	}{
		{
			name: "empty library",
			localDB: &db.LocalSwitchFilesDB{
				TitlesMap: map[string]*db.SwitchGameFiles{},
				Skipped:   map[db.ExtendedFileInfo]db.SkippedFile{},
			},
		},
		{
			name: "skipped files only",
			localDB: &db.LocalSwitchFilesDB{
				TitlesMap: map[string]*db.SwitchGameFiles{},
				Skipped: map[db.ExtendedFileInfo]db.SkippedFile{
					{FileName: "notes.txt"}: {ReasonText: "file type is not supported"},
				},
				NumFiles: 1,
			},
			wantIssues: 1,
		},
		{
			name: "updates and DLC without a base game",
			localDB: &db.LocalSwitchFilesDB{
				TitlesMap: map[string]*db.SwitchGameFiles{
					"0100000000010": {
						Updates: map[int]db.SwitchFileInfo{
							2: {ExtendedInfo: db.ExtendedFileInfo{FileName: "Game Update.nsp"}},
						},
						Dlc: map[string]db.SwitchFileInfo{
							"0100000000010001": {ExtendedInfo: db.ExtendedFileInfo{FileName: "Game DLC.nsp"}},
						},
					},
				},
				Skipped: map[db.ExtendedFileInfo]db.SkippedFile{},
			},
			wantIssues: 2,
		},
		{
			name: "populated library without issues",
			localDB: &db.LocalSwitchFilesDB{
				TitlesMap: map[string]*db.SwitchGameFiles{
					"0100000000010": {
						BaseExist: true,
						File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "Game.nsp"}},
					},
				},
				Skipped:  map[db.ExtendedFileInfo]db.SkippedFile{},
				NumFiles: 1,
			},
			wantRows: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			switchDB := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}}
			if test.name == "populated library without issues" {
				switchDB.TitlesMap["0100000000010"] = &db.SwitchTitle{Attributes: db.TitleAttributes{Id: "0100000000010000"}}
			}
			got := buildLocalLibraryData(test.localDB, switchDB, false)
			if got.LibraryData == nil || len(got.LibraryData) != test.wantRows {
				t.Errorf("library data = %#v; want non-nil slice with %d rows", got.LibraryData, test.wantRows)
			}
			if got.Issues == nil || len(got.Issues) != test.wantIssues {
				t.Errorf("issues = %#v; want non-nil slice with %d rows", got.Issues, test.wantIssues)
			}
		})
	}

	empty := buildLocalLibraryData(
		&db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}},
		&db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}},
		false,
	)
	encoded, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"library_data":[],"issues":[],"num_files":0}`; string(encoded) != want {
		t.Fatalf("empty library JSON = %s, want %s", encoded, want)
	}
}

func TestBuildLocalLibraryDataPreservesIssueEntries(t *testing.T) {
	localDB := &db.LocalSwitchFilesDB{
		TitlesMap: map[string]*db.SwitchGameFiles{},
		Skipped: map[db.ExtendedFileInfo]db.SkippedFile{
			{FileName: "notes.txt"}: {ReasonText: "file type is not supported"},
		},
	}
	got := buildLocalLibraryData(localDB, &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}}, false)
	want := []Pair{{Key: "notes.txt", Value: "file type is not supported"}}
	if !reflect.DeepEqual(got.Issues, want) {
		t.Fatalf("issues = %#v, want %#v", got.Issues, want)
	}
}

func TestBuildLocalLibraryDataUsesSharedTitleResolver(t *testing.T) {
	const key = "010044901c5c2"
	game := &db.SwitchGameFiles{
		BaseExist: true,
		File: db.SwitchFileInfo{
			ExtendedInfo: db.ExtendedFileInfo{FileName: "test.nsp"},
			Metadata: &switchfs.ContentMetaAttributes{Ncap: &switchfs.Nacp{TitleName: map[string]switchfs.NacpTitle{
				"BritishEnglish": {Title: "British Game Name"},
			}}},
		},
	}
	remote := &db.SwitchTitle{Attributes: db.TitleAttributes{Id: "010044901C5C2000", Name: "カプコン ファイティング コレクション2"}}
	localDB := &db.LocalSwitchFilesDB{
		TitlesMap: map[string]*db.SwitchGameFiles{key: game},
		Skipped:   map[db.ExtendedFileInfo]db.SkippedFile{},
	}
	switchDB := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{key: remote}}

	got := buildLocalLibraryData(localDB, switchDB, false)
	if len(got.LibraryData) != 1 || got.LibraryData[0].Name != "British Game Name" {
		t.Fatalf("Library title = %#v, want embedded British English name", got.LibraryData)
	}
	if remote.Attributes.Name != "カプコン ファイティング コレクション2" {
		t.Fatalf("resolver mutated catalog title: %q", remote.Attributes.Name)
	}

	game.File.Metadata = nil
	got = buildLocalLibraryData(localDB, switchDB, false)
	if len(got.LibraryData) != 1 || got.LibraryData[0].Name != "test" {
		t.Fatalf("Library fallback title = %#v, want filename title %q", got.LibraryData, "test")
	}
}

func TestBuildLocalLibraryDataHandlesMissingMetadata(t *testing.T) {
	const key = "010044901c5c2"
	localDB := &db.LocalSwitchFilesDB{
		TitlesMap: map[string]*db.SwitchGameFiles{
			key: {
				BaseExist: true,
				File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "test.nsp"}},
			},
		},
		Skipped: map[db.ExtendedFileInfo]db.SkippedFile{},
	}

	got := buildLocalLibraryData(localDB, nil, false)
	if len(got.LibraryData) != 1 || got.LibraryData[0].Name != "test" || got.LibraryData[0].TitleId != "" {
		t.Fatalf("Library data = %#v, want filename fallback with no title ID", got.LibraryData)
	}
}

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

			library := buildLocalLibraryData(localDB, &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}}, false)
			if len(library.LibraryData) != 1 {
				t.Fatalf("library data rows = %d, want 1", len(library.LibraryData))
			}
			if got := library.LibraryData[0].Version; got != test.wantVersion {
				t.Errorf("version = %q, want %q", got, test.wantVersion)
			}
		})
	}
}

func TestBuildLocalLibraryDataAddsCatalogMetadataSizeAndUpdateID(t *testing.T) {
	localDB := &db.LocalSwitchFilesDB{
		TitlesMap: map[string]*db.SwitchGameFiles{
			"0100000000010": {
				BaseExist:    true,
				LatestUpdate: 65536,
				File:         db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "Game.nsp", Size: 4096}},
			},
			"0100000000020": {
				BaseExist: true,
				IsSplit:   true,
				File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "Split", Size: 4096, IsDir: true}},
			},
		},
		Skipped: map[db.ExtendedFileInfo]db.SkippedFile{},
	}
	switchDB := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100000000010": {Attributes: db.TitleAttributes{
			Id: "0100000000010000", Name: "Game", Publisher: "Studio", ReleaseDate: 20240102, ParsedReleaseDate: "2024-01-02",
			Category: db.FlexibleStrings{"Puzzle"}, Languages: db.FlexibleStrings{"en", "ja"},
			NumberOfPlayers: 2, Rating: 12, RatingContent: db.FlexibleStrings{"Mild Violence"},
		}},
		"0100000000020": {Attributes: db.TitleAttributes{Id: "0100000000020000", Name: "Split", ParsedReleaseDate: "0"}},
	}}

	rows := map[string]LibraryTemplateData{}
	for _, row := range buildLocalLibraryData(localDB, switchDB, false).LibraryData {
		rows[row.Name] = row
	}

	game := rows["Game"]
	wantMetadata := TitleMetadata{Publisher: "Studio", Genres: []string{"Puzzle"}, Languages: []string{"en", "ja"}, Players: 2, AgeRating: 12, RatingContent: []string{"Mild Violence"}}
	if !reflect.DeepEqual(game.TitleMetadata, wantMetadata) || game.ReleaseDate != "2024-01-02" || game.FileSize != 4096 || game.UpdateID != "0100000000010800" {
		t.Fatalf("Game row = %#v", game)
	}
	split := rows["Split"]
	if split.FileSize != 0 || split.UpdateID != "" || split.ReleaseDate != "" {
		t.Fatalf("split row should have no size, update ID or placeholder release date: %#v", split)
	}
}

func TestLibraryRowHelpersRejectUnknownValues(t *testing.T) {
	if got := baseFileSize(nil); got != 0 {
		t.Fatalf("baseFileSize(nil) = %d", got)
	}
	if got := baseFileSize(&db.SwitchGameFiles{File: db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{Size: -1}}}); got != 0 {
		t.Fatalf("negative size = %d, want 0", got)
	}
	if got := updateTitleID("short", 1); got != "" {
		t.Fatalf("updateTitleID for a malformed ID = %q", got)
	}
	if got := updateTitleID("0100000000010000", 0); got != "" {
		t.Fatalf("updateTitleID without an update = %q", got)
	}
}
