package process

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestScanFoldersAreReadOnlyUnlessImporting(t *testing.T) {
	for _, importing := range []bool{false, true} {
		for _, perGame := range []bool{false, true} {
			for _, baseInLibrary := range []bool{false, true} {
				t.Run(fmt.Sprintf("import=%v/per-game=%v/library-base=%v", importing, perGame, baseInLibrary), func(t *testing.T) {
					parent := t.TempDir()
					library := filepath.Join(parent, "library")
					scans := []string{filepath.Join(parent, "library-backup"), filepath.Join(parent, "other-scan")}
					baseRoot := scans[0]
					if baseInLibrary {
						baseRoot = library
					}
					roots := map[string]string{"game": baseRoot, "update": scans[1], "dlc": library}
					ids := map[string]string{"game": "0100E95004039000", "update": "0100E95004039800", "dlc": "0100E9500403A001"}
					files := map[string]db.SwitchFileInfo{}
					sources := map[string]string{}
					want := map[string]string{}
					wantMoves := 0
					for kind, root := range roots {
						sourceFolder := filepath.Join(root, "incoming", "nested")
						mustMkdir(t, sourceFolder)
						sources[kind] = filepath.Join(sourceFolder, kind+".nsp")
						writeFixture(t, sources[kind], "synthetic "+kind)
						files[kind] = db.SwitchFileInfo{
							ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: kind + ".nsp"},
							Metadata:     metadataForID(ids[kind], 0),
						}
						inLibrary := root == library
						if !inLibrary && !importing {
							want[kind] = sources[kind]
							continue
						}
						targetFolder := library
						if perGame {
							targetFolder = filepath.Join(library, "Scan Game")
						} else if kind == "game" && inLibrary {
							targetFolder = sourceFolder
						}
						if kind == "update" {
							targetFolder = filepath.Join(targetFolder, "Updates")
						} else if kind == "dlc" {
							targetFolder = filepath.Join(targetFolder, "DLC")
						}
						want[kind] = filepath.Join(targetFolder, ids[kind]+".nsp")
						wantMoves++
					}
					for _, root := range append([]string{library}, scans...) {
						mustMkdir(t, root)
					}
					local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
						"0100e95004039": {BaseExist: true, File: files["game"], Updates: map[int]db.SwitchFileInfo{0: files["update"]}, Dlc: map[string]db.SwitchFileInfo{ids["dlc"]: files["dlc"]}},
					}}
					remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
						"0100e95004039": {Attributes: db.TitleAttributes{Id: ids["game"], Name: "Scan Game"}},
					}}
					options := settings.OrganizeOptions{
						MoveScanFilesToLibrary: importing, CreateFolderPerGame: perGame, FolderNameTemplate: "{TITLE_NAME}",
						RenameFiles: true, FileNameTemplate: "{TITLE_ID}", UpdatesFolder: "Updates", DlcFolder: "DLC", DeleteEmptyFolders: true,
					}
					plan, err := BuildOrganizationPlan(library, local, remote, options)
					if err != nil {
						t.Fatal(err)
					}
					if len(plan.Conflicts) != 0 || len(plan.Moves) != wantMoves {
						t.Fatalf("plan = %#v, want %d moves without conflicts", plan, wantMoves)
					}
					preview, err := BuildOrganizationPreview(library, options, local, remote)
					if err != nil {
						t.Fatal(err)
					}
					for _, entry := range preview {
						got := filepath.Join(library, entry.Path)
						if want[entry.Kind] == sources[entry.Kind] {
							// Read-only files are omitted, so the preview falls back
							// to a synthetic example inside the library.
							if got == sources[entry.Kind] {
								t.Errorf("%s preview shows read-only scan file %q", entry.Kind, got)
							}
							continue
						}
						if got != want[entry.Kind] {
							t.Errorf("%s preview = %q, want %q", entry.Kind, entry.Path, want[entry.Kind])
						}
					}
					if err := ExecuteOrganizationPlan(plan, local, library, options, nil, scans...); err != nil {
						t.Fatal(err)
					}
					for kind, file := range files {
						if want[kind] == sources[kind] {
							assertExists(t, sources[kind])
						} else {
							assertMoved(t, sources[kind], want[kind], kind)
						}
						if got, err := os.ReadFile(want[kind]); err != nil || string(got) != "synthetic "+kind {
							t.Errorf("%s content = %q, err = %v", kind, got, err)
						}
						if roots[kind] != library {
							// Empty-folder cleanup never touches scan folders.
							assertExists(t, file.ExtendedInfo.BaseFolder)
							continue
						}
						retained := false
						for _, destination := range want {
							retained = retained || filepath.Dir(destination) == file.ExtendedInfo.BaseFolder
						}
						if !retained {
							assertNotExists(t, file.ExtendedInfo.BaseFolder)
						}
					}
					for _, root := range append([]string{library}, scans...) {
						assertExists(t, root)
					}
					repeat, err := BuildOrganizationPlan(library, local, remote, options)
					if err != nil || len(repeat.Moves) != 0 || len(repeat.Conflicts) != 0 {
						t.Fatalf("repeat plan = %#v, err = %v; want no changes", repeat, err)
					}
				})
			}
		}
	}
}

