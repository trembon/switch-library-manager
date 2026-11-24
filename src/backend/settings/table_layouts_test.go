package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIsTableLayoutKeyAllowsOnlyKnownTables(t *testing.T) {
	for _, key := range []string{TableLayoutLibrary, TableLayoutMissingGames, TableLayoutMissingUpdates, TableLayoutMissingDLC, TableLayoutIssues} {
		if !IsTableLayoutKey(key) {
			t.Errorf("IsTableLayoutKey(%q) = false", key)
		}
	}
	for _, key := range []string{"", "Library", "settings", "../library"} {
		if IsTableLayoutKey(key) {
			t.Errorf("IsTableLayoutKey(%q) = true", key)
		}
	}
}

func TestNormalizeTableLayoutsSanitizesUntrustedValues(t *testing.T) {
	tooMany := TableLayout{}
	for i := 0; i < maxTableLayoutColumns+5; i++ {
		tooMany.Columns = append(tooMany.Columns, ColumnLayout{Field: "f" + strings.Repeat("x", i), Visible: true})
	}

	got := normalizeTableLayouts(map[string]TableLayout{
		TableLayoutLibrary: {Columns: []ColumnLayout{
			{Field: " name ", Visible: true, Width: 120},
			{Field: "name", Visible: false},
			{Field: "", Visible: true},
			{Field: "   ", Visible: true},
			{Field: strings.Repeat("a", maxTableLayoutFieldLength+1), Visible: true},
			{Field: "path", Visible: false, Width: -20},
			{Field: "icon", Visible: true, Width: maxTableLayoutColumnWidth + 1},
		}},
		TableLayoutIssues:     {Columns: []ColumnLayout{{Field: ""}}},
		"unknown":             {Columns: []ColumnLayout{{Field: "name", Visible: true}}},
		TableLayoutMissingDLC: tooMany,
	})

	wantLibrary := TableLayout{Columns: []ColumnLayout{
		{Field: "name", Visible: true, Width: 120},
		{Field: "path", Visible: false},
		{Field: "icon", Visible: true, Width: maxTableLayoutColumnWidth},
	}}
	if !reflect.DeepEqual(got[TableLayoutLibrary], wantLibrary) {
		t.Fatalf("library layout = %#v, want %#v", got[TableLayoutLibrary], wantLibrary)
	}
	if _, ok := got[TableLayoutIssues]; ok {
		t.Fatal("a layout without usable columns should be dropped")
	}
	if _, ok := got["unknown"]; ok {
		t.Fatal("a layout for an unknown table should be dropped")
	}
	if n := len(got[TableLayoutMissingDLC].Columns); n != maxTableLayoutColumns {
		t.Fatalf("missing DLC layout kept %d columns, want %d", n, maxTableLayoutColumns)
	}
	if normalizeTableLayouts(map[string]TableLayout{"unknown": {}}) != nil {
		t.Fatal("no remaining layouts should normalize to nil")
	}
}

func TestWithTableLayoutReplacesResetsAndLeavesCurrentUnchanged(t *testing.T) {
	current := defaultSettings()
	current.GUI.TableLayouts = map[string]TableLayout{
		TableLayoutIssues: {Columns: []ColumnLayout{{Field: "key", Visible: true}}},
	}

	updated, err := WithTableLayout(current, TableLayoutLibrary, TableLayout{Columns: []ColumnLayout{{Field: "name", Visible: false, Width: 90}, {Field: "name", Visible: true}}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]TableLayout{
		TableLayoutIssues:  {Columns: []ColumnLayout{{Field: "key", Visible: true}}},
		TableLayoutLibrary: {Columns: []ColumnLayout{{Field: "name", Visible: false, Width: 90}}},
	}
	if !reflect.DeepEqual(updated.GUI.TableLayouts, want) {
		t.Fatalf("updated layouts = %#v, want %#v", updated.GUI.TableLayouts, want)
	}
	if len(current.GUI.TableLayouts) != 1 {
		t.Fatalf("current settings were modified: %#v", current.GUI.TableLayouts)
	}

	reset, err := WithTableLayout(updated, TableLayoutIssues, TableLayout{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reset.GUI.TableLayouts[TableLayoutIssues]; ok || len(reset.GUI.TableLayouts) != 1 {
		t.Fatalf("reset layouts = %#v, want only the library layout", reset.GUI.TableLayouts)
	}

	cleared, err := WithTableLayout(reset, TableLayoutLibrary, TableLayout{Columns: []ColumnLayout{{Field: " "}}})
	if err != nil || cleared.GUI.TableLayouts != nil {
		t.Fatalf("clearing the last layout = %#v, %v; want nil layouts", cleared.GUI.TableLayouts, err)
	}
}

func TestWithTableLayoutRejectsUnknownTablesAndMissingSettings(t *testing.T) {
	if _, err := WithTableLayout(defaultSettings(), "settings", TableLayout{}); err == nil || !strings.Contains(err.Error(), "unknown table") {
		t.Fatalf("unknown table error = %v", err)
	}
	if _, err := WithTableLayout(nil, TableLayoutLibrary, TableLayout{}); err == nil || !strings.Contains(err.Error(), "not loaded") {
		t.Fatalf("missing settings error = %v", err)
	}
}

func TestTableLayoutsRoundTripAndAreSanitizedOnLoad(t *testing.T) {
	isolateSettings(t)
	base := t.TempDir()
	custom := defaultSettings()
	custom.GUI.TableLayouts = map[string]TableLayout{
		TableLayoutMissingDLC: {Columns: []ColumnLayout{{Field: "game.name", Visible: true, Width: 200}, {Field: "dlc.id", Visible: false}}},
	}
	if err := SaveSettingsWithError(custom, base); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(base, SETTINGS_FILENAME))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `"visible": false`) {
		t.Fatalf("hidden columns must be written explicitly:\n%s", contents)
	}

	settingsInstance = nil
	loaded, err := ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.GUI.TableLayouts, custom.GUI.TableLayouts) {
		t.Fatalf("loaded layouts = %#v, want %#v", loaded.GUI.TableLayouts, custom.GUI.TableLayouts)
	}

	edited := strings.Replace(string(contents), `"table_layouts": {`, `"table_layouts": {"bogus": {"columns": [{"field": "x", "visible": true}]}, `, 1)
	edited = strings.Replace(edited, `"width": 200`, `"width": 99999`, 1)
	if err := os.WriteFile(filepath.Join(base, SETTINGS_FILENAME), []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}
	settingsInstance = nil
	loaded, err = ReadSettings(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.GUI.TableLayouts["bogus"]; ok {
		t.Fatal("unknown table layout survived loading")
	}
	if width := loaded.GUI.TableLayouts[TableLayoutMissingDLC].Columns[0].Width; width != maxTableLayoutColumnWidth {
		t.Fatalf("loaded width = %d, want %d", width, maxTableLayoutColumnWidth)
	}
}
