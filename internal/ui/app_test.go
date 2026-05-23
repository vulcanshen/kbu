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
// Without this, a slow EnrichLinks (e.g. ClusterRole's two cluster-wide
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
	cycleMsg := LinkPushMsg{Ref: k8s.RefTarget{Type: k8s.ResourcePods, Name: "nginx-x", Namespace: "default"}}
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

	msg := linkDrillFetchedMsg{
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
	msg := linkDrillFetchedMsg{
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
