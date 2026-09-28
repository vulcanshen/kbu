package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// stackTestApp is a 120×40 app with every popup wired and nothing open.
func stackTestApp(t *testing.T) AppModel {
	t.Helper()
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	m.ready = true
	return m
}

// openTestSpaceMenu opens the panel 2 Space menu on a Pod row, fully open.
func openTestSpaceMenu(m *AppModel) {
	m.spaceMenu.SetSize(m.width, m.height)
	openPodMenu(&m.spaceMenu, k8s.ResourcePods, k8s.ResourceItem{Name: "nginx", Namespace: "default"},
		panel2CompareCtx{canLock: true})
}

// centeredRect is where a centered popup string lands on a w×h screen —
// the same placement popupRowAt assumes.
func centeredRect(popup string, w, h int) (x, y, pw, ph int) {
	lines := strings.Split(popup, "\n")
	ph = len(lines)
	pw = lipgloss.Width(lines[0])
	x, y = popupOrigin(pw, ph, w, h)
	return x, y, pw, ph
}

// menuRowUnder returns a screen point on a selectable Space-menu row that
// the popup drawn above the menu also covers — a click there used to
// commit the menu row hidden underneath.
func menuRowUnder(t *testing.T, m AppModel, above string) (int, int) {
	t.Helper()
	_, my, _, _ := centeredRect(m.spaceMenu.renderFullPopup(), m.width, m.height)
	ax, ay, aw, ah := centeredRect(above, m.width, m.height)
	x := m.width / 2
	if x < ax || x >= ax+aw {
		t.Fatalf("screen centre %d is outside the popup above (%d..%d)", x, ax, ax+aw)
	}
	for i, it := range m.spaceMenu.items {
		if it.header || it.separator {
			continue
		}
		if y := my + 2 + i; y >= ay && y < ay+ah {
			return x, y
		}
	}
	t.Fatal("no Space-menu row lies under the popup above")
	return 0, 0
}

func leftClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

// firedMenuAction reports whether cmd (or anything batched in it) is a
// Space-menu commit.
func firedMenuAction(cmd tea.Cmd) bool {
	for _, msg := range drainCmd(cmd) {
		if _, ok := msg.(MenuActionMsg); ok {
			return true
		}
	}
	return false
}

// drainCmd runs cmd and returns every message it yields, looking inside
// tea.Batch. Ticks are skipped (they block for their interval).
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drainCmd(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// tdp X2 + D3: a click lands on the popup the user sees on top. The
// confirm a Space menu opens is centred over the menu; clicking it used
// to reach the menu first and run the menu row hidden beneath.
func TestStack_ClickOnConfirmDoesNotRunMenuRowBeneath(t *testing.T) {
	m := stackTestApp(t)
	openTestSpaceMenu(&m)
	m.confirm.SetSize(m.width, m.height)
	_ = m.confirm.Show(ConfirmDelete, "⚠ Delete resource? This cannot be undone.",
		"kubectl delete pods nginx -n default", func() tea.Msg { return nil })
	m.confirm.animator.Finalize()

	x, y := menuRowUnder(t, m, m.confirm.renderFullPopup())
	updated, cmd := m.Update(leftClick(x, y))
	if firedMenuAction(cmd) {
		t.Fatal("a click on the confirm ran the Space-menu row under it")
	}
	if got := updated.(AppModel); !got.confirm.owns() {
		t.Error("a left click must not close or accept the confirm")
	}
}

func TestStack_ClickOnSortPickerDoesNotRunMenuRowBeneath(t *testing.T) {
	m := stackTestApp(t)
	openTestSpaceMenu(&m)
	m.listPicker.SetSize(m.width, m.height)
	_ = m.listPicker.Open("sort:column", "Sort Pods by…", []ListPickerItem{
		{Key: "Name", Label: "Name"}, {Key: "Age", Label: "Age"},
	})
	m.listPicker.animator.Finalize()

	x, y := menuRowUnder(t, m, m.listPicker.renderFullPopup())
	_, cmd := m.Update(leftClick(x, y))
	if firedMenuAction(cmd) {
		t.Fatal("a click on the sort picker ran the Space-menu row under it")
	}
}

func TestStack_ClickOnYamlDoesNotRunMenuRowBeneath(t *testing.T) {
	m := stackTestApp(t)
	openTestSpaceMenu(&m)
	m.yamlPopup.SetSize(m.width, m.height)
	_ = m.yamlPopup.Open("kind: Pod\nmetadata:\n  name: nginx\n", k8s.ResourcePods,
		k8s.ResourceItem{Name: "nginx"})
	m.yamlPopup.animator.Finalize()

	x, y := menuRowUnder(t, m, m.yamlPopup.renderFullPopup())
	_, cmd := m.Update(leftClick(x, y))
	if firedMenuAction(cmd) {
		t.Fatal("a click on the YAML viewer ran the Space-menu row under it")
	}
}

type confirmedMsg struct{}

// tdp D3: the key reference stacked on a confirm takes the keys. Enter
// on it must not accept the confirm underneath.
func TestStack_EnterOnHelpOverConfirmDoesNotConfirm(t *testing.T) {
	m := stackTestApp(t)
	m.confirm.SetSize(m.width, m.height)
	_ = m.confirm.Show(ConfirmDelete, "⚠ Delete resource?", "kubectl delete pods nginx",
		func() tea.Msg { return confirmedMsg{} })
	m.confirm.animator.Finalize()
	m.help.SetSize(m.width, m.height)
	_ = m.openKeyRef()
	m.help.animator.Finalize()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, msg := range drainCmd(cmd) {
		if _, ok := msg.(confirmedMsg); ok {
			t.Fatal("Enter on the key reference accepted the confirm beneath it")
		}
	}
	if got := updated.(AppModel); !got.confirm.owns() {
		t.Error("the confirm beneath the key reference must stay open")
	}
}

// tdp D3: the draw order is the routing order — the key reference that
// takes the keys over a confirm is also the one drawn over it.
func TestStack_HelpIsDrawnOverConfirm(t *testing.T) {
	m := stackTestApp(t)
	m.confirm.SetSize(m.width, m.height)
	_ = m.confirm.Show(ConfirmDelete, "⚠ Delete resource?", "kubectl delete pods nginx", nil)
	m.confirm.animator.Finalize()
	m.help.SetSize(m.width, m.height)
	_ = m.openKeyRef()
	m.help.animator.Finalize()

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Confirm keys") {
		t.Fatal("the key reference must be on screen")
	}
	if strings.Contains(view, "Delete resource?") {
		t.Error("the confirm must be drawn under the key reference, not over it")
	}
}

