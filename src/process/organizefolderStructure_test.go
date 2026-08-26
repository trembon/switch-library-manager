package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/db"
	"github.com/trembon/switch-library-manager/settings"
	"github.com/trembon/switch-library-manager/switchfs"
	"go.uber.org/zap"
)

type progressEvent struct {
	curr    int
	total   int
	message string
}

type progressRecorder struct {
	events []progressEvent
}

func (p *progressRecorder) UpdateProgress(curr int, total int, message string) {
	p.events = append(p.events, progressEvent{curr: curr, total: total, message: message})
}

func TestApplyTemplate(t *testing.T) {
	data := map[string]string{
		settings.TEMPLATE_TITLE_NAME:  "The Game / Demo",
		settings.TEMPLATE_TITLE_ID:    "0100abcd00001000",
		settings.TEMPLATE_VERSION:     "12",
		settings.TEMPLATE_VERSION_TXT: "1.2.0",
		settings.TEMPLATE_REGION:      "US",
		settings.TEMPLATE_TYPE:        "UPD",
		settings.TEMPLATE_DLC_NAME:    "The Game - Extra Pack",
	}

	tests := []struct {
		name     string
		safe     bool
		template string
		try      int
		want     string
	}{
		{"all placeholders and repeated values", false, "{TITLE_NAME}-{TITLE_NAME}-{TITLE_ID}-{VERSION}-{VERSION_TXT}-{REGION}-{TYPE}-{DLC_NAME}", 0, "The Game  Demo-The Game  Demo-0100ABCD00001000-12-1.2.0-US-UPD-The Game - Extra Pack"},
		{"safe name", true, "Pokémon™: {TITLE_NAME} [v{VERSION}]", 0, "Pokémon The Game Demo [v12]"},
		{"collision suffix", false, "name.", 2, "name(2)"},
		{"cleanup and illegal characters", false, "[]()<>a:b%?", 0, "ab"},
		{"trailing period", false, "name...", 0, "name.."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyTemplate(data, tt.safe, tt.template, tt.try); got != tt.want {
				t.Fatalf("applyTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTemplateHelpers(t *testing.T) {
	options := settings.OrganizeOptions{
		RenameFiles:        true,
		FolderNameTemplate: "{TITLE_NAME}",
		FileNameTemplate:   "{TITLE_ID}-{VERSION}",
	}
	data := map[string]string{settings.TEMPLATE_TITLE_NAME: "Game", settings.TEMPLATE_TITLE_ID: "abcd", settings.TEMPLATE_VERSION: "3"}

	if got := getFolderName(options, data); got != "Game" {
		t.Fatalf("getFolderName() = %q", got)
	}
	if got := getFileName(options, "original.nsp", data, 0); got != "ABCD-3.nsp" {
		t.Fatalf("getFileName() = %q", got)
	}
	options.RenameFiles = false
	if got := getFileName(options, "original.nsp", data, 4); got != "original.nsp" {
		t.Fatalf("getFileName() with rename disabled = %q", got)
	}
}

func TestIsOptionsValid(t *testing.T) {
	tests := []struct {
		name string
		opt  settings.OrganizeOptions
		want bool
	}{
		{"defaults", settings.OrganizeOptions{}, true},
		{"rename requires a template", settings.OrganizeOptions{RenameFiles: true}, false},
		{"rename requires title identity", settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{VERSION}"}, false},
		{"valid file template", settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_ID}"}, true},
		{"folder requires a template", settings.OrganizeOptions{CreateFolderPerGame: true}, false},
		{"folder requires title identity", settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{REGION}"}, false},
		{"valid folder template", settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"}, true},
		{"both valid", settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_NAME}", CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_ID}"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOptionsValid(tt.opt); got != tt.want {
				t.Fatalf("IsOptionsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTitleNameAndDLCNameHelpers(t *testing.T) {
	metadata := contentMetadata("0100abcd00001000", 3, "3.0.0")
	metadata.Ncap.TitleName["AmericanEnglish"] = switchfs.NacpTitle{Title: "Name From NACP"}
	local := &db.SwitchGameFiles{File: db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "Name From File [0100abcd00001000][v3].nsp"}, Metadata: metadata}}

	remote := &db.SwitchTitle{Attributes: db.TitleAttributes{Id: metadata.TitleId, Name: "Remote Name"}, Dlc: map[string]db.TitleAttributes{
		"0100abcd00001a01": {Name: "Remote Name - Expansion\nPack"},
	}}
	if got := getTitleName(remote, local); got != "Remote Name" {
		t.Fatalf("remote title name = %q", got)
	}
	remote.Attributes.Name = "日本語"
	if got := getTitleName(remote, local); got != "Name From NACP" {
		t.Fatalf("NACP title name = %q", got)
	}
	local.File.Metadata.Ncap.TitleName = nil
	if got := getTitleName(nil, local); got != "Name From File " {
		t.Fatalf("filename title name = %q", got)
	}
	if got := getTitleName(nil, nil); got != "Unknown Title" {
		t.Fatalf("nil title name = %q", got)
	}

	if got := getDlcName(remote, db.SwitchFileInfo{Metadata: metadataForID("0100abcd00001a01", 1)}); got != "Remote Name - Expansion Pack" {
		t.Fatalf("DLC name = %q", got)
	}
	if got := getDlcName(remote, db.SwitchFileInfo{Metadata: metadataForID("missing", 1)}); got != "" {
		t.Fatalf("unknown DLC name = %q", got)
	}
	if got := getDlcName(nil, db.SwitchFileInfo{}); got != "" {
		t.Fatalf("nil DLC name = %q", got)
	}
}

func TestOrganizeByFoldersMovesBaseUpdateAndDLC(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	mustMkdir(t, source)

	baseID := "0100E95004039000"
	updateID := "0100E95004039800"
	dlcOneID := "0100E9500403A001"
	dlcTwoID := "0100E9500403A002"
	baseName := "base.nsp"
	updateName := "update.nsp"
	dlcOneName := "dlc-one.nsp"
	dlcTwoName := "dlc-two.nsp"
	for _, name := range []string{baseName, updateName, dlcOneName, dlcTwoName} {
		writeFixture(t, filepath.Join(source, name), name)
	}

	options := settings.OrganizeOptions{
		CreateFolderPerGame: true,
		FolderNameTemplate:  "{TITLE_NAME}",
		RenameFiles:         true,
		FileNameTemplate:    "{TITLE_ID}_{TYPE}_{VERSION}_{VERSION_TXT}",
		UpdatesFolder:       "updates",
		DlcFolder:           "dlc",
		SwitchSafeFileNames: false,
		DeleteEmptyFolders:  true,
	}
	setProcessSettings(t, baseFolder, options)

	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: baseName}, Metadata: contentMetadata(baseID, 0, "1.0.0")},
			BaseExist: true,
			Updates:   map[int]db.SwitchFileInfo{5: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: updateName}, Metadata: contentMetadata(updateID, 5, "5.0.0")}},
			Dlc: map[string]db.SwitchFileInfo{
				dlcOneID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcOneName}, Metadata: metadataForID(dlcOneID, 1)},
				dlcTwoID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcTwoName}, Metadata: metadataForID(dlcTwoID, 1)},
			},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: baseID, Name: "Test Game", Region: "US"}, Dlc: map[string]db.TitleAttributes{
			dlcOneID: {Id: dlcOneID, Name: "Test Game - Expansion"},
			dlcTwoID: {Id: dlcTwoID, Name: "Test Game - Expansion"},
		}},
	}}

	var progress progressRecorder
	OrganizeByFolders(baseFolder, local, remote, &progress)

	destination := filepath.Join(baseFolder, "Test Game")
	assertMoved(t, filepath.Join(source, baseName), filepath.Join(destination, baseID+"__0_1.0.0.nsp"), "base")
	assertMoved(t, filepath.Join(source, updateName), filepath.Join(destination, "updates", updateID+"_UPD_5_5.0.0.nsp"), "update")
	assertMoved(t, filepath.Join(source, dlcOneName), filepath.Join(destination, "dlc", dlcOneID+"_DLC_1_.nsp"), "first DLC")
	assertMoved(t, filepath.Join(source, dlcTwoName), filepath.Join(destination, "dlc", dlcTwoID+"_DLC_1_.nsp"), "second DLC")
	if len(progress.events) == 0 || progress.events[len(progress.events)-1].message != "done" {
		t.Fatalf("organization progress did not finish: %#v", progress.events)
	}

	OrganizeByFolders(baseFolder, local, remote, nil)
}

