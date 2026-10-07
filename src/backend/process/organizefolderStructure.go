package process

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"go.uber.org/zap"
	"robpike.io/nihongo"
)

var (
	folderIllegalCharsRegex = regexp.MustCompile(`[/\\?%*:;=|"<>]`)
	nonAscii                = regexp.MustCompile("[a-zA-Z0-9áéíóú@#%&',.\\s-\\[\\]\\(\\)\\+]")
	cjk                     = regexp.MustCompile("[\u2f70-\u2FA1\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\uff66-\uff9f\\p{Katakana}\\p{Hiragana}\\p{Hangul}]")
)

var packageContentsRegex = regexp.MustCompile(`(?i)\(([1-9][0-9]{0,8})g(?:\+([1-9][0-9]{0,8})u)?(?:\+([1-9][0-9]{0,8})d)?\)`)

func DeleteOldUpdates(baseFolder string, localDB *db.LocalSwitchFilesDB, updateProgress db.ProgressUpdater) error {
	return DeleteOldUpdatesAt(baseFolder, "", localDB, updateProgress)
}

// DeleteOldUpdatesAt removes only scan-verified redundant files and limits
// empty-folder cleanup to the supplied library root.
func DeleteOldUpdatesAt(baseFolder, cleanupRoot string, localDB *db.LocalSwitchFilesDB, updateProgress db.ProgressUpdater) error {
	if localDB == nil {
		return fmt.Errorf("local library has not been scanned")
	}
	settingsObj, err := settings.ReadSettings(baseFolder)
	if err != nil {
		return fmt.Errorf("read settings before deleting old updates: %w", err)
	}
	if cleanupRoot == "" {
		cleanupRoot = settingsObj.Paths.LibraryFolder
	}
	candidates := append([]db.CleanupCandidate(nil), localDB.CleanupCandidates...)
	sort.Slice(candidates, func(i, j int) bool {
		left := filepath.Join(candidates[i].File.BaseFolder, candidates[i].File.FileName)
		right := filepath.Join(candidates[j].File.BaseFolder, candidates[j].File.FileName)
		return left < right
	})
	removed := 0
	for _, candidate := range candidates {
		source := filepath.Join(candidate.File.BaseFolder, candidate.File.FileName)
		replacement := filepath.Join(candidate.Replacement.BaseFolder, candidate.Replacement.FileName)
		sourceInfo, err := os.Lstat(source)
		if err != nil {
			return fmt.Errorf("verify cleanup candidate %q: %w", source, err)
		}
		if !sourceInfo.Mode().IsRegular() || sourceInfo.Size() != candidate.File.Size || !sourceInfo.ModTime().Equal(candidate.File.ModTime) {
			return fmt.Errorf("cleanup candidate %q changed after scanning", source)
		}
		replacementInfo, err := os.Lstat(replacement)
		if err != nil {
			return fmt.Errorf("verify replacement %q for %q: %w", replacement, source, err)
		}
		if !replacementInfo.Mode().IsRegular() || replacementInfo.Size() != candidate.Replacement.Size || !replacementInfo.ModTime().Equal(candidate.Replacement.ModTime) {
			return fmt.Errorf("replacement %q for cleanup candidate %q changed after scanning", replacement, source)
		}
		if os.SameFile(sourceInfo, replacementInfo) {
			continue
		}
		if updateProgress != nil {
			updateProgress.UpdateProgress(removed+1, len(candidates), "deleting "+source)
		}
		if err := os.Remove(source); err != nil {
			return fmt.Errorf("delete verified duplicate %q: %w", source, err)
		}
		deleteSkippedPath(localDB, candidate.File)
		removed++
		zap.S().Infof("Deleted verified duplicate file: %s", source)
	}
	localDB.CleanupCandidates = nil
	if removed > 0 && settingsObj.Organization.DeleteEmptyFolders && cleanupRoot != "" {
		if err := deleteEmptyFolders(cleanupRoot); err != nil {
			return fmt.Errorf("delete empty folders: %w", err)
		}
	}
	return nil
}

func deleteSkippedPath(localDB *db.LocalSwitchFilesDB, file db.ExtendedFileInfo) {
	for key := range localDB.Skipped {
		if filepath.Clean(filepath.Join(key.BaseFolder, key.FileName)) == filepath.Clean(filepath.Join(file.BaseFolder, file.FileName)) {
			delete(localDB.Skipped, key)
		}
	}
}

