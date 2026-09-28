package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

// ── the menu model ───────────────────────────────────────────────────

func TestBracketHotkey_PrintsTheKeyAsPressed(t *testing.T) {
	cases := []struct{ label, key, want string }{
		{"YAML", "Y", "[Y]AML"},
		{"Pin Pods", "P", "[P]in Pods"},
		{"Unpin Pods", "P", "Un[P]in Pods"}, // bracket prints the key (Shift+P), not the label's p
		{"Copy", "y", "Cop[y]"},             // lowercase key stays lowercase: y, not Shift+Y
		{"Zoom", "z", "[z]oom"},
		{"Search", "/", "[/] Search"}, // not in the label: in front (tdp D4)
		{"Switch tab", "l", "[l] Switch tab"},
		{"[Enter] Drill in", "enter", "[Enter] Drill in"}, // multi-char keys write their own bracket
		{"[Alt-S]ort panel 2 list", "alt+S", "[Alt-S]ort panel 2 list"},
		{"Global operation", "", "Global operation"},
	}
	for _, c := range cases {
		if got := bracketHotkey(c.label, c.key); got != c.want {
			t.Errorf("bracketHotkey(%q, %q) = %q, want %q", c.label, c.key, got, c.want)
		}
	}
}

func testMenu(items []menuItem, spaceToggle bool) MenuPopupModel {
	m := NewSpaceMenuModel(theme.DefaultTheme())
	m.spaceToggle = spaceToggle
	_ = m.Open(" t ", items, "", k8s.ResourceItem{})
	m.animator.Finalize()
	return m
}

func ranAction(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		return ""
	}
	if msg, ok := cmd().(MenuActionMsg); ok {
		return msg.Action
	}
	return ""
}

// tdp M6: a dimmed row is listed and the cursor can rest on it, but
// neither Enter nor its hotkey runs it.
func TestMenu_DimmedRowDoesNotRun(t *testing.T) {
	m := testMenu([]menuItem{
		{label: "Edit", key: "E", disabled: true},
		{label: "YAML", key: "Y"},
	}, true)
	if m.cursor != 0 {
		t.Fatalf("the cursor must be able to rest on a dimmed row, at %d", m.cursor)
	}
	if _, cmd := m.Update(key("enter")); ranAction(t, cmd) != "" {
		t.Error("Enter ran a dimmed row")
	}
	if _, cmd := m.Update(key("E")); ranAction(t, cmd) != "" {
		t.Error("the hotkey of a dimmed row ran it")
	}
	if _, cmd := m.Update(key("Y")); ranAction(t, cmd) != "Y" {
		t.Error("an enabled row's hotkey must run it")
	}
}

// tdp D4: j/k move over chrome rows and wrap at the ends; Enter runs the
// cursor row; Enter / Esc rows are cursor-only.
func TestMenu_CursorSkipsChromeAndWraps(t *testing.T) {
	m := testMenu(groupedMenu(
		[]menuItem{{label: "[Enter] Drill in", key: "enter", action: "drill"}},
		[]menuItem{{label: "Zoom", key: "z"}}), true)
	if got := m.items[m.cursor].actionName(); got != "drill" {
		t.Fatalf("cursor starts on %q, want the first row", got)
	}
	m, _ = m.Update(key("j"))
	m, _ = m.Update(key("j"))
	if got := m.items[m.cursor].actionName(); got != globalOpAction {
		t.Errorf("two j's: cursor on %q, want the Global operation row", got)
	}
	m, _ = m.Update(key("j"))
	if got := m.items[m.cursor].actionName(); got != "drill" {
		t.Errorf("j past the last row must wrap to the first, got %q", got)
	}
	if _, cmd := m.Update(key("enter")); ranAction(t, cmd) != "drill" {
		t.Error("Enter must run the cursor row")
	}
}

