package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestOrganizationGameNameOverrides(t *testing.T) {
	const id, key = "0100000000010000", "0100000000010"
	for _, missingBase := range []bool{false, true} {
		t.Run(map[bool]string{true: "missing base", false: "base"}[missingBase], func(t *testing.T) {
			root := t.TempDir()
			file := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: root, FileName: "Original.nsp"}, Metadata: previewTestMetadata(id, 0, "1.0")}
			if err := os.WriteFile(filepath.Join(root, file.ExtendedInfo.FileName), []byte("synthetic"), 0600); err != nil {
				t.Fatal(err)
			}
			game := &db.SwitchGameFiles{BaseExist: !missingBase, File: file, Updates: map[int]db.SwitchFileInfo{}}
			if missingBase {
				file.Metadata = previewTestMetadata("0100000000010800", 2, "2.0")
				game.Updates[2] = file
			}
			local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{key: game}}
			remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{key: {Attributes: db.TitleAttributes{Id: id, Name: "Catalog"}}}}
			names := map[string]string{id: "Custom 日本語"}
			options := settings.OrganizeOptions{RenameFiles: true, CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}", FileNameTemplate: "{TITLE_NAME} [{TITLE_ID}]", AllowUnicodeFileNames: true, ProcessWhenMissingBaseGame: true}
			plan, err := BuildOrganizationPlan(root, local, remote, options, names)
			if err != nil || len(plan.Moves) != 1 || len(plan.Conflicts) != 0 {
				t.Fatalf("plan = %#v, %v", plan, err)
			}
			if !strings.Contains(plan.Moves[0].Destination, "Custom 日本語") {
				t.Fatal(plan.Moves)
			}
			preview, err := BuildOrganizationPreview(root, options, local, remote, names)
			if err != nil {
				t.Fatal(err)
			}
			kind := "game"
			if missingBase {
				kind = "update"
			}
			for _, row := range preview {
				if row.Kind == kind && filepath.Join(root, row.Path) != plan.Moves[0].Destination {
					t.Fatalf("preview = %v, plan = %v", row, plan.Moves)
				}
			}
			if remote.TitlesMap[key].Attributes.Name != "Catalog" || game.File.ExtendedInfo.FileName != "Original.nsp" {
				t.Fatal("planning changed source data")
			}
			if err := os.MkdirAll(filepath.Dir(plan.Moves[0].Destination), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(plan.Moves[0].Destination, []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
			blocked, err := BuildOrganizationPlan(root, local, remote, options, names)
			if err != nil || len(blocked.Conflicts) == 0 {
				t.Fatalf("collision = %#v, %v", blocked, err)
			}
		})
	}
}

