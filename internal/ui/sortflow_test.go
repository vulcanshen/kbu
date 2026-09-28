package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
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
		shellPty:        NewPtyView("ptyview_shell"),
		txPty:           NewPtyView("ptyview_tx"),
		toast:           NewToastModel(th),
		breadcrumbPopup: NewBreadcrumbPopupModel(th),
		appLog:          NewAppLogModel(th),
		listPicker:      NewListPickerModel(th),
		spaceMenu:       NewSpaceMenuModel(th),
		globalMenu:      NewGlobalMenuModel(th),
		sidebar:         NewSidebarModel(th),
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
	// No sort config for the kind, all items in the same namespace →
	// the (namespace, name) fallback degenerates to pure Name asc.
	// Covers the Unset path: clearing a saved sort must reshape items
	// immediately, not preserve the previous sort order until the
	// next kind switch.
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

func TestApplySortToItems_NoConfig_GroupsByNamespaceThenName(t *testing.T) {
	// kubectl's default ordering for cross-namespace lists is
	// (namespace, name). Pure name asc would scatter items across
	// namespaces; users called this out as feeling wrong.
	mk := func(ns, name string) k8s.ResourceItem {
		return k8s.ResourceItem{
			Name:      name,
			Namespace: ns,
			UID:       ns + "/" + name,
			Row:       []string{name},
		}
	}
	items := []k8s.ResourceItem{
		mk("monitoring", "grafana"),
		mk("default", "api-2"),
		mk("default", "api-1"),
		mk("monitoring", "alertmanager"),
	}
	m := appWithCfg(items, config.DefaultConfig())
	m.applySortToItems()
	want := []struct{ ns, name string }{
		{"default", "api-1"},
		{"default", "api-2"},
		{"monitoring", "alertmanager"},
		{"monitoring", "grafana"},
	}
	for i, w := range want {
		if m.items[i].Namespace != w.ns || m.items[i].Name != w.name {
			t.Errorf("fallback[%d] = %s/%s, want %s/%s", i, m.items[i].Namespace, m.items[i].Name, w.ns, w.name)
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

// tdp M6 + F7: with no sort yet, Reset is still listed — dimmed — so the
// picker opens at the height it keeps once the first tier lands (it used
// to grow a Reset row while open).
func TestOpenSortColumnPicker_EmptyChainListsResetDimmed(t *testing.T) {
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortColumnPicker(k8s.ResourcePods)

	var reset *ListPickerItem
	for i, it := range m.listPicker.items {
		if it.Key == sortResetKey {
			reset = &m.listPicker.items[i]
		}
	}
	if reset == nil {
		t.Fatal("Reset must be listed even with no sort")
	}
	if !reset.Disabled {
		t.Error("Reset must be dimmed while there is nothing to reset")
	}
}

func TestOpenSortColumnPicker_NonEmptyChainEmitsRegionHeaders(t *testing.T) {
	// Chain present → two regions split by a separator, each
	// prefixed with its lowercase header ("fields", "all").
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Name", config.SortDirectionAscending)
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortColumnPicker(k8s.ResourcePods)

	var fieldsIdx, sepIdx, allIdx, resetIdx int = -1, -1, -1, -1
	for i, it := range m.listPicker.items {
		switch {
		case it.Header && it.Label == "fields":
			fieldsIdx = i
		case it.Separator:
			sepIdx = i
		case it.Header && it.Label == "all":
			allIdx = i
		case it.Key == sortResetKey:
			resetIdx = i
		}
	}
	if fieldsIdx < 0 || sepIdx < 0 || allIdx < 0 || resetIdx < 0 {
		t.Fatalf("expected fields header, separator, all header, reset; got items=%v", m.listPicker.items)
	}
	if !(fieldsIdx < sepIdx && sepIdx < allIdx && allIdx < resetIdx) {
		t.Errorf("expected order fields(%d) < sep(%d) < all(%d) < reset(%d)", fieldsIdx, sepIdx, allIdx, resetIdx)
	}
}

func TestOpenSortColumnPicker_TitleHasSortIcon(t *testing.T) {
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortColumnPicker(k8s.ResourcePods)
	if !strings.HasPrefix(m.listPicker.title, sortPopupIcon) {
		t.Errorf("title should start with sortPopupIcon, got %q", m.listPicker.title)
	}
}

func TestOpenSortDirectionPicker_TitleHasSortIcon(t *testing.T) {
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortDirectionPicker(k8s.ResourcePods, "Name")
	if !strings.HasPrefix(m.sortDirPicker.title, sortPopupIcon) {
		t.Errorf("direction picker title should start with sortPopupIcon, got %q", m.sortDirPicker.title)
	}
}

func TestOpenSortColumnPicker_SingleTierBadgeOmitsPriority(t *testing.T) {
	// Single-tier chain: picker badge is just the arrow ("↑" / "↓"),
	// no "(1)" — mirrors the table header collapse rule so picker
	// and header read the same way.
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Name", config.SortDirectionAscending)
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortColumnPicker(k8s.ResourcePods)

	for _, it := range m.listPicker.items {
		if it.Key == "Name" {
			if strings.Contains(it.Badge, "(") {
				t.Errorf("single-tier badge must omit priority paren, got %q", it.Badge)
			}
			if it.Badge == "" {
				t.Errorf("single-tier badge must still show arrow, got empty")
			}
			return
		}
	}
	t.Error("Name row not found in picker items")
}

func TestOpenSortColumnPicker_MultiTierBadgeShowsPriority(t *testing.T) {
	// Multi-tier chain: badges show "(N) ↑/↓" so user can read the
	// tier order off the picker without consulting the header.
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Name", config.SortDirectionAscending)
	cfg.SetSort("pod", "Age", config.SortDirectionDescending)
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortColumnPicker(k8s.ResourcePods)

	got := map[string]string{}
	for _, it := range m.listPicker.items {
		if it.Key == "Name" || it.Key == "Age" {
			got[it.Key] = it.Badge
		}
	}
	if !strings.HasPrefix(got["Name"], "(1)") {
		t.Errorf("primary tier (Name) badge should start with (1), got %q", got["Name"])
	}
	if !strings.HasPrefix(got["Age"], "(2)") {
		t.Errorf("secondary tier (Age) badge should start with (2), got %q", got["Age"])
	}
}

func TestAltSHotkey_DrillDown_NoOp(t *testing.T) {
	// Container drill view: Alt+Shift+S must be a silent no-op so
	// the user doesn't see a "Sort Pods by…" picker while viewing
	// containers. Matches E/D/C drill-mode gating.
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	m.activePanel = TablePanel
	pod := k8s.ResourceItem{Name: "p", Namespace: "default"}
	m.drillDownPod = &pod

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}, Alt: true})

	if m.listPicker.IsActive() {
		t.Error("Alt+S during container drill must be no-op; picker became active")
	}
	if cmd != nil {
		t.Errorf("Alt+S during drill must return nil cmd, got %T (msg=%v)", cmd, cmd())
	}
}

