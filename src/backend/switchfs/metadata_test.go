package switchfs

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"testing"
)

func makeCNMT(titleID uint64, version uint32, metaType byte, contentIDs ...[16]byte) []byte {
	data := make([]byte, 0x20+len(contentIDs)*0x38)
	binary.LittleEndian.PutUint64(data[0:8], titleID)
	binary.LittleEndian.PutUint32(data[8:12], version)
	data[0xC] = metaType
	for i, id := range contentIDs {
		position := 0x20 + i*0x38
		copy(data[position+0x20:position+0x30], id[:])
		data[position+0x36] = byte(i + 1)
	}
	binary.LittleEndian.PutUint16(data[0x10:0x12], uint16(len(contentIDs)))
	return data
}

func TestReadBinaryCnmt(t *testing.T) {
	id := [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	cnmtData := makeCNMT(0x0102030405060708, 42, ContentMetaType_Application, id)
	container := makePFS0(pfs0Magic, []string{"x.cnmt.nca"}, [][]byte{cnmtData})
	parsed, err := readPfs0(bytes.NewReader(container), 0)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := readBinaryCnmt(parsed, container)
	if err != nil {
		t.Fatal(err)
	}
	if meta.TitleId != "0102030405060708" || meta.Version != 42 || meta.Type != "BASE" {
		t.Fatalf("unexpected metadata: %#v", meta)
	}
	if meta.Contents["Program"].ID != "000102030405060708090a0b0c0d0e0f" {
		t.Fatalf("unexpected content: %#v", meta.Contents)
	}
}

func TestReadBinaryCnmtRejectsTruncation(t *testing.T) {
	tests := [][]byte{nil, make([]byte, 0x1f)}
	for _, data := range tests {
		t.Run(string(rune(len(data))), func(t *testing.T) {
			pfs := &PFS0{Files: []fileEntry{{StartOffset: 0, Size: uint64(len(data))}}}
			if _, err := readBinaryCnmt(pfs, data); err == nil {
				t.Fatal("expected truncated CNMT error")
			}
		})
	}
	data := make([]byte, 0x20)
	binary.LittleEndian.PutUint16(data[0x10:0x12], 1)
	pfs := &PFS0{Files: []fileEntry{{StartOffset: 0, Size: uint64(len(data))}}}
	if _, err := readBinaryCnmt(pfs, data); err == nil {
		t.Fatal("expected content table bounds error")
	}
	if _, err := readBinaryCnmt(&PFS0{}, make([]byte, 0x20)); err == nil {
		t.Fatal("expected unexpected PFS0 error")
	}
	data = makeCNMT(1, 1, ContentMetaType_Patch, [16]byte{1})
	data[0x20+0x36] = 7
	pfs = &PFS0{Files: []fileEntry{{StartOffset: 0, Size: uint64(len(data))}}}
	meta, err := readBinaryCnmt(pfs, data)
	if err != nil || meta.Type != "UPD" || meta.Contents[""].ID == "" {
		t.Fatalf("unexpected patch metadata: %#v %v", meta, err)
	}
}

func TestReadXmlCnmtTitleID(t *testing.T) {
	valid := []byte(`<ContentMeta><Id>0XABCDEF0123456789</Id><Version>7</Version><Type>BASE</Type></ContentMeta>`)
	meta, err := readXmlCnmt(valid)
	if err != nil || meta.TitleId != "abcdef0123456789" || meta.Version != 7 {
		t.Fatalf("unexpected XML metadata: %#v, %v", meta, err)
	}
	for _, id := range []string{"0x12", "not-a-title-id"} {
		data, _ := xml.Marshal(ContentMeta{ID: id})
		if _, err := readXmlCnmt(data); err == nil {
			t.Fatalf("expected invalid title ID error for %q", id)
		}
	}
	if _, err := readXmlCnmt([]byte("<ContentMeta>")); err == nil {
		t.Fatal("expected malformed XML error")
	}
}