func TestOrganizeByFoldersProcessesMissingBaseAndNilMetadata(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	mustMkdir(t, source)
	updateID := "0100E95004039800"
	dlcID := "0100E9500403A001"
	updateName := "Missing Base [0100E95004039000][v7].nsp"
	dlcName := "missing-dlc.nsp"
	writeFixture(t, filepath.Join(source, updateName), "update")
	writeFixture(t, filepath.Join(source, dlcName), "dlc")
	options := settings.OrganizeOptions{
		CreateFolderPerGame:        true,
		FolderNameTemplate:         "{TITLE_NAME}",
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_ID}_{TYPE}_{VERSION}",
		UpdatesFolder:              "updates",
		DlcFolder:                  "dlc",
		ProcessWhenMissingBaseGame: true,
		SwitchSafeFileNames:        false,
	}
	setProcessSettings(t, baseFolder, options)

	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "missing-base.nsp"}},
			Updates:   map[int]db.SwitchFileInfo{7: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: updateName}, Metadata: metadataForID(updateID, 7)}},
			Dlc:       map[string]db.SwitchFileInfo{dlcID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcName}}},
			BaseExist: false,
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Missing Base"}},
	}}
	OrganizeByFolders(baseFolder, local, remote, nil)

	assertMoved(t, filepath.Join(source, updateName), filepath.Join(baseFolder, "Missing Base", "updates", updateID+"_UPD_7.nsp"), "missing-base update")
	assertMoved(t, filepath.Join(source, dlcName), filepath.Join(baseFolder, "Missing Base", "dlc", dlcID+"_DLC_0.nsp"), "metadata-free DLC")
}

