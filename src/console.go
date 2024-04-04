package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/jedib0t/go-pretty/table"
	"github.com/schollz/progressbar/v3"
	"github.com/trembon/switch-library-manager/db"
	"github.com/trembon/switch-library-manager/process"
	"github.com/trembon/switch-library-manager/settings"
	"go.uber.org/zap"
)

const (
	ATTACH_PARENT_PROCESS = ^uint32(0) // (DWORD)-1
)

var (
	modkernel32       = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole = modkernel32.NewProc("AttachConsole")
)

func AttachConsole(dwParentProcess uint32) (ok bool) {
	r0, _, _ := syscall.SyscallN(procAttachConsole.Addr(), 1, uintptr(dwParentProcess), 0, 0)
	ok = bool(r0 != 0)
	return
}

var (
	progressBar *progressbar.ProgressBar
)

type Console struct {
	baseFolder  string
	sugarLogger *zap.SugaredLogger
}

func CreateConsole(baseFolder string, sugarLogger *zap.SugaredLogger) *Console {
	return &Console{baseFolder: baseFolder, sugarLogger: sugarLogger}
}

func (c *Console) Start() {
	ok := AttachConsole(ATTACH_PARENT_PROCESS)
	if ok {
		fmt.Println("Okay, attached")
	}

	flagSet := flag.NewFlagSet("console", flag.ContinueOnError)

	nspFolder := flagSet.String("f", "", "path to NSP folder")
	recursive := flagSet.Bool("r", true, "recursively scan sub folders")
	exportCsvFolder := flagSet.String("export", "", "path to NSP folder")

	flagSet.Parse(os.Args[1:])
	fmt.Println("1")

	fmt.Println("2")

	settingsObj := settings.ReadSettings(c.baseFolder)

	fmt.Println("3")
	csvOutput := *exportCsvFolder
	fmt.Println("exportCsvFolder", *exportCsvFolder)
	/*if indexOf(os.Args, "--export-csv") >= 0 {
		index := indexOf(os.Args, "--export-csv")
		if len(os.Args) > index+1 && !strings.HasPrefix(os.Args[index+1], "--") {
			csvOutput = os.Args[index+1]
		} else {
			csvOutput = filepath.Join(c.baseFolder, "csv")
		}

		if _, err := os.Stat(csvOutput); os.IsNotExist(err) {
			err = os.Mkdir(csvOutput, os.ModePerm)
			if err != nil {
				fmt.Printf("Failed to create folder for csv output %v - %v\n", csvOutput, err)
				zap.S().Errorf("Failed to create folder for csv output %v - %v\n", csvOutput, err)
			}
		}
	}*/
	fmt.Println("4")

	//1. load the titles JSON object
	fmt.Printf("Downlading latest switch titles json file")
	progressBar = progressbar.New(2)

	filename := filepath.Join(c.baseFolder, settings.TITLE_JSON_FILENAME)
	titleFile, titlesEtag, err := db.LoadAndUpdateFile(settings.TITLES_JSON_URL, filename, settingsObj.TitlesEtag)
	if err != nil {
		fmt.Printf("title json file doesn't exist\n")
		return
	}
	settingsObj.TitlesEtag = titlesEtag
	progressBar.Add(1)
	//2. load the versions JSON object
	filename = filepath.Join(c.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsEtag, err := db.LoadAndUpdateFile(settings.VERSIONS_JSON_URL, filename, settingsObj.VersionsEtag)
	if err != nil {
		fmt.Printf("version json file doesn't exist\n")
		return
	}
	settingsObj.VersionsEtag = versionsEtag
	progressBar.Add(1)
	progressBar.Finish()
	newUpdate, err := settings.CheckForUpdates()

	if newUpdate {
		fmt.Printf("\n=== New version available, download from Github ===\n")
	}

	//3. update the config file with new etag
	settings.SaveSettings(settingsObj, c.baseFolder)

	//4. create switch title db
	titlesDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)

	//5. read local files
	folderToScan := settingsObj.Folder
	if nspFolder != nil && *nspFolder != "" {
		folderToScan = *nspFolder
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

	recursiveMode := settingsObj.ScanRecursively
	if recursive != nil && *recursive != true {
		recursiveMode = *recursive
	}

	localDbManager, err := db.NewLocalSwitchDBManager(c.baseFolder)
	if err != nil {
		fmt.Printf("failed to create local files db :%v\n", err)
		return
	}
	defer localDbManager.Close()

	scanFolders := settingsObj.ScanFolders
	scanFolders = append(scanFolders, folderToScan)

	localDB, err := localDbManager.CreateLocalSwitchFilesDB(scanFolders, c, recursiveMode, true)
	if err != nil {
		fmt.Printf("\nfailed to process local folder\n %v", err)
		return
	}
	progressBar.Finish()

	p := (float32(len(localDB.TitlesMap)) / float32(len(titlesDB.TitlesMap))) * 100

	fmt.Printf("Local library completion status: %.2f%% (have %d titles, out of %d titles)\n", p, len(localDB.TitlesMap), len(titlesDB.TitlesMap))

	c.processIssues(localDB)

	if settingsObj.OrganizeOptions.DeleteOldUpdateFiles {
		progressBar = progressbar.New(2000)
		fmt.Printf("\nDeleting old updates\n")
		process.DeleteOldUpdates(c.baseFolder, localDB, c)
		progressBar.Finish()
	}

	if settingsObj.OrganizeOptions.RenameFiles || settingsObj.OrganizeOptions.CreateFolderPerGame {
		progressBar = progressbar.New(2000)
		fmt.Printf("\nStarting library organization\n")
		process.OrganizeByFolders(folderToScan, localDB, titlesDB, c)
		progressBar.Finish()
	}

	if settingsObj.CheckForMissingUpdates {
		fmt.Printf("\nChecking for missing updates\n")

		missingUpdatesCsvFile := ""
		if csvOutput != "" {
			missingUpdatesCsvFile = filepath.Join(csvOutput, "missing_updates.csv")
		}

		c.processMissingUpdates(localDB, titlesDB, settingsObj, missingUpdatesCsvFile)
	}

	if settingsObj.CheckForMissingDLC {
		fmt.Printf("\nChecking for missing DLC\n")

		missingDlcCsvFile := ""
		if csvOutput != "" {
			missingDlcCsvFile = filepath.Join(csvOutput, "missing_dlc.csv")
		}

		c.processMissingDLC(localDB, titlesDB, missingDlcCsvFile)
	}

	fmt.Printf("Completed")
}

func (c *Console) processIssues(localDB *db.LocalSwitchFilesDB) {
	if len(localDB.Skipped) != 0 {
		fmt.Print("\nSkipped files:\n\n")
	} else {
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Skipped file", "Reason"})
	i := 0
	for k, v := range localDB.Skipped {
		t.AppendRow([]interface{}{i, path.Join(k.BaseFolder, k.FileName), v})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(localDB.Skipped)})
	t.Render()
}

func (c *Console) processMissingUpdates(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, settingsObj *settings.AppSettings, csvOutput string) {
	var csvWriter *csv.Writer = nil
	var csvFile *os.File = nil
	if csvOutput != "" {
		csvFile, _ = os.Create(csvOutput)
		csvWriter = csv.NewWriter(csvFile)
	}

	incompleteTitles := process.ScanForMissingUpdates(localDB.TitlesMap, titlesDB.TitlesMap, settingsObj.IgnoreDLCUpdates)
	if len(incompleteTitles) != 0 {
		fmt.Print("\nFound available updates:\n\n")
	} else {
		fmt.Print("\nAll NSP's are up to date!\n\n")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Local version", "Latest Version", "Update Date"})
	i := 0
	for _, v := range incompleteTitles {
		if csvWriter != nil {
			row := []string{v.Attributes.Name, v.Attributes.Id, strconv.Itoa(v.LocalUpdate), strconv.Itoa(v.LatestUpdate), v.LatestUpdateDate}
			_ = csvWriter.Write(row)
		}

		t.AppendRow([]interface{}{i, v.Attributes.Name, v.Attributes.Id, v.LocalUpdate, v.LatestUpdate, v.LatestUpdateDate})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(incompleteTitles)})
	t.Render()

	if csvOutput != "" {
		csvWriter.Flush()
		csvFile.Close()
	}
}

func (c *Console) processMissingDLC(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, csvOutput string) {
	var csvWriter *csv.Writer = nil
	var csvFile *os.File = nil
	if csvOutput != "" {
		csvFile, _ = os.Create(csvOutput)
		csvWriter = csv.NewWriter(csvFile)
	}

	settingsObj := settings.ReadSettings(c.baseFolder)
	ignoreIds := map[string]struct{}{}
	for _, id := range settingsObj.IgnoreDLCTitleIds {
		ignoreIds[strings.ToLower(id)] = struct{}{}
	}
	incompleteTitles := process.ScanForMissingDLC(localDB.TitlesMap, titlesDB.TitlesMap, ignoreIds)
	if len(incompleteTitles) != 0 {
		fmt.Print("\nFound missing DLCS:\n\n")
	} else {
		fmt.Print("\nYou have all the DLCS!\n\n")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Missing DLCs (titleId - Name)"})
	i := 0
	for _, v := range incompleteTitles {
		if csvWriter != nil {
			row := []string{v.Attributes.Name, v.Attributes.Id, strings.Join(v.MissingDLC, "\n")}
			_ = csvWriter.Write(row)
		}

		t.AppendRow([]interface{}{i, v.Attributes.Name, v.Attributes.Id, strings.Join(v.MissingDLC, "\n")})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(incompleteTitles)})
	t.Render()

	if csvOutput != "" {
		csvWriter.Flush()
		csvFile.Close()
	}
}

func (c *Console) UpdateProgress(curr int, total int, message string) {
	progressBar.ChangeMax(total)
	progressBar.Set(curr)
}

func indexOf(s []string, e string) int {
	for i, a := range s {
		if a == e {
			return i
		}
	}
	return -1
}
