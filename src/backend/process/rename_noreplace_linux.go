//go:build linux

package process

import (
	"errors"

	"golang.org/x/sys/unix"
)

func renameNoReplace(from, to string) error {
	return unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE)
}

// isUnsupportedExclusiveRenameError reports whether renameat2 could not honor
// RENAME_NOREPLACE. Filesystems without support for the flag, such as cifs/SMB
// mounts and some NFS and FUSE mounts, report EINVAL; kernels without
// renameat2 report ENOSYS.
func isUnsupportedExclusiveRenameError(err error) bool {
	return errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) ||
		errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP)
}
