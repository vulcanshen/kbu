package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func newPanel2Menu(t *testing.T) Panel2MenuPopupModel {
	t.Helper()
	th, err := theme.LoadTheme("")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}
	m := NewPanel2MenuPopupModel(th)
	m.SetSize(120, 40)
	return m
}

func drainPanel2MenuToInteractive(t *testing.T, m *Panel2MenuPopupModel, openCmd tea.Cmd) {
	t.Helper()
	if openCmd == nil {
		return
	}
	for i := 0; i < 50; i++ {
		if m.IsInteractive() {
			return
		}
		msg := openCmd()
		if tick, ok := msg.(AnimTickMsg); ok {
			openCmd = m.HandleTick(tick)
			continue
		}
		break
	}
	if !m.IsInteractive() {
		t.Fatalf("popup never became interactive after 50 ticks")
	}
}

// ── items shape ────────────────────────────────────────────────────────────

func TestPanel2Menu_Items_PodHasShell(t *testing.T) {
	// Pod has containers — Shell must appear. Order: row-targeted
	// hotkey group (Y/E/S/D) first, then list-level Order, then
	// Enter (drill) at the very end. Enter is at the bottom because
	// it's a navigation action with no per-row payload; cursor opens
	// on the first hotkey entry (the most commonly-used inspect
	// action, Y).
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, panel2CompareCtx{})
	// itemKeys filters separators; comparing the filtered sequence
	// keeps the test focused on commitable hotkeys.
	gotKeys := itemKeys(items)
	wantKeys := []string{"Y", "E", "S", "D", "Enter", "alt+S"}
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("Pod menu keys=%v, want %v", gotKeys, wantKeys)
	}
	for i, want := range wantKeys {
		if gotKeys[i] != want {
			t.Errorf("Pod menu key[%d]=%q, want %q", i, gotKeys[i], want)
		}
	}
}

func TestPanel2Menu_Items_ServiceNoShell(t *testing.T) {
	// Service has no containers — Shell must be omitted. Service also
	// doesn't drill — no Enter entry.
	items := buildPanel2MenuItems(k8s.ResourceServices, k8s.ResourceItem{}, false, panel2CompareCtx{})
	for _, it := range items {
		if it.key == "S" {
			t.Errorf("Service menu must not have Shell, got items=%v", items)
		}
		if it.key == "Enter" {
			t.Errorf("Service menu must not have Enter entry (no drill), got items=%v", items)
		}
	}
	keys := itemKeys(items)
	for _, want := range []string{"Y", "E", "D"} {
		if !contains(keys, want) {
			t.Errorf("Service menu missing %q, got %v", want, keys)
		}
	}
}

func TestPanel2Menu_Items_HelmManagedRuleA(t *testing.T) {
	// Helm-managed pod: Edit and Delete dropped (Rule A read-only).
	// Shell + Enter still present (read actions).
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, true, panel2CompareCtx{})
	keys := itemKeys(items)
	for _, banned := range []string{"E", "D"} {
		if contains(keys, banned) {
			t.Errorf("helm-managed Pod must not show %q (Rule A read-only), got %v", banned, keys)
		}
	}
	for _, want := range []string{"Enter", "Y", "S"} {
		if !contains(keys, want) {
			t.Errorf("helm-managed Pod missing %q, got %v", want, keys)
		}
	}
}

func TestPanel2Menu_Items_EventsNoEditNoDelete(t *testing.T) {
	// Events: system-generated immutable — E, D dropped. Events
	// don't drill either, so no Enter entry. Order entry still
	// applies (sorting events by Age / Last Seen / Reason is one of
	// the highest-value Order use cases).
	items := buildPanel2MenuItems(k8s.ResourceEvents, k8s.ResourceItem{}, false, panel2CompareCtx{})
	keys := itemKeys(items)
	want := []string{"Y", "alt+S"}
	if len(keys) != len(want) {
		t.Errorf("Events menu should be %v, got %v", want, keys)
	}
	for i, k := range want {
		if i >= len(keys) || keys[i] != k {
			t.Errorf("Events menu key[%d]=%q, want %q", i, keys[i], k)
		}
	}
}

