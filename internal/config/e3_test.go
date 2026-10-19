package config

import (
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
	if mv := MovePlatformConfigDir(); mv.From != "" {
		t.Errorf("no old directory: moved %+v", mv)
	}
}

// oldPlatformDir lays out the directory an older kbu kept its settings in
// (macOS ~/Library/Application Support/kbu, Windows %AppData%\kbu), with a
// config.yaml, and returns it and the family's directory.
func oldPlatformDir(t *testing.T) (old, family string) {
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
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "config.yaml"), []byte("editor: vi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return old, family
}

// tdp E3: when only the old directory is there, the first start moves the
// whole of it to the family's.
func TestE3_OldPlatformDirMoves(t *testing.T) {
	old, family := oldPlatformDir(t)
	if mv := MovePlatformConfigDir(); mv.From != old || mv.To != family || mv.Err != nil {
		t.Fatalf("MovePlatformConfigDir() = %+v, want from %q to %q", mv, old, family)
	}
	if exists(old) {
		t.Error("the old directory is still there")
	}
	if b, err := os.ReadFile(filepath.Join(family, "config.yaml")); err != nil || string(b) != "editor: vi\n" {
		t.Errorf("config.yaml did not move: %q, %v", b, err)
	}
	if got := ConfigDir(); got != family {
		t.Errorf("ConfigDir() = %q, want %q", got, family)
	}
	if mv := MovePlatformConfigDir(); mv.From != "" {
		t.Errorf("the second start moved again: %+v", mv)
	}
}

// tdp E3: when the move fails, kbu keeps using the old directory, and the
// move says why.
func TestE3_AFailedMoveKeepsTheOldDir(t *testing.T) {
	old, family := oldPlatformDir(t)
	// ~/.config is a file: the family's directory cannot be made.
	if err := os.WriteFile(filepath.Dir(family), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if mv := MovePlatformConfigDir(); mv.From != old || mv.Err == nil {
		t.Fatalf("MovePlatformConfigDir() = %+v, want a failed move from %q", mv, old)
	}
	if got := ConfigDir(); got != old {
		t.Errorf("ConfigDir() = %q, want the old %q", got, old)
	}
}

// When both are there the family's wins and nothing moves; $KBU__CONFIG
// moves nothing either.
func TestE3_NothingMovesOverAnExistingDir(t *testing.T) {
	old, family := oldPlatformDir(t)
	if err := os.MkdirAll(family, 0o755); err != nil {
		t.Fatal(err)
	}
	if mv := MovePlatformConfigDir(); mv.From != "" || !exists(old) {
		t.Errorf("both directories: moved %+v", mv)
	}
	if got := ConfigDir(); got != family {
		t.Errorf("both directories: ConfigDir() = %q, want %q", got, family)
	}
	if err := os.Remove(family); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "mine")
	t.Setenv("KBU__CONFIG", dir)
	if mv := MovePlatformConfigDir(); mv.From != "" || !exists(old) {
		t.Errorf("with KBU__CONFIG: moved %+v", mv)
	}
	if got := ConfigDir(); got != dir {
		t.Errorf("ConfigDir() = %q, want %q", got, dir)
	}
}
