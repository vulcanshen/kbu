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

// Visible items layout (24 total):
//  0: Cluster             (category)
//  1: Namespaces           (resource)
//  2: Nodes                (resource)
//  3: Workloads            (category)
//  4: Pods                 (resource)  <- initial cursor
//  5: Deployments          (resource)
//  6: DaemonSets           (resource)
//  7: StatefulSets         (resource)
//  8: Jobs                 (resource)
//  9: CronJobs             (resource)
// 10: Network              (category)
// 11: Services             (resource)
// 12: Ingresses            (resource)
// 13: Config               (category)
// 14: ConfigMaps           (resource)
// 15: Secrets              (resource)
// 16: Access               (category)
// 17: ServiceAccounts      (resource)
// 18: RBAC                 (category)
// 19: ClusterRoles         (resource)
// 20: ClusterRoleBindings  (resource)
// 21: Roles                (resource)
// 22: RoleBindings         (resource)
// 23: Events               (resource, standalone)

func TestSidebarModel_InitialState(t *testing.T) {
	m := newTestSidebar()

	// Cursor should be on Pods (index 4).
	if m.cursor != 4 {
		t.Errorf("expected cursor=4 (Pods), got %d", m.cursor)
	}

	// Pods should be selected by default.
	if m.Selected() != k8s.ResourcePods {
		t.Errorf("expected selected=ResourcePods, got %v", m.Selected())
	}

	// All items should be visible (no collapse/expand).
	// 6 categories + 2 + 6 + 2 + 2 + 1 + 4 resources + 1 standalone = 24
	visible := m.visibleItems()
	if len(visible) != 24 {
		t.Errorf("expected 24 visible items, got %d", len(visible))
	}
}

func TestSidebarModel_NavigateDown(t *testing.T) {
	m := newTestSidebar()

	// Initially at Pods (index 4).
	if m.cursor != 4 {
		t.Fatalf("expected cursor=4, got %d", m.cursor)
	}

	// Press j — should move to Deployments (index 5), skipping no categories.
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('j'))
	if m.cursor != 5 {
		t.Errorf("expected cursor=5 after j, got %d", m.cursor)
	}

	// Should emit ResourceSelectedMsg for Deployments.
	if cmd == nil {
		t.Fatal("expected cmd to be non-nil after j")
	}
	msg := cmd()
	rsm, ok := msg.(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("expected ResourceSelectedMsg, got %T", msg)
	}
	if rsm.Type != k8s.ResourceDeployments {
		t.Errorf("expected ResourceSelectedMsg.Type=ResourceDeployments, got %v", rsm.Type)
	}
}

func TestSidebarModel_NavigateUp(t *testing.T) {
	m := newTestSidebar()

	// Initially at Pods (index 4). Press k — should move to Nodes (index 2),
	// skipping Workloads category (index 3).
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('k'))
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 (Nodes) after k, got %d", m.cursor)
	}

	// Should emit ResourceSelectedMsg for Nodes.
	if cmd == nil {
		t.Fatal("expected cmd to be non-nil after k")
	}
	msg := cmd()
	rsm, ok := msg.(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("expected ResourceSelectedMsg, got %T", msg)
	}
	if rsm.Type != k8s.ResourceNodes {
		t.Errorf("expected ResourceSelectedMsg.Type=ResourceNodes, got %v", rsm.Type)
	}
}

func TestSidebarModel_CursorSkipsCategories(t *testing.T) {
	m := newTestSidebar()

	visible := m.visibleItems()

	// Navigate through all items with j and verify cursor never lands on a category.
	for i := 0; i < 20; i++ {
		m, _ = m.Update(keyMsg('j'))
		if m.cursor >= 0 && m.cursor < len(visible) {
			if visible[m.cursor].isCategory {
				t.Errorf("cursor landed on category at index %d (%s)", m.cursor, visible[m.cursor].label)
			}
		}
	}

	// Navigate back with k and verify cursor never lands on a category.
	for i := 0; i < 20; i++ {
		m, _ = m.Update(keyMsg('k'))
		if m.cursor >= 0 && m.cursor < len(visible) {
			if visible[m.cursor].isCategory {
				t.Errorf("cursor landed on category at index %d (%s)", m.cursor, visible[m.cursor].label)
			}
		}
	}
}

func TestSidebarModel_AutoSelectOnMove(t *testing.T) {
	m := newTestSidebar()

	// Move down from Pods — should auto-select Deployments.
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('j'))
	if m.Selected() != k8s.ResourceDeployments {
		t.Errorf("expected selected=ResourceDeployments after j, got %v", m.Selected())
	}
	if cmd == nil {
		t.Fatal("expected cmd after j")
	}
	msg := cmd()
	rsm, ok := msg.(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("expected ResourceSelectedMsg, got %T", msg)
	}
	if rsm.Type != k8s.ResourceDeployments {
		t.Errorf("expected msg type=ResourceDeployments, got %v", rsm.Type)
	}

	// Move up from Deployments — should auto-select Pods.
	m, cmd = m.Update(keyMsg('k'))
	if m.Selected() != k8s.ResourcePods {
		t.Errorf("expected selected=ResourcePods after k, got %v", m.Selected())
	}
	if cmd == nil {
		t.Fatal("expected cmd after k")
	}
	msg = cmd()
	rsm, ok = msg.(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("expected ResourceSelectedMsg, got %T", msg)
	}
	if rsm.Type != k8s.ResourcePods {
		t.Errorf("expected msg type=ResourcePods, got %v", rsm.Type)
	}
}

