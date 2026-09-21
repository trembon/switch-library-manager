package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"go.uber.org/zap"
)

func TestGetLibraryTitleNameFallsBackForEmptyRemoteName(t *testing.T) {
	title := &db.SwitchTitle{Attributes: db.TitleAttributes{Name: ""}}
	if got := getLibraryTitleName(title, "", "Golf Story [01005EA00A57E000][v0].nsp"); got != "Golf Story " {
		t.Fatalf("title name = %q, want %q", got, "Golf Story ")
	}
}

func TestGetLibraryTitleNamePreservesNamePrecedence(t *testing.T) {
	title := &db.SwitchTitle{Attributes: db.TitleAttributes{Name: "Remote Name"}}
	if got := getLibraryTitleName(title, "NACP Name", "File Name [0100000000010000][v1].nsp"); got != "Remote Name" {
		t.Fatalf("remote title name = %q, want %q", got, "Remote Name")
	}

	emptyTitle := &db.SwitchTitle{}
	if got := getLibraryTitleName(emptyTitle, "NACP Name", "File Name [0100000000010000][v1].nsp"); got != "NACP Name" {
		t.Fatalf("NACP title name = %q, want %q", got, "NACP Name")
	}
}

func TestNormalizeSettingsInitializesFrontendLists(t *testing.T) {
	value := normalizeSettings(settings.AppSettings{})

	if value.Paths.ScanFolders == nil || value.MissingContent.IgnoreDLCTitleIDs == nil || value.MissingContent.IgnoreUpdateIDs == nil || value.Scan.IgnoreFileTypes == nil {
		t.Fatal("expected settings lists to be initialized")
	}
}

func TestSettingsMigrationMessageIncludesBackupAndDefaultInstructions(t *testing.T) {
	message := settingsMigrationMessage(&settings.MigrationInfo{BackupPath: "C:\\app\\settings.old.json"})
	if !strings.Contains(message, "C:\\app\\settings.old.json") || !strings.Contains(message, "current default values") || !strings.Contains(message, "docs/settings.md") {
		t.Fatalf("migration message = %q", message)
	}
	if strings.Contains(message, "v1") || strings.Contains(message, "v2") {
		t.Fatalf("migration message contains schema-specific wording: %q", message)
	}
}

func TestFrontendModelsPreserveJSONContract(t *testing.T) {
	data, err := json.Marshal(LocalLibraryData{
		LibraryData: []LibraryTemplateData{{TitleId: "0100000000001000"}},
		Issues:      []Pair{{Key: "file.nsp", Value: "issue"}},
		NumFiles:    1,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `{"library_data":[{"id":0,"name":"","version":"","dlc":"","titleId":"0100000000001000","path":"","icon":"","update":0,"region":"","type":""}],"issues":[{"key":"file.nsp","value":"issue"}],"num_files":1}`
	if string(data) != want {
		t.Fatalf("JSON = %s, want %s", data, want)
	}
}

func TestRescanLibraryHardModeRebuildsLocalScan(t *testing.T) {
	baseFolder := t.TempDir()
	libraryFolder := filepath.Join(baseFolder, "library")
	if err := os.Mkdir(libraryFolder, 0755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(libraryFolder, "First Game [0100000000001000][v0].nsp")
	if err := os.WriteFile(first, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveSettingsWithError(&settings.AppSettings{
		Paths: settings.PathSettings{LibraryFolder: libraryFolder, ScanFolders: []string{}},
		Scan:  settings.ScanSettings{IgnoreFileTypes: []string{}},
	}, baseFolder); err != nil {
		t.Fatal(err)
	}

	localManager, err := db.NewLocalSwitchDBManager(baseFolder)
	if err != nil {
		t.Fatal(err)
	}
	defer localManager.Close()
	application := &App{
		baseFolder:     baseFolder,
		localDbManager: localManager,
		sugarLogger:    zap.NewNop().Sugar(),
		state:          State{switchDB: &db.SwitchTitlesDB{}},
	}

	firstScan, err := application.RescanLibrary(false)
	if err != nil {
		t.Fatal(err)
	}
	if firstScan.NumFiles != 1 {
		t.Fatalf("first scan files = %d, want 1", firstScan.NumFiles)
	}

	second := filepath.Join(libraryFolder, "Second Game [0100000000002000][v0].nsp")
	if err := os.WriteFile(second, []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}
	normalScan, err := application.RescanLibrary(false)
	if err != nil {
		t.Fatal(err)
	}
	if normalScan.NumFiles != 1 {
		t.Fatalf("normal rescan files = %d, want cached 1", normalScan.NumFiles)
	}

	hardScan, err := application.RescanLibrary(true)
	if err != nil {
		t.Fatal(err)
	}
	if hardScan.NumFiles != 2 {
		t.Fatalf("hard rescan files = %d, want 2", hardScan.NumFiles)
	}
}