// tdp K5: Space closes the Space menu it opened; on the global operation
// popup it does nothing.
func TestMenu_SpaceClosesOnlyTheSpaceMenu(t *testing.T) {
	space := testMenu(groupedMenu(nil, nil), true)
	space, _ = space.Update(key(" "))
	if space.animator.Owns() {
		t.Error("Space must close the Space menu")
	}
	global := NewGlobalMenuModel(theme.DefaultTheme())
	_ = global.Open(" g ", globalActions, "", k8s.ResourceItem{})
	global.animator.Finalize()
	global, cmd := global.Update(key(" "))
	if !global.animator.Owns() || cmd != nil {
		t.Error("Space must do nothing on the global operation popup")
	}
}

// ── M2: every Space menu has the same shape ──────────────────────────

// checkShape asserts tdp M2's fixed shape: item operation, then panel
// operation, each under its header when present, a rule, and one final
// Global operation row with no header of its own.
func checkShape(t *testing.T, items []menuItem) {
	t.Helper()
	n := len(items)
	if n < 2 || items[n-1].actionName() != globalOpAction || !items[n-2].separator {
		t.Fatalf("a Space menu must end with a rule and the Global operation row: %v", labels(items))
	}
	var headers []string
	for _, it := range items {
		if it.header {
			headers = append(headers, it.label)
		}
	}
	for _, h := range headers {
		if h != "item operation" && h != "panel operation" {
			t.Errorf("unexpected region header %q (the global row has none): %v", h, labels(items))
		}
	}
	if len(headers) == 2 && headers[0] != "item operation" {
		t.Errorf("item operation must come before panel operation: %v", labels(items))
	}
	if items[0].selectable() && items[0].actionName() != globalOpAction {
		t.Errorf("rows before the Global row must sit under a region header: %v", labels(items))
	}
}

func regionKeys(items []menuItem, region string) []string {
	var out []string
	in := false
	for _, it := range items {
		switch {
		case it.header:
			in = it.label == region
		case it.separator:
			in = false
		case in:
			out = append(out, it.actionName())
		}
	}
	return out
}

func menuFixture(t *testing.T, items []k8s.ResourceItem, cursor int) AppModel {
	t.Helper()
	m := appWithItems(items, cursor)
	m.width, m.height = 120, 40
	m.sidebar = NewSidebarModel(m.theme)
	m.statusLine = NewStatusLineModel(m.theme)
	return m
}

func openedMenu(t *testing.T, m AppModel) (AppModel, []menuItem) {
	t.Helper()
	updated, _ := m.Update(key(" "))
	got := updated.(AppModel)
	if !got.spaceMenu.owns() {
		t.Fatal("Space must open the Space menu (tdp K5, M7)")
	}
	got.spaceMenu.animator.Finalize()
	checkShape(t, got.spaceMenu.items)
	return got, got.spaceMenu.items
}

func podItem(name string, labels map[string]string) k8s.ResourceItem {
	return k8s.ResourceItem{
		Name: name, Namespace: "default", UID: "uid-" + name, Row: []string{name},
		Raw: &metav1.ObjectMeta{Name: name, Labels: labels},
	}
}

// Panel 1: the cursor's kind in item operation, Drag and Search in panel
// operation. Drag is dimmed with fewer than two pinned kinds (tdp M6).
func TestSpaceMenu_Panel1(t *testing.T) {
	m := menuFixture(t, nil, 0)
	m.activePanel = SidebarPanel
	m.sidebar.SetPinned([]k8s.ResourceType{k8s.ResourcePods})
	m.sidebar.SnapCursorToKind(k8s.ResourcePods)

	_, items := openedMenu(t, m)
	if got := regionKeys(items, "item operation"); strings.Join(got, " ") != "P S enter y" {
		t.Errorf("panel 1 item operation = %v, want P S enter y", got)
	}
	if got := regionKeys(items, "panel operation"); strings.Join(got, " ") != "D /" {
		t.Errorf("panel 1 panel operation = %v, want D /", got)
	}
	if !menuRow(t, items, "D").disabled {
		t.Error("Drag must be dimmed with a single pinned kind")
	}
}