func TestOrganizeByFoldersWithoutGameFoldersAndWithDlcCollision(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	updatesFolder := filepath.Join(baseFolder, "updates")
	dlcFolder := filepath.Join(baseFolder, "dlc")
	mustMkdir(t, source)
	mustMkdir(t, updatesFolder)
	mustMkdir(t, dlcFolder)
	baseName := "base.nsp"
	updateName := "update.nsp"
	dlcOneName := "one.nsp"
	dlcTwoName := "two.nsp"
	for _, name := range []string{baseName, updateName, dlcOneName, dlcTwoName} {
		writeFixture(t, filepath.Join(source, name), name)
	}
	baseID := "0100E95004039000"
	updateID := "0100E95004039800"
	dlcOneID := "0100E9500403A001"
	dlcTwoID := "0100E9500403A002"
	setProcessSettings(t, baseFolder, settings.OrganizeOptions{
		RenameFiles:      true,
		FileNameTemplate: "{TITLE_ID}_{DLC_NAME}",
		UpdatesFolder:    updatesFolder,
		DlcFolder:        dlcFolder,
	})
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: baseName}, Metadata: contentMetadata(baseID, 0, "")},
			Updates:   map[int]db.SwitchFileInfo{3: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: updateName}, Metadata: metadataForID(updateID, 3)}},
			Dlc: map[string]db.SwitchFileInfo{
				dlcOneID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcOneName}, Metadata: metadataForID(dlcOneID, 1)},
				dlcTwoID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: dlcTwoName}, Metadata: metadataForID(dlcTwoID, 1)},
			},
		},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{"0100E95004039": {
		Attributes: db.TitleAttributes{Id: baseID, Name: "Game"},
		Dlc: map[string]db.TitleAttributes{
			dlcOneID: {Id: dlcOneID, Name: "Game - Expansion"},
			dlcTwoID: {Id: dlcTwoID, Name: "Game - Expansion"},
		},
	}}}
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertMoved(t, filepath.Join(source, baseName), filepath.Join(source, baseID+"_.nsp"), "base without folder")
	assertMoved(t, filepath.Join(source, updateName), filepath.Join(updatesFolder, updateID+"_.nsp"), "update outside folder")
	assertMoved(t, filepath.Join(source, dlcOneName), filepath.Join(dlcFolder, dlcOneID+"_Expansion.nsp"), "first colliding DLC")
	assertMoved(t, filepath.Join(source, dlcTwoName), filepath.Join(dlcFolder, dlcTwoID+"_Expansion.nsp"), "second colliding DLC")

	setProcessSettings(t, baseFolder, settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"})
	secondSource := filepath.Join(baseFolder, "bad-source")
	mustMkdir(t, secondSource)
	writeFixture(t, filepath.Join(secondSource, "bad.nsp"), "bad")
	writeFixture(t, filepath.Join(baseFolder, "occupied"), "not a folder")
	local.TitlesMap["0100E95004039"].File.ExtendedInfo = db.ExtendedFileInfo{BaseFolder: secondSource, FileName: "bad.nsp"}
	remote.TitlesMap["0100E95004039"].Attributes.Name = "occupied"
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertExists(t, filepath.Join(secondSource, "bad.nsp"))
}

