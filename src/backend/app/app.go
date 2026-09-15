package app

import (
	"context"
	"fmt"
	"io/fs"
	"sync"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"go.uber.org/zap"
)

type State struct {
	mu sync.Mutex

	switchDB *db.SwitchTitlesDB
	localDB  *db.LocalSwitchFilesDB
}

type App struct {
	state          State
	baseFolder     string
	localDbManager *db.LocalSwitchDBManager
	sugarLogger    *zap.SugaredLogger
	ctx            context.Context
}

// Start prepares the application dependencies and starts the Wails event loop.
func Start(baseFolder string, sugarLogger *zap.SugaredLogger, assets fs.FS) error {
	localDbManager, err := db.NewLocalSwitchDBManager(baseFolder)
	if err != nil {
		sugarLogger.Error("failed to create local files db", err)
		return err
	}
	defer localDbManager.Close()

	settings.InitSwitchKeys(baseFolder)
	application := &App{
		baseFolder:     baseFolder,
		localDbManager: localDbManager,
		sugarLogger:    sugarLogger,
	}
	return application.run(assets)
}

func (a *App) run(assets fs.FS) error {
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		return fmt.Errorf("prepare frontend assets: %w", err)
	}

	if err := wails.Run(&options.App{
		Title:            "Switch Library Manager (" + settings.SLM_VERSION + ")",
		Width:            1200,
		Height:           600,
		AlwaysOnTop:      true,
		BackgroundColour: options.NewRGB(51, 51, 51),
		AssetServer:      &assetserver.Options{Assets: frontend},
		Menu:             a.applicationMenu(),
		OnStartup:        a.startup,
		OnShutdown:       a.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.trembon.switch-library-manager",
		},
		Bind: []interface{}{a},
	}); err != nil {
		return fmt.Errorf("running Wails: %w", err)
	}
	return nil
}
