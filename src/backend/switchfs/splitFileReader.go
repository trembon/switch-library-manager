package switchfs

import (
	"errors"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/avast/retry-go/v5"
)

type ReadAtCloser interface {
	io.ReaderAt
	io.Closer
}

type splitFile struct {
	info      []os.FileInfo
	files     []ReadAtCloser
	path      string
	chunkSize int64
}

type fileWrapper struct {
	file ReadAtCloser
	path string
}

func NewFileWrapper(filePath string) (*fileWrapper, error) {
	result := fileWrapper{}
	result.path = filePath
	file, err := _openFile(filePath)
	if err != nil {
		return nil, err
	}
	result.file = file
	return &result, nil
}

func (sp *fileWrapper) ReadAt(p []byte, off int64) (n int, err error) {
	if sp.file != nil {
		return sp.file.ReadAt(p, off)
	}
	return 0, errors.New("file is not opened")
}

func (sp *fileWrapper) Close() error {

	if sp.file != nil {
		return sp.file.Close()
	}

	return nil
}

func NewSplitFileReader(filePath string) (*splitFile, error) {
	result := splitFile{}
	index := strings.LastIndex(filePath, string(os.PathSeparator))
	if index < 0 {
		return nil, errors.New("split file must have a parent directory")
	}
	splitFileFolder := filePath[:index]
	files, err := os.ReadDir(splitFileFolder)
	if err != nil {
		return nil, err
	}
	baseName := filePath[index+1:]
	baseEnd := len(baseName)
	requestedPart := -1
	for baseEnd > 0 && baseName[baseEnd-1] >= '0' && baseName[baseEnd-1] <= '9' {
		baseEnd--
	}
	if baseEnd < len(baseName) {
		requestedPart, err = strconv.Atoi(baseName[baseEnd:])
		if err != nil {
			return nil, errors.New("invalid split file part")
		}
	}
	partPrefix := baseName[:baseEnd]
	for _, file := range files {
		name := file.Name()
		if !strings.HasPrefix(name, partPrefix) || len(name) == len(partPrefix) {
			continue
		}
		partNumber, parseErr := strconv.Atoi(name[len(partPrefix):])
		if parseErr == nil && partNumber >= 0 {
			info, err := file.Info()
			if err != nil {
				return nil, err
			}
			result.info = append(result.info, info)
		}
	}
	if len(result.info) == 0 {
		return nil, errors.New("no split file parts found")
	}
	sort.Slice(result.info, func(i, j int) bool {
		iPart, _ := strconv.Atoi(result.info[i].Name()[len(partPrefix):])
		jPart, _ := strconv.Atoi(result.info[j].Name()[len(partPrefix):])
		return iPart < jPart
	})
	for i, info := range result.info {
		partNumber, _ := strconv.Atoi(info.Name()[len(partPrefix):])
		if partNumber != i {
			return nil, errors.New("missing split file part " + strconv.Itoa(i))
		}
	}
	if requestedPart >= len(result.info) {
		return nil, errors.New("requested split file part is missing")
	}
	result.path = splitFileFolder
	result.chunkSize = result.info[0].Size()
	if result.chunkSize <= 0 {
		return nil, errors.New("split file parts must not be empty")
	}
	for i, info := range result.info {
		if info.Size() != result.chunkSize && i < len(result.info)-1 {
			return nil, errors.New("split file parts have inconsistent sizes")
		}
	}
	result.files = make([]ReadAtCloser, len(result.info))
	return &result, nil
}

func (sp *splitFile) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("offset is out of bounds")
	}
	if len(p) == 0 {
		return 0, nil
	}
	if sp.chunkSize <= 0 {
		return 0, errors.New("invalid split file chunk size")
	}

	for len(p) > 0 {
		part := off / sp.chunkSize
		if part >= int64(len(sp.info)) {
			if n == 0 {
				return 0, errors.New("missing part " + strconv.FormatInt(part, 10))
			}
			return n, io.EOF
		}
		partOffset := off % sp.chunkSize
		partIndex := int(part)
		if sp.files[partIndex] == nil {
			file, openErr := _openFile(path.Join(sp.path, sp.info[partIndex].Name()))
			if openErr != nil {
				if n == 0 {
					return 0, openErr
				}
				return n, openErr
			}
			sp.files[partIndex] = file
		}

		available := sp.info[partIndex].Size() - partOffset
		if available <= 0 {
			return n, io.EOF
		}
		want := int64(len(p))
		if want > available {
			want = available
		}
		read, readErr := sp.files[partIndex].ReadAt(p[:int(want)], partOffset)
		n += read
		off += int64(read)
		p = p[read:]
		if readErr != nil {
			if len(p) == 0 && readErr == io.EOF {
				return n, nil
			}
			return n, readErr
		}
		if read < int(want) {
			return n, io.ErrUnexpectedEOF
		}
	}
	return n, nil
}

func _openFile(path string) (*os.File, error) {
	var file *os.File
	var err error
	err = retry.New(
		retry.Attempts(5),
	).Do(
		func() error {
			file, err = os.Open(path)
			return err
		},
	)
	return file, err
}

func (sp *splitFile) Close() error {
	var closeErr error
	for _, file := range sp.files {
		if file != nil {
			if err := file.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
	}
	return closeErr
}

func OpenFile(filePath string) (ReadAtCloser, error) {
	//check if it's a split file
	if filePath == "" {
		return nil, errors.New("empty file path")
	}
	if _, err := strconv.Atoi(filePath[len(filePath)-1:]); err == nil {
		return NewSplitFileReader(filePath)
	} else {
		return NewFileWrapper(filePath)
	}
}