func IsOptionsValid(options settings.OrganizeOptions) bool {
	if options.RenameFiles {
		if options.FileNameTemplate == "" {
			zap.S().Error("file name template cannot be empty")
			return false
		}
		if !strings.Contains(options.FileNameTemplate, settings.TEMPLATE_TITLE_NAME) &&
			!strings.Contains(options.FileNameTemplate, settings.TEMPLATE_TITLE_ID) {
			zap.S().Error("file name template needs to contain one of the following - titleId or title name")
			return false
		}

	}

	if options.CreateFolderPerGame {
		if options.FolderNameTemplate == "" {
			zap.S().Error("folder name template cannot be empty")
			return false
		}
		if !strings.Contains(options.FolderNameTemplate, settings.TEMPLATE_TITLE_NAME) &&
			!strings.Contains(options.FolderNameTemplate, settings.TEMPLATE_TITLE_ID) {
			zap.S().Error("folder name template needs to contain one of the following - titleId or title name")
			return false
		}
	}
	return true
}

func getDlcName(switchTitle *db.SwitchTitle, file db.SwitchFileInfo) string {
	if switchTitle == nil || file.Metadata == nil {
		return ""
	}
	if dlcAttributes, ok := switchTitle.Dlc[file.Metadata.TitleId]; ok {
		name := dlcAttributes.Name
		name = strings.ReplaceAll(name, "\n", " ")
		return name
	}
	return ""
}

func getTitleName(switchTitle *db.SwitchTitle, v *db.SwitchGameFiles) string {
	// Check if switchTitle is not nil and contains a name
	if switchTitle != nil && switchTitle.Attributes.Name != "" {
		res := cjk.FindAllString(switchTitle.Attributes.Name, -1)
		if len(res) == 0 {
			return switchTitle.Attributes.Name
		}
	}

	// Check if v and its Metadata are present
	if v != nil && v.File.Metadata != nil && v.File.Metadata.Ncap != nil {
		// Check if the title name exists
		name := v.File.Metadata.Ncap.TitleName["AmericanEnglish"].Title
		if name != "" {
			return name
		}
	}

	// For non-eShop games, get the name from the file
	if v != nil {
		return db.ParseTitleNameFromFileName(v.File.ExtendedInfo.FileName)
	}

	// Default return if no valid name is found
	return "Unknown Title"
}

func getFolderName(options settings.OrganizeOptions, templateData map[string]string) string {

	return applyTemplate(templateData, options.SwitchSafeFileNames, options.FolderNameTemplate, 0)
}

func getFileName(options settings.OrganizeOptions, originalName string, templateData map[string]string, nameTry int) string {
	if !options.RenameFiles {
		return originalName
	}
	ext := path.Ext(originalName)
	result := applyTemplate(templateData, options.SwitchSafeFileNames, options.FileNameTemplate, nameTry)
	return result + ext
}

func setDlcVersionTemplateData(templateData map[string]string, dlc db.SwitchFileInfo) {
	templateData[settings.TEMPLATE_VERSION] = "0"
	templateData[settings.TEMPLATE_VERSION_TXT] = ""
	if dlc.Metadata == nil {
		return
	}

	templateData[settings.TEMPLATE_VERSION] = strconv.Itoa(dlc.Metadata.Version)
	if dlc.Metadata.Ncap != nil && dlc.Metadata.Ncap.DisplayVersion != "" {
		templateData[settings.TEMPLATE_VERSION_TXT] = dlc.Metadata.Ncap.DisplayVersion
		return
	}
	if dlc.Metadata.Version == 0 {
		templateData[settings.TEMPLATE_VERSION_TXT] = "1.0.0"
	}
}

func setBaseFileVersionTemplateData(templateData map[string]string, game *db.SwitchGameFiles) {
	templateData[settings.TEMPLATE_VERSION] = "0"
	templateData[settings.TEMPLATE_VERSION_TXT] = ""
	if game == nil {
		return
	}

	if game.File.Metadata != nil && game.File.Metadata.Ncap != nil {
		templateData[settings.TEMPLATE_VERSION_TXT] = game.File.Metadata.Ncap.DisplayVersion
	}

	version, update, found := highestBundledUpdate(game)
	if !found {
		return
	}

	templateData[settings.TEMPLATE_VERSION] = strconv.Itoa(version)
	templateData[settings.TEMPLATE_VERSION_TXT] = ""
	if update.Metadata != nil && update.Metadata.Ncap != nil {
		templateData[settings.TEMPLATE_VERSION_TXT] = update.Metadata.Ncap.DisplayVersion
	}
}

