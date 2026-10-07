package process

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testTemporaryFile struct {
	file     *os.File
	writeErr error
	chmodErr error
	syncErr  error
	closeErr error
}

type testSourceFile struct {
	*os.File
	statErr  error
	closeErr error
	readErr  error
	reader   io.Reader
}

func (f testSourceFile) Stat() (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return f.File.Stat()
}

func (f testSourceFile) Close() error {
	err := f.File.Close()
	if f.closeErr != nil {
		return f.closeErr
	}
	return err
}

func (f testSourceFile) WriteTo(destination io.Writer) (int64, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	if f.reader != nil {
		return io.Copy(destination, f.reader)
	}
	return f.File.WriteTo(destination)
}

func (f testTemporaryFile) Write(data []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.file.Write(data)
}

func (f testTemporaryFile) Name() string { return f.file.Name() }
func (f testTemporaryFile) Chmod(mode os.FileMode) error {
	if f.chmodErr != nil {
		return f.chmodErr
	}
	return f.file.Chmod(mode)
}
func (f testTemporaryFile) Close() error {
	err := f.file.Close()
	if f.closeErr != nil {
		return f.closeErr
	}
	return err
}

func (f testTemporaryFile) Sync() error {
	if f.syncErr != nil {
		return f.syncErr
	}
	return f.file.Sync()
}

