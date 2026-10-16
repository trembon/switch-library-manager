package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/trembon/switch-library-manager/backend/fileio"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

var (
	versionRegex              = regexp.MustCompile(`\[[vV]?(?P<version>[0-9]{1,10})]`)
	titleIdRegex              = regexp.MustCompile(`\[(?P<titleId>[A-Fa-f0-9]{16})]`)
	switchFileExtensionSuffix = regexp.MustCompile(`(?i)\.(nsp|nsz|xci|xcz)$`)
	versionNameSuffix         = regexp.MustCompile(`(?i)\s*\(v[0-9]+(\.[0-9]+)*(\s+v[0-9]+(\.[0-9]+)*)*\)$`)
	emptyVersionNameSuffix    = regexp.MustCompile(`(?i)\s*\(v\)$`)
	packageContentsNameSuffix = regexp.MustCompile(`(?i)\s*\([1-9][0-9]{0,8}g(\+[1-9][0-9]{0,8}u)?(\+[1-9][0-9]{0,8}d)?\)$`)
)

const (
	DB_TABLE_FILE_SCAN_METADATA = "deep-scan"
	DB_TABLE_LOCAL_LIBRARY      = "local-library"

	REASON_UNSUPPORTED_TYPE = iota
	REASON_DUPLICATE
	REASON_OLD_UPDATE
	REASON_UNRECOGNISED
	REASON_MALFORMED_FILE
)

type LocalSwitchDBManager struct {
	db *PersistentDB
}

func NewLocalSwitchDBManager(baseFolder string) (*LocalSwitchDBManager, error) {
	db, err := NewPersistentDB(baseFolder)
	if err != nil {
		return nil, err
	}
	return &LocalSwitchDBManager{db: db}, nil
}

func (ldb *LocalSwitchDBManager) Close() {
	ldb.db.Close()
}

type ExtendedFileInfo struct {
	FileName   string
	BaseFolder string
	Size       int64
	ModTime    time.Time
	IsDir      bool
}

type SwitchFileInfo struct {
	ExtendedInfo ExtendedFileInfo
	Metadata     *switchfs.ContentMetaAttributes
}

type SwitchGameFiles struct {
	File         SwitchFileInfo
	BaseExist    bool
	Updates      map[int]SwitchFileInfo
	Dlc          map[string]SwitchFileInfo
	MultiContent bool
	LatestUpdate int
	IsSplit      bool
}

type SkippedFile struct {
	ReasonCode     int
	ReasonText     string
	AdditionalInfo string
}

type LocalSwitchFilesDB struct {
	TitlesMap         map[string]*SwitchGameFiles
	Skipped           map[ExtendedFileInfo]SkippedFile
	CleanupCandidates []CleanupCandidate
	NumFiles          int
}

// CleanupCandidate is a scanned standalone file that is redundant with a
// distinct record still present in TitlesMap. Candidates are transient; a
// persisted scan snapshot cannot authorize a later deletion.
type CleanupCandidate struct {
	File        ExtendedFileInfo
	Replacement ExtendedFileInfo
	ReasonCode  int
}

func (ldb *LocalSwitchDBManager) CreateLocalSwitchFilesDB(folders []string,
	progress ProgressUpdater, recursive bool, ignoreCache bool) (*LocalSwitchFilesDB, error) {

	titles := map[string]*SwitchGameFiles{}
	skipped := map[ExtendedFileInfo]SkippedFile{}
	files := []ExtendedFileInfo{}
	cleanupCandidates := []CleanupCandidate{}

	if !ignoreCache {
		if err := ldb.db.GetEntry(DB_TABLE_LOCAL_LIBRARY, "files", &files); err != nil {
			return nil, err
		}
		if err := ldb.db.GetEntry(DB_TABLE_LOCAL_LIBRARY, "skipped", &skipped); err != nil {
			return nil, err
		}
		if err := ldb.db.GetEntry(DB_TABLE_LOCAL_LIBRARY, "titles", &titles); err != nil {
			return nil, err
		}
	}

	if ignoreCache || len(titles) == 0 {
		seenFiles := seenScanFiles{}
		for i, folder := range folders {
			err := scanFolder(folder, recursive, &files, progress, seenFiles)
			if progress != nil {
				progress.UpdateProgress(i+1, len(folders)+1, "scanning files in "+folder)
			}
			if err != nil {
				return nil, fmt.Errorf("scan folder %q: %w", folder, err)
			}
		}

		fileContents := ldb.processLocalFiles(files, progress, titles, skipped)
		cleanupCandidates = buildCleanupCandidates(files, fileContents, titles, skipped)

		if err := ldb.db.addEntries(DB_TABLE_LOCAL_LIBRARY, map[string]interface{}{
			"files":   files,
			"skipped": skipped,
			"titles":  titles,
		}); err != nil {
			return nil, err
		}
	}

	if progress != nil {
		progress.UpdateProgress(len(files), len(files), "Complete")
	}

	return &LocalSwitchFilesDB{TitlesMap: titles, Skipped: skipped, CleanupCandidates: cleanupCandidates, NumFiles: len(files)}, nil
}

