package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWindowStateMissingAndValid(t *testing.T) {
	base := t.TempDir()
	state, err := ReadWindowState(base)
	if err != nil || state != nil {
		t.Fatalf("missing window state = %#v, %v; want nil, nil", state, err)
	}

	want := &WindowState{Width: 1440, Height: 900, X: -1280, Y: 32, ScreenWidth: 1920, ScreenHeight: 1080}
	if err := SaveWindowState(want, base); err != nil {
		t.Fatal(err)
	}
	got, err := ReadWindowState(base)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != *want {
		t.Fatalf("window state = %#v, want %#v", got, want)
	}

	contents, err := os.ReadFile(filepath.Join(base, WINDOW_STATE_FILENAME))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `"width": 1440`) || !strings.Contains(string(contents), `"x": -1280`) || !strings.Contains(string(contents), `"screen_width": 1920`) {
		t.Fatalf("window state JSON missing saved fields: %s", contents)
	}
}

func TestReadWindowStateAcceptsLegacyStateWithoutScreenSize(t *testing.T) {
	base := t.TempDir()
	filename := filepath.Join(base, WINDOW_STATE_FILENAME)
	if err := os.WriteFile(filename, []byte(`{"width":1200,"height":600,"x":10,"y":20}`), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadWindowState(base)
	if err != nil {
		t.Fatal(err)
	}
	if want := (WindowState{Width: 1200, Height: 600, X: 10, Y: 20}); got == nil || *got != want {
		t.Fatalf("legacy window state = %#v, want %#v", got, want)
	}
}

func TestReadWindowStateRejectsMalformedAndInvalidData(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
	}{
		{name: "malformed JSON", data: `{"width":`},
		{name: "zero width", data: `{"width":0,"height":600,"x":0,"y":0}`},
		{name: "negative height", data: `{"width":1200,"height":-1,"x":0,"y":0}`},
		{name: "oversized width", data: `{"width":20000,"height":600,"x":0,"y":0}`},
		{name: "partial screen size", data: `{"width":1200,"height":600,"x":0,"y":0,"screen_width":1920}`},
		{name: "negative screen size", data: `{"width":1200,"height":600,"x":0,"y":0,"screen_width":-1,"screen_height":-1}`},
		{name: "oversized screen size", data: `{"width":1200,"height":600,"x":0,"y":0,"screen_width":20000,"screen_height":1080}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			filename := filepath.Join(base, WINDOW_STATE_FILENAME)
			if err := os.WriteFile(filename, []byte(test.data), 0644); err != nil {
				t.Fatal(err)
			}
			if state, err := ReadWindowState(base); err == nil || state != nil {
				t.Fatalf("ReadWindowState() = %#v, %v; want an error", state, err)
			}
		})
	}
}

func TestSaveWindowStateRejectsInvalidBounds(t *testing.T) {
	base := t.TempDir()
	for _, state := range []*WindowState{
		nil,
		{Width: 0, Height: 600},
		{Width: 1200, Height: 0},
		{Width: maxWindowDimension + 1, Height: 600},
	} {
		if err := SaveWindowState(state, base); err == nil {
			t.Fatalf("SaveWindowState(%#v) unexpectedly succeeded", state)
		}
	}
	if _, err := os.Stat(filepath.Join(base, WINDOW_STATE_FILENAME)); !os.IsNotExist(err) {
		t.Fatalf("invalid bounds created a state file, stat error = %v", err)
	}
}
