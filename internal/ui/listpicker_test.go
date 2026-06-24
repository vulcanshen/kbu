package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/km8/internal/theme"
)

func newListPicker(t *testing.T) ListPickerModel {
	t.Helper()
	th, err := theme.LoadTheme("")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}
	m := NewListPickerModel(th)
	m.SetSize(120, 40)
	return m
}

func drainListPickerToInteractive(t *testing.T, m *ListPickerModel, openCmd tea.Cmd) {
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

func TestListPicker_EnterCommitsCursorRow(t *testing.T) {
	// Cursor starts at index 0 (or at the "current" badged row).
	// Enter emits ListPickerActionMsg{PickerID, Key=row 0's Key}.
	m := newListPicker(t)
	cmd := m.Open("column", "Sort Pods by…", []ListPickerItem{
		{Key: "Name", Label: "Name"},
		{Key: "Age", Label: "Age"},
	})
	drainListPickerToInteractive(t, &m, cmd)

	_, batchCmd := m.Update(key("enter"))
	if batchCmd == nil {
		t.Fatal("Enter must emit a commit msg")
	}
	found := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		am, ok := msg.(ListPickerActionMsg)
		if !ok {
			return false
		}
		if am.PickerID != "column" || am.Key != "Name" {
			t.Errorf("commit msg = %+v, want {PickerID:column Key:Name}", am)
		}
		found = true
		return true
	})
	if !found {
		t.Error("Enter did not emit ListPickerActionMsg")
	}
}

func TestListPicker_CursorStartsOnCurrentBadge(t *testing.T) {
	// "current" badge marks the active selection; Open puts the
	// cursor on it so the user immediately sees where they are.
	m := newListPicker(t)
	cmd := m.Open("direction", "Sort Pods by Age…", []ListPickerItem{
		{Key: "asc", Label: "Ascending"},
		{Key: "desc", Label: "Descending", Badge: "current"},
		{Key: "unset", Label: "Unset"},
	})
	drainListPickerToInteractive(t, &m, cmd)

	if m.cursor != 1 {
		t.Errorf("cursor on open = %d, want 1 (the row with Badge=current)", m.cursor)
	}
}

func TestListPicker_OpenWhileOpenSwapsInPlace(t *testing.T) {
	// Chained pickers: app.go calls Open(...) for step 2 while step
	// 1 is still open. Open must not run a new animation — just
	// swap title + items + reset cursor. The animator stays in
	// PopupOpen so the popup stays visible.
	m := newListPicker(t)
	cmd := m.Open("column", "Step 1", []ListPickerItem{
		{Key: "a", Label: "A"},
		{Key: "b", Label: "B"},
	})
	drainListPickerToInteractive(t, &m, cmd)

	cmd2 := m.Open("direction", "Step 2", []ListPickerItem{
		{Key: "asc", Label: "Ascending"},
		{Key: "desc", Label: "Descending"},
	})
	if cmd2 != nil {
		t.Errorf("Open while already open must return nil cmd (no fresh animation), got %T", cmd2)
	}
	if !m.IsInteractive() {
		t.Error("picker should stay interactive across in-place swap")
	}
	if m.pickerID != "direction" || m.title != "Step 2" || len(m.items) != 2 || m.items[0].Key != "asc" {
		t.Errorf("in-place swap left stale content: id=%q title=%q items=%v", m.pickerID, m.title, m.items)
	}
}

func TestListPicker_EscEmitsCancel(t *testing.T) {
	m := newListPicker(t)
	cmd := m.Open("column", "Sort Pods by…", []ListPickerItem{
		{Key: "Name", Label: "Name"},
	})
	drainListPickerToInteractive(t, &m, cmd)

	_, batchCmd := m.Update(key("esc"))
	if batchCmd == nil {
		t.Fatal("Esc must emit close + cancel cmds")
	}
	found := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		cm, ok := msg.(ListPickerCancelMsg)
		if !ok {
			return false
		}
		if cm.PickerID != "column" {
			t.Errorf("cancel msg PickerID = %q, want column", cm.PickerID)
		}
		found = true
		return true
	})
	if !found {
		t.Error("Esc did not emit ListPickerCancelMsg")
	}
}

func TestListPicker_VimNavigation(t *testing.T) {
	m := newListPicker(t)
	cmd := m.Open("column", "Sort…", []ListPickerItem{
		{Key: "a", Label: "A"},
		{Key: "b", Label: "B"},
		{Key: "c", Label: "C"},
	})
	drainListPickerToInteractive(t, &m, cmd)

	m, _ = m.Update(key("j"))
	m, _ = m.Update(key("j"))
	if m.cursor != 2 {
		t.Errorf("after jj cursor = %d, want 2", m.cursor)
	}
	m, _ = m.Update(key("k"))
	if m.cursor != 1 {
		t.Errorf("after k cursor = %d, want 1", m.cursor)
	}
	m, _ = m.Update(key("g"))
	if m.cursor != 0 {
		t.Errorf("after g cursor = %d, want 0", m.cursor)
	}
	m, _ = m.Update(key("G"))
	if m.cursor != 2 {
		t.Errorf("after G cursor = %d, want 2", m.cursor)
	}
}
