package fileio

import (
	"errors"
	"io"

	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func ReadSplitFileMetadata(filePath string) (map[string]*switchfs.ContentMetaAttributes, error) {
	//check if this is a NS* or XC* file
	_, err := switchfs.ReadPfs0File(filePath)
	isXCI := false
	if err != nil {
		_, err = readXciHeader(filePath)
		if err != nil {
			return nil, errors.New("split file is not an XCI/XCZ or NSP/NSZ")
		}
		isXCI = true
	}

	if isXCI {
		return switchfs.ReadXciMetadata(filePath)
	} else {
		return switchfs.ReadNspMetadata(filePath)
	}
}

func readXciHeader(filePath string) ([]byte, error) {
	file, err := switchfs.OpenFile(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	header := make([]byte, 0x200)
	n, err := file.ReadAt(header, 0)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n != len(header) {
		return nil, errors.New("truncated XCI header")
	}

	if string(header[0x100:0x104]) != "HEAD" {
		return nil, errors.New("not an XCI/XCZ file")
	}
	return header, nil
}
