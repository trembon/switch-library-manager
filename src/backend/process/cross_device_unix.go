//go:build !windows

package process

import (
	"errors"
	"syscall"
)

func isCrossDeviceMoveError(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
