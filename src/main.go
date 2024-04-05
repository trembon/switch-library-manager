package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/trembon/switch-library-manager/settings"
	"go.uber.org/zap"
)

func main() {
	fixWindowsConsole()

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

	appSettings := settings.ReadSettings(workingFolder)

	logger := createLogger(workingFolder, appSettings.Debug)

	defer logger.Sync() // flushes buffer, if any
	sugar := logger.Sugar()

	sugar.Info("[SLM starts]")
	sugar.Infof("[Executable: %v]", exePath)
	sugar.Infof("[Working directory: %v]", workingFolder)

	files, err := AssetDir(workingFolder)
	if files == nil && err == nil {
		appSettings.GUI = false
	}

	if appSettings.GUI {
		CreateGUI(workingFolder, sugar).Start()
	} else {
		CreateConsole(workingFolder, sugar).Start()
	}

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

func fixWindowsConsole() {
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
