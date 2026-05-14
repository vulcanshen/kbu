package ui

import (
	"fmt"
	"testing"

	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestTable() TableModel {
	t := theme.DefaultTheme()
	m := NewTableModel(t)
	m.SetFocused(true)
	return m
}

func sampleRows(n int) [][]string {
	rows := make([][]string, n)
	for i := 0; i < n; i++ {
		rows[i] = []string{
			fmt.Sprintf("pod-%d", i),
			"1/1",
			"Running",
			"0",
			"5m",
			"node-1",
		}
	}
	return rows
}

// keyMsg is declared in sidebar_test.go (same package)

func TestTableModel_InitialState(t *testing.T) {
	m := newTestTable()

	// Should start with Pods columns
	podCols := ColumnsForResource(k8s.ResourcePods)
	if len(m.columns) != len(podCols) {
		t.Fatalf("expected %d columns for Pods, got %d", len(podCols), len(m.columns))
	}
	for i, col := range m.columns {
		if col.Title != podCols[i].Title {
			t.Errorf("column %d: expected title %q, got %q", i, podCols[i].Title, col.Title)
		}
	}

	// Empty rows
	if len(m.rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(m.rows))
	}

	// Cursor at 0
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}

	// Resource type is Pods
	if m.resourceType != k8s.ResourcePods {
		t.Errorf("expected resource type Pods, got %v", m.resourceType)
	}
}

func TestTableModel_NavigateDown(t *testing.T) {
	m := newTestTable()
	m.SetRows(sampleRows(5))

	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", m.cursor)
	}

	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2 after second j, got %d", m.cursor)
	}
}

func TestTableModel_NavigateUp(t *testing.T) {
	m := newTestTable()
	m.SetRows(sampleRows(5))
	m.cursor = 2

	m, _ = m.Update(keyMsg('k'))
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after k, got %d", m.cursor)
	}
}

func TestTableModel_NavigateDownAtBottom(t *testing.T) {
	m := newTestTable()
	m.SetRows(sampleRows(5))
	m.cursor = 4 // last row

	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 4 {
		t.Errorf("expected cursor to stay at 4, got %d", m.cursor)
	}
}

func TestTableModel_NavigateUpAtTop(t *testing.T) {
	m := newTestTable()
	m.SetRows(sampleRows(5))
	m.cursor = 0

	m, _ = m.Update(keyMsg('k'))
	if m.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.cursor)
	}
}

func TestTableModel_GG(t *testing.T) {
	m := newTestTable()
	m.SetRows(sampleRows(10))
	m.cursor = 5

	// First g: sets pendingG
	m, _ = m.Update(keyMsg('g'))
	if !m.pendingG {
		t.Fatal("expected pendingG to be true after first g")
	}
	if m.cursor != 5 {
		t.Errorf("expected cursor to remain at 5 after first g, got %d", m.cursor)
	}

	// Second g: cursor goes to 0
	m, _ = m.Update(keyMsg('g'))
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0 after gg, got %d", m.cursor)
	}
	if m.pendingG {
		t.Error("expected pendingG to be false after gg")
	}
}

func TestTableModel_ShiftG(t *testing.T) {
	m := newTestTable()
	m.SetRows(sampleRows(10))
	m.cursor = 0

	m, _ = m.Update(keyMsg('G'))
	if m.cursor != 9 {
		t.Errorf("expected cursor at 9 (last row), got %d", m.cursor)
	}
}

func TestTableModel_ResourceSelectedMsg(t *testing.T) {
	m := newTestTable()
	m.SetRows(sampleRows(5))
	m.cursor = 3

	msg := ResourceSelectedMsg{Type: k8s.ResourceDeployments}
	m, _ = m.Update(msg)

	deployCols := ColumnsForResource(k8s.ResourceDeployments)
	if len(m.columns) != len(deployCols) {
		t.Fatalf("expected %d columns for Deployments, got %d", len(deployCols), len(m.columns))
	}
	for i, col := range m.columns {
		if col.Title != deployCols[i].Title {
			t.Errorf("column %d: expected %q, got %q", i, deployCols[i].Title, col.Title)
		}
	}
	if len(m.rows) != 0 {
		t.Errorf("expected rows to be cleared, got %d", len(m.rows))
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor reset to 0, got %d", m.cursor)
	}
	if m.resourceType != k8s.ResourceDeployments {
		t.Errorf("expected resource type Deployments, got %v", m.resourceType)
	}
}

func TestTableModel_SetRows(t *testing.T) {
	m := newTestTable()
	rows := sampleRows(3)
	m.SetRows(rows)

	if len(m.rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(m.rows))
	}
	if m.rows[0][0] != "pod-0" {
		t.Errorf("expected first row name %q, got %q", "pod-0", m.rows[0][0])
	}
	if m.rows[2][0] != "pod-2" {
		t.Errorf("expected last row name %q, got %q", "pod-2", m.rows[2][0])
	}
}

func TestTableModel_ScrollOffset(t *testing.T) {
	m := newTestTable()
	m.SetSize(80, 5) // 5 lines total, 4 visible rows (height - 1 for header)
	m.SetRows(sampleRows(20))

	// Navigate down past visible area
	for i := 0; i < 6; i++ {
		m, _ = m.Update(keyMsg('j'))
	}

	if m.cursor != 6 {
		t.Errorf("expected cursor at 6, got %d", m.cursor)
	}

	// scrollOffset should have adjusted so cursor is visible
	visible := m.visibleRows()
	if m.cursor >= m.scrollOffset+visible {
		t.Errorf("cursor %d should be visible with scrollOffset %d and visible %d",
			m.cursor, m.scrollOffset, visible)
	}
	if m.scrollOffset <= 0 {
		t.Errorf("expected scrollOffset > 0 after scrolling down, got %d", m.scrollOffset)
	}
}

func TestColumnsForResource(t *testing.T) {
	allTypes := k8s.AllResourceTypes()

	if len(allTypes) != 13 {
		t.Fatalf("expected 13 resource types, got %d", len(allTypes))
	}

	for _, rt := range allTypes {
		cols := ColumnsForResource(rt)
		if len(cols) == 0 {
			t.Errorf("ColumnsForResource(%v) returned empty columns", rt)
		}
		for i, col := range cols {
			if col.Title == "" {
				t.Errorf("ColumnsForResource(%v) column %d has empty title", rt, i)
			}
			if col.MinWidth <= 0 {
				t.Errorf("ColumnsForResource(%v) column %d %q has non-positive MinWidth %d",
					rt, i, col.Title, col.MinWidth)
			}
		}
	}
}
