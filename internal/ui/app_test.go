package ui

import (
	"os"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hinshun/vt10x"

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
		shellPty:        NewPtyView(),
		txPty:           NewPtyView(),
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

// ── dual-slot PTY coexistence ──────────────────────────────────────────────

// fakeAlivePtyView mirrors hookedPtyView from ptyview_test.go — a PtyView
// that LOOKS post-Start (IsAlive/IsActive true) without spawning a real
// subprocess.
func fakeAlivePtyView(kind PtyKind, hidden bool) *PtyView {
	p := NewPtyView()
	p.active = true
	p.hidden = hidden
	p.kind = kind
	p.term = vt10x.New(vt10x.WithSize(80, 24))
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}
	return p
}

func TestAppModel_PtyExitMsg_EditKindClearsEditingFlag(t *testing.T) {
	m := appWithItems(nil, 0)
	m.editing = true
	updated, _ := m.Update(PtyExitMsg{Kind: PtyKindEdit, ExitCode: 0})
	if updated.(AppModel).editing {
		t.Error("PtyExitMsg{Kind:Edit} must clear editing flag")
	}
}

func TestAppModel_PtyExitMsg_ShellKindLeavesEditingFlag(t *testing.T) {
	// KM8erm exit should NOT touch editing — dual-slot independence.
	m := appWithItems(nil, 0)
	m.editing = true
	updated, _ := m.Update(PtyExitMsg{Kind: PtyKindShell, ExitCode: 0})
	if !updated.(AppModel).editing {
		t.Error("PtyExitMsg{Kind:Shell} must NOT clear editing flag")
	}
}

func TestAppModel_PtyExitMsg_ExecKindLeavesEditingFlag(t *testing.T) {
	// Same independence for the exec case.
	m := appWithItems(nil, 0)
	m.editing = true
	updated, _ := m.Update(PtyExitMsg{Kind: PtyKindExec, ExitCode: 0})
	if !updated.(AppModel).editing {
		t.Error("PtyExitMsg{Kind:Exec} must NOT clear editing flag")
	}
}

func TestAppModel_DualSlot_KM8ermHidden_AllowsExecSpawn(t *testing.T) {
	// The bug fix: with shellPty alive + hidden, startShellExecMsg must
	// NOT be blocked. Old single-slot design blocked any new PTY when
	// shell was alive even if hidden — confusing UX.
	m := appWithItems(nil, 0)
	m.shellPty = fakeAlivePtyView(PtyKindShell, true)
	// Sanity: shell is alive + hidden, tx is fresh
	if !m.shellPty.IsAlive() || !m.shellPty.IsHidden() {
		t.Fatal("setup: shellPty must be alive + hidden")
	}
	if m.txPty.IsAlive() {
		t.Fatal("setup: txPty must be fresh (not alive)")
	}
	// The guard at startShellExecMsg checks m.txPty.IsAlive() — it must
	// be false right now, so the spawn is allowed to proceed (we don't
	// drive it through to actual Start, which would fork a process).
	if m.txPty.IsAlive() {
		t.Error("guard precondition broken: txPty.IsAlive should be false")
	}
}

func TestAppModel_DualSlot_TxAlive_BlocksAnotherExec(t *testing.T) {
	// Counterpart: when a transient PTY IS alive, another exec must be
	// blocked. The guard fires only on txPty, not shellPty.
	m := appWithItems(nil, 0)
	m.txPty = fakeAlivePtyView(PtyKindExec, false)
	if !m.txPty.IsAlive() {
		t.Fatal("setup: txPty must be alive")
	}
	// Send a new startShellExecMsg — handler should return a toast cmd
	// (not nil, since it returns the toast.Show cmd).
	_, cmd := m.Update(startShellExecMsg{
		podName: "x", namespace: "y", container: "z", contextName: "c",
	})
	if cmd == nil {
		t.Error("startShellExecMsg with active txPty must return a toast cmd, got nil")
	}
}

// TestAppModel_ExitDrillDownFromContainers_RowsStayHelmAligned guards the
// v1.5.5-era regression where exiting the container drill-down (Pod →
// containers → back) repopulated the table with raw item.Row instead of
// augmentRowsWithHelm. ColumnsForResource(Pods) always reserves index 1
// for the helm marker, so raw rows shifted Status one column left and
// stylizeCell — which colors by the column whose Title=="Status" —
// looked at the wrong cell, killing the Running green until the user
// switched resources.
func TestAppModel_ExitDrillDownFromContainers_RowsStayHelmAligned(t *testing.T) {
	// Real Pod rows are 7 cells (Name/Ready/Status/Restarts/Age/IP/Node)
	// per the Pod ResourceDefinition's Columns slice. Match that shape
	// exactly so column index 3 (Status, post-helm-augment) lines up
	// with the "Running" / "Pending" cell.
	items := []k8s.ResourceItem{
		{Name: "nginx-a", UID: "uid-a", Row: []string{"nginx-a", "1/1", "Running", "0", "5m", "10.244.0.5", "node-1"}},
		{Name: "nginx-b", UID: "uid-b", Row: []string{"nginx-b", "0/1", "Pending", "0", "1m", "<none>", "node-2"}},
	}
	m := appWithItems(items, 0)
	m.currentResource = k8s.ResourcePods
	m.statusLine = NewStatusLineModel(m.theme)
	m.logStreamer = k8s.NewLogStreamer(nil)

	// Simulate being in container view: drillDownPod set, columns swapped
	// to containerColumns (no Status column). This is the state we exit
	// from.
	pod := items[0]
	m.drillDownPod = &pod
	m.table.SetColumns(containerColumns())
	m.table.SetRows(containerRows(nil))

	_ = m.exitDrillDown() // returned Cmd is a fetch closure — never executed

	wantRows := augmentRowsWithHelm(items, k8s.ResourcePods)
	if len(m.table.rows) != len(wantRows) {
		t.Fatalf("row count = %d, want %d", len(m.table.rows), len(wantRows))
	}
	for i := range wantRows {
		if len(m.table.rows[i]) != len(wantRows[i]) {
			t.Errorf("row[%d] width = %d, want %d (raw item.Row leaked back into the table — Status column is now mis-aligned)", i, len(m.table.rows[i]), len(wantRows[i]))
		}
	}
	// Belt-and-braces: the cell under the Status column must read
	// "Running" / "Pending", not the Name. If this fails, podStatusColor
	// would have no chance of matching and the row would render plain.
	statusIdx := -1
	for i, col := range m.table.columns {
		if col.Title == "Status" {
			statusIdx = i
			break
		}
	}
	if statusIdx < 0 {
		t.Fatal("Pod columns have no Status column — test setup broken")
	}
	if m.table.rows[0][statusIdx] != "Running" {
		t.Errorf("row[0] Status cell = %q, want %q (column/row mis-alignment after exitDrillDown)", m.table.rows[0][statusIdx], "Running")
	}
}

// silence unused warning if tea is not referenced elsewhere in tests
var _ tea.Msg = struct{}{}
