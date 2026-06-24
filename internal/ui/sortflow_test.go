package ui

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vulcanshen/km8/internal/config"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// appWithCfg builds the smaller fixture the sort tests need: items
// already in panel-2 + a Config we can write through. k8sClient is
// nil because every sort helper goes through sortRegistry() →
// k8s.DefaultRegistry, which is populated by builtins.go's init.
func appWithCfg(items []k8s.ResourceItem, cfg *config.Config) AppModel {
	th := theme.DefaultTheme()
	tbl := NewTableModel(th)
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = it.Row
	}
	tbl.SetRows(rows)
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
		appLog:          NewAppLogModel(th),
		listPicker:      NewListPickerModel(th),
		currentResource: k8s.ResourcePods,
		cfg:             cfg,
	}
}

func makePod(name string, restarts int32) k8s.ResourceItem {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.Now()},
	}
	return k8s.ResourceItem{
		Name: name,
		UID:  "uid-" + name,
		Row:  []string{name, "1/1", "Running", itoa(int(restarts)), "1h", "10.0.0.1", "node-a"},
		Raw:  pod,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func TestApplySortToItems_NoConfig_FallsBackToNameAsc(t *testing.T) {
	// No sort config for the kind → fall back to metadata.name
	// ascending. Covers the Unset path: clearing a saved sort must
	// reshape items immediately, not preserve the previous sort
	// order until the next kind switch.
	items := []k8s.ResourceItem{makePod("zzz", 5), makePod("aaa", 0), makePod("mmm", 2)}
	m := appWithCfg(items, config.DefaultConfig())
	m.applySortToItems()
	want := []string{"aaa", "mmm", "zzz"}
	for i, w := range want {
		if m.items[i].Name != w {
			t.Errorf("no-config fallback items[%d] = %q, want %q (Name asc)", i, m.items[i].Name, w)
		}
	}
}

func TestApplySortToItems_NameAscending_ReordersItems(t *testing.T) {
	items := []k8s.ResourceItem{makePod("zzz", 5), makePod("aaa", 0), makePod("mmm", 2)}
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Name", config.SortDirectionAscending)
	m := appWithCfg(items, cfg)
	m.applySortToItems()
	want := []string{"aaa", "mmm", "zzz"}
	for i, w := range want {
		if m.items[i].Name != w {
			t.Errorf("Name asc items[%d] = %q, want %q", i, m.items[i].Name, w)
		}
	}
}

func TestApplySortToItems_RestartsDescending_UsesIntComparator(t *testing.T) {
	// "Restarts" routed through the int comparator — "10" must rank
	// above "2", not below by string lex sort.
	items := []k8s.ResourceItem{makePod("a", 2), makePod("b", 10), makePod("c", 0)}
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Restarts", config.SortDirectionDescending)
	m := appWithCfg(items, cfg)
	m.applySortToItems()
	want := []string{"b", "a", "c"} // 10, 2, 0
	for i, w := range want {
		if m.items[i].Name != w {
			t.Errorf("Restarts desc items[%d] = %q, want %q", i, m.items[i].Name, w)
		}
	}
}

func TestSyncTableSortIndicator_NoConfig_ClearsIndicator(t *testing.T) {
	items := []k8s.ResourceItem{makePod("a", 0)}
	m := appWithCfg(items, config.DefaultConfig())
	m.table.SetSortIndicator("Age", "desc")
	m.syncTableSortIndicator()
	if m.table.sortColumn != "" || m.table.sortDirection != "" {
		t.Errorf("missing config should clear indicator, got %q/%q", m.table.sortColumn, m.table.sortDirection)
	}
}

func TestSyncTableSortIndicator_PushesSavedSort(t *testing.T) {
	items := []k8s.ResourceItem{makePod("a", 0)}
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Age", config.SortDirectionDescending)
	m := appWithCfg(items, cfg)
	m.syncTableSortIndicator()
	if m.table.sortColumn != "Age" || m.table.sortDirection != "desc" {
		t.Errorf("indicator = %q/%q, want Age/desc", m.table.sortColumn, m.table.sortDirection)
	}
}

func TestCommitSortFlow_PersistsAndApplies(t *testing.T) {
	// Full happy path: flow caches kind+column, commit writes config,
	// re-applies sort to live items, and refreshes the header
	// indicator. XDG redirect MUST come before commit — otherwise
	// the test's cfg.Save() would write to the real ~/.config/km8.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	items := []k8s.ResourceItem{makePod("zzz", 5), makePod("aaa", 0), makePod("mmm", 2)}
	cfg := config.DefaultConfig()
	m := appWithCfg(items, cfg)
	m.sortFlowKind = k8s.ResourcePods
	m.sortFlowColumn = "Name"

	// Direction commit (Ascending). The picker close cmd is a no-op
	// when the picker was never opened — that's fine for this test.
	_ = m.commitSortFlow(config.SortDirectionAscending)

	if got := cfg.GetSort("pod"); got == nil || got.Column != "Name" || got.Direction != "asc" {
		t.Errorf("persisted sort = %+v, want {Column:Name Direction:asc}", got)
	}
	if m.items[0].Name != "aaa" {
		t.Errorf("items not re-sorted on commit, first = %q, want aaa", m.items[0].Name)
	}
	if m.table.sortColumn != "Name" || m.table.sortDirection != "asc" {
		t.Errorf("table indicator not updated: %q/%q", m.table.sortColumn, m.table.sortDirection)
	}
	if m.sortFlowKind != "" || m.sortFlowColumn != "" {
		t.Errorf("flow state must reset after commit, got kind=%q column=%q", m.sortFlowKind, m.sortFlowColumn)
	}
}

func TestCommitSortFlow_UnsetOnUnrelatedColumn_NoOp(t *testing.T) {
	// User selects "Unset" on a column that wasn't the sorted one →
	// must NOT clobber the existing sort. The design says "Unset on
	// never-sorted column = no-op", and "never-sorted" includes
	// "not the column currently sorted".
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Age", config.SortDirectionDescending)
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	m.sortFlowKind = k8s.ResourcePods
	m.sortFlowColumn = "Name" // a column that ISN'T currently sorted

	_ = m.commitSortFlow("unset")

	got := cfg.GetSort("pod")
	if got == nil || got.Column != "Age" || got.Direction != "desc" {
		t.Errorf("unrelated-column Unset must preserve sort, got %+v", got)
	}
}

func TestCommitSortFlow_UnsetOnCurrentColumn_ClearsSort(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Age", config.SortDirectionDescending)
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	m.sortFlowKind = k8s.ResourcePods
	m.sortFlowColumn = "Age"

	_ = m.commitSortFlow("unset")

	if got := cfg.GetSort("pod"); got != nil {
		t.Errorf("Unset on current column must clear sort, got %+v", got)
	}
	if m.table.sortColumn != "" {
		t.Errorf("table indicator must clear on Unset, got %q", m.table.sortColumn)
	}
}

func TestCommitSortFlow_UnsetImmediatelyRevertsToNameAsc(t *testing.T) {
	// Regression: previously, Unset cleared the config but left
	// m.items in their last sorted order — user had to switch
	// kinds in panel 1 for the fallback to kick in. The fix is in
	// applySortToItems falling back to Name asc; this test guards
	// the end-to-end behaviour via commitSortFlow.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	items := []k8s.ResourceItem{makePod("zzz", 9), makePod("aaa", 1), makePod("mmm", 5)}
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Restarts", config.SortDirectionDescending)
	m := appWithCfg(items, cfg)
	// Simulate the initial sort having been applied (Restarts desc → zzz, mmm, aaa).
	m.applySortToItems()
	if m.items[0].Name != "zzz" {
		t.Fatalf("setup: Restarts desc should put zzz first, got %q", m.items[0].Name)
	}

	// User triggers Unset on the currently-sorted column.
	m.sortFlowKind = k8s.ResourcePods
	m.sortFlowColumn = "Restarts"
	_ = m.commitSortFlow("unset")

	// After Unset: items must be in Name asc fallback order
	// immediately, without waiting for a kind switch.
	want := []string{"aaa", "mmm", "zzz"}
	for i, w := range want {
		if m.items[i].Name != w {
			t.Errorf("post-Unset items[%d] = %q, want %q (Name asc fallback)", i, m.items[i].Name, w)
		}
	}
}

func TestTableModel_columnTitles_RendersArrowOnSortedColumn(t *testing.T) {
	th := theme.DefaultTheme()
	m := NewTableModel(th)
	m.columns = []Column{{Title: "Name"}, {Title: "Ready"}, {Title: "Age"}}
	m.SetSortIndicator("Age", "desc")
	got := m.columnTitles()
	if got[0] != "Name" {
		t.Errorf("non-sorted column got %q, want plain title", got[0])
	}
	if !strings.HasPrefix(got[2], "Age ") {
		t.Errorf("sorted column %q should be 'Age ' + glyph", got[2])
	}
	if len(got[2]) <= len("Age ") {
		t.Errorf("sorted column %q missing glyph", got[2])
	}
	m.SetSortIndicator("", "")
	got = m.columnTitles()
	if got[2] != "Age" {
		t.Errorf("after clear indicator, Age column = %q, want plain Age", got[2])
	}
}