func TestMoveFileCrossFilesystemFallbackCopiesAndRemovesSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.nsp")
	destination := filepath.Join(root, "library", "source.nsp")
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, source, "synthetic cross-filesystem content")
	wantTime := time.Date(2020, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(source, wantTime, wantTime); err != nil {
		t.Fatal(err)
	}
	move := organizationMoveForTest(t, source, destination)

	if err := moveFileWithOperations(move, crossDeviceTestOperations(source)); err != nil {
		t.Fatal(err)
	}
	assertNotExists(t, source)
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(wantTime) {
		t.Fatalf("destination modification time = %v, want %v", info.ModTime(), wantTime)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "synthetic cross-filesystem content" {
		t.Fatalf("destination content = %q, err = %v", got, err)
	}
}

func TestMoveFileCrossFilesystemFallbackRetainsSourceOnFailure(t *testing.T) {
	writeErr := errors.New("injected write failure")
	syncErr := errors.New("injected flush failure")
	createErr := errors.New("injected create failure")
	openErr := errors.New("injected source open failure")
	statErr := errors.New("injected source inspection failure")
	readErr := errors.New("injected source read failure")
	chmodErr := errors.New("injected permission failure")
	chtimesErr := errors.New("injected timestamp failure")
	closeErr := errors.New("injected close failure")
	publishErr := errors.New("injected publish failure")
	removeErr := errors.New("injected source removal failure")
	tests := []struct {
		name                string
		configure           func(*testing.T, string, organizationFileOperations) organizationFileOperations
		wantDestination     bool
		wantDestinationText string
	}{
		{
			name: "source open failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				ops.openSource = func(string) (organizationSourceFile, error) { return nil, openErr }
				return ops
			},
		},
		{
			name: "source metadata failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				open := ops.openSource
				ops.openSource = func(path string) (organizationSourceFile, error) {
					file, err := open(path)
					if err != nil {
						return nil, err
					}
					return testSourceFile{File: file.(*os.File), statErr: statErr}, nil
				}
				return ops
			},
		},
		{
			name: "source path inspection failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				ops.lstat = func(string) (os.FileInfo, error) { return nil, statErr }
				return ops
			},
		},
		{
			name: "source read failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				open := ops.openSource
				ops.openSource = func(path string) (organizationSourceFile, error) {
					file, err := open(path)
					if err != nil {
						return nil, err
					}
					return testSourceFile{File: file.(*os.File), readErr: readErr}, nil
				}
				return ops
			},
		},
		{
			name: "copied byte count mismatch",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				open := ops.openSource
				ops.openSource = func(path string) (organizationSourceFile, error) {
					file, err := open(path)
					if err != nil {
						return nil, err
					}
					return testSourceFile{File: file.(*os.File), reader: strings.NewReader("short")}, nil
				}
				return ops
			},
		},
		{
			name: "temporary creation failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				ops.createTemp = func(string, string) (organizationTemporaryFile, error) { return nil, createErr }
				return ops
			},
		},
		{
			name: "copy write failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				create := ops.createTemp
				ops.createTemp = func(directory, pattern string) (organizationTemporaryFile, error) {
					file, err := create(directory, pattern)
					if err != nil {
						return nil, err
					}
					return testTemporaryFile{file: file.(*os.File), writeErr: writeErr}, nil
				}
				return ops
			},
		},
		{
			name: "temporary flush failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				create := ops.createTemp
				ops.createTemp = func(directory, pattern string) (organizationTemporaryFile, error) {
					file, err := create(directory, pattern)
					if err != nil {
						return nil, err
					}
					return testTemporaryFile{file: file.(*os.File), syncErr: syncErr}, nil
				}
				return ops
			},
		},
		{
			name: "permission preservation failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				create := ops.createTemp
				ops.createTemp = func(directory, pattern string) (organizationTemporaryFile, error) {
					file, err := create(directory, pattern)
					if err != nil {
						return nil, err
					}
					return testTemporaryFile{file: file.(*os.File), chmodErr: chmodErr}, nil
				}
				return ops
			},
		},
		{
			name: "timestamp preservation failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				ops.chtimes = func(string, time.Time, time.Time) error { return chtimesErr }
				return ops
			},
		},
		{
			name: "temporary close failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				create := ops.createTemp
				ops.createTemp = func(directory, pattern string) (organizationTemporaryFile, error) {
					file, err := create(directory, pattern)
					if err != nil {
						return nil, err
					}
					return testTemporaryFile{file: file.(*os.File), closeErr: closeErr}, nil
				}
				return ops
			},
		},
		{
			name: "source close failure",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				open := ops.openSource
				ops.openSource = func(path string) (organizationSourceFile, error) {
					file, err := open(path)
					if err != nil {
						return nil, err
					}
					return testSourceFile{File: file.(*os.File), closeErr: closeErr}, nil
				}
				return ops
			},
		},
		{
			name: "publish failure",
			configure: func(_ *testing.T, source string, ops organizationFileOperations) organizationFileOperations {
				defaultRename := ops.renameNoReplace
				calls := 0
				ops.renameNoReplace = func(from, to string) error {
					calls++
					if filepath.Clean(from) == filepath.Clean(source) {
						return crossDeviceTestError
					}
					if calls > 1 {
						return publishErr
					}
					return defaultRename(from, to)
				}
				return ops
			},
		},
		{
			name: "source changes during copy",
			configure: func(_ *testing.T, source string, ops organizationFileOperations) organizationFileOperations {
				defaultChtimes := ops.chtimes
				ops.chtimes = func(path string, atime, mtime time.Time) error {
					if err := defaultChtimes(path, atime, mtime); err != nil {
						return err
					}
					changed := mtime.Add(time.Minute)
					return os.Chtimes(source, changed, changed)
				}
				return ops
			},
		},
		{
			name: "destination appears before publish",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				defaultRename := ops.renameNoReplace
				calls := 0
				ops.renameNoReplace = func(from, to string) error {
					calls++
					if calls == 1 {
						return crossDeviceTestError
					}
					if err := os.WriteFile(to, []byte("preexisting late destination"), 0644); err != nil {
						return err
					}
					return defaultRename(from, to)
				}
				return ops
			},
			wantDestination:     true,
			wantDestinationText: "preexisting late destination",
		},
		{
			name: "source removal failure after publish",
			configure: func(_ *testing.T, source string, ops organizationFileOperations) organizationFileOperations {
				defaultRemove := ops.remove
				ops.remove = func(path string) error {
					if filepath.Clean(path) == filepath.Clean(source) {
						return removeErr
					}
					return defaultRemove(path)
				}
				return ops
			},
			wantDestination:     true,
			wantDestinationText: "synthetic move content",
		},
		{
			name: "source changes after publish",
			configure: func(_ *testing.T, source string, ops organizationFileOperations) organizationFileOperations {
				defaultRename := ops.renameNoReplace
				calls := 0
				ops.renameNoReplace = func(from, to string) error {
					calls++
					if filepath.Clean(from) == filepath.Clean(source) {
						return crossDeviceTestError
					}
					changed := time.Now().Add(time.Minute)
					if err := os.Chtimes(source, changed, changed); err != nil {
						return err
					}
					return defaultRename(from, to)
				}
				return ops
			},
			wantDestination:     true,
			wantDestinationText: "synthetic move content",
		},
		{
			name: "source path changes before removal check",
			configure: func(_ *testing.T, _ string, ops organizationFileOperations) organizationFileOperations {
				lstat := ops.lstat
				calls := 0
				ops.lstat = func(path string) (os.FileInfo, error) {
					calls++
					if calls == 3 {
						return nil, statErr
					}
					return lstat(path)
				}
				return ops
			},
			wantDestination:     true,
			wantDestinationText: "synthetic move content",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.nsp")
			destination := filepath.Join(root, "library", "source.nsp")
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				t.Fatal(err)
			}
			writeFixture(t, source, "synthetic move content")
			move := organizationMoveForTest(t, source, destination)
			ops := test.configure(t, source, crossDeviceTestOperations(source))

			err := moveFileWithOperations(move, ops)
			if err == nil {
				t.Fatal("moveFileWithOperations() returned nil for injected failure")
			}
			if _, err := os.Stat(source); err != nil {
				t.Fatalf("source should remain after failed move: %v", err)
			}
			destinationContents, destinationErr := os.ReadFile(destination)
			if test.wantDestination {
				if destinationErr != nil || string(destinationContents) != test.wantDestinationText {
					t.Fatalf("destination = %q, err = %v; want %q", destinationContents, destinationErr, test.wantDestinationText)
				}
			} else if !errors.Is(destinationErr, os.ErrNotExist) {
				t.Fatalf("destination should not be published after failure: read err = %v", destinationErr)
			}
			entries, err := os.ReadDir(filepath.Dir(destination))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".slm-move-") {
					t.Errorf("temporary copy %q was left behind", entry.Name())
				}
			}
		})
	}
}

var crossDeviceTestError = errors.New("injected cross-device rename")

func crossDeviceTestOperations(source string) organizationFileOperations {
	operations := defaultOrganizationFileOperations()
	defaultRename := operations.renameNoReplace
	operations.renameNoReplace = func(from, to string) error {
		if filepath.Clean(from) == filepath.Clean(source) {
			return crossDeviceTestError
		}
		return defaultRename(from, to)
	}
	operations.isCrossDevice = func(err error) bool { return errors.Is(err, crossDeviceTestError) }
	return operations
}

func organizationMoveForTest(t *testing.T, source, destination string) OrganizationMove {
	t.Helper()
	info, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	return OrganizationMove{Source: source, Destination: destination, SourceSize: info.Size(), SourceStamp: info}
}
