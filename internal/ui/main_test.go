package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain redirects XDG_CONFIG_HOME to a per-process temp dir for
// every test in the ui package. This is a defensive guard: any test
// that calls cfg.Save() — directly or transitively via a method
// like commitSortFlow — would otherwise hit the real
// ~/.config/km8/config.yaml and overwrite the user's actual pins /
// sort / editor settings.
//
// This blanket setup is cheap (one mkdir per process) and prevents
// the whole class of "test accidentally clobbers prod config"
// bugs from recurring without each new test author having to
// remember to t.Setenv() themselves. Individual tests can still
// override XDG_CONFIG_HOME via t.Setenv if they want their own
// isolated dir.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "km8-ui-test-xdg-")
	if err != nil {
		// Refuse to run without isolation — silently falling back to
		// the real path is exactly the failure mode this guard
		// exists to prevent.
		panic("TestMain: cannot create temp XDG dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	os.Exit(m.Run())
}
