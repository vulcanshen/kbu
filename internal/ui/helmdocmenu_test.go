package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func newHelmDocMenu(t *testing.T) HelmDocMenuPopupModel {
	t.Helper()
	th, err := theme.LoadTheme("")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}
	m := NewHelmDocMenuPopupModel(th)
	m.SetSize(120, 40)
	return m
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// drainAnimationToInteractive forces the popup past its open animation so
// Update will start accepting key input. Without this, the first Update
// drops keystrokes because IsInteractive() is still false.
func drainAnimationToInteractive(t *testing.T, m *HelmDocMenuPopupModel, openCmd tea.Cmd) {
	t.Helper()
	// Replay the open command's emitted ticks until the animator settles.
	if openCmd == nil {
		return
	}
	for i := 0; i < 50; i++ {
		if m.IsInteractive() {
			return
		}
		msg := openCmd()
		if tick, ok := msg.(AnimTickMsg); ok {
			openCmd = m.HandleTick(tick)
			continue
		}
		break
	}
	if !m.IsInteractive() {
		t.Fatalf("popup never became interactive after 50 ticks")
	}
}

func TestHelmDocMenu_OpenSetsTarget(t *testing.T) {
	m := newHelmDocMenu(t)
	cmd := m.Open("nginx", "default")
	if !m.IsActive() {
		t.Fatal("popup should be Active after Open")
	}
	if m.releaseName != "nginx" || m.releaseNS != "default" {
		t.Errorf("target snapshot wrong: name=%q ns=%q", m.releaseName, m.releaseNS)
	}
	if cmd == nil {
		t.Error("Open returned nil cmd — animator should fire one")
	}
}

func TestHelmDocMenu_Items_MatchHelmDocConsts(t *testing.T) {
	if len(helmDocMenuItems) != 5 {
		t.Fatalf("expected 5 menu items, got %d", len(helmDocMenuItems))
	}
	wants := []string{
		k8s.HelmDocManifest,
		k8s.HelmDocNotes,
		k8s.HelmDocUserValues,
		k8s.HelmDocMergedValues,
		k8s.HelmDocHooks,
	}
	for i, w := range wants {
		if helmDocMenuItems[i].docKind != w {
			t.Errorf("item[%d].docKind = %q, want %q", i, helmDocMenuItems[i].docKind, w)
		}
		if helmDocMenuItems[i].label == "" {
			t.Errorf("item[%d].label is empty", i)
		}
	}
}

func TestHelmDocMenu_CursorJK(t *testing.T) {
	m := newHelmDocMenu(t)
	cmd := m.Open("nginx", "default")
	drainAnimationToInteractive(t, &m, cmd)

	m, _ = m.Update(key("j"))
	if m.cursor != 1 {
		t.Errorf("after j: cursor=%d, want 1", m.cursor)
	}
	m, _ = m.Update(key("j"))
	m, _ = m.Update(key("j"))
	m, _ = m.Update(key("j"))
	if m.cursor != 4 {
		t.Errorf("after 4xj: cursor=%d, want 4", m.cursor)
	}
	// At bottom, j wraps to 0.
	m, _ = m.Update(key("j"))
	if m.cursor != 0 {
		t.Errorf("j past bottom: cursor=%d, want wrap to 0", m.cursor)
	}
	// At top, k wraps to last.
	m, _ = m.Update(key("k"))
	if m.cursor != 4 {
		t.Errorf("k past top: cursor=%d, want wrap to 4", m.cursor)
	}
}

func TestHelmDocMenu_GG_GotoEnds(t *testing.T) {
	m := newHelmDocMenu(t)
	cmd := m.Open("nginx", "default")
	drainAnimationToInteractive(t, &m, cmd)

	m, _ = m.Update(key("G"))
	if m.cursor != 4 {
		t.Errorf("G should jump to last: cursor=%d, want 4", m.cursor)
	}
	m, _ = m.Update(key("g"))
	if m.cursor != 0 {
		t.Errorf("g should jump to first: cursor=%d, want 0", m.cursor)
	}
}

func TestHelmDocMenu_EnterEmitsRequestMsg(t *testing.T) {
	m := newHelmDocMenu(t)
	cmd := m.Open("nginx", "default")
	drainAnimationToInteractive(t, &m, cmd)

	m, _ = m.Update(key("j")) // cursor=1 → Creator Notes
	_, batchCmd := m.Update(key("enter"))
	if batchCmd == nil {
		t.Fatal("Enter should produce a batched close+request cmd")
	}
	// Walk the batch and find the HelmDocRequestMsg producer.
	found := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		req, ok := msg.(HelmDocRequestMsg)
		if !ok {
			return false
		}
		if req.DocKind != k8s.HelmDocNotes || req.ReleaseName != "nginx" || req.Namespace != "default" {
			t.Errorf("request msg fields wrong: %+v", req)
		}
		found = true
		return true
	})
	if !found {
		t.Error("HelmDocRequestMsg not found in Enter's batch")
	}
}

func TestHelmDocMenu_SpaceCloses(t *testing.T) {
	// New v1.5.x mental model: Space = mirror open, close the popup. Commit
	// goes through Enter only. Verifies Space does NOT emit a doc request
	// and does fire animator close.
	m := newHelmDocMenu(t)
	cmd := m.Open("nginx", "default")
	drainAnimationToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key(" "))
	if closeCmd == nil {
		t.Fatal("Space should fire animator close cmd")
	}
	expectMsg(t, closeCmd, func(msg tea.Msg) bool {
		if _, ok := msg.(HelmDocRequestMsg); ok {
			t.Error("Space must not emit HelmDocRequestMsg under new mental model")
		}
		return false
	})
}

func TestHelmDocMenu_EscCloses(t *testing.T) {
	m := newHelmDocMenu(t)
	cmd := m.Open("nginx", "default")
	drainAnimationToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key("esc"))
	if closeCmd == nil {
		t.Fatal("Esc should fire animator close cmd")
	}
}

// expectMsg walks a (possibly batched) tea.Cmd and invokes `match` against
// each produced message until match returns true or the batch is exhausted.
// Tea's batch internals aren't exported, so this best-effort approach just
// invokes the cmd and inspects the single message returned — adequate for
// the simple emit-then-close pattern used here.
func expectMsg(t *testing.T, cmd tea.Cmd, match func(tea.Msg) bool) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	// Single message path.
	if match(msg) {
		return
	}
	// tea.Batch returns BatchMsg whose contents are individual cmds.
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if match(c()) {
				return
			}
		}
	}
}
