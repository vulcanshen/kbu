package ui

import (
	"testing"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// withToast shows a toast, fully in.
func withToast(m AppModel) AppModel {
	_ = m.toast.Show("Copied!")
	m.toast.animator.Finalize()
	return m
}

// escOnce sends one Esc.
func escOnce(m AppModel) AppModel {
	updated, _ := m.Update(key("esc"))
	return updated.(AppModel)
}

// tdp K4, F3: Esc closes one layer, the top one, and a toast counts —
// it is drawn over everything. Over a popup, the first Esc takes the
// toast; the popup stays until the next.
func TestK4_EscTakesTheToastBeforeAPopup(t *testing.T) {
	m := stackTestApp(t)
	m.yamlPopup.SetSize(m.width, m.height)
	_ = m.yamlPopup.Open("a: 1\n", k8s.ResourcePods, k8s.ResourceItem{Name: "a"})
	m.yamlPopup.animator.Finalize()
	m = escOnce(withToast(m))
	if m.toast.Owns() {
		t.Error("Esc left the toast up and closed something beneath it")
	}
	if !m.yamlPopup.animator.Owns() {
		t.Error("the first Esc closed the YAML viewer under the toast")
	}
	if m = escOnce(m); m.yamlPopup.animator.Owns() {
		t.Error("the second Esc should close the YAML viewer")
	}
}

// tdp K4: popups — the toast included — close before a mode ends. The
// drag mode's own Tab toast says "Esc leaves drag mode first": that Esc
// takes the toast, the next one leaves the mode.
func TestK4_EscTakesTheToastBeforeAMode(t *testing.T) {
	m := dragApp(t)
	m = escOnce(withToast(m))
	if m.toast.Owns() {
		t.Error("Esc in drag mode left the toast up")
	}
	if !m.sidebar.IsDragging() {
		t.Error("the Esc that took the toast also ended the drag")
	}
	if m = escOnce(m); m.sidebar.IsDragging() {
		t.Error("the second Esc should end the drag")
	}
}

// tdp K4, K8: the same while typing a search — the toast first, then
// the search.
func TestK4_EscTakesTheToastBeforeTyping(t *testing.T) {
	m := menuFixture(t, []k8s.ResourceItem{{Name: "a"}, {Name: "b"}}, 0)
	m.setPanel(TablePanel)
	updated, _ := m.Update(key("/"))
	m = updated.(AppModel)
	if !m.table.IsSearching() {
		t.Fatal("setup: / must start the panel 2 search")
	}
	m = escOnce(withToast(m))
	if m.toast.Owns() || !m.table.IsSearching() {
		t.Errorf("first Esc: toast up %v, still typing %v — want the toast gone, the search kept", m.toast.Owns(), m.table.IsSearching())
	}
}
