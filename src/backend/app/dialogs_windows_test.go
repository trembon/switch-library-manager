package app

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestFileManagerCommandWindowsQuotesOnlyPath(t *testing.T) {
	path := filepath.Clean(`C:\Users\Test\Switch Games\Pokémon [0100000000001000].nsp`)
	command, args := fileManagerCommand(path)
	if command != "explorer.exe" || len(args) != 2 || args[0] != "/select," || args[1] != path {
		t.Fatalf("fileManagerCommand() = %q, %#v; want explorer.exe, /select, and %q", command, args, path)
	}

	argv := append([]string{command}, args...)
	quotedArgs := make([]string, len(argv))
	for i, arg := range argv {
		quotedArgs[i] = syscall.EscapeArg(arg)
	}
	got := strings.Join(quotedArgs, " ")
	want := `explorer.exe /select, "C:\Users\Test\Switch Games\Pokémon [0100000000001000].nsp"`
	if got != want {
		t.Fatalf("Windows command line = %q; want %q", got, want)
	}
}

func TestFileManagerCommandWindowsPreservesUNCPath(t *testing.T) {
	path := filepath.Clean(`\\server\share with spaces\library\game [0100000000001000].nsp`)
	command, args := fileManagerCommand(path)
	if command != "explorer.exe" || len(args) != 2 || args[0] != "/select," || args[1] != path {
		t.Fatalf("fileManagerCommand() = %q, %#v; want explorer.exe, /select, and UNC path %q", command, args, path)
	}
}
