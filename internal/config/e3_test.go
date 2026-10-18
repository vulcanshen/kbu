package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	if LegacyConfigNotice() != "" {
		t.Errorf("no old directory: LegacyConfigNotice() = %q, want empty", LegacyConfigNotice())
	}
}

// An older kbu kept its settings in the platform's directory (macOS
// ~/Library/Application Support/kbu, Windows %AppData%\kbu). While only
// that one is there, kbu keeps using it, and says so in the App Log; once
// the family's directory exists, that one wins.
func TestE3_OldPlatformDirIsStillRead(t *testing.T) {
	home := isolateHome(t)
	platform, err := os.UserConfigDir()
	if err != nil {
		t.Skip("no platform settings directory here:", err)
	}
	old, family := filepath.Join(platform, "kbu"), filepath.Join(home, ".config", "kbu")
	if old == family {
		t.Skipf("on %s the platform's directory is the family's", runtime.GOOS)
	}
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := ConfigDir(); got != old {
		t.Errorf("only the old directory: ConfigDir() = %q, want %q", got, old)
	}
	if n := LegacyConfigNotice(); !strings.Contains(n, old) || !strings.Contains(n, family) {
		t.Errorf("LegacyConfigNotice() = %q, want it to name %q and %q", n, old, family)
	}
	if err := os.MkdirAll(family, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := ConfigDir(); got != family {
		t.Errorf("both directories: ConfigDir() = %q, want %q", got, family)
	}
	if n := LegacyConfigNotice(); n != "" {
		t.Errorf("both directories: LegacyConfigNotice() = %q, want empty", n)
	}
}

// $KBU__CONFIG still moves everything, old directory or not.
func TestE3_KBUConfigWinsOverTheOldDir(t *testing.T) {
	isolateHome(t)
	if platform, err := os.UserConfigDir(); err == nil {
		_ = os.MkdirAll(filepath.Join(platform, "kbu"), 0o755)
	}
	dir := filepath.Join(t.TempDir(), "mine")
	t.Setenv("KBU__CONFIG", dir)
	if got := ConfigDir(); got != dir {
		t.Errorf("ConfigDir() = %q, want %q", got, dir)
	}
	if n := LegacyConfigNotice(); n != "" {
		t.Errorf("LegacyConfigNotice() = %q, want empty", n)
	}
}