func TestPanel2Menu_Items_NodesEditNoDelete(t *testing.T) {
	// Nodes: Edit kept (admin label/taint changes), Delete blocked.
	// Nodes don't drill — no Enter entry.
	items := buildPanel2MenuItems(k8s.ResourceNodes, k8s.ResourceItem{}, false, panel2CompareCtx{})
	keys := itemKeys(items)
	if !contains(keys, "Y") || !contains(keys, "E") {
		t.Errorf("Nodes menu missing YAML or Edit, got %v", keys)
	}
	if contains(keys, "D") {
		t.Errorf("Nodes menu must not have Delete (admin infra action), got %v", keys)
	}
	if contains(keys, "Enter") {
		t.Errorf("Nodes menu must not have Enter (no drill), got %v", keys)
	}
}

func TestPanel2Menu_Items_NamespacesEditNoDelete(t *testing.T) {
	// Namespaces: Edit kept (labels/annotations), Delete blocked.
	// Namespaces don't drill — no Enter entry.
	items := buildPanel2MenuItems(k8s.ResourceNamespaces, k8s.ResourceItem{}, false, panel2CompareCtx{})
	keys := itemKeys(items)
	if !contains(keys, "Y") || !contains(keys, "E") {
		t.Errorf("Namespaces menu missing YAML or Edit, got %v", keys)
	}
	if contains(keys, "D") {
		t.Errorf("Namespaces menu must not have Delete (cascades to all workloads), got %v", keys)
	}
}

func TestPanel2Menu_Items_HelmManagedNoContainer(t *testing.T) {
	// Helm-managed Service: row-level write actions (E/D) blocked by
	// Rule A, no Shell (no container), no drill — Y remains. Order
	// stays because it's a view-level operation (sorting a
	// helm-managed list is still useful).
	items := buildPanel2MenuItems(k8s.ResourceServices, k8s.ResourceItem{}, true, panel2CompareCtx{})
	keys := itemKeys(items)
	want := []string{"Y", "alt+S"}
	if len(keys) != len(want) {
		t.Errorf("helm-managed Service menu should be %v, got %v", want, keys)
	}
	for i, k := range want {
		if i >= len(keys) || keys[i] != k {
			t.Errorf("helm-managed Service key[%d]=%q, want %q", i, keys[i], k)
		}
	}
}

func TestPanel2Menu_Items_CompareNotAnchored_ShowsMarkEntry(t *testing.T) {
	// No anchor set + canLock (panel-2 has >1 items) → "Mark as
	// Compare anchor" appears with key "C". The same key serves the
	// "Show diff" entry in the locked state — both surface
	// contextually via a single muscle-memory hotkey.
	ctx := panel2CompareCtx{locked: false, canLock: true}
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, ctx)
	found := false
	for _, it := range items {
		if it.key == "C" && it.label == "Mark as Compare anchor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Mark as Compare anchor' (key=C) in not-anchored menu, got %v", itemKeys(items))
	}
}

func TestPanel2Menu_Items_CompareSingleItem_HidesMark(t *testing.T) {
	// canLock=false (single item — nothing to compare against) hides
	// the Mark entry entirely. The C hotkey gates on the same flag at
	// the AppModel layer so direct presses are no-ops too.
	ctx := panel2CompareCtx{locked: false, canLock: false}
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, ctx)
	if contains(itemKeys(items), "C") {
		t.Errorf("C entry must be hidden when canLock=false, got %v", itemKeys(items))
	}
}

func TestPanel2Menu_Items_CompareAnchored_ShowsCompareTo(t *testing.T) {
	// Anchor set + cursor on a comparable row (different UID, same
	// kind) → "Compare to anchor" appears with key "C". No "Exit
	// compare mode" entry — Esc is the exit gesture, surfaced via the
	// panel-2 bottom-left hint. Label is the [C]ompare-prefixed form
	// (the C lives at the start, bracketed by bracketHotkey).
	ctx := panel2CompareCtx{locked: true, canLock: true, cursorComparable: true}
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, ctx)
	found := false
	for _, it := range items {
		if it.key == "C" && it.label == "Compare to anchor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Compare to anchor' (key=C) in anchored menu, got %v", itemKeys(items))
	}
	for _, banned := range []string{"LockCompare", "CompareTo", "ExitCompare"} {
		if contains(itemKeys(items), banned) {
			t.Errorf("legacy compare key %q must not appear, got %v", banned, itemKeys(items))
		}
	}
}