func scanFolder(folder string, recursive bool, files *[]ExtendedFileInfo, progress ProgressUpdater, seenFiles seenScanFiles) error {
	return scanFolderWithWalker(folder, recursive, files, progress, seenFiles, walkDirWithFileInfo)
}

func walkDirWithFileInfo(folder string, fn filepath.WalkFunc) error {
	// WalkDir lets the callback skip a directory before its contents are read.
	return filepath.WalkDir(folder, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return fn(path, nil, err)
		}
		info, err := entry.Info()
		return fn(path, info, err)
	})
}

func scanFolderWithWalker(
	folder string,
	recursive bool,
	files *[]ExtendedFileInfo,
	progress ProgressUpdater,
	seenFiles seenScanFiles,
	walk func(string, filepath.WalkFunc) error,
) error {
	canonicalRoot := canonicalScanRoot(folder)
	return walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if path != folder && errors.Is(err, os.ErrPermission) {
				zap.S().Warnw("Permission denied while scanning; skipping path", "path", path, "error", err)
				return nil
			}
			zap.S().Error("Error while scanning folders", err)
			return err
		}
		if path == folder {
			return nil
		}
		if info == nil {
			return fmt.Errorf("file information is unavailable for %q", path)
		}
		if info.IsDir() {
			if !recursive {
				return filepath.SkipDir
			}
			return nil
		}

		if runtime.GOOS == "darwin" && strings.EqualFold(info.Name(), ".ds_store") {
			return nil
		}

		base := path[0 : len(path)-len(info.Name())]
		if scannedPathSeen(canonicalRoot, folder, path, seenFiles) {
			return nil
		}
		if progress != nil {
			progress.UpdateProgress(-1, -1, "scanning "+info.Name())
		}
		*files = append(*files, ExtendedFileInfo{FileName: info.Name(), BaseFolder: base, Size: info.Size(), ModTime: info.ModTime(), IsDir: info.IsDir()})

		return nil
	})
}

// canonicalScanRoot resolves a scan folder so overlapping, symlinked, or
// junctioned roots produce the same file keys. Resolution failures fall back
// to the absolute or original path; the walk itself reports unreadable roots.
func canonicalScanRoot(folder string) string {
	root, err := filepath.Abs(folder)
	if err != nil {
		zap.S().Warnf("unable to resolve absolute scan folder %q: %v", folder, err)
		root = folder
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		zap.S().Warnf("unable to resolve links in scan folder %q: %v", root, err)
		return filepath.Clean(root)
	}
	return filepath.Clean(resolved)
}

// seenScanFiles holds the files already reached during one scan, keyed by a
// canonical path that is case-folded where volumes are usually
// case-insensitive.
type seenScanFiles map[string][]seenScanFile

type seenScanFile struct {
	canonicalPath string
	path          string
}

// scannedPathSeen reports whether a file was already reached through another
// scan root. Files are identified by canonical path. Size, timestamps, and
// os.SameFile are not used for files with different names: on Windows,
// directory entries from volumes without file IDs (exFAT, FAT32, many network
// shares) compare as the same file whenever size and timestamps match, which
// silently dropped distinct small DLC files. A hard link under a different
// name is therefore listed separately, matching the behavior before
// overlapping roots were deduplicated.
//
// Paths that differ only by letter case are the same file on a
// case-insensitive volume but distinct files in a case-sensitive directory
// (case-sensitive APFS, or NTFS with per-directory case sensitivity), so those
// are compared by identity instead.
func scannedPathSeen(canonicalRoot, folder, path string, seenFiles seenScanFiles) bool {
	canonicalPath := filepath.Clean(path)
	if rel, err := filepath.Rel(folder, path); err == nil {
		canonicalPath = filepath.Join(canonicalRoot, rel)
	}
	key := canonicalPath
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		key = strings.ToLower(key)
	}
	for _, seen := range seenFiles[key] {
		if seen.canonicalPath == canonicalPath || sameFileByPath(seen.path, path) {
			return true
		}
	}
	seenFiles[key] = append(seenFiles[key], seenScanFile{canonicalPath: canonicalPath, path: path})
	return false
}

