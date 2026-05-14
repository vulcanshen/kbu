package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestSidebar() SidebarModel {
	t := theme.DefaultTheme()
	m := NewSidebarModel(t)
	m.SetSize(30, 40)
	m.SetFocused(true)
	return m
}

func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestSidebarModel_InitialState(t *testing.T) {
	m := newTestSidebar()

	// Cursor should be at 0.
	if m.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", m.cursor)
	}

	// Pods should be selected by default.
	if m.Selected() != k8s.ResourcePods {
		t.Errorf("expected selected=ResourcePods, got %v", m.Selected())
	}

	// All categories should be expanded.
	for _, cat := range m.categories {
		if !cat.Expanded {
			t.Errorf("expected category %q to be expanded", cat.Label)
		}
	}

	// Verify visible items count:
	// 4 categories + 2 + 6 + 2 + 2 resources + 1 standalone = 17
	visible := m.visibleItems()
	if len(visible) != 17 {
		t.Errorf("expected 17 visible items, got %d", len(visible))
	}
}

func TestSidebarModel_NavigateDown(t *testing.T) {
	m := newTestSidebar()

	// Initially at position 0 (Cluster category).
	if m.cursor != 0 {
		t.Fatalf("expected cursor=0, got %d", m.cursor)
	}

	// Press j — should move to position 1 (Namespaces).
	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", m.cursor)
	}

	// Press j again — should move to position 2 (Nodes).
	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 after second j, got %d", m.cursor)
	}
}

func TestSidebarModel_NavigateUp(t *testing.T) {
	m := newTestSidebar()

	// Move down twice first.
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 2 {
		t.Fatalf("expected cursor=2, got %d", m.cursor)
	}

	// Press k — should move up to position 1.
	m, _ = m.Update(keyMsg('k'))
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after k, got %d", m.cursor)
	}

	// Press k again — should move up to position 0.
	m, _ = m.Update(keyMsg('k'))
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after second k, got %d", m.cursor)
	}

	// Press k at top — should stay at 0.
	m, _ = m.Update(keyMsg('k'))
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 at top boundary, got %d", m.cursor)
	}
}

func TestSidebarModel_CollapseCategory(t *testing.T) {
	m := newTestSidebar()

	// Cursor is at 0 (Cluster category, expanded).
	if !m.categories[0].Expanded {
		t.Fatal("expected Cluster to be expanded initially")
	}

	// Press Enter on Cluster category.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Cluster should now be collapsed.
	if m.categories[0].Expanded {
		t.Error("expected Cluster to be collapsed after Enter")
	}

	// Visible items should decrease by 2 (Namespaces + Nodes).
	visible := m.visibleItems()
	if len(visible) != 15 {
		t.Errorf("expected 15 visible items after collapse, got %d", len(visible))
	}
}

func TestSidebarModel_ExpandCategory(t *testing.T) {
	m := newTestSidebar()

	// Collapse Cluster first.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.categories[0].Expanded {
		t.Fatal("expected Cluster to be collapsed")
	}

	// Press Enter again to expand.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.categories[0].Expanded {
		t.Error("expected Cluster to be expanded after second Enter")
	}

	visible := m.visibleItems()
	if len(visible) != 17 {
		t.Errorf("expected 17 visible items after expand, got %d", len(visible))
	}
}

func TestSidebarModel_SelectResource(t *testing.T) {
	m := newTestSidebar()

	// Navigate to position 1 (Namespaces resource under Cluster).
	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", m.cursor)
	}

	// Press Enter to select.
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Selected should be Namespaces.
	if m.Selected() != k8s.ResourceNamespaces {
		t.Errorf("expected selected=ResourceNamespaces, got %v", m.Selected())
	}

	// Cmd should produce a ResourceSelectedMsg.
	if cmd == nil {
		t.Fatal("expected cmd to be non-nil")
	}
	msg := cmd()
	rsMsg, ok := msg.(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("expected ResourceSelectedMsg, got %T", msg)
	}
	if rsMsg.Type != k8s.ResourceNamespaces {
		t.Errorf("expected ResourceSelectedMsg.Type=ResourceNamespaces, got %v", rsMsg.Type)
	}
}

func TestSidebarModel_GG(t *testing.T) {
	m := newTestSidebar()

	// Move cursor down several times.
	for i := 0; i < 5; i++ {
		m, _ = m.Update(keyMsg('j'))
	}
	if m.cursor != 5 {
		t.Fatalf("expected cursor=5, got %d", m.cursor)
	}

	// Press g (first).
	m, _ = m.Update(keyMsg('g'))
	if !m.pendingG {
		t.Fatal("expected pendingG to be true after first g")
	}

	// Press g (second) — cursor should go to 0.
	m, _ = m.Update(keyMsg('g'))
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after gg, got %d", m.cursor)
	}
	if m.pendingG {
		t.Error("expected pendingG to be false after gg")
	}
}

func TestSidebarModel_ShiftG(t *testing.T) {
	m := newTestSidebar()

	// Press G (shift+g) — cursor should go to last visible item.
	m, _ = m.Update(keyMsg('G'))

	visible := m.visibleItems()
	expected := len(visible) - 1
	if m.cursor != expected {
		t.Errorf("expected cursor=%d after G, got %d", expected, m.cursor)
	}
}

func TestSidebarModel_SkipCollapsedChildren(t *testing.T) {
	m := newTestSidebar()

	// Collapse Cluster (position 0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.categories[0].Expanded {
		t.Fatal("expected Cluster collapsed")
	}

	// Now position 0 is Cluster (collapsed).
	// Press j — should skip to Workloads (next visible item, which is position 1).
	m, _ = m.Update(keyMsg('j'))

	visible := m.visibleItems()
	// Position 1 should be Workloads category.
	if m.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", m.cursor)
	}
	item := visible[m.cursor]
	if !item.isCategory || item.label != "Workloads" {
		t.Errorf("expected Workloads category at cursor, got %+v", item)
	}
}

func TestSidebarModel_HCollapsesParent(t *testing.T) {
	m := newTestSidebar()

	// Navigate to position 1 (Namespaces, under Cluster).
	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", m.cursor)
	}

	// Press h — parent (Cluster) should collapse, cursor moves to Cluster.
	m, _ = m.Update(keyMsg('h'))

	if m.categories[0].Expanded {
		t.Error("expected Cluster to be collapsed after h")
	}

	// Cursor should now be at Cluster category (position 0).
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after h collapse, got %d", m.cursor)
	}

	visible := m.visibleItems()
	item := visible[m.cursor]
	if !item.isCategory || item.label != "Cluster" {
		t.Errorf("expected cursor on Cluster category, got %+v", item)
	}
}
