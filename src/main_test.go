package main

import (
	"testing"

	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestResolveGUIMode(t *testing.T) {
	tests := []struct {
		name     string
		settings bool
		modeSet  bool
		mode     string
		wantGUI  bool
	}{
		{name: "settings gui", settings: true, wantGUI: true},
		{name: "settings console", settings: false, wantGUI: false},
		{name: "console flag overrides", settings: true, modeSet: true, mode: "console", wantGUI: false},
		{name: "gui flag overrides", settings: false, modeSet: true, mode: "gui", wantGUI: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveGUIMode(test.settings, test.modeSet, test.mode); got != test.wantGUI {
				t.Fatalf("resolveGUIMode() = %v, want %v", got, test.wantGUI)
			}
		})
	}
}

func TestConsoleMigrationPolicy(t *testing.T) {
	migration := &settings.MigrationInfo{BackupPath: "settings.old.json"}
	if !shouldAbortConsoleMigration(false, migration) {
		t.Fatal("console mode should stop when migration is pending")
	}
	if shouldAbortConsoleMigration(true, migration) {
		t.Fatal("GUI mode should continue with generated defaults")
	}
	if shouldAbortConsoleMigration(false, nil) {
		t.Fatal("console mode should continue without migration")
	}
}
