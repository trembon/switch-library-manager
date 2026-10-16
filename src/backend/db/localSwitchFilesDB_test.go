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
	if err := scanFolder(library, false, &files, nil, seenScanFiles{}); err != nil {
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

	type scanCase struct {
		name      string
		roots     []string
		recursive bool
		want      int
	}
	tests := []scanCase{
		{name: "same folder twice", roots: []string{library, library}, want: 1},
		{name: "trailing separator", roots: []string{library, library + string(os.PathSeparator)}, want: 1},
		{name: "nested root inside recursive root", roots: []string{library, nested}, recursive: true, want: 2},
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		tests = append(tests, scanCase{name: "different letter case", roots: []string{library, strings.ToUpper(library)}, want: 1})
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(library, link); err == nil {
		tests = append(tests, scanCase{name: "symlinked root", roots: []string{library, link}, recursive: true, want: 2})
	} else {
		t.Logf("symlink root case skipped: %v", err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := []ExtendedFileInfo{}
			seen := seenScanFiles{}
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
	if len(stored) != 0 {
		t.Fatalf("invalid cached metadata was kept or replaced by filename data: %#v", stored)
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

func TestAcceptReadMetadataKeepsAndCachesPartiallyValidFiles(t *testing.T) {
	base := t.TempDir()
	resetDBSettings(t, base, func(*settings.AppSettings) {})
	manager, err := NewLocalSwitchDBManager(base)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	file := ExtendedFileInfo{FileName: "package.xci", BaseFolder: base, Size: 1}
	key := "package-key"
	skipped := map[ExtendedFileInfo]SkippedFile{}
	valid := manager.acceptReadMetadata(file, key, map[string]*switchfs.ContentMetaAttributes{
		"0100000000010000": {TitleId: "0100000000010000"},
		"bad":              {TitleId: "bad"},
	}, skipped)
	if len(valid) != 1 || valid["0100000000010000"] == nil {
		t.Fatalf("valid records = %#v", valid)
	}
	if diagnostic, ok := skipped[file]; ok {
		t.Fatalf("partially valid file recorded as skipped: %#v", diagnostic)
	}
	var stored map[string]*switchfs.ContentMetaAttributes
	if err := manager.db.GetEntry(DB_TABLE_FILE_SCAN_METADATA, key, &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored["0100000000010000"] == nil {
		t.Fatalf("valid subset was not cached: %#v", stored)
	}

	readFailure := SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: "failed to read NSP [reason: synthetic]"}
	skipped[file] = readFailure
	if got := manager.acceptReadMetadata(file, "other-key", map[string]*switchfs.ContentMetaAttributes{"bad": {TitleId: "bad"}}, skipped); got != nil {
		t.Fatalf("invalid-only records accepted: %#v", got)
	}
	if skipped[file] != readFailure {
		t.Fatalf("read failure diagnostic was overwritten: %#v", skipped[file])
	}

	delete(skipped, file)
	manager.acceptReadMetadata(file, "other-key", map[string]*switchfs.ContentMetaAttributes{"bad": {TitleId: "bad"}}, skipped)
	if diagnostic := skipped[file]; diagnostic.ReasonCode != REASON_MALFORMED_FILE || !strings.Contains(diagnostic.ReasonText, "ignored 1 record") {
		t.Fatalf("missing malformed diagnostic: %#v", diagnostic)
	}
}

func TestScannedPathSeenKeepsDistinctFilesDifferingOnlyByCase(t *testing.T) {
	// Two roots resolving to the same canonical folder name but holding
	// distinct files whose names differ only by case model a case-sensitive
	// directory: the folded keys collide, but the files are not the same.
	base := t.TempDir()
	first := filepath.Join(base, "first")
	second := filepath.Join(base, "second")
	for _, dir := range []string{first, second} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	upper := filepath.Join(first, "Game.nsp")
	lower := filepath.Join(second, "game.nsp")
	for _, path := range []string{upper, lower} {
		if err := os.WriteFile(path, []byte("synthetic"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	canonical := filepath.Join(base, "library")
	seen := seenScanFiles{}
	if scannedPathSeen(canonical, first, upper, seen) {
		t.Fatal("first file reported as seen")
	}
	if scannedPathSeen(canonical, second, lower, seen) {
		t.Fatal("distinct file differing only by case was dropped")
	}
	if !scannedPathSeen(canonical, first, upper, seen) {
		t.Fatal("same canonical path was not deduplicated")
	}
	if scannedPathSeen(canonical, second, filepath.Join(second, "missing.nsp"), seen) {
		t.Fatal("new file reported as seen")
	}
}

func TestBuildCleanupCandidatesSkipsAliasOfReplacement(t *testing.T) {
	base := t.TempDir()
	keeperPath := filepath.Join(base, "DLC New [0100000000011001][v3].nsp")
	if err := os.WriteFile(keeperPath, []byte("synthetic"), 0o644); err != nil {
		t.Fatal(err)
	}
	aliasPath := filepath.Join(base, "DLC Alias [0100000000011001][v3].nsp")
	if err := os.Link(keeperPath, aliasPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	copyPath := filepath.Join(base, "DLC Copy [0100000000011001][v3].nsp")
	if err := os.WriteFile(copyPath, []byte("synthetic"), 0o644); err != nil {
		t.Fatal(err)
	}

	id := "0100000000011001"
	keeper := ExtendedFileInfo{BaseFolder: base, FileName: filepath.Base(keeperPath)}
	alias := ExtendedFileInfo{BaseFolder: base, FileName: filepath.Base(aliasPath)}
	duplicate := ExtendedFileInfo{BaseFolder: base, FileName: filepath.Base(copyPath)}
	titles := map[string]*SwitchGameFiles{"0100000000010": {
		Dlc: map[string]SwitchFileInfo{id: {ExtendedInfo: keeper, Metadata: &switchfs.ContentMetaAttributes{TitleId: id, Version: 3}}},
	}}
	contents := map[ExtendedFileInfo]map[string]*switchfs.ContentMetaAttributes{
		alias:     {id: {TitleId: id, Version: 3}},
		duplicate: {id: {TitleId: id, Version: 3}},
	}
	skipped := map[ExtendedFileInfo]SkippedFile{
		alias:     {ReasonCode: REASON_DUPLICATE},
		duplicate: {ReasonCode: REASON_DUPLICATE},
	}
	candidates := buildCleanupCandidates([]ExtendedFileInfo{keeper, alias, duplicate}, contents, titles, skipped)
	if len(candidates) != 1 || candidates[0].File != duplicate {
		t.Fatalf("cleanup candidates = %#v, want only the distinct copy", candidates)
	}
}
