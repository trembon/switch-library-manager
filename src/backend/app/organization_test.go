package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"go.uber.org/zap"
)

func TestOrganizeLibraryConflictCancelsCleanupAndReportsIssue(t *testing.T) {
	base := t.TempDir()
	library := filepath.Join(base, "library")
	if err := os.MkdirAll(library, 0755); err != nil {
		t.Fatal(err)
	}
	baseID := "0100000000010000"
	primaryPath := filepath.Join(library, "A Base ["+baseID+"][v0].nsp")
	duplicatePath := filepath.Join(library, "Game ["+baseID+"][v0].nsp")
	if err := os.WriteFile(primaryPath, []byte("primary base"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(duplicatePath, []byte("duplicate base"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveSettingsWithError(&settings.AppSettings{
		Paths: settings.PathSettings{LibraryFolder: library, ScanFolders: []string{}},
		Scan:  settings.ScanSettings{IgnoreFileTypes: []string{}},
		Organization: settings.OrganizeOptions{
			RenameFiles:          true,
			FileNameTemplate:     "{TITLE_NAME} [{TITLE_ID}][v0]",
			DeleteOldUpdateFiles: true,
		},
	}, base); err != nil {
		t.Fatal(err)
	}
	manager, err := db.NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	application := &App{
		baseFolder:     base,
		localDbManager: manager,
		sugarLogger:    zap.NewNop().Sugar(),
		state: State{
			switchDB: &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
				"0100000000010": {Attributes: db.TitleAttributes{Id: baseID, Name: "Game"}},
			}},
			localDB: &db.LocalSwitchFilesDB{},
		},
	}

	result, err := application.OrganizeLibrary()
	if err != nil {
		t.Fatalf("OrganizeLibrary() returned unexpected error: %v", err)
	}
	if result.Status != "blocked" {
		t.Fatalf("OrganizeLibrary() status = %q, want blocked", result.Status)
	}
	if result.ConflictCount == 0 {
		t.Fatal("OrganizeLibrary() returned no destination conflicts")
	}
	if len(result.Library.LibraryData) == 0 {
		t.Fatal("OrganizeLibrary() did not return the scanned library with its blocked result")
	}
	for path, want := range map[string]string{primaryPath: "primary base", duplicatePath: "duplicate base"} {
		content, err := os.ReadFile(path)
		if err != nil || string(content) != want {
			t.Fatalf("file %q changed despite preflight conflict: content=%q err=%v", path, content, err)
		}
	}
	if len(application.state.organizationIssues) == 0 {
		t.Fatal("organization conflict was not retained for the Issues view")
	}
	foundConflict := false
	for _, issue := range result.Library.Issues {
		if strings.Contains(issue.Value, "organization conflict") && strings.Contains(issue.Value, duplicatePath) {
			foundConflict = true
			break
		}
	}
	if !foundConflict {
		t.Fatalf("blocked result does not include the destination conflict: %#v", result.Library.Issues)
	}
}

func TestOrganizeLibraryCompletedReturnsCurrentLibrary(t *testing.T) {
	base := t.TempDir()
	library := filepath.Join(base, "library")
	if err := os.MkdirAll(library, 0755); err != nil {
		t.Fatal(err)
	}
	baseID := "0100000000010000"
	gamePath := filepath.Join(library, "Game ["+baseID+"][v0].nsp")
	if err := os.WriteFile(gamePath, []byte("synthetic game file"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveSettingsWithError(&settings.AppSettings{
		Paths: settings.PathSettings{LibraryFolder: library, ScanFolders: []string{}},
		Scan:  settings.ScanSettings{IgnoreFileTypes: []string{}},
		Organization: settings.OrganizeOptions{
			RenameFiles:      true,
			FileNameTemplate: "{TITLE_NAME} [{TITLE_ID}][v0]",
		},
	}, base); err != nil {
		t.Fatal(err)
	}
	manager, err := db.NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	application := &App{
		baseFolder:     base,
		localDbManager: manager,
		sugarLogger:    zap.NewNop().Sugar(),
		state: State{
			switchDB: &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
				"0100000000010": {Attributes: db.TitleAttributes{Id: baseID, Name: "Game"}},
			}},
			localDB: &db.LocalSwitchFilesDB{},
		},
	}

	result, err := application.OrganizeLibrary()
	if err != nil {
		t.Fatalf("OrganizeLibrary() returned unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("OrganizeLibrary() status = %q, want completed", result.Status)
	}
	if result.ConflictCount != 0 {
		t.Fatalf("OrganizeLibrary() conflict count = %d, want 0", result.ConflictCount)
	}
	if len(result.Library.LibraryData) != 1 || result.Library.LibraryData[0].TitleId != baseID {
		t.Fatalf("OrganizeLibrary() library data = %#v, want current game record", result.Library.LibraryData)
	}
}

func TestOrganizeLibraryImportsScanFolderWithNoOtherOrganizationOptions(t *testing.T) {
	base := t.TempDir()
	library := filepath.Join(base, "library")
	incoming := filepath.Join(base, "drop")
	for _, directory := range []string{library, incoming} {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
	}
	baseID := "0100000000010000"
	source := filepath.Join(incoming, "Original Name ["+baseID+"][v0].nsp")
	if err := os.WriteFile(source, []byte("synthetic Switch file"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveSettingsWithError(&settings.AppSettings{
		Paths:        settings.PathSettings{LibraryFolder: library, ScanFolders: []string{incoming}},
		Scan:         settings.ScanSettings{Recursive: true, IgnoreFileTypes: []string{}},
		Organization: settings.OrganizeOptions{MoveScanFilesToLibrary: true},
	}, base); err != nil {
		t.Fatal(err)
	}
	manager, err := db.NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	application := &App{
		baseFolder:     base,
		localDbManager: manager,
		sugarLogger:    zap.NewNop().Sugar(),
		state: State{
			switchDB: &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
				"0100000000010": {Attributes: db.TitleAttributes{Id: baseID, Name: "Original Name"}},
			}},
			localDB: &db.LocalSwitchFilesDB{},
		},
	}

	result, err := application.OrganizeLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" {
		t.Fatalf("OrganizeLibrary() status = %q, want completed", result.Status)
	}
	destination := filepath.Join(library, filepath.Base(source))
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source file still exists after importing: %v", err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "synthetic Switch file" {
		t.Fatalf("imported file content = %q, err = %v", got, err)
	}
}

func TestOrganizeLibraryInvalidOptionsRemainErrors(t *testing.T) {
	base := t.TempDir()
	settingsObj := &settings.AppSettings{
		Paths: settings.PathSettings{LibraryFolder: filepath.Join(base, "library"), ScanFolders: []string{}},
		Scan:  settings.ScanSettings{IgnoreFileTypes: []string{}},
		Organization: settings.OrganizeOptions{
			RenameFiles:      true,
			FileNameTemplate: "{TITLE_NAME}",
		},
	}
	if err := settings.SaveSettingsWithError(settingsObj, base); err != nil {
		t.Fatal(err)
	}
	settingsObj.Organization.FileNameTemplate = "{VERSION}"
	defer func() {
		settingsObj.Organization.FileNameTemplate = "{TITLE_NAME}"
		if err := settings.SaveSettingsWithError(settingsObj, base); err != nil {
			t.Errorf("restore valid organization options: %v", err)
		}
	}()
	manager, err := db.NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	application := &App{
		baseFolder:     base,
		localDbManager: manager,
		sugarLogger:    zap.NewNop().Sugar(),
		state: State{
			switchDB: &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}},
			localDB:  &db.LocalSwitchFilesDB{},
		},
	}

	result, err := application.OrganizeLibrary()
	if err == nil {
		t.Fatal("OrganizeLibrary() returned nil error for invalid organization options")
	}
	if result.Status != "" || result.ConflictCount != 0 || result.Library.LibraryData != nil || result.Library.Issues != nil {
		t.Fatalf("failed OrganizeLibrary() returned partial result %#v", result)
	}
}
