package ui

import (
	"strings"
	"testing"

	"github.com/vulcanshen/kbu/internal/k8s"
)

func refKeys(rows []helpRow) []string {
	var out []string
	for _, r := range rows {
		if !r.header {
			out = append(out, r.key)
		}
	}
	return out
}

func pressQuestion(t *testing.T, m AppModel) AppModel {
	t.Helper()
	updated, _ := m.Update(key("?"))
	got := updated.(AppModel)
	if !got.help.owns() {
		t.Fatal("? must open the key reference")
	}
	got.help.animator.Finalize()
	return got
}

// tdp K6, M4: on a panel, ? lists that panel's keys — every hotkey of its
// Space menu — and the core keys.
func TestK6_PanelKeyReferenceListsItsMenuKeysAndCoreKeys(t *testing.T) {
	items := []k8s.ResourceItem{podItem("a", nil), podItem("b", nil)}
	m := menuFixture(t, items, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods

	got := pressQuestion(t, m)
	keys := refKeys(got.help.rows)
	_, menu, _, _ := got.tableMenu()
	for _, it := range menu {
		if it.selectable() && it.key != "" && !contains(keys, keyName(it.key)) {
			t.Errorf("the panel 2 key reference is missing its Space menu key %q", it.key)
		}
	}
	for _, core := range []string{"Tab", "Space", "?", "q", "Ctrl-C"} {
		if !contains(keys, core) {
			t.Errorf("the panel key reference is missing the core key %q", core)
		}
	}
}

// tdp K6: over a popup, ? lists only that popup's keys — none of the
// panel's.
func TestK6_PopupKeyReferenceListsOnlyThePopupsKeys(t *testing.T) {
	m := stackTestApp(t)
	_ = m.confirm.Show(ConfirmDelete, "Delete?", "kubectl delete pods a", nil)
	m.confirm.animator.Finalize()

	got := pressQuestion(t, m)
	keys := refKeys(got.help.rows)
	for _, want := range []string{"Enter/y", "Esc/n"} {
		if !contains(keys, want) {
			t.Errorf("the confirm's key reference is missing %q (tdp F6)", want)
		}
	}
	for _, panelKey := range []string{"Tab", "Space", "1–3"} {
		if contains(keys, panelKey) {
			t.Errorf("the confirm's key reference must not list the panel key %q", panelKey)
		}
	}
	if !strings.Contains(got.help.rows[0].desc, "delete") {
		t.Errorf("the confirm's Enter row must say what it does, got %q", got.help.rows[0].desc)
	}
}

// tdp K6 + F4: ? stacks the reference on the popup; Esc on it returns to
// that popup.
func TestK6_ReferenceStacksOnThePopupAndEscReturns(t *testing.T) {
	m := stackTestApp(t)
	_ = m.comparePopup.Open("a: 1\n", "a: 2\n", "l", "r")
	m.comparePopup.animator.Finalize()

	got := pressQuestion(t, m)
	if !got.comparePopup.owns() {
		t.Fatal("the Compare popup must stay under its key reference")
	}
	if !contains(refKeys(got.help.rows), "L") {
		t.Error("the Compare key reference must list L")
	}
	updated, _ := got.Update(key("esc"))
	got = updated.(AppModel)
	if got.help.owns() || !got.comparePopup.owns() {
		t.Error("Esc on the key reference must return to the Compare popup")
	}
}

// tdp K6 on the menus themselves: the Space menu and the global popup
// answer ? too.
func TestK6_MenusAnswerQuestionMark(t *testing.T) {
	m := stackTestApp(t)
	openPodMenu(&m.spaceMenu, k8s.ResourcePods, podItem("a", nil), panel2CompareCtx{canLock: true})
	got := pressQuestion(t, m)
	if keys := refKeys(got.help.rows); !contains(keys, "Y") || !contains(keys, "Space") {
		t.Errorf("the Space menu's key reference must list its rows and Space, got %v", keys)
	}
}

// tdp K8: while typing, ? is a character.
func TestK6_QuestionMarkWhileTypingIsACharacter(t *testing.T) {
	m := stackTestApp(t)
	m.activePanel = TablePanel
	m.table.SetFocused(true)
	m.table, _ = m.table.Update(key("/"))
	updated, _ := m.Update(key("?"))
	got := updated.(AppModel)
	if got.help.owns() {
		t.Error("? while typing a search must not open the key reference")
	}
	if got.table.searchQuery != "?" {
		t.Errorf("? while typing must be typed, query %q", got.table.searchQuery)
	}
}

// tdp M1: the footer always shows ? and Space; Esc there reads "back",
// never like leaving the app (K4).
func TestM1_FooterShowsTheEntryKeys(t *testing.T) {
	m := stackTestApp(t)
	footer := m.statusLine.layoutLine()
	for _, want := range []string{"?:help", "Space:menu", "Esc:back"} {
		if !strings.Contains(footer, want) {
			t.Errorf("the footer must show %q, got %q", want, footer)
		}
	}
}
