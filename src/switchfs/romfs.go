package switchfs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"
)

type RomfsHeader struct {
	HeaderSize          uint64
	DirHashTableOffset  uint64
	DirHashTableSize    uint64
	DirMetaTableOffset  uint64
	DirMetaTableSize    uint64
	FileHashTableOffset uint64
	FileHashTableSize   uint64
	FileMetaTableOffset uint64
	FileMetaTableSize   uint64
	DataOffset          uint64
}

type RomfsFileEntry struct {
	parent    uint32
	sibling   uint32
	offset    uint64
	size      uint64
	hash      uint32
	name_size uint32
	name      string
}

func readRomfsHeader(data []byte) (RomfsHeader, error) {
	if len(data) < 0x50 {
		return RomfsHeader{}, errors.New("RomFS header is truncated")
	}
	header := RomfsHeader{}
	header.HeaderSize = binary.LittleEndian.Uint64(data[0x0+(0x8*0) : 0x0+(0x8*1)])
	header.DirHashTableOffset = binary.LittleEndian.Uint64(data[0x0+(0x8*1) : 0x0+(0x8*2)])
	header.DirHashTableSize = binary.LittleEndian.Uint64(data[0x0+(0x8*2) : 0x0+(0x8*3)])
	header.DirMetaTableOffset = binary.LittleEndian.Uint64(data[0x0+(0x8*3) : 0x0+(0x8*4)])
	header.DirMetaTableSize = binary.LittleEndian.Uint64(data[0x0+(0x8*4) : 0x0+(0x8*5)])
	header.FileHashTableOffset = binary.LittleEndian.Uint64(data[0x0+(0x8*5) : 0x0+(0x8*6)])
	header.FileHashTableSize = binary.LittleEndian.Uint64(data[0x0+(0x8*6) : 0x0+(0x8*7)])
	header.FileMetaTableOffset = binary.LittleEndian.Uint64(data[0x0+(0x8*7) : 0x0+(0x8*8)])
	header.FileMetaTableSize = binary.LittleEndian.Uint64(data[0x0+(0x8*8) : 0x0+(0x8*9)])
	header.DataOffset = binary.LittleEndian.Uint64(data[0x0+(0x8*9) : 0x0+(0x8*10)])
	if header.HeaderSize < 0x50 || header.HeaderSize > uint64(len(data)) {
		return RomfsHeader{}, errors.New("invalid RomFS header size")
	}
	for _, table := range [][2]uint64{
		{header.DirHashTableOffset, header.DirHashTableSize},
		{header.DirMetaTableOffset, header.DirMetaTableSize},
		{header.FileHashTableOffset, header.FileHashTableSize},
		{header.FileMetaTableOffset, header.FileMetaTableSize},
	} {
		if table[0] > uint64(len(data)) || table[1] > uint64(len(data))-table[0] {
			return RomfsHeader{}, errors.New("RomFS table is outside the data")
		}
	}
	if header.DataOffset > uint64(len(data)) {
		return RomfsHeader{}, errors.New("RomFS data offset is outside the data")
	}
	return header, nil
}

func readRomfsFileEntry(data []byte, header RomfsHeader) (map[string]RomfsFileEntry, error) {
	if header.FileMetaTableOffset > uint64(len(data)) || header.FileMetaTableSize > uint64(len(data))-header.FileMetaTableOffset {
		return nil, errors.New("failed to read romfs")
	}
	dirBytes := data[int(header.FileMetaTableOffset):int(header.FileMetaTableOffset+header.FileMetaTableSize)]
	result := map[string]RomfsFileEntry{}
	for offset := uint64(0); offset < uint64(len(dirBytes)); {
		if uint64(len(dirBytes))-offset < 0x20 {
			return nil, errors.New("truncated RomFS file entry")
		}
		entry := RomfsFileEntry{}
		entry.parent = binary.LittleEndian.Uint32(dirBytes[offset : offset+0x4])
		entry.sibling = binary.LittleEndian.Uint32(dirBytes[offset+0x4 : offset+0x8])
		entry.offset = binary.LittleEndian.Uint64(dirBytes[offset+0x8 : offset+0x10])
		entry.size = binary.LittleEndian.Uint64(dirBytes[offset+0x10 : offset+0x18])
		entry.hash = binary.LittleEndian.Uint32(dirBytes[offset+0x18 : offset+0x1C])
		entry.name_size = binary.LittleEndian.Uint32(dirBytes[offset+0x1C : offset+0x20])
		nameEnd := offset + 0x20 + uint64(entry.name_size)
		if nameEnd < offset || nameEnd > uint64(len(dirBytes)) || entry.name_size%2 != 0 {
			return nil, fmt.Errorf("invalid RomFS file name at offset %d", offset)
		}
		name16 := make([]uint16, entry.name_size/2)
		for i := range name16 {
			name16[i] = binary.LittleEndian.Uint16(dirBytes[offset+0x20+uint64(i)*2:])
		}
		entry.name = string(utf16.Decode(name16))
		result[entry.name] = entry
		offset = nameEnd
	}
	return result, nil

	//fmt.Println(string(section[DataOffset+offset+0x3060:DataOffset+offset+0x3060 +0x10]))
}
