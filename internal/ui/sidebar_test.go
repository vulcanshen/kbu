package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func TestSidebarModel_SearchJKAreTypedNotNavigation(t *testing.T) {
	m := newTestSidebar()

	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('k'))

	if m.searchQuery != "jk" {
		t.Errorf("j/k in search must be typed as chars, got query %q", m.searchQuery)
	}
	// Cursor may jump to first match due to filtering — that's expected. The
	// regression we guard against is j/k bypassing the rune handler entirely.
}

func TestSidebarModel_SearchEnterNoMatchClearsFilter(t *testing.T) {
	// Bug: typing junk + Enter left searching=false with a non-empty
	// searchQuery and zero visible items. handleKey's "no visible
	// items → return early" guard then swallowed every subsequent key
	// including `/` to restart search — the panel was stuck until
	// focus changed. Enter on an empty match now behaves like Esc:
	// clear the filter and restore the cursor.
	m := newTestSidebar()

	m, _ = m.Update(keyMsg('/'))
	for _, r := range "xxxnomatchxxx" {
		m, _ = m.Update(keyMsg(r))
	}
	if m.searchQuery == "" {
		t.Fatal("setup: search query expected to be populated")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.searching {
		t.Error("Enter must exit search input mode")
	}
	if m.searchQuery != "" {
		t.Errorf("Enter on empty match must clear searchQuery, got %q", m.searchQuery)
	}
	if m.HasActiveFilter() {
		t.Error("filter must drop when Enter has no match")
	}

	// `/` must work again — the original symptom of the bug was this
	// key getting eaten.
	m, _ = m.Update(keyMsg('/'))
	if !m.searching {
		t.Error("`/` after Enter-no-match must re-enter search mode")
	}
}

func TestSidebarModel_CopyableContent(t *testing.T) {
	m := newTestSidebar()
	got := m.CopyableContent()
	if got == "" {
		t.Fatal("expected non-empty copyable content")
	}
	// Category should be flush-left.
	if !strings.Contains(got, "\nWorkloads\n") {
		t.Errorf("expected flush-left category 'Workloads', got:\n%s", got)
	}
	// Resources should be indented with two spaces.
	if !strings.Contains(got, "  Pods") {
		t.Errorf("expected indented 'Pods', got:\n%s", got)
	}
}

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

// Visible items layout (33 total):
//  0: Cluster                       (category)
//  1: Namespaces                    (resource)
//  2: Nodes                         (resource)
//  3: Events                        (resource)
//  4: Workloads                     (category)
//  5: Pods                          (resource)  <- initial cursor
//  6: Deployments                   (resource)
//  7: DaemonSets                    (resource)
//  8: StatefulSets                  (resource)
//  9: Jobs                          (resource)
// 10: CronJobs                      (resource)
// 11: Network                       (category)
// 12: Services                      (resource)
// 13: Ingresses                     (resource)
// 14: NetworkPolicies               (resource)
// 15: EndpointSlices                (resource)
// 16: IngressClasses                (resource)
// 17: Config                        (category)
// 18: ConfigMaps                    (resource)
// 19: Secrets                       (resource)
// 20: Storage                       (category)
// 21: PersistentVolumes             (resource)
// 22: PersistentVolumeClaims        (resource)
// 23: StorageClasses                (resource)
// 24: RBAC                          (category)
// 25: ClusterRoles                  (resource)
// 26: ClusterRoleBindings           (resource)
// 27: Roles                         (resource)
// 28: RoleBindings                  (resource)
// 29: ServiceAccounts               (resource)
// 30: Autoscaling                   (category)
// 31: HorizontalPodAutoscalers      (resource)
// 32: PodDisruptionBudgets          (resource)

func TestSidebarModel_InitialState(t *testing.T) {
	m := newTestSidebar()

	// Cursor should be on Pods (index 4).
	if m.cursor != 5 {
		t.Errorf("expected cursor=5 (Pods), got %d", m.cursor)
	}

	// Pods should be selected by default.
	if m.Selected() != k8s.ResourcePods {
		t.Errorf("expected selected=ResourcePods, got %v", m.Selected())
	}

	// 7 categories (Cluster, Workloads, Network, Config, Storage, RBAC, Autoscaling) + 26 resources = 33
	visible := m.visibleItems()
	if len(visible) != 33 {
		t.Errorf("expected 33 visible items, got %d", len(visible))
	}
}

func TestSidebarModel_NavigateDown(t *testing.T) {
	m := newTestSidebar()

	// Initially at Pods (index 4).
	if m.cursor != 5 {
		t.Fatalf("expected cursor=5, got %d", m.cursor)
	}

	// Press j — should move to Deployments (index 6).
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('j'))
	if m.cursor != 6 {
		t.Errorf("expected cursor=6 after j, got %d", m.cursor)
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

	// Initially at Pods (index 5). Press k — should move to Events (index 3),
	// skipping Workloads category (index 4).
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('k'))
	if m.cursor != 3 {
		t.Errorf("expected cursor=3 (Events) after k, got %d", m.cursor)
	}

	// Should emit ResourceSelectedMsg for Events.
	if cmd == nil {
		t.Fatal("expected cmd to be non-nil after k")
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
	// Should be at CronJobs (index 10).
	if m.cursor != 10 {
		t.Fatalf("expected cursor=10 after 5 j's, got %d", m.cursor)
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

	// Press G — cursor should go to last resource item (PodDisruptionBudgets, index 32).
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('G'))

	if m.cursor != 32 {
		t.Errorf("expected cursor=32 (PodDisruptionBudgets) after G, got %d", m.cursor)
	}

	// Verify it's the last resource.
	visible := m.visibleItems()
	item := visible[m.cursor]
	if item.isCategory {
		t.Error("expected cursor on resource, got category")
	}
	if item.resourceType != k8s.ResourcePodDisruptionBudgets {
		t.Errorf("expected PodDisruptionBudgets resource, got %v", item.resourceType)
	}

	if cmd == nil {
		t.Fatal("expected cmd after G")
	}
	msg := cmd()
	rsm, ok := msg.(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("expected ResourceSelectedMsg, got %T", msg)
	}
	if rsm.Type != k8s.ResourcePodDisruptionBudgets {
		t.Errorf("expected ResourceSelectedMsg.Type=ResourcePodDisruptionBudgets, got %v", rsm.Type)
	}
}

// TestSidebarModel_EnterEmitsFocusTableMsg verifies l/Enter on the sidebar
// (non-search) emit FocusTableMsg — they used to call activateResource and
// re-fire ResourceSelectedMsg, but j/k already auto-selects, so the only
// useful work left for l/Enter is forwarding focus to panel 2.
func TestSidebarModel_EnterEmitsFocusTableMsg(t *testing.T) {
	m := newTestSidebar()

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Enter must return a Cmd")
	}
	if _, ok := cmd().(FocusTableMsg); !ok {
		t.Fatalf("expected FocusTableMsg, got %T", cmd())
	}
}

