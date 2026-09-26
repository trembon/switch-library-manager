package app

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) SelectFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "Select games folder"})
}

func (a *App) ConfirmOrganization() (bool, error) {
	selection, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Confirmation",
		Message:       "Are you sure you want to begin library organization?\n\nThis action will modify your local library files",
		DefaultButton: "Yes",
		CancelButton:  "No",
	})
	return selection == "Yes", err
}

func (a *App) ShowMessage(kind, title, message, detail string) error {
	dialogType := wailsruntime.InfoDialog
	switch kind {
	case "error":
		dialogType = wailsruntime.ErrorDialog
	case "warning":
		dialogType = wailsruntime.WarningDialog
	case "question":
		dialogType = wailsruntime.QuestionDialog
	}
	if detail != "" {
		message += "\n\n" + detail
	}
	_, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:    dialogType,
		Title:   title,
		Message: message,
	})
	return err
}

func (a *App) ShowInFolder(path string) error {
	if path == "" {
		return errors.New("path is empty")
	}
	var command string
	var args []string
	switch runtime.GOOS {
	case "windows":
		command = "explorer.exe"
		args = []string{"/select," + filepath.Clean(path)}
	case "darwin":
		command = "open"
		args = []string{"-R", filepath.Clean(path)}
	default:
		command = "xdg-open"
		args = []string{filepath.Dir(filepath.Clean(path))}
	}
	if err := exec.Command(command, args...).Start(); err != nil {
		return fmt.Errorf("show %q in file manager: %w", path, err)
	}
	return nil
}