func TestSidebarModel_GG(t *testing.T) {
	m := newTestSidebar()

	// Move cursor down several times.
	for i := 0; i < 5; i++ {
		m, _ = m.Update(keyMsg('j'))
	}
	// Should be at CronJobs (index 9).
	if m.cursor != 9 {
		t.Fatalf("expected cursor=9 after 5 j's, got %d", m.cursor)
	}

	// Press g (first).
	m, _ = m.Update(keyMsg('g'))
	if !m.pendingG {
		t.Fatal("expected pendingG to be true after first g")
	}

	// Press g (second) — cursor should go to first resource item (Namespaces, index 1).
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('g'))
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 (Namespaces) after gg, got %d", m.cursor)
	}
	if m.pendingG {
		t.Error("expected pendingG to be false after gg")
	}

	// Should emit ResourceSelectedMsg for Namespaces.
	if cmd == nil {
		t.Fatal("expected cmd after gg")
	}
	msg := cmd()
	rsm, ok := msg.(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("expected ResourceSelectedMsg, got %T", msg)
	}
	if rsm.Type != k8s.ResourceNamespaces {
		t.Errorf("expected ResourceSelectedMsg.Type=ResourceNamespaces, got %v", rsm.Type)
	}
}

func TestSidebarModel_ShiftG(t *testing.T) {
	m := newTestSidebar()

	// Press G — cursor should go to last resource item (Events, index 23).
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('G'))

	if m.cursor != 23 {
		t.Errorf("expected cursor=23 (Events) after G, got %d", m.cursor)
	}

	// Verify it's the Events resource.
	visible := m.visibleItems()
	item := visible[m.cursor]
	if item.isCategory {
		t.Error("expected cursor on resource, got category")
	}
	if item.resourceType != k8s.ResourceEvents {
		t.Errorf("expected Events resource, got %v", item.resourceType)
	}

	// Should emit ResourceSelectedMsg for Events.
	if cmd == nil {
		t.Fatal("expected cmd after G")
	}
	msg := cmd()
	rsm, ok := msg.(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("expected ResourceSelectedMsg, got %T", msg)
	}
	if rsm.Type != k8s.ResourceEvents {
		t.Errorf("expected ResourceSelectedMsg.Type=ResourceEvents, got %v", rsm.Type)
	}
}

func TestSidebarModel_EnterOnResource(t *testing.T) {
	m := newTestSidebar()

	// Cursor is on Pods (index 4). Press Enter.
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Selected should be Pods.
	if m.Selected() != k8s.ResourcePods {
		t.Errorf("expected selected=ResourcePods, got %v", m.Selected())
	}

	// Cmd should produce a ResourceSelectedMsg.
	if cmd == nil {
		t.Fatal("expected cmd to be non-nil")
	}
	msg := cmd()
	rsm, ok := msg.(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("expected ResourceSelectedMsg, got %T", msg)
	}
	if rsm.Type != k8s.ResourcePods {
		t.Errorf("expected ResourceSelectedMsg.Type=ResourcePods, got %v", rsm.Type)
	}
}

func TestSidebarModel_NavigateUpAtTop(t *testing.T) {
	m := newTestSidebar()

	// Move cursor to first resource (Namespaces, index 1).
	m, _ = m.Update(keyMsg('g'))
	m, _ = m.Update(keyMsg('g'))
	if m.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", m.cursor)
	}

	// Press k at the top resource — should stay at 1 (no resource above).
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('k'))
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 at top boundary, got %d", m.cursor)
	}

	// No cmd should be emitted since cursor didn't move.
	if cmd != nil {
		t.Error("expected nil cmd when cursor doesn't move at top")
	}
}

func TestSidebarModel_NavigateDownAtBottom(t *testing.T) {
	m := newTestSidebar()

	// Move cursor to last resource (Events, index 23).
	m, _ = m.Update(keyMsg('G'))
	if m.cursor != 23 {
		t.Fatalf("expected cursor=23, got %d", m.cursor)
	}

	// Press j at the bottom resource — should stay at 23.
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('j'))
	if m.cursor != 23 {
		t.Errorf("expected cursor=23 at bottom boundary, got %d", m.cursor)
	}

	// No cmd should be emitted since cursor didn't move.
	if cmd != nil {
		t.Error("expected nil cmd when cursor doesn't move at bottom")
	}
}