func TestPanel2Menu_Items_CompareAnchoredOnAnchorRow_ShowsUnmark(t *testing.T) {
	// Cursor sitting on the anchor row itself: cursorOnAnchor=true.
	// "Unmark Compare anchor" surfaces with key "C" — pressing C
	// toggles the anchor off, mirroring the C-on-anchor cancel
	// behaviour in compareHotkeyDispatch.
	ctx := panel2CompareCtx{locked: true, canLock: true, cursorOnAnchor: true}
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, ctx)
	found := false
	for _, it := range items {
		if it.key == "C" && it.label == "Unmark Compare anchor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Unmark Compare anchor' (key=C) when cursor on anchor row, got %v", itemKeys(items))
	}
}

func TestPanel2Menu_Items_CompareAnchoredKindMismatch_HidesC(t *testing.T) {
	// Anchor set but user has switched panel-1 to a different kind:
	// cursorComparable=false AND cursorOnAnchor=false. Mark / Compare-to
	// / Unmark all make no sense here — C entry is hidden.
	ctx := panel2CompareCtx{locked: true, canLock: true, cursorComparable: false, cursorOnAnchor: false}
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, ctx)
	if contains(itemKeys(items), "C") {
		t.Errorf("C entry must be hidden when anchor kind doesn't match current kind, got %v", itemKeys(items))
	}
}

func TestPanel2Menu_Items_DeploymentDrillsToPods(t *testing.T) {
	// Deployment drills to Pods — Enter entry is the last row-level
	// item (sits above the separator, before the panel-level Order
	// entry); hint says "drill into pods" (KubectlName "pod" + "s").
	items := buildPanel2MenuItems(k8s.ResourceDeployments, k8s.ResourceItem{}, false, panel2CompareCtx{})
	enter := findItemByKey(items, "Enter")
	if enter == nil {
		t.Fatalf("Deployment menu must contain an Enter entry, got %v", itemKeys(items))
	}
	if enter.hint != "drill into pods" {
		t.Errorf("Deployment Enter hint = %q, want %q", enter.hint, "drill into pods")
	}
}

func TestPanel2Menu_Items_PodDrillsToContainers(t *testing.T) {
	// Pod drills to containers (special-cased — containers aren't a K8s
	// API resource so we don't go through the registry's KubectlName).
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, panel2CompareCtx{})
	enter := findItemByKey(items, "Enter")
	if enter == nil {
		t.Fatalf("Pod menu must contain an Enter entry, got %v", itemKeys(items))
	}
	if enter.hint != "drill into containers" {
		t.Errorf("Pod Enter hint = %q, want %q", enter.hint, "drill into containers")
	}
}

func TestPanel2Menu_Items_TwoRegionEmitsHeaders(t *testing.T) {
	// Kinds with an Order entry render as two regions: "item
	// operation" prepended above the row-targeted group, "panel
	// operation" between separator and Order. Locks the labelled-
	// region layout.
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, panel2CompareCtx{})
	var itemHdrIdx, sepIdx, panelHdrIdx, orderIdx int = -1, -1, -1, -1
	for i, it := range items {
		switch {
		case it.header && it.label == "item operation":
			itemHdrIdx = i
		case it.separator:
			sepIdx = i
		case it.header && it.label == "panel operation":
			panelHdrIdx = i
		case it.key == "alt+S":
			orderIdx = i
		}
	}
	if itemHdrIdx < 0 || sepIdx < 0 || panelHdrIdx < 0 || orderIdx < 0 {
		t.Fatalf("expected item header, separator, panel header, Sort; got items=%+v", items)
	}
	if !(itemHdrIdx < sepIdx && sepIdx < panelHdrIdx && panelHdrIdx < orderIdx) {
		t.Errorf("expected sort: item(%d) < sep(%d) < panel(%d) < sort(%d)", itemHdrIdx, sepIdx, panelHdrIdx, orderIdx)
	}
	if itemHdrIdx != 0 {
		t.Errorf("item operation header must be at top, got idx=%d", itemHdrIdx)
	}
}

func TestPanel2Menu_Items_NoOrderNoHeaders(t *testing.T) {
	// Kinds without sort columns get no Order entry, so the menu
	// stays single-region and skips the header chrome. Verify with
	// a kind whose registry def lacks columns — Events with no
	// edit/delete still gets Order because it has columns; use
	// container-drill OpenForContainer path which bypasses
	// buildPanel2MenuItems entirely. Instead, hammer
	// buildPanel2MenuItems with a forged kind shape via the helm-
	// managed Events case (Events still has columns, so Order
	// appears; we can't easily force a no-columns kind from
	// tests). Skipped — Order ubiquity in real kinds means this
	// single-region branch is exercised only by container drill
	// which doesn't call buildPanel2MenuItems. Left here as a
	// placeholder for future kinds with empty Columns.
	t.Skip("all registered kinds expose >=1 column; single-region branch is reachable only via OpenForContainer which uses a different code path")
}

