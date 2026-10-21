package config

import (
	"path/filepath"
	"slices"
	"testing"
)

// tdp E3, files no longer used: theme.yaml where the settings are, and once
// kbu's directory is there, km8's.
func TestNoLongerUsed(t *testing.T) {
	cfg, xdg := t.TempDir(), t.TempDir()
	t.Setenv("KBU__CONFIG", cfg)
	t.Setenv("XDG_CONFIG_HOME", xdg) // km8's under it, the platform's none
	want := []string{filepath.Join(cfg, "theme.yaml"), filepath.Join(xdg, "km8")}
	if got := NoLongerUsed(); !slices.Equal(got, want) {
		t.Errorf("NoLongerUsed() = %v, want %v", got, want)
	}
	// No settings directory yet: km8's is still to be copied, it is used.
	t.Setenv("KBU__CONFIG", filepath.Join(cfg, "not-yet"))
	if got := NoLongerUsed(); !slices.Equal(got, []string{filepath.Join(cfg, "not-yet", "theme.yaml")}) {
		t.Errorf("before the copy: %v", got)
	}
}
