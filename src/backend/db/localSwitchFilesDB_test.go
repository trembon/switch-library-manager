package db

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

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
