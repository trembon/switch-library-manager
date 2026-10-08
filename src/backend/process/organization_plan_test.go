package process

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestBuildOrganizationPlanFindsDuplicateDestinationsBeforeMoving(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	firstPath := filepath.Join(source, "first.nsp")
	secondPath := filepath.Join(source, "second.nsp")
	writeFixture(t, firstPath, "first DLC")
	writeFixture(t, secondPath, "second DLC")

	firstID := "0100E9500403A001"
	secondID := "0100E9500403A002"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			Dlc: map[string]db.SwitchFileInfo{
				firstID:  {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "first.nsp"}, Metadata: metadataForID(firstID, 1)},
				secondID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "second.nsp"}, Metadata: metadataForID(secondID, 1)},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Game"}},
	}}
	options := settings.OrganizeOptions{
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_NAME}",
		ProcessWhenMissingBaseGame: true,
	}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 2 {
		t.Fatalf("conflicts = %#v, want both colliding DLC files", plan.Conflicts)
	}
	if len(plan.Moves) != 2 {
		t.Fatalf("planned moves = %d, want 2 before validation", len(plan.Moves))
	}
	if err := ExecuteOrganizationPlan(plan, local, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted conflicting plan")
	} else {
		var conflictErr *OrganizationConflictsError
		if !errors.As(err, &conflictErr) || len(conflictErr.Conflicts) != 2 {
			t.Fatalf("ExecuteOrganizationPlan() error = %v, want conflict error", err)
		}
	}
	assertExists(t, firstPath)
	assertExists(t, secondPath)
	assertNotExists(t, filepath.Join(source, "Game.nsp"))
}

func TestBuildOrganizationPlanReportsOccupiedDestinationWithoutOverwrite(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	sourcePath := filepath.Join(source, "original.nsp")
	destinationPath := filepath.Join(source, "Game.nsp")
	writeFixture(t, sourcePath, "source content")
	writeFixture(t, destinationPath, "existing destination")
	file := db.ExtendedFileInfo{BaseFolder: source, FileName: "original.nsp"}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: file, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Game"}},
	}}
	options := settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_NAME}"}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].Reason != "destination already exists" {
		t.Fatalf("conflicts = %#v, want occupied destination", plan.Conflicts)
	}
	if err := ExecuteOrganizationPlan(plan, local, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted occupied destination")
	}
	assertExists(t, sourcePath)
	content, err := os.ReadFile(destinationPath)
	if err != nil || string(content) != "existing destination" {
		t.Fatalf("existing destination changed: content=%q err=%v", content, err)
	}
}

func TestBuildOrganizationPlanReportsBlockedDestinationParent(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	sourcePath := filepath.Join(source, "original.nsp")
	writeFixture(t, sourcePath, "source content")
	blockedParent := filepath.Join(base, "occupied")
	writeFixture(t, blockedParent, "not a folder")
	file := db.ExtendedFileInfo{BaseFolder: source, FileName: "original.nsp"}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: file, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "occupied"}},
	}}
	options := settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].OtherSource != blockedParent {
		t.Fatalf("conflicts = %#v, want blocked parent %q", plan.Conflicts, blockedParent)
	}
	assertExists(t, sourcePath)
}

func TestAddOrganizationMovePreflightOutcomes(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source.nsp")
	writeFixture(t, source, "source")
	blockedParent := filepath.Join(base, "blocked")
	writeFixture(t, blockedParent, "not a directory")
	existingDestination := filepath.Join(base, "existing.nsp")
	writeFixture(t, existingDestination, "existing")

	var plan OrganizationPlan
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, source); err != nil || len(plan.Moves) != 0 {
		t.Fatalf("same-path move: moves=%#v err=%v", plan.Moves, err)
	}
	seen := map[string]struct{}{}
	if err := addOrganizationMove(&plan, seen, source, filepath.Join(base, "target.nsp")); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, seen, source, filepath.Join(base, "another.nsp")); err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 1 {
		t.Fatalf("duplicate source was planned more than once: %#v", plan.Moves)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, filepath.Join(base, "missing.nsp"), filepath.Join(base, "missing-target.nsp")); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, base, filepath.Join(base, "directory-target")); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, filepath.Join(blockedParent, "child.nsp")); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, existingDestination); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, filepath.Join(base, "invalid\x00parent", "target.nsp")); err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 5 {
		t.Fatalf("preflight conflicts = %#v, want missing source, directory source, blocked parent, occupied destination, and invalid parent", plan.Conflicts)
	}
}