func packageContents(game *db.SwitchGameFiles) string {
	if game == nil || !game.BaseExist {
		return ""
	}

	// Count logical records in the base file itself; MultiContent also covers
	// records spread across separate files and is not sufficient for naming.
	updateCount := 0
	for _, update := range game.Updates {
		if samePhysicalFilePath(game.File.ExtendedInfo, update.ExtendedInfo) {
			updateCount++
		}
	}

	dlcCount := 0
	for _, dlc := range game.Dlc {
		if samePhysicalFilePath(game.File.ExtendedInfo, dlc.ExtendedInfo) {
			dlcCount++
		}
	}

	if updateCount != 0 || dlcCount != 0 {
		parts := []string{"1G"}
		if updateCount != 0 {
			parts = append(parts, strconv.Itoa(updateCount)+"U")
		}
		if dlcCount != 0 {
			parts = append(parts, strconv.Itoa(dlcCount)+"D")
		}
		return strings.Join(parts, "+")
	}

	// Filename-only scans cannot discover package members, so preserve a valid
	// source marker when one is present.
	return packageContentsFromFileName(game.File.ExtendedInfo.FileName)
}

func packageContentsFromFileName(fileName string) string {
	match := packageContentsRegex.FindStringSubmatch(fileName)
	if len(match) != 4 || (match[2] == "" && match[3] == "") {
		return ""
	}

	parts := []string{match[1] + "G"}
	if match[2] != "" {
		parts = append(parts, match[2]+"U")
	}
	if match[3] != "" {
		parts = append(parts, match[3]+"D")
	}
	return strings.Join(parts, "+")
}

func highestBundledUpdate(game *db.SwitchGameFiles) (int, db.SwitchFileInfo, bool) {
	if game == nil || !game.BaseExist {
		return 0, db.SwitchFileInfo{}, false
	}

	latestVersion := 0
	var latestUpdate db.SwitchFileInfo
	found := false
	for version, update := range game.Updates {
		if !samePhysicalFilePath(game.File.ExtendedInfo, update.ExtendedInfo) {
			continue
		}
		if !found || version > latestVersion {
			latestVersion = version
			latestUpdate = update
			found = true
		}
	}
	return latestVersion, latestUpdate, found
}

func samePhysicalFilePath(left, right db.ExtendedFileInfo) bool {
	if left.FileName == "" || right.FileName == "" {
		return false
	}
	leftPath := filepath.Clean(filepath.Join(left.BaseFolder, left.FileName))
	rightPath := filepath.Clean(filepath.Join(right.BaseFolder, right.FileName))
	return leftPath == rightPath
}

func organizationDestinationFolder(libraryRoot, destinationPath, sourceFolder string, options settings.OrganizeOptions) (string, bool, error) {
	importing := false
	if options.MoveScanFilesToLibrary {
		insideLibrary, err := organizationPathWithin(libraryRoot, sourceFolder)
		if err != nil {
			return "", false, err
		}
		importing = !insideLibrary
	}
	if importing && !options.CreateFolderPerGame {
		destinationPath = libraryRoot
	}
	return destinationPath, importing, nil
}

func organizationTargetPath(libraryRoot, destinationPath, sourceFolder, originalName string, options settings.OrganizeOptions, contentKind string, templateData map[string]string, nameTry int) (string, error) {
	var importing bool
	var err error
	destinationPath, importing, err = organizationDestinationFolder(libraryRoot, destinationPath, sourceFolder, options)
	if err != nil {
		return "", err
	}
	fileName := getFileName(options, originalName, templateData, nameTry)
	configuredFolder := ""
	switch contentKind {
	case "update":
		configuredFolder = options.UpdatesFolder
	case "dlc":
		configuredFolder = options.DlcFolder
	}
	if configuredFolder == "" {
		if !options.CreateFolderPerGame && !importing {
			return filepath.Join(sourceFolder, fileName), nil
		}
		return filepath.Join(destinationPath, fileName), nil
	}
	if importing && filepath.IsAbs(configuredFolder) {
		return filepath.Join(configuredFolder, fileName), nil
	}
	if !options.CreateFolderPerGame && !importing {
		return filepath.Join(configuredFolder, fileName), nil
	}
	target := filepath.Join(destinationPath, configuredFolder, fileName)
	if importing {
		insideLibrary, err := organizationPathWithin(libraryRoot, target)
		if err != nil {
			return "", err
		}
		if !insideLibrary {
			return "", fmt.Errorf("relative organization folder %q escapes the library folder", configuredFolder)
		}
	}
	return target, nil
}

