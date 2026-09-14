package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/trembon/switch-library-manager/db"
	"github.com/trembon/switch-library-manager/process"
	"github.com/trembon/switch-library-manager/settings"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

//go:embed all:resources/app
var frontendAssets embed.FS

type Pair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LocalLibraryData struct {
	LibraryData []LibraryTemplateData `json:"library_data"`
	Issues      []Pair                `json:"issues"`
	NumFiles    int                   `json:"num_files"`
}

type SwitchTitle struct {
	Name        string `json:"name"`
	TitleId     string `json:"titleId"`
	Icon        string `json:"icon"`
	Region      string `json:"region"`
	ReleaseDate string `json:"release_date"`
}

type LibraryTemplateData struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Dlc     string `json:"dlc"`
	TitleId string `json:"titleId"`
	Path    string `json:"path"`
	Icon    string `json:"icon"`
	Update  int    `json:"update"`
	Region  string `json:"region"`
	Type    string `json:"type"`
}

type ProgressUpdate struct {
	Curr    int    `json:"curr"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

type State struct {
	sync.Mutex
	switchDB *db.SwitchTitlesDB
	localDB  *db.LocalSwitchFilesDB
}

type GUI struct {
	state          State
	baseFolder     string
	localDbManager *db.LocalSwitchDBManager
	sugarLogger    *zap.SugaredLogger
	ctx            context.Context
}

func CreateGUI(baseFolder string, sugarLogger *zap.SugaredLogger) *GUI {
	return &GUI{state: State{}, baseFolder: baseFolder, sugarLogger: sugarLogger}
}

func (g *GUI) Start() error {
	localDbManager, err := db.NewLocalSwitchDBManager(g.baseFolder)
	if err != nil {
		g.sugarLogger.Error("Failed to create local files db", err)
		return err
	}

	settings.InitSwitchKeys(g.baseFolder)
	g.localDbManager = localDbManager
	defer localDbManager.Close()

	frontend, err := fs.Sub(frontendAssets, "resources/app")
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
		Menu:             g.applicationMenu(),
		OnStartup:        g.startup,
		OnShutdown:       g.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.trembon.switch-library-manager",
		},
		Bind: []interface{}{g},
	}); err != nil {
		return fmt.Errorf("running Wails: %w", err)
	}
	return nil
}

func (g *GUI) startup(ctx context.Context) {
	g.state.Lock()
	defer g.state.Unlock()
	g.ctx = ctx
}

func (g *GUI) shutdown(context.Context) {}

func (g *GUI) applicationMenu() *menu.Menu {
	appMenu := menu.NewMenu()
	appMenu.Append(menu.EditMenu())

	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Rescan", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(g.ctx, "rescan", false)
	})
	fileMenu.AddText("Hard rescan", nil, func(_ *menu.CallbackData) {
		g.state.Lock()
		err := g.localDbManager.ClearScanData()
		g.state.Unlock()
		if err != nil {
			g.emitError(err)
			return
		}
		wailsruntime.EventsEmit(g.ctx, "rescan", true)
	})

	return appMenu
}

func (g *GUI) emitError(err error) {
	if err == nil {
		return
	}
	g.sugarLogger.Error(err)
	if g.ctx != nil {
		wailsruntime.EventsEmit(g.ctx, "error", err.Error())
	}
}

func (g *GUI) LoadSettings() *settings.AppSettings {
	g.state.Lock()
	defer g.state.Unlock()
	result := settings.ReadSettings(g.baseFolder)
	if g.ctx != nil {
		wailsruntime.WindowSetAlwaysOnTop(g.ctx, false)
	}
	return result
}

func (g *GUI) SaveSettings(value settings.AppSettings) error {
	g.state.Lock()
	defer g.state.Unlock()
	if value.ScanFolders == nil {
		value.ScanFolders = []string{}
	}
	if value.IgnoreDLCTitleIds == nil {
		value.IgnoreDLCTitleIds = []string{}
	}
	if value.IgnoreUpdateTitleIds == nil {
		value.IgnoreUpdateTitleIds = []string{}
	}
	if value.IgnoreFileTypes == nil {
		value.IgnoreFileTypes = []string{}
	}
	return settings.SaveSettingsWithError(&value, g.baseFolder)
}

func (g *GUI) IsKeysFileAvailable() bool {
	keys, _ := settings.SwitchKeys()
	return keys != nil && keys.GetKey("header_key") != ""
}

func (g *GUI) CheckUpdate() (bool, error) {
	newUpdate, err := settings.CheckForUpdates()
	if err != nil && strings.Contains(err.Error(), "dial tcp") {
		return false, nil
	}
	return newUpdate, err
}

func (g *GUI) UpdateDB() error {
	g.state.Lock()
	defer g.state.Unlock()
	if g.state.switchDB != nil {
		return nil
	}
	switchDB, err := g.buildSwitchDb()
	if err != nil {
		return err
	}
	g.state.switchDB = switchDB
	return nil
}

func (g *GUI) UpdateLocalLibrary(ignoreCache bool) (LocalLibraryData, error) {
	g.state.Lock()
	defer g.state.Unlock()
	if g.state.switchDB == nil {
		return LocalLibraryData{}, errors.New("title database is not loaded")
	}
	localDB, err := g.buildLocalDB(g.localDbManager, ignoreCache)
	if err != nil {
		return LocalLibraryData{}, err
	}

	response := LocalLibraryData{}
	for k, v := range localDB.TitlesMap {
		if v.BaseExist {
			version := ""
			name := ""
			if v.File.Metadata.Ncap != nil {
				version = v.File.Metadata.Ncap.DisplayVersion
				name = v.File.Metadata.Ncap.TitleName["AmericanEnglish"].Title
			}

			if v.Updates != nil && len(v.Updates) != 0 {
				if v.Updates[v.LatestUpdate].Metadata.Ncap != nil {
					version = v.Updates[v.LatestUpdate].Metadata.Ncap.DisplayVersion
				} else {
					version = ""
				}
			}
			if title, ok := g.state.switchDB.TitlesMap[k]; ok {
				name = getLibraryTitleName(title, name, v.File.ExtendedInfo.FileName)
				response.LibraryData = append(response.LibraryData, LibraryTemplateData{
					Icon:    title.Attributes.IconUrl,
					Name:    name,
					TitleId: title.Attributes.Id,
					Update:  v.LatestUpdate,
					Version: version,
					Region:  title.Attributes.Region,
					Type:    getType(v),
					Path:    filepath.Join(v.File.ExtendedInfo.BaseFolder, v.File.ExtendedInfo.FileName),
				})
			} else {
				if name == "" {
					name = db.ParseTitleNameFromFileName(v.File.ExtendedInfo.FileName)
				}
				response.LibraryData = append(response.LibraryData, LibraryTemplateData{
					Name:    name,
					Update:  v.LatestUpdate,
					Version: version,
					Type:    getType(v),
					TitleId: v.File.Metadata.TitleId,
					Path:    filepath.Join(v.File.ExtendedInfo.BaseFolder, v.File.ExtendedInfo.FileName),
				})
			}
		} else {
			for _, update := range v.Updates {
				response.Issues = append(response.Issues, Pair{Key: filepath.Join(update.ExtendedInfo.BaseFolder, update.ExtendedInfo.FileName), Value: "base file is missing"})
			}
			for _, dlc := range v.Dlc {
				response.Issues = append(response.Issues, Pair{Key: filepath.Join(dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName), Value: "base file is missing"})
			}
		}
	}
	for k, v := range localDB.Skipped {
		response.Issues = append(response.Issues, Pair{Key: filepath.Join(k.BaseFolder, k.FileName), Value: v.ReasonText})
	}
	response.NumFiles = localDB.NumFiles
	return response, nil
}

func (g *GUI) GetMissingGames() ([]SwitchTitle, error) {
	g.state.Lock()
	defer g.state.Unlock()
	if g.state.switchDB == nil || g.state.localDB == nil {
		return nil, errors.New("local and title databases must be loaded")
	}
	return g.getMissingGames(), nil
}

func (g *GUI) GetMissingDLC() ([]process.IncompleteTitle, error) {
	g.state.Lock()
	defer g.state.Unlock()
	if g.state.switchDB == nil || g.state.localDB == nil {
		return nil, errors.New("local and title databases must be loaded")
	}
	settingsObj := settings.ReadSettings(g.baseFolder)
	ignoreIDs := make(map[string]struct{}, len(settingsObj.IgnoreDLCTitleIds))
	for _, id := range settingsObj.IgnoreDLCTitleIds {
		ignoreIDs[strings.ToLower(id)] = struct{}{}
	}
	missing := process.ScanForMissingDLC(g.state.localDB.TitlesMap, g.state.switchDB.TitlesMap, ignoreIDs)
	values := make([]process.IncompleteTitle, 0, len(missing))
	for _, value := range missing {
		values = append(values, value)
	}
	return values, nil
}

func (g *GUI) GetMissingUpdates() ([]process.IncompleteTitle, error) {
	g.state.Lock()
	defer g.state.Unlock()
	if g.state.switchDB == nil || g.state.localDB == nil {
		return nil, errors.New("local and title databases must be loaded")
	}
	settingsObj := settings.ReadSettings(g.baseFolder)
	ignoreIDs := make(map[string]struct{}, len(settingsObj.IgnoreUpdateTitleIds))
	for _, id := range settingsObj.IgnoreUpdateTitleIds {
		ignoreIDs[strings.ToLower(id)] = struct{}{}
	}
	missing := process.ScanForMissingUpdates(g.state.localDB.TitlesMap, g.state.switchDB.TitlesMap, ignoreIDs, settingsObj.IgnoreDLCUpdates)
	values := make([]process.IncompleteTitle, 0, len(missing))
	for _, value := range missing {
		values = append(values, value)
	}
	return values, nil
}

func (g *GUI) OrganizeLibrary() error {
	g.state.Lock()
	defer g.state.Unlock()
	if g.state.switchDB == nil || g.state.localDB == nil {
		return errors.New("local and title databases must be loaded")
	}
	folderToScan := settings.ReadSettings(g.baseFolder).Folder
	organizeOptions := settings.ReadSettings(g.baseFolder).OrganizeOptions
	if !process.IsOptionsValid(organizeOptions) {
		return errors.New("the organize options in settings.json are not valid, please check that the template contains file/folder name")
	}
	process.OrganizeByFolders(folderToScan, g.state.localDB, g.state.switchDB, g)
	if settings.ReadSettings(g.baseFolder).OrganizeOptions.DeleteOldUpdateFiles {
		process.DeleteOldUpdates(g.baseFolder, g.state.localDB, g)
	}
	return nil
}

func (g *GUI) SelectFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(g.ctx, wailsruntime.OpenDialogOptions{Title: "Select games folder"})
}

func (g *GUI) ConfirmOrganization() (bool, error) {
	selection, err := wailsruntime.MessageDialog(g.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Confirmation",
		Message:       "Are you sure you want to begin library organization?\n\nThis action will modify your local library files",
		DefaultButton: "Yes",
		CancelButton:  "No",
	})
	return selection == "Yes", err
}

func (g *GUI) ShowMessage(kind, title, message, detail string) error {
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
	_, err := wailsruntime.MessageDialog(g.ctx, wailsruntime.MessageDialogOptions{
		Type:    dialogType,
		Title:   title,
		Message: message,
	})
	return err
}

func (g *GUI) ShowInFolder(path string) error {
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

func getType(gameFile *db.SwitchGameFiles) string {
	if gameFile.IsSplit {
		return "split"
	}
	if gameFile.MultiContent {
		return "multi-content"
	}
	ext := filepath.Ext(gameFile.File.ExtendedInfo.FileName)
	if len(ext) > 1 {
		return ext[1:]
	}
	return ""
}

func getLibraryTitleName(title *db.SwitchTitle, name, fileName string) string {
	if title != nil && title.Attributes.Name != "" {
		return title.Attributes.Name
	}
	if name != "" {
		return name
	}
	return db.ParseTitleNameFromFileName(fileName)
}

func (g *GUI) buildSwitchDb() (*db.SwitchTitlesDB, error) {
	settingsObj := settings.ReadSettings(g.baseFolder)
	g.UpdateProgress(1, 4, "Downloading titles.json")
	filename := filepath.Join(g.baseFolder, settings.TITLE_JSON_FILENAME)
	titleFile, titlesEtag, err := db.LoadAndUpdateFile(settingsObj.TitlesJsonUrl, filename, settingsObj.TitlesEtag)
	if err != nil {
		return nil, errors.New("failed to download switch titles [reason:" + err.Error() + "]")
	}
	settingsObj.TitlesEtag = titlesEtag

	g.UpdateProgress(2, 4, "Downloading versions.json")
	filename = filepath.Join(g.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsEtag, err := db.LoadAndUpdateFile(settingsObj.VersionsJsonUrl, filename, settingsObj.VersionsEtag)
	if err != nil {
		return nil, errors.New("failed to download switch updates [reason:" + err.Error() + "]")
	}
	settingsObj.VersionsEtag = versionsEtag
	settings.SaveSettings(settingsObj, g.baseFolder)

	g.UpdateProgress(3, 4, "Processing switch titles and updates ...")
	switchTitleDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)
	g.UpdateProgress(4, 4, "Finishing up...")
	return switchTitleDB, err
}

func (g *GUI) buildLocalDB(localDbManager *db.LocalSwitchDBManager, ignoreCache bool) (*db.LocalSwitchFilesDB, error) {
	settingsObj := settings.ReadSettings(g.baseFolder)
	scanFolders := append([]string{}, settingsObj.ScanFolders...)
	scanFolders = append(scanFolders, settingsObj.Folder)
	localDB, err := localDbManager.CreateLocalSwitchFilesDB(scanFolders, g, settingsObj.ScanRecursively, ignoreCache)
	g.state.localDB = localDB
	return localDB, err
}

func (g *GUI) UpdateProgress(curr int, total int, message string) {
	progressMessage := ProgressUpdate{Curr: curr, Total: total, Message: message}
	g.sugarLogger.Debugf("%v (%v/%v)", message, curr, total)
	if g.ctx != nil {
		wailsruntime.EventsEmit(g.ctx, "updateProgress", progressMessage)
	}
}

func (g *GUI) getMissingGames() []SwitchTitle {
	var result []SwitchTitle
	for k, v := range g.state.switchDB.TitlesMap {
		if _, ok := g.state.localDB.TitlesMap[k]; ok {
			continue
		}
		if v.Attributes.Name == "" || v.Attributes.Id == "" {
			continue
		}
		options := settings.ReadSettings(g.baseFolder)
		if options.HideDemoGames && v.Attributes.IsDemo {
			continue
		}
		result = append(result, SwitchTitle{
			TitleId:     v.Attributes.Id,
			Name:        v.Attributes.Name,
			Icon:        v.Attributes.BannerUrl,
			Region:      v.Attributes.Region,
			ReleaseDate: v.Attributes.ParsedReleaseDate,
		})
	}
	return result
}
