package consoleapp

import (
	"testing"
	"time"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/process"
)

func TestCsvExportFilename(t *testing.T) {
	date := time.Date(2026, time.September, 21, 23, 45, 0, 0, time.FixedZone("test", 2*60*60))
	tests := map[string]string{
		csvGamesKind:          "slm_games.2026-09-21.csv",
		csvMissingGamesKind:   "slm_missing_games.2026-09-21.csv",
		csvMissingUpdatesKind: "slm_missing_updates.2026-09-21.csv",
		csvMissingDLCKind:     "slm_missing_dlc.2026-09-21.csv",
		csvIssuesKind:         "slm_issues.2026-09-21.csv",
	}

	for kind, want := range tests {
		t.Run(kind, func(t *testing.T) {
			if got := csvExportFilename(kind, date); got != want {
				t.Fatalf("csvExportFilename(%q) = %q, want %q", kind, got, want)
			}
		})
	}
}

func TestFlattenMissingDLC(t *testing.T) {
	titles := map[string]process.IncompleteTitle{
		"game-z": {
			Attributes: db.TitleAttributes{Id: "game-z-id", Name: "Zulu"},
			MissingDLC: []process.MissingDLC{
				{Id: "dlc-z-id", Name: "Expansion"},
				{Id: "dlc-a-id", Name: "Adventure"},
				{Id: "dlc-y-id", Name: "Expansion"},
			},
		},
		"game-a": {
			Attributes: db.TitleAttributes{Id: "game-a-id", Name: "Alpha"},
			MissingDLC: []process.MissingDLC{{Id: "dlc-b-id", Name: "Bonus"}},
		},
	}

	rows := flattenMissingDLC(titles)
	if len(rows) != 4 {
		t.Fatalf("flattened missing DLC rows = %d, want 4", len(rows))
	}
	want := []struct{ game, gameID, dlc, dlcID string }{
		{"Alpha", "game-a-id", "Bonus", "dlc-b-id"},
		{"Zulu", "game-z-id", "Adventure", "dlc-a-id"},
		{"Zulu", "game-z-id", "Expansion", "dlc-y-id"},
		{"Zulu", "game-z-id", "Expansion", "dlc-z-id"},
	}
	for i, row := range rows {
		if row.Game.Name != want[i].game || row.Game.Id != want[i].gameID || row.DLC.Name != want[i].dlc || row.DLC.Id != want[i].dlcID {
			t.Errorf("row %d = (%q, %q, %q, %q), want (%q, %q, %q, %q)", i, row.Game.Name, row.Game.Id, row.DLC.Name, row.DLC.Id, want[i].game, want[i].gameID, want[i].dlc, want[i].dlcID)
		}
		csvRow := missingDLCCSVRow(row)
		if len(csvRow) != 3 || csvRow[0] != want[i].game || csvRow[1] != want[i].gameID || csvRow[2] != want[i].dlc+" ["+want[i].dlcID+"]" {
			t.Errorf("CSV row %d = %#v", i, csvRow)
		}
	}
}