func TestPanel2Menu_HeaderRowsAreSkipped(t *testing.T) {
	// Initial cursor must land on the first commitable item (idx
	// after the header) and j/k must skip headers + separator.
	m := newPanel2Menu(t)
	cmd := m.Open(k8s.ResourcePods, k8s.ResourceItem{Name: "p", Namespace: "default"}, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	// items[0] is the "item operation" header → cursor should be 1.
	if m.cursor == 0 {
		t.Fatalf("cursor must skip item-operation header on Open, got 0")
	}
	if m.items[m.cursor].header {
		t.Errorf("cursor must not land on a header row, got idx=%d (%+v)", m.cursor, m.items[m.cursor])
	}
	// Walk to the end with G — must land on Sort (alt+S), not the
	// "panel operation" header above it.
	m, _ = m.Update(key("G"))
	if m.items[m.cursor].header || m.items[m.cursor].separator {
		t.Errorf("G must land on a selectable row, got %+v", m.items[m.cursor])
	}
	if m.items[m.cursor].key != "alt+S" {
		t.Errorf("G must land on Sort (last selectable), got key=%q", m.items[m.cursor].key)
	}
}

func TestPanel2Menu_Items_EnterPrecedesSeparator(t *testing.T) {
	// Enter is row-level — must sit above the separator that splits
	// row-targeted entries from panel-level (Order). Locks the
	// layout decision so a future reorder can't push Enter past the
	// boundary.
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, panel2CompareCtx{})
	sepIdx, enterIdx := -1, -1
	for i, it := range items {
		if it.separator && sepIdx < 0 {
			sepIdx = i
		}
		if it.key == "Enter" {
			enterIdx = i
		}
	}
	if sepIdx < 0 || enterIdx < 0 {
		t.Fatalf("Pod menu must have both a separator and an Enter entry, got %v", itemKeys(items))
	}
	if enterIdx >= sepIdx {
		t.Errorf("Enter (idx=%d) must precede the separator (idx=%d) — Enter is row-level, separator marks the row/panel boundary", enterIdx, sepIdx)
	}
}

func TestPanel2Menu_Open_CanPopAppendsEsc(t *testing.T) {
	// When user has drilled into a parent's children (e.g. Deployment →
	// Pods via Enter on the Deployment row), Open(...canPop=true) appends
	// an "Esc" entry so users see they can pop back to the parent list.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "nginx-abc-xyz", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item, true, panel2CompareCtx{}) // canPop=true → from drill
	drainPanel2MenuToInteractive(t, &m, cmd)

	first := m.items[0]
	if first.key != "Esc" {
		t.Fatalf("when canPop=true, first item must be Esc, got %q (full=%v)", first.key, itemKeys(m.items))
	}
	if first.hint != "back to parent list" {
		t.Errorf("Esc hint = %q, want %q", first.hint, "back to parent list")
	}
}

func TestPanel2Menu_Open_NoCanPopOmitsEsc(t *testing.T) {
	// Root list (not drilled): canPop=false → no Esc entry.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "nginx", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	for _, it := range m.items {
		if it.key == "Esc" {
			t.Errorf("canPop=false must not produce an Esc entry, got %v", itemKeys(m.items))
		}
	}
}

// ── commit paths ───────────────────────────────────────────────────────────

func TestPanel2Menu_EnterEmitsActionMsg(t *testing.T) {
	// Cursor starts at index 0 — the top of the menu. With the
	// hotkey-group-first layout that's "Y" (view YAML). Pressing Enter
	// on cursor 0 commits Action="Y".
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "nginx", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, batchCmd := m.Update(key("enter"))
	if batchCmd == nil {
		t.Fatal("Enter must return a Cmd (close + action)")
	}
	found := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		am, ok := msg.(Panel2MenuActionMsg)
		if !ok {
			return false
		}
		if am.Action != "Y" {
			t.Errorf("cursor 0 should commit Y (YAML, the top hotkey entry), got %q", am.Action)
		}
		if am.Item.Name != "nginx" {
			t.Errorf("action item.Name=%q, want nginx", am.Item.Name)
		}
		found = true
		return true
	})
	if !found {
		t.Error("Enter did not emit Panel2MenuActionMsg")
	}
}

