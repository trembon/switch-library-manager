package fileio

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func makeContainer(magic string, name string) []byte {
	entrySize := 0x18
	if magic == "HFS0" {
		entrySize = 0x40
	}
	data := make([]byte, 0x10+entrySize+len(name)+1)
	copy(data, magic)
	binary.LittleEndian.PutUint32(data[4:8], 1)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(name)+1))
	binary.LittleEndian.PutUint32(data[0x10+16:0x10+20], 0)
	copy(data[0x10+entrySize:], name+"\x00")
	return data
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReadSplitFileMetadataDispatchesNSPAndXCI(t *testing.T) {
	dir := t.TempDir()
	nspPath := filepath.Join(dir, "sample.nsp")
	writeFile(t, nspPath, makeContainer("PFS0", "content.nca"))
	metadata, err := ReadSplitFileMetadata(nspPath)
	if err != nil || len(metadata) != 0 {
		t.Fatalf("NSP dispatch: %#v %v", metadata, err)
	}

	root := makeContainer("HFS0", "secure")
	secure := makeContainer("PFS0", "")
	xci := make([]byte, 0x200)
	copy(xci[0x100:0x104], "HEAD")
	binary.LittleEndian.PutUint64(xci[0x130:0x138], 0x200)
	xci = append(xci, root...)
	xci = append(xci, secure...)
	xciPath := filepath.Join(dir, "sample.xci")
	writeFile(t, xciPath, xci)
	metadata, err = ReadSplitFileMetadata(xciPath)
	if err != nil || len(metadata) != 0 {
		t.Fatalf("XCI dispatch: %#v %v", metadata, err)
	}
}

func TestReadSplitFileMetadataSupportsSplitNSP(t *testing.T) {
	dir := t.TempDir()
	data := makeContainer("PFS0", "content.nca")
	partSize := 8
	for i, start := 0, 0; start < len(data); i, start = i+1, start+partSize {
		end := start + partSize
		if end > len(data) {
			end = len(data)
		}
		writeFile(t, filepath.Join(dir, "sample.nsp."+itoa(i)), data[start:end])
	}
	metadata, err := ReadSplitFileMetadata(filepath.Join(dir, "sample.nsp.0"))
	if err != nil || len(metadata) != 0 {
		t.Fatalf("split NSP dispatch: %#v %v", metadata, err)
	}
}

func TestReadSplitFileMetadataRejectsMalformedAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"bad.nsp", "bad.xci"} {
		path := filepath.Join(dir, name)
		writeFile(t, path, []byte("bad"))
		if _, err := ReadSplitFileMetadata(path); err == nil {
			t.Fatalf("expected dispatch error for %s", name)
		}
	}
	missing := filepath.Join(dir, "missing.nsp.1")
	writeFile(t, filepath.Join(dir, "missing.nsp.0"), makeContainer("PFS0", "x"))
	if _, err := ReadSplitFileMetadata(missing); err == nil {
		t.Fatal("expected missing split part error")
	}
	if _, err := readXciHeader(filepath.Join(dir, "does-not-exist")); err == nil {
		t.Fatal("expected missing XCI error")
	}
	short := filepath.Join(dir, "short.xci")
	writeFile(t, short, make([]byte, 0x100))
	if _, err := readXciHeader(short); err == nil {
		t.Fatal("expected short XCI error")
	}
}

func itoa(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	result := ""
	for value > 0 {
		result = string(digits[value%10]) + result
		value /= 10
	}
	return result
}
