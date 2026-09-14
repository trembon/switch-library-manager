package switchfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPfs0FileOrdinaryAndSplit(t *testing.T) {
	data := makePFS0(pfs0Magic, []string{"content.nca"}, [][]byte{{1, 2, 3}})
	dir := t.TempDir()
	ordinary := filepath.Join(dir, "ordinary.nsp")
	if err := os.WriteFile(ordinary, data, 0o600); err != nil {
		t.Fatal(err)
	}
	parsed, err := ReadPfs0File(ordinary)
	if err != nil || parsed.Filepath != ordinary || len(parsed.Files) != 1 {
		t.Fatalf("ordinary PFS0: %#v %v", parsed, err)
	}

	partSize := 7
	for i, start := 0, 0; start < len(data); i, start = i+1, start+partSize {
		end := start + partSize
		if end > len(data) {
			end = len(data)
		}
		if err := os.WriteFile(filepath.Join(dir, "split.nsp."+itoa(i)), data[start:end], 0o600); err != nil {
			t.Fatal(err)
		}
	}
	parsed, err = ReadPfs0File(filepath.Join(dir, "split.nsp.0"))
	if err != nil || len(parsed.Files) != 1 {
		t.Fatalf("split PFS0: %#v %v", parsed, err)
	}
}

func TestReadNspMetadataErrorsAndSuccess(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.nsp")
	if err := os.WriteFile(invalid, []byte("not PFS0"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNspMetadata(invalid); err == nil {
		t.Fatal("expected invalid NSP error")
	}

	cnmt := makeCNMT(0x0102030405060708, 7, ContentMetaType_Application)
	inner := makePFS0(pfs0Magic, []string{"meta.cnmt"}, [][]byte{cnmt})
	inner = append(inner, make([]byte, (-len(inner))&0x1ff)...)
	nca := makeSyntheticNCA(t, inner, 0)
	outer := makePFS0(pfs0Magic, []string{"meta.cnmt.nca", "ignored.txt"}, [][]byte{nca, []byte("ignored")})
	path := filepath.Join(dir, "metadata.nsp")
	if err := os.WriteFile(path, outer, 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err := ReadNspMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	got := metadata["0102030405060708"]
	if got == nil || got.Version != 7 || got.Type != "BASE" {
		t.Fatalf("unexpected NSP metadata: %#v", metadata)
	}

	badOuter := makePFS0(pfs0Magic, []string{"meta.cnmt.nca"}, [][]byte{{1}})
	badPath := filepath.Join(dir, "bad-metadata.nsp")
	if err := os.WriteFile(badPath, badOuter, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNspMetadata(badPath); err == nil {
		t.Fatal("expected NCA metadata parsing error")
	}
}

func TestReadPfs0FileOpenError(t *testing.T) {
	if _, err := ReadPfs0File(filepath.Join(t.TempDir(), "missing.nsp")); err == nil {
		t.Fatal("expected missing file error")
	}
}

func itoa(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	result := make([]byte, 0, 3)
	for value > 0 {
		result = append([]byte{digits[value%10]}, result...)
		value /= 10
	}
	return string(result)
}
