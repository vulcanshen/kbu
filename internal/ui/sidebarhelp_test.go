package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/km8/internal/theme"
)

func newSidebarHelp(t *testing.T) SidebarHelpPopupModel {
	t.Helper()
	th, err := theme.LoadTheme("")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}
	m := NewSidebarHelpPopupModel(th)
	m.SetSize(120, 40)
	return m
}

func drainSidebarHelpToInteractive(t *testing.T, m *SidebarHelpPopupModel, openCmd tea.Cmd) {
	t.Helper()
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

func TestSidebarHelp_OpenRendersAllRows(t *testing.T) {
	m := newSidebarHelp(t)
	cmd := m.Open()
	drainSidebarHelpToInteractive(t, &m, cmd)

	view := m.RenderPopup()
	// Spot-check that the key labels show up — content is fixed-string
	// so this also doubles as a regression catch if someone empties the
	// rows list.
	for _, want := range []string{"j/k", "Enter", "/", "Esc", "N", "C"} {
		if !strings.Contains(view, want) {
			t.Errorf("rendered popup missing key %q", want)
		}
	}
}

func TestSidebarHelp_SpaceCloses(t *testing.T) {
	m := newSidebarHelp(t)
	cmd := m.Open()
	drainSidebarHelpToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key(" "))
	if closeCmd == nil {
		t.Fatal("Space must trigger animator close")
	}
}

func TestSidebarHelp_EscCloses(t *testing.T) {
	m := newSidebarHelp(t)
	cmd := m.Open()
	drainSidebarHelpToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key("esc"))
	if closeCmd == nil {
		t.Fatal("Esc must trigger animator close")
	}
}

func TestSidebarHelp_EnterCloses(t *testing.T) {
	// Enter is treated as another close key — popup is read-only, there's
	// nothing to commit. Avoids the surprise of "Enter does nothing".
	m := newSidebarHelp(t)
	cmd := m.Open()
	drainSidebarHelpToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key("enter"))
	if closeCmd == nil {
		t.Fatal("Enter must close the read-only help popup")
	}
}
