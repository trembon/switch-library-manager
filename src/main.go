package main

import (
	"embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/trembon/switch-library-manager/backend/app"
	"github.com/trembon/switch-library-manager/backend/console"
	"github.com/trembon/switch-library-manager/backend/consoleapp"
	"github.com/trembon/switch-library-manager/backend/settings"
	"go.uber.org/zap"
)

//go:embed all:frontend
var frontendAssets embed.FS

func main() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("failed to get executable directory, please ensure app has sufficient permissions. aborting")
		return
	}

	workingFolder := filepath.Dir(exePath)

	if runtime.GOOS == "darwin" {
		if strings.Contains(workingFolder, ".app") {
			appIndex := strings.Index(workingFolder, ".app")
			sepIndex := strings.LastIndex(workingFolder[:appIndex], string(os.PathSeparator))
			workingFolder = workingFolder[:sepIndex]
		}
	}

	console.InitializeFlags()
	consoleFlags := console.GetFlagsValues()

	preparedSettings, err := settings.PrepareSettings(workingFolder)
	if err != nil {
		fmt.Printf("failed to load settings: %v\n", err)
		return
	}
	appSettings := preparedSettings.Settings

	logger := createLogger(workingFolder, appSettings.Logging.Debug)

	defer logger.Sync() // flushes buffer, if any
	sugar := logger.Sugar()

	sugar.Info("[SLM starts]")
	sugar.Infof("[Executable: %v]", exePath)
	sugar.Infof("[Working directory: %v]", workingFolder)

	console.LogFlags(sugar)

	useGUI := resolveGUIMode(appSettings.GUI.Enabled, consoleFlags.Mode.IsSet(), consoleFlags.Mode.String())
	if shouldAbortConsoleMigration(useGUI, preparedSettings.Migration) {
		fmt.Printf("settings migration required: the older settings file was preserved as %s; update the new settings.json using docs/settings.md before using console mode\n", preparedSettings.Migration.BackupPath)
		return
	}

	if useGUI {
		if err := app.StartWithMigration(workingFolder, sugar, frontendAssets, preparedSettings.Migration); err != nil {
			sugar.Error("GUI startup failed", err)
		}
	} else {
		console.FixConsoleOutput()
		consoleapp.CreateConsole(workingFolder, sugar, consoleFlags).Start()
	}
}

func resolveGUIMode(settingsGUIEnabled, modeSet bool, mode string) bool {
	if !modeSet {
		return settingsGUIEnabled
	}
	if mode == "console" {
		return false
	}
	if mode == "gui" {
		return true
	}
	return settingsGUIEnabled
}

func shouldAbortConsoleMigration(useGUI bool, migration *settings.MigrationInfo) bool {
	return !useGUI && migration != nil
}

func createLogger(workingFolder string, debug bool) *zap.Logger {
	var config zap.Config
	if debug {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	logPath := filepath.Join(workingFolder, "slm.log")
	// delete old file
	os.Remove(logPath)

	if runtime.GOOS == "windows" {
		zap.RegisterSink("winfile", func(u *url.URL) (zap.Sink, error) {
			// Remove leading slash left by url.Parse()
			return os.OpenFile(u.Path[1:], os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		})
		logPath = "winfile:///" + logPath
	}

	config.OutputPaths = []string{logPath}
	config.ErrorOutputPaths = []string{logPath}
	logger, err := config.Build()
	if err != nil {
		fmt.Printf("failed to create logger - %v", err)
		panic(1)
	}
	zap.ReplaceGlobals(logger)
	return logger
}
