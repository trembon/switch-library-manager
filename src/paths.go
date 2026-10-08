package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const macOSDataDirectoryName = "Switch Library Manager"

// resolveRuntimeDataFolder selects the persistent data directory. userConfigDir
// is injected to keep startup path selection testable without depending on the
// host operating system.
func resolveRuntimeDataFolder(goos, executablePath string, userConfigDir func() (string, error)) (string, error) {
	executableFolder := filepath.Dir(executablePath)
	if goos != "darwin" {
		return executableFolder, nil
	}
	if userConfigDir == nil {
		return "", fmt.Errorf("resolve user config directory: lookup function is nil")
	}

	configFolder, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	if configFolder == "" {
		return "", fmt.Errorf("resolve user config directory: returned an empty path")
	}

	dataFolder := filepath.Join(configFolder, macOSDataDirectoryName)
	if err := os.MkdirAll(dataFolder, 0755); err != nil {
		return "", fmt.Errorf("create application data directory %q: %w", dataFolder, err)
	}

	return dataFolder, nil
}
