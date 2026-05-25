package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func TestBuildShellTerminalCmd_UsesShellEnv(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Setenv("SHELL", "/usr/bin/fish")
	cmd := buildShellTerminalCmd()
	if cmd == nil {
		t.Fatal("buildShellTerminalCmd returned nil")
	}
	if cmd.Args[0] != "/usr/bin/fish" {
		t.Errorf("expected /usr/bin/fish, got %q", cmd.Args[0])
	}
	if len(cmd.Args) < 2 || cmd.Args[1] != "-l" {
		t.Errorf("expected login shell flag, got args %v", cmd.Args)
	}
}

func TestBuildShellTerminalCmd_FallbackWhenShellUnset(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Unsetenv("SHELL")
	cmd := buildShellTerminalCmd()
	if cmd.Args[0] != "/bin/sh" {
		t.Errorf("expected /bin/sh fallback, got %q", cmd.Args[0])
	}
}

func TestTerminalTitle_PrefixAndSuffix(t *testing.T) {
	title := terminalTitle()
	if !strings.Contains(title, "KM8erm") {
		t.Errorf("title must contain 'KM8erm' marker, got %q", title)
	}
	// We deliberately pass mDNS suffixes through (`.local`, `.home`, ...)
	// because the user wants the raw hostname. No suffix assertion.
}

// appWithItems builds a minimal AppModel for currentItemUID + ResourceDetailMsg
// testing. NewAppModel needs a real k8s.Client; for these tests we only need
// the items list, table cursor, and a detail model — wire those by hand.
func appWithItems(items []k8s.ResourceItem, cursor int) AppModel {
	th := theme.DefaultTheme()
	tbl := NewTableModel(th)
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = it.Row
	}
	tbl.SetRows(rows)
	tbl.cursor = cursor
	d := NewDetailModel(th)
	d.SetResourceType(k8s.ResourcePods)
	return AppModel{
		items:           items,
		table:           tbl,
		detail:          d,
		theme:           th,
		ptyView:         NewPtyView(), // AppModel.Update dereferences ptyView early
		toast:           NewToastModel(th),
		breadcrumbPopup: NewBreadcrumbPopupModel(th),
	}
}

func TestAppModel_CurrentItemUID(t *testing.T) {
	items := []k8s.ResourceItem{
		{Name: "a", UID: "uid-a", Row: []string{"a"}},
		{Name: "b", UID: "uid-b", Row: []string{"b"}},
		{Name: "c", UID: "uid-c", Row: []string{"c"}},
	}

	cases := []struct {
		name    string
		items   []k8s.ResourceItem
		cursor  int
		wantUID string
	}{
		{"first row", items, 0, "uid-a"},
		{"middle row", items, 1, "uid-b"},
		{"last row", items, 2, "uid-c"},
		{"empty list", nil, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := appWithItems(tc.items, tc.cursor)
			if got := m.currentItemUID(); got != tc.wantUID {
				t.Errorf("currentItemUID() = %q, want %q", got, tc.wantUID)
			}
		})
	}
}

// TestAppModel_ResourceDetailMsg_DropsStale verifies the UID guard rejects
// a fetch result whose ItemUID doesn't match the currently selected row.
// Without this, a slow EnrichRelatives (e.g. ClusterRole's two cluster-wide
// List calls) finishing after the user moved on would overwrite the
// freshly displayed detail.
func TestAppModel_ResourceDetailMsg_DropsStale(t *testing.T) {
	items := []k8s.ResourceItem{
		{Name: "a", UID: "uid-a", Row: []string{"a"}},
		{Name: "b", UID: "uid-b", Row: []string{"b"}},
	}
	m := appWithItems(items, 1) // cursor on "b"

	// Stale: msg for "a" while cursor is on "b" — must be dropped.
	stale := ResourceDetailMsg{
		ItemUID: "uid-a",
		Detail:  k8s.ResourceDetail{Name: "a", Kind: "Pod"},
	}
	updated, _ := m.Update(stale)
	got := updated.(AppModel)
	if got.detail.hasData {
		t.Errorf("stale msg must not populate detail")
	}

	// Fresh: msg for the currently selected row — applies.
	fresh := ResourceDetailMsg{
		ItemUID: "uid-b",
		Detail:  k8s.ResourceDetail{Name: "b", Kind: "Pod"},
	}
	updated2, _ := got.Update(fresh)
	got2 := updated2.(AppModel)
	if !got2.detail.hasData {
		t.Errorf("matching msg must populate detail")
	}
}

