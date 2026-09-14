package main

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/asticode/go-astilectron"
)

func TestEmbeddedAstilectronArchive(t *testing.T) {
	data, err := Asset("vendor_astilectron_bundler/astilectron.zip")
	if err != nil {
		t.Fatalf("load embedded Astilectron archive: %v", err)
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("read embedded Astilectron archive: %v", err)
	}

	wantedPath := "astilectron-" + astilectron.DefaultVersionAstilectron + "/main.js"
	for _, file := range archive.File {
		if file.Name != wantedPath {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open embedded %s: %v", wantedPath, err)
		}
		_, readErr := io.Copy(io.Discard, reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatalf("read embedded %s: %v", wantedPath, readErr)
		}
		if closeErr != nil {
			t.Fatalf("close embedded %s: %v", wantedPath, closeErr)
		}
		return
	}

	t.Fatalf("embedded Astilectron archive does not contain %s", wantedPath)
}
