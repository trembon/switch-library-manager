package main

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStripRestartParentArgument(t *testing.T) {
	got, pid, err := stripRestartParentArgument([]string{"-m", "gui", restartParentArgument + "123", "-f", "games"})
	if err != nil {
		t.Fatal(err)
	}
	if pid != 123 || !reflect.DeepEqual(got, []string{"-m", "gui", "-f", "games"}) {
		t.Fatalf("stripped args = %#v, pid = %d; want original flags and pid 123", got, pid)
	}
	if _, _, err := stripRestartParentArgument([]string{restartParentArgument + "bad"}); err == nil {
		t.Fatal("invalid parent pid was accepted")
	}
	if _, _, err := stripRestartParentArgument([]string{restartParentArgument + "1", restartParentArgument + "2"}); err == nil {
		t.Fatal("duplicate parent arguments were accepted")
	}
}

func TestReplacementCommandTargetsGUIExecutable(t *testing.T) {
	const executable = `C:\Program Files\Switch Library Manager\slm.exe`
	cmd, err := replacementCommand(executable, 456)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{executable, restartParentArgument + "456", "-m", "gui"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("replacement command args = %#v, want %#v", cmd.Args, want)
	}
	if cmd.Dir != "" {
		t.Fatalf("replacement working directory = %q, want inherited working directory", cmd.Dir)
	}
	if cmd.SysProcAttr != nil {
		t.Fatalf("replacement process has startup attributes %#v; want normal GUI window startup", cmd.SysProcAttr)
	}
	if _, err := replacementCommand("", 456); err == nil {
		t.Fatal("empty executable path was accepted")
	}
}

func TestStartReplacementProcessReportsLaunchFailure(t *testing.T) {
	wantErr := errors.New("start failed")
	cmd := exec.Command("replacement")
	released := false
	err := startReplacementProcess(cmd, func() error { return wantErr }, func(*os.Process) error {
		released = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("startReplacementProcess() error = %v, want wrapped start failure", err)
	}
	if released {
		t.Fatal("process handle was released after start failed")
	}
}

func TestStartReplacementProcessReleasesStartedProcess(t *testing.T) {
	cmd := exec.Command("replacement")
	process := &os.Process{Pid: os.Getpid()}
	cmd.Process = process
	var released *os.Process
	err := startReplacementProcess(cmd, func() error { return nil }, func(got *os.Process) error {
		released = got
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if released != process {
		t.Fatal("replacement process handle was not released")
	}
}

func TestWaitForRestartParentRejectsInvalidPIDAndTimesOut(t *testing.T) {
	if err := waitForRestartParent(0, time.Second); err == nil {
		t.Fatal("zero parent pid was accepted")
	}
	if err := waitForRestartParent(os.Getpid(), 20*time.Millisecond); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("waiting for current process error = %v, want timeout", err)
	}
}

func TestWaitForRestartParentObservesExitedProcess(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestRestartWaitHelperProcess$")
	cmd.Env = append(os.Environ(), "SLM_RESTART_HELPER_MILLISECONDS=120")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := waitForRestartParent(cmd.Process.Pid, 2*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("waitForRestartParent() error = %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait helper process: %v", err)
	}
}

func TestRestartWaitHelperProcess(t *testing.T) {
	if milliseconds := os.Getenv("SLM_RESTART_HELPER_MILLISECONDS"); milliseconds != "" {
		delay, err := time.ParseDuration(milliseconds + "ms")
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(delay)
	}
}
