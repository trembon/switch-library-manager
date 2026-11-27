package db

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFlexibleStringsAcceptsListsStringsAndIgnoresOtherShapes(t *testing.T) {
	cases := map[string]FlexibleStrings{
		`["Action", " RPG ", "", 7, null, {"a":1}]`: {"Action", "RPG"},
		`"Puzzle"`:  {"Puzzle"},
		`"  "`:      nil,
		`null`:      nil,
		`12`:        nil,
		`{"a":"b"}`: nil,
		`[]`:        nil,
		`[1, 2]`:    nil,
	}
	for input, want := range cases {
		var got FlexibleStrings
		if err := json.Unmarshal([]byte(input), &got); err != nil {
			t.Fatalf("Unmarshal(%s) returned error: %v", input, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Unmarshal(%s) = %#v, want %#v", input, got, want)
		}
	}
}

func TestFlexibleStringsResetsPreviousValue(t *testing.T) {
	got := FlexibleStrings{"stale"}
	if err := got.UnmarshalJSON([]byte(" ")); err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("blank input should clear the value, got %#v", got)
	}
}

func TestFlexibleIntAcceptsNumbersAndNumericStrings(t *testing.T) {
	cases := map[string]FlexibleInt{
		`4`:      4,
		`"12"`:   12,
		`" 7 "`:  7,
		`null`:   0,
		`"many"`: 0,
		`2.5`:    0,
		`[3]`:    0,
		`true`:   0,
		`"\x"`:   0,
	}
	for input, want := range cases {
		got := FlexibleInt(99)
		if err := got.UnmarshalJSON([]byte(input)); err != nil {
			t.Fatalf("UnmarshalJSON(%s) returned error: %v", input, err)
		}
		if got != want {
			t.Errorf("UnmarshalJSON(%s) = %d, want %d", input, got, want)
		}
	}
	got := FlexibleInt(5)
	if err := got.UnmarshalJSON(nil); err != nil || got != 0 {
		t.Fatalf("empty input = %d, %v; want 0, nil", got, err)
	}
}

func TestTitleCatalogToleratesMixedMetadataTypes(t *testing.T) {
	titles := `{
		"0100000000010000": {"id": "0100000000010000", "name": "Typed", "category": ["Action", "RPG"], "languages": ["en", "ja"], "numberOfPlayers": 4, "rating": 12, "ratingContent": ["Violence"]},
		"0100000000020000": {"id": "0100000000020000", "name": "Odd", "category": "Puzzle", "languages": null, "numberOfPlayers": "2", "rating": "unrated", "ratingContent": {"bad": true}},
		"0100000000030000": {"id": "0100000000030000", "name": "Missing"}
	}`
	if err := ValidateTitlesJSON([]byte(titles)); err != nil {
		t.Fatalf("ValidateTitlesJSON rejected mixed metadata: %v", err)
	}
	database, err := CreateSwitchTitleDB(strings.NewReader(titles), strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("CreateSwitchTitleDB rejected mixed metadata: %v", err)
	}

	typed := database.TitlesMap["0100000000010"].Attributes
	if !reflect.DeepEqual(typed.Category, FlexibleStrings{"Action", "RPG"}) || !reflect.DeepEqual(typed.Languages, FlexibleStrings{"en", "ja"}) ||
		typed.NumberOfPlayers != 4 || typed.Rating != 12 || !reflect.DeepEqual(typed.RatingContent, FlexibleStrings{"Violence"}) {
		t.Fatalf("typed metadata decoded as %#v", typed)
	}
	odd := database.TitlesMap["0100000000020"].Attributes
	if !reflect.DeepEqual(odd.Category, FlexibleStrings{"Puzzle"}) || odd.Languages != nil || odd.NumberOfPlayers != 2 || odd.Rating != 0 || odd.RatingContent != nil {
		t.Fatalf("mixed metadata decoded as %#v", odd)
	}
	missing := database.TitlesMap["0100000000030"].Attributes
	if missing.Category != nil || missing.NumberOfPlayers != 0 || missing.Rating != 0 {
		t.Fatalf("absent metadata decoded as %#v", missing)
	}
}