// sameFileByPath compares two paths that differ only by letter case. Failing
// to compare keeps both files: a listed duplicate is visible, a dropped file is
// not.
func sameFileByPath(left, right string) bool {
	leftInfo, err := os.Stat(left)
	if err != nil {
		zap.S().Warnf("unable to compare scanned file %q: %v", left, err)
		return false
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		zap.S().Warnf("unable to compare scanned file %q: %v", right, err)
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}

func sameExtendedFilePath(left, right ExtendedFileInfo) bool {
	if left.FileName == "" || right.FileName == "" {
		return false
	}
	leftPath := filepath.Clean(filepath.Join(left.BaseFolder, left.FileName))
	rightPath := filepath.Clean(filepath.Join(right.BaseFolder, right.FileName))
	return leftPath == rightPath
}

func (ldb *LocalSwitchDBManager) ClearScanData() error {
	err := ldb.db.ClearTable(DB_TABLE_FILE_SCAN_METADATA)
	if errors.Is(err, bolt.ErrBucketNotFound) {
		return nil
	}
	return err
}

// ClearLocalLibrarySnapshot removes cached file paths before a destructive
// operation so a failed refresh cannot resurrect stale library state.
func (ldb *LocalSwitchDBManager) ClearLocalLibrarySnapshot() error {
	err := ldb.db.ClearTable(DB_TABLE_LOCAL_LIBRARY)
	if errors.Is(err, bolt.ErrBucketNotFound) {
		return nil
	}
	return err
}

func (ldb *LocalSwitchDBManager) processLocalFiles(files []ExtendedFileInfo,
	progress ProgressUpdater,
	titles map[string]*SwitchGameFiles,
	skipped map[ExtendedFileInfo]SkippedFile) map[ExtendedFileInfo]map[string]*switchfs.ContentMetaAttributes {

	settingsObj, err := settings.ReadSettings("") // use empty path, as it will use existing settings instance
	if err != nil {
		zap.S().Errorf("Failed to read settings while processing local files: %v", err)
		return nil
	}
	ignoreFileTypes := map[string]struct{}{}
	for _, ext := range settingsObj.Scan.IgnoreFileTypes {
		if strings.HasPrefix(ext, ".") {
			ignoreFileTypes[strings.ToLower(ext)] = struct{}{}
		} else {
			ignoreFileTypes["."+strings.ToLower(ext)] = struct{}{}
		}
	}
	if runtime.GOOS == "darwin" {
		ignoreFileTypes[".ds_store"] = struct{}{}
	}

	ind := 0
	total := len(files)
	fileContents := make(map[ExtendedFileInfo]map[string]*switchfs.ContentMetaAttributes, len(files))
	for _, file := range files {
		ind += 1
		if progress != nil {
			progress.UpdateProgress(ind, total, "process:"+file.FileName)
		}

		//scan sub-folders if flag is present
		filePath := filepath.Join(file.BaseFolder, file.FileName)
		if file.IsDir {
			continue
		}

		if runtime.GOOS == "darwin" && strings.EqualFold(file.FileName, ".ds_store") {
			continue
		}

		fileName := strings.ToLower(file.FileName)
		isSplit := false

		if len(fileName) >= 2 {
			partNum, err := strconv.Atoi(fileName[len(fileName)-2:])
			if err == nil {
				if partNum == 0 {
					isSplit = true
				} else {
					continue
				}
			}
		}

		// only handle XCI/XCZ and NSP/NSZ files
		fileExtension := filepath.Ext(fileName)
		if !isSplit && fileExtension != ".xci" && fileExtension != ".xcz" && fileExtension != ".nsp" && fileExtension != ".nsz" {
			_, explicitlyIgnored := ignoreFileTypes[fileExtension]
			if !explicitlyIgnored && !settingsObj.Scan.IgnoreUnsupportedFileTypes {
				skipped[file] = SkippedFile{ReasonCode: REASON_UNSUPPORTED_TYPE, ReasonText: "file type is not supported"}
			}
			continue
		}

		contentMap, err := ldb.getGameMetadata(file, filePath, skipped)

		if err != nil {
			if _, ok := skipped[file]; !ok {
				skipped[file] = SkippedFile{ReasonText: "unable to determine title-Id / version - " + err.Error(), ReasonCode: REASON_UNRECOGNISED}
			}
			continue
		}
		if diagnostic, exists := skipped[file]; !exists || diagnostic.ReasonCode != REASON_MALFORMED_FILE {
			fileContents[file] = contentMap
		}

		// A file containing a base title and one of its updates is a package,
		// not an independently removable update file. Record this per title
		// before processing the map because its iteration order is unspecified.
		// getGameMetadata returns non-nil records with lowercase title IDs.
		fileBasePrefixes := map[string]struct{}{}
		for _, contentMetadata := range contentMap {
			if !strings.HasSuffix(contentMetadata.TitleId, "000") {
				continue
			}
			prefix, err := titleIDPrefix(contentMetadata.TitleId)
			if err == nil {
				fileBasePrefixes[prefix] = struct{}{}
			}
		}

		for _, metadata := range contentMap {

			idPrefix, prefixErr := titleIDPrefix(metadata.TitleId)
			if prefixErr != nil {
				skipped[file] = SkippedFile{ReasonText: "unable to determine title-Id / version - " + prefixErr.Error(), ReasonCode: REASON_UNRECOGNISED}
				continue
			}

			multiContent := len(contentMap) > 1
			switchTitle := &SwitchGameFiles{
				MultiContent: multiContent,
				Updates:      map[int]SwitchFileInfo{},
				Dlc:          map[string]SwitchFileInfo{},
				BaseExist:    false,
				IsSplit:      isSplit,
				LatestUpdate: 0,
			}
			if t, ok := titles[idPrefix]; ok {
				switchTitle = t
			}
			switchTitle.MultiContent = switchTitle.MultiContent || multiContent
			titles[idPrefix] = switchTitle

			//process Updates
			if strings.HasSuffix(metadata.TitleId, "800") {
				metadata.Type = "Update"

				_, currentFileHasBase := fileBasePrefixes[idPrefix]
				currentIsPackaged := currentFileHasBase || (switchTitle.BaseExist && sameExtendedFilePath(file, switchTitle.File.ExtendedInfo))
				if update, ok := switchTitle.Updates[metadata.Version]; ok {
					reason := "duplicate update file\n" + filepath.Join(update.ExtendedInfo.BaseFolder, update.ExtendedInfo.FileName) + "\n" + filepath.Join(file.BaseFolder, file.FileName)
					existingIsPackaged := switchTitle.BaseExist && sameExtendedFilePath(update.ExtendedInfo, switchTitle.File.ExtendedInfo)
					if currentIsPackaged && !existingIsPackaged {
						// Prefer the package record for naming its base file. The
						// standalone duplicate is the removable file.
						skipped[update.ExtendedInfo] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: reason}
						switchTitle.Updates[metadata.Version] = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
					} else {
						// A bundled update is never a removable duplicate update on its
						// own. A duplicate package may still be reported as a duplicate
						// file when the existing update is also bundled.
						if !currentIsPackaged || existingIsPackaged {
							skipped[file] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: reason}
						}
					}
					zap.S().Warnf("-->Duplicate update file found [%v] and [%v]", update.ExtendedInfo.FileName, file.FileName)
					continue
				}
				switchTitle.Updates[metadata.Version] = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
				if metadata.Version > switchTitle.LatestUpdate {
					if switchTitle.LatestUpdate != 0 {
						oldUpdate := switchTitle.Updates[switchTitle.LatestUpdate]
						if !(switchTitle.BaseExist && sameExtendedFilePath(oldUpdate.ExtendedInfo, switchTitle.File.ExtendedInfo)) {
							skipped[oldUpdate.ExtendedInfo] = SkippedFile{ReasonCode: REASON_OLD_UPDATE, ReasonText: "old update file, newer update exist locally\nnew: " + filepath.Join(file.BaseFolder, file.FileName) + "\nold:" + filepath.Join(oldUpdate.ExtendedInfo.BaseFolder, oldUpdate.ExtendedInfo.FileName)}
						}
					}
					switchTitle.LatestUpdate = metadata.Version
				} else {
					if !currentIsPackaged {
						newerUpdate := switchTitle.Updates[switchTitle.LatestUpdate]
						skipped[file] = SkippedFile{ReasonCode: REASON_OLD_UPDATE, ReasonText: "old update file, newer update exist locally\nnew: " + filepath.Join(newerUpdate.ExtendedInfo.BaseFolder, newerUpdate.ExtendedInfo.FileName) + "\nold:" + filepath.Join(file.BaseFolder, file.FileName)}
					}
				}
				continue
			}

			//process base
			if strings.HasSuffix(metadata.TitleId, "000") {
				metadata.Type = "Base"
				if switchTitle.BaseExist {
					skipped[file] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: "duplicate base file\n" + filepath.Join(switchTitle.File.ExtendedInfo.BaseFolder, switchTitle.File.ExtendedInfo.FileName) + "\n" + filepath.Join(file.BaseFolder, file.FileName)}
					zap.S().Warnf("-->Duplicate base file found [%v] and [%v]", file.FileName, switchTitle.File.ExtendedInfo.FileName)
					continue
				}
				switchTitle.File = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
				switchTitle.BaseExist = true

				continue
			}

			if dlc, ok := switchTitle.Dlc[metadata.TitleId]; ok {
				if metadata.Version < dlc.Metadata.Version {
					skipped[file] = SkippedFile{ReasonCode: REASON_OLD_UPDATE, ReasonText: "old DLC file, newer version exist locally\nnew: " + filepath.Join(dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName) + "\nold:" + filepath.Join(file.BaseFolder, file.FileName)}
					zap.S().Warnf("-->Old DLC file found [%v] and [%v]", file.FileName, dlc.ExtendedInfo.FileName)
					continue
				} else if metadata.Version == dlc.Metadata.Version {
					skipped[file] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: "duplicate DLC file\n" + filepath.Join(dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName) + "\n" + filepath.Join(file.BaseFolder, file.FileName)}
					zap.S().Warnf("-->Duplicate DLC file found [%v] and [%v]", file.FileName, dlc.ExtendedInfo.FileName)
					continue
				}
			}
			//not an update, and not main TitleAttributes, so treat it as a DLC
			metadata.Type = "DLC"
			switchTitle.Dlc[metadata.TitleId] = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
		}
	}
	return fileContents
}

