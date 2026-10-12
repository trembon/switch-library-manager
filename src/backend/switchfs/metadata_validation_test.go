package switchfs

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBinaryCnmtContentTypes(t *testing.T) {
	for contentType, want := range []string{"Meta", "Program", "Data", "Control", "HtmlDocument", "LegalInformation", "DeltaFragment"} {
		t.Run(want, func(t *testing.T) {
			id := [16]byte{0xab, byte(contentType)}
			data := makeCNMT(1, 7, ContentMetaType_Application, id)
			data[0x20+0x36] = byte(contentType)
			partition := &PFS0{Files: []fileEntry{{Size: uint64(len(data))}}}
			metadata, err := readBinaryCnmt(partition, data)
			if err != nil {
				t.Fatal(err)
			}
			if len(metadata.Contents) != 1 || metadata.Contents[want].ID != fmt.Sprintf("%x", id) {
				t.Fatalf("content type %d = %#v, want %s", contentType, metadata.Contents, want)
			}
		})
	}
}

func TestReadCnmtRejectsOutOfBoundsAndNonhexadecimalIDs(t *testing.T) {
	for _, entry := range []fileEntry{{StartOffset: 33}, {StartOffset: 16, Size: 17}} {
		if _, err := readBinaryCnmt(&PFS0{Files: []fileEntry{entry}}, make([]byte, 32)); err == nil || !strings.Contains(err.Error(), "outside its container") {
			t.Fatalf("CNMT range error = %v", err)
		}
	}
	if _, err := readXmlCnmt([]byte(`<ContentMeta><Id>010000000000000g</Id></ContentMeta>`)); err == nil || !strings.Contains(err.Error(), "invalid CNMT title ID") {
		t.Fatalf("XML title ID error = %v", err)
	}
}

func TestMetadataContainersPropagateInnerParsingErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		section []byte
		want    string
	}{
		{"invalid PFS0", []byte("invalid inner container"), "Invalid NSP headerBytes"},
		{"short CNMT", makePFS0(pfs0Magic, []string{"meta.cnmt"}, [][]byte{{1}}), "CNMT header is truncated"},
	} {
		for _, format := range []string{"nsp", "xci"} {
			t.Run(test.name+"/"+format, func(t *testing.T) {
				section := append([]byte(nil), test.section...)
				section = append(section, make([]byte, (-len(section))&0x1ff)...)
				container := makePFS0(pfs0Magic, []string{"meta.cnmt.nca"}, [][]byte{makeSyntheticNCA(t, section, 0)})
				read := ReadNspMetadata
				if format == "xci" {
					container = syntheticXciWithRoot(makePFS0(hfs0Magic, []string{"secure"}, [][]byte{container}))
					read = ReadXciMetadata
				}
				path := filepath.Join(t.TempDir(), "malformed."+format)
				if err := os.WriteFile(path, container, 0600); err != nil {
					t.Fatal(err)
				}
				if metadata, err := read(path); err == nil || !strings.Contains(err.Error(), test.want) || metadata != nil {
					t.Fatalf("metadata=%#v, error=%v; want %q", metadata, err, test.want)
				}
			})
		}
	}
}

func TestXciRejectsMissingAndMalformedSecurePartition(t *testing.T) {
	for _, test := range []struct {
		name string
		root []byte
		want string
	}{
		{"missing", makePFS0(hfs0Magic, []string{"normal"}, [][]byte{{1}}), "secure partition not found"},
		{"malformed", makePFS0(hfs0Magic, []string{"secure"}, [][]byte{[]byte("invalid partition")}), "Invalid NSP headerBytes"},
		{"short NCA", makePFS0(hfs0Magic, []string{"secure"}, [][]byte{makePFS0(pfs0Magic, []string{"meta.cnmt.nca"}, [][]byte{{1}})}), "failed to read NCA header"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.xci")
			if err := os.WriteFile(path, syntheticXciWithRoot(test.root), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadXciMetadata(path); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("XCI error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestExtractNacpPropagatesMalformedControlErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func([]byte)
		want string
	}{
		{"invalid RomFS header", func(data []byte) { binary.LittleEndian.PutUint64(data, 0) }, "invalid RomFS header size"},
		{"truncated file table", func(data []byte) { binary.LittleEndian.PutUint64(data[0x40:], 0x1f) }, "truncated RomFS file entry"},
		{"truncated NACP", func(data []byte) { binary.LittleEndian.PutUint64(data[0x50+0x10:], 1) }, "NACP file is too small"},
	} {
		t.Run(test.name, func(t *testing.T) {
			section := makeRomfsSection(t, "control.nacp")
			test.edit(section)
			nca := makeSyntheticNCA(t, section, 0)
			cnmt := &ContentMetaAttributes{Contents: map[string]Content{"Control": {ID: "control-id"}}}
			partition := &PFS0{Files: []fileEntry{{Name: "control-id.nca"}}}
			if _, err := ExtractNacp(cnmt, bytes.NewReader(nca), partition, 0); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NACP error = %v, want %q", err, test.want)
			}
		})
	}
	cnmt := &ContentMetaAttributes{Contents: map[string]Content{"Control": {ID: "control-id"}}}
	partition := &PFS0{Files: []fileEntry{{Name: "control-id.nca"}}}
	if _, err := ExtractNacp(cnmt, bytes.NewReader(nil), partition, 0); err == nil || !strings.Contains(err.Error(), "failed to read NCA header") {
		t.Fatalf("control NCA read error = %v", err)
	}
}

func TestReadNacpRejectsInvalidCompressedTitles(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		size uint16
		want string
	}{
		{"zero length", nil, 0, "compressed NACP title data is invalid"},
		{"oversized length", nil, 0x3000, "compressed NACP title data is invalid"},
		{"corrupt stream", []byte{0xff}, 1, "failed to decompress NACP title data"},
		{"too small", compressedTitleFixture(t, 0x5fff), 0, "decompressed NACP title data is too small"},
		{"too large", compressedTitleFixture(t, 0x6001), 0, "decompressed NACP title data is too large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := make([]byte, 0x4000)
			size := test.size
			if len(test.data) > 1 {
				size = uint16(len(test.data))
			}
			binary.LittleEndian.PutUint16(data, size)
			copy(data[2:], test.data)
			data[0x3215] = 1
			if _, err := readNacp(data, RomfsHeader{}, RomfsFileEntry{size: uint64(len(data))}); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("compressed NACP error = %v, want %q", err, test.want)
			}
		})
	}
}

func syntheticXciWithRoot(root []byte) []byte {
	data := append(make([]byte, 0x200), root...)
	copy(data[0x100:], "HEAD")
	binary.LittleEndian.PutUint64(data[0x130:], 0x200)
	return data
}

func compressedTitleFixture(t *testing.T, size int) []byte {
	t.Helper()
	var output bytes.Buffer
	writer, err := flate.NewWriter(&output, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(make([]byte, size)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
