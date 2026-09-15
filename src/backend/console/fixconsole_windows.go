//go:build windows

package console

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

func FixConsoleOutput() {
	if runtime.GOOS == "windows" {
		const ATTACH_PARENT_PROCESS = ^uintptr(0)
		proc := syscall.MustLoadDLL("kernel32.dll").MustFindProc("AttachConsole")
		r0, _, err0 := proc.Call(ATTACH_PARENT_PROCESS)

		if r0 == 0 { // Allocation failed, probably process already has a console
			fmt.Printf("Could not allocate console: %s. Check build flags..", err0)
			os.Exit(1)
		}

		hout, err1 := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
		herr, err2 := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)
		if err1 != nil || err2 != nil {
			os.Exit(2)
		}

		os.Stdout = os.NewFile(uintptr(hout), "/dev/stdout")
		os.Stderr = os.NewFile(uintptr(herr), "/dev/stderr")

		fmt.Print("\r\n")
	}
}
