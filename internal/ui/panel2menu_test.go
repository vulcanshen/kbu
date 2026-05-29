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
	// Pod has containers — Shell must appear. Pod also drills (→ containers)
	// so Enter entry is appended at the END (with separator). Full set:
	// Y / E / S / D / Enter, Enter has groupBreak=true.
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false)
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
	items := buildPanel2MenuItems(k8s.ResourceServices, k8s.ResourceItem{}, false)
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
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, true)
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
	items := buildPanel2MenuItems(k8s.ResourceEvents, k8s.ResourceItem{}, false)
	keys := itemKeys(items)
	if len(keys) != 1 || keys[0] != "Y" {
		t.Errorf("Events menu should be YAML only, got %v", keys)
	}
}

func TestPanel2Menu_Items_NodesEditNoDelete(t *testing.T) {
	// Nodes: Edit kept (admin label/taint changes), Delete blocked.
	// Nodes don't drill — no Enter entry.
	items := buildPanel2MenuItems(k8s.ResourceNodes, k8s.ResourceItem{}, false)
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
	items := buildPanel2MenuItems(k8s.ResourceNamespaces, k8s.ResourceItem{}, false)
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
	items := buildPanel2MenuItems(k8s.ResourceServices, k8s.ResourceItem{}, true)
	keys := itemKeys(items)
	if len(keys) != 1 || keys[0] != "Y" {
		t.Errorf("helm-managed Service should only have YAML, got %v", keys)
	}
}

func TestPanel2Menu_Items_DeploymentDrillsToPods(t *testing.T) {
	// Deployment drills to Pods — Enter entry is last with groupBreak,
	// hint should say "drill into pods" (KubectlName "pod" + "s").
	items := buildPanel2MenuItems(k8s.ResourceDeployments, k8s.ResourceItem{}, false)
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
	items := buildPanel2MenuItems(k8s.ResourcePods, k8s.ResourceItem{}, false)
	last := items[len(items)-1]
	if last.key != "Enter" {
		t.Fatalf("Pod last item must be Enter, got %q", last.key)
	}
	if last.hint != "drill into containers" {
		t.Errorf("Pod Enter hint = %q, want %q", last.hint, "drill into containers")
	}
}

// ── commit paths ───────────────────────────────────────────────────────────

func TestPanel2Menu_EnterEmitsActionMsg(t *testing.T) {
	// Cursor starts at 0 = YAML. Pressing Enter commits Y.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "nginx", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item)
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
			t.Errorf("cursor 0 should commit Y (YAML), got %q", am.Action)
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

func TestPanel2Menu_EnterOnDrillRowEmitsEnter(t *testing.T) {
	// Pod menu ends with Enter (drill) at the last index. G jumps cursor
	// to the last item; pressing Enter commits Action="Enter" → app.go
	// maps to enterDrillDown.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "nginx", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item)
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
			t.Errorf("last cursor (drill row) should commit Enter, got %q", am.Action)
		}
		found = true
		return true
	})
	if !found {
		t.Error("Enter on drill row did not emit Panel2MenuActionMsg")
	}
}

func TestPanel2Menu_HotkeyDirectTrigger(t *testing.T) {
	// Pressing "E" while menu is open commits Edit without cursor move.
	m := newPanel2Menu(t)
	item := k8s.ResourceItem{Name: "nginx", Namespace: "default"}
	cmd := m.Open(k8s.ResourcePods, item)
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
	cmd := m.Open(k8s.ResourceServices, item)
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
	cmd := m.Open(k8s.ResourcePods, item)
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, cmd2 := m.Update(key("E"))
	if cmd2 != nil {
		t.Errorf("E hotkey on helm-managed Pod must be no-op (Rule A), got cmd=%T", cmd2)
	}
}

// ── close paths ────────────────────────────────────────────────────────────

func TestPanel2Menu_SpaceCloses(t *testing.T) {
	m := newPanel2Menu(t)
	cmd := m.Open(k8s.ResourcePods, k8s.ResourceItem{Name: "nginx"})
	drainPanel2MenuToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key(" "))
	if closeCmd == nil {
		t.Fatal("Space must fire animator close cmd")
	}
}

func TestPanel2Menu_EscCloses(t *testing.T) {
	m := newPanel2Menu(t)
	cmd := m.Open(k8s.ResourcePods, k8s.ResourceItem{Name: "nginx"})
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
	// maps this to exitDrillDown (pop back to pod's container list).
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