func TestPanel2Menu_CursorOnEnterRowCommitsEnter(t *testing.T) {
	// Pod menu order: Y / E / S / D / Enter / ── / O. G jumps to
	// the last SELECTABLE row — Order. k from there backs up over
	// the separator onto Enter (drill); committing it emits
	// Action="Enter" → app.go dispatches enterDrillDown. Covers
	// the navigation around the separator so a regression that
	// reorders the items can't silently break the drill path.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "nginx", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	m, _ = m.Update(key("G")) // last selectable = Order
	m, _ = m.Update(key("k")) // back up to Enter (skips separator)
	_, batchCmd := m.Update(key("enter"))
	if batchCmd == nil {
		t.Fatal("Enter must return a Cmd")
	}
	found := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		am, ok := msg.(Panel2MenuActionMsg)
		if !ok {
			return false
		}
		if am.Action != "Enter" {
			t.Errorf("last cursor (Enter drill row) should commit Enter, got %q", am.Action)
		}
		found = true
		return true
	})
	if !found {
		t.Error("Enter on last row did not emit Panel2MenuActionMsg")
	}
}

func TestPanel2Menu_HotkeyDirectTrigger(t *testing.T) {
	// Pressing "E" while menu is open commits Edit without cursor move.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "nginx", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, batchCmd := m.Update(key("E"))
	if batchCmd == nil {
		t.Fatal("E hotkey must commit Edit")
	}
	found := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		am, ok := msg.(Panel2MenuActionMsg)
		if !ok {
			return false
		}
		if am.Action != "E" {
			t.Errorf("E hotkey should emit action E, got %q", am.Action)
		}
		found = true
		return true
	})
	if !found {
		t.Error("E hotkey did not emit Panel2MenuActionMsg")
	}
}

func TestPanel2Menu_AltSHotkeyEmitsSortAction(t *testing.T) {
	// Alt+Shift+S while panel 2 menu is open must commit with
	// Action "alt+S" so AppModel routes it to openSortColumnPicker
	// — same path as cursor+Enter on the Sort entry.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "my-pod", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, batchCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}, Alt: true})
	if batchCmd == nil {
		t.Fatal("Alt+S must emit a commit cmd")
	}
	found := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		am, ok := msg.(Panel2MenuActionMsg)
		if !ok {
			return false
		}
		if am.Action != "alt+S" {
			t.Errorf("Alt+S should emit action alt+S, got %q", am.Action)
		}
		if am.Resource != k8s.ResourcePods {
			t.Errorf("Alt+S action carries Resource=%q, want Pods", am.Resource)
		}
		found = true
		return true
	})
	if !found {
		t.Error("Alt+S did not emit Panel2MenuActionMsg")
	}
}

func TestPanel2Menu_SortEntryUsesAltShiftSLabel(t *testing.T) {
	// Panel-2 Sort entry renders the literal "[Alt][S]ort panel 2
	// list" label (bracketHotkey is bypassed because the key is the
	// multi-char chord "alt+S", so the pre-composed markers pass
	// through). Locks the wording the popup-design mindset settled
	// on for the panel-2 entry.
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, panel2CompareCtx{})
	sort := findItemByKey(items, "alt+S")
	if sort == nil {
		t.Fatal("Pods menu must contain a Sort entry (key=alt+S)")
	}
	want := "[Alt][S]ort panel 2 list"
	if sort.label != want {
		t.Errorf("Sort label = %q, want %q", sort.label, want)
	}
}

func TestPanel2Menu_HotkeyNotInMenuIsNoOp(t *testing.T) {
	// Service has no Shell — pressing "S" while menu is open must NOT
	// commit (no item matches). Avoids bypassing Rule A or capability
	// gates via hotkey.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "my-svc", Namespace: "default"}
	cmd := m.Open(k8s.ResourceServices, item, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, cmd2 := m.Update(key("S"))
	if cmd2 != nil {
		t.Errorf("S hotkey on Service (no Shell) must be no-op, got cmd=%T", cmd2)
	}
}

func TestPanel2Menu_HotkeyBlockedByRuleA(t *testing.T) {
	// Helm-managed Pod: E removed by Rule A. Hotkey E must NOT commit.
	m := newPanel2Menu(t)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "nginx",
			Labels: map[string]string{"app.kubernetes.io/managed-by": "Helm"},
		},
	}
	item := k8s.ResourceItem{Name: "nginx", Namespace: "default", Raw: pod}
	cmd := m.Open(k8s.ResourcePods, item, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, cmd2 := m.Update(key("E"))
	if cmd2 != nil {
		t.Errorf("E hotkey on helm-managed Pod must be no-op (Rule A), got cmd=%T", cmd2)
	}
}

