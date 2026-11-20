package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
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

// tdp E3: a key of config.yaml kbu does not read is kept, also when kbu
// writes the file, and is an INFO at every start: "No longer used" for one
// an older kbu read, "Not read" for a typo. The old Alterm keys are read
// and dropped as before.
func TestE3_KeysNotRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	write(t, path, "editor: vi\nrefesh: 5\nmouse_opt_config:\n  enabled: false\ncompare:\n  layout: split\nkm8erm_shell: zsh\n")
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"No longer used: compare in " + path + "; delete it if you don't need it",
		"No longer used: mouse_opt_config in " + path + "; delete it if you don't need it",
		"Not read: refesh in " + path + "; delete it if you don't need it",
	}
	if got := cfg.NotRead(path); !slices.Equal(got, want) {
		t.Errorf("NotRead = %q, want %q", got, want)
	}
	if cfg.AltermShell != "zsh" {
		t.Errorf("km8erm_shell was not read: %q", cfg.AltermShell)
	}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	saved := read(path)
	for _, key := range []string{"refesh: 5", "mouse_opt_config:", "compare:", "editor: vi"} {
		if !strings.Contains(saved, key) {
			t.Errorf("writing the file lost %q:\n%s", key, saved)
		}
	}
	if strings.Contains(saved, "km8erm_shell") {
		t.Errorf("writing the file kept km8erm_shell:\n%s", saved)
	}
}

// tdp E3: a value kbu cannot use goes back to its default, and says what it
// was: a negative input_tab_width, a panel that is none of the three, a sort
// in neither direction (taken out).
func TestE3_Misfits(t *testing.T) {
	cfg := &Config{InputTabWidth: -2, Editor: "vi"}
	if got := cfg.Misfits(); len(got) != 1 || got[0] != (terminu.Misfit{Key: "input_tab_width", Was: "-2", Now: "4"}) || cfg.InputTabWidth != 0 {
		t.Errorf("config misfits %+v, input_tab_width now %d", got, cfg.InputTabWidth)
	}
	if got := (&Config{InputTabWidth: 2}).Misfits(); len(got) != 0 {
		t.Errorf("input_tab_width 2 did not fit: %+v", got)
	}
	s := &State{Panel: "nowhere", Kind: "pods"}
	if got := s.Misfits(); len(got) != 1 || got[0] != (terminu.Misfit{Key: "panel", Was: "nowhere", Now: "sidebar"}) || s.Panel != "" {
		t.Errorf("state misfits %+v, panel now %q", got, s.Panel)
	}
	k := KindData{"pod": {Sort: SortChain{{Column: "Age", Direction: "sideways"}, {Column: "Name", Direction: "asc"}}},
		"node": {Sort: SortChain{{Column: "Age", Direction: ""}}}}
	got := k.Misfits()
	want := []terminu.Misfit{{Key: `node sort by Age (direction "")`}, {Key: `pod sort by Age (direction "sideways")`}}
	if !slices.Equal(got, want) {
		t.Errorf("kinds misfits %+v, want %+v", got, want)
	}
	if _, ok := k["node"]; ok || len(k["pod"].Sort) != 1 || k["pod"].Sort[0].Column != "Name" {
		t.Errorf("the misfits are still in the pins and sorts: %+v", k)
	}
}

// tdp E3: a km8 user's first start of kbu moves km8's settings to kbu's
// directory item by item, its logs to the data, and takes the empty km8
// directory away.
func TestE3_Km8MovesItemByItem(t *testing.T) {
	home := isolateHome(t)
	km8 := legacyConfigDir()
	write(t, filepath.Join(km8, "config.yaml"), "editor: vi\n")
	write(t, filepath.Join(km8, "state.yaml"), "kind: pods\n")
	write(t, filepath.Join(km8, "logs", "crash.log"), "boom\n")
	moved := MoveOldPlaces()
	kbu := filepath.Join(home, ".config", "kbu")
	if ConfigDir() != kbu || read(filepath.Join(kbu, "config.yaml")) != "editor: vi\n" || read(filepath.Join(kbu, "state.yaml")) != "kind: pods\n" {
		t.Errorf("km8's settings did not move to %s: %+v", kbu, moved)
	}
	if read(filepath.Join(home, ".kbu", "logs", "crash.log")) != "boom\n" {
		t.Errorf("km8's logs did not move to the data: %+v", moved)
	}
	if exists(km8) {
		t.Error("the empty km8 directory is still there")
	}
	for _, mv := range moved {
		if mv.Err != nil {
			t.Errorf("a move failed: %+v", mv)
		}
	}
}

// Where kbu has its own settings already (an older kbu copied km8's), km8's
// directory stays, a directory no longer used, and nothing is asked.
func TestE3_Km8StaysWhenKbuHasItsOwn(t *testing.T) {
	home := isolateHome(t)
	km8 := legacyConfigDir()
	write(t, filepath.Join(km8, "config.yaml"), "editor: vi\n")
	write(t, filepath.Join(home, ".config", "kbu", "config.yaml"), "editor: nano\n")
	if moved := MoveOldPlaces(); len(moved) != 0 {
		t.Errorf("moved %+v", moved)
	}
	if read(filepath.Join(km8, "config.yaml")) != "editor: vi\n" || !slices.Contains(NoLongerUsed(), km8) {
		t.Errorf("km8's directory is not left as no longer used: %q", NoLongerUsed())
	}
}

// When km8's settings cannot be moved, none is, and kbu keeps using km8's
// directory; the start says so with both paths.
func TestE3_Km8NotMovedKeepsIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a folder Windows does not keep from being written to")
	}
	home := isolateHome(t)
	km8 := legacyConfigDir()
	write(t, filepath.Join(km8, "config.yaml"), "editor: vi\n")
	parent := filepath.Join(home, ".config")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0o755) })
	if km8 == filepath.Join(parent, "km8") {
		t.Skip("km8's directory is under the folder made read-only here")
	}
	moved := MoveOldPlaces()
	if len(moved) != 1 || moved[0].Err == nil || moved[0].From != km8 {
		t.Errorf("moved %+v, want km8's directory not moved", moved)
	}
	if ConfigDir() != km8 || read(filepath.Join(km8, "config.yaml")) != "editor: vi\n" {
		t.Errorf("ConfigDir() = %q, want km8's %q", ConfigDir(), km8)
	}
}
