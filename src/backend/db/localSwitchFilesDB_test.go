package db

import "testing"

func TestParseTitleIdFromFileName(t *testing.T) {
	fileName := "Super Mario [0100000000010000][v0].nsp"
	titleId, err := parseTitleIdFromFileName(fileName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if titleId == nil || *titleId != "0100000000010000" {
		if titleId == nil {
			t.Fatalf("expected title ID not nil")
		}
		t.Fatalf("expected 0100000000010000 got %v", *titleId)
	}
}

func TestParseTitleIdFromFileNameInvalid(t *testing.T) {
	fileName := "Invalid [01000000000100,0][v0].nsp"
	_, err := parseTitleIdFromFileName(fileName)
	if err == nil {
		t.Fatalf("expected error for invalid title id")
	}
}