func TestOrganizeByFoldersSplitFilesAndMissingBaseSkip(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "split-incoming")
	mustMkdir(t, source)
	writeFixture(t, filepath.Join(source, "game.00"), "part 0")
	writeFixture(t, filepath.Join(source, "game.01"), "part 1")
	writeFixture(t, filepath.Join(source, "keep.txt"), "keep")
	setProcessSettings(t, baseFolder, settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"})
	file := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "game.00"}, Metadata: contentMetadata("0100E95004039000", 0, "")}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"split": {File: file, BaseExist: true, IsSplit: true},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"split": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Split"}},
	}}
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertMoved(t, filepath.Join(source, "game.00"), filepath.Join(baseFolder, "Split", "game.00"), "split part zero")
	assertMoved(t, filepath.Join(source, "game.01"), filepath.Join(baseFolder, "Split", "game.01"), "split part one")
	assertExists(t, filepath.Join(source, "keep.txt"))
	missingSource := filepath.Join(baseFolder, "missing")
	mustMkdir(t, missingSource)
	writeFixture(t, filepath.Join(missingSource, "missing.nsp"), "missing")
	missing := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: missingSource, FileName: "missing.nsp"}}
	local = &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{"missing": {File: missing, BaseExist: false}}}
	remote = &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{"missing": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Missing"}}}}
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertExists(t, filepath.Join(missingSource, "missing.nsp"))
}

func TestOrganizeByFoldersSkipsMultiContentRecordsAndInvalidOptions(t *testing.T) {
	baseFolder := t.TempDir()
	source := filepath.Join(baseFolder, "incoming")
	mustMkdir(t, source)
	name := "multi.nsp"
	writeFixture(t, filepath.Join(source, name), "multi")
	options := settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"}
	setProcessSettings(t, baseFolder, options)
	file := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: name}, Metadata: contentMetadata("0100E95004039000", 0, "1")}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {File: file, BaseExist: true, MultiContent: true, Updates: map[int]db.SwitchFileInfo{1: file}, Dlc: map[string]db.SwitchFileInfo{"dlc": file}},
	}, Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Multi"}}}}
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertMoved(t, filepath.Join(source, name), filepath.Join(baseFolder, "Multi", name), "multi-content base")

	setProcessSettings(t, baseFolder, settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{VERSION}"})
	OrganizeByFolders(baseFolder, local, remote, nil)
	assertExists(t, filepath.Join(baseFolder, "Multi", name))
}

func TestDeleteOldUpdates(t *testing.T) {
	baseFolder := t.TempDir()
	oldFolder := filepath.Join(baseFolder, "old")
	mustMkdir(t, oldFolder)
	oldPath := filepath.Join(oldFolder, "old.nsp")
	newPath := filepath.Join(oldFolder, "new.nsp")
	writeFixture(t, oldPath, "old")
	writeFixture(t, newPath, "new")
	setProcessSettings(t, baseFolder, settings.OrganizeOptions{DeleteEmptyFolders: true})
	var progress progressRecorder
	local := &db.LocalSwitchFilesDB{Skipped: map[db.ExtendedFileInfo]db.SkippedFile{
		{BaseFolder: oldFolder, FileName: "old.nsp"}:             {ReasonCode: db.REASON_OLD_UPDATE},
		{BaseFolder: oldFolder, FileName: "already-missing.nsp"}: {ReasonCode: db.REASON_OLD_UPDATE},
		{BaseFolder: oldFolder, FileName: "new.nsp"}:             {ReasonCode: db.REASON_DUPLICATE},
	}}
	DeleteOldUpdates(baseFolder, local, &progress)
	assertNotExists(t, oldPath)
	assertExists(t, newPath)
	assertExists(t, oldFolder)
	if len(progress.events) != 4 {
		t.Fatalf("progress events = %d, want 4: %#v", len(progress.events), progress.events)
	}

	DeleteOldUpdates(baseFolder, &db.LocalSwitchFilesDB{Skipped: map[db.ExtendedFileInfo]db.SkippedFile{}}, nil)
	cleanupFolder := filepath.Join(baseFolder, "cleanup")
	mustMkdir(t, cleanupFolder)
	writeFixture(t, filepath.Join(cleanupFolder, "old.nsp"), "old")
	DeleteOldUpdates(baseFolder, &db.LocalSwitchFilesDB{Skipped: map[db.ExtendedFileInfo]db.SkippedFile{
		{BaseFolder: cleanupFolder, FileName: "old.nsp"}: {ReasonCode: db.REASON_OLD_UPDATE},
	}}, nil)
	assertNotExists(t, cleanupFolder)
}

