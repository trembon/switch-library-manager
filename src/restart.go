package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const restartParentArgument = "--slm-restart-parent="

const restartParentTimeout = 30 * time.Second

func stripRestartParentArgument(args []string) ([]string, int, error) {
	remaining := make([]string, 0, len(args))
	parentPID := 0
	for _, arg := range args {
		if !strings.HasPrefix(arg, restartParentArgument) {
			remaining = append(remaining, arg)
			continue
		}
		if parentPID != 0 {
			return nil, 0, fmt.Errorf("restart parent argument was provided more than once")
		}
		value := strings.TrimPrefix(arg, restartParentArgument)
		pid, err := strconv.Atoi(value)
		if err != nil || pid <= 0 {
			return nil, 0, fmt.Errorf("invalid restart parent process ID %q", value)
		}
		parentPID = pid
	}
	return remaining, parentPID, nil
}

func replacementCommand(executable string, parentPID int) (*exec.Cmd, error) {
	if executable == "" {
		return nil, errors.New("executable path is empty")
	}
	if parentPID <= 0 {
		return nil, fmt.Errorf("invalid parent process ID %d", parentPID)
	}
	return exec.Command(executable, restartParentArgument+strconv.Itoa(parentPID), "-m", "gui"), nil
}

func launchReplacementApplication(executable string, parentPID int) error {
	cmd, err := replacementCommand(executable, parentPID)
	if err != nil {
		return err
	}
	return startReplacementProcess(cmd, cmd.Start, func(process *os.Process) error {
		return process.Release()
	})
}

func startReplacementProcess(cmd *exec.Cmd, start func() error, release func(*os.Process) error) error {
	if err := start(); err != nil {
		return fmt.Errorf("start replacement process: %w", err)
	}
	if cmd.Process == nil {
		return errors.New("replacement process started without a process handle")
	}
	if err := release(cmd.Process); err != nil {
		return fmt.Errorf("release replacement process handle: %w", err)
	}
	return nil
}
