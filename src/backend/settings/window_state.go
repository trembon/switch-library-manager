package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	WINDOW_STATE_FILENAME = "window-state.json"
	maxWindowDimension    = 16384
)

// WindowState contains the last normal window bounds saved by the GUI.
type WindowState struct {
	Width        int `json:"width"`
	Height       int `json:"height"`
	X            int `json:"x"`
	Y            int `json:"y"`
	ScreenWidth  int `json:"screen_width"`
	ScreenHeight int `json:"screen_height"`
}

// ReadWindowState returns nil when no saved state exists.
func ReadWindowState(baseFolder string) (*WindowState, error) {
	filename := filepath.Join(baseFolder, WINDOW_STATE_FILENAME)
	data, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read window state: %w", err)
	}

	var state WindowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode window state: %w", err)
	}
	if err := validateWindowState(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

// SaveWindowState validates and atomically replaces the saved window bounds.
func SaveWindowState(state *WindowState, baseFolder string) error {
	if state == nil {
		return errors.New("window state is nil")
	}
	if err := validateWindowState(state); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", " ")
	if err != nil {
		return fmt.Errorf("marshal window state: %w", err)
	}
	if err := writeSettingsFile(filepath.Join(baseFolder, WINDOW_STATE_FILENAME), data); err != nil {
		return fmt.Errorf("write window state: %w", err)
	}
	return nil
}

func validateWindowState(state *WindowState) error {
	if state == nil {
		return errors.New("window state is nil")
	}
	if state.Width <= 0 || state.Width > maxWindowDimension {
		return fmt.Errorf("window state width must be between 1 and %d", maxWindowDimension)
	}
	if state.Height <= 0 || state.Height > maxWindowDimension {
		return fmt.Errorf("window state height must be between 1 and %d", maxWindowDimension)
	}
	if (state.ScreenWidth == 0) != (state.ScreenHeight == 0) {
		return errors.New("window state screen width and height must both be set or both be zero")
	}
	if state.ScreenWidth < 0 || state.ScreenWidth > maxWindowDimension {
		return fmt.Errorf("window state screen width must be between 0 and %d", maxWindowDimension)
	}
	if state.ScreenHeight < 0 || state.ScreenHeight > maxWindowDimension {
		return fmt.Errorf("window state screen height must be between 0 and %d", maxWindowDimension)
	}
	return nil
}
