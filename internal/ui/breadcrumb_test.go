package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

func newTestBreadcrumb() BreadcrumbPopupModel {
	return NewBreadcrumbPopupModel(theme.DefaultTheme())
}

// TestBreadcrumbPopup_EnterEmitsSwitchMsg — v1.5.x mental model: Enter
// commits the cursor row as a panel 1+2 switch (replaces the previous
// jump-to-drill-level behavior). RequestSwitchToResourceMsg goes through
// the shared confirm gate at the AppModel layer.
func TestBreadcrumbPopup_EnterEmitsSwitchMsg(t *testing.T) {
	m := newTestBreadcrumb()
	chain := []k8s.RefTarget{
		{Type: k8s.ResourcePods, Name: "pod-a", Namespace: "ns-x"},
		{Type: k8s.ResourceDeployments, Name: "dep-b", Namespace: "ns-x"},
		{Type: k8s.ResourceConfigMaps, Name: "cfg-c", Namespace: "ns-x"},
	}
	m.Open(chain)
	m.animator.State = PopupOpen
	m.cursor = 1 // Deployment

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter must return a Cmd (switch)")
	}
	rm, ok := cmd().(RequestSwitchToResourceMsg)
	if !ok {
		t.Fatalf("Enter cmd output = %T, want RequestSwitchToResourceMsg", cmd())
	}
	if rm.Ref != chain[1] {
		t.Errorf("RequestSwitchToResourceMsg.Ref = %+v, want %+v", rm.Ref, chain[1])
	}
}

// TestBreadcrumbPopup_SpaceCloses — v1.5.x mental model: Space mirrors
// open and closes the popup without committing. Aligns with the global
// rule "any menu popup Space = close". Don't call cmd() — animator
// close uses tea.Tick which would block under test harness.
func TestBreadcrumbPopup_SpaceCloses(t *testing.T) {
	m := newTestBreadcrumb()
	chain := []k8s.RefTarget{
		{Type: k8s.ResourcePods, Name: "pod-a", Namespace: "ns-x"},
		{Type: k8s.ResourceDeployments, Name: "dep-b", Namespace: "ns-x"},
	}
	m.Open(chain)
	m.animator.State = PopupOpen
	m.cursor = 1

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if cmd == nil {
		t.Fatal("Space must return a close Cmd")
	}
	// Type-level check would require invoking cmd(), which blocks on
	// the animator tick. Non-nil is the contract — close cmd is an
	// implementation detail of PopupAnimator.
}

// TestBreadcrumbPopup_EnterNoOpOnEmptyChain — bounds check still applies
// to Enter (commit needs a valid cursor row).
func TestBreadcrumbPopup_EnterNoOpOnEmptyChain(t *testing.T) {
	m := newTestBreadcrumb()
	m.Open(nil)
	m.animator.State = PopupOpen
	m.cursor = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("Enter on empty chain should return nil cmd, got %T", cmd)
	}
}