// TestSidebarModel_LRetired — `l` no longer focuses the table panel.
// v1.5.x: Enter is the sole drill / focus-next-panel key; `l` retired.
func TestSidebarModel_LRetired(t *testing.T) {
	m := newTestSidebar()

	_, cmd := m.Update(keyMsg('l'))
	if cmd == nil {
		return // no-op is fine
	}
	if _, ok := cmd().(FocusTableMsg); ok {
		t.Fatalf("l must NOT emit FocusTableMsg anymore (Enter is sole focus-next key)")
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

	// Move cursor to last resource (PodDisruptionBudgets, index 32).
	m, _ = m.Update(keyMsg('G'))
	if m.cursor != 32 {
		t.Fatalf("expected cursor=32, got %d", m.cursor)
	}

	// Press j at the bottom resource — should stay at 32.
	var cmd tea.Cmd
	m, cmd = m.Update(keyMsg('j'))
	if m.cursor != 32 {
		t.Errorf("expected cursor=32 at bottom boundary, got %d", m.cursor)
	}

	// No cmd should be emitted since cursor didn't move.
	if cmd != nil {
		t.Error("expected nil cmd when cursor doesn't move at bottom")
	}
}

func TestTruncateSidebarLabel(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		maxWidth int
		want     string
	}{
		{"fits exactly", "Pods", 4, "Pods"},
		{"fits with room", "Pods", 10, "Pods"},
		{"truncates long", "PersistentVolumeClaims", 18, "PersistentVolumeC…"},
		{"truncates to single ellipsis", "PersistentVolumes", 1, "…"},
		{"zero width returns empty", "Pods", 0, ""},
		{"negative width returns empty", "Pods", -1, ""},
		{"empty label", "", 10, ""},
		{"one char fits", "P", 1, "P"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateSidebarLabel(tt.label, tt.maxWidth)
			if got != tt.want {
				t.Errorf("truncateSidebarLabel(%q, %d) = %q, want %q", tt.label, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestSidebarModel_Search_CategoryMatchExpandsAllItems(t *testing.T) {
	m := newTestSidebar()
	m.searchQuery = "cluster"
	items := m.visibleItems()

	// Cluster category header should appear and ALL its native children
	// (Namespaces / Nodes / Events) should be visible — that's the new
	// category-match behavior. Other categories with item-level matches
	// (e.g. RBAC's ClusterRoles) may also appear, which is intended.
	clusterHeaderIdx := -1
	for i, it := range items {
		if it.isCategory && strings.EqualFold(it.label, "Cluster") {
			clusterHeaderIdx = i
			break
		}
	}
	if clusterHeaderIdx < 0 {
		t.Fatal("expected Cluster category header when searching 'cluster'")
	}
	clusterChildren := 0
	for i := clusterHeaderIdx + 1; i < len(items) && !items[i].isCategory; i++ {
		clusterChildren++
	}
	if clusterChildren < 3 {
		t.Errorf("expected ≥3 children under Cluster on category match, got %d", clusterChildren)
	}
}

func TestSidebarModel_Search_ItemMatchOnlyShowsMatching(t *testing.T) {
	m := newTestSidebar()
	m.searchQuery = "pods"
	items := m.visibleItems()

	// Item-level match: only the category containing matching items appears.
	// Cluster category shouldn't appear because "Namespaces" / "Nodes" / "Events"
	// don't contain "pods".
	for _, it := range items {
		if it.isCategory && strings.EqualFold(it.label, "Cluster") {
			t.Errorf("Cluster category should not appear when searching 'pods'")
		}
	}
}

func TestSidebarModel_SetSelected_MovesCursor(t *testing.T) {
	m := newTestSidebar()
	// Initial cursor is on Pods (index 5). Jump to Services (index 12).
	m.SetSelected(k8s.ResourceServices)
	if got := m.Selected(); got != k8s.ResourceServices {
		t.Errorf("Selected() = %v, want %v", got, k8s.ResourceServices)
	}
	visible := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(visible) || visible[m.cursor].resourceType != k8s.ResourceServices {
		t.Errorf("cursor not pointing at Services row, cursor=%d", m.cursor)
	}
}

func TestSidebarModel_SetSelected_NoOpOnMissingType(t *testing.T) {
	m := newTestSidebar()
	beforeCursor := m.cursor
	beforeSelected := m.Selected()
	// CRDs aren't in the default registry yet — sidebar shouldn't move.
	m.SetSelected(k8s.ResourceType("definitely-not-a-real-kind"))
	if m.cursor != beforeCursor {
		t.Errorf("cursor moved on missing type: before=%d, after=%d", beforeCursor, m.cursor)
	}
	if m.Selected() != beforeSelected {
		t.Errorf("selected changed on missing type: before=%v, after=%v", beforeSelected, m.Selected())
	}
}
