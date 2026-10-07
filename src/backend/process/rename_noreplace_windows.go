//go:build windows

package process

import "golang.org/x/sys/windows"

func renameNoReplace(from, to string) error {
	fromPath, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	toPath, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(fromPath, toPath, 0)
}
