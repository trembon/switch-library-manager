package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestImportOrganizationMovesExternalFilesAndKeepsLibraryPlacement(t *testing.T) {
	library := t.TempDir()
	inside := filepath.Join(library, "existing")
	drop := filepath.Join(t.TempDir(), "drop")
	mustMkdir(t, inside)
	mustMkdir(t, drop)
	basePath := filepath.Join(inside, "base.nsp")
	updatePath := filepath.Join(drop, "update.nsp")
	dlcPath := filepath.Join(drop, "dlc.nsp")
	writeFixture(t, basePath, "existing base")
	writeFixture(t, updatePath, "incoming update")
	writeFixture(t, dlcPath, "incoming DLC")

	baseID := "0100E95004039000"
	updateID := "0100E95004039800"
	dlcID := "0100E9500403A001"
	key := "0100e95004039"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		key: {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: inside, FileName: "base.nsp"}, Metadata: metadataForID(baseID, 0)},
			Updates: map[int]db.SwitchFileInfo{
				5: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: drop, FileName: "update.nsp"}, Metadata: metadataForID(updateID, 5)},
			},
			Dlc: map[string]db.SwitchFileInfo{
				dlcID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: drop, FileName: "dlc.nsp"}, Metadata: metadataForID(dlcID, 1)},
			},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		key: {Attributes: db.TitleAttributes{Id: baseID, Name: "Import Test"}},
	}}
	options := settings.OrganizeOptions{
		MoveScanFilesToLibrary: true,
		UpdatesFolder:          "updates",
		DlcFolder:              "dlc",
	}

	plan, err := BuildOrganizationPlan(library, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 0 || len(plan.Moves) != 2 {
		t.Fatalf("plan has %d moves and %d conflicts, want 2 moves and no conflicts: %#v", len(plan.Moves), len(plan.Conflicts), plan)
	}
	wantDestinations := map[string]bool{
		filepath.Join(library, "updates", "update.nsp"): false,
		filepath.Join(library, "dlc", "dlc.nsp"):        false,
	}
	for _, move := range plan.Moves {
		if _, ok := wantDestinations[move.Destination]; !ok {
			t.Errorf("unexpected import destination %q", move.Destination)
			continue
		}
		wantDestinations[move.Destination] = true
	}
	for destination, found := range wantDestinations {
		if !found {
			t.Errorf("plan did not include destination %q", destination)
		}
	}

	preview, err := BuildOrganizationPreview(library, options, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	previewPaths := map[string]string{}
	for _, entry := range preview {
		previewPaths[entry.Kind] = entry.Path
	}
	if previewPaths["game"] != filepath.Join("existing", "base.nsp") ||
		previewPaths["update"] != filepath.Join("updates", "update.nsp") ||
		previewPaths["dlc"] != filepath.Join("dlc", "dlc.nsp") {
		t.Fatalf("preview paths = %#v, want library base placement and imported update/DLC paths", previewPaths)
	}

	if err := ExecuteOrganizationPlan(plan, local, library, options, nil); err != nil {
		t.Fatal(err)
	}
	assertExists(t, basePath)
	assertMoved(t, updatePath, filepath.Join(library, "updates", "update.nsp"), "imported update")
	assertMoved(t, dlcPath, filepath.Join(library, "dlc", "dlc.nsp"), "imported DLC")
}

func TestImportOrganizationMovesFilesToLibraryRootWithoutOtherOptions(t *testing.T) {
	library := t.TempDir()
	drop := filepath.Join(t.TempDir(), "drop")
	mustMkdir(t, drop)
	baseID := "0100E95004039000"
	source := filepath.Join(drop, "keep this name.nsp")
	writeFixture(t, source, "synthetic game")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: drop, FileName: filepath.Base(source)}, Metadata: metadataForID(baseID, 0)},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100e95004039": {Attributes: db.TitleAttributes{Id: baseID, Name: "Import Test"}},
	}}
	options := settings.OrganizeOptions{MoveScanFilesToLibrary: true}
	plan, err := BuildOrganizationPlan(library, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 1 || plan.Moves[0].Destination != filepath.Join(library, filepath.Base(source)) {
		t.Fatalf("move-only plan = %#v, want one original-name move to library root", plan)
	}
	if err := ExecuteOrganizationPlan(plan, local, library, options, nil); err != nil {
		t.Fatal(err)
	}
	assertMoved(t, source, filepath.Join(library, filepath.Base(source)), "move-only import")
}

func TestImportOrganizationRejectsRelativeFolderEscapeAndAllowsAbsoluteFolder(t *testing.T) {
	library := t.TempDir()
	drop := filepath.Join(t.TempDir(), "drop")
	mustMkdir(t, drop)
	updateID := "0100E95004039800"
	source := filepath.Join(drop, "update.nsp")
	writeFixture(t, source, "synthetic update")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			Updates: map[int]db.SwitchFileInfo{
				5: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: drop, FileName: filepath.Base(source)}, Metadata: metadataForID(updateID, 5)},
			},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{}
	options := settings.OrganizeOptions{MoveScanFilesToLibrary: true, ProcessWhenMissingBaseGame: true, UpdatesFolder: "../../outside"}
	if _, err := BuildOrganizationPlan(library, local, remote, options); err == nil {
		t.Fatal("BuildOrganizationPlan accepted a relative import destination outside the library")
	}
	assertExists(t, source)

	explicitFolder := filepath.Join(t.TempDir(), "external-updates")
	options.UpdatesFolder = explicitFolder
	plan, err := BuildOrganizationPlan(library, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 1 || plan.Moves[0].Destination != filepath.Join(explicitFolder, filepath.Base(source)) {
		t.Fatalf("absolute-folder plan = %#v, want explicit destination under %q", plan, explicitFolder)
	}
}