func TestOrganizationGameNameOverridePreservesDLCName(t *testing.T) {
	const baseID, key, dlcID = "0100000000010000", "0100000000010", "0100000000011001"
	for _, test := range []struct {
		name         string
		allowUnicode bool
	}{
		{name: "Unicode names", allowUnicode: true},
		{name: "safe names", allowUnicode: false},
	} {
		for _, template := range []struct {
			name      string
			value     string
			automatic string
			custom    string
		}{
			{name: "game and DLC names", value: "{TITLE_NAME} ({DLC_NAME})", automatic: "Catalog (Expansion Pack).nsp", custom: "Pack (Expansion Pack).nsp"},
			{name: "DLC name and ID", value: "{DLC_NAME}[{TITLE_ID}]", automatic: "Expansion Pack[0100000000011001].nsp", custom: "Expansion Pack[0100000000011001].nsp"},
		} {
			t.Run(test.name+"/"+template.name, func(t *testing.T) {
				root := t.TempDir()
				source := filepath.Join(root, "Original DLC.nsp")
				writeFixture(t, source, "synthetic DLC")
				game := &db.SwitchGameFiles{Dlc: map[string]db.SwitchFileInfo{
					dlcID: {
						ExtendedInfo: db.ExtendedFileInfo{BaseFolder: root, FileName: filepath.Base(source)},
						Metadata:     previewTestMetadata(dlcID, 0, "1.0"),
					},
				}}
				local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{key: game}}
				remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{key: {
					Attributes: db.TitleAttributes{Id: baseID, Name: "Catalog"},
					Dlc:        map[string]db.TitleAttributes{dlcID: {Id: dlcID, Name: "Catalog - Expansion Pack"}},
				}}}
				options := settings.OrganizeOptions{
					RenameFiles:                true,
					FileNameTemplate:           template.value,
					AllowUnicodeFileNames:      test.allowUnicode,
					ProcessWhenMissingBaseGame: true,
				}
				names := map[string]string{baseID: "Pack"}
				var plan OrganizationPlan
				for _, overrides := range []map[string]string{nil, names} {
					wantName := template.automatic
					if overrides != nil {
						wantName = template.custom
					}
					want := filepath.Join(root, wantName)
					var err error
					plan, err = BuildOrganizationPlan(root, local, remote, options, overrides)
					if err != nil || len(plan.Moves) != 1 || len(plan.Conflicts) != 0 || plan.Moves[0].Destination != want {
						t.Fatalf("plan = %#v, %v, want one move to %q", plan, err, want)
					}
					preview, err := BuildOrganizationPreview(root, options, local, remote, overrides)
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, row := range preview {
						if row.Kind == "dlc" {
							found = true
							if filepath.Join(root, row.Path) != want {
								t.Fatalf("DLC preview = %#v, want %q", row, want)
							}
						}
					}
					if !found {
						t.Fatal("preview omitted the DLC")
					}
				}
				if remote.TitlesMap[key].Attributes.Name != "Catalog" || remote.TitlesMap[key].Dlc[dlcID].Name != "Catalog - Expansion Pack" || game.Dlc[dlcID].ExtendedInfo.FileName != filepath.Base(source) {
					t.Fatal("planning changed source data")
				}

				destination := plan.Moves[0].Destination
				writeFixture(t, destination, "existing destination")
				blocked, err := BuildOrganizationPlan(root, local, remote, options, names)
				if err != nil || len(blocked.Conflicts) == 0 {
					t.Fatalf("occupied destination plan = %#v, %v", blocked, err)
				}
				if err := ExecuteOrganizationPlan(blocked, local, root, options, nil); err == nil {
					t.Fatal("execution accepted an occupied destination")
				}
				for path, want := range map[string]string{source: "synthetic DLC", destination: "existing destination"} {
					if content, err := os.ReadFile(path); err != nil || string(content) != want {
						t.Fatalf("file %q changed after conflict: content=%q, err=%v", path, content, err)
					}
				}
				if err := os.Remove(destination); err != nil {
					t.Fatal(err)
				}
				if err := ExecuteOrganizationPlan(plan, local, root, options, nil); err != nil {
					t.Fatal(err)
				}
				assertMoved(t, source, destination, "custom-named DLC")
				if content, err := os.ReadFile(destination); err != nil || string(content) != "synthetic DLC" {
					t.Fatalf("organized DLC content = %q, err = %v", content, err)
				}
				second, err := BuildOrganizationPlan(root, local, remote, options, names)
				if err != nil || len(second.Moves) != 0 || len(second.Conflicts) != 0 {
					t.Fatalf("second organization = %#v, %v, want no moves or conflicts", second, err)
				}
			})
		}
	}
}

func TestDLCNameOnlyTemplateIgnoresGameNameOverride(t *testing.T) {
	for _, safe := range []bool{false, true} {
		for _, titleName := range []string{"Catalog", "Pack"} {
			data := map[string]string{
				automaticTitleNameKey:        "Catalog",
				settings.TEMPLATE_TITLE_NAME: titleName,
				settings.TEMPLATE_DLC_NAME:   "Catalog - Expansion Pack",
			}
			if got := applyTemplate(data, safe, "{DLC_NAME}", 0); got != "Expansion Pack" {
				t.Fatalf("DLC-only template with title %q and safe=%v = %q, want %q", titleName, safe, got, "Expansion Pack")
			}
		}
	}
}
