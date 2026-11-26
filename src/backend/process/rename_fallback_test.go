package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var unsupportedRenameTestError = errors.New("injected unsupported exclusive rename")

func TestIsUnsupportedExclusiveRenameError(t *testing.T) {
	// unsupported lists the platforms whose exclusive rename reports the error
	// when a filesystem cannot honor the no-replace flag.
	for _, test := range []struct {
		name        string
		err         error
		unsupported []string
	}{
		{"ENOTSUP", syscall.ENOTSUP, []string{"darwin", "linux"}},
		{"EOPNOTSUPP", syscall.EOPNOTSUPP, []string{"darwin", "linux"}},
		{"wrapped ENOTSUP", fmt.Errorf("rename: %w", syscall.ENOTSUP), []string{"darwin", "linux"}},
		{"wrapped EOPNOTSUPP", &os.LinkError{Op: "rename", Err: syscall.EOPNOTSUPP}, []string{"darwin", "linux"}},
		{"invalid", syscall.EINVAL, []string{"darwin", "linux"}},
		{"wrapped invalid", &os.LinkError{Op: "rename", Err: syscall.EINVAL}, []string{"darwin", "linux"}},
		{"no renameat2", syscall.ENOSYS, []string{"linux"}},
		{"same message", errors.New(syscall.ENOTSUP.Error()), nil},
		{"generic unsupported", errors.ErrUnsupported, nil},
		{"permission", syscall.EACCES, nil},
		{"exists", syscall.EEXIST, nil},
		{"cross device", syscall.EXDEV, nil},
		{"nil", nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			want := slices.Contains(test.unsupported, runtime.GOOS)
			if got := isUnsupportedExclusiveRenameError(test.err); got != want {
				t.Fatalf("isUnsupportedExclusiveRenameError(%v) = %v, want %v", test.err, got, want)
			}
		})
	}
}

