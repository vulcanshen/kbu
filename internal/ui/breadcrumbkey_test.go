package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// relativesAt is the app focused on panel 3's Relatives tab of a Pod,
// drilled depth-1 levels down (depth 1 = not drilled).
func relativesAt(t *testing.T, depth int) AppModel {
	t.Helper()
	m := stackTestApp(t)
	m.setPanel(DetailPanel)
	m.detail.SetResourceType(k8s.ResourcePods)
	m.detail.SetDetail(samplePodRelativesDetail(), nil)
	if !m.detail.SwitchToTabByName("Relatives") {
		t.Fatal("setup: no Relatives tab")
	}
	for i := 1; i < depth; i++ {
		m.detail.PushDrillFrame(
			k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx", Namespace: "default"},
			k8s.ResourceItem{Name: "nginx", Namespace: "default"},
			k8s.ResourceDetail{Name: "nginx", Namespace: "default", Kind: "Deployment"})
	}
	if m.detail.Depth() != depth {
		t.Fatalf("setup: depth %d, want %d", m.detail.Depth(), depth)
	}
	return m
}

// Drilled into Relatives, the Space menu's panel operations have
// [B]readcrumb — the name of the popup it opens — and B runs it.
func TestRelatives_BreadcrumbRowIsB(t *testing.T) {
	m := relativesAt(t, 2)
	_ = m.openSpaceMenu()
	m.spaceMenu.SetSize(m.width, m.height)
	m.spaceMenu.animator.Finalize()
	row := menuRow(t, m.spaceMenu.items, "B")
	if row.label != "Breadcrumb" {
		t.Errorf("the B row is %q, want Breadcrumb", row.label)
	}
	if !strings.Contains(ansi.Strip(m.spaceMenu.renderFullPopup()), "[B]readcrumb") {
		t.Error("the menu must show the row as [B]readcrumb")
	}
	for _, it := range m.spaceMenu.items {
		if strings.Contains(it.label, "ancestor") {
			t.Errorf("old row still there: %q", it.label)
		}
	}

	// B in the menu opens the breadcrumb over it (the menu stays, F4).
	m = pressInMenu(t, m, "B")
	m.breadcrumbPopup.animator.Finalize()
	if m.topLayer() != &m.breadcrumbPopup {
		t.Fatalf("B in the menu should open the breadcrumb, top is %T", m.topLayer())
	}
	if !m.spaceMenu.owns() {
		t.Error("the Space menu must stay under the breadcrumb")
	}
}

// B on the panel itself opens the breadcrumb; so does ? list it.
func TestRelatives_BOnThePanelOpensTheBreadcrumb(t *testing.T) {
	m := relativesAt(t, 2)
	updated, cmd := m.Update(key("B"))
	m = updated.(AppModel)
	_ = cmd
	if !m.breadcrumbPopup.owns() {
		t.Fatal("B on a drilled Relatives tab must open the breadcrumb")
	}
	fresh := relativesAt(t, 2)
	_, rows := fresh.keyRef()
	found := false
	for _, r := range rows {
		if r.key == "B" {
			found = true
		}
	}
	if !found {
		t.Error("? must list B")
	}
}

// At the first level there is nothing up the chain: no row, and B does
// nothing.
func TestRelatives_NoBreadcrumbAtTheFirstLevel(t *testing.T) {
	m := relativesAt(t, 1)
	title, items := m.detailMenu()
	_ = title
	for _, it := range items {
		if it.key == "B" {
			t.Errorf("the first level has a %q row", it.label)
		}
	}
	updated, _ := m.Update(key("B"))
	if got := updated.(AppModel); got.breadcrumbPopup.owns() {
		t.Error("B at the first level must do nothing")
	}
}
