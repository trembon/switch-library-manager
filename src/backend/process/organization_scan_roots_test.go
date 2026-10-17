package process

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestOrganizationUsesEachScanRootUnlessImporting(t *testing.T) {
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
					want := map[string]string{}
					for kind, root := range roots {
						sourceFolder := filepath.Join(root, "incoming", "nested")
						mustMkdir(t, sourceFolder)
						writeFixture(t, filepath.Join(sourceFolder, kind+".nsp"), "synthetic "+kind)
						files[kind] = db.SwitchFileInfo{
							ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: kind + ".nsp"},
							Metadata:     metadataForID(ids[kind], 0),
						}
						if importing {
							root = library
						}
						targetFolder := root
						if perGame {
							targetFolder = filepath.Join(root, "Scan Game")
						} else if kind == "game" && (!importing || baseInLibrary) {
							targetFolder = sourceFolder
						}
						if kind == "update" {
							targetFolder = filepath.Join(targetFolder, "Updates")
						} else if kind == "dlc" {
							targetFolder = filepath.Join(targetFolder, "DLC")
						}
						want[kind] = filepath.Join(targetFolder, ids[kind]+".nsp")
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
					plan, err := BuildOrganizationPlan(library, local, remote, options, scans...)
					if err != nil {
						t.Fatal(err)
					}
					if len(plan.Conflicts) != 0 || len(plan.Moves) != 3 {
						t.Fatalf("plan = %#v, want three moves without conflicts", plan)
					}
					preview, err := BuildOrganizationPreview(library, options, local, remote, scans...)
					if err != nil {
						t.Fatal(err)
					}
					for _, entry := range preview {
						if filepath.Join(library, entry.Path) != want[entry.Kind] {
							t.Errorf("%s preview = %q, want %q", entry.Kind, entry.Path, want[entry.Kind])
						}
					}
					if err := ExecuteOrganizationPlan(plan, local, library, options, nil, scans...); err != nil {
						t.Fatal(err)
					}
					for kind, file := range files {
						assertMoved(t, filepath.Join(file.ExtendedInfo.BaseFolder, file.ExtendedInfo.FileName), want[kind], kind)
						if got, err := os.ReadFile(want[kind]); err != nil || string(got) != "synthetic "+kind {
							t.Errorf("%s content = %q, err = %v", kind, got, err)
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
					repeat, err := BuildOrganizationPlan(library, local, remote, options, scans...)
					if err != nil || len(repeat.Moves) != 0 || len(repeat.Conflicts) != 0 {
						t.Fatalf("repeat plan = %#v, err = %v; want no changes", repeat, err)
					}
				})
			}
		}
	}
}

func TestScanRootOrganizationHandlesMissingBaseAndSplitFiles(t *testing.T) {
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
				plan, err := BuildOrganizationPlan(library, local, remote, options, scan)
				if err != nil || len(plan.Moves) != 2 || len(plan.Conflicts) != 0 {
					t.Fatalf("plan = %#v, err = %v; want two moves", plan, err)
				}
				if err := ExecuteOrganizationPlan(plan, local, library, options, nil, scan); err != nil {
					t.Fatal(err)
				}
				root := scan
				if importing {
					root = library
				}
				for _, name := range names {
					assertMoved(t, filepath.Join(sourceFolder, name), filepath.Join(root, "Scan Game", name), name)
				}
			})
		}
	}
}

func TestOrganizationSourceRootHandlesOverlapsAndAliases(t *testing.T) {
	parent := t.TempDir()
	library := filepath.Join(parent, "library")
	scan := filepath.Join(parent, "scan")
	nested := filepath.Join(scan, "nested")
	for _, root := range []string{library, nested} {
		mustMkdir(t, root)
	}
	for _, roots := range [][]string{{scan, nested}, {nested, scan}} {
		root, importing, err := organizationSourceRoot(library, filepath.Join(nested, "incoming"), settings.OrganizeOptions{}, roots)
		if err != nil || importing || root != nested {
			t.Errorf("root = %q, importing = %v, err = %v; want nested scan root", root, importing, err)
		}
	}
	root, _, err := organizationSourceRoot(library, filepath.Join(library, "incoming"), settings.OrganizeOptions{}, []string{parent, filepath.Join(library, "incoming")})
	if err != nil || root != library {
		t.Fatalf("main library precedence: root = %q, err = %v", root, err)
	}
	if _, _, err := organizationSourceRoot(library, filepath.Join(parent, "unknown"), settings.OrganizeOptions{}, []string{scan}); err == nil {
		t.Fatal("accepted a source outside configured roots")
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(nested, alias); err != nil {
		t.Skipf("directory symlinks are unavailable: %v", err)
	}
	root, _, err = organizationSourceRoot(library, alias, settings.OrganizeOptions{}, []string{scan})
	if err != nil || root != scan {
		t.Fatalf("alias root = %q, err = %v; want %q", root, err, scan)
	}
}

func TestEmptyFolderCleanupPreservesOverlappingRoots(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested-scan")
	empty := filepath.Join(nested, "empty")
	mustMkdir(t, empty)
	if err := deleteEmptyOrganizationFolders(root, []string{nested, root, ""}); err != nil {
		t.Fatal(err)
	}
	assertNotExists(t, empty)
	assertExists(t, nested)
	assertExists(t, root)
}

func TestOldUpdateCleanupIncludesScanRoots(t *testing.T) {
	base := t.TempDir()
	library, scan := filepath.Join(base, "library"), filepath.Join(base, "scan")
	oldFolder := filepath.Join(scan, "old")
	mustMkdir(t, library)
	mustMkdir(t, oldFolder)
	oldPath, newPath := filepath.Join(oldFolder, "old.nsp"), filepath.Join(scan, "new.nsp")
	writeFixture(t, oldPath, "old update")
	writeFixture(t, newPath, "new update")
	if err := settings.SaveSettingsWithError(&settings.AppSettings{
		Paths:        settings.PathSettings{LibraryFolder: library, ScanFolders: []string{scan}},
		Organization: settings.OrganizeOptions{DeleteOldUpdateFiles: true, DeleteEmptyFolders: true},
	}, base); err != nil {
		t.Fatal(err)
	}
	local := &db.LocalSwitchFilesDB{CleanupCandidates: []db.CleanupCandidate{{File: extendedInfoForPath(t, oldPath), Replacement: extendedInfoForPath(t, newPath)}}}
	if err := DeleteOldUpdatesAt(base, library, local, nil); err != nil {
		t.Fatal(err)
	}
	assertNotExists(t, oldPath)
	assertNotExists(t, oldFolder)
	assertExists(t, newPath)
	assertExists(t, scan)
	assertExists(t, library)
}