func buildCleanupCandidates(
	files []ExtendedFileInfo,
	fileContents map[ExtendedFileInfo]map[string]*switchfs.ContentMetaAttributes,
	titles map[string]*SwitchGameFiles,
	skipped map[ExtendedFileInfo]SkippedFile,
) []CleanupCandidate {
	candidates := make([]CleanupCandidate, 0)
	for _, file := range files {
		reason, ok := skipped[file]
		if !ok || (reason.ReasonCode != REASON_DUPLICATE && reason.ReasonCode != REASON_OLD_UPDATE) || file.IsDir {
			continue
		}
		// A split container is made up of multiple physical parts. Removing
		// only its first part would leave the container unusable.
		if isSplitFilePart(file.FileName) {
			continue
		}
		contentMap := fileContents[file]
		if len(contentMap) != 1 {
			continue
		}
		var metadata *switchfs.ContentMetaAttributes
		for _, record := range contentMap {
			metadata = record
		}
		if metadata == nil {
			continue
		}
		prefix, err := titleIDPrefix(metadata.TitleId)
		if err != nil || titles[prefix] == nil {
			continue
		}
		if replacement, ok := cleanupReplacement(titles[prefix], metadata, file); ok && !mayBeSamePhysicalFile(file, replacement) {
			candidates = append(candidates, CleanupCandidate{File: file, Replacement: replacement, ReasonCode: reason.ReasonCode})
		}
	}
	return candidates
}

