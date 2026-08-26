package switchfs

import (
	"encoding/binary"
	"testing"
)

func putRomfsHeader(data []byte, header RomfsHeader) {
	values := []uint64{header.HeaderSize, header.DirHashTableOffset, header.DirHashTableSize, header.DirMetaTableOffset, header.DirMetaTableSize, header.FileHashTableOffset, header.FileHashTableSize, header.FileMetaTableOffset, header.FileMetaTableSize, header.DataOffset}
	for i, value := range values {
		binary.LittleEndian.PutUint64(data[i*8:], value)
	}
}

func TestReadRomfsHeaderAndFileEntry(t *testing.T) {
	name := []uint16{'c', 'o', 'n', 't', 'r', 'o', 'l', '.', 'n', 'a', 'c', 'p'}
	entry := make([]byte, 0x20+len(name)*2)
	binary.LittleEndian.PutUint32(entry[0x1C:0x20], uint32(len(name)*2))
	for i, value := range name {
		binary.LittleEndian.PutUint16(entry[0x20+i*2:], value)
	}
	data := make([]byte, 0x100+len(entry))
	header := RomfsHeader{HeaderSize: 0x50, FileMetaTableOffset: 0x50, FileMetaTableSize: uint64(len(entry)), DataOffset: 0x100}
	putRomfsHeader(data, header)
	copy(data[0x50:], entry)
	gotHeader, err := readRomfsHeader(data)
	if err != nil || gotHeader != header {
		t.Fatalf("unexpected RomFS header: %#v, %v", gotHeader, err)
	}
	entries, err := readRomfsFileEntry(data, header)
	if err != nil {
		t.Fatal(err)
	}
	if entries["control.nacp"].name != "control.nacp" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestRomfsMalformedInput(t *testing.T) {
	if _, err := readRomfsHeader(make([]byte, 0x4f)); err == nil {
		t.Fatal("expected short header error")
	}
	base := RomfsHeader{FileMetaTableOffset: 0x50, FileMetaTableSize: 0x20}
	if _, err := readRomfsFileEntry(make([]byte, 0x50), base); err == nil {
		t.Fatal("expected table bounds error")
	}
	data := make([]byte, 0x70)
	binary.LittleEndian.PutUint32(data[0x50+0x1C:], 3)
	base.FileMetaTableSize = 0x20
	if _, err := readRomfsFileEntry(data, base); err == nil {
		t.Fatal("expected malformed name error")
	}
	data = make([]byte, 0x90)
	binary.LittleEndian.PutUint32(data[0x50+0x1C:], 0x40)
	base.FileMetaTableSize = 0x40
	if _, err := readRomfsFileEntry(data, base); err == nil {
		t.Fatal("expected truncated name error")
	}
	for _, test := range []struct {
		name string
		edit func(*RomfsHeader)
	}{
		{name: "header size", edit: func(header *RomfsHeader) { header.HeaderSize = 0x40 }},
		{name: "table", edit: func(header *RomfsHeader) { header.DirMetaTableOffset = 0x80; header.DirMetaTableSize = 0x81 }},
		{name: "data", edit: func(header *RomfsHeader) { header.DataOffset = 0x101 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := make([]byte, 0x100)
			header := RomfsHeader{HeaderSize: 0x50}
			test.edit(&header)
			putRomfsHeader(data, header)
			if _, err := readRomfsHeader(data); err == nil {
				t.Fatal("expected malformed RomFS header error")
			}
		})
	}
}

func TestReadNacp(t *testing.T) {
	data := make([]byte, 0x3080)
	copy(data[0x3000:], []byte("123456789"))
	copy(data[0x3060:], []byte("1.2.3"))
	binary.LittleEndian.PutUint32(data[0x302C:], 0x11223344)
	copy(data, []byte("Example Game"))
	entry := RomfsFileEntry{offset: 0, size: uint64(len(data))}
	nacp, err := readNacp(data, RomfsHeader{DataOffset: 0}, entry)
	if err != nil {
		t.Fatal(err)
	}
	if nacp.TitleName["AmericanEnglish"].Title != "Example Game" || nacp.Isbn != "123456789" || nacp.DisplayVersion != "1.2.3" || nacp.SupportedLanguageFlag != 0x11223344 {
		t.Fatalf("unexpected NACP: %#v", nacp)
	}
	for _, entry := range []RomfsFileEntry{{offset: 1, size: 0x3080}, {offset: 0, size: 0x307f}} {
		if _, err := readNacp(data, RomfsHeader{DataOffset: 0}, entry); err == nil {
			t.Fatal("expected NACP bounds error")
		}
	}
}

func TestLanguageStringOutOfRange(t *testing.T) {
	if Language(-1).String() != "Unknown" || Language(99).String() != "Unknown" {
		t.Fatal("unexpected out-of-range language string")
	}
}
