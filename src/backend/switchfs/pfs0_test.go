package switchfs

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

func makePFS0(magic string, names []string, payloads [][]byte) []byte {
	entrySize := PfsfileEntryTableSize
	if magic == hfs0Magic {
		entrySize = HfsfileEntryTableSize
	}
	var stringTable []byte
	nameOffsets := make([]uint32, len(names))
	for i, name := range names {
		nameOffsets[i] = uint32(len(stringTable))
		stringTable = append(stringTable, []byte(name)...)
		stringTable = append(stringTable, 0)
	}
	headerLen := 0x10 + entrySize*len(names) + len(stringTable)
	result := make([]byte, headerLen)
	copy(result, magic)
	binary.LittleEndian.PutUint32(result[4:8], uint32(len(names)))
	binary.LittleEndian.PutUint32(result[8:12], uint32(len(stringTable)))
	offset := 0
	for i, payload := range payloads {
		entry := result[0x10+i*entrySize:]
		binary.LittleEndian.PutUint64(entry[0:8], uint64(offset))
		binary.LittleEndian.PutUint64(entry[8:16], uint64(len(payload)))
		binary.LittleEndian.PutUint32(entry[16:20], nameOffsets[i])
		offset += len(payload)
	}
	copy(result[0x10+entrySize*len(names):], stringTable)
	for _, payload := range payloads {
		result = append(result, payload...)
	}
	return result
}

func TestReadPfs0AndHfs0(t *testing.T) {
	for _, magic := range []string{pfs0Magic, hfs0Magic} {
		t.Run(magic, func(t *testing.T) {
			data := makePFS0(magic, []string{"a.nca", "b.nca"}, [][]byte{{1, 2}, {3, 4, 5}})
			parsed, err := readPfs0(bytes.NewReader(data), 0)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.HeaderLen != uint64(len(data)-5) || len(parsed.Files) != 2 {
				t.Fatalf("unexpected PFS0 metadata: %#v", parsed)
			}
			if parsed.Files[1].Name != "b.nca" || parsed.Files[1].StartOffset != uint64(len(data)-3) {
				t.Fatalf("unexpected second entry: %#v", parsed.Files[1])
			}
		})
	}
}

func TestReadPfs0RejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"short header", []byte("PFS")},
		{"bad magic", make([]byte, 0xC)},
		{"short entry", func() []byte {
			data := make([]byte, 0x10)
			copy(data, pfs0Magic)
			binary.LittleEndian.PutUint32(data[4:8], 1)
			return data
		}()},
		{"bad name offset", func() []byte {
			data := makePFS0(pfs0Magic, []string{"x"}, [][]byte{{}})
			binary.LittleEndian.PutUint32(data[0x10+16:0x10+20], 99)
			return data
		}()},
		{"unterminated name", func() []byte {
			data := makePFS0(pfs0Magic, []string{"x"}, [][]byte{{}})
			data[0x10+16] = 0
			data[0x10+16+1] = 0
			data[len(data)-2] = 'x'
			data = data[:len(data)-1]
			return data
		}()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("parser panicked: %v", recovered)
				}
			}()
			if _, err := readPfs0(bytes.NewReader(test.data), 0); err == nil {
				t.Fatal("expected malformed input error")
			}
		})
	}
}

func TestReadPfs0OffsetAndReaderErrors(t *testing.T) {
	data := append([]byte("prefix"), makePFS0(pfs0Magic, []string{"x"}, [][]byte{{1}})...)
	if _, err := readPfs0(bytes.NewReader(data), int64(len("prefix"))); err != nil {
		t.Fatal(err)
	}
	if _, err := readPfs0(bytes.NewReader(data), -1); err == nil {
		t.Fatal("expected negative offset error")
	}
	if _, err := readPfs0(strings.NewReader(""), 0); err == nil {
		t.Fatal("expected reader error")
	}
}

func TestReadPfs0OverflowAndReadAtFull(t *testing.T) {
	data := makePFS0(pfs0Magic, []string{"x"}, [][]byte{{}})
	binary.LittleEndian.PutUint64(data[0x10:], ^uint64(0))
	if _, err := readPfs0(bytes.NewReader(data), 0); err == nil {
		t.Fatal("expected PFS0 entry offset overflow")
	}
	if err := readAtFull(bytes.NewReader([]byte{1}), make([]byte, 2), 0); err == nil {
		t.Fatal("expected short read error")
	}
	if err := readAtFull(bytes.NewReader([]byte{1}), make([]byte, 0), 0); err != nil {
		t.Fatal(err)
	}
	if err := readAtFull(fullReadErrorReader{}, make([]byte, 1), 0); err == nil {
		t.Fatal("expected error returned after a full read")
	}
}

type fullReadErrorReader struct{}

func (fullReadErrorReader) ReadAt(p []byte, _ int64) (int, error) {
	if len(p) > 0 {
		p[0] = 1
	}
	return len(p), io.ErrUnexpectedEOF
}
