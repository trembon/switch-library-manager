package db

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestParseTitleIdFromFileName(t *testing.T) {
	fileName := "Super Mario [0100000000010000][v0].nsp"
	titleId, err := parseTitleIdFromFileName(fileName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if titleId == nil || *titleId != "0100000000010000" {
		if titleId == nil {
			t.Fatalf("expected title ID not nil")
		}
		t.Fatalf("expected 0100000000010000 got %v", *titleId)
	}
}

func TestParseTitleIdFromFileNameInvalid(t *testing.T) {
	fileName := "Invalid [01000000000100,0][v0].nsp"
	_, err := parseTitleIdFromFileName(fileName)
	if err == nil {
		t.Fatalf("expected error for invalid title id")
	}
}

func TestScanKeepsDLCWhenContainerMetadataIsEmpty(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run("cached="+strconv.FormatBool(cached), func(t *testing.T) {
			base := t.TempDir()
			resetDBSettings(t, base, func(s *settings.AppSettings) {
				s.Paths.ProdKeys = filepath.Join(base, "prod.keys")
			})
			if err := os.WriteFile(filepath.Join(base, "prod.keys"), []byte("header_key = synthetic\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := settings.InitSwitchKeys(base); err != nil {
				t.Fatal(err)
			}
			manager, err := NewLocalSwitchDBManager(base)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()

			// A valid PFS0 container with no readable CNMT records. Use the IDs
			// and versions shown in issue #143, without any game data or keys.
			container := append([]byte("PFS0"), make([]byte, 12)...)
			files := []ExtendedFileInfo{}
			for _, name := range []string{
				"Nickelodeon All-Star Brawl 2 (The Quickster Costume)[010010701AFB3002][v0].nsz",
				"MELTY BLOOD TYPE LUMINA (DLC Miyako Arima Round Announcements)[01001E30126A100F][v0].nsz",
				"Blazblue Entropy Effect X (Hakumen Palette [X-File])[01003DA0254A5015][v65536].nsz",
				"unidentified.nsz",
			} {
				file := ExtendedFileInfo{FileName: name, BaseFolder: base, Size: int64(len(container))}
				path := filepath.Join(base, name)
				if err := os.WriteFile(path, container, 0o600); err != nil {
					t.Fatal(err)
				}
				if cached {
					key := path + "|" + name + "|" + strconv.Itoa(len(container))
					if err := manager.db.AddEntry(DB_TABLE_FILE_SCAN_METADATA, key, map[string]*switchfs.ContentMetaAttributes{}); err != nil {
						t.Fatal(err)
					}
				}
				files = append(files, file)
			}

			titles := map[string]*SwitchGameFiles{}
			skipped := map[ExtendedFileInfo]SkippedFile{}
			verifiedContents := manager.processLocalFiles(files, nil, titles, skipped)
			if len(verifiedContents) != 0 {
				t.Fatalf("filename fallback must not authorize duplicate cleanup: %#v", verifiedContents)
			}
			for id, version := range map[string]int{
				"010010701afb3002": 0,
				"01001e30126a100f": 0,
				"01003da0254a5015": 65536,
			} {
				prefix, err := TitleIDPrefix(id)
				if err != nil {
					t.Fatal(err)
				}
				game := titles[prefix]
				if game == nil || game.Dlc[id].Metadata == nil || game.Dlc[id].Metadata.Version != version {
					t.Errorf("DLC %s disappeared instead of using filename metadata: %#v", id, game)
				}
			}
			for _, file := range files {
				if diagnostic, ok := skipped[file]; !ok || diagnostic.ReasonCode != REASON_MALFORMED_FILE {
					t.Errorf("missing metadata diagnostic for %s: %#v", file.FileName, diagnostic)
				}
				var stored map[string]*switchfs.ContentMetaAttributes
				key := filepath.Join(base, file.FileName) + "|" + file.FileName + "|" + strconv.Itoa(len(container))
				if err := manager.db.GetEntry(DB_TABLE_FILE_SCAN_METADATA, key, &stored); err != nil {
					t.Fatal(err)
				}
				if !cached && stored != nil {
					t.Errorf("empty metadata was cached for %s", file.FileName)
				}
			}
		})
	}
}

func TestScanKeepsDistinctFilesWithSameSizeAndTime(t *testing.T) {
	// Issue #143: small DLC files often share a byte size and timestamp. On
	// volumes without file IDs (exFAT, FAT32, many network shares) os.SameFile
	// reported such files as identical, so all but one were silently dropped.
	base := t.TempDir()
	library := filepath.Join(base, "library")
	if err := os.Mkdir(library, 0o755); err != nil {
		t.Fatal(err)
	}
	resetDBSettings(t, base, func(*settings.AppSettings) {})
	names := []string{
		"Game [01003DA0254A4000][v0].nsz",
		"Game (Palette A)[01003DA0254A5015][v65536].nsz",
		"Game (Palette B)[01003DA0254A5016][v0].nsz",
	}
	modTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for _, name := range names {
		path := filepath.Join(library, name)
		if err := os.WriteFile(path, []byte("synthetic same-size fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	files := []ExtendedFileInfo{}
	if err := scanFolder(library, false, &files, nil, map[string]struct{}{}); err != nil {
		t.Fatal(err)
	}
	if len(files) != len(names) {
		t.Fatalf("scanned %d files, want %d: %#v", len(files), len(names), files)
	}

	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	local, err := manager.CreateLocalSwitchFilesDB([]string{library}, nil, false, true)
	if err != nil {
		t.Fatal(err)
	}
	game := local.TitlesMap["01003da0254a4"]
	if game == nil || !game.BaseExist {
		t.Fatalf("base game missing: %#v", local.TitlesMap)
	}
	for id, version := range map[string]int{"01003da0254a5015": 65536, "01003da0254a5016": 0} {
		dlc, ok := game.Dlc[id]
		if !ok || dlc.Metadata == nil || dlc.Metadata.Version != version {
			t.Errorf("DLC %s missing or wrong version: %#v", id, game.Dlc)
		}
	}
}

func TestScanDeduplicatesSameFileThroughEquivalentRoots(t *testing.T) {
	base := t.TempDir()
	library := filepath.Join(base, "library")
	nested := filepath.Join(library, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(library, "Root [0100000000010000][v0].nsp"),
		filepath.Join(nested, "Nested [0100000000020000][v0].nsp"),
	} {
		if err := os.WriteFile(path, []byte("synthetic fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name      string
		roots     []string
		recursive bool
		want      int
	}{
		{name: "same folder twice", roots: []string{library, library}, want: 1},
		{name: "trailing separator", roots: []string{library, library + string(os.PathSeparator)}, want: 1},
		{name: "nested root inside recursive root", roots: []string{library, nested}, recursive: true, want: 2},
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		tests = append(tests, struct {
			name      string
			roots     []string
			recursive bool
			want      int
		}{name: "different letter case", roots: []string{library, strings.ToUpper(library)}, want: 1})
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(library, link); err == nil {
		tests = append(tests, struct {
			name      string
			roots     []string
			recursive bool
			want      int
		}{name: "symlinked root", roots: []string{library, link}, recursive: true, want: 2})
	} else {
		t.Logf("symlink root case skipped: %v", err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := []ExtendedFileInfo{}
			seen := map[string]struct{}{}
			for _, root := range test.roots {
				if err := scanFolder(root, test.recursive, &files, nil, seen); err != nil {
					t.Fatal(err)
				}
			}
			if len(files) != test.want {
				t.Fatalf("scanned %d files, want %d: %#v", len(files), test.want, files)
			}
		})
	}
}

func TestScanFallsBackToFilenameWhenCachedTitleIDIsInvalid(t *testing.T) {
	base := t.TempDir()
	resetDBSettings(t, base, func(s *settings.AppSettings) {
		s.Paths.ProdKeys = filepath.Join(base, "prod.keys")
	})
	if err := os.WriteFile(filepath.Join(base, "prod.keys"), []byte("header_key = synthetic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.InitSwitchKeys(base); err != nil {
		t.Fatal(err)
	}
	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	name := "Game (Palette A)[01003DA0254A5015][v65536].nsz"
	container := append([]byte("PFS0"), make([]byte, 12)...)
	path := filepath.Join(base, name)
	if err := os.WriteFile(path, container, 0o600); err != nil {
		t.Fatal(err)
	}
	file := ExtendedFileInfo{FileName: name, BaseFolder: base, Size: int64(len(container))}
	key := path + "|" + name + "|" + strconv.Itoa(len(container))
	if err := manager.db.AddEntry(DB_TABLE_FILE_SCAN_METADATA, key, map[string]*switchfs.ContentMetaAttributes{
		"bad": {TitleId: "not-a-title-id", Version: 1},
	}); err != nil {
		t.Fatal(err)
	}

	titles := map[string]*SwitchGameFiles{}
	skipped := map[ExtendedFileInfo]SkippedFile{}
	manager.processLocalFiles([]ExtendedFileInfo{file}, nil, titles, skipped)

	game := titles["01003da0254a4"]
	if game == nil || game.Dlc["01003da0254a5015"].Metadata == nil || game.Dlc["01003da0254a5015"].Metadata.Version != 65536 {
		t.Fatalf("DLC was not recovered from filename: %#v", titles)
	}
	if diagnostic, ok := skipped[file]; !ok || diagnostic.ReasonCode != REASON_MALFORMED_FILE {
		t.Fatalf("missing malformed diagnostic: %#v", skipped)
	}
	var stored map[string]*switchfs.ContentMetaAttributes
	if err := manager.db.GetEntry(DB_TABLE_FILE_SCAN_METADATA, key, &stored); err != nil {
		t.Fatal(err)
	}
	if _, ok := stored["bad"]; !ok || len(stored) != 1 {
		t.Fatalf("invalid metadata must not be replaced by filename data in the cache: %#v", stored)
	}
}

func TestValidContentMetadataDropsNilAndInvalidRecords(t *testing.T) {
	valid, dropped := validContentMetadata(map[string]*switchfs.ContentMetaAttributes{
		"a": nil,
		"b": {TitleId: ""},
		"c": {TitleId: "01003DA0254A5015", Version: 3},
		"d": {TitleId: "0100000000010000"[:15]},
	})
	if dropped != 3 || len(valid) != 1 || valid["01003da0254a5015"] == nil || valid["01003da0254a5015"].TitleId != "01003da0254a5015" {
		t.Fatalf("valid=%#v dropped=%d", valid, dropped)
	}
}
