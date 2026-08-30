package switchfs

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"testing"
)

func TestExtractNacpBranches(t *testing.T) {
	reader := bytes.NewReader(nil)
	if _, err := ExtractNacp(&ContentMetaAttributes{}, reader, &PFS0{}, 0); err == nil {
		t.Fatal("expected missing control error")
	}
	cnmt := &ContentMetaAttributes{Contents: map[string]Content{
		"Control": {ID: "missing"},
	}}
	partition := &PFS0{Files: []fileEntry{{Name: "other", StartOffset: 0}}}
	if _, err := ExtractNacp(cnmt, reader, partition, 0); err == nil {
		t.Fatal("expected missing control NCA error")
	}

	section := makeRomfsSection(t, "control.nacp")
	nca := makeSyntheticNCA(t, section, 0)
	_, decoded, err := openMetaNcaDataSection(bytes.NewReader(nca), 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readRomfsHeader(decoded); err != nil {
		t.Fatalf("synthetic RomFS: %x: %v", decoded[:16], err)
	}
	control := &PFS0{Files: []fileEntry{{Name: "control-id", StartOffset: 0}}}
	got, err := ExtractNacp(&ContentMetaAttributes{Contents: map[string]Content{
		"Control": {ID: "control-id"},
	}}, bytes.NewReader(nca), control, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.TitleName["AmericanEnglish"].Title != "Synthetic Game" {
		t.Fatalf("unexpected extracted NACP: %#v", got)
	}

	unsupported := makeSyntheticNCA(t, make([]byte, 0x200), 1)
	if _, err := ExtractNacp(&ContentMetaAttributes{Contents: map[string]Content{
		"Control": {ID: "control-id"},
	}}, bytes.NewReader(unsupported), control, 0); err == nil {
		t.Fatal("expected unsupported filesystem error")
	}

	noNacp := makeRomfsSection(t, "other")
	noNacpNca := makeSyntheticNCA(t, noNacp, 0)
	if _, err := ExtractNacp(&ContentMetaAttributes{Contents: map[string]Content{
		"Control": {ID: "control-id"},
	}}, bytes.NewReader(noNacpNca), control, 0); err == nil {
		t.Fatal("expected missing control.nacp error")
	}
}

func TestReadNacpCompressedTitles(t *testing.T) {
	const nacpSize = 0x4000
	const titleDataSize = 0x6000
	titleData := make([]byte, titleDataSize)
	copy(titleData, []byte("Compressed Game"))
	var compressed bytes.Buffer
	writer, err := flate.NewWriter(&compressed, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(titleData); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if compressed.Len() > 0xffff {
		t.Fatalf("compressed title data is too large: %d", compressed.Len())
	}

	data := make([]byte, nacpSize)
	binary.LittleEndian.PutUint16(data, uint16(compressed.Len()))
	copy(data[2:], compressed.Bytes())
	data[0x3215] = 1
	nacp, err := readNacp(data, RomfsHeader{}, RomfsFileEntry{size: nacpSize})
	if err != nil {
		t.Fatal(err)
	}
	if nacp.TitleName["AmericanEnglish"].Title != "Compressed Game" {
		t.Fatalf("unexpected compressed NACP title: %q", nacp.TitleName["AmericanEnglish"].Title)
	}
}

func makeRomfsSection(t *testing.T, name string) []byte {
	t.Helper()
	const (
		fileTableOffset = 0x50
		dataOffset      = 0x100
		nacpSize        = 0x3080
	)
	nameBytes := []byte(name)
	entrySize := 0x20 + len(nameBytes)*2
	sectionSize := dataOffset + nacpSize
	if name != "control.nacp" {
		sectionSize = dataOffset + 0x200
	}
	sectionSize = (sectionSize + 0x1ff) &^ 0x1ff
	data := make([]byte, sectionSize)
	putRomfsHeader(data, RomfsHeader{
		HeaderSize:          0x50,
		FileMetaTableOffset: fileTableOffset,
		FileMetaTableSize:   uint64(entrySize),
		DataOffset:          dataOffset,
	})
	entry := data[fileTableOffset:]
	binary.LittleEndian.PutUint32(entry[0x1c:0x20], uint32(len(nameBytes)*2))
	binary.LittleEndian.PutUint64(entry[0x8:0x10], 0)
	if name == "control.nacp" {
		binary.LittleEndian.PutUint64(entry[0x10:0x18], nacpSize)
	} else {
		binary.LittleEndian.PutUint64(entry[0x10:0x18], 0x200)
	}
	for i, value := range nameBytes {
		binary.LittleEndian.PutUint16(entry[0x20+i*2:], uint16(value))
	}
	if name == "control.nacp" {
		copy(data[dataOffset:], []byte("Synthetic Game"))
		copy(data[dataOffset+0x3060:], []byte("1.0.0"))
	}
	return data
}