func TestReadOnlyScanFolderSkipsMissingBaseAndSplitFiles(t *testing.T) {
	for _, importing := range []bool{false, true} {
		for _, split := range []bool{false, true} {
			t.Run(fmt.Sprintf("import=%v/split=%v", importing, split), func(t *testing.T) {
				library, scan := t.TempDir(), t.TempDir()
				sourceFolder := filepath.Join(scan, "incoming")
				mustMkdir(t, sourceFolder)
				game := &db.SwitchGameFiles{}
				names := []string{"update.nsp", "dlc.nsp"}
				if split {
					names = []string{"game.nsp00", "game.nsp01"}
					game.BaseExist, game.IsSplit = true, true
					game.File = db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: names[0]}, Metadata: metadataForID("0100E95004039000", 0)}
				} else {
					game.Updates = map[int]db.SwitchFileInfo{1: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: names[0]}, Metadata: metadataForID("0100E95004039800", 1)}}
					game.Dlc = map[string]db.SwitchFileInfo{"0100E9500403A001": {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: names[1]}, Metadata: metadataForID("0100E9500403A001", 0)}}
				}
				for _, name := range names {
					writeFixture(t, filepath.Join(sourceFolder, name), "synthetic "+name)
				}
				local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{"0100e95004039": game}}
				options := settings.OrganizeOptions{MoveScanFilesToLibrary: importing, CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}", ProcessWhenMissingBaseGame: true}
				remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{"0100e95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Scan Game"}}}}
				plan, err := BuildOrganizationPlan(library, local, remote, options)
				wantMoves := 0
				if importing {
					wantMoves = 2
				}
				if err != nil || len(plan.Moves) != wantMoves || len(plan.Conflicts) != 0 {
					t.Fatalf("plan = %#v, err = %v; want %d moves", plan, err, wantMoves)
				}
				if err := ExecuteOrganizationPlan(plan, local, library, options, nil, scan); err != nil {
					t.Fatal(err)
				}
				for _, name := range names {
					if importing {
						assertMoved(t, filepath.Join(sourceFolder, name), filepath.Join(library, "Scan Game", name), name)
					} else {
						assertExists(t, filepath.Join(sourceFolder, name))
						assertNotExists(t, filepath.Join(scan, "Scan Game"))
					}
				}
			})
		}
	}
}

