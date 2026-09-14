package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolateSettings(t *testing.T) {
	t.Helper()
	oldSettings, oldVersionURL, oldKeys := settingsInstance, versionURL, keysInstance
	settingsInstance = nil
	keysInstance = nil
	versionURL = SLM_VERSION_URL
	t.Cleanup(func() {
		settingsInstance, versionURL, keysInstance = oldSettings, oldVersionURL, oldKeys
	})
}

func TestDefaultSettingsAndJSON(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	s := ReadSettings(base)
	if s.GUI != true || !s.ScanRecursively || !s.CheckForMissingUpdates || !s.CheckForMissingDLC || s.GuiPagingSize != 100 {
		t.Fatalf("unexpected defaults: %#v", s)
	}
	if s.TitlesJsonUrl != DEFAULT_TITLES_JSON_URL || s.VersionsJsonUrl != DEFAULT_VERSIONS_JSON_URL || !s.OrganizeOptions.SwitchSafeFileNames {
		t.Fatalf("unexpected default URLs/options: %#v", s)
	}
	if _, err := os.Stat(filepath.Join(base, SETTINGS_FILENAME)); err != nil {
		t.Fatal(err)
	}
	raw := ReadSettingsAsJSON(base)
	var decoded AppSettings
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.OrganizeOptions.FileNameTemplate == "" {
		t.Fatal("default JSON omitted organization template")
	}
}

func TestSettingsSaveRoundTripAndVerification(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	custom := &AppSettings{
		TitlesJsonUrl:   "https://titles.example",
		VersionsJsonUrl: "https://versions.example",
		TitlesEtag:      "titles-etag",
		VersionsEtag:    "versions-etag",
		Folder:          "library",
		ScanFolders:     []string{"one", "two"},
		IgnoreFileTypes: []string{"txt"},
		GuiPagingSize:   25,
		OrganizeOptions: OrganizeOptions{FileNameTemplate: "{TITLE_ID}"},
	}
	SaveSettings(custom, base)
	settingsInstance = nil
	loaded := ReadSettings(base)
	if loaded.Folder != "library" || loaded.GuiPagingSize != 25 || len(loaded.ScanFolders) != 2 {
		t.Fatalf("round-trip mismatch: %#v", loaded)
	}
	if loaded.TitlesEtag == "" || loaded.VersionsEtag == "" {
		t.Fatalf("verified ETags should be populated: %#v", loaded)
	}
	if loaded.TitlesJsonUrl != custom.TitlesJsonUrl || loaded.VersionsJsonUrl != custom.VersionsJsonUrl {
		t.Fatal("custom URLs were not retained")
	}
	if loaded.OrganizeOptions.FileNameTemplate != "{TITLE_ID}" {
		t.Fatal("custom organization options were not retained")
	}

	verified := verifySettings(base, &AppSettings{})
	if verified.TitlesJsonUrl != DEFAULT_TITLES_JSON_URL || verified.VersionsJsonUrl != DEFAULT_VERSIONS_JSON_URL || verified.TitlesEtag == "" || verified.VersionsEtag == "" {
		t.Fatalf("verification defaults: %#v", verified)
	}
	if err := os.WriteFile(filepath.Join(base, TITLE_JSON_FILENAME), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, VERSIONS_JSON_FILENAME), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	verified = verifySettings(base, &AppSettings{TitlesEtag: "keep-t", VersionsEtag: "keep-v"})
	if verified.TitlesEtag != "keep-t" || verified.VersionsEtag != "keep-v" {
		t.Fatalf("existing data reset etags: %#v", verified)
	}
}

func TestMalformedSettingsFallsBackToDefaults(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	loaded := ReadSettings(base)
	if loaded.TitlesJsonUrl != DEFAULT_TITLES_JSON_URL || loaded.GuiPagingSize != 100 {
		t.Fatalf("malformed settings did not reset defaults: %#v", loaded)
	}
	if got := ReadSettingsAsJSON(base); !strings.Contains(got, DEFAULT_TITLES_JSON_URL) {
		t.Fatalf("default settings were not persisted: %s", got)
	}
}

