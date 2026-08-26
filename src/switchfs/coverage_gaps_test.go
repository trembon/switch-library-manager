package switchfs

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestFsValidationBranches(t *testing.T) {
	if got := getFsEntry(nil, 0); got != (fsEntry{}) {
		t.Fatalf("nil FS entry: %#v", got)
	}
	if got := getFsEntry(&ncaHeader{headerBytes: make([]byte, 0x250)}, -1); got != (fsEntry{}) {
		t.Fatalf("negative FS entry: %#v", got)
	}
	bytesWithEntry := make([]byte, 0x260)
	binary.LittleEndian.PutUint32(bytesWithEntry[0x240:], 4)
	binary.LittleEndian.PutUint32(bytesWithEntry[0x244:], 3)
	if got := getFsEntry(&ncaHeader{headerBytes: bytesWithEntry}, 0); got != (fsEntry{}) {
		t.Fatalf("reversed FS entry: %#v", got)
	}
	if _, err := getFsHeader(nil, 0); err == nil {
		t.Fatal("expected nil FS header error")
	}
	if _, err := getFsHeader(&ncaHeader{headerBytes: make([]byte, 0x600)}, -1); err == nil {
		t.Fatal("expected negative FS header index error")
	}
	if _, err := getFsHeader(&ncaHeader{headerBytes: make([]byte, 0x600)}, 1); err == nil {
		t.Fatal("expected out-of-range FS header error")
	}
	if _, err := (&fsHeader{fsHeaderBytes: make([]byte, 0x100), hashType: 3}).getHashInfo(); err != nil {
		t.Fatal(err)
	}
	if _, err := (*fsHeader)(nil).getHashInfo(); err == nil {
		t.Fatal("expected nil hash info error")
	}
}

func TestOpenMetaNcaDataSectionValidationBranches(t *testing.T) {
	if _, _, err := openMetaNcaDataSection(bytes.NewReader(make([]byte, 0xc00)), 0); err == nil {
		t.Fatal("expected missing header key error")
	}
	initTestKeys(t)
	if _, _, err := openMetaNcaDataSection(bytes.NewReader(make([]byte, 0xc00)), 1); err == nil {
		t.Fatal("expected NCA header read error")
	}
	badHeader := make([]byte, 0xc00)
	if _, _, err := openMetaNcaDataSection(bytes.NewReader(badHeader), 0); err == nil {
		t.Fatal("expected invalid NCA header error")
	}

	plainSection := make([]byte, 0x200)
	validNCA := makeSyntheticNCA(t, plainSection, 0)
	if _, _, err := openMetaNcaDataSection(bytes.NewReader(validNCA), math.MaxInt64); err == nil {
		t.Fatal("expected NCA section offset overflow")
	}
	zeroSection := makeSyntheticNCA(t, plainSection, 0)
	zeroHeader := make([]byte, 0xc00)
	copy(zeroHeader, zeroSection[:0xc00])
	binary.LittleEndian.PutUint32(zeroHeader[0x244:], 6)
	zeroSection = replaceEncryptedNcaHeader(t, zeroSection, zeroHeader)
	if _, _, err := openMetaNcaDataSection(bytes.NewReader(zeroSection), 0); err == nil {
		t.Fatal("expected empty NCA section error")
	}
	unsupportedEncryption := mutateSyntheticNCAHeader(t, validNCA, func(header []byte) {
		header[0x404] = 1
	})
	if _, _, err := openMetaNcaDataSection(bytes.NewReader(unsupportedEncryption), 0); err == nil {
		t.Fatal("expected unsupported encryption error")
	}
	unsupportedHash := mutateSyntheticNCAHeader(t, validNCA, func(header []byte) {
		header[0x403] = 1
	})
	if _, _, err := openMetaNcaDataSection(bytes.NewReader(unsupportedHash), 0); err == nil {
		t.Fatal("expected unsupported hash error")
	}
	rights := mutateSyntheticNCAHeader(t, validNCA, func(header []byte) {
		header[0x230] = 1
	})
	if _, _, err := openMetaNcaDataSection(bytes.NewReader(rights), 0); err == nil {
		t.Fatal("expected rights ID error")
	}
}

