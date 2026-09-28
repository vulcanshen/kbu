package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/kbu/internal/k8s"
)

var ctrlC = tea.KeyMsg{Type: tea.KeyCtrlC}

// quits reports whether cmd starts the leave flow (a quitMsg, possibly
// inside a batch).
func quits(cmd tea.Cmd) bool {
	for _, msg := range drainCmd(cmd) {
		if isQuitMsg(msg) {
			return true
		}
	}
	return false
}

// tdp K9: q and Ctrl+C are the same leave flow on every surface that is
// not typing — panels, menus, confirms, viewers, pickers.
func TestK9_QAndCtrlCLeaveFromEverySurface(t *testing.T) {
	surfaces := []struct {
		name string
		open func(m *AppModel)
	}{
		{"panel", func(*AppModel) {}},
		{"Space menu", func(m *AppModel) {
			openPodMenu(&m.spaceMenu, k8s.ResourcePods, k8s.ResourceItem{Name: "a"}, panel2CompareCtx{})
		}},
		{"global operation popup", func(m *AppModel) {
			_ = m.openGlobalMenu()
			m.globalMenu.animator.Finalize()
		}},
		{"confirm", func(m *AppModel) {
			_ = m.confirm.Show(ConfirmDelete, "Delete?", "kubectl delete pods a", nil)
			m.confirm.animator.Finalize()
		}},
		{"yaml", func(m *AppModel) {
			_ = m.yamlPopup.Open("a: 1\n", k8s.ResourcePods, k8s.ResourceItem{Name: "a"}, "")
			m.yamlPopup.animator.Finalize()
		}},
		{"namespace picker list", func(m *AppModel) {
			_ = m.namespacePicker.OpenLoading()
			m.namespacePicker.SetNamespaces([]string{"default"})
			m.namespacePicker.animator.Finalize()
		}},
	}
	for _, s := range surfaces {
		for _, k := range []tea.KeyMsg{key("q"), ctrlC} {
			m := stackTestApp(t)
			s.open(&m)
			if _, cmd := m.Update(k); !quits(cmd) {
				t.Errorf("%s on the %s must start the leave flow", k.String(), s.name)
			}
		}
	}
}

// tdp K8 + K9: while typing, q is a character; Ctrl+C still leaves.
func TestK9_WhileTypingQTypesCtrlCLeaves(t *testing.T) {
	typingSurfaces := []struct {
		name  string
		open  func(m *AppModel)
		query func(m AppModel) string
	}{
		{"panel search", func(m *AppModel) {
			m.activePanel = TablePanel
			m.table.SetFocused(true)
			m.table, _ = m.table.Update(key("/"))
		}, func(m AppModel) string { return m.table.searchQuery }},
		{"namespace filter", func(m *AppModel) {
			_ = m.namespacePicker.OpenLoading()
			m.namespacePicker.SetNamespaces([]string{"default"})
			m.namespacePicker.animator.Finalize()
			m.namespacePicker, _ = m.namespacePicker.Update(key("/"))
		}, func(m AppModel) string { return m.namespacePicker.searchQuery }},
		{"yaml search", func(m *AppModel) {
			_ = m.yamlPopup.Open("a: 1\n", k8s.ResourcePods, k8s.ResourceItem{Name: "a"}, "")
			m.yamlPopup.animator.Finalize()
			m.yamlPopup, _ = m.yamlPopup.Update(key("/"))
		}, func(m AppModel) string { return m.yamlPopup.SearchQuery() }},
	}
	for _, s := range typingSurfaces {
		m := stackTestApp(t)
		s.open(&m)
		updated, cmd := m.Update(key("q"))
		if quits(cmd) {
			t.Errorf("q while typing in the %s must be a character, not leave", s.name)
		}
		if got := s.query(updated.(AppModel)); got != "q" {
			t.Errorf("q while typing in the %s: query = %q, want \"q\"", s.name, got)
		}
		if _, cmd := m.Update(ctrlC); !quits(cmd) {
			t.Errorf("Ctrl+C while typing in the %s must still leave", s.name)
		}
	}
}

// tdp K9: Ctrl+C is the same flow as q — through quitMsg, so the session
// state is saved and PTYs are stopped (it used to tea.Quit directly).
func TestK9_CtrlCGoesThroughTheLeaveFlow(t *testing.T) {
	m := stackTestApp(t)
	_, cmd := m.Update(ctrlC)
	if cmd == nil {
		t.Fatal("Ctrl+C must return a cmd")
	}
	if msg := cmd(); !isQuitMsg(msg) {
		t.Errorf("Ctrl+C must emit quitMsg (the one leave flow), got %T", msg)
	}
}

// tdp K10: a PTY on top owns every key — q and Ctrl+C go to the
// subprocess, not the leave flow.
func TestK9_PtyOnTopKeepsQAndCtrlC(t *testing.T) {
	m := stackTestApp(t)
	m.shellPty = fakeAlivePtyView(PtyKindShell, false)
	m.shellPty.animator.State = PopupOpen
	for _, k := range []tea.KeyMsg{key("q"), ctrlC} {
		if _, cmd := m.Update(k); quits(cmd) {
			t.Errorf("%s in a visible PTY must go to the subprocess", k.String())
		}
	}
}

// tdp K10 + F4: confirming the exit key's question ends the edit / exec
// session; Esc on the question returns to the PTY.
func TestK10_EditExitKeyAsksThenEnds(t *testing.T) {
	m := stackTestApp(t)
	m.txPty = fakeAlivePtyView(PtyKindEdit, false)
	m.txPty.animator.State = PopupOpen

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Alt: true, Runes: []rune{'t'}})
	m = updated.(AppModel)
	for _, msg := range drainCmd(cmd) {
		next, _ := m.Update(msg)
		m = next.(AppModel)
	}
	if !m.confirm.owns() || m.confirm.action != ConfirmLeaveEdit {
		t.Fatal("Alt+t in kubectl edit must ask before leaving")
	}
	if m.topLayer() != &m.confirm {
		t.Fatal("the question must stack over the PTY")
	}
	m.confirm.animator.Finalize()
	back, _ := m.Update(key("esc"))
	if got := back.(AppModel); got.topLayer() != got.txPty {
		t.Error("Esc on the question must return to the PTY")
	}
	accepted := false
	for _, msg := range drainCmd(m.confirm.onConfirm) {
		if _, ok := msg.(ptyKillMsg); ok {
			accepted = true
		}
	}
	if !accepted {
		t.Error("accepting must end the kubectl edit session")
	}
}
