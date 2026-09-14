package main

import (
	"testing"

	"github.com/trembon/switch-library-manager/db"
)

func TestGetLibraryTitleNameFallsBackForEmptyRemoteName(t *testing.T) {
	title := &db.SwitchTitle{Attributes: db.TitleAttributes{Name: ""}}
	if got := getLibraryTitleName(title, "", "Golf Story [01005EA00A57E000][v0].nsp"); got != "Golf Story " {
		t.Fatalf("title name = %q, want %q", got, "Golf Story ")
	}
}

func TestGetLibraryTitleNamePreservesNamePrecedence(t *testing.T) {
	title := &db.SwitchTitle{Attributes: db.TitleAttributes{Name: "Remote Name"}}
	if got := getLibraryTitleName(title, "NACP Name", "File Name [0100000000010000][v1].nsp"); got != "Remote Name" {
		t.Fatalf("remote title name = %q, want %q", got, "Remote Name")
	}

	emptyTitle := &db.SwitchTitle{}
	if got := getLibraryTitleName(emptyTitle, "NACP Name", "File Name [0100000000010000][v1].nsp"); got != "NACP Name" {
		t.Fatalf("NACP title name = %q, want %q", got, "NACP Name")
	}
}