func TestAddOrganizationMoveSkipsHardlinkAlias(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source.nsp")
	destination := filepath.Join(base, "destination.nsp")
	writeFixture(t, source, "same inode")
	if err := os.Link(source, destination); err != nil {
		t.Skipf("hard links are unavailable in this test environment: %v", err)
	}
	var plan OrganizationPlan
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, destination); err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("hard-link alias produced a move or conflict: %#v", plan)
	}
}

func TestOrganizationConflictsErrorText(t *testing.T) {
	err := (&OrganizationConflictsError{Conflicts: []OrganizationConflict{{}, {}}}).Error()
	if err != "organization canceled: 2 destination conflict(s); see Issues" {
		t.Fatalf("conflict error text = %q", err)
	}
}

func TestUniqueOrganizationConflictsRemovesDuplicates(t *testing.T) {
	conflict := OrganizationConflict{Source: "source", Destination: "destination", Reason: "occupied"}
	unique := uniqueOrganizationConflicts([]OrganizationConflict{conflict, conflict})
	if len(unique) != 1 || unique[0] != conflict {
		t.Fatalf("unique conflicts = %#v", unique)
	}
}

func TestBuildOrganizationPlanRejectsInvalidOptions(t *testing.T) {
	_, err := BuildOrganizationPlan(t.TempDir(), &db.LocalSwitchFilesDB{}, nil, settings.OrganizeOptions{
		RenameFiles: true,
	})
	if err == nil {
		t.Fatal("BuildOrganizationPlan() accepted an empty rename template")
	}
	if _, err := BuildOrganizationPlan(t.TempDir(), nil, nil, settings.OrganizeOptions{}); err == nil {
		t.Fatal("BuildOrganizationPlan() accepted a missing local scan")
	}
}

func TestBuildOrganizationPlanUsesFilenameMetadataWithoutRemoteTitle(t *testing.T) {
	base := t.TempDir()
	sourceFolder := filepath.Join(base, "incoming")
	mustMkdir(t, sourceFolder)
	fileName := "Game [0100E95004039000][v0].nsp"
	writeFixture(t, filepath.Join(sourceFolder, fileName), "base")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: fileName}, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	options := settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_ID}"}
	plan, err := BuildOrganizationPlan(base, local, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 0 || len(plan.Moves) != 1 || filepath.Base(plan.Moves[0].Destination) != "0100E95004039000.nsp" {
		t.Fatalf("filename metadata plan = %#v", plan)
	}
}

func TestBuildOrganizationPlanUsesV0DLCDisplayVersionFallback(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	writeFixture(t, filepath.Join(source, "dlc.nsp"), "synthetic DLC")
	dlcID := "0100E9500403A001"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			Dlc: map[string]db.SwitchFileInfo{
				dlcID: {
					ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "dlc.nsp"},
					Metadata:     metadataForID(dlcID, 0),
				},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100e95004039": {Attributes: db.TitleAttributes{Name: "Game"}},
	}}
	options := settings.OrganizeOptions{
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_NAME} [v{VERSION}][v{VERSION_TXT}]",
		ProcessWhenMissingBaseGame: true,
	}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(source, "Game [v0][v1.0.0].nsp")
	if len(plan.Moves) != 1 || plan.Moves[0].Destination != want {
		t.Fatalf("moves = %#v, want destination %q", plan.Moves, want)
	}
}