// mayBeSamePhysicalFile reports whether removing file could also remove its
// replacement. Distinct scan paths can still name one physical file through
// aliases that path canonicalization cannot see, such as subst or mapped
// drives, UNC paths, bind mounts, or hard links. os.SameFile can report false
// matches on volumes without file IDs, which only withholds a cleanup offer, so
// any uncertainty is treated as the same file.
func mayBeSamePhysicalFile(file, replacement ExtendedFileInfo) bool {
	fileInfo, err := os.Stat(filepath.Join(file.BaseFolder, file.FileName))
	if err != nil {
		zap.S().Warnf("unable to verify cleanup candidate %q: %v", file.FileName, err)
		return true
	}
	replacementInfo, err := os.Stat(filepath.Join(replacement.BaseFolder, replacement.FileName))
	if err != nil {
		zap.S().Warnf("unable to verify cleanup replacement %q: %v", replacement.FileName, err)
		return true
	}
	return os.SameFile(fileInfo, replacementInfo)
}

func isSplitFilePart(fileName string) bool {
	if len(fileName) < 2 {
		return false
	}
	part, err := strconv.Atoi(fileName[len(fileName)-2:])
	return err == nil && part == 0
}

func cleanupReplacement(game *SwitchGameFiles, metadata *switchfs.ContentMetaAttributes, candidate ExtendedFileInfo) (ExtendedFileInfo, bool) {
	id := strings.ToLower(metadata.TitleId)
	if strings.HasSuffix(id, "000") {
		keeper := game.File
		return keeper.ExtendedInfo, game.BaseExist && keeper.Metadata != nil && strings.EqualFold(keeper.Metadata.TitleId, id) && !sameExtendedFilePath(keeper.ExtendedInfo, candidate)
	}
	if strings.HasSuffix(id, "800") {
		var keeper SwitchFileInfo
		found := false
		for _, update := range game.Updates {
			if update.Metadata == nil || !strings.EqualFold(update.Metadata.TitleId, id) || update.Metadata.Version < metadata.Version || sameExtendedFilePath(update.ExtendedInfo, candidate) {
				continue
			}
			if !found || update.Metadata.Version > keeper.Metadata.Version {
				keeper, found = update, true
			}
		}
		return keeper.ExtendedInfo, found
	}
	keeper, found := game.Dlc[id]
	if !found || keeper.Metadata == nil || !strings.EqualFold(keeper.Metadata.TitleId, id) || keeper.Metadata.Version < metadata.Version || sameExtendedFilePath(keeper.ExtendedInfo, candidate) {
		return ExtendedFileInfo{}, false
	}
	return keeper.ExtendedInfo, true
}

