package db

import (
	bytes2 "bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"
)

type ProgressUpdater interface {
	UpdateProgress(curr int, total int, message string)
}

// LoadAndUpdateFile downloads or opens filePath and returns an open file.
// The caller owns the returned file and must close it.
func LoadAndUpdateFile(url string, filePath string, etag string) (*os.File, string, error) {
	// An ETag is only useful when there is a local file to use after a 304
	// response. In particular, the default ETag in cache.json must not be sent
	// on first run when filePath does not yet contain the downloaded data.
	localAvailable := false
	fileInfo, statErr := os.Stat(filePath)
	if statErr == nil {
		localAvailable = !fileInfo.IsDir() && fileInfo.Size() > 0
	} else if os.IsNotExist(statErr) {
		created, err := os.Create(filePath)
		if err != nil {
			zap.S().Errorf("Failed to create file %v - %v\n", filePath, err)
			return nil, "", fmt.Errorf("create local data file %q: %w", filePath, err)
		}
		if err := created.Close(); err != nil {
			return nil, "", fmt.Errorf("close new local data file %q: %w", filePath, err)
		}
	} else {
		return nil, "", fmt.Errorf("check local data file %q: %w", filePath, statErr)
	}
	if !localAvailable {
		etag = ""
	}

	var file *os.File = nil

	//try to check if there is a new version
	//if so, save the file
	bytes, newEtag, err := downloadBytesFromUrl(url, etag)
	if err == nil {
		//validate json structure
		var test map[string]interface{}
		err = decodeToJsonObject(bytes2.NewReader(bytes), &test)
		if err == nil {
			file, err = saveFile(bytes, filePath)
			etag = newEtag
		} else {
			zap.S().Infof("ignoring new update [%v], reason - [malformed json file]", url)
		}
	} else {
		zap.S().Infof("file [%v] was not downloaded, reason - [%v]", url, err)
	}

	if file == nil {
		if !localAvailable {
			if err != nil {
				return nil, "", fmt.Errorf("download failed and no local data file is available: %w", err)
			}
			return nil, "", errors.New("download failed and no local data file is available")
		}

		//load file
		file, err = os.Open(filePath)
		if err != nil {
			zap.S().Infof("ignoring new update [%v], reason - [malformed json file]", url)
			return nil, "", err
		}

		fileInfo, err := os.Stat(filePath)
		if err != nil || fileInfo.IsDir() || fileInfo.Size() == 0 {
			file.Close()
			zap.S().Infof("Local file is empty, a directory, or corrupted")
			if err != nil {
				return nil, "", fmt.Errorf("stat local data file %q: %w", filePath, err)
			}
			return nil, "", errors.New("local data file is empty or is a directory")
		}
	}

	return file, etag, nil
}

func decodeToJsonObject(reader io.Reader, target interface{}) error {
	err := json.NewDecoder(reader).Decode(target)
	return err
}

func downloadBytesFromUrl(url string, etag string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("If-None-Match", etag)
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).DialContext,
	}
	client := http.Client{
		Transport: transport,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, "", errors.New("got a non 200 response - " + resp.Status)
	}
	//getting the new etag
	etag = resp.Header.Get("Etag")

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, "", err
		}
		return body, etag, nil
	}

	return nil, "", errors.New("no new updates")
}

func saveFile(bytes []byte, fileName string) (*os.File, error) {

	err := ioutil.WriteFile(fileName, bytes, 0644)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	return file, nil
}