func TestBuildOrganizationPlanBlocksOccupiedV0DLCFallbackDestination(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	sourcePath := filepath.Join(source, "dlc.nsp")
	destinationPath := filepath.Join(source, "Game [v0][v1.0.0].nsp")
	writeFixture(t, sourcePath, "synthetic DLC")
	writeFixture(t, destinationPath, "existing destination")
	dlcID := "0100E9500403A001"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			Dlc: map[string]db.SwitchFileInfo{
				dlcID: {
					ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: filepath.Base(sourcePath)},
					Metadata:     metadataForID(dlcID, 0),
				},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100e95004039": {Attributes: db.TitleAttributes{Name: "Game"}},
	}}
	options := settings.OrganizeOptions{
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_NAME} [v{VERSION}][v{VERSION_TXT}]",
		ProcessWhenMissingBaseGame: true,
	}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].Destination != destinationPath {
		t.Fatalf("conflicts = %#v, want occupied destination %q", plan.Conflicts, destinationPath)
	}
	if err := ExecuteOrganizationPlan(plan, local, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted an occupied v0 DLC destination")
	}
	if content, err := os.ReadFile(sourcePath); err != nil || string(content) != "synthetic DLC" {
		t.Fatalf("source changed during planning: content=%q err=%v", content, err)
	}
	if content, err := os.ReadFile(destinationPath); err != nil || string(content) != "existing destination" {
		t.Fatalf("destination changed during planning: content=%q err=%v", content, err)
	}
}

func TestBuildOrganizationPlanRejectsInvalidSplitModel(t *testing.T) {
	base := t.TempDir()
	sourceFolder := filepath.Join(base, "split")
	mustMkdir(t, sourceFolder)
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			IsSplit:   true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: "game.nsp"}, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	options := settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"}
	if _, err := BuildOrganizationPlan(base, local, nil, options); err == nil {
		t.Fatal("BuildOrganizationPlan() accepted a split record without a part-00 filename")
	}
	local.TitlesMap["0100E95004039"].File.ExtendedInfo.BaseFolder = filepath.Join(base, "missing")
	local.TitlesMap["0100E95004039"].File.ExtendedInfo.FileName = "game.00"
	if _, err := BuildOrganizationPlan(base, local, nil, options); err == nil {
		t.Fatal("BuildOrganizationPlan() accepted a missing split directory")
	}
}

func TestAddSplitMovesFiltersPartsAndNonnumericFiles(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "split")
	destination := filepath.Join(base, "organized")
	mustMkdir(t, source)
	for name, content := range map[string]string{
		"game.00":  "part 0",
		"game.01":  "part 1",
		"game.xx":  "not a part",
		"other.02": "other file",
	} {
		writeFixture(t, filepath.Join(source, name), content)
	}
	var plan OrganizationPlan
	if err := addSplitMoves(&plan, map[string]struct{}{}, "game.00", source, destination); err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 2 {
		t.Fatalf("split moves = %#v, want only parts 00 and 01", plan.Moves)
	}
	if err := addSplitMoves(&plan, map[string]struct{}{}, "game.00", filepath.Join(base, "missing"), destination); err == nil {
		t.Fatal("addSplitMoves() accepted a missing split directory")
	}
}

func TestValidateOrganizationPlanCatchesDestinationThatIsAnotherSource(t *testing.T) {
	base := t.TempDir()
	first := filepath.Join(base, "first.nsp")
	second := filepath.Join(base, "second.nsp")
	third := filepath.Join(base, "third.nsp")
	for _, path := range []string{first, second} {
		writeFixture(t, path, "fixture")
	}
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	plan := OrganizationPlan{Moves: []OrganizationMove{
		{Source: first, Destination: second, SourceSize: firstInfo.Size(), SourceStamp: firstInfo},
		{Source: second, Destination: third, SourceSize: secondInfo.Size(), SourceStamp: secondInfo},
	}}
	validateOrganizationPlan(&plan)
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].Reason != "destination belongs to another planned source" {
		t.Fatalf("planned-source conflict = %#v", plan.Conflicts)
	}
}

func TestAddOrganizationMoveReportsInvalidDestinationPath(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source.nsp")
	writeFixture(t, source, "source")
	var plan OrganizationPlan
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, filepath.Join(base, "invalid\x00destination.nsp")); err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || !strings.Contains(plan.Conflicts[0].Reason, "cannot inspect destination") {
		t.Fatalf("invalid destination conflict = %#v", plan.Conflicts)
	}
}