func TestMoveSettingPreservesInLibraryUpdateWhenBaseIsMissing(t *testing.T) {
	library := t.TempDir()
	updates := filepath.Join(library, "existing-updates")
	mustMkdir(t, updates)
	source := filepath.Join(updates, "standalone-update.nsp")
	writeFixture(t, source, "existing update")
	updateID := "0100E95004039800"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			Updates: map[int]db.SwitchFileInfo{
				5: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: updates, FileName: filepath.Base(source)}, Metadata: metadataForID(updateID, 5)},
			},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	options := settings.OrganizeOptions{MoveScanFilesToLibrary: true, ProcessWhenMissingBaseGame: true}
	plan, err := BuildOrganizationPlan(library, local, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 0 {
		t.Fatalf("plan moved existing in-library update: %#v", plan.Moves)
	}
	assertExists(t, source)
}

func TestImportOrganizationMovesEverySplitPartToLibraryRoot(t *testing.T) {
	library := t.TempDir()
	drop := filepath.Join(t.TempDir(), "drop")
	mustMkdir(t, drop)
	baseID := "0100E95004039000"
	firstPart := "game.nsp00"
	secondPart := "game.nsp01"
	writeFixture(t, filepath.Join(drop, firstPart), "split part zero")
	writeFixture(t, filepath.Join(drop, secondPart), "split part one")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			BaseExist: true,
			IsSplit:   true,
			File: db.SwitchFileInfo{
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: drop, FileName: firstPart},
				Metadata:     metadataForID(baseID, 0),
			},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	options := settings.OrganizeOptions{MoveScanFilesToLibrary: true}
	plan, err := BuildOrganizationPlan(library, local, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 2 || len(plan.Conflicts) != 0 {
		t.Fatalf("split import plan = %#v, want both parts and no conflicts", plan)
	}
	if err := ExecuteOrganizationPlan(plan, local, library, options, nil); err != nil {
		t.Fatal(err)
	}
	assertMoved(t, filepath.Join(drop, firstPart), filepath.Join(library, firstPart), "first split part")
	assertMoved(t, filepath.Join(drop, secondPart), filepath.Join(library, secondPart), "second split part")
}

func TestImportOrganizationConflictLeavesSourceAndDestinationUntouched(t *testing.T) {
	library := t.TempDir()
	drop := filepath.Join(t.TempDir(), "drop")
	mustMkdir(t, drop)
	baseID := "0100E95004039000"
	source := filepath.Join(drop, "game.nsp")
	destination := filepath.Join(library, filepath.Base(source))
	writeFixture(t, source, "incoming file")
	writeFixture(t, destination, "existing library file")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			BaseExist: true,
			File: db.SwitchFileInfo{
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: drop, FileName: filepath.Base(source)},
				Metadata:     metadataForID(baseID, 0),
			},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	options := settings.OrganizeOptions{MoveScanFilesToLibrary: true}
	plan, err := BuildOrganizationPlan(library, local, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 {
		t.Fatalf("plan conflicts = %#v, want occupied library destination", plan.Conflicts)
	}
	if err := ExecuteOrganizationPlan(plan, local, library, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted an occupied import destination")
	}
	for path, want := range map[string]string{source: "incoming file", destination: "existing library file"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("%q = %q, err = %v; want %q", path, got, err, want)
		}
	}
}

func TestOrganizationPathWithinResolvesDirectoryAliases(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	alias := filepath.Join(t.TempDir(), "alias")
	mustMkdir(t, inside)
	if err := os.Symlink(inside, alias); err != nil {
		t.Skipf("directory symlinks are unavailable: %v", err)
	}

	within, err := organizationPathWithin(root, alias)
	if err != nil {
		t.Fatal(err)
	}
	if !within {
		t.Fatalf("symlink alias %q was treated as outside library %q", alias, root)
	}
}

func TestOrganizationPathWithinRejectsSiblingPrefix(t *testing.T) {
	root := filepath.Join(t.TempDir(), "library")
	sibling := filepath.Join(filepath.Dir(root), "library-backup")
	mustMkdir(t, root)
	mustMkdir(t, sibling)

	within, err := organizationPathWithin(root, sibling)
	if err != nil {
		t.Fatal(err)
	}
	if within {
		t.Fatalf("sibling path %q was treated as inside %q", sibling, root)
	}
}

func TestResolveOrganizationPathNormalizesExistingAndMissingPaths(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{root, filepath.Join(root, "missing", "folder")} {
		got, err := resolveOrganizationPath(path)
		if err != nil {
			t.Fatalf("resolveOrganizationPath(%q): %v", path, err)
		}
		want, err := filepath.Abs(filepath.Clean(path))
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Clean(want) {
			t.Errorf("resolveOrganizationPath(%q) = %q, want %q", path, got, filepath.Clean(want))
		}
	}
	if _, err := resolveOrganizationPath("invalid\x00path"); err == nil {
		t.Fatal("resolveOrganizationPath accepted a path containing a NUL byte")
	}
}