func (ldb *LocalSwitchDBManager) getGameMetadata(file ExtendedFileInfo,
	filePath string,
	skipped map[ExtendedFileInfo]SkippedFile) (map[string]*switchfs.ContentMetaAttributes, error) {

	var metadata map[string]*switchfs.ContentMetaAttributes = nil
	keys, _ := settings.SwitchKeys()
	var err error
	fileKey := filePath + "|" + file.FileName + "|" + strconv.Itoa(int(file.Size))
	if keys != nil && keys.GetKey("header_key") != "" {
		err = ldb.db.GetEntry(DB_TABLE_FILE_SCAN_METADATA, fileKey, &metadata)

		if err != nil {
			zap.S().Warnf("%v", err)
		}

		if cached, dropped := validContentMetadata(metadata); len(cached) > 0 && dropped == 0 {
			return cached, nil
		}
		if len(metadata) > 0 {
			// Remove the untrusted entry so a file that still yields no valid
			// record does not keep it; a valid re-read replaces it below.
			if err := ldb.db.DeleteEntry(DB_TABLE_FILE_SCAN_METADATA, fileKey); err != nil {
				zap.S().Warnf("[file:%v] failed to remove invalid cached metadata: %v", file.FileName, err)
			}
		}
		metadata = nil

		fileName := strings.ToLower(file.FileName)
		if strings.HasSuffix(fileName, "nsp") ||
			strings.HasSuffix(fileName, "nsz") {
			metadata, err = switchfs.ReadNspMetadata(filePath)
			if err != nil {
				skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: fmt.Sprintf("failed to read NSP [reason: %v]", err)}
				zap.S().Errorf("[file:%v] failed to read NSP [reason: %v]\n", file.FileName, err)
			}
		} else if strings.HasSuffix(fileName, "xci") ||
			strings.HasSuffix(fileName, "xcz") {
			metadata, err = switchfs.ReadXciMetadata(filePath)
			if err != nil {
				skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: fmt.Sprintf("failed to read NSP [reason: %v]", err)}
				zap.S().Errorf("[file:%v] failed to read file [reason: %v]\n", file.FileName, err)
			}
		} else if strings.HasSuffix(fileName, "00") {
			metadata, err = fileio.ReadSplitFileMetadata(filePath)
			if err != nil {
				skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: fmt.Sprintf("failed to read split files [reason: %v]", err)}
				zap.S().Errorf("[file:%v] failed to read NSP [reason: %v]\n", file.FileName, err)
			}
		}
	}

	if valid := ldb.acceptReadMetadata(file, fileKey, metadata, skipped); len(valid) > 0 {
		return valid, nil
	}

	//fallback to parse data from filename

	//parse title id
	titleId, _ := parseTitleIdFromFileName(file.FileName)
	version, _ := parseVersionFromFileName(file.FileName)

	if titleId == nil || version == nil {
		return nil, errors.New("unable to determine titileId / version")
	}
	metadata = map[string]*switchfs.ContentMetaAttributes{}
	metadata[*titleId] = &switchfs.ContentMetaAttributes{TitleId: *titleId, Version: *version}

	return metadata, nil
}