func TestExecuteOrganizationPlanRechecksSourceAndLateDestination(t *testing.T) {
	base := t.TempDir()
	sourceFolder := filepath.Join(base, "incoming")
	mustMkdir(t, sourceFolder)
	source := filepath.Join(sourceFolder, "original.nsp")
	writeFixture(t, source, "before")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: "original.nsp"}, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Game"}},
	}}
	options := settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_NAME}"}
	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil || len(plan.Conflicts) != 0 {
		t.Fatalf("BuildOrganizationPlan() = %#v, %v", plan, err)
	}
	writeFixture(t, source, "source changed after preflight")
	if err := ExecuteOrganizationPlan(plan, nil, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() moved a source changed after preflight")
	}
	assertExists(t, source)
	if err := ExecuteOrganizationPlan(OrganizationPlan{}, nil, base, options, nil); err != nil {
		t.Fatalf("ExecuteOrganizationPlan(empty) = %v", err)
	}

	writeFixture(t, source, "before second plan")
	plan, err = BuildOrganizationPlan(base, local, remote, options)
	if err != nil || len(plan.Conflicts) != 0 {
		t.Fatalf("second BuildOrganizationPlan() = %#v, %v", plan, err)
	}
	destination := filepath.Join(sourceFolder, "Game.nsp")
	writeFixture(t, destination, "appeared after preflight")
	if err := ExecuteOrganizationPlan(plan, nil, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() overwrote a late destination")
	}
	assertExists(t, source)
	content, err := os.ReadFile(destination)
	if err != nil || string(content) != "appeared after preflight" {
		t.Fatalf("late destination changed: content=%q err=%v", content, err)
	}
}

func TestExecuteOrganizationPlanReportsMissingSourceBlockedDirectoryAndCleanupErrors(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "missing.nsp")
	if err := ExecuteOrganizationPlan(OrganizationPlan{Moves: []OrganizationMove{{
		Source: missing, Destination: filepath.Join(base, "target.nsp"),
	}}}, nil, base, settings.OrganizeOptions{}, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted a disappeared source")
	}

	source := filepath.Join(base, "source.nsp")
	writeFixture(t, source, "source")
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	blockedParent := filepath.Join(base, "blocked")
	writeFixture(t, blockedParent, "not a directory")
	blockedPlan := OrganizationPlan{Moves: []OrganizationMove{{
		Source: source, Destination: filepath.Join(blockedParent, "target.nsp"), SourceSize: info.Size(), SourceStamp: info,
	}}}
	if err := ExecuteOrganizationPlan(blockedPlan, nil, base, settings.OrganizeOptions{}, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() created a child beneath a file")
	}
	assertExists(t, source)

	cleanupSource := filepath.Join(base, "cleanup-source.nsp")
	cleanupDestination := filepath.Join(base, "cleanup-destination.nsp")
	writeFixture(t, cleanupSource, "cleanup")
	cleanupInfo, err := os.Stat(cleanupSource)
	if err != nil {
		t.Fatal(err)
	}
	cleanupPlan := OrganizationPlan{Moves: []OrganizationMove{{
		Source: cleanupSource, Destination: cleanupDestination, SourceSize: cleanupInfo.Size(), SourceStamp: cleanupInfo,
	}}}
	if err := ExecuteOrganizationPlan(cleanupPlan, nil, filepath.Join(base, "missing-cleanup-root"), settings.OrganizeOptions{DeleteEmptyFolders: true}, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() ignored empty-folder cleanup failure")
	}
	assertExists(t, cleanupDestination)
}

func TestUpdateLocalFilePathsReportsMissingMovedFile(t *testing.T) {
	missingDestination := filepath.Join(t.TempDir(), "missing.nsp")
	if err := updateLocalFilePaths(&db.LocalSwitchFilesDB{}, "source.nsp", missingDestination); err == nil {
		t.Fatal("updateLocalFilePaths() accepted a missing moved file")
	}
}

func TestDeleteEmptyFoldersPreservesRootAndRemovesEmptyChildren(t *testing.T) {
	root := filepath.Join(t.TempDir(), "library")
	child := filepath.Join(root, "empty", "nested")
	mustMkdir(t, child)
	if err := deleteEmptyFolders(root); err != nil {
		t.Fatal(err)
	}
	assertExists(t, root)
	assertNotExists(t, filepath.Join(root, "empty"))
}
