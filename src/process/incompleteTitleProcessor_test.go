package process

import (
	"encoding/json"
	"testing"

	"github.com/trembon/switch-library-manager/db"
)

func TestScanForMissingUpdates(t *testing.T) {
	baseID := "0100abcd00001000"
	updateID := "0100abcd00001800"
	dlcID := "0100abcd00001a01"
	local := map[string]*db.SwitchGameFiles{
		"0100abcd00001": {
			BaseExist: true,
			File:      db.SwitchFileInfo{Metadata: metadataForID(baseID, 0)},
			Updates: map[int]db.SwitchFileInfo{
				2: {Metadata: metadataForID(updateID, 2)},
			},
			Dlc: map[string]db.SwitchFileInfo{
				dlcID: {Metadata: metadataForID(dlcID, 1)},
			},
		},
		"missing-base": {
			File:      db.SwitchFileInfo{Metadata: metadataForID("0100feed00001000", 0)},
			BaseExist: false,
		},
		"not-remote": {
			File:      db.SwitchFileInfo{Metadata: metadataForID("0100feed00002000", 0)},
			BaseExist: true,
		},
	}
	remote := map[string]*db.SwitchTitle{
		"0100abcd00001": {
			Attributes: db.TitleAttributes{Id: baseID, Name: "Game"},
			Updates:    map[int]string{1: "old", 5: "latest"},
			Dlc: map[string]db.TitleAttributes{
				dlcID: {Id: dlcID, Name: "DLC", Version: json.Number("3"), ReleaseDate: 20240102},
			},
		},
	}

	result := ScanForMissingUpdates(local, remote, map[string]struct{}{}, false)
	game, ok := result[baseID]
	if !ok {
		t.Fatal("expected missing base update and DLC update")
	}
	if game.LocalUpdate != 2 || game.LatestUpdate != 5 || game.LatestUpdateDate != "latest" {
		t.Fatalf("update result = %#v", game)
	}
	dlc, ok := result[dlcID]
	if !ok || dlc.LocalUpdate != 1 || dlc.LatestUpdate != 3 || dlc.LatestUpdateDate != "2024-01-02" {
		t.Fatalf("DLC result = %#v, present = %v", dlc, ok)
	}

	ignored := ScanForMissingUpdates(local, remote, map[string]struct{}{baseID: {}}, false)
	if len(ignored) != 0 {
		t.Fatalf("ignored base result = %#v", ignored)
	}
	ignoredDLC := ScanForMissingUpdates(local, remote, map[string]struct{}{dlcID: {}}, false)
	if _, ok := ignoredDLC[dlcID]; ok {
		t.Fatal("ignored DLC was reported")
	}
	withoutDLC := ScanForMissingUpdates(local, remote, map[string]struct{}{}, true)
	if _, ok := withoutDLC[dlcID]; ok {
		t.Fatal("DLC update was reported when ignored")
	}

	noMissing := map[string]*db.SwitchGameFiles{
		"0100abcd00001": {BaseExist: true, File: db.SwitchFileInfo{Metadata: metadataForID(baseID, 0)}, Updates: map[int]db.SwitchFileInfo{5: {}}},
	}
	if got := ScanForMissingUpdates(noMissing, remote, nil, true); len(got) != 0 {
		t.Fatalf("complete result = %#v", got)
	}
}

