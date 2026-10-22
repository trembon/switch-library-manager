package switchfs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writePart(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSplitFileReaderBoundariesAndClose(t *testing.T) {
	dir := t.TempDir()
	partPath := writePart(t, dir, "game.nsp.0", []byte("0123456789"))
	writePart(t, dir, "game.nsp.1", []byte("abcdefghij"))
	writePart(t, dir, "game.nsp.2", []byte("XYZ"))
	reader, err := NewSplitFileReader(partPath)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 8)
	n, err := reader.ReadAt(data, 7)
	if err != nil || n != len(data) || string(data) != "789abcde" {
		t.Fatalf("cross-part read: n=%d err=%v data=%q", n, err, data)
	}
	data = make([]byte, 3)
	n, err = reader.ReadAt(data, 20)
	if err != nil || n != 3 || string(data) != "XYZ" {
		t.Fatalf("last part read: n=%d err=%v data=%q", n, err, data)
	}
	if _, err := reader.ReadAt(make([]byte, 1), 23); err == nil {
		t.Fatal("expected missing part error")
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadAt(make([]byte, 1), 0); err == nil {
		t.Fatal("expected closed chunk error")
	}
}

func TestSplitFileReaderValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.nsp.0")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSplitFileReader(path); err == nil {
		t.Fatal("expected empty part error")
	}
	missing := filepath.Join(dir, "missing.nsp.1")
	writePart(t, dir, "missing.nsp.0", []byte("123"))
	if _, err := NewSplitFileReader(missing); err == nil {
		t.Fatal("expected missing part error")
	}
	emptyDirPath := filepath.Join(t.TempDir(), "none.nsp.0")
	if _, err := NewSplitFileReader(emptyDirPath); err == nil {
		t.Fatal("expected no parts error")
	}
	if _, err := OpenFile(""); err == nil {
		t.Fatal("expected empty path error")
	}
	if n, err := (&splitFile{chunkSize: 1}).ReadAt(nil, 0); err != nil || n != 0 {
		t.Fatalf("empty read should be harmless: %d %v", n, err)
	}
}

func TestFileWrapperAndOpenFile(t *testing.T) {
	dir := t.TempDir()
	filePath := writePart(t, dir, "ordinary.nsp", []byte("payload"))
	reader, err := OpenFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 7)
	if _, err := reader.ReadAt(data, 0); err != nil || !bytes.Equal(data, []byte("payload")) {
		t.Fatalf("ordinary read: %q %v", data, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadAt(make([]byte, 1), 0); err == nil {
		t.Fatal("expected closed wrapper error")
	}
	if _, err := (&fileWrapper{}).ReadAt(make([]byte, 1), 0); err == nil {
		t.Fatal("expected unopened wrapper error")
	}
	if err := (&fileWrapper{}).Close(); err != nil {
		t.Fatal(err)
	}
}
