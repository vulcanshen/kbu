package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestBreadcrumb() BreadcrumbPopupModel {
	return NewBreadcrumbPopupModel(theme.DefaultTheme())
}

// TestBreadcrumbPopup_EnterEmitsLinkJumpMsg confirms the existing
// enter-to-jump behavior survives the space-hotkey addition.
func TestBreadcrumbPopup_EnterEmitsLinkJumpMsg(t *testing.T) {
	m := newTestBreadcrumb()
	chain := []k8s.RefTarget{
		{Type: k8s.ResourcePods, Name: "pod-a", Namespace: "ns-x"},
		{Type: k8s.ResourceDeployments, Name: "dep-b", Namespace: "ns-x"},
		{Type: k8s.ResourceConfigMaps, Name: "cfg-c", Namespace: "ns-x"},
	}
	m.Open(chain)
	m.animator.State = PopupOpen
	m.cursor = 1

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter must return a Cmd (close + jump)")
	}
	// tea.Batch's exact shape isn't inspectable, but we can drive the cmd
	// repeatedly and check the message types observed. For this we only
	// check that the cmd is non-nil; the more specific test below covers
	// Space's message routing.
}

// TestBreadcrumbPopup_SpaceEmitsRequestSwitchToResourceMsg verifies that
// space on the cursor-selected level emits RequestSwitchToResourceMsg —
// AppModel turns that into a confirm popup whose on-confirm fires the
// actual SwitchToResourceMsg. Routing through the request type keeps
// breadcrumb space and Relatives-tab space sharing one confirm gate.
func TestBreadcrumbPopup_SpaceEmitsRequestSwitchToResourceMsg(t *testing.T) {
	m := newTestBreadcrumb()
	chain := []k8s.RefTarget{
		{Type: k8s.ResourcePods, Name: "pod-a", Namespace: "ns-x"},
		{Type: k8s.ResourceDeployments, Name: "dep-b", Namespace: "ns-x"},
		{Type: k8s.ResourceConfigMaps, Name: "cfg-c", Namespace: "ns-x"},
	}
	m.Open(chain)
	m.animator.State = PopupOpen
	m.cursor = 1 // Deployment

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if cmd == nil {
		t.Fatal("Space must return a Cmd")
	}

	// Single cmd (no close-self) — breadcrumb stays open under the
	// confirm popup. AppModel handles the cleanup if the user confirms.
	rm, ok := cmd().(RequestSwitchToResourceMsg)
	if !ok {
		t.Fatalf("Space cmd output = %T, want RequestSwitchToResourceMsg", cmd())
	}
	want := chain[1]
	if rm.Ref != want {
		t.Errorf("RequestSwitchToResourceMsg.Ref = %+v, want %+v", rm.Ref, want)
	}
}

// TestBreadcrumbPopup_SpaceNoOpOnEmptyChain guards the bounds check —
// space when the chain is empty or cursor is out of range must not
// crash and must not emit a switch.
func TestBreadcrumbPopup_SpaceNoOpOnEmptyChain(t *testing.T) {
	m := newTestBreadcrumb()
	m.Open(nil)
	m.animator.State = PopupOpen
	m.cursor = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if cmd != nil {
		t.Errorf("Space on empty chain should return nil cmd, got %T", cmd)
	}
}