// Panel 2 on a Pod row: every row action is listed (tdp M3), and so are
// the panel's list actions.
func TestSpaceMenu_Panel2PodRow(t *testing.T) {
	items := []k8s.ResourceItem{podItem("a", nil), podItem("b", nil)}
	m := menuFixture(t, items, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods

	_, rows := openedMenu(t, m)
	if got := regionKeys(rows, "item operation"); strings.Join(got, " ") != "Y E S D C drill y" {
		t.Errorf("Pod item operation = %v, want Y E S D C drill y", got)
	}
	if got := regionKeys(rows, "panel operation"); strings.Join(got, " ") != "alt+S / . z" {
		t.Errorf("Pod panel operation = %v, want alt+S / . z", got)
	}
}

// tdp M6 (item 8): a helm-managed row lists Edit / Delete dimmed rather
// than hiding them, and the E / D hotkeys do nothing on it either — no
// confirm, no explaining toast.
func TestSpaceMenu_HelmManagedRowDimsEditAndDelete(t *testing.T) {
	helm := podItem("a", map[string]string{"app.kubernetes.io/managed-by": "Helm"})
	m := menuFixture(t, []k8s.ResourceItem{helm, podItem("b", nil)}, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods

	_, rows := openedMenu(t, m)
	if !menuRow(t, rows, "E").disabled || !menuRow(t, rows, "D").disabled {
		t.Error("Edit and Delete must be listed dimmed on a helm-managed row")
	}
	for _, k := range []string{"E", "D"} {
		updated, cmd := m.Update(key(k))
		got := updated.(AppModel)
		if got.confirm.owns() || got.toast.Owns() || cmd != nil {
			t.Errorf("%s on a helm-managed row must do nothing (confirm=%v toast=%v cmd=%v)",
				k, got.confirm.owns(), got.toast.Owns(), cmd != nil)
		}
	}
}

// tdp M6 (item 8): with only one row there is nothing to compare the
// anchor with — Mark as Compare anchor is dimmed, not hidden.
func TestSpaceMenu_SingleRowDimsMarkAnchor(t *testing.T) {
	m := menuFixture(t, []k8s.ResourceItem{podItem("a", nil)}, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods
	_, rows := openedMenu(t, m)
	if !menuRow(t, rows, "C").disabled {
		t.Error("Mark as Compare anchor must be dimmed when panel 2 has one row")
	}
}

// A Helm Release row: its documents are item operations next to YAML;
// no kubectl edit / delete (helm manages it), no helm-visibility toggle.
func TestSpaceMenu_ReleaseRow(t *testing.T) {
	rel := k8s.ResourceItem{Name: "web", Namespace: "apps", UID: "uid-web", Row: []string{"web"}}
	m := menuFixture(t, []k8s.ResourceItem{rel}, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourceReleases

	_, rows := openedMenu(t, m)
	want := "Y doc:manifest doc:notes doc:values doc:all-values doc:hooks y"
	got := strings.Join(regionKeys(rows, "item operation"), " ")
	for _, a := range []string{"E", "D", "."} {
		if contains(itemKeys(rows), a) {
			t.Errorf("a Release row must not list %q: %v", a, labels(rows))
		}
	}
	for _, d := range helmDocRows {
		if !strings.Contains(got, helmDocActionPrefix+d.docKind) {
			t.Errorf("the Release menu must list %q (have %q, want like %q)", d.label, got, want)
		}
	}
	// E / D on the panel never reach kubectl for a Release either.
	for _, k := range []string{"E", "D"} {
		updated, _ := m.Update(key(k))
		if got := updated.(AppModel); got.confirm.owns() {
			t.Errorf("%s on a Release row must not open a kubectl confirm", k)
		}
	}
}

// An empty panel 2 still opens the menu (tdp M7): no item region, the
// panel's own actions, and the Global row.
func TestSpaceMenu_EmptyPanel2(t *testing.T) {
	m := menuFixture(t, nil, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods
	_, rows := openedMenu(t, m)
	if got := regionKeys(rows, "item operation"); len(got) != 0 {
		t.Errorf("an empty list has no item operation region, got %v", got)
	}
	if got := regionKeys(rows, "panel operation"); strings.Join(got, " ") != "alt+S / . z" {
		t.Errorf("empty panel 2 operations = %v", got)
	}
}

// In a drill, [Esc] Back is a panel operation naming the list it returns
// to (the old "Esc ↖" row sat first, before the item region).
func TestSpaceMenu_DrillBackRow(t *testing.T) {
	items := []k8s.ResourceItem{podItem("a", nil)}
	m := menuFixture(t, items, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods
	m.drillDownStack = []drillDownEntry{{parentType: k8s.ResourceDeployments, parentName: "web"}}
	_, rows := openedMenu(t, m)
	back := menuRow(t, rows, "back")
	if back.label != "[Esc] Back" || back.hint != "to the Deployments list" {
		t.Errorf("back row = %q / %q", back.label, back.hint)
	}
	if !contains(regionKeys(rows, "panel operation"), "back") {
		t.Error("[Esc] Back must be a panel operation")
	}
}

// tdp M3: while compare mode is on, leaving it is a row too.
func TestSpaceMenu_CompareModeListsExit(t *testing.T) {
	items := []k8s.ResourceItem{podItem("a", nil), podItem("b", nil)}
	m := menuFixture(t, items, 1)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods
	m.setCompareLock(items[0], k8s.ResourcePods)
	_, rows := openedMenu(t, m)
	if first := regionKeys(rows, "item operation"); len(first) == 0 || first[0] != "C" {
		t.Errorf("Compare to anchor must lead the item region, got %v", first)
	}
	if !contains(regionKeys(rows, "panel operation"), "exit-compare") {
		t.Error("Exit compare mode must be listed while compare mode is on")
	}
}

// Panel 3: each tab's own actions are rows (tdp M3), switching tab
// included.
func TestSpaceMenu_Panel3Tabs(t *testing.T) {
	m := menuFixture(t, nil, 0)
	m.activePanel = DetailPanel
	m.detail.SetResourceType(k8s.ResourcePods)
	m.detail.SwitchToTabByName("Logs")
	_, rows := openedMenu(t, m)
	if got := regionKeys(rows, "panel operation"); strings.Join(got, " ") != "G y Y z l" {
		t.Errorf("Logs panel operation = %v, want G y Y z l", got)
	}
}

// ── the global operation popup ───────────────────────────────────────

// tdp M4, F4: the Global operation row opens the global operation popup
// over the Space menu; Esc on it goes back to the Space menu.
func TestGlobalMenu_OpensOverTheSpaceMenuAndEscGoesBack(t *testing.T) {
	m := menuFixture(t, nil, 0)
	m.activePanel = TablePanel
	m, rows := openedMenu(t, m)
	for m.spaceMenu.items[m.spaceMenu.cursor].actionName() != globalOpAction {
		m.spaceMenu, _ = m.spaceMenu.Update(key("j"))
	}
	_ = rows
	app := pressInMenu(t, m, "enter")
	if !app.globalMenu.owns() || !app.spaceMenu.owns() {
		t.Fatalf("Global operation must open the global popup over the Space menu (global=%v space=%v)",
			app.globalMenu.owns(), app.spaceMenu.owns())
	}
	app.globalMenu.animator.Finalize()
	if got := itemKeys(app.globalMenu.items); strings.Join(got, " ") != "N C alt+t > ! q" {
		t.Errorf("the global popup must list every global action, got %v", got)
	}
	updated, _ := app.Update(key("esc"))
	app = updated.(AppModel)
	if app.globalMenu.owns() || !app.spaceMenu.owns() {
		t.Error("Esc on the global popup must return to the Space menu")
	}
}

// tdp K9: leaving the app is in the global operation popup.
func TestGlobalMenu_QuitLeaves(t *testing.T) {
	m := menuFixture(t, nil, 0)
	_ = m.openGlobalMenu()
	m.globalMenu.animator.Finalize()
	var cmd tea.Cmd
	m.globalMenu, cmd = m.globalMenu.Update(key("q"))
	action, ok := cmd().(MenuActionMsg)
	if !ok {
		t.Fatal("q must run the Quit row")
	}
	_, cmd = m.Update(action)
	if cmd == nil || !isQuitMsg(cmd()) {
		t.Error("the Quit row must start the quit flow")
	}
}

// tdp F4: a popup opened from the global popup (Settings here) stacks on
// it; both menus stay beneath.
func TestGlobalMenu_SettingsStacksOverIt(t *testing.T) {
	m := menuFixture(t, nil, 0)
	m.spaceMenu.animator.State = PopupOpen
	_ = m.openGlobalMenu()
	m.globalMenu.animator.Finalize()
	var cmd tea.Cmd
	m.globalMenu, cmd = m.globalMenu.Update(key(">"))
	updated, _ := m.Update(cmd())
	got := updated.(AppModel)
	if !got.settingsPopup.owns() || !got.globalMenu.owns() || !got.spaceMenu.owns() {
		t.Errorf("Settings must open over the global popup, which stays (settings=%v global=%v space=%v)",
			got.settingsPopup.owns(), got.globalMenu.owns(), got.spaceMenu.owns())
	}
}

// ── rows run their hotkey ────────────────────────────────────────────

// A row that only changes state (Zoom) does its job and closes the menu
// (tdp T1); a row that opens a popup (YAML) leaves the menu beneath it
// (F4).
func TestSpaceMenu_StateRowClosesPopupRowStays(t *testing.T) {
	items := []k8s.ResourceItem{podItem("a", nil)}
	m := menuFixture(t, items, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods
	m.detail.SetDetail(k8s.ResourceDetail{YAML: "kind: Pod\n"}, nil)
	m, _ = openedMenu(t, m)

	zoomed := pressInMenu(t, m, "z")
	if !zoomed.tableExpanded || zoomed.spaceMenu.owns() {
		t.Errorf("Zoom must zoom panel 2 and close the menu (zoomed=%v menu=%v)", zoomed.tableExpanded, zoomed.spaceMenu.owns())
	}
	yaml := pressInMenu(t, m, "Y")
	if !yaml.yamlPopup.owns() || !yaml.spaceMenu.owns() {
		t.Errorf("YAML must open over the menu, which stays (yaml=%v menu=%v)", yaml.yamlPopup.owns(), yaml.spaceMenu.owns())
	}
}

// tdp L1: at 80 × 40 the tallest Space menu (a Pod row in a drill, in
// compare mode) fits the screen.
func TestSpaceMenu_FitsAt80x40(t *testing.T) {
	items := []k8s.ResourceItem{podItem("a", nil), podItem("b", nil)}
	m := menuFixture(t, items, 1)
	m.width, m.height = 80, 40
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods
	m.drillDownStack = []drillDownEntry{{parentType: k8s.ResourceDeployments, parentName: "web"}}
	m.setCompareLock(items[0], k8s.ResourcePods)
	m, _ = openedMenu(t, m)
	popup := m.spaceMenu.renderFullPopup()
	lines := strings.Split(popup, "\n")
	if len(lines) > 40 {
		t.Errorf("the Space menu is %d rows tall at 80×40", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > 80 {
			t.Errorf("row %d is %d wide at 80 columns", i, w)
		}
	}
}