func TestCheckForUpdates(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		status     int
		wantUpdate bool
		wantError  bool
	}{
		{name: "newer", body: `{"version":"2.0.0"}`, wantUpdate: true},
		{name: "same", body: `{"version":"1.10.0"}`},
		{name: "older", body: `{"version":"1.0.0"}`},
		{name: "malformed", body: "[", wantError: true},
		{name: "missing version", body: `{}`, wantError: true},
		{name: "server error", body: "no", status: http.StatusBadGateway, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateSettings(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.status != 0 {
					w.WriteHeader(tt.status)
				}
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			versionURL = server.URL
			updated, err := CheckForUpdates()
			if tt.wantError != (err != nil) || updated != tt.wantUpdate {
				t.Fatalf("updated=%v err=%v", updated, err)
			}
		})
	}
	isolateSettings(t)
	versionURL = "http://127.0.0.1:1/unreachable"
	if _, err := CheckForUpdates(); err == nil {
		t.Fatal("expected network error")
	}
}

func TestKeyDiscoveryOrderAndMissingKeys(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	configuredDir := filepath.Join(base, "configured")
	if err := os.Mkdir(configuredDir, 0755); err != nil {
		t.Fatal(err)
	}
	configuredFile := filepath.Join(configuredDir, "prod.keys")
	if err := os.WriteFile(configuredFile, []byte("header_key = configured\nfoo = bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "prod.keys"), []byte("header_key = current\n"), 0644); err != nil {
		t.Fatal(err)
	}
	SaveSettings(&AppSettings{Prodkeys: configuredDir}, base)
	keys, err := InitSwitchKeys(base)
	if err != nil || keys.GetKey("header_key") != "configured" || keys.GetKey("foo") != "bar" {
		t.Fatalf("configured key discovery: keys=%v err=%v", keys, err)
	}

	isolateSettings(t)
	SaveSettings(&AppSettings{Prodkeys: filepath.Join(base, "missing.keys")}, base)
	keys, err = InitSwitchKeys(base)
	if err != nil || keys.GetKey("header_key") != "current" {
		t.Fatalf("current-folder fallback: keys=%v err=%v", keys, err)
	}

	isolateSettings(t)
	emptyBase := t.TempDir()
	SaveSettings(&AppSettings{}, emptyBase)
	keys, err = InitSwitchKeys(emptyBase)
	if err == nil || keys != nil || !strings.Contains(err.Error(), "prod.keys") {
		t.Fatalf("missing key result: keys=%v err=%v", keys, err)
	}
	if current, err := SwitchKeys(); err != nil || current != nil {
		t.Fatalf("unexpected singleton after failed discovery: %v, %v", current, err)
	}
}

func TestKeyDiscoveryHomeFallback(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(home, ".switch"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".switch", "prod.keys"), []byte("header_key = home\n"), 0644); err != nil {
		t.Fatal(err)
	}
	SaveSettings(&AppSettings{}, base)
	keys, err := InitSwitchKeys(base)
	if err != nil || keys.GetKey("header_key") != "home" {
		t.Fatalf("home key discovery: keys=%v err=%v", keys, err)
	}
}

func TestKeyFilePathCaseAndReadSettingsSingleton(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	keyFile := filepath.Join(base, "keys.KEYS")
	if err := os.WriteFile(keyFile, []byte("header_key = value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	SaveSettings(&AppSettings{Prodkeys: keyFile}, base)
	keys, err := InitSwitchKeys(base)
	if err != nil || keys.GetKey("header_key") != "value" || keys.GetKey("missing") != "" {
		t.Fatalf("key file discovery: keys=%v err=%v", keys, err)
	}
	other := t.TempDir()
	if got := ReadSettings(other); got != settingsInstance {
		t.Fatal("settings singleton was not retained")
	}
}
