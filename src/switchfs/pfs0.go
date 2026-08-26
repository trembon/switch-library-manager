package switchfs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	PfsfileEntryTableSize = 0x18
	HfsfileEntryTableSize = 0x40
	pfs0Magic             = "PFS0"
	hfs0Magic             = "HFS0"
	maxPfs0FileCount      = 1 << 20
)

type fileEntry struct {
	StartOffset uint64
	Size        uint64
	Name        string
}

// PFS0 struct to represent PFS0 filesystem of NSP
type PFS0 struct {
	Filepath  string
	Size      uint64
	HeaderLen uint64
	Files     []fileEntry
}

// https://wiki.oatmealdome.me/PFS0_(File_Format)
func ReadPfs0File(filePath string) (*PFS0, error) {

	file, err := OpenFile(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	p, err := readPfs0(file, 0x0)
	if err != nil {
		return nil, err
	}
	p.Filepath = filePath
	return p, nil
}

func readPfs0(reader io.ReaderAt, offset int64) (*PFS0, error) {
	if reader == nil {
		return nil, errors.New("nil reader")
	}
	if offset < 0 {
		return nil, errors.New("negative PFS0 offset")
	}

	header := make([]byte, 0xC)
	if err := readAtFull(reader, header, offset); err != nil {
		return nil, fmt.Errorf("failed to read PFS0 header: %w", err)
	}
	var fileEntryTableSize uint64
	if string(header[:0x4]) == pfs0Magic {
		fileEntryTableSize = uint64(PfsfileEntryTableSize)
	} else if string(header[:0x4]) == hfs0Magic {
		fileEntryTableSize = uint64(HfsfileEntryTableSize)
	} else {
		return nil, errors.New("Invalid NSP headerBytes. Expected 'PFS0'/'HFS0', got '" + string(header[:0x4]) + "'")
	}
	p := &PFS0{}

	fileCount := binary.LittleEndian.Uint32(header[0x4:0x8])
	if fileCount > maxPfs0FileCount {
		return nil, errors.New("PFS0 contains too many files")
	}

	fileEntryTableOffset := uint64(0x10) + fileEntryTableSize*uint64(fileCount)

	stringsLen := uint64(binary.LittleEndian.Uint32(header[0x8:0xC]))
	if fileEntryTableOffset > math.MaxInt64 || stringsLen > math.MaxInt64-fileEntryTableOffset || uint64(offset) > math.MaxInt64-fileEntryTableOffset-stringsLen {
		return nil, errors.New("PFS0 header size overflows reader offset")
	}
	p.HeaderLen = fileEntryTableOffset + stringsLen
	if stringsLen > uint64(maxInt()) {
		return nil, errors.New("PFS0 string table is too large")
	}
	fileNamesBuffer := make([]byte, int(stringsLen))
	if err := readAtFull(reader, fileNamesBuffer, offset+int64(fileEntryTableOffset)); err != nil {
		return nil, fmt.Errorf("failed to read PFS0 string table: %w", err)
	}

	p.Files = make([]fileEntry, 0, int(fileCount))
	// go over the fileEntries
	for i := uint32(0); i < fileCount; i++ {
		fileEntryTable := make([]byte, int(fileEntryTableSize))
		entryOffset := uint64(0x10) + fileEntryTableSize*uint64(i)
		if err := readAtFull(reader, fileEntryTable, offset+int64(entryOffset)); err != nil {
			return nil, fmt.Errorf("failed to read PFS0 file entry %d: %w", i, err)
		}

		fileOffset := binary.LittleEndian.Uint64(fileEntryTable[0:8])
		fileSize := binary.LittleEndian.Uint64(fileEntryTable[8:16])
		nameOffset := uint64(binary.LittleEndian.Uint32(fileEntryTable[16:20]))
		if nameOffset > stringsLen {
			return nil, fmt.Errorf("PFS0 file entry %d has an invalid name offset", i)
		}
		var nameBytes []byte
		for _, b := range fileNamesBuffer[nameOffset:] {
			if b == 0x0 {
				break
			} else {
				nameBytes = append(nameBytes, b)
			}
		}

		if fileOffset > math.MaxUint64-p.HeaderLen {
			return nil, fmt.Errorf("PFS0 file entry %d offset overflows", i)
		}
		p.Files = append(p.Files, fileEntry{fileOffset + p.HeaderLen, fileSize, string(nameBytes)})
	}

	return p, nil
}

func readAtFull(reader io.ReaderAt, p []byte, offset int64) error {
	if len(p) == 0 {
		return nil
	}
	n, err := reader.ReadAt(p, offset)
	if n != len(p) {
		if err != nil {
			return fmt.Errorf("read %d of %d bytes: %w", n, len(p), err)
		}
		return fmt.Errorf("read %d of %d bytes", n, len(p))
	}
	if err != nil {
		return err
	}
	return nil
}

func maxInt() int {
	return int(^uint(0) >> 1)
}