func TestOpenSortDirectionPicker_OmitsUnsetForNewColumn(t *testing.T) {
	// Picking a column that's NOT in the chain → direction picker
	// should only offer Ascending / Descending (no Unset, because
	// unsetting a never-sorted column is a guaranteed no-op and
	// surfacing it just clutters the picker).
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortDirectionPicker(k8s.ResourcePods, "Name")

	for _, it := range m.sortDirPicker.items {
		if it.Key == "unset" {
			t.Errorf("Unset must be hidden for a column not in chain; items=%v", m.sortDirPicker.items)
		}
	}
}

func TestOpenSortDirectionPicker_ShowsUnsetForInChainColumn(t *testing.T) {
	// Picking a column already in the chain → Unset surfaces so the
	// user can drop that tier. Matches the column step's Reset row,
	// which only shows when there's something to reset.
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Name", config.SortDirectionAscending)
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortDirectionPicker(k8s.ResourcePods, "Name")

	found := false
	for _, it := range m.sortDirPicker.items {
		if it.Key == "unset" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Unset must surface when column is in chain; items=%v", m.sortDirPicker.items)
	}
}

func TestSyncTableSortIndicator_NoConfig_ClearsIndicator(t *testing.T) {
	items := []k8s.ResourceItem{makePod("a", 0)}
	m := appWithCfg(items, config.DefaultConfig())
	m.table.SetSortIndicators([]config.SortConfig{{Column: "Age", Direction: "desc"}})
	m.syncTableSortIndicator()
	if len(m.table.sortChain) != 0 {
		t.Errorf("missing config should clear chain, got %+v", m.table.sortChain)
	}
}

func TestSyncTableSortIndicator_PushesSavedSort(t *testing.T) {
	items := []k8s.ResourceItem{makePod("a", 0)}
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Age", config.SortDirectionDescending)
	m := appWithCfg(items, cfg)
	m.syncTableSortIndicator()
	if len(m.table.sortChain) != 1 ||
		m.table.sortChain[0].Column != "Age" ||
		m.table.sortChain[0].Direction != "desc" {
		t.Errorf("chain = %+v, want [{Age desc}]", m.table.sortChain)
	}
}

