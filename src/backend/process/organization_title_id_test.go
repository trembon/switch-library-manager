package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestOrganizationPreservesTitleIDsWithoutCatalogBase(t *testing.T) {
	const (
		baseID       = "0100F07026DA8000"
		updateID     = "0100F07026DA8800"
		key          = "0100f07026da8"
		baseName     = "SiN Reloaded [0100F07026DA8000][v0].nsp"
		updateName   = "SiN Reloaded [0100F07026DA8800][v65536].nsp"
		updateTitles = `{"0100F07026DA8800":{"id":"0100F07026DA8800","name":null,"version":65536}}`
		versions     = `{"0100f07026da8000":{"65536":"2026-10-16"}}`
	)
	for _, test := range []struct {
		name          string
		titles        string
		fileName      string
		localBaseID   string
		createFolders bool
		wantMoves     int
	}{
		{name: "update-only catalog keeps matching filenames", titles: updateTitles, fileName: baseName},
		{name: "update-only catalog renames abbreviated filename", titles: updateTitles, fileName: "S.nsp", wantMoves: 1},
		{name: "update-only catalog fills folder title ID", titles: updateTitles, fileName: "S.nsp", createFolders: true, wantMoves: 2},
		{name: "no catalog", fileName: "S.nsp", wantMoves: 1},
		{name: "empty catalog", titles: `{}`, fileName: "S.nsp", wantMoves: 1},
		{name: "DLC-only catalog", titles: `{"0100F07026DA9001":{"id":"0100F07026DA9001","name":"Expansion"}}`, fileName: "S.nsp", wantMoves: 1},
		{
			name:        "populated catalog ID takes precedence over local metadata",
			titles:      `{"0100F07026DA8000":{"id":"0100F07026DA8000","name":"SiN Reloaded"}}`,
			fileName:    "S.nsp",
			localBaseID: "0100F07026DAA000",
			wantMoves:   1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			baseSource := filepath.Join(root, test.fileName)
			updateSource := filepath.Join(root, updateName)
			writeFixture(t, baseSource, "synthetic base")
			writeFixture(t, updateSource, "synthetic update")

			localID := test.localBaseID
			if localID == "" {
				localID = baseID
			}
			metadata := contentMetadata(strings.ToLower(localID), 0, "1.0.0")
			metadata.Ncap.TitleName["AmericanEnglish"] = switchfs.NacpTitle{Title: "SiN Reloaded"}
			local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
				key: {
					BaseExist: true,
					File:      db.SwitchFileInfo{ExtendedInfo: extendedInfoForPath(t, baseSource), Metadata: metadata},
					Updates: map[int]db.SwitchFileInfo{65536: {
						ExtendedInfo: extendedInfoForPath(t, updateSource),
						Metadata:     metadataForID(strings.ToLower(updateID), 65536),
					}},
				},
			}}
			var remote *db.SwitchTitlesDB
			if test.titles != "" {
				var err error
				remote, err = db.CreateSwitchTitleDB(strings.NewReader(test.titles), strings.NewReader(versions))
				if err != nil {
					t.Fatalf("create synthetic catalog: %v", err)
				}
			}
			options := settings.OrganizeOptions{
				RenameFiles:         true,
				FileNameTemplate:    "{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]",
				CreateFolderPerGame: test.createFolders,
				FolderNameTemplate:  "{TITLE_NAME} [{TITLE_ID}]",
			}
			destinationFolder := root
			if test.createFolders {
				destinationFolder = filepath.Join(root, "SiN Reloaded ["+baseID+"]")
			}
			wantPaths := map[string]string{
				"game":   filepath.Join(destinationFolder, baseName),
				"update": filepath.Join(destinationFolder, updateName),
			}
			preview, err := BuildOrganizationPreview(root, options, local, remote)
			if err != nil {
				t.Fatalf("build preview: %v", err)
			}
			for kind, want := range wantPaths {
				found := false
				for _, entry := range preview {
					if entry.Kind == kind {
						found = true
						if got := filepath.Join(root, entry.Path); got != want {
							t.Errorf("%s preview path = %q, want %q", kind, got, want)
						}
					}
				}
				if !found {
					t.Errorf("preview missing %s entry", kind)
				}
			}

			plan, err := BuildOrganizationPlan(root, local, remote, options)
			if err != nil {
				t.Fatalf("build organization plan: %v", err)
			}
			if len(plan.Conflicts) != 0 || len(plan.Moves) != test.wantMoves {
				t.Fatalf("organization plan = %#v, want %d moves without conflicts", plan, test.wantMoves)
			}
			for _, move := range plan.Moves {
				want := wantPaths["game"]
				if move.Source == updateSource {
					want = wantPaths["update"]
				}
				if move.Destination != want {
					t.Fatalf("planned destination = %q, want %q", move.Destination, want)
				}
			}
			if err := ExecuteOrganizationPlan(plan, local, root, options, nil); err != nil {
				t.Fatalf("execute organization plan: %v", err)
			}
			for kind, content := range map[string]string{"game": "synthetic base", "update": "synthetic update"} {
				got, err := os.ReadFile(wantPaths[kind])
				if err != nil || string(got) != content {
					t.Errorf("%s file content = %q, err = %v, want %q", kind, got, err, content)
				}
			}
			for _, move := range plan.Moves {
				assertNotExists(t, move.Source)
			}
			secondPlan, err := BuildOrganizationPlan(root, local, remote, options)
			if err != nil {
				t.Fatalf("build second organization plan: %v", err)
			}
			if len(secondPlan.Conflicts) != 0 || len(secondPlan.Moves) != 0 {
				t.Fatalf("second organization plan = %#v, want no further moves", secondPlan)
			}
		})
	}
}
