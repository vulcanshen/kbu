package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/theme"
)

func openHelp(t *testing.T, rows []helpRow, w, h int) HelpModel {
	t.Helper()
	m := NewHelpModel(theme.DefaultTheme())
	m.SetSize(w, h)
	_ = m.Open("Test keys", rows)
	m.animator.Finalize()
	return m
}

func manyRows(n int) []helpRow {
	rows := make([]helpRow, n)
	for i := range rows {
		rows[i] = helpRow{key: fmt.Sprintf("k%d", i), desc: fmt.Sprintf("row %d", i)}
	}
	return rows
}

func TestHelpModel_InitialState(t *testing.T) {
	m := NewHelpModel(theme.DefaultTheme())
	if m.IsActive() {
		t.Error("the key reference must start closed")
	}
}

// tdp K6: ? again, or Esc, closes the key reference.
func TestHelpModel_QuestionMarkAndEscClose(t *testing.T) {
	for _, k := range []tea.KeyMsg{key("?"), key("esc")} {
		m := openHelp(t, manyRows(3), 100, 40)
		m, _ = m.Update(k)
		if m.animator.Owns() {
			t.Errorf("%s must close the key reference", k.String())
		}
	}
}

// tdp M4, F1: the key reference is a note — no cursor, nothing to run.
// Enter and letter keys do nothing.
func TestHelpModel_IsReadOnly(t *testing.T) {
	m := openHelp(t, manyRows(3), 100, 40)
	for _, k := range []tea.KeyMsg{key("enter"), key("y"), key(" ")} {
		next, cmd := m.Update(k)
		if cmd != nil || !next.animator.Owns() {
			t.Errorf("%q must do nothing on the key reference", k.String())
		}
	}
}

// It is as tall as its rows, capped by the screen, and scrolls past that.
func TestHelpModel_ScrollsWhenTallerThanTheScreen(t *testing.T) {
	m := openHelp(t, manyRows(80), 100, 30)
	if got := len(strings.Split(m.renderFullPopup(), "\n")); got > 30 {
		t.Fatalf("the key reference is %d rows tall on a 30-row screen", got)
	}
	m, _ = m.Update(key("j"))
	if m.scrollOffset != 1 {
		t.Errorf("j must scroll one row, offset %d", m.scrollOffset)
	}
	m, _ = m.Update(key("G"))
	if !strings.Contains(ansi.Strip(m.renderFullPopup()), "row 79") {
		t.Error("G must scroll to the last row")
	}
	short := openHelp(t, manyRows(3), 100, 30)
	if got := len(strings.Split(short.renderFullPopup(), "\n")); got != 3+4 {
		t.Errorf("a 3-row reference must be 7 rows tall (rows + frame), got %d", got)
	}
}

func TestHelpModel_RenderNeverPanicsAcrossSizes(t *testing.T) {
	for _, size := range [][2]int{{20, 8}, {80, 24}, {80, 40}, {200, 60}} {
		m := openHelp(t, manyRows(40), size[0], size[1])
		_ = m.RenderPopup()
	}
}
