package consoleapp

import (
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/schollz/progressbar/v3"
	"github.com/trembon/switch-library-manager/backend/console"
	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/process"
	"github.com/trembon/switch-library-manager/backend/settings"
	"go.uber.org/zap"
)

var (
	progressBar *progressbar.ProgressBar
)

const (
	csvGamesKind          = "games"
	csvMissingGamesKind   = "missing_games"
	csvMissingUpdatesKind = "missing_updates"
	csvMissingDLCKind     = "missing_dlc"
	csvIssuesKind         = "issues"
)

func csvExportFilename(kind string, date time.Time) string {
	return fmt.Sprintf("slm_%s.%s.csv", kind, date.Format("2006-01-02"))
}

type Console struct {
	baseFolder   string
	sugarLogger  *zap.SugaredLogger
	consoleFlags *console.ConsoleFlags
}

func CreateConsole(baseFolder string, sugarLogger *zap.SugaredLogger, consoleFlags *console.ConsoleFlags) *Console {
	return &Console{baseFolder: baseFolder, sugarLogger: sugarLogger, consoleFlags: consoleFlags}
}

func (c *Console) Start() {
	settingsObj, err := settings.ReadSettings(c.baseFolder)
	if err != nil {
		fmt.Printf("Failed to load settings: %v\n", err)
		return
	}
	cache, err := settings.ReadCache(c.baseFolder)
	if err != nil {
		fmt.Printf("Failed to load cache: %v\n", err)
		return
	}

	// 0. prepare csv export folder
	csvOutput := ""
	if c.consoleFlags.ExportCsv.IsSet() {
		csvOutput = c.consoleFlags.ExportCsv.String()

		if _, err := os.Stat(csvOutput); os.IsNotExist(err) {
			err = os.Mkdir(csvOutput, os.ModePerm)
			if err != nil {
				fmt.Printf("Failed to create folder for csv export %v - %v\n", csvOutput, err)
				zap.S().Errorf("Failed to create folder for csv export %v - %v\n", csvOutput, err)
			}
		}
	}
	exportDate := time.Now()

	//1. load the titles JSON object
	fmt.Println("Downloading latest switch titles json file")
	progressBar = progressbar.New(2)

	filename := filepath.Join(c.baseFolder, settings.TITLE_JSON_FILENAME)
	titleFile, titleCache, err := db.LoadAndUpdateFile(settingsObj.DataSources.TitlesURL, filename, db.RemoteFileCache{
		URL: cache.TitlesURL, ETag: cache.TitlesETag, SHA256: cache.TitlesSHA256,
	}, db.ValidateTitlesJSON)
	if err != nil {
		fmt.Printf("title json file doesn't exist\n")
		return
	}
	defer func() {
		if closeErr := titleFile.Close(); closeErr != nil {
			fmt.Printf("failed to close title json file: %v\n", closeErr)
			if c.sugarLogger != nil {
				c.sugarLogger.Errorf("failed to close title json file: %v", closeErr)
			}
		}
	}()
	cache.TitlesETag, cache.TitlesURL, cache.TitlesSHA256 = titleCache.ETag, titleCache.URL, titleCache.SHA256
	progressBar.Add(1)
	//2. load the versions JSON object
	filename = filepath.Join(c.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsCache, err := db.LoadAndUpdateFile(settingsObj.DataSources.VersionsURL, filename, db.RemoteFileCache{
		URL: cache.VersionsURL, ETag: cache.VersionsETag, SHA256: cache.VersionsSHA256,
	}, db.ValidateVersionsJSON)
	if err != nil {
		fmt.Printf("version json file doesn't exist\n")
		return
	}
	defer func() {
		if closeErr := versionsFile.Close(); closeErr != nil {
			fmt.Printf("failed to close version json file: %v\n", closeErr)
			if c.sugarLogger != nil {
				c.sugarLogger.Errorf("failed to close version json file: %v", closeErr)
			}
		}
	}()
	cache.VersionsETag, cache.VersionsURL, cache.VersionsSHA256 = versionsCache.ETag, versionsCache.URL, versionsCache.SHA256
	progressBar.Add(1)
	progressBar.Finish()
	newUpdate, err := settings.CheckForUpdates()

	if newUpdate {
		fmt.Printf("\n=== New version available, download from Github ===\n")
	}

	//3. update the internal cache with new etags
	if err := settings.SaveCacheWithError(cache, c.baseFolder); err != nil {
		fmt.Printf("Failed to save cache: %v\n", err)
		return
	}

	//4. create switch title db
	titlesDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)

	//5. read local files
	folderToScan := settingsObj.Paths.LibraryFolder
	if c.consoleFlags.NspFolder.IsSet() && c.consoleFlags.NspFolder.String() != "" {
		folderToScan = c.consoleFlags.NspFolder.String()
	}

	if folderToScan == "" {
		fmt.Printf("\n\nNo folder to scan was defined, please edit settings.json with the folder path\n")
		return
	}
	fmt.Printf("\n\nScanning folder [%v]", folderToScan)
	progressBar = progressbar.New(2000)
	keys, _ := settings.InitSwitchKeys(c.baseFolder)
	if keys == nil || keys.GetKey("header_key") == "" {
		fmt.Printf("\n!!NOTE!!: keys file was not found, deep scan is disabled, library will be based on file tags.\n %v", err)
	}

	recursiveMode := settingsObj.Scan.Recursive
	if c.consoleFlags.Recursive.IsSet() {
		recursiveMode = c.consoleFlags.Recursive.Bool()
	}

	localDbManager, err := db.NewLocalSwitchDBManager(c.baseFolder)
	if err != nil {
		fmt.Printf("failed to create local files db :%v\n", err)
		return
	}
	defer localDbManager.Close()

	scanFolders := settingsObj.Paths.ScanFolders
	scanFolders = append(scanFolders, folderToScan)

	localDB, err := localDbManager.CreateLocalSwitchFilesDB(scanFolders, c, recursiveMode, true)
	if err != nil {
		fmt.Printf("\nfailed to process local folder\n %v", err)
		return
	}
	progressBar.Finish()

	p := (float32(len(localDB.TitlesMap)) / float32(len(titlesDB.TitlesMap))) * 100

	fmt.Printf("Local library completion status: %.2f%% (have %d titles, out of %d titles)\n", p, len(localDB.TitlesMap), len(titlesDB.TitlesMap))

	issuesCsvFile := ""
	if csvOutput != "" {
		issuesCsvFile = filepath.Join(csvOutput, csvExportFilename(csvIssuesKind, exportDate))
	}
	c.processIssues(localDB, issuesCsvFile)

	if settingsObj.Organization.DeleteOldUpdateFiles {
		progressBar = progressbar.New(2000)
		fmt.Printf("\nDeleting old updates\n")
		process.DeleteOldUpdates(c.baseFolder, localDB, c)
		progressBar.Finish()
	}

	if settingsObj.Organization.RenameFiles || settingsObj.Organization.CreateFolderPerGame {
		progressBar = progressbar.New(2000)
		fmt.Printf("\nStarting library organization\n")
		process.OrganizeByFolders(folderToScan, localDB, titlesDB, c)
		progressBar.Finish()
	}

	if settingsObj.MissingContent.CheckForUpdates {
		fmt.Printf("\nChecking for missing updates\n")

		missingUpdatesCsvFile := ""
		if csvOutput != "" {
			missingUpdatesCsvFile = filepath.Join(csvOutput, csvExportFilename(csvMissingUpdatesKind, exportDate))
		}

		c.processMissingUpdates(localDB, titlesDB, settingsObj, missingUpdatesCsvFile)
	}

	if settingsObj.MissingContent.CheckForDLC {
		fmt.Printf("\nChecking for missing DLC\n")

		missingDlcCsvFile := ""
		if csvOutput != "" {
			missingDlcCsvFile = filepath.Join(csvOutput, csvExportFilename(csvMissingDLCKind, exportDate))
		}

		c.processMissingDLC(localDB, titlesDB, missingDlcCsvFile)
	}

	fmt.Printf("Completed")
}

