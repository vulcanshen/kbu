package ui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestTableModel_EnterSearch(t *testing.T) {
	m := newTestTable()
	m.SetRows(sampleRows(5))

	// Press / to enter search mode.
	m, _ = m.Update(keyMsg('/'))
	if !m.searching {
		t.Fatal("expected searching=true after /")
	}
	if m.searchQuery != "" {
		t.Errorf("expected empty searchQuery, got %q", m.searchQuery)
	}

	// All rows should still be visible (empty query).
	if len(m.rows) != 5 {
		t.Errorf("expected 5 rows with empty query, got %d", len(m.rows))
	}
}

func TestTableModel_SearchFilter(t *testing.T) {
	m := newTestTable()
	rows := [][]string{
		{"nginx-pod", "1/1", "Running", "0", "5m", "node-1"},
		{"redis-pod", "1/1", "Running", "0", "3m", "node-2"},
		{"postgres-db", "1/1", "Running", "0", "10m", "node-1"},
		{"nginx-svc", "1/1", "Pending", "0", "2m", "node-3"},
	}
	m.SetRows(rows)

	// Enter search mode.
	m, _ = m.Update(keyMsg('/'))
	if !m.searching {
		t.Fatal("expected searching=true after /")
	}

	// Type "nginx" — should filter to 2 rows.
	for _, r := range "nginx" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.searchQuery != "nginx" {
		t.Errorf("expected searchQuery='nginx', got %q", m.searchQuery)
	}
	if len(m.rows) != 2 {
		t.Fatalf("expected 2 filtered rows for 'nginx', got %d", len(m.rows))
	}
	if m.rows[0][0] != "nginx-pod" {
		t.Errorf("expected first filtered row 'nginx-pod', got %q", m.rows[0][0])
	}
	if m.rows[1][0] != "nginx-svc" {
		t.Errorf("expected second filtered row 'nginx-svc', got %q", m.rows[1][0])
	}
}

func TestTableModel_SearchFilterCaseInsensitive(t *testing.T) {
	m := newTestTable()
	rows := [][]string{
		{"Nginx-Pod", "1/1", "Running", "0", "5m", "node-1"},
		{"redis-pod", "1/1", "Running", "0", "3m", "node-2"},
	}
	m.SetRows(rows)

	// Enter search mode and type "nginx" (lowercase).
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "nginx" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Should match "Nginx-Pod" case-insensitively.
	if len(m.rows) != 1 {
		t.Fatalf("expected 1 filtered row for case-insensitive 'nginx', got %d", len(m.rows))
	}
	if m.rows[0][0] != "Nginx-Pod" {
		t.Errorf("expected filtered row 'Nginx-Pod', got %q", m.rows[0][0])
	}
}

func TestTableModel_SearchClearOnEsc(t *testing.T) {
	m := newTestTable()
	rows := [][]string{
		{"nginx-pod", "1/1", "Running", "0", "5m", "node-1"},
		{"redis-pod", "1/1", "Running", "0", "3m", "node-2"},
		{"postgres-db", "1/1", "Running", "0", "10m", "node-1"},
	}
	m.SetRows(rows)

	// Enter search, type "nginx", then press Esc.
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "nginx" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(m.rows) != 1 {
		t.Fatalf("expected 1 row for 'nginx', got %d", len(m.rows))
	}

	// Press Esc — should clear filter and exit search mode.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.searching {
		t.Error("expected searching=false after Esc")
	}
	if m.searchQuery != "" {
		t.Errorf("expected empty searchQuery after Esc, got %q", m.searchQuery)
	}
	if len(m.rows) != 3 {
		t.Errorf("expected all 3 rows after Esc, got %d", len(m.rows))
	}
}

func TestTableModel_SearchConfirmOnEnter(t *testing.T) {
	m := newTestTable()
	rows := [][]string{
		{"nginx-pod", "1/1", "Running", "0", "5m", "node-1"},
		{"redis-pod", "1/1", "Running", "0", "3m", "node-2"},
		{"postgres-db", "1/1", "Running", "0", "10m", "node-1"},
	}
	m.SetRows(rows)

	// Enter search, type "redis", then press Enter.
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "redis" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(m.rows) != 1 {
		t.Fatalf("expected 1 row for 'redis', got %d", len(m.rows))
	}

	// Press Enter — should exit search mode but keep filter.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.searching {
		t.Error("expected searching=false after Enter")
	}
	if m.searchQuery != "redis" {
		t.Errorf("expected searchQuery='redis' after Enter, got %q", m.searchQuery)
	}
	if len(m.rows) != 1 {
		t.Errorf("expected filter to remain active with 1 row, got %d", len(m.rows))
	}
}

