package ui

import (
	"path/filepath"
	"testing"

	"github.com/vulcanshen/kbu/internal/config"
)

// L in the Compare popup writes the layout it switched to into
// config.yaml (compare.layout), so the next start opens with it.
func TestCompare_LayoutSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("KBU__CONFIG", dir) // never the user's real config

	m := stackTestApp(t)
	m.comparePopup.SetDefaultLayout(CompareLayoutUnified)
	_ = m.comparePopup.Open("a: 1\n", "a: 2\n", "l", "r")
	m.comparePopup.animator.Finalize()

	pressL := func() {
		t.Helper()
		updated, cmd := m.Update(key("L"))
		m = updated.(AppModel)
		for _, msg := range drainCmd(cmd) {
			next, _ := m.Update(msg)
			m = next.(AppModel)
		}
	}
	saved := func() string {
		t.Helper()
		cfg, err := config.LoadFrom(path)
		if err != nil {
			t.Fatalf("reading the saved config: %v", err)
		}
		return cfg.Compare.Layout
	}

	pressL()
	if got := saved(); got != "split" {
		t.Fatalf("after L the saved compare.layout is %q, want split", got)
	}
	if parseCompareLayout(saved()) != CompareLayoutSplit {
		t.Error("a restart must open Compare side by side")
	}
	pressL()
	if got := saved(); got != "unified" {
		t.Errorf("after a second L the saved compare.layout is %q, want unified", got)
	}
}
