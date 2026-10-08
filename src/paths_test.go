package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRuntimeDataFolderUsesUserConfigDirOnMacOS(t *testing.T) {
	configFolder := t.TempDir()
	bundleParent := t.TempDir()
	executablePath := filepath.Join(bundleParent, "Switch Library Manager.app", "Contents", "MacOS", "slm")
	legacySettings := []byte("legacy settings stay here")
	if err := os.WriteFile(filepath.Join(bundleParent, "settings.json"), legacySettings, 0600); err != nil {
		t.Fatal(err)
	}

	configLookupCalled := false
	dataFolder, err := resolveRuntimeDataFolder("darwin", executablePath, func() (string, error) {
		configLookupCalled = true
		return configFolder, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	wantDataFolder := filepath.Join(configFolder, macOSDataDirectoryName)
	if dataFolder != wantDataFolder {
		t.Fatalf("data folder = %q, want %q", dataFolder, wantDataFolder)
	}
	if !configLookupCalled {
		t.Fatal("user config directory lookup was not called")
	}
	if info, err := os.Stat(wantDataFolder); err != nil || !info.IsDir() {
		t.Fatalf("application data folder was not created: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(filepath.Join(wantDataFolder, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy settings were imported into the new profile, stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundleParent, macOSDataDirectoryName)); !os.IsNotExist(err) {
		t.Fatalf("application data folder was created beside the app, stat error = %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(bundleParent, "settings.json")); err != nil || string(got) != string(legacySettings) {
		t.Fatalf("legacy settings changed: contents=%q err=%v", got, err)
	}
}

func TestResolveRuntimeDataFolderKeepsExecutableFolderOnOtherPlatforms(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			executablePath := filepath.Join(t.TempDir(), "application", "slm")
			configLookupCalled := false
			dataFolder, err := resolveRuntimeDataFolder(goos, executablePath, func() (string, error) {
				configLookupCalled = true
				return t.TempDir(), nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Dir(executablePath); dataFolder != want {
				t.Fatalf("data folder = %q, want executable folder %q", dataFolder, want)
			}
			if configLookupCalled {
				t.Fatal("user config directory lookup should not be called on this platform")
			}
		})
	}
}

func TestResolveRuntimeDataFolderReportsUserConfigDirectoryFailures(t *testing.T) {
	wantErr := errors.New("config unavailable")
	_, err := resolveRuntimeDataFolder("darwin", filepath.Join(t.TempDir(), "app", "slm"), func() (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped config lookup error", err)
	}
	if !strings.Contains(err.Error(), "resolve user config directory") {
		t.Fatalf("error does not identify config directory resolution: %v", err)
	}
}

func TestResolveRuntimeDataFolderReportsDirectoryCreationFailures(t *testing.T) {
	configRoot := t.TempDir()
	configFile := filepath.Join(configRoot, "not-a-directory")
	if err := os.WriteFile(configFile, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := resolveRuntimeDataFolder("darwin", filepath.Join(t.TempDir(), "app", "slm"), func() (string, error) {
		return configFile, nil
	})
	if err == nil || !strings.Contains(err.Error(), "create application data directory") {
		t.Fatalf("error = %v, want application data directory creation error", err)
	}
}
