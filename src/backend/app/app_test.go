package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
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