func TestOrganizationSourceScope(t *testing.T) {
	parent := t.TempDir()
	library := filepath.Join(parent, "library")
	nestedScan := filepath.Join(library, "nested-scan")
	sibling := filepath.Join(parent, "library-backup")
	for _, folder := range []string{nestedScan, sibling} {
		mustMkdir(t, folder)
	}
	cases := []struct {
		name          string
		source        string
		importing     bool
		wantIncluded  bool
		wantImporting bool
	}{
		{"library file", filepath.Join(library, "incoming"), false, true, false},
		{"scan folder nested in library", filepath.Join(nestedScan, "incoming"), false, true, false},
		{"external scan file is read-only", filepath.Join(sibling, "incoming"), false, false, false},
		{"external scan file is imported", filepath.Join(sibling, "incoming"), true, true, true},
		{"library file is not imported", filepath.Join(library, "incoming"), true, true, false},
	}
	for _, tc := range cases {
		included, importing, err := organizationSourceScope(library, tc.source, settings.OrganizeOptions{MoveScanFilesToLibrary: tc.importing})
		if err != nil || included != tc.wantIncluded || importing != tc.wantImporting {
			t.Errorf("%s: included = %v, importing = %v, err = %v; want %v, %v", tc.name, included, importing, err, tc.wantIncluded, tc.wantImporting)
		}
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(nestedScan, alias); err != nil {
		t.Skipf("directory symlinks are unavailable: %v", err)
	}
	included, importing, err := organizationSourceScope(library, alias, settings.OrganizeOptions{})
	if err != nil || !included || importing {
		t.Fatalf("alias into library: included = %v, importing = %v, err = %v", included, importing, err)
	}
}

func TestEmptyFolderCleanupIsLibraryOnly(t *testing.T) {
	parent := t.TempDir()
	library := filepath.Join(parent, "library")
	nested := filepath.Join(library, "nested-scan")
	libraryEmpty := filepath.Join(library, "empty", "deeper")
	nestedEmpty := filepath.Join(nested, "empty")
	external := filepath.Join(parent, "scan")
	externalEmpty := filepath.Join(external, "empty")
	for _, folder := range []string{libraryEmpty, nestedEmpty, externalEmpty} {
		mustMkdir(t, folder)
	}
	if err := deleteEmptyOrganizationFolders(library, []string{nested, external, ""}); err != nil {
		t.Fatal(err)
	}
	assertNotExists(t, filepath.Join(library, "empty"))
	assertNotExists(t, nestedEmpty)
	assertExists(t, nested)
	assertExists(t, library)
	assertExists(t, externalEmpty)
	if err := deleteEmptyOrganizationFolders("", []string{external}); err != nil {
		t.Fatalf("empty library root: %v", err)
	}
	assertExists(t, externalEmpty)
}

func TestCleanupCandidateScopeWithoutLibraryFolder(t *testing.T) {
	candidate := db.CleanupCandidate{
		File:        db.ExtendedFileInfo{BaseFolder: t.TempDir(), FileName: "old.nsp"},
		Replacement: db.ExtendedFileInfo{BaseFolder: t.TempDir(), FileName: "new.nsp"},
	}
	if allowed, err := cleanupCandidateInScope("", candidate, false); err != nil || allowed {
		t.Fatalf("without library folder: allowed = %v, err = %v; want false", allowed, err)
	}
	if allowed, err := cleanupCandidateInScope("", candidate, true); err != nil || !allowed {
		t.Fatalf("importing without library folder: allowed = %v, err = %v; want true", allowed, err)
	}
}

func TestOldUpdateCleanupLeavesScanFoldersUnlessImporting(t *testing.T) {
	cases := []struct {
		name              string
		candidateInScan   bool
		replacementInScan bool
		importing         bool
		wantDeleted       bool
	}{
		{"library candidate and replacement", false, false, false, true},
		{"scan candidate is read-only", true, true, false, false},
		{"library candidate kept when replacement is in scan folder", false, true, false, false},
		{"scan candidate deleted when importing", true, false, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			library, scan := filepath.Join(base, "library"), filepath.Join(base, "scan")
			folderFor := func(inScan bool) string {
				if inScan {
					return filepath.Join(scan, "old")
				}
				return filepath.Join(library, "old")
			}
			oldFolder := folderFor(tc.candidateInScan)
			newFolder := filepath.Join(library, "current")
			if tc.replacementInScan {
				newFolder = filepath.Join(scan, "current")
			}
			for _, folder := range []string{library, scan, oldFolder, newFolder} {
				mustMkdir(t, folder)
			}
			oldPath, newPath := filepath.Join(oldFolder, "old.nsp"), filepath.Join(newFolder, "new.nsp")
			writeFixture(t, oldPath, "old update")
			writeFixture(t, newPath, "new update")
			if err := settings.SaveSettingsWithError(&settings.AppSettings{
				Paths: settings.PathSettings{LibraryFolder: library, ScanFolders: []string{scan}},
				Organization: settings.OrganizeOptions{
					DeleteOldUpdateFiles: true, DeleteEmptyFolders: true, MoveScanFilesToLibrary: tc.importing,
				},
			}, base); err != nil {
				t.Fatal(err)
			}
			local := &db.LocalSwitchFilesDB{CleanupCandidates: []db.CleanupCandidate{{File: extendedInfoForPath(t, oldPath), Replacement: extendedInfoForPath(t, newPath)}}}
			if err := DeleteOldUpdatesAt(base, library, local, nil); err != nil {
				t.Fatal(err)
			}
			assertExists(t, newPath)
			if !tc.wantDeleted {
				assertExists(t, oldPath)
				return
			}
			assertNotExists(t, oldPath)
			if tc.candidateInScan {
				// Empty-folder cleanup is limited to the library.
				assertExists(t, oldFolder)
			} else {
				assertNotExists(t, oldFolder)
			}
			assertExists(t, scan)
			assertExists(t, library)
		})
	}
}
