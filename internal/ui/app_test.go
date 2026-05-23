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
		items:   items,
		table:   tbl,
		detail:  d,
		theme:   th,
		ptyView: NewPtyView(), // AppModel.Update dereferences ptyView early
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
