package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// isolateHome points $HOME (and Windows' %USERPROFILE% / %AppData%) at a
// fresh directory and clears the variables that would move the settings,
// so ConfigDir sees only what the test lays out.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("KBU__CONFIG", "")
	t.Setenv("KBU__DATA", "")
	return home
}

// tdp E3: the settings are in $XDG_CONFIG_HOME/kbu, else ~/.config/kbu, on
// every platform — not the platform's own settings directory.
func TestE3_ConfigDirIsTheFamilys(t *testing.T) {
	home := isolateHome(t)
	if got, want := ConfigDir(), filepath.Join(home, ".config", "kbu"); got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}
	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, want := ConfigDir(), filepath.Join(xdg, "kbu"); got != want {
		t.Errorf("with XDG_CONFIG_HOME: ConfigDir() = %q, want %q", got, want)
	}
	if mv := MoveOldPlaces(); len(mv) != 0 {
		t.Errorf("no old directory: moved %+v", mv)
	}
}

// oldPlatformDir lays out the directory an older kbu kept its settings in
// (macOS ~/Library/Application Support/kbu, Windows %AppData%\kbu): a
// config.yaml, a state.yaml and a log. It returns it, the family's settings
// directory and the data one's logs.
func oldPlatformDir(t *testing.T) (old, family, logs string) {
	t.Helper()
	home := isolateHome(t)
	platform, err := os.UserConfigDir()
	if err != nil {
		t.Skip("no platform settings directory here:", err)
	}
	old, family = filepath.Join(platform, "kbu"), filepath.Join(home, ".config", "kbu")
	if old == family {
		t.Skipf("on %s the platform's directory is the family's", runtime.GOOS)
	}
	write(t, filepath.Join(old, "config.yaml"), "editor: vi\n")
	write(t, filepath.Join(old, "state.yaml"), "kind: pod\n")
	write(t, filepath.Join(old, "logs", "audit.log"), "edit\n")
	return old, family, filepath.Join(home, ".kbu", "logs")
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

// tdp E3: the first start moves what the old directory has, item by item —
// the settings to the family's directory, the logs to the data one — and
// takes the old directory away once it is empty.
func TestE3_OldPlacesMoveItemByItem(t *testing.T) {
	old, family, logs := oldPlatformDir(t)
	for _, mv := range MoveOldPlaces() {
		if mv.Err != nil {
			t.Errorf("%+v", mv)
		}
	}
	if exists(old) {
		t.Error("the old directory is still there")
	}
	if read(filepath.Join(family, "config.yaml")) != "editor: vi\n" || read(filepath.Join(family, "state.yaml")) != "kind: pod\n" {
		t.Error("the settings did not move")
	}
	if read(filepath.Join(logs, "audit.log")) != "edit\n" {
		t.Error("the log did not move to the data directory")
	}
	if got := ConfigDir(); got != family {
		t.Errorf("ConfigDir() = %q, want %q", got, family)
	}
	if mv := MoveOldPlaces(); len(mv) != 0 {
		t.Errorf("the second start moved again: %+v", mv)
	}
}

// tdp E3: what the new place has already stays where it was, for the user
// to merge; the rest moves.
func TestE3_WhatIsThereAlreadyStays(t *testing.T) {
	old, family, _ := oldPlatformDir(t)
	write(t, filepath.Join(family, "config.yaml"), "editor: nano\n")
	kept := false
	for _, mv := range MoveOldPlaces() {
		if mv.From == filepath.Join(old, "config.yaml") {
			kept = errors.Is(mv.Err, ErrThereAlready)
		} else if mv.Err != nil {
			t.Errorf("%+v", mv)
		}
	}
	if !kept {
		t.Error("config.yaml was not left where it was")
	}
	if read(filepath.Join(old, "config.yaml")) != "editor: vi\n" || read(filepath.Join(family, "config.yaml")) != "editor: nano\n" {
		t.Error("a config.yaml was overwritten or lost")
	}
	if read(filepath.Join(family, "state.yaml")) != "kind: pod\n" {
		t.Error("state.yaml did not move")
	}
}

// tdp E3: when the settings cannot be moved, none is, and kbu keeps using
// the old directory; the move says why.
func TestE3_AFailedMoveKeepsTheOldDir(t *testing.T) {
	old, family, _ := oldPlatformDir(t)
	// ~/.config is a file: the family's directory cannot be made.
	if err := os.WriteFile(filepath.Dir(family), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	failed := false
	for _, mv := range MoveOldPlaces() {
		if mv.What == "settings" {
			failed = mv.From == old && mv.Err != nil
		}
	}
	if !failed {
		t.Error("no failed move of the settings")
	}
	if got := ConfigDir(); got != old || read(filepath.Join(old, "config.yaml")) != "editor: vi\n" {
		t.Errorf("ConfigDir() = %q, want the old %q with its settings", got, old)
	}
}

// $KBU__CONFIG moves no settings.
func TestE3_KBUConfigMovesNoSettings(t *testing.T) {
	old, _, _ := oldPlatformDir(t)
	dir := filepath.Join(t.TempDir(), "mine")
	t.Setenv("KBU__CONFIG", dir)
	for _, mv := range MoveOldPlaces() {
		if mv.What == "settings" {
			t.Errorf("with KBU__CONFIG: moved %+v", mv)
		}
	}
	if read(filepath.Join(old, "config.yaml")) != "editor: vi\n" {
		t.Error("with KBU__CONFIG the old settings went")
	}
	if got := ConfigDir(); got != dir {
		t.Errorf("ConfigDir() = %q, want %q", got, dir)
	}
}

// The logs an older kbu wrote in the settings directory (on Linux too) move
// to the data one: tdp E3 says logs are data.
func TestE3_OldLogsMoveToTheData(t *testing.T) {
	home := isolateHome(t)
	write(t, filepath.Join(ConfigDir(), "logs", "crash.log"), "boom\n")
	write(t, filepath.Join(ConfigDir(), "config.yaml"), "editor: vi\n")
	MoveOldPlaces()
	if read(filepath.Join(home, ".kbu", "logs", "crash.log")) != "boom\n" {
		t.Error("the old log did not move to the data directory")
	}
	if read(filepath.Join(ConfigDir(), "config.yaml")) != "editor: vi\n" {
		t.Error("the settings moved too")
	}
}
