package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/asticode/go-astilectron"
	"github.com/trembon/switch-library-manager/settings"
)

const staleAstilectronArchiveError = "content in archive does not match specified internal path"

func runBootstrapWithRecovery(run func() error, clearCache func() error) error {
	initialErr := run()
	if initialErr == nil || !strings.Contains(initialErr.Error(), staleAstilectronArchiveError) {
		return initialErr
	}

	if err := clearCache(); err != nil {
		return fmt.Errorf("clearing cached Astilectron archive after bootstrap failure: %w", errors.Join(initialErr, err))
	}

	if err := run(); err != nil {
		return fmt.Errorf("bootstrap retry after clearing cached Astilectron archive failed: %w", errors.Join(initialErr, err))
	}
	return nil
}

func clearCachedAstilectronArchive(baseFolder string) error {
	archivePath := filepath.Join(astilectronDataDirectory(baseFolder), "vendor", "astilectron-v"+astilectron.DefaultVersionAstilectron+".zip")
	if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", archivePath, err)
	}
	return nil
}

func astilectronDataDirectory(baseFolder string) string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "Switch Library Manager ("+settings.SLM_VERSION+")")
	}
	return baseFolder
}