func (c *Console) processIssues(localDB *db.LocalSwitchFilesDB, csvOutput string) {
	if len(localDB.Skipped) != 0 {
		fmt.Print("\nSkipped files:\n\n")
	} else {
		return
	}

	csv := CreateCsvFile(csvOutput, []string{"Skipped file", "Reason", "Reason_Code"})

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Skipped file", "Reason"})
	i := 0
	for k, v := range localDB.Skipped {
		csv.Write([]string{path.Join(k.BaseFolder, k.FileName), v.ReasonText, strconv.Itoa(v.ReasonCode)})

		t.AppendRow([]interface{}{i, path.Join(k.BaseFolder, k.FileName), v})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(localDB.Skipped)})
	t.Render()

	csv.Close()
}

func (c *Console) processMissingUpdates(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, settingsObj *settings.AppSettings, csvOutput string) {
	ignoreIds := map[string]struct{}{}
	for _, id := range settingsObj.MissingContent.IgnoreUpdateIDs {
		ignoreIds[strings.ToLower(id)] = struct{}{}
	}

	incompleteTitles := process.ScanForMissingUpdates(localDB.TitlesMap, titlesDB.TitlesMap, ignoreIds, settingsObj.MissingContent.IgnoreDLCUpdates)
	if len(incompleteTitles) != 0 {
		fmt.Print("\nFound available updates:\n\n")
	} else {
		fmt.Print("\nAll NSP's are up to date!\n\n")
		return
	}

	csv := CreateCsvFile(csvOutput, []string{"Title", "TitleId", "Local version", "Latest Version", "Update Date"})

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Local version", "Latest Version", "Update Date"})
	i := 0
	for _, v := range incompleteTitles {
		csv.Write([]string{v.Attributes.Name, v.Attributes.Id, strconv.Itoa(v.LocalUpdate), strconv.Itoa(v.LatestUpdate), v.LatestUpdateDate})

		t.AppendRow([]interface{}{i, v.Attributes.Name, v.Attributes.Id, v.LocalUpdate, v.LatestUpdate, v.LatestUpdateDate})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(incompleteTitles)})
	t.Render()

	csv.Close()
}