// TestAppModel_LinkPushMsg_CycleBlocked verifies that drilling into a
// resource already on the chain is blocked with a toast — prevents
// infinite-loop navigation like Pod -> ConfigMap -> Pod (same pod).
func TestAppModel_LinkPushMsg_CycleBlocked(t *testing.T) {
	items := []k8s.ResourceItem{
		{Name: "nginx-x", UID: "uid-pod", Namespace: "default", Row: []string{"nginx-x"}},
	}
	m := appWithItems(items, 0)
	m.currentResource = k8s.ResourcePods
	// Seed root detail so RootRef returns a meaningful ref.
	m.detail.SetDetail(k8s.ResourceDetail{Name: "nginx-x", Namespace: "default", Kind: "Pod"}, nil)

	// Drilling into the SAME root pod is a cycle.
	cycleMsg := RelativePushMsg{Ref: k8s.RefTarget{Type: k8s.ResourcePods, Name: "nginx-x", Namespace: "default"}}
	updated, cmd := m.Update(cycleMsg)
	got := updated.(AppModel)
	if got.detail.Depth() != 1 {
		t.Errorf("cycle should not push a frame, depth=%d", got.detail.Depth())
	}
	if cmd == nil {
		t.Fatal("cycle should return a toast Cmd")
	}
	// Execute the Cmd to verify it's the toast (toast.Show returns a Cmd).
	if msg := cmd(); msg == nil {
		t.Errorf("cycle toast Cmd returned nil")
	}
}

// TestAppModel_linkDrillFetchedMsg_PushesFrame verifies that a successful
// drill fetch lands on detail.drillStack and depth increases.
func TestAppModel_linkDrillFetchedMsg_PushesFrame(t *testing.T) {
	items := []k8s.ResourceItem{
		{Name: "nginx-x", UID: "uid-pod", Namespace: "default", Row: []string{"nginx-x"}},
	}
	m := appWithItems(items, 0)
	m.detail.SetDetail(k8s.ResourceDetail{Name: "nginx-x", Namespace: "default", Kind: "Pod"}, nil)

	msg := relativeDrillFetchedMsg{
		ref:       k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx", Namespace: "default"},
		sourceUID: "uid-pod",
		item:      k8s.ResourceItem{Name: "nginx", Namespace: "default", UID: "uid-dep"},
		detail:    k8s.ResourceDetail{Name: "nginx", Namespace: "default", Kind: "Deployment"},
	}
	updated, _ := m.Update(msg)
	got := updated.(AppModel)
	if got.detail.Depth() != 2 {
		t.Errorf("expected depth 2 after push, got %d", got.detail.Depth())
	}
}

// TestAppModel_linkDrillFetchedMsg_StaleDrop verifies that drill results
// whose sourceUID doesn't match the currently selected table row get
// dropped (user moved on while the fetch was in flight).
func TestAppModel_linkDrillFetchedMsg_StaleDrop(t *testing.T) {
	items := []k8s.ResourceItem{
		{Name: "nginx-x", UID: "uid-pod", Namespace: "default", Row: []string{"nginx-x"}},
	}
	m := appWithItems(items, 0)
	m.detail.SetDetail(k8s.ResourceDetail{Name: "nginx-x", Namespace: "default", Kind: "Pod"}, nil)

	// sourceUID points at a different pod the user has since left.
	msg := relativeDrillFetchedMsg{
		ref:       k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx"},
		sourceUID: "uid-stale",
		item:      k8s.ResourceItem{Name: "nginx", UID: "uid-dep"},
		detail:    k8s.ResourceDetail{Name: "nginx", Kind: "Deployment"},
	}
	updated, _ := m.Update(msg)
	if updated.(AppModel).detail.Depth() != 1 {
		t.Errorf("stale drill result must not push, depth=%d", updated.(AppModel).detail.Depth())
	}
}

// TestAppModel_ResourceDetailMsg_DropsWhenNoSelection verifies that
// detail-fetch results arriving after the items list was cleared
// (namespace/context change) get dropped rather than populating the
// freshly cleared panel.
func TestAppModel_ResourceDetailMsg_DropsWhenNoSelection(t *testing.T) {
	m := appWithItems(nil, 0)
	msg := ResourceDetailMsg{
		ItemUID: "uid-a",
		Detail:  k8s.ResourceDetail{Name: "a", Kind: "Pod"},
	}
	updated, _ := m.Update(msg)
	if updated.(AppModel).detail.hasData {
		t.Errorf("msg with no selection must not populate detail")
	}
}

// TestAppModel_SwitchToResourceMsg_RoutesSidebarAndPending verifies the
// confirmed Relatives-tab jump: sidebar selection updates synchronously,
// pendingTableSelect captures the target ref for the next ResourceDataMsg,
// and the cmd returned re-emits ResourceSelectedMsg so the watcher restarts.
func TestAppModel_SwitchToResourceMsg_RoutesSidebarAndPending(t *testing.T) {
	m := appWithItems(nil, 0)
	m.sidebar = NewSidebarModel(theme.DefaultTheme())
	target := k8s.RefTarget{Type: k8s.ResourceServices, Name: "svc-a", Namespace: "ns-a"}

	updated, cmd := m.Update(SwitchToResourceMsg{Ref: target})
	got := updated.(AppModel)

	if got.sidebar.Selected() != k8s.ResourceServices {
		t.Errorf("sidebar.Selected() = %v, want ResourceServices", got.sidebar.Selected())
	}
	if got.pendingTableSelect == nil || *got.pendingTableSelect != target {
		t.Errorf("pendingTableSelect = %+v, want %+v", got.pendingTableSelect, target)
	}
	if cmd == nil {
		t.Fatal("expected cmd to re-emit ResourceSelectedMsg, got nil")
	}
	rsm, ok := cmd().(ResourceSelectedMsg)
	if !ok {
		t.Fatalf("cmd output type = %T, want ResourceSelectedMsg", cmd())
	}
	if rsm.Type != k8s.ResourceServices {
		t.Errorf("ResourceSelectedMsg.Type = %v, want ResourceServices", rsm.Type)
	}
}

