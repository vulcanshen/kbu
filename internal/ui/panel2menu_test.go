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
	// Pod has containers — Shell must appear. Order: hotkey group
	// (Y/E/S/D) first, then Enter (drill) at the very end. Enter is
	// at the bottom because it's a navigation action with no per-row
	// payload; cursor opens on the first hotkey entry (the most
	// commonly-used inspect action, Y).
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, panel2CompareCtx{})
	wantKeys := []string{"Y", "E", "S", "D", "Enter"}
	if len(items) != len(wantKeys) {
		t.Fatalf("Pod menu len=%d, want %d (keys=%v)", len(items), len(wantKeys), wantKeys)
	}
	for i, want := range wantKeys {
		if items[i].key != want {
			t.Errorf("Pod menu item[%d].key=%q, want %q", i, items[i].key, want)
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
	// Events: system-generated immutable — E, D dropped. Events don't drill
	// either, so no Enter entry. Result: YAML only.
	items := buildPanel2MenuItems(k8s.ResourceEvents, k8s.ResourceItem{}, false, panel2CompareCtx{})
	keys := itemKeys(items)
	if len(keys) != 1 || keys[0] != "Y" {
		t.Errorf("Events menu should be YAML only, got %v", keys)
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
	// Helm-managed Service: only YAML remains.
	items := buildPanel2MenuItems(k8s.ResourceServices, k8s.ResourceItem{}, true, panel2CompareCtx{})
	keys := itemKeys(items)
	if len(keys) != 1 || keys[0] != "Y" {
		t.Errorf("helm-managed Service should only have YAML, got %v", keys)
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

func TestPanel2Menu_Items_CompareAnchored_ShowsShowDiff(t *testing.T) {
	// Anchor set + cursor on a comparable row (different UID, same
	// kind) → "Show diff against Compare anchor" appears with key "C".
	// No "Exit compare mode" entry — Esc is the exit gesture, surfaced
	// via the panel-2 bottom-left hint.
	ctx := panel2CompareCtx{locked: true, canLock: true, cursorComparable: true}
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, ctx)
	found := false
	for _, it := range items {
		if it.key == "C" && it.label == "Show diff vs Compare anchor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Show diff vs Compare anchor' (key=C) in anchored menu, got %v", itemKeys(items))
	}
	for _, banned := range []string{"LockCompare", "CompareTo", "ExitCompare"} {
		if contains(itemKeys(items), banned) {
			t.Errorf("legacy compare key %q must not appear, got %v", banned, itemKeys(items))
		}
	}
}

func TestPanel2Menu_Items_CompareAnchoredOnAnchorRow_HidesC(t *testing.T) {
	// Cursor sitting on the anchor row itself: cursorComparable=false.
	// No "Show diff" (self vs self is empty), no "Mark" (already
	// anchored, re-marking same item is a no-op). C entry is hidden.
	ctx := panel2CompareCtx{locked: true, canLock: true, cursorComparable: false}
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, ctx)
	if contains(itemKeys(items), "C") {
		t.Errorf("C entry must be hidden when cursor sits on the anchor row, got %v", itemKeys(items))
	}
}

func TestPanel2Menu_Items_DeploymentDrillsToPods(t *testing.T) {
	// Deployment drills to Pods — Enter entry sits at the very BOTTOM
	// of the menu (the navigation tail, after the hotkey group); hint
	// says "drill into pods" (KubectlName "pod" + "s").
	items := buildPanel2MenuItems(k8s.ResourceDeployments, k8s.ResourceItem{}, false, panel2CompareCtx{})
	last := items[len(items)-1]
	if last.key != "Enter" {
		t.Fatalf("Deployment last item must be Enter (drill), got %q", last.key)
	}
	if last.hint != "drill into pods" {
		t.Errorf("Deployment Enter hint = %q, want %q", last.hint, "drill into pods")
	}
}

func TestPanel2Menu_Items_PodDrillsToContainers(t *testing.T) {
	// Pod drills to containers (special-cased — containers aren't a K8s
	// API resource so we don't go through the registry's KubectlName).
	// Enter is the LAST entry — Enter sits at the menu tail.
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false, panel2CompareCtx{})
	last := items[len(items)-1]
	if last.key != "Enter" {
		t.Fatalf("Pod last item must be Enter, got %q", last.key)
	}
	if last.hint != "drill into containers" {
		t.Errorf("Pod Enter hint = %q, want %q", last.hint, "drill into containers")
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
	// Pod menu order: Y / E / S / D / Enter. G jumps cursor to the
	// last item — Enter (drill) — and pressing Enter on it commits
	// Action="Enter" → app.go dispatches enterDrillDown. Covers the
	// navigation tail so a regression that re-sorts the items can't
	// silently break the drill path.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "nginx", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item, false, panel2CompareCtx{})
	drainPanel2MenuToInteractive(t, &m, cmd)

	m, _ = m.Update(key("G"))
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

func TestPanel2Menu_OpenForContainer_ShellAndBack(t *testing.T) {
	// Container drill view: Shell (containers aren't standalone API objects
	// so YAML/Edit/Delete don't apply) + Esc back row (with separator) so
	// users discover they can pop back up.
	m := newPanel2Menu(t)
	cmd := m.OpenForContainer("nginx-pod", "default", "nginx")
	drainPanel2MenuToInteractive(t, &m, cmd)

	wantKeys := []string{"S", "Esc"}
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

func TestPanel2Menu_OpenForContainer_EscRowEmitsEsc(t *testing.T) {
	// Cursor j to the Esc row, then Enter commits Action="Esc" — app.go
	// maps this to exitDrillDown (pop back to pod list).
	m := newPanel2Menu(t)
	cmd := m.OpenForContainer("nginx-pod", "default", "nginx")
	drainPanel2MenuToInteractive(t, &m, cmd)

	m, _ = m.Update(key("j"))
	_, batchCmd := m.Update(key("enter"))
	if batchCmd == nil {
		t.Fatal("Enter on Esc row must return a commit cmd")
	}
	found := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		am, ok := msg.(Panel2MenuActionMsg)
		if !ok {
			return false
		}
		if am.Action != "Esc" {
			t.Errorf("container Esc row must commit Esc, got %q", am.Action)
		}
		found = true
		return true
	})
	if !found {
		t.Error("container Esc row did not emit Panel2MenuActionMsg")
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func itemKeys(items []panel2MenuItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.key
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
