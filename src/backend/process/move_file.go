package process

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type organizationSourceFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}

type organizationTemporaryFile interface {
	io.Writer
	Name() string
	Chmod(os.FileMode) error
	Sync() error
	Close() error
}

type organizationFileOperations struct {
	renameNoReplace func(string, string) error
	isCrossDevice   func(error) bool
	openSource      func(string) (organizationSourceFile, error)
	createTemp      func(string, string) (organizationTemporaryFile, error)
	lstat           func(string) (os.FileInfo, error)
	remove          func(string) error
	chtimes         func(string, time.Time, time.Time) error
}

func defaultOrganizationFileOperations() organizationFileOperations {
	return organizationFileOperations{
		renameNoReplace: renameNoReplace,
		isCrossDevice:   isCrossDeviceMoveError,
		openSource: func(path string) (organizationSourceFile, error) {
			return os.Open(path)
		},
		createTemp: func(directory, pattern string) (organizationTemporaryFile, error) {
			return os.CreateTemp(directory, pattern)
		},
		lstat:   os.Lstat,
		remove:  os.Remove,
		chtimes: os.Chtimes,
	}
}

func moveFile(from, to string) error {
	if sameOrganizationPath(from, to) {
		return nil
	}
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	return moveOrganizationFile(OrganizationMove{Source: from, Destination: to, SourceSize: info.Size(), SourceStamp: info})
}

func moveOrganizationFile(move OrganizationMove) error {
	if sameOrganizationPath(move.Source, move.Destination) {
		return nil
	}
	return moveFileWithOperations(move, defaultOrganizationFileOperations())
}

func moveFileWithOperations(move OrganizationMove, operations organizationFileOperations) (retErr error) {
	if err := operations.renameNoReplace(move.Source, move.Destination); err == nil {
		return nil
	} else if !operations.isCrossDevice(err) {
		return err
	}

	source, err := operations.openSource(move.Source)
	if err != nil {
		return fmt.Errorf("open source for cross-filesystem copy: %w", err)
	}
	sourceOpen := true
	defer func() {
		if sourceOpen {
			if err := source.Close(); err != nil {
				retErr = errors.Join(retErr, fmt.Errorf("close source after cross-filesystem copy: %w", err))
			}
		}
	}()

	initial, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect source before cross-filesystem copy: %w", err)
	}
	if err := validateMoveSource(move, initial); err != nil {
		return err
	}
	pathInfo, err := operations.lstat(move.Source)
	if err != nil {
		return fmt.Errorf("inspect source path before cross-filesystem copy: %w", err)
	}
	if err := validateMoveSource(move, pathInfo); err != nil || !os.SameFile(initial, pathInfo) {
		if err != nil {
			return err
		}
		return fmt.Errorf("source %q changed before cross-filesystem copy", move.Source)
	}

	temporary, err := operations.createTemp(filepath.Dir(move.Destination), ".slm-move-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary destination for cross-filesystem copy: %w", err)
	}
	temporaryOpen := true
	published := false
	defer func() {
		if temporaryOpen {
			if err := temporary.Close(); err != nil {
				retErr = errors.Join(retErr, fmt.Errorf("close temporary copy: %w", err))
			}
		}
		if !published {
			if err := operations.remove(temporary.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
				retErr = errors.Join(retErr, fmt.Errorf("remove temporary copy %q: %w", temporary.Name(), err))
			}
		}
	}()

	copied, err := io.Copy(temporary, source)
	if err != nil {
		return fmt.Errorf("copy source to temporary destination: %w", err)
	}
	if copied != initial.Size() {
		return fmt.Errorf("cross-filesystem copy of %q wrote %d bytes, expected %d", move.Source, copied, initial.Size())
	}
	if err := temporary.Chmod(initial.Mode().Perm()); err != nil {
		return fmt.Errorf("preserve source permissions on temporary copy: %w", err)
	}
	if err := operations.chtimes(temporary.Name(), initial.ModTime(), initial.ModTime()); err != nil {
		return fmt.Errorf("preserve source modification time on temporary copy: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("flush temporary copy: %w", err)
	}
	if err := temporary.Close(); err != nil {
		temporaryOpen = false
		return fmt.Errorf("close temporary copy: %w", err)
	}
	temporaryOpen = false

	finalInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("verify source after cross-filesystem copy: %w", err)
	}
	if err := validateMoveSource(move, finalInfo); err != nil || !os.SameFile(initial, finalInfo) {
		if err != nil {
			return err
		}
		return fmt.Errorf("source %q changed during cross-filesystem copy", move.Source)
	}
	closeErr := source.Close()
	sourceOpen = false
	if closeErr != nil {
		return fmt.Errorf("close source before publishing cross-filesystem copy: %w", closeErr)
	}
	if err := verifyMoveSourcePath(move, operations.lstat); err != nil {
		return err
	}
	if err := operations.renameNoReplace(temporary.Name(), move.Destination); err != nil {
		return fmt.Errorf("publish cross-filesystem copy at %q: %w", move.Destination, err)
	}
	published = true
	if err := verifyMoveSourcePath(move, operations.lstat); err != nil {
		return fmt.Errorf("copy was published at %q but source was retained: %w", move.Destination, err)
	}
	if err := operations.remove(move.Source); err != nil {
		return fmt.Errorf("copy was published at %q but source %q could not be removed: %w", move.Destination, move.Source, err)
	}
	return nil
}

func validateMoveSource(move OrganizationMove, current os.FileInfo) error {
	if current == nil || !current.Mode().IsRegular() {
		return fmt.Errorf("source %q is no longer a regular file", move.Source)
	}
	if move.SourceStamp != nil &&
		(!os.SameFile(move.SourceStamp, current) || current.Size() != move.SourceSize || !current.ModTime().Equal(move.SourceStamp.ModTime())) {
		return fmt.Errorf("source %q changed after organization preflight", move.Source)
	}
	return nil
}

func verifyMoveSourcePath(move OrganizationMove, lstat func(string) (os.FileInfo, error)) error {
	current, err := lstat(move.Source)
	if err != nil {
		return fmt.Errorf("inspect source %q before removal: %w", move.Source, err)
	}
	if err := validateMoveSource(move, current); err != nil {
		return err
	}
	return nil
}
