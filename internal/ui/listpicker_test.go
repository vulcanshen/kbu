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

func TestListPicker_OpenWhileOpenAnimatesSwap(t *testing.T) {
	// Chained pickers: app.go calls Open(...) for step 2 while step 1
	// is still open. Open now runs a mini "yawn" animation —
	// pending content is held until the Compress → Expand midpoint,
	// then promoted into the active fields. Animator stays inside
	// the swap states until both phases run out, so the popup never
	// disappears; from the user's perspective the popup just briefly
	// squeezes and the new content is already inside.
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
	if cmd2 == nil {
		t.Fatal("Open while already open must return a swap-tick cmd, got nil")
	}
	// Pending fields stashed; active content still shows step 1.
	if m.pickerID != "column" || m.title != "Step 1" {
		t.Errorf("active content must stay on step 1 until the midpoint, got id=%q title=%q", m.pickerID, m.title)
	}
	if m.pendingPickerID != "direction" || m.pendingTitle != "Step 2" || len(m.pendingItems) != 2 {
		t.Errorf("pending fields not stashed: id=%q title=%q items=%v", m.pendingPickerID, m.pendingTitle, m.pendingItems)
	}
	// Drive the animation through both phases; the midpoint tick
	// promotes pending → active, then the expand phase finishes.
	for i := 0; i < 50 && (m.animator.State == PopupSwappingCompress || m.animator.State == PopupSwappingExpand); i++ {
		next := m.HandleTick(AnimTickMsg{Target: m.animator.Target})
		_ = next
	}
	if m.animator.State != PopupOpen {
		t.Fatalf("animator should land back on PopupOpen after swap, got %v", m.animator.State)
	}
	if m.pickerID != "direction" || m.title != "Step 2" || len(m.items) != 2 || m.items[0].Key != "asc" {
		t.Errorf("swap end left stale content: id=%q title=%q items=%v", m.pickerID, m.title, m.items)
	}
	if m.pendingItems != nil || m.pendingPickerID != "" {
		t.Errorf("pending fields should be cleared after promotion, got pending=%q items=%v", m.pendingPickerID, m.pendingItems)
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