func (c *Console) processMissingDLC(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, csvOutput string) {
	settingsObj, err := settings.ReadSettings(c.baseFolder)
	if err != nil {
		return
	}
	ignoreIds := map[string]struct{}{}
	for _, id := range settingsObj.MissingContent.IgnoreDLCTitleIDs {
		ignoreIds[strings.ToLower(id)] = struct{}{}
	}
	incompleteTitles := process.ScanForMissingDLC(localDB.TitlesMap, titlesDB.TitlesMap, ignoreIds)
	if len(incompleteTitles) != 0 {
		fmt.Print("\nFound missing DLCS:\n\n")
	} else {
		fmt.Print("\nYou have all the DLCS!\n\n")
		return
	}

	csv := CreateCsvFile(csvOutput, []string{"Title", "TitleId", "Dlc (titleId - Name)"})

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Missing DLCs (titleId - Name)"})
	i := 0
	for _, v := range incompleteTitles {
		for _, dlc := range v.MissingDLC {
			csv.Write([]string{v.Attributes.Name, v.Attributes.Id, dlc})
		}

		t.AppendRow([]interface{}{i, v.Attributes.Name, v.Attributes.Id, strings.Join(v.MissingDLC, "\n")})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(incompleteTitles)})
	t.Render()

	csv.Close()
}

func (c *Console) UpdateProgress(curr int, total int, message string) {
	progressBar.ChangeMax(total)
	progressBar.Set(curr)
}

type CsvFile struct {
	Writer *csv.Writer
	File   *os.File
}

func CreateCsvFile(output string, header []string) *CsvFile {
	if output != "" {
		file, _ := os.Create(output)
		writer := csv.NewWriter(file)

		_ = writer.Write(header)

		instance := &CsvFile{Writer: writer, File: file}
		return instance
	} else {
		return nil
	}
}

func (csv *CsvFile) Close() {
	if csv != nil {
		csv.Writer.Flush()
		csv.File.Close()
	}
}

func (csv *CsvFile) Write(row []string) {
	if csv != nil {
		_ = csv.Writer.Write(row)
	}
}