func TestCommitSortFlow_PersistsAndApplies(t *testing.T) {
	// Full happy path: flow caches kind+column, commit writes config,
	// re-applies sort to live items, and refreshes the header
	// indicator. XDG redirect MUST come before commit — otherwise
	// the test's cfg.Save() would write to the real ~/.config/kbu.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	items := []k8s.ResourceItem{makePod("zzz", 5), makePod("aaa", 0), makePod("mmm", 2)}
	cfg := config.DefaultConfig()
	m := appWithCfg(items, cfg)
	m.sortFlowKind = k8s.ResourcePods
	m.sortFlowColumn = "Name"

	// Direction commit (Ascending). The picker close cmd is a no-op
	// when the picker was never opened — that's fine for this test.
	_ = m.commitSortFlow(config.SortDirectionAscending)

	chain := cfg.GetSort("pod")
	if len(chain) != 1 || chain[0].Column != "Name" || chain[0].Direction != "asc" {
		t.Errorf("persisted chain = %+v, want [{Name asc}]", chain)
	}
	if m.items[0].Name != "aaa" {
		t.Errorf("items not re-sorted on commit, first = %q, want aaa", m.items[0].Name)
	}
	if len(m.table.sortChain) != 1 || m.table.sortChain[0].Column != "Name" {
		t.Errorf("table chain not updated: %+v", m.table.sortChain)
	}
	// sortFlowKind stays set across the loop — the picker re-opens
	// at the column step so the user can stack tiers. sortFlowColumn
	// IS cleared (that step's column is consumed). Esc on the
	// re-opened picker clears both via ListPickerCancelMsg.
	if m.sortFlowKind == "" {
		t.Error("sortFlowKind must remain set after commit (column picker loops back open)")
	}
	if m.sortFlowColumn != "" {
		t.Errorf("sortFlowColumn must clear after commit, got %q", m.sortFlowColumn)
	}
}

// tdp F1, F4, F7: the direction is its own popup stacked on the column
// picker. Picking a direction closes that step and refreshes the column
// picker's badges in place; the column picker keeps the height it opened
// with.
func TestSortFlow_DirectionStepStacksAndReturnsToColumns(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortColumnPicker(k8s.ResourcePods)
	m.listPicker.animator.Finalize()
	openHeight := len(m.listPicker.items)

	_ = m.openSortDirectionPicker(k8s.ResourcePods, "Name")
	m.sortDirPicker.animator.Finalize()
	if !m.listPicker.owns() || !m.sortDirPicker.owns() {
		t.Fatal("the direction step must open over the column picker, which stays")
	}

	_ = m.commitSortFlow(config.SortDirectionAscending)
	if m.sortDirPicker.owns() {
		t.Error("committing a direction must close the direction step")
	}
	if !m.listPicker.owns() || m.listPicker.pickerID != "sort:column" {
		t.Error("the column picker must stay open for the next tier")
	}
	if got := len(m.listPicker.items); got != openHeight {
		t.Errorf("column picker rows = %d after a tier landed, opened with %d (height fixed at open)", got, openHeight)
	}
	var badged bool
	for _, it := range m.listPicker.items {
		if it.Key == "Name" && it.Badge != "" {
			badged = true
		}
		if it.Key == sortResetKey && it.Disabled {
			t.Error("Reset must light up once a tier exists")
		}
	}
	if !badged {
		t.Error("the Name column must be badged after the tier landed")
	}
}

