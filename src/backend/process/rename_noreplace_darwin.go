//go:build darwin

package process

import "golang.org/x/sys/unix"

func renameNoReplace(from, to string) error {
	return unix.RenamexNp(from, to, unix.RENAME_EXCL)
}
