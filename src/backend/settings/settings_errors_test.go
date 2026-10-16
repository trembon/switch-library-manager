package settings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveSettingsRejectsInvalidValuesWithoutReplacingCurrentSettings(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*AppSettings) *AppSettings
		want string
	}{
		{"nil", func(*AppSettings) *AppSettings { return nil }, "settings are nil"},
		{"unsupported schema", func(s *AppSettings) *AppSettings { s.SchemaVersion++; return s }, "unsupported settings schema_version"},
		{"versions URL", func(s *AppSettings) *AppSettings { s.DataSources.VersionsURL = "/versions.json"; return s }, "data_sources.versions_url"},
		{"nonhexadecimal ID", func(s *AppSettings) *AppSettings {
			s.MissingContent.IgnoreDLCTitleIDs = []string{"010000000000000g"}
			return s
		}, "only hexadecimal characters"},
		{"folder template", func(s *AppSettings) *AppSettings {
			s.Organization.CreateFolderPerGame = true
			s.Organization.FolderNameTemplate = "{VERSION}"
			return s
		}, "folder_name_template"},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateSettings(t)
			base := t.TempDir()
			current := defaultSettings()
			if err := SaveSettingsWithError(current, base); err != nil {
				t.Fatal(err)
			}
			filename := filepath.Join(base, SETTINGS_FILENAME)
			before, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			if err := SaveSettingsWithError(test.edit(defaultSettings()), base); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("save error = %v, want %q", err, test.want)
			}
			after, err := os.ReadFile(filename)
			if err != nil || string(after) != string(before) {
				t.Fatalf("invalid save changed settings file: %q, %v", after, err)
			}
			if settingsInstance != current {
				t.Fatal("invalid save replaced current settings")
			}
		})
	}
}

func TestSaveSettingsAllowsEmptyIgnoreIDs(t *testing.T) {
	isolateSettings(t)
	options := defaultSettings()
	options.MissingContent.IgnoreDLCTitleIDs = []string{"", "0100ABCDEF000001"}
	options.MissingContent.IgnoreUpdateIDs = []string{"", "0100abcdef000800"}
	base := t.TempDir()
	if err := SaveSettingsWithError(options, base); err != nil {
		t.Fatal(err)
	}
	settingsInstance = nil
	loaded, err := ReadSettings(base)
	if err != nil || strings.Join(loaded.MissingContent.IgnoreDLCTitleIDs, ",") != ",0100ABCDEF000001" || strings.Join(loaded.MissingContent.IgnoreUpdateIDs, ",") != ",0100abcdef000800" {
		t.Fatalf("ignore IDs did not round-trip: %#v, %v", loaded, err)
	}
}

func TestPrepareSettingsReportsFileAndFieldErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{"invalid schema type", `{"schema_version":"two"}`, "decode settings"},
		{"invalid field type", `{"schema_version":2,"gui":{"page_size":"many"}}`, "decode settings"},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateSettings(t)
			base := t.TempDir()
			filename := filepath.Join(base, SETTINGS_FILENAME)
			if err := os.WriteFile(filename, []byte(test.body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadSettingsAsJSON(base); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load error = %v, want %q", err, test.want)
			}
			if settingsInstance != nil {
				t.Fatal("failed load initialized settings")
			}
			after, err := os.ReadFile(filename)
			if err != nil || string(after) != test.body {
				t.Fatalf("failed load changed settings: %q, %v", after, err)
			}
		})
	}
	for _, name := range []string{"missing parent", "invalid path", "directory"} {
		t.Run(name, func(t *testing.T) {
			isolateSettings(t)
			base := t.TempDir()
			switch name {
			case "missing parent":
				base = filepath.Join(base, "missing")
			case "invalid path":
				base = filepath.Join(base, "invalid\x00")
			case "directory":
				if err := os.Mkdir(filepath.Join(base, SETTINGS_FILENAME), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := PrepareSettings(base); err == nil || !strings.Contains(err.Error(), "settings") {
				t.Fatalf("load error = %v, want settings filesystem error", err)
			}
			if settingsInstance != nil {
				t.Fatal("failed load initialized settings")
			}
		})
	}
}

func TestSettingsWriteFailurePreservesCurrentSettings(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	current := defaultSettings()
	if err := SaveSettingsWithError(current, base); err != nil {
		t.Fatal(err)
	}
	if err := SaveSettingsWithError(defaultSettings(), filepath.Join(base, "missing")); err == nil || !strings.Contains(err.Error(), "create temporary settings file") {
		t.Fatalf("save error = %v, want temporary file creation error", err)
	}
	if settingsInstance != current {
		t.Fatal("failed write replaced current settings")
	}
}

func TestSettingsReplacementFailureCleansTemporaryFile(t *testing.T) {
	base := t.TempDir()
	filename := filepath.Join(base, SETTINGS_FILENAME)
	if err := os.Mkdir(filename, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(filename, "keep.txt")
	if err := os.WriteFile(marker, []byte("retain"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeSettingsFile(filename, []byte(`{"schema_version":2}`)); err == nil || !strings.Contains(err.Error(), "replace settings file") {
		t.Fatalf("replacement error = %v", err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "retain" {
		t.Fatalf("failed replacement changed destination: %q, %v", data, err)
	}
	assertNoTemporarySettingsFiles(t, base)
}

func TestSettingsBackupFailuresRetainOriginalFiles(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "missing.json")
	if _, err := preserveOldSettings(missing); err == nil || !strings.Contains(err.Error(), "preserve older settings") {
		t.Fatalf("backup error = %v", err)
	}
	if _, err := preserveOldSettings(filepath.Join(base, "invalid\x00", SETTINGS_FILENAME)); err == nil || !strings.Contains(err.Error(), "check older settings backup") {
		t.Fatalf("backup inspection error = %v", err)
	}
	backup := filepath.Join(base, "settings.old.json")
	if err := os.WriteFile(backup, []byte("original settings"), 0600); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(base, SETTINGS_FILENAME)
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "keep.txt"), []byte("retain"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := restoreOldSettings(blocked, backup); err == nil || !strings.Contains(err.Error(), "remove incomplete settings file") {
		t.Fatalf("restore error = %v", err)
	}
	if err := restoreOldSettings(filepath.Join(base, "invalid\x00"), backup); err == nil || !strings.Contains(err.Error(), "check incomplete settings file") {
		t.Fatalf("restore inspection error = %v", err)
	}
	if err := restoreOldSettings(missing, filepath.Join(base, "missing-backup.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing backup error = %v", err)
	}
	if data, err := os.ReadFile(backup); err != nil || string(data) != "original settings" {
		t.Fatalf("failed restore changed backup: %q, %v", data, err)
	}
}

func TestCacheReportsReadAndWriteFailures(t *testing.T) {
	base := t.TempDir()
	if err := SaveCacheWithError(nil, base); err == nil || !strings.Contains(err.Error(), "cache is nil") {
		t.Fatalf("nil cache error = %v", err)
	}
	if err := SaveCacheWithError(&Cache{}, filepath.Join(base, "missing")); err == nil || !strings.Contains(err.Error(), "write cache") {
		t.Fatalf("write cache error = %v", err)
	}
	if _, err := ReadCache(filepath.Join(base, "invalid\x00")); err == nil || !strings.Contains(err.Error(), "open cache") {
		t.Fatalf("open cache error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(base, CACHE_FILENAME), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCache(base); err == nil || !strings.Contains(err.Error(), "cache") {
		t.Fatalf("directory cache error = %v", err)
	}
}

func TestWindowStateFilesystemFailures(t *testing.T) {
	base := t.TempDir()
	state := &WindowState{Width: 1280, Height: 720}
	if err := SaveWindowState(state, filepath.Join(base, "missing")); err == nil || !strings.Contains(err.Error(), "write window state") {
		t.Fatalf("write window state error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(base, WINDOW_STATE_FILENAME), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadWindowState(base); err == nil || !strings.Contains(err.Error(), "read window state") {
		t.Fatalf("read window state error = %v", err)
	}
	assertNoTemporarySettingsFiles(t, base)
}

func assertNoTemporarySettingsFiles(t *testing.T, base string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(base, ".settings.json.tmp-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary settings files = %v, error = %v", files, err)
	}
}