func applyTemplate(templateData map[string]string, useSafeNames bool, template string, nameTry int) string {
	result := strings.ReplaceAll(template, "{"+settings.TEMPLATE_TITLE_NAME+"}", templateData[settings.TEMPLATE_TITLE_NAME])
	result = strings.ReplaceAll(result, "{"+settings.TEMPLATE_TITLE_ID+"}", strings.ToUpper(templateData[settings.TEMPLATE_TITLE_ID]))
	result = strings.ReplaceAll(result, "{"+settings.TEMPLATE_VERSION+"}", templateData[settings.TEMPLATE_VERSION])
	result = strings.ReplaceAll(result, "{"+settings.TEMPLATE_TYPE+"}", templateData[settings.TEMPLATE_TYPE])
	result = strings.ReplaceAll(result, "{"+settings.TEMPLATE_VERSION_TXT+"}", templateData[settings.TEMPLATE_VERSION_TXT])
	result = strings.ReplaceAll(result, "{"+settings.TEMPLATE_REGION+"}", templateData[settings.TEMPLATE_REGION])
	result = strings.ReplaceAll(result, "{"+settings.TEMPLATE_SIZE_GB+"}", templateData[settings.TEMPLATE_SIZE_GB])
	result = strings.ReplaceAll(result, "{"+settings.TEMPLATE_SIZE_MB+"}", templateData[settings.TEMPLATE_SIZE_MB])
	result = strings.ReplaceAll(result, "{"+settings.TEMPLATE_PACKAGE_CONTENTS+"}", templateData[settings.TEMPLATE_PACKAGE_CONTENTS])

	//remove title name from dlc name
	dlcName := strings.Replace(templateData[settings.TEMPLATE_DLC_NAME], templateData[settings.TEMPLATE_TITLE_NAME], "", 1)
	dlcName = strings.TrimSpace(dlcName)
	dlcName = strings.TrimPrefix(dlcName, "-")
	dlcName = strings.TrimSpace(dlcName)

	result = strings.ReplaceAll(result, "{"+settings.TEMPLATE_DLC_NAME+"}", dlcName)
	result = strings.ReplaceAll(result, "[]", "")
	result = strings.ReplaceAll(result, "()", "")
	result = strings.ReplaceAll(result, "<>", "")

	result = strings.TrimSuffix(result, ".")

	if nameTry > 0 {
		result = result + "(" + strconv.Itoa(nameTry) + ")"
	}

	if useSafeNames {
		result = nihongo.RomajiString(result)

		// handle known characters that have safe variants
		result = strings.ReplaceAll(result, "ō", "o")

		safe := nonAscii.FindAllString(result, -1)
		result = strings.Join(safe, "")
	}

	space := regexp.MustCompile(`\s+`)
	result = space.ReplaceAllString(result, " ")

	result = strings.TrimSpace(result)
	return folderIllegalCharsRegex.ReplaceAllString(result, "")
}

func setFileSizeTemplateData(templateData map[string]string, size int64) {
	const (
		bytesPerTenthGB = int64(100_000_000)
		bytesPerMB      = int64(1_000_000)
	)

	gbTenths := size / bytesPerTenthGB
	if size%bytesPerTenthGB >= bytesPerTenthGB/2 {
		gbTenths++
	}

	mb := size / bytesPerMB
	if size%bytesPerMB >= bytesPerMB/2 {
		mb++
	}

	templateData[settings.TEMPLATE_SIZE_GB] = strconv.FormatInt(gbTenths/10, 10) + "." + strconv.FormatInt(gbTenths%10, 10) + "GB"
	templateData[settings.TEMPLATE_SIZE_MB] = strconv.FormatInt(mb, 10) + "MB"
}

func createFolder(path string, logger *zap.SugaredLogger) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.Mkdir(path, os.ModePerm)
		if err != nil {
			logger.Errorf("Failed to create folder %v - %v\n", path, err)
			return err
		}
	}
	return nil
}

func deleteEmptyFolders(path string) error {
	root, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve cleanup root %q: %w", path, err)
	}
	directories := make([]string, 0)
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && info.IsDir() {
			directories = append(directories, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(directories, func(i, j int) bool {
		if len(directories[i]) != len(directories[j]) {
			return len(directories[i]) > len(directories[j])
		}
		return directories[i] > directories[j]
	})
	for _, directory := range directories {
		if filepath.Clean(directory) == filepath.Clean(root) {
			continue
		}
		if err := deleteEmptyFolder(directory); err != nil {
			return fmt.Errorf("delete empty folder %q: %w", directory, err)
		}
	}
	return nil
}

func deleteEmptyFolder(path string) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	if len(files) != 0 {
		return nil
	}

	zap.S().Infof("\nDeleting empty folder [%v]", path)
	return os.Remove(path)
}
