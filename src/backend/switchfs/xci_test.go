package switchfs

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestReadSecurePartitionErrors(t *testing.T) {
	if _, _, err := readSecurePartition(bytes.NewReader(nil), nil, 0); err == nil {
		t.Fatal("expected nil partition error")
	}
	root := &PFS0{Files: []fileEntry{{Name: "normal", StartOffset: 0}}}
	secure, offset, err := readSecurePartition(bytes.NewReader(nil), root, 0)
	if err != nil || secure != nil || offset != 0 {
		t.Fatalf("unexpected missing secure result: %#v %d %v", secure, offset, err)
	}
	root.Files = []fileEntry{{Name: "secure", StartOffset: ^uint64(0)}}
	if _, _, err := readSecurePartition(bytes.NewReader(nil), root, 1); err == nil {
		t.Fatal("expected secure offset overflow error")
	}
	secureData := makePFS0(pfs0Magic, []string{"entry"}, [][]byte{{1, 2, 3}})
	rootData := makePFS0(hfs0Magic, []string{"secure"}, [][]byte{secureData})
	secureOffset := uint64(0x20)
	fileData := append(make([]byte, secureOffset), rootData...)
	parsedRoot, err := readPfs0(bytes.NewReader(fileData), int64(secureOffset))
	if err != nil {
		t.Fatal(err)
	}
	parsedSecure, offset, err := readSecurePartition(bytes.NewReader(fileData), parsedRoot, secureOffset)
	if err != nil || parsedSecure == nil || offset != int64(secureOffset)+int64(parsedRoot.Files[0].StartOffset) {
		t.Fatalf("secure partition: %#v %d %v", parsedSecure, offset, err)
	}
}

func TestReadXciMetadataRejectsMalformedHeaders(t *testing.T) {
	if _, err := ReadXciMetadata(""); err == nil {
		t.Fatal("expected empty path error")
	}
	dir := t.TempDir()
	short := filepath.Join(dir, "short.xci")
	if err := os.WriteFile(short, make([]byte, 0x1ff), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadXciMetadata(short); err == nil {
		t.Fatal("expected truncated XCI header error")
	}
	bad := filepath.Join(dir, "bad.xci")
	if err := os.WriteFile(bad, make([]byte, 0x200), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadXciMetadata(bad); err == nil {
		t.Fatal("expected invalid XCI header error")
	}
}

func TestReadXciMetadataSynthetic(t *testing.T) {
	controlID := [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	cnmt := makeCNMT(0x0102030405060708, 11, ContentMetaType_Application, controlID)
	cnmt[0x20+0x36] = 3
	inner := makePFS0(pfs0Magic, []string{"meta.cnmt"}, [][]byte{cnmt})
	inner = append(inner, make([]byte, (-len(inner))&0x1ff)...)
	metaNCA := makeSyntheticNCA(t, inner, 0)
	controlNCA := makeSyntheticNCA(t, makeRomfsSection(t, "control.nacp"), 0)
	controlName := "000102030405060708090a0b0c0d0e0f.nca"
	secure := makePFS0(pfs0Magic, []string{"meta.cnmt.nca", controlName}, [][]byte{metaNCA, controlNCA})
	root := makePFS0(hfs0Magic, []string{"secure"}, [][]byte{secure})
	rootOffset := uint64(0x200)
	xci := make([]byte, int(rootOffset)+len(root))
	copy(xci, make([]byte, 0x200))
	copy(xci[rootOffset:], root)
	copy(xci[0x100:], []byte("HEAD"))
	binary.LittleEndian.PutUint64(xci[0x130:], rootOffset)
	dir := t.TempDir()
	path := filepath.Join(dir, "synthetic.xci")
	if err := os.WriteFile(path, xci, 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err := ReadXciMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	got := metadata["0102030405060708"]
	if got == nil || got.Version != 11 || got.Ncap == nil || got.Ncap.TitleName["AmericanEnglish"].Title != "Synthetic Game" {
		t.Fatalf("unexpected XCI metadata: %#v, entry=%#v", metadata, got)
	}
}

func TestGetNcaById(t *testing.T) {
	files := &PFS0{Files: []fileEntry{{Name: "one.nca"}, {Name: "target.nca"}}}
	if got := getNcaById(files, "target"); got == nil || got.Name != "target.nca" {
		t.Fatalf("unexpected NCA lookup: %#v", got)
	}
	if getNcaById(files, "absent") != nil {
		t.Fatal("unexpected NCA lookup result")
	}
}