// acceptReadMetadata keeps the records read from a container that have a valid
// title ID and caches them. Records without one are logged rather than
// recorded in skipped, because the file's valid content is still in the
// library and must remain eligible for duplicate cleanup. When nothing valid
// remains, a malformed-file diagnostic is recorded unless reading the file
// already recorded a more specific one.
func (ldb *LocalSwitchDBManager) acceptReadMetadata(file ExtendedFileInfo, fileKey string, metadata map[string]*switchfs.ContentMetaAttributes, skipped map[ExtendedFileInfo]SkippedFile) map[string]*switchfs.ContentMetaAttributes {
	valid, dropped := validContentMetadata(metadata)
	if dropped > 0 {
		zap.S().Warnf("[file:%v] ignored %d content metadata record(s) without a valid title ID", file.FileName, dropped)
	}
	if len(valid) > 0 {
		if err := ldb.db.AddEntry(DB_TABLE_FILE_SCAN_METADATA, fileKey, valid); err != nil {
			zap.S().Warnf("[file:%v] failed to cache content metadata: %v", file.FileName, err)
		}
		return valid
	}
	if _, exists := skipped[file]; metadata != nil && !exists {
		reason := "no content metadata found in file; trying title ID and version from filename"
		if dropped > 0 {
			reason = fmt.Sprintf("no content metadata with a valid title ID found in file (ignored %d record(s)); trying title ID and version from filename", dropped)
		}
		skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: reason}
	}
	return nil
}

// validContentMetadata returns the records with a usable title ID, keyed by
// the normalized ID, and the number of nil or invalid records it dropped.
func validContentMetadata(metadata map[string]*switchfs.ContentMetaAttributes) (map[string]*switchfs.ContentMetaAttributes, int) {
	valid := make(map[string]*switchfs.ContentMetaAttributes, len(metadata))
	dropped := 0
	for _, record := range metadata {
		if record == nil {
			dropped++
			continue
		}
		id := strings.ToLower(record.TitleId)
		if _, err := titleIDPrefix(id); err != nil {
			dropped++
			continue
		}
		record.TitleId = id
		valid[id] = record
	}
	return valid, dropped
}

func parseVersionFromFileName(fileName string) (*int, error) {
	res := versionRegex.FindStringSubmatch(fileName)
	if len(res) != 2 {
		return nil, errors.New("failed to parse name - no version id found")
	}
	ver, err := strconv.Atoi(res[1])
	if err != nil {
		return nil, errors.New("failed to parse name - no version id found")
	}
	return &ver, nil
}

func parseTitleIdFromFileName(fileName string) (*string, error) {
	res := titleIdRegex.FindStringSubmatch(fileName)

	if len(res) != 2 {
		return nil, errors.New("failed to parse name - no title id found")
	}
	titleId := strings.ToLower(res[1])
	return &titleId, nil
}

func ParseTitleNameFromFileName(fileName string) string {
	ind := strings.Index(fileName, "[")
	if ind != -1 {
		fileName = fileName[:ind]
	}

	name := strings.TrimSpace(fileName)
	for {
		if match := switchFileExtensionSuffix.FindStringIndex(name); match != nil {
			name = strings.TrimSpace(name[:match[0]])
			continue
		}
		if match := packageContentsNameSuffix.FindStringIndex(name); match != nil {
			name = strings.TrimSpace(name[:match[0]])
			continue
		}
		if match := emptyVersionNameSuffix.FindStringIndex(name); match != nil && versionNameSuffix.MatchString(strings.TrimSpace(name[:match[0]])) {
			name = strings.TrimSpace(name[:match[0]])
			continue
		}
		if match := versionNameSuffix.FindStringIndex(name); match != nil {
			name = strings.TrimSpace(name[:match[0]])
			continue
		}
		return name
	}
}