// TestAppModel_SwitchToResourceMsg_ClearsSearchFilters guards against a
// stale search filter hiding the freshly switched-to resource. Before
// the fix, an active sidebar filter like "svc" could survive the switch
// and leave the new resource type invisible in panel 1. Detail-panel
// search was removed in the post-v1.5 panel-3 simplification, so only
// the sidebar still has search state to seed/assert.
func TestAppModel_SwitchToResourceMsg_ClearsSearchFilters(t *testing.T) {
	m := appWithItems(nil, 0)
	m.sidebar = NewSidebarModel(theme.DefaultTheme())
	m.sidebar.searchQuery = "stale-sidebar-filter"
	m.sidebar.searching = true

	target := k8s.RefTarget{Type: k8s.ResourceServices, Name: "svc-a", Namespace: "ns-a"}
	updated, _ := m.Update(SwitchToResourceMsg{Ref: target})
	got := updated.(AppModel)

	if got.sidebar.searchQuery != "" || got.sidebar.searching {
		t.Errorf("sidebar search must be cleared, got query=%q searching=%v",
			got.sidebar.searchQuery, got.sidebar.searching)
	}
	// Table search clears via the chained ResourceSelectedMsg handler —
	// not directly here. Covered by table's own ResourceSelectedMsg tests.
}

// TestAppModel_HonorPendingTableSelect_FindsRow verifies the cursor-snap
// hook used by the Relatives-tab space hotkey: when items for the matching
// kind arrive, cursor jumps to the row whose name+namespace matches and
// pending clears.
func TestAppModel_HonorPendingTableSelect_FindsRow(t *testing.T) {
	m := appWithItems(nil, 0)
	target := k8s.RefTarget{Type: k8s.ResourceServices, Name: "svc-b", Namespace: "ns-a"}
	m.pendingTableSelect = &target

	items := []k8s.ResourceItem{
		{Name: "svc-a", Namespace: "ns-a", UID: "u-a", Row: []string{"svc-a"}},
		{Name: "svc-b", Namespace: "ns-a", UID: "u-b", Row: []string{"svc-b"}},
		{Name: "svc-c", Namespace: "ns-a", UID: "u-c", Row: []string{"svc-c"}},
	}
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = it.Row
	}
	m.table.SetRows(rows)
	m.honorPendingTableSelect(k8s.ResourceServices, items)

	if m.table.SelectedRow() != 1 {
		t.Errorf("table cursor = %d, want 1 (svc-b)", m.table.SelectedRow())
	}
	if m.pendingTableSelect != nil {
		t.Errorf("pendingTableSelect should clear after honoring, got %+v", m.pendingTableSelect)
	}
}

// TestAppModel_HonorPendingTableSelect_MissingTargetClears verifies that
// when the requested resource isn't in the result set (different namespace
// scope, drifted away), cursor stays put and pending clears (so we don't
// keep hunting on every subsequent watcher tick).
func TestAppModel_HonorPendingTableSelect_MissingTargetClears(t *testing.T) {
	m := appWithItems(nil, 0)
	missing := k8s.RefTarget{Type: k8s.ResourceServices, Name: "not-here", Namespace: "ns-x"}
	m.pendingTableSelect = &missing

	items := []k8s.ResourceItem{
		{Name: "svc-a", Namespace: "ns-a", UID: "u-a", Row: []string{"svc-a"}},
	}
	m.table.SetRows([][]string{{"svc-a"}})
	m.honorPendingTableSelect(k8s.ResourceServices, items)

	if m.table.SelectedRow() != 0 {
		t.Errorf("missing target: cursor should stay at 0, got %d", m.table.SelectedRow())
	}
	if m.pendingTableSelect != nil {
		t.Errorf("pendingTableSelect should clear even on miss, got %+v", m.pendingTableSelect)
	}
}

// TestAppModel_HonorPendingTableSelect_KindMismatchSkips verifies that
// data arriving for an unrelated kind doesn't consume the pending
// pointer — it must survive for the correct ResourceDataMsg.
func TestAppModel_HonorPendingTableSelect_KindMismatchSkips(t *testing.T) {
	m := appWithItems(nil, 0)
	target := k8s.RefTarget{Type: k8s.ResourceServices, Name: "svc-b", Namespace: "ns-a"}
	m.pendingTableSelect = &target

	m.honorPendingTableSelect(k8s.ResourcePods, []k8s.ResourceItem{})

	if m.pendingTableSelect == nil {
		t.Error("pendingTableSelect must survive a kind-mismatched data arrival")
	}
}
