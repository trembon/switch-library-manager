//go:build darwin

package process

import (
	"errors"

	"golang.org/x/sys/unix"
)

func renameNoReplace(from, to string) error {
	return unix.RenamexNp(from, to, unix.RENAME_EXCL)
}

// isUnsupportedExclusiveRenameError reports whether renamex_np could not honor
// RENAME_EXCL. SMB shares report ENOTSUP; some NFS and FUSE-backed filesystems
// report EINVAL for the unsupported flag.
func isUnsupportedExclusiveRenameError(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EINVAL)
}