func TestMoveFileRenameFallback(t *testing.T) {
	renameErr := errors.New("injected standard rename failure")
	for _, crossFilesystem := range []bool{false, true} {
		mode := "direct"
		if crossFilesystem {
			mode = "cross filesystem publication"
		}
		t.Run(mode, func(t *testing.T) {
			for _, test := range []struct {
				name            string
				destinationKind string
				inspectionErr   error
				renameErr       error
				wantErr         error
				wantRenameCalls int
			}{
				{name: "success", wantRenameCalls: 1},
				{name: "existing file", destinationKind: "file", wantErr: os.ErrExist},
				{name: "existing directory", destinationKind: "directory", wantErr: os.ErrExist},
				{name: "dangling symlink", destinationKind: "symlink", wantErr: os.ErrExist},
				{name: "destination appears after preflight", destinationKind: "late file", wantErr: os.ErrExist},
				{name: "inspection failure", inspectionErr: os.ErrPermission, wantErr: os.ErrPermission},
				{name: "rename failure", renameErr: renameErr, wantErr: renameErr, wantRenameCalls: 1},
				{name: "unsupported standard rename is not retried", renameErr: unsupportedRenameTestError, wantErr: unsupportedRenameTestError, wantRenameCalls: 1},
			} {
				t.Run(test.name, func(t *testing.T) {
					root := t.TempDir()
					source := filepath.Join(root, "source.nsp")
					destination := filepath.Join(root, "library", "destination.nsp")
					if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
						t.Fatal(err)
					}
					writeFixture(t, source, "synthetic move content")
					move := organizationMoveForTest(t, source, destination)
					switch test.destinationKind {
					case "file":
						writeFixture(t, destination, "existing content")
					case "directory":
						if err := os.Mkdir(destination, 0755); err != nil {
							t.Fatal(err)
						}
					case "symlink":
						if err := os.Symlink("missing.nsp", destination); err != nil {
							if runtime.GOOS == "windows" {
								t.Skipf("creating symlinks requires Windows privileges: %v", err)
							}
							t.Fatal(err)
						}
					}

					ops := defaultOrganizationFileOperations()
					exclusiveCalls, renameCalls, logCalls := 0, 0, 0
					inspectedDestination := false
					ops.renameNoReplace = func(from, to string) error {
						exclusiveCalls++
						if crossFilesystem && from == source {
							return crossDeviceTestError
						}
						if test.destinationKind == "late file" {
							writeFixture(t, to, "existing content")
						}
						return &os.LinkError{Op: "rename", Old: from, New: to, Err: unsupportedRenameTestError}
					}
					ops.isRenameUnsupported = func(err error) bool { return errors.Is(err, unsupportedRenameTestError) }
					ops.isCrossDevice = func(err error) bool { return errors.Is(err, crossDeviceTestError) }
					ops.lstat = func(path string) (os.FileInfo, error) {
						if path == destination {
							inspectedDestination = true
							if test.inspectionErr != nil {
								return nil, &os.PathError{Op: "lstat", Path: path, Err: test.inspectionErr}
							}
						}
						return os.Lstat(path)
					}
					ops.rename = func(from, to string) error {
						renameCalls++
						if !inspectedDestination {
							t.Fatal("standard rename attempted before inspecting destination")
						}
						if test.renameErr != nil {
							return &os.LinkError{Op: "rename", Old: from, New: to, Err: test.renameErr}
						}
						return os.Rename(from, to)
					}
					ops.logRenameFallback = func(from, to string) {
						logCalls++
						if to != destination || (!crossFilesystem && from != source) ||
							(crossFilesystem && !strings.HasPrefix(filepath.Base(from), ".slm-move-")) {
							t.Fatalf("unexpected fallback log paths: %q -> %q", from, to)
						}
					}

					err := moveFileWithOperations(move, ops)
					if !errors.Is(err, test.wantErr) {
						t.Fatalf("moveFileWithOperations() error = %v, want %v", err, test.wantErr)
					}
					wantExclusiveCalls := 1
					if crossFilesystem {
						wantExclusiveCalls = 2
					}
					if exclusiveCalls != wantExclusiveCalls || renameCalls != test.wantRenameCalls || logCalls != 1 || !inspectedDestination {
						t.Fatalf("exclusive calls = %d, standard calls = %d, logs = %d, destination inspected = %v", exclusiveCalls, renameCalls, logCalls, inspectedDestination)
					}
					if test.wantErr == nil {
						assertNotExists(t, source)
						assertRenameFallbackContents(t, destination, "synthetic move content")
					} else {
						assertRenameFallbackContents(t, source, "synthetic move content")
						switch test.destinationKind {
						case "file", "late file":
							assertRenameFallbackContents(t, destination, "existing content")
						case "directory":
							if info, err := os.Lstat(destination); err != nil || !info.IsDir() {
								t.Fatalf("destination directory was changed: info = %v, err = %v", info, err)
							}
						case "symlink":
							if target, err := os.Readlink(destination); err != nil || target != "missing.nsp" {
								t.Fatalf("destination symlink was changed: target = %q, err = %v", target, err)
							}
						default:
							assertNotExists(t, destination)
						}
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
		})
	}
}

func TestRenameOrganizationFileDoesNotFallbackForOtherResults(t *testing.T) {
	for _, result := range []error{nil, os.ErrExist, os.ErrPermission, syscall.EINVAL, syscall.EIO, syscall.EXDEV, errors.New("operation not supported")} {
		t.Run(fmt.Sprint(result), func(t *testing.T) {
			ops := defaultOrganizationFileOperations()
			ops.renameNoReplace = func(string, string) error { return result }
			ops.isRenameUnsupported = func(err error) bool { return errors.Is(err, unsupportedRenameTestError) }
			ops.lstat = func(string) (os.FileInfo, error) {
				t.Fatal("unexpected fallback destination check")
				return nil, nil
			}
			ops.rename = func(string, string) error {
				t.Fatal("unexpected standard rename")
				return nil
			}
			ops.logRenameFallback = func(string, string) { t.Fatal("unexpected fallback log") }
			if err := renameOrganizationFile("source.nsp", "destination.nsp", ops); !errors.Is(err, result) {
				t.Fatalf("renameOrganizationFile() error = %v, want %v", err, result)
			}
		})
	}
}

func TestMoveFileRenameFallbackCanReturnCrossDevice(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.nsp")
	destination := filepath.Join(root, "destination.nsp")
	writeFixture(t, source, "synthetic cross-filesystem content")
	move := organizationMoveForTest(t, source, destination)
	ops := defaultOrganizationFileOperations()
	ops.renameNoReplace = func(string, string) error { return unsupportedRenameTestError }
	ops.isRenameUnsupported = func(err error) bool { return errors.Is(err, unsupportedRenameTestError) }
	ops.isCrossDevice = func(err error) bool { return errors.Is(err, crossDeviceTestError) }
	renameCalls, logCalls := 0, 0
	ops.rename = func(from, to string) error {
		renameCalls++
		if from == source {
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: crossDeviceTestError}
		}
		return os.Rename(from, to)
	}
	ops.logRenameFallback = func(string, string) { logCalls++ }
	if err := moveFileWithOperations(move, ops); err != nil {
		t.Fatal(err)
	}
	if renameCalls != 2 || logCalls != 2 {
		t.Fatalf("standard rename calls = %d, logs = %d; want 2 each", renameCalls, logCalls)
	}
	assertNotExists(t, source)
	assertRenameFallbackContents(t, destination, "synthetic cross-filesystem content")
}

func assertRenameFallbackContents(t *testing.T, path, want string) {
	t.Helper()
	if got, err := os.ReadFile(path); err != nil || string(got) != want {
		t.Fatalf("file %q content = %q, err = %v; want %q", path, got, err, want)
	}
}

func TestRenameFallbackLoggerWarnsOnce(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := &renameFallbackLogger{logger: zap.New(core).Sugar}
	logger.log("a.nsp", "library/a.nsp")
	logger.log("b.nsp", "library/b.nsp")
	logger.log("c.nsp", "library/c.nsp")

	if warnings := logs.FilterLevelExact(zapcore.WarnLevel).All(); len(warnings) != 1 || !strings.Contains(warnings[0].Message, "a.nsp") {
		t.Fatalf("warnings = %v, want one warning naming the first fallback", warnings)
	}
	if debug := logs.FilterLevelExact(zapcore.DebugLevel).Len(); debug != 3 {
		t.Fatalf("debug entries = %d, want one per fallback", debug)
	}
}