// ── close paths ────────────────────────────────────────────────────────────

func TestPanel2Menu_SpaceCloses(t *testing.T) {
	m := newPanel2Menu(t)
	cmd := m.Open(k8s.ResourcePods, k8s.ResourceItem{Name: "nginx"}, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key(" "))
	if closeCmd == nil {
		t.Fatal("Space must fire animator close cmd")
	}
}

func TestPanel2Menu_EscCloses(t *testing.T) {
	m := newPanel2Menu(t)
	cmd := m.Open(k8s.ResourcePods, k8s.ResourceItem{Name: "nginx"}, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key("esc"))
	if closeCmd == nil {
		t.Fatal("Esc must fire animator close cmd")
	}
}

// ── container drill (OpenForContainer) ─────────────────────────────────────

func TestPanel2Menu_OpenForContainer_ShellOnly(t *testing.T) {
	// v1.7: container drill menu surfaces just Shell. The previous
	// "Esc back to pod list" entry was a discoverability hint that
	// duplicated the universal Esc gesture (any popup's Esc closes
	// the popup, and exitDrillDown fires whether or not the menu is
	// open). Removed per popup-design mindset — no redundant entries.
	m := newPanel2Menu(t)
	cmd := m.OpenForContainer("nginx-pod", "default", "nginx")
	drainPanel2MenuToInteractive(t, &m, cmd)

	wantKeys := []string{"S"}
	if len(m.items) != len(wantKeys) {
		t.Fatalf("container menu len=%d, want %d (keys=%v)", len(m.items), len(wantKeys), itemKeys(m.items))
	}
	for i, want := range wantKeys {
		if m.items[i].key != want {
			t.Errorf("container menu item[%d].key=%q, want %q", i, m.items[i].key, want)
		}
	}
}

func TestPanel2Menu_OpenForContainer_EnterCommitsShell(t *testing.T) {
	m := newPanel2Menu(t)
	cmd := m.OpenForContainer("nginx-pod", "default", "nginx")
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, batchCmd := m.Update(key("enter"))
	if batchCmd == nil {
		t.Fatal("Enter must return a commit cmd")
	}
	found := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		am, ok := msg.(Panel2MenuActionMsg)
		if !ok {
			return false
		}
		if am.Action != "S" {
			t.Errorf("container Enter must commit S (Shell), got %q", am.Action)
		}
		found = true
		return true
	})
	if !found {
		t.Error("container Enter did not emit Panel2MenuActionMsg")
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func findItemByKey(items []panel2MenuItem, key string) *panel2MenuItem {
	for i := range items {
		if items[i].key == key {
			return &items[i]
		}
	}
	return nil
}

func itemKeys(items []panel2MenuItem) []string {
	// Separators and headers are visual-only chrome — excluded from
	// the "what hotkeys does this menu expose" view so tests can
	// reason about the commitable hotkey sequence without caring
	// about the section labels around them.
	var out []string
	for _, it := range items {
		if it.separator || it.header {
			continue
		}
		out = append(out, it.key)
	}
	return out
}

func contains(s []string, want string) bool {
	for _, x := range s {
		if x == want {
			return true
		}
	}
	return false
}

func TestBracketHotkey_RendersUppercaseEvenWhenLabelHasLowercase(t *testing.T) {
	cases := []struct {
		name  string
		label string
		key   string
		want  string
	}{
		{"all-caps label, all-caps key", "YAML", "Y", "[Y]AML"},
		{"mixed-case label, key matches uppercase letter", "Pin Pods", "P", "[P]in Pods"},
		{"mixed-case label, key matches lowercase letter", "Unpin Pods", "P", "Un[P]in Pods"},
		{"multi-char key falls back", "Enter Drill", "Enter", "Enter Drill"},
		{"missing letter falls back", "Compare", "Z", "Compare"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bracketHotkey(tc.label, tc.key)
			if got != tc.want {
				t.Errorf("bracketHotkey(%q, %q) = %q, want %q", tc.label, tc.key, got, tc.want)
			}
		})
	}
}
