package process

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestBuildOrganizationPreviewUsesActualBaseUpdateAndDLC(t *testing.T) {
	root := t.TempDir()
	options := settings.OrganizeOptions{
		CreateFolderPerGame: true,
		FolderNameTemplate:  "{TITLE_NAME}",
		RenameFiles:         true,
		FileNameTemplate:    "{TITLE_ID}_{TYPE}_{VERSION}",
		UpdatesFolder:       "updates",
		DlcFolder:           "dlc",
	}
	baseID := "0100E95004039000"
	updateID := "0100E95004039800"
	dlcID := "0100E9500403A001"
	gameKey := "0100e95004039"
	source := filepath.Join(root, "incoming")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		gameKey: {
			BaseExist: true,
			File: db.SwitchFileInfo{
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "base.nsp"},
				Metadata:     previewTestMetadata(baseID, 0, "1.0.0"),
			},
			Updates: map[int]db.SwitchFileInfo{5: {
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "update.nsp"},
				Metadata:     previewTestMetadata(updateID, 5, "5.0.0"),
			}},
			Dlc: map[string]db.SwitchFileInfo{dlcID: {
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "dlc.nsp"},
				Metadata:     previewTestMetadata(dlcID, 1, "1.0.0"),
			}},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		gameKey: {
			Attributes: db.TitleAttributes{Id: baseID, Name: "Preview Game", Region: "US"},
			Dlc:        map[string]db.TitleAttributes{dlcID: {Id: dlcID, Name: "Preview Game - Expansion"}},
		},
	}}

	got := BuildOrganizationPreview(root, options, local, remote)
	if len(got) != 3 {
		t.Fatalf("preview entries = %#v, want three entries", got)
	}
	want := map[string]string{
		"game":   filepath.Join("Preview Game", baseID+"_BASE_0.nsp"),
		"update": filepath.Join("Preview Game", "updates", updateID+"_UPD_5.nsp"),
		"dlc":    filepath.Join("Preview Game", "dlc", dlcID+"_DLC_1.nsp"),
	}
	for _, entry := range got {
		if entry.Path != want[entry.Kind] {
			t.Errorf("%s preview path = %q, want %q", entry.Kind, entry.Path, want[entry.Kind])
		}
	}
}

func TestBuildOrganizationPreviewFallsBackAndKeepsOriginalNamesWhenDisabled(t *testing.T) {
	root := t.TempDir()
	options := settings.OrganizeOptions{UpdatesFolder: "updates", DlcFolder: "dlc"}

	got := BuildOrganizationPreview(root, options, nil, nil)
	if len(got) != 3 {
		t.Fatalf("fallback preview entries = %#v, want three entries", got)
	}
	for _, entry := range got {
		if strings.Contains(entry.Path, "Example Adventure") {
			t.Errorf("disabled rename unexpectedly used title name in %s path: %q", entry.Kind, entry.Path)
		}
	}
	if got[0].Path != "example-adventure.nsp" {
		t.Errorf("fallback game path = %q, want original filename", got[0].Path)
	}
	if got[1].Path != filepath.Join("updates", "example-adventure-update.nsp") {
		t.Errorf("fallback update path = %q", got[1].Path)
	}
	if got[2].Path != filepath.Join("dlc", "example-adventure-dlc.nsp") {
		t.Errorf("fallback DLC path = %q", got[2].Path)
	}
}

func TestBuildOrganizationPreviewAppliesSafeNames(t *testing.T) {
	root := t.TempDir()
	options := settings.OrganizeOptions{
		CreateFolderPerGame: true,
		FolderNameTemplate:  "{TITLE_NAME}",
		RenameFiles:         true,
		FileNameTemplate:    "{TITLE_NAME}",
		SwitchSafeFileNames: true,
	}
	key := "0100e95004039"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{key: {
		BaseExist: true,
		File: db.SwitchFileInfo{
			ExtendedInfo: db.ExtendedFileInfo{BaseFolder: root, FileName: "source.nsp"},
			Metadata:     previewTestMetadata("0100E95004039000", 0, "1.0.0"),
		},
	}}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{key: {
		Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Pokémon: Demo"},
	}}}

	got := BuildOrganizationPreview(root, options, local, remote)
	if len(got) != 3 {
		t.Fatalf("safe-name preview entries = %#v, want fallback-filled entries", got)
	}
	if strings.ContainsAny(got[0].Path, ":?*<>|\"") {
		t.Errorf("safe-name preview contains an unsafe character: %q", got[0].Path)
	}
}

func previewTestMetadata(id string, version int, displayVersion string) *switchfs.ContentMetaAttributes {
	return &switchfs.ContentMetaAttributes{TitleId: id, Version: version, Ncap: &switchfs.Nacp{DisplayVersion: displayVersion}}
}
