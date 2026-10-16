//go:build !windows

package main

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

func waitForRestartParent(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("invalid parent process ID %d", pid)
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("check parent process %d: %w", pid, err)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out after %s waiting for parent process %d to exit", timeout, pid)
		}
		if remaining < 50*time.Millisecond {
			select {
			case <-ticker.C:
			case <-time.After(remaining):
			}
		} else {
			<-ticker.C
		}
	}
}