func TestSplitReaderValidationBranches(t *testing.T) {
	dir := t.TempDir()
	noParent := filepath.Join("plain.nsp.0")
	if _, err := NewSplitFileReader(noParent); err == nil {
		t.Fatal("expected split parent error")
	}
	writePart(t, dir, "gap.nsp.0", []byte("123"))
	writePart(t, dir, "gap.nsp.2", []byte("123"))
	if _, err := NewSplitFileReader(filepath.Join(dir, "gap.nsp.0")); err == nil {
		t.Fatal("expected split gap error")
	}
	writePart(t, dir, "uneven.nsp.0", []byte("123"))
	writePart(t, dir, "uneven.nsp.1", []byte("12"))
	writePart(t, dir, "uneven.nsp.2", []byte("123"))
	if _, err := NewSplitFileReader(filepath.Join(dir, "uneven.nsp.0")); err == nil {
		t.Fatal("expected uneven split error")
	}
	writePart(t, dir, "requested.nsp.0", []byte("123"))
	if _, err := NewSplitFileReader(filepath.Join(dir, "requested.nsp.1")); err == nil {
		t.Fatal("expected missing requested part error")
	}
	if _, err := (&splitFile{}).ReadAt(make([]byte, 1), 0); err == nil {
		t.Fatal("expected invalid chunk size error")
	}
	if _, err := (&splitFile{chunkSize: 1}).ReadAt(make([]byte, 1), -1); err == nil {
		t.Fatal("expected negative split offset error")
	}
}

func TestReadNspDlcMetadata(t *testing.T) {
	inner := makePFS0(pfs0Magic, []string{"meta.cnmt"}, [][]byte{makeCNMT(2, 3, ContentMetaType_AddOnContent)})
	inner = append(inner, make([]byte, (-len(inner))&0x1ff)...)
	nca := makeSyntheticNCA(t, inner, 0)
	outer := makePFS0(pfs0Magic, []string{"meta.cnmt.nca"}, [][]byte{nca})
	path := filepath.Join(t.TempDir(), "dlc.nsp")
	if err := os.WriteFile(path, outer, 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err := ReadNspMetadata(path)
	if err != nil || metadata["0000000000000002"] == nil || metadata["0000000000000002"].Ncap != nil {
		t.Fatalf("unexpected DLC metadata: %#v %v", metadata, err)
	}
}

func TestReadXciMetadataValidationBranches(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		name string
		edit func([]byte)
	}{
		{name: "root offset overflow", edit: func(data []byte) {
			copy(data[0x100:], []byte("HEAD"))
			binary.LittleEndian.PutUint64(data[0x130:], math.MaxUint64)
		}},
		{name: "root parse error", edit: func(data []byte) {
			copy(data[0x100:], []byte("HEAD"))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := make([]byte, 0x200)
			test.edit(data)
			path := filepath.Join(dir, test.name+".xci")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadXciMetadata(path); err == nil {
				t.Fatal("expected XCI validation error")
			}
		})
	}

	root := &PFS0{Files: []fileEntry{{Name: "secure", StartOffset: 0}}}
	if _, _, err := readSecurePartition(bytes.NewReader([]byte("bad")), root, 0); err == nil {
		t.Fatal("expected secure partition parse error")
	}
}

func replaceEncryptedNcaHeader(t *testing.T, nca, header []byte) []byte {
	t.Helper()
	result := append([]byte(nil), nca...)
	copy(result, encryptNcaHeader(header, testHeaderKey))
	return result
}

func mutateSyntheticNCAHeader(t *testing.T, nca []byte, mutate func([]byte)) []byte {
	t.Helper()
	header := make([]byte, 0xc00)
	parsed, err := DecryptNcaHeader(testHeaderKey, nca[:0xc00])
	if err != nil {
		t.Fatal(err)
	}
	copy(header, parsed.headerBytes)
	mutate(header)
	hash := sha256Sum(header[0x400:0x600])
	copy(header[0x280:], hash[:])
	return replaceEncryptedNcaHeader(t, nca, header)
}