// tdp F4: Esc on the direction step goes back to the column step; the
// flow is still on (the chosen column is dropped, the kind is kept).
func TestSortFlow_EscOnDirectionReturnsToColumns(t *testing.T) {
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	_ = m.openSortColumnPicker(k8s.ResourcePods)
	m.listPicker.animator.Finalize()
	_ = m.openSortDirectionPicker(k8s.ResourcePods, "Name")
	m.sortDirPicker.animator.Finalize()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app := updated.(AppModel)
	expectMsg(t, cmd, func(msg tea.Msg) bool {
		if cancel, ok := msg.(ListPickerCancelMsg); ok {
			next, _ := app.Update(cancel)
			app = next.(AppModel)
			return true
		}
		return false
	})
	if app.sortDirPicker.owns() {
		t.Fatal("Esc must close the direction step")
	}
	if !app.listPicker.owns() {
		t.Error("Esc on the direction step must leave the column picker open")
	}
	if app.sortFlowKind != k8s.ResourcePods || app.sortFlowColumn != "" {
		t.Errorf("after Esc on direction: kind=%q column=%q, want kind kept, column dropped", app.sortFlowKind, app.sortFlowColumn)
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
	if len(got) != 1 || got[0].Column != "Age" || got[0].Direction != "desc" {
		t.Errorf("unrelated-column Unset must preserve chain, got %+v", got)
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

	if got := cfg.GetSort("pod"); len(got) != 0 {
		t.Errorf("Unset on sole-tier column must clear chain, got %+v", got)
	}
	if len(m.table.sortChain) != 0 {
		t.Errorf("table chain must clear after Unset, got %+v", m.table.sortChain)
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

func TestResetSortFlow_ClearsChainAndStaysOnColumns(t *testing.T) {
	// Reset: drop the chain and re-apply the Name asc fallback to
	// live items immediately; the column picker stays open for a
	// fresh chain. sortFlowKind stays set; only Esc on the column
	// picker clears it (via ListPickerCancelMsg).
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	items := []k8s.ResourceItem{makePod("zzz", 9), makePod("aaa", 1), makePod("mmm", 5)}
	cfg := config.DefaultConfig()
	cfg.SetSort("pod", "Restarts", config.SortDirectionDescending)
	m := appWithCfg(items, cfg)
	m.applySortToItems()

	m.sortFlowKind = k8s.ResourcePods
	m.sortFlowColumn = "" // column step shortcut: column is never picked
	_ = m.resetSortFlow()

	if got := cfg.GetSort("pod"); len(got) != 0 {
		t.Errorf("Reset must remove the sort entry, got %+v", got)
	}
	if len(m.table.sortChain) != 0 {
		t.Errorf("table chain must clear after Reset, got %+v", m.table.sortChain)
	}
	want := []string{"aaa", "mmm", "zzz"}
	for i, w := range want {
		if m.items[i].Name != w {
			t.Errorf("post-Reset items[%d] = %q, want %q (Name asc fallback)", i, m.items[i].Name, w)
		}
	}
	if m.sortFlowKind == "" {
		t.Error("sortFlowKind must stay set after Reset (loop continues)")
	}
	if m.sortFlowColumn != "" {
		t.Errorf("sortFlowColumn must clear after Reset, got %q", m.sortFlowColumn)
	}
}

// tdp M6: with no sort, Reset is dimmed — Enter on it does nothing and
// saves nothing.
func TestResetSortFlow_DimmedWithNoSort(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, config.DefaultConfig())
	_ = m.openSortColumnPicker(k8s.ResourcePods)
	m.listPicker.animator.Finalize()
	for i := 0; i < len(m.listPicker.items) && m.listPicker.items[m.listPicker.cursor].Key != sortResetKey; i++ {
		m.listPicker, _ = m.listPicker.Update(key("j"))
	}
	if m.listPicker.items[m.listPicker.cursor].Key != sortResetKey {
		t.Fatal("the Reset row must be listed")
	}
	if _, cmd := m.listPicker.Update(key("enter")); cmd != nil {
		t.Error("Enter on the dimmed Reset must do nothing")
	}
	if got := m.cfg.GetSort("pod"); len(got) != 0 {
		t.Errorf("no sort entry may appear, got %+v", got)
	}
}

func TestTableModel_columnTitles_SingleSort_RendersArrowOnly(t *testing.T) {
	// Single-tier chain renders as "Age ↑/↓" — same look as v1.6,
	// no priority "(N)" badge because there's only one column to
	// disambiguate.
	th := theme.DefaultTheme()
	m := NewTableModel(th)
	m.columns = []Column{{Title: "Name"}, {Title: "Ready"}, {Title: "Age"}}
	m.SetSortIndicators([]config.SortConfig{{Column: "Age", Direction: "desc"}})
	got := m.columnTitles()
	if got[0] != "Name" {
		t.Errorf("non-sorted column got %q, want plain title", got[0])
	}
	if !strings.HasPrefix(got[2], "Age ") {
		t.Errorf("sorted column %q should be 'Age ' + arrow", got[2])
	}
	if strings.Contains(got[2], "(") {
		t.Errorf("single-tier sort must not show priority badge, got %q", got[2])
	}
	m.SetSortIndicators(nil)
	got = m.columnTitles()
	if got[2] != "Age" {
		t.Errorf("after clear indicator, Age column = %q, want plain Age", got[2])
	}
}

func TestTableModel_columnTitles_MultiSort_RendersPriorityBadges(t *testing.T) {
	// Multi-tier chain renders priority "(N)" + arrow on each
	// sorted column so the user can see the chain order at a glance.
	th := theme.DefaultTheme()
	m := NewTableModel(th)
	m.columns = []Column{{Title: "Name"}, {Title: "Ready"}, {Title: "Age"}}
	m.SetSortIndicators([]config.SortConfig{
		{Column: "Age", Direction: "desc"},
		{Column: "Name", Direction: "asc"},
	})
	got := m.columnTitles()
	if !strings.Contains(got[2], "Age (1)") {
		t.Errorf("primary column should show '(1)' badge, got %q", got[2])
	}
	if !strings.Contains(got[0], "Name (2)") {
		t.Errorf("secondary column should show '(2)' badge, got %q", got[0])
	}
	if got[1] != "Ready" {
		t.Errorf("non-sorted column should stay plain, got %q", got[1])
	}
}

// TestUpdateRouting_ListPickerWinsOverSpaceMenu pins tdp F4 + D3: when
// the Space menu spawns the sort listPicker (via [Alt-S]ort), keys go to
// the picker on top, not the menu underneath.
func TestUpdateRouting_ListPickerWinsOverSpaceMenu(t *testing.T) {
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)

	// Source menu open at layer 1.
	openPodMenu(&m.spaceMenu, k8s.ResourcePods, k8s.ResourceItem{Name: "nginx"}, panel2CompareCtx{})
	panel2CursorBefore := m.spaceMenu.cursor

	// Sort picker opens on top at layer 2.
	_ = m.openSortColumnPicker(k8s.ResourcePods)
	m.listPicker.animator.Finalize()
	if !m.spaceMenu.IsActive() || !m.listPicker.IsActive() {
		t.Fatalf("setup: both popups must be active, got spaceMenu=%v listPicker=%v",
			m.spaceMenu.IsActive(), m.listPicker.IsActive())
	}
	listCursorBefore := m.listPicker.cursor

	// Press j — must move the picker's cursor, NOT the menu's.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app := updated.(AppModel)

	if app.listPicker.cursor == listCursorBefore {
		t.Error("j must move listPicker cursor when stacked on the Space menu")
	}
	if app.spaceMenu.cursor != panel2CursorBefore {
		t.Errorf("j must NOT move the Space menu cursor; was %d, now %d",
			panel2CursorBefore, app.spaceMenu.cursor)
	}
}

// TestUpdateRouting_SortFromPanel1SpaceMenuStacksOverIt pins tdp F4 for
// panel 1: the Space menu's [S]ort row opens the sort picker OVER the
// menu (it no longer closes itself first), and j/k then drive the picker
// while the menu waits underneath.
func TestUpdateRouting_SortFromPanel1SpaceMenuStacksOverIt(t *testing.T) {
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)
	m.activePanel = SidebarPanel
	m.sidebar.SnapCursorToKind(k8s.ResourcePods)
	_ = m.openSpaceMenu()
	m.spaceMenu.animator.Finalize()
	menuCursorBefore := m.spaceMenu.cursor

	app := pressInMenu(t, m, "S")
	if !app.spaceMenu.owns() {
		t.Fatal("the panel 1 Space menu must stay open under the sort picker")
	}
	if !app.listPicker.owns() {
		t.Fatal("[S]ort must open the sort picker")
	}
	app.listPicker.animator.Finalize()
	listCursorBefore := app.listPicker.cursor

	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(AppModel)
	if app.listPicker.cursor == listCursorBefore {
		t.Error("j must move the sort picker cursor")
	}
	if app.spaceMenu.cursor != menuCursorBefore {
		t.Error("j must not move the Space menu cursor under the picker")
	}
}

// tdp D2: each step takes its own layer — the direction step sits one
// layer above the column picker it stacks on, and the column picker keeps
// its layer while tiers land.
func TestSortFlow_StepsTakeTheirOwnLayers(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := config.DefaultConfig()
	m := appWithCfg([]k8s.ResourceItem{makePod("a", 0)}, cfg)

	// Source menu underneath → column picker at 2, direction at 3.
	openPodMenu(&m.spaceMenu, k8s.ResourcePods, k8s.ResourceItem{Name: "nginx"}, panel2CompareCtx{})
	_ = m.openSortColumnPicker(k8s.ResourcePods)
	m.listPicker.animator.Finalize()
	if got := m.listPicker.layer; got != 2 {
		t.Fatalf("column picker over the Space menu: layer %d, want 2", got)
	}
	_ = m.openSortDirectionPicker(k8s.ResourcePods, "Name")
	m.sortDirPicker.animator.Finalize()
	if got := m.sortDirPicker.layer; got != 3 {
		t.Errorf("direction step over the column picker: layer %d, want 3", got)
	}
	_ = m.commitSortFlow(config.SortDirectionAscending)
	if got := m.listPicker.layer; got != 2 {
		t.Errorf("the column picker must keep layer 2 after a tier, got %d", got)
	}
}
