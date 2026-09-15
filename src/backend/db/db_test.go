package db

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
	bolt "go.etcd.io/bbolt"
)

type progressRecorder struct {
	mu      sync.Mutex
	updates []string
}

func (p *progressRecorder) UpdateProgress(_ int, _ int, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.updates = append(p.updates, message)
}

func resetDBSettings(t *testing.T, baseFolder string, configure func(*settings.AppSettings)) {
	t.Helper()
	appSettings := &settings.AppSettings{ScanFolders: []string{}, IgnoreFileTypes: []string{}}
	configure(appSettings)
	settings.SaveSettings(appSettings, baseFolder)
}

func TestCreateSwitchTitleDBGroupsTitlesAndVersions(t *testing.T) {
	titles := `{
		"0100000000010000":{"id":"0100000000010000","name":"Game","releaseDate":20240102},
		"0100000000010800":{"id":"0100000000010800","name":"Update"},
		"0100000000011001":{"id":"0100000000011001","name":"DLC"}
	}`
	versions := `{"0100000000010000":{"1":"2024-01-01","2":"2024-02-01"}}`

	db, err := CreateSwitchTitleDB(strings.NewReader(titles), strings.NewReader(versions))
	if err != nil {
		t.Fatal(err)
	}
	game, ok := db.TitlesMap["0100000000010"]
	if !ok {
		t.Fatalf("group not found: %#v", db.TitlesMap)
	}
	if game.Attributes.Name != "Game" || game.Attributes.ParsedReleaseDate != "2024-01-02" {
		t.Fatalf("unexpected base attributes: %#v", game.Attributes)
	}
	if len(game.Updates) != 2 || game.Updates[2] != "2024-02-01" {
		t.Fatalf("unexpected updates: %#v", game.Updates)
	}
	if game.Dlc["0100000000011001"].Name != "DLC" {
		t.Fatalf("unexpected DLC: %#v", game.Dlc)
	}
}

func TestCreateSwitchTitleDBRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		titles string
	}{
		{name: "bad titles JSON", titles: "{"},
		{name: "bad versions JSON", titles: `{"0100000000010000":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions := strings.NewReader(`{}`)
			if tt.name == "bad versions JSON" {
				versions = strings.NewReader("[")
			}
			_, err := CreateSwitchTitleDB(strings.NewReader(tt.titles), versions)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
	if _, err := titleIDPrefix("0100000000010001"); err == nil {
		t.Fatal("expected invalid DLC group nibble error")
	}
}

func TestCreateSwitchTitleDBSkipsUnsupportedTitleIDs(t *testing.T) {
	titles := `{
		"0100000000000816":{"name":"Unsupported system item"},
		"0100000000010000":{"name":"Game"}
	}`

	db, err := CreateSwitchTitleDB(strings.NewReader(titles), strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := db.TitlesMap["010000000000"]; ok {
		t.Fatal("unsupported title ID was added to the database")
	}
	if game, ok := db.TitlesMap["0100000000010"]; !ok || game.Attributes.Name != "Game" {
		t.Fatalf("valid title was not loaded: %#v", db.TitlesMap)
	}
}

func TestFilenameParsing(t *testing.T) {
	if version, err := parseVersionFromFileName("game [v123].nsp"); err != nil || *version != 123 {
		t.Fatalf("version with v: %v, %v", version, err)
	}
	if version, err := parseVersionFromFileName("game [V7].nsp"); err != nil || *version != 7 {
		t.Fatalf("version with V: %v, %v", version, err)
	}
	if version, err := parseVersionFromFileName("game [0].nsp"); err != nil || *version != 0 {
		t.Fatalf("version without v: %v, %v", version, err)
	}
	if _, err := parseVersionFromFileName("game.nsp"); err == nil {
		t.Fatal("expected missing version error")
	}
	if id, err := parseTitleIdFromFileName("Game [0100000000010000].nsp"); err != nil || *id != "0100000000010000" {
		t.Fatalf("title ID: %v, %v", id, err)
	}
	if _, err := parseTitleIdFromFileName("Game [010000000001000G].nsp"); err == nil {
		t.Fatal("expected invalid title ID error")
	}
	if ParseTitleNameFromFileName("Game [id][v1].nsp") != "Game " || ParseTitleNameFromFileName("Game.nsp") != "Game.nsp" {
		t.Fatal("unexpected title name parsing")
	}
}

func TestScanFolderAndClassifyFilenameFallback(t *testing.T) {
	base := t.TempDir()
	library := filepath.Join(base, "library")
	if err := os.Mkdir(library, 0755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(library, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	resetDBSettings(t, base, func(s *settings.AppSettings) {
		s.IgnoreFileTypes = []string{".ignored"}
	})
	files := []string{
		"Base [0100000000010000][v1].nsp",
		"Duplicate Base [0100000000010000][v1].nsp",
		"Update New [0100000000010800][v2].nsp",
		"Update Old [0100000000010800][v1].nsp",
		"Update Duplicate [0100000000010800][v2].nsp",
		"DLC New [0100000000011001][v3].nsp",
		"DLC Old [0100000000011001][v2].nsp",
		"ignored.ignored",
		"unsupported.txt",
		"unknown.nsp",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(library, name), []byte("synthetic"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(nested, "Nested [0100000000020000][v1].nsp"), []byte("synthetic"), 0644); err != nil {
		t.Fatal(err)
	}

	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	progress := &progressRecorder{}
	local, err := manager.CreateLocalSwitchFilesDB([]string{library}, progress, false, true)
	if err != nil {
		t.Fatal(err)
	}
	game := local.TitlesMap["0100000000010"]
	if game == nil || !game.BaseExist || game.LatestUpdate != 2 || len(game.Dlc) != 1 {
		t.Fatalf("unexpected local classification: %#v", game)
	}
	if game.File.Metadata.TitleId != "0100000000010000" || game.Updates[2].Metadata.Version != 2 {
		t.Fatalf("unexpected fallback metadata: %#v", game)
	}
	if len(local.TitlesMap) != 1 || local.NumFiles != len(files) {
		t.Fatalf("unexpected scan result: %#v, files=%d", local.TitlesMap, local.NumFiles)
	}
	assertSkippedReason(t, local.Skipped, "Duplicate Base", REASON_DUPLICATE)
	assertSkippedReason(t, local.Skipped, "Update Old", REASON_OLD_UPDATE)
	assertSkippedReason(t, local.Skipped, "Update New", REASON_DUPLICATE)
	assertSkippedReason(t, local.Skipped, "DLC Old", REASON_OLD_UPDATE)
	assertSkippedReason(t, local.Skipped, "unsupported.txt", REASON_UNSUPPORTED_TYPE)
	assertSkippedReason(t, local.Skipped, "unknown.nsp", REASON_UNRECOGNISED)
	if _, ok := findSkipped(local.Skipped, "ignored.ignored"); ok {
		t.Fatal("ignored extension was recorded")
	}
	if len(progress.updates) == 0 {
		t.Fatal("expected progress updates")
	}
	recursive, err := manager.CreateLocalSwitchFilesDB([]string{library}, nil, true, true)
	if err != nil || recursive.NumFiles != len(files)+1 || recursive.TitlesMap["0100000000020"] == nil {
		t.Fatalf("recursive scan: result=%#v err=%v", recursive, err)
	}
	manager.Close()
	manager, err = NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	cached, err := manager.CreateLocalSwitchFilesDB([]string{filepath.Join(base, "does-not-exist")}, nil, false, false)
	if err != nil || cached.NumFiles != len(files)+1 {
		t.Fatalf("cached scan: result=%#v err=%v", cached, err)
	}
}

func assertSkippedReason(t *testing.T, skipped map[ExtendedFileInfo]SkippedFile, name string, reason int) {
	t.Helper()
	entry, ok := findSkipped(skipped, name)
	if !ok || entry.ReasonCode != reason {
		t.Fatalf("%q missing or reason=%d, want %d; skipped=%#v", name, entry.ReasonCode, reason, skipped)
	}
}

func findSkipped(skipped map[ExtendedFileInfo]SkippedFile, name string) (SkippedFile, bool) {
	for file, entry := range skipped {
		if file.FileName == name || strings.HasPrefix(file.FileName, name+" ") {
			return entry, true
		}
	}
	return SkippedFile{}, false
}

func TestPersistenceRoundTripAndCacheClearing(t *testing.T) {
	base := t.TempDir()
	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	value := map[string]int{"answer": 42}
	if err := manager.db.AddEntry("test", "value", value); err != nil {
		t.Fatal(err)
	}
	var loaded map[string]int
	if err := manager.db.GetEntry("test", "value", &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded["answer"] != 42 {
		t.Fatalf("loaded value: %#v", loaded)
	}
	if err := manager.db.ClearTable("test"); err != nil {
		t.Fatal(err)
	}
	var cleared map[string]int
	if err := manager.db.GetEntry("test", "value", &cleared); err != nil {
		t.Fatal(err)
	}
	if cleared != nil {
		t.Fatalf("cleared value was returned: %#v", cleared)
	}
	if err := manager.db.AddEntry(DB_TABLE_FILE_SCAN_METADATA, "cache", value); err != nil {
		t.Fatal(err)
	}
	if err := manager.ClearScanData(); err != nil {
		t.Fatal(err)
	}
	manager.Close()

	reopened, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var missing map[string]int
	if err := reopened.db.GetEntry("test", "value", &missing); err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("cleared value survived reopen: %#v", missing)
	}
	var clearedCache map[string]int
	if err := reopened.db.GetEntry(DB_TABLE_FILE_SCAN_METADATA, "cache", &clearedCache); err != nil {
		t.Fatal(err)
	}
	if clearedCache != nil {
		t.Fatalf("cleared cache survived reopen: %#v", clearedCache)
	}
	if invalid, err := NewPersistentDB(filepath.Join(base, "missing-parent")); err == nil || invalid != nil {
		t.Fatalf("expected open error for missing parent: db=%v err=%v", invalid, err)
	}
}

func TestLoadAndUpdateFileETagFallbackAndValidation(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "titles.json")
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
		case 1:
			if got := r.Header.Get("If-None-Match"); got != "old" {
				t.Errorf("If-None-Match = %q", got)
			}
			w.Header().Set("ETag", "new")
			_, _ = w.Write([]byte(`{"title":"ok"}`))
		case 2:
			if got := r.Header.Get("If-None-Match"); got != "new" {
				t.Errorf("If-None-Match = %q", got)
			}
			w.WriteHeader(http.StatusNotModified)
		case 3:
			w.Header().Set("ETag", "bad")
			_, _ = w.Write([]byte("not-json"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	file, etag, err := LoadAndUpdateFile(server.URL, path, "old")
	if err != nil || etag != "new" {
		t.Fatalf("download: file=%v etag=%q err=%v", file, etag, err)
	}
	file.Close()
	contents, _ := os.ReadFile(path)
	if string(contents) != `{"title":"ok"}` {
		t.Fatalf("saved contents: %s", contents)
	}
	file, etag, err = LoadAndUpdateFile(server.URL, path, etag)
	if err != nil || etag != "new" {
		t.Fatalf("304 fallback: file=%v etag=%q err=%v", file, etag, err)
	}
	file.Close()
	file, etag, err = LoadAndUpdateFile(server.URL, path, etag)
	if err != nil || etag != "new" {
		t.Fatalf("invalid JSON fallback: file=%v etag=%q err=%v", file, etag, err)
	}
	file.Close()
	contents, _ = os.ReadFile(path)
	if string(contents) != `{"title":"ok"}` {
		t.Fatal("invalid JSON replaced valid local file")
	}

	missing := filepath.Join(base, "missing.json")
	if file, _, err = LoadAndUpdateFile(server.URL+"/missing", missing, ""); err == nil || file != nil {
		t.Fatalf("expected missing fallback error, file=%v err=%v", file, err)
	}
}

func TestDownloadBytesRejectsBadURLAndStatus(t *testing.T) {
	if _, _, err := downloadBytesFromUrl(":", ""); err == nil {
		t.Fatal("expected bad URL error")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	if _, _, err := downloadBytesFromUrl(server.URL, ""); err == nil || !strings.Contains(err.Error(), "non 200") {
		t.Fatalf("unexpected status error: %v", err)
	}
}

func TestGetGameMetadataUsesDeepCacheWhenKeysAvailable(t *testing.T) {
	base := t.TempDir()
	resetDBSettings(t, base, func(s *settings.AppSettings) {})
	keyPath := filepath.Join(base, "prod.keys")
	if err := os.WriteFile(keyPath, []byte("header_key = synthetic\n"), 0644); err != nil {
		t.Fatal(err)
	}
	settings.SaveSettings(&settings.AppSettings{Prodkeys: keyPath, ScanFolders: []string{}}, base)
	if _, err := settings.InitSwitchKeys(base); err != nil {
		t.Fatal(err)
	}
	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	file := ExtendedFileInfo{FileName: "cached.nsp", BaseFolder: base, Size: 10}
	cached := map[string]*switchfs.ContentMetaAttributes{
		"0100000000010000": {TitleId: "0100000000010000", Version: 9},
	}
	key := filepath.Join(base, file.FileName) + "|" + file.FileName + "|10"
	if err := manager.db.AddEntry(DB_TABLE_FILE_SCAN_METADATA, key, cached); err != nil {
		t.Fatal(err)
	}
	got, err := manager.getGameMetadata(file, filepath.Join(base, file.FileName), map[ExtendedFileInfo]SkippedFile{})
	if err != nil || got["0100000000010000"].Version != 9 {
		t.Fatalf("cached metadata: %#v, %v", got, err)
	}
}

func TestFilenameFallbackHandlesShortAndInvalidFiles(t *testing.T) {
	base := t.TempDir()
	resetDBSettings(t, base, func(s *settings.AppSettings) {})
	settings.SaveSettings(&settings.AppSettings{Prodkeys: filepath.Join(base, "missing.keys")}, base)
	if _, err := settings.InitSwitchKeys(base); err == nil {
		t.Fatal("expected synthetic key lookup to fail")
	}
	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	titles := map[string]*SwitchGameFiles{}
	skipped := map[ExtendedFileInfo]SkippedFile{}
	manager.processLocalFiles([]ExtendedFileInfo{
		{FileName: "x", BaseFolder: base, Size: 1},
		{FileName: "directory", BaseFolder: base, IsDir: true},
		{FileName: "bad [010000000001000G][v1].nsp", BaseFolder: base, Size: 1},
	}, nil, titles, skipped)
	if len(skipped) != 2 || skipped[(ExtendedFileInfo{FileName: "bad [010000000001000G][v1].nsp", BaseFolder: base, Size: 1})].ReasonCode != REASON_UNRECOGNISED {
		t.Fatalf("unexpected short/invalid-file diagnostics: %#v", skipped)
	}
}

func TestPersistentDBHandlesBadGob(t *testing.T) {
	base := t.TempDir()
	pd, err := NewPersistentDB(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := pd.AddEntry("bad", "value", func() {}); err == nil {
		t.Fatal("expected gob encoding error")
	}
	pd.Close()
}

func TestScannerPropagatesDatabaseErrors(t *testing.T) {
	base := t.TempDir()
	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	manager.Close()

	if got, err := manager.CreateLocalSwitchFilesDB(nil, nil, false, false); err == nil || got != nil {
		t.Fatalf("expected cached read error, result=%#v err=%v", got, err)
	}
	if got, err := manager.CreateLocalSwitchFilesDB(nil, nil, false, true); err == nil || got != nil {
		t.Fatalf("expected scan write error, result=%#v err=%v", got, err)
	}
	if got, err := NewLocalSwitchDBManager(filepath.Join(base, "missing")); err == nil || got != nil {
		t.Fatalf("expected manager creation error, manager=%v err=%v", got, err)
	}
}

func TestScannerClassifiesCachedMultiContentAndDlcEdges(t *testing.T) {
	base := t.TempDir()
	resetDBSettings(t, base, func(s *settings.AppSettings) {})
	if _, err := settings.InitSwitchKeys(filepath.Join(base, "no-keys")); err == nil {
		t.Fatal("expected missing keys")
	}
	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	file := ExtendedFileInfo{FileName: "multi.nsp", BaseFolder: base, Size: 20}
	filePath := filepath.Join(base, file.FileName)
	metadata := map[string]*switchfs.ContentMetaAttributes{
		"0100000000010000": {TitleId: "0100000000010000", Version: 1},
		"0100000000011001": {TitleId: "0100000000011001", Version: 1},
	}
	key := filePath + "|" + file.FileName + "|20"
	if err := manager.db.AddEntry(DB_TABLE_FILE_SCAN_METADATA, key, metadata); err != nil {
		t.Fatal(err)
	}
	for _, entry := range metadata {
		entry.TitleId = strings.ToUpper(entry.TitleId)
	}
	manager.processLocalFiles([]ExtendedFileInfo{file}, nil, map[string]*SwitchGameFiles{}, map[ExtendedFileInfo]SkippedFile{})

	titles := map[string]*SwitchGameFiles{}
	skipped := map[ExtendedFileInfo]SkippedFile{}
	manager.processLocalFiles([]ExtendedFileInfo{
		{FileName: "update old [0100000000010800][v1].nsp", BaseFolder: base},
		{FileName: "update new [0100000000010800][v2].nsp", BaseFolder: base},
		{FileName: "dlc [0100000000011001][v1].nsp", BaseFolder: base},
		{FileName: "dlc duplicate [0100000000011001][v1].nsp", BaseFolder: base},
		{FileName: "dlc newer [0100000000011001][v2].nsp", BaseFolder: base},
	}, nil, titles, skipped)
	game := titles["0100000000010"]
	if game == nil || game.LatestUpdate != 2 || game.Dlc["0100000000011001"].Metadata.Version != 2 {
		t.Fatalf("unexpected edge classification: %#v", game)
	}
	assertSkippedReason(t, skipped, "update old", REASON_OLD_UPDATE)
	assertSkippedReason(t, skipped, "dlc duplicate", REASON_DUPLICATE)
}

func TestScannerRecordsInvalidCachedMetadataAndSplitErrors(t *testing.T) {
	base := t.TempDir()
	resetDBSettings(t, base, func(s *settings.AppSettings) {})
	keyPath := filepath.Join(base, "prod.keys")
	if err := os.WriteFile(keyPath, []byte("header_key = synthetic\n"), 0644); err != nil {
		t.Fatal(err)
	}
	settings.SaveSettings(&settings.AppSettings{Prodkeys: keyPath, ScanFolders: []string{}}, base)
	if _, err := settings.InitSwitchKeys(base); err != nil {
		t.Fatal(err)
	}
	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	invalid := ExtendedFileInfo{FileName: "invalid.nsp", BaseFolder: base, Size: 1}
	invalidKey := filepath.Join(base, invalid.FileName) + "|" + invalid.FileName + "|1"
	if err := manager.db.AddEntry(DB_TABLE_FILE_SCAN_METADATA, invalidKey, map[string]*switchfs.ContentMetaAttributes{
		"invalid": {TitleId: "invalid", Version: 1},
	}); err != nil {
		t.Fatal(err)
	}
	skipped := map[ExtendedFileInfo]SkippedFile{}
	manager.processLocalFiles([]ExtendedFileInfo{invalid}, nil, map[string]*SwitchGameFiles{}, skipped)
	assertSkippedReason(t, skipped, invalid.FileName, REASON_UNRECOGNISED)

	for _, name := range []string{"bad.nsp", "bad.xci", "bad00"} {
		file := ExtendedFileInfo{FileName: name, BaseFolder: base, Size: 1}
		if err := os.WriteFile(filepath.Join(base, name), []byte("bad"), 0644); err != nil {
			t.Fatal(err)
		}
		got, err := manager.getGameMetadata(file, filepath.Join(base, name), skipped)
		if err == nil || got != nil {
			t.Fatalf("%s: expected parser error, metadata=%#v err=%v", name, got, err)
		}
		if skipped[file].ReasonCode != REASON_MALFORMED_FILE {
			t.Fatalf("%s: unexpected diagnostic: %#v", name, skipped[file])
		}
	}
}

func TestPersistentDBReadAndWriteErrors(t *testing.T) {
	base := t.TempDir()
	pd, err := NewPersistentDB(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := pd.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucket([]byte("corrupt"))
		if err != nil {
			return err
		}
		return bucket.Put([]byte("value"), []byte("not-gob"))
	}); err != nil {
		t.Fatal(err)
	}
	var value map[string]string
	if err := pd.GetEntry("corrupt", "value", &value); err == nil {
		t.Fatal("expected gob decoding error")
	}
	if _, err := saveFile([]byte("data"), filepath.Join(base, "missing", "directory")); err == nil {
		t.Fatal("expected saveFile error")
	}
	pd.Close()
	if err := pd.AddEntry("closed", "value", "value"); err == nil {
		t.Fatal("expected closed database write error")
	}
	if err := pd.GetEntry("closed", "value", &value); err == nil {
		t.Fatal("expected closed database read error")
	}
}

func TestDownloadAndLoadFileCreationErrors(t *testing.T) {
	base := t.TempDir()
	if _, _, err := LoadAndUpdateFile(":", filepath.Join(base, "missing", "titles.json"), ""); err == nil {
		t.Fatal("expected file creation error")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"valid":true}`))
	}))
	url := server.URL
	server.Close()
	if _, _, err := downloadBytesFromUrl(url, ""); err == nil {
		t.Fatal("expected HTTP client error")
	}

	saved, err := saveFile([]byte("data"), filepath.Join(base, "saved.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := saved.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTitleIDPrefixValidationAndGrouping(t *testing.T) {
	tests := []struct {
		id      string
		prefix  string
		wantErr bool
	}{
		{id: "0100000000010000", prefix: "0100000000010"},
		{id: "0100000000010800", prefix: "0100000000010"},
		{id: "0100000000012101", prefix: "0100000000011"},
		{id: "short", wantErr: true},
		{id: "010000000001000g", wantErr: true},
		{id: "0100000000010001", wantErr: true},
	}
	for _, tt := range tests {
		got, err := titleIDPrefix(tt.id)
		if tt.wantErr {
			if err == nil {
				t.Errorf("titleIDPrefix(%q) accepted invalid ID", tt.id)
			}
		} else if err != nil || got != tt.prefix {
			t.Errorf("titleIDPrefix(%q) = %q, %v; want %q", tt.id, got, err, tt.prefix)
		}
	}
}
