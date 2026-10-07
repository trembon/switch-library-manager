//go:build windows

package process

import (
	"errors"

	"golang.org/x/sys/windows"
)

func isCrossDeviceMoveError(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}
