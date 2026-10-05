//go:build windows

package main

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

func waitForRestartParent(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("invalid parent process ID %d", pid)
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil
		}
		return fmt.Errorf("open parent process %d: %w", pid, err)
	}
	result, waitErr := windows.WaitForSingleObject(handle, uint32(timeout/time.Millisecond))
	closeErr := windows.CloseHandle(handle)
	if waitErr != nil {
		return errors.Join(fmt.Errorf("wait for parent process %d: %w", pid, waitErr), closeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close parent process handle: %w", closeErr)
	}
	switch result {
	case uint32(windows.WAIT_OBJECT_0):
		return nil
	case uint32(windows.WAIT_TIMEOUT):
		return fmt.Errorf("timed out after %s waiting for parent process %d to exit", timeout, pid)
	default:
		return fmt.Errorf("unexpected result %d while waiting for parent process %d", result, pid)
	}
}
