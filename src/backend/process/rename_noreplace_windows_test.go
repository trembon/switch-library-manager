//go:build windows

package process

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRenameNoReplacePreservesOccupiedDestination(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.nsp")
	destination := filepath.Join(directory, "destination.nsp")
	if err := os.WriteFile(source, []byte("source content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("destination content"), 0600); err != nil {
		t.Fatal(err)
	}

	err := renameNoReplace(source, destination)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("renameNoReplace() error = %v, want an existence error", err)
	}

	for path, want := range map[string]string{
		source:      "source content",
		destination: "destination content",
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %q: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("contents of %q = %q, want %q", path, got, want)
		}
	}
}