func TestTableModel_SearchOriginalIndex(t *testing.T) {
	m := newTestTable()
	rows := [][]string{
		{"nginx-pod", "1/1", "Running", "0", "5m", "node-1"},   // original index 0
		{"redis-pod", "1/1", "Running", "0", "3m", "node-2"},   // original index 1
		{"postgres-db", "1/1", "Running", "0", "10m", "node-1"}, // original index 2
		{"nginx-svc", "1/1", "Pending", "0", "2m", "node-3"},   // original index 3
	}
	m.SetRows(rows)

	// Enter search, type "nginx".
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "nginx" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// 2 rows should be visible: nginx-pod (original 0) and nginx-svc (original 3).
	if len(m.rows) != 2 {
		t.Fatalf("expected 2 filtered rows, got %d", len(m.rows))
	}

	// OriginalIndex for display index 0 should be 0.
	if m.OriginalIndex(0) != 0 {
		t.Errorf("expected OriginalIndex(0)=0, got %d", m.OriginalIndex(0))
	}

	// OriginalIndex for display index 1 should be 3.
	if m.OriginalIndex(1) != 3 {
		t.Errorf("expected OriginalIndex(1)=3, got %d", m.OriginalIndex(1))
	}

	// SelectedRow should return the original index.
	m.cursor = 1
	if m.SelectedRow() != 3 {
		t.Errorf("expected SelectedRow()=3 when cursor=1, got %d", m.SelectedRow())
	}
}

func TestTableModel_SearchBackspace(t *testing.T) {
	m := newTestTable()
	rows := [][]string{
		{"nginx-pod", "1/1", "Running", "0", "5m", "node-1"},
		{"redis-pod", "1/1", "Running", "0", "3m", "node-2"},
	}
	m.SetRows(rows)

	// Enter search, type "nginx", then backspace one char.
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "nginx" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(m.rows) != 1 {
		t.Fatalf("expected 1 row for 'nginx', got %d", len(m.rows))
	}

	// Backspace — query becomes "ngin", still matches only "nginx-pod".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.searchQuery != "ngin" {
		t.Errorf("expected searchQuery='ngin' after backspace, got %q", m.searchQuery)
	}
	if len(m.rows) != 1 {
		t.Errorf("expected 1 row for 'ngin', got %d", len(m.rows))
	}

	// Backspace 4 more times — query becomes empty, all rows visible.
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	if m.searchQuery != "" {
		t.Errorf("expected empty searchQuery after clearing, got %q", m.searchQuery)
	}
	if len(m.rows) != 2 {
		t.Errorf("expected 2 rows with empty query, got %d", len(m.rows))
	}
}

func TestTableModel_SearchNavigateWhileSearching(t *testing.T) {
	m := newTestTable()
	rows := [][]string{
		{"nginx-pod-1", "1/1", "Running", "0", "5m", "node-1"},
		{"nginx-pod-2", "1/1", "Running", "0", "3m", "node-2"},
		{"nginx-pod-3", "1/1", "Running", "0", "10m", "node-1"},
	}
	m.SetRows(rows)

	// Enter search, type "nginx".
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "nginx" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}

	// Navigate down with arrow key while searching.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after down arrow while searching, got %d", m.cursor)
	}

	// Still in search mode.
	if !m.searching {
		t.Error("expected to still be in search mode after navigating")
	}
}

func TestTableModel_SetRowsPreservesFilter(t *testing.T) {
	m := newTestTable()
	rows := [][]string{
		{"nginx-pod", "1/1", "Running", "0", "5m", "node-1"},
		{"redis-pod", "1/1", "Running", "0", "3m", "node-2"},
	}
	m.SetRows(rows)

	// Enter search, type "nginx", confirm with Enter.
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "nginx" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Now set new rows (simulating a watch update).
	newRows := [][]string{
		{"nginx-pod", "1/1", "Running", "0", "6m", "node-1"},
		{"redis-pod", "1/1", "Running", "0", "4m", "node-2"},
		{"nginx-new", "1/1", "Running", "0", "1m", "node-3"},
	}
	m.SetRows(newRows)

	// Filter "nginx" should still be active, showing 2 of 3 rows.
	if m.searchQuery != "nginx" {
		t.Errorf("expected searchQuery='nginx' after SetRows, got %q", m.searchQuery)
	}
	if len(m.rows) != 2 {
		t.Errorf("expected 2 filtered rows after SetRows, got %d", len(m.rows))
	}
}

func TestTableModel_SearchColumnMatch(t *testing.T) {
	m := newTestTable()
	rows := [][]string{
		{"pod-1", "1/1", "Running", "0", "5m", "node-1"},
		{"pod-2", "0/1", "Pending", "0", "3m", "node-2"},
		{"pod-3", "1/1", "Error", "5", "10m", "node-1"},
	}
	m.SetRows(rows)

	// Search for "Pending" — should match on status column.
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "Pending" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if len(m.rows) != 1 {
		t.Fatalf("expected 1 row matching 'Pending', got %d", len(m.rows))
	}
	if m.rows[0][0] != "pod-2" {
		t.Errorf("expected matching row 'pod-2', got %q", m.rows[0][0])
	}
}

func TestColumnsForResource(t *testing.T) {
	allTypes := k8s.AllResourceTypes()

	if len(allTypes) != 17 {
		t.Fatalf("expected 17 resource types, got %d", len(allTypes))
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
