package app

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestFileManagerCommand(t *testing.T) {
	path := filepath.Join("library with spaces", "Pokémon [0100000000001000].nsp")
	command, args := fileManagerCommand(path)
	cleanPath := filepath.Clean(path)

	var wantCommand string
	var wantArgs []string
	switch runtime.GOOS {
	case "windows":
		wantCommand = "explorer.exe"
		wantArgs = []string{"/select,", cleanPath}
	case "darwin":
		wantCommand = "open"
		wantArgs = []string{"-R", cleanPath}
	default:
		wantCommand = "xdg-open"
		wantArgs = []string{filepath.Dir(cleanPath)}
	}
	if command != wantCommand || !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("fileManagerCommand(%q) = %q, %#v; want %q, %#v", path, command, args, wantCommand, wantArgs)
	}
}

func TestShowInFolderRejectsEmptyPath(t *testing.T) {
	if err := (&App{}).ShowInFolder(""); err == nil || err.Error() != "path is empty" {
		t.Fatalf("ShowInFolder(\"\") error = %v; want path is empty", err)
	}
}
