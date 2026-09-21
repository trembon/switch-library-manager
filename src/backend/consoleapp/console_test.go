package consoleapp

import (
	"testing"
	"time"
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