// tdp F3: a popup running its close animation hands the next key to the
// layer beneath. Esc on the confirm, then Esc again before the close
// animation ends, must close the Space menu under it — the second Esc
// used to be swallowed by the closing confirm.
func TestStack_ClosingPopupHandsEscToTheLayerBeneath(t *testing.T) {
	m := stackTestApp(t)
	openTestSpaceMenu(&m)
	m.confirm.SetSize(m.width, m.height)
	_ = m.confirm.Show(ConfirmDelete, "⚠ Delete resource?", "kubectl delete pods nginx", nil)
	m.confirm.animator.Finalize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(AppModel)
	if got.confirm.owns() || !got.confirm.drawn() {
		t.Fatalf("setup: the first Esc must start closing the confirm (owns=%v drawn=%v)",
			got.confirm.owns(), got.confirm.drawn())
	}
	if !got.spaceMenu.owns() {
		t.Fatal("setup: the first Esc must leave the Space menu open")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got = updated.(AppModel); got.spaceMenu.owns() {
		t.Error("Esc during the confirm's close animation must close the Space menu beneath it")
	}
}

// tdp F3: the same holds for a toast — Esc starts its fade, and a second
// Esc during the fade goes to the panel (here: back out of the drill).
func TestStack_FadingToastHandsEscToThePanel(t *testing.T) {
	items := []k8s.ResourceItem{{Name: "a", UID: "uid-a", Row: []string{"a"}}}
	m := appWithItems(items, 0)
	m.currentResource = k8s.ResourcePods
	m.activePanel = TablePanel
	m.statusLine = NewStatusLineModel(m.theme)
	m.logStreamer = k8s.NewLogStreamer(nil)
	m.drillDownStack = []drillDownEntry{{parentType: k8s.ResourceDeployments, parentName: "web", parentItems: items}}
	_ = m.toast.Show("Copied!")
	m.toast.animator.Finalize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(AppModel)
	if got.toast.Owns() || len(got.drillDownStack) != 1 {
		t.Fatalf("setup: the first Esc must only start the toast's fade (owns=%v drill=%d)",
			got.toast.Owns(), len(got.drillDownStack))
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if n := len(updated.(AppModel).drillDownStack); n != 0 {
		t.Errorf("Esc during the toast's fade must back out of the drill, drill depth = %d", n)
	}
}

// tdp F3 + D2: a popup on its way out no longer counts toward the depth
// the next popup's layer colour is taken from.
func TestStack_PopupDepthSkipsClosingPopups(t *testing.T) {
	m := stackTestApp(t)
	openTestSpaceMenu(&m)
	_ = m.confirm.Show(ConfirmDelete, "⚠ Delete resource?", "kubectl delete pods nginx", nil)
	m.confirm.animator.Finalize()
	if d := m.popupDepth(); d != 2 {
		t.Fatalf("menu + confirm open: depth = %d, want 2", d)
	}
	_ = m.confirm.Close()
	if d := m.popupDepth(); d != 1 {
		t.Errorf("confirm closing: depth = %d, want 1", d)
	}
}