func TestScanForMissingUpdatesDLCBranches(t *testing.T) {
	baseID := "0100abcd00001000"
	dlcGoodID := "0100abcd00001a01"
	dlcBadNumberID := "0100abcd00001a02"
	dlcNoLocalID := "0100abcd00001a03"
	local := map[string]*db.SwitchGameFiles{"0100abcd00001": {
		BaseExist: true,
		File:      db.SwitchFileInfo{Metadata: metadataForID(baseID, 0)},
		Dlc: map[string]db.SwitchFileInfo{
			dlcGoodID:      {Metadata: metadataForID(dlcGoodID, 1)},
			dlcBadNumberID: {Metadata: metadataForID(dlcBadNumberID, 1)},
			dlcNoLocalID:   {},
		},
	}}
	remote := map[string]*db.SwitchTitle{"0100abcd00001": {
		Attributes: db.TitleAttributes{Id: baseID},
		Dlc: map[string]db.TitleAttributes{
			dlcGoodID:      {Id: dlcGoodID, Version: json.Number("2"), ReleaseDate: 0},
			dlcBadNumberID: {Id: dlcBadNumberID, Version: json.Number("not-a-number")},
			dlcNoLocalID:   {Id: dlcNoLocalID, Version: json.Number("2")},
		},
	}}
	result := ScanForMissingUpdates(local, remote, map[string]struct{}{}, false)
	if got, ok := result[dlcGoodID]; !ok || got.LatestUpdateDate != "-" {
		t.Fatalf("DLC with missing release date = %#v, present = %v", got, ok)
	}
	if _, ok := result[dlcBadNumberID]; ok {
		t.Fatal("DLC with invalid remote version was reported")
	}
	if _, ok := result[dlcNoLocalID]; ok {
		t.Fatal("DLC with no local record was reported")
	}

	local["0100abcd00001"].Dlc[dlcGoodID] = db.SwitchFileInfo{}
	if _, ok := ScanForMissingUpdates(local, remote, nil, false)[dlcGoodID]; ok {
		t.Fatal("metadata-free local DLC was reported")
	}
}

func TestScanForMissingDLC(t *testing.T) {
	baseID := "0100abcd00001000"
	missingID := "0100abcd00001a01"
	ignoredID := "0100abcd00001a02"
	remote := map[string]*db.SwitchTitle{
		"0100abcd00001": {Attributes: db.TitleAttributes{Id: baseID}, Dlc: map[string]db.TitleAttributes{
			missingID: {Name: "Expansion", Id: missingID},
			ignoredID: {Name: "Ignored", Id: ignoredID},
		}},
	}
	local := map[string]*db.SwitchGameFiles{
		"0100abcd00001": {BaseExist: true, Dlc: map[string]db.SwitchFileInfo{ignoredID: {}}},
		"missing-base":  {BaseExist: false},
		"not-remote":    {BaseExist: true},
	}
	result := ScanForMissingDLC(local, remote, map[string]struct{}{ignoredID: {}})
	game, ok := result[baseID]
	if !ok || len(game.MissingDLC) != 1 || game.MissingDLC[0] != "Expansion ["+missingID+"]" {
		t.Fatalf("missing DLC result = %#v, present = %v", game, ok)
	}
	if got := ScanForMissingDLC(local, remote, map[string]struct{}{missingID: {}, ignoredID: {}}); len(got) != 0 {
		t.Fatalf("fully ignored result = %#v", got)
	}
	if got := ScanForMissingDLC(map[string]*db.SwitchGameFiles{}, remote, nil); len(got) != 0 {
		t.Fatalf("empty local result = %#v", got)
	}
}

func TestScanForBrokenFiles(t *testing.T) {
	base := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "base.nsp"}}
	dlc := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "dlc.nsp"}}
	update := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{FileName: "update.nsp"}}
	result := ScanForBrokenFiles(map[string]*db.SwitchGameFiles{
		"has-base":     {BaseExist: true, File: base, Dlc: map[string]db.SwitchFileInfo{"base": dlc}, Updates: map[int]db.SwitchFileInfo{1: update}},
		"missing-base": {BaseExist: false, Dlc: map[string]db.SwitchFileInfo{"dlc": dlc}, Updates: map[int]db.SwitchFileInfo{1: update}},
	})
	if len(result) != 2 {
		t.Fatalf("broken files = %#v, want 2", result)
	}
	for _, file := range result {
		if file.ExtendedInfo.FileName != "dlc.nsp" && file.ExtendedInfo.FileName != "update.nsp" {
			t.Fatalf("unexpected broken file = %#v", file)
		}
	}
	if got := ScanForBrokenFiles(map[string]*db.SwitchGameFiles{}); len(got) != 0 {
		t.Fatalf("empty broken files = %#v", got)
	}
}