func TestFilesystemHelpers(t *testing.T) {
	baseFolder := t.TempDir()
	created := filepath.Join(baseFolder, "created")
	logger := zap.NewNop().Sugar()
	if err := createFolder(created, logger); err != nil {
		t.Fatalf("createFolder() = %v", err)
	}
	if err := createFolder(created, logger); err != nil {
		t.Fatalf("createFolder() existing = %v", err)
	}
	nonEmpty := filepath.Join(baseFolder, "non-empty")
	mustMkdir(t, nonEmpty)
	writeFixture(t, filepath.Join(nonEmpty, "file"), "file")
	if err := deleteEmptyFolder(nonEmpty); err != nil {
		t.Fatalf("deleteEmptyFolder(non-empty) = %v", err)
	}
	if err := deleteEmptyFolder(created); err != nil {
		t.Fatalf("deleteEmptyFolder(empty) = %v", err)
	}
	assertNotExists(t, created)
	if err := deleteEmptyFolder(filepath.Join(baseFolder, "missing")); err == nil {
		t.Fatal("deleteEmptyFolder(missing) returned nil")
	}

	from := filepath.Join(baseFolder, "from")
	to := filepath.Join(baseFolder, "to")
	writeFixture(t, from, "move")
	if err := moveFile(from, from); err != nil {
		t.Fatalf("moveFile(same path) = %v", err)
	}
	if err := moveFile(from, to); err != nil {
		t.Fatalf("moveFile() = %v", err)
	}
	assertExists(t, to)
	if err := moveFile(from, filepath.Join(baseFolder, "missing", "to")); err == nil {
		t.Fatal("moveFile(missing destination) returned nil")
	}
	if err := deleteEmptyFolders(filepath.Join(baseFolder, "not-there")); err == nil {
		t.Fatal("deleteEmptyFolders(missing) returned nil")
	}
}

func setProcessSettings(t *testing.T, baseFolder string, options settings.OrganizeOptions) {
	t.Helper()
	settings.SaveSettings(&settings.AppSettings{OrganizeOptions: options}, baseFolder)
}

func contentMetadata(id string, version int, displayVersion string) *switchfs.ContentMetaAttributes {
	return &switchfs.ContentMetaAttributes{TitleId: id, Version: version, Ncap: &switchfs.Nacp{
		DisplayVersion: displayVersion,
		TitleName:      map[string]switchfs.NacpTitle{},
	}}
}

func metadataForID(id string, version int) *switchfs.ContentMetaAttributes {
	return &switchfs.ContentMetaAttributes{TitleId: id, Version: version}
}

func writeFixture(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("write fixture %q: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func assertMoved(t *testing.T, source, destination, label string) {
	t.Helper()
	assertNotExists(t, source)
	if _, err := os.Stat(destination); err != nil {
		entries, readErr := os.ReadDir(filepath.Dir(destination))
		t.Fatalf("%s destination %q is missing: %v (directory entries: %v, read error: %v)", label, destination, err, entries, readErr)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %q to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q not to exist, stat error = %v", path, err)
	}
}

func TestParseTitleNameFromFileName(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"Name [id].nsp", "Name "},
		{"Name.nsp", "Name.nsp"},
		{"[id].nsp", ""},
	} {
		if got := db.ParseTitleNameFromFileName(test.input); got != test.want {
			t.Errorf("ParseTitleNameFromFileName(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestTemplateOutputHasNoWhitespaceRuns(t *testing.T) {
	data := map[string]string{settings.TEMPLATE_TITLE_NAME: "  A\t B  "}
	if got := applyTemplate(data, false, "{TITLE_NAME}", 0); strings.ContainsAny(got, "\t\n") || got != "A B" {
		t.Fatalf("normalized template = %q", got)
	}
}
