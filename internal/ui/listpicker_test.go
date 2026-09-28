package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/kbu/internal/theme"
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

// SetItems refreshes an open picker in place — the sort flow redraws the
// column badges after a tier lands — without moving the cursor or
// changing the row count the popup opened with.
func TestListPicker_SetItemsRefreshesInPlace(t *testing.T) {
	m := newListPicker(t)
	cmd := m.Open("sort:column", "Sort", []ListPickerItem{
		{Key: "a", Label: "A"},
		{Key: "b", Label: "B"},
	})
	drainListPickerToInteractive(t, &m, cmd)
	m, _ = m.Update(key("j"))

	m.SetItems([]ListPickerItem{
		{Key: "a", Label: "A"},
		{Key: "b", Label: "B", Badge: "↑"},
	})
	if m.cursor != 1 || m.items[1].Badge != "↑" || !m.IsInteractive() {
		t.Errorf("SetItems must keep the cursor and the popup open, got cursor=%d badge=%q open=%v",
			m.cursor, m.items[1].Badge, m.IsInteractive())
	}
}

// tdp M6: a dimmed row takes the cursor but Enter does nothing on it.
func TestListPicker_DisabledRowDoesNotCommit(t *testing.T) {
	m := newListPicker(t)
	cmd := m.Open("sort:column", "Sort", []ListPickerItem{
		{Key: "reset", Label: "Reset", Disabled: true},
		{Key: "a", Label: "A"},
	})
	drainListPickerToInteractive(t, &m, cmd)
	if m.cursor != 0 {
		t.Fatalf("the cursor must be able to rest on a dimmed row, at %d", m.cursor)
	}
	if _, cmd := m.Update(key("enter")); cmd != nil {
		t.Error("Enter on a dimmed row must do nothing")
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

func TestListPicker_SeparatorSkippedByNavigation(t *testing.T) {
	// jk, gG and the initial cursor must all skip separator rows.
	// Layout: [a, b, ─, c]
	m := newListPicker(t)
	cmd := m.Open("column", "Sort…", []ListPickerItem{
		{Key: "a", Label: "A"},
		{Key: "b", Label: "B"},
		{Separator: true},
		{Key: "c", Label: "C"},
	})
	drainListPickerToInteractive(t, &m, cmd)

	if m.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", m.cursor)
	}
	// jjj: 0→1→3 (skip separator at 2) →0 (wrap)
	m, _ = m.Update(key("j"))
	if m.cursor != 1 {
		t.Errorf("after j cursor = %d, want 1", m.cursor)
	}
	m, _ = m.Update(key("j"))
	if m.cursor != 3 {
		t.Errorf("j must skip separator at idx 2; cursor = %d, want 3", m.cursor)
	}
	m, _ = m.Update(key("j"))
	if m.cursor != 0 {
		t.Errorf("j past last must wrap to 0; cursor = %d", m.cursor)
	}
	// k from 0 wraps to 3 (skip separator)
	m, _ = m.Update(key("k"))
	if m.cursor != 3 {
		t.Errorf("k must wrap to last selectable (3); cursor = %d", m.cursor)
	}
	// G goes to last selectable, not separator
	m, _ = m.Update(key("g"))
	m, _ = m.Update(key("G"))
	if m.cursor != 3 {
		t.Errorf("G must land on last selectable (3); cursor = %d", m.cursor)
	}
}

func TestListPicker_EnterOnSeparatorIsNoOp(t *testing.T) {
	// Even if cursor somehow lands on a separator, Enter must not
	// commit. Direct manipulation simulates that edge case.
	m := newListPicker(t)
	cmd := m.Open("column", "Sort…", []ListPickerItem{
		{Key: "a", Label: "A"},
		{Separator: true},
		{Key: "b", Label: "B"},
	})
	drainListPickerToInteractive(t, &m, cmd)
	m.cursor = 1 // force cursor onto separator
	_, batchCmd := m.Update(key("enter"))
	if batchCmd != nil {
		t.Errorf("Enter on separator must not emit any cmd, got %T", batchCmd)
	}
}

func TestListPicker_CursorStartsOnCurrentSkippingSeparator(t *testing.T) {
	// Open must still respect Badge=current even when a separator is
	// before the badged row.
	m := newListPicker(t)
	cmd := m.Open("column", "Sort…", []ListPickerItem{
		{Key: "a", Label: "A"},
		{Separator: true},
		{Key: "b", Label: "B", Badge: "current"},
	})
	drainListPickerToInteractive(t, &m, cmd)
	if m.cursor != 2 {
		t.Errorf("cursor on open = %d, want 2 (the Badge=current row)", m.cursor)
	}
}

func TestListPicker_HeaderSkippedByNavigation(t *testing.T) {
	// Header rows are non-selectable region labels — j/k/g/G must
	// skip past them just like separators do.
	m := newListPicker(t)
	cmd := m.Open("column", "Sort…", []ListPickerItem{
		{Header: true, Label: "fields"},
		{Key: "a", Label: "A"},
		{Key: "b", Label: "B"},
		{Separator: true},
		{Header: true, Label: "all"},
		{Key: "reset", Label: "Reset"},
	})
	drainListPickerToInteractive(t, &m, cmd)

	// Initial cursor lands on first selectable (idx 1, "a"), NOT
	// on the header at idx 0.
	if m.cursor != 1 {
		t.Errorf("initial cursor = %d, want 1 (first selectable, skipping header)", m.cursor)
	}
	// j from "a" lands on "b" (idx 2).
	m, _ = m.Update(key("j"))
	if m.cursor != 2 {
		t.Errorf("after j cursor = %d, want 2", m.cursor)
	}
	// j from "b" jumps over separator (idx 3) and header (idx 4)
	// to reset (idx 5).
	m, _ = m.Update(key("j"))
	if m.cursor != 5 {
		t.Errorf("j must skip separator AND header; cursor = %d, want 5", m.cursor)
	}
	// G goes to last selectable (idx 5).
	m, _ = m.Update(key("G"))
	if m.cursor != 5 {
		t.Errorf("G must land on last selectable (5); cursor = %d", m.cursor)
	}
	// g goes to first selectable (idx 1).
	m, _ = m.Update(key("g"))
	if m.cursor != 1 {
		t.Errorf("g must land on first selectable (1); cursor = %d", m.cursor)
	}
}

func TestListPicker_EnterOnHeaderIsNoOp(t *testing.T) {
	m := newListPicker(t)
	cmd := m.Open("column", "Sort…", []ListPickerItem{
		{Header: true, Label: "fields"},
		{Key: "a", Label: "A"},
	})
	drainListPickerToInteractive(t, &m, cmd)
	m.cursor = 0 // force cursor onto header
	_, batchCmd := m.Update(key("enter"))
	if batchCmd != nil {
		t.Errorf("Enter on header must not emit any cmd, got %T", batchCmd)
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
