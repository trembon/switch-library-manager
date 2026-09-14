package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/asticode/go-astilectron"
)

func TestRunBootstrapWithRecovery(t *testing.T) {
	initialErr := errors.New("starting astilectron failed: content in archive does not match specified internal path astilectron-0.57.0")
	retryErr := errors.New("retry failed")
	clearErr := errors.New("clear failed")

	tests := []struct {
		name             string
		runErrors        []error
		clearError       error
		wantCalls        int
		wantClearCalls   int
		wantError        bool
		wantInitialError bool
		wantRetryError   bool
	}{
		{
			name:           "successful retry",
			runErrors:      []error{initialErr, nil},
			wantCalls:      2,
			wantClearCalls: 1,
		},
		{
			name:      "unrelated error is returned without retry",
			runErrors: []error{errors.New("network unavailable")},
			wantCalls: 1,
			wantError: true,
		},
		{
			name:             "cleanup failure preserves initial error",
			runErrors:        []error{initialErr},
			clearError:       clearErr,
			wantCalls:        1,
			wantClearCalls:   1,
			wantError:        true,
			wantInitialError: true,
		},
		{
			name:             "retry failure preserves both errors",
			runErrors:        []error{initialErr, retryErr},
			wantCalls:        2,
			wantClearCalls:   1,
			wantError:        true,
			wantInitialError: true,
			wantRetryError:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runCalls := 0
			clearCalls := 0
			err := runBootstrapWithRecovery(func() error {
				currentCall := runCalls
				runCalls++
				if currentCall < len(test.runErrors) {
					return test.runErrors[currentCall]
				}
				return nil
			}, func() error {
				clearCalls++
				return test.clearError
			})

			if runCalls != test.wantCalls {
				t.Fatalf("expected %d bootstrap calls, got %d", test.wantCalls, runCalls)
			}
			if clearCalls != test.wantClearCalls {
				t.Fatalf("expected %d cache cleanup calls, got %d", test.wantClearCalls, clearCalls)
			}
			if (err != nil) != test.wantError {
				t.Fatalf("expected error: %t, got %v", test.wantError, err)
			}
			if test.wantInitialError && !errors.Is(err, initialErr) {
				t.Fatalf("expected initial error to be preserved, got %v", err)
			}
			if test.wantRetryError && !errors.Is(err, retryErr) {
				t.Fatalf("expected retry error to be preserved, got %v", err)
			}
		})
	}
}

func TestClearCachedAstilectronArchive(t *testing.T) {
	appData := t.TempDir()
	baseFolder := t.TempDir()
	t.Setenv("APPDATA", appData)

	vendorDir := filepath.Join(astilectronDataDirectory(baseFolder), "vendor")
	archivePath := filepath.Join(vendorDir, "astilectron-v"+astilectron.DefaultVersionAstilectron+".zip")
	sentinelPath := filepath.Join(vendorDir, "electron.zip")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{archivePath, sentinelPath} {
		if err := os.WriteFile(path, []byte("runtime cache"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if err := clearCachedAstilectronArchive(baseFolder); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("expected Astilectron archive to be removed, stat error: %v", err)
	}
	if _, err := os.Stat(sentinelPath); err != nil {
		t.Fatalf("expected unrelated runtime cache to remain: %v", err)
	}
}

func TestClearCachedAstilectronArchiveIgnoresMissingArchive(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if err := clearCachedAstilectronArchive(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
