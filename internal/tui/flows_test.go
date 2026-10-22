package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	corev1 "k8s.io/api/core/v1"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// The namespace picker's checks mean what the popup shows in Green.
func TestPickedSelection(t *testing.T) {
	names := []string{"a", "b", "c"}
	cases := []struct {
		name string
		now  []bool
		want []string // nil for All
	}{
		{"All checked, a too", []bool{true, true, false, false}, nil},
		{"only a", []bool{false, true, false, false}, []string{"a"}},
		{"a and b", []bool{false, true, true, false}, []string{"a", "b"}},
		{"nothing checked", []bool{false, false, false, false}, nil},
	}
	for _, c := range cases {
		got := pickedSelection(names, c.now)
		if c.want == nil {
			if !got.IsAll() {
				t.Errorf("%s: %v, want All", c.name, got.List())
			}
			continue
		}
		if got.IsAll() || !slices.Equal(got.List(), c.want) {
			t.Errorf("%s: %v, want %v", c.name, got.List(), c.want)
		}
	}
}

// Enter on a pod drills into its containers; Esc comes back to the pod.
func TestDrillIntoContainersAndBack(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "j", "Enter")
	if !m.atContainers() || !strings.Contains(m.listP.Label, "Containers") {
		t.Fatalf("Enter did not drill into the containers: %q", m.listP.Label)
	}
	if r, _ := m.list.Current(); r.ID != "app" {
		t.Errorf("row %q, want the container app", r.ID)
	}
	if m.det.kind != "" || !slices.Equal(m.det.tabs, []string{"Logs"}) {
		t.Errorf("panel 3 at a container: kind %q tabs %v", m.det.kind, m.det.tabs)
	}
	m.press("Esc")
	if m.atContainers() {
		t.Fatal("Esc did not come back up")
	}
	if it, _ := m.currentItem(); it.Name != "b" {
		t.Errorf("back on %q, want b, the pod drilled into", it.Name)
	}
}

// The sort's two steps write the kind's sort to kinds.yaml and show it on
// the header; the direction goes back to the columns, saying the sort it set
// (tdp T1), the names in the titles as they are.
func TestSortTwoSteps(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "Alt-S")
	if !strings.Contains(m.screen_(), "SORT BY: Pods") {
		t.Fatalf("Alt-S opened no sort menu:\n%s", m.screen_())
	}
	m.press("Enter") // Namespace, the first column
	if !strings.Contains(m.screen_(), "DIRECTION: Namespace") {
		t.Fatalf("no direction step:\n%s", m.screen_())
	}
	m.press("j", "Enter") // Descending
	if s := m.screen_(); strings.Contains(s, "DIRECTION:") || !strings.Contains(s, "sorted 1st, descending") {
		t.Errorf("the direction did not go back to the columns:\n%s", s)
	}
	want := config.SortChain{{Column: "Namespace", Direction: config.SortDirectionDescending}}
	if got := m.kindData.GetSort("pod"); !slices.Equal(got, want) {
		t.Errorf("sort %v, want %v", got, want)
	}
	if got := m.list.Sorting(); !slices.Equal(got, []terminu.SortKey{{Col: 0, Desc: true}}) {
		t.Errorf("list sorted by %v", got)
	}
	saved, _ := config.LoadKindData()
	if got := saved.GetSort("pod"); !slices.Equal(got, want) {
		t.Errorf("saved sort %v, want %v", got, want)
	}
}

// Esc on panel 2 goes one level at a time: the anchor, then the drill.
func TestPanel2EscOneLevel(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "Enter") // into a's containers
	m.drill[len(m.drill)-1].containers = append(m.drill[len(m.drill)-1].containers, k8s.ContainerInfo{Name: "side"})
	m.press("Esc")
	if m.atContainers() {
		t.Fatal("Esc did not leave the drill")
	}
	m.press("C") // the anchor on a
	m.press("Esc")
	if m.list.Anchor != "" {
		t.Error("the first Esc kept the anchor")
	}
}

// The Space menu of panel 2 lists what kbu can do to a pod, in tdp M2's
// order, the panel's own after.
func TestPanel2SpaceMenu(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "Space")
	s := m.screen_()
	for _, want := range []string{"ITEM OPERATION", "[Enter] Drill in", "[Y]AML", "[E]dit", "[S]hell", "[D]elete",
		"[C]ompare", "Cop[y]", "PANEL OPERATION", "[Alt-S]ort panel 2 list", "[/] Search", "[z]oom",
		"Global operation"} {
		if !strings.Contains(s, want) {
			t.Errorf("the Space menu has no %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "Exit compare mode") || strings.Contains(s, "Back ") {
		t.Errorf("rows for an anchor or a drill that are not there:\n%s", s)
	}
}

// The global operation popup has kbu's global actions and Quit (tdp M4).
func TestGlobalOperations(t *testing.T) {
	m := newTestModel(t)
	m.press("Space", "G", "Enter")
	s := m.screen_()
	for _, want := range []string{"[N]amespace", "[C]ontext", "[Alt-t]erm", "[q]uit"} {
		if !strings.Contains(s, want) {
			t.Errorf("the global operation popup has no %q:\n%s", want, s)
		}
	}
}

// tdp T1: an edit keeps the YAML viewer it was started from, and when the
// edit ends the viewer reads the resource again: what the API returns, else
// (a CRD) what the watch has now.
func TestAnEditRereadsTheYAML(t *testing.T) {
	m := newTestModel(t)
	before, after := pod("demo", "a"), pod("demo", "a")
	after.Raw.(*corev1.Pod).Labels = map[string]string{"edited": "yes"}
	m.feed(before)
	got := ""
	show := func(y string) { got = y }
	m.drive(m.onMsg(rereadMsg{key: itemKey(before), item: after, show: show}))
	if !strings.Contains(got, "edited") {
		t.Errorf("the viewer did not get what the API read:\n%s", got)
	}
	got = ""
	m.feed(after)
	m.drive(m.onMsg(rereadMsg{key: itemKey(before), err: errors.New("unsupported ref type"), show: show}))
	if !strings.Contains(got, "edited") {
		t.Errorf("the viewer did not get what the watch has:\n%s", got)
	}
	ran := false
	m.drive(m.txExited("edit", func() tea.Cmd { ran = true; return nil })(nil))
	if !ran {
		t.Error("the end of the edit did not read the resource again")
	}
}

// tdp T1: what opens something to look at keeps the menu it was run from
// (Esc goes back to it); a drill clears it.
func TestTheMenuStaysUnderWhatItOpens(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "Space", "Y")
	if !strings.Contains(m.screen_(), "YAML") {
		t.Fatalf("Y from the Space menu opened no YAML:\n%s", m.screen_())
	}
	m.press("Esc")
	if !strings.Contains(m.screen_(), "ITEM OPERATION") {
		t.Errorf("Esc from the YAML did not go back to the menu:\n%s", m.screen_())
	}
	m.press("Esc", "Space", "Enter")
	if strings.Contains(m.screen_(), "ITEM OPERATION") {
		t.Errorf("the drill kept the menu:\n%s", m.screen_())
	}
}

// The YAML viewer's / finds the hits the viewer draws (Viewer.FindText); none
// found is told.
func TestYAMLSearch(t *testing.T) {
	for _, c := range []struct {
		q     string
		found bool
	}{{"kind", true}, {"zzzz", false}} {
		m := newTestModel(t)
		m.feed(pod("demo", "a"), pod("demo", "b"))
		m.press("Tab", "Y", "/")
		for _, r := range c.q {
			m.press(string(r))
		}
		m.press("Enter")
		if told := strings.Contains(m.screen_(), "No match for "+c.q); told == c.found {
			t.Errorf("search %q: no-match toast %v, want %v:\n%s", c.q, told, !c.found, m.screen_())
		}
	}
}

// kbu starts on the panel the user left from (tdp K3: what they expect).
func TestStartsOnThePanelLeftFrom(t *testing.T) {
	for panel, want := range map[string]func(*Model) *terminu.Panel{
		"table":  func(m *Model) *terminu.Panel { return m.listP },
		"detail": func(m *Model) *terminu.Panel { return m.detailP },
	} {
		state := config.DefaultState()
		state.Kind, state.Panel = "pod", panel
		m := newTestModelWith(t, Options{State: state})
		if m.screen.Focused() != want(m) {
			t.Errorf("state panel %q: the focus is not on it", panel)
		}
	}
}

// The namespace picker opens at once, loading (tdp F7), and the namespaces
// fill it when they come.
func TestNamespacePickerLoads(t *testing.T) {
	m := newTestModel(t)
	m.drive(m.namespacePicker(k8s.AllNamespaces()))
	if s := m.screen_(); !strings.Contains(s, "NAMESPACES") || strings.Contains(s, "alpha") {
		t.Fatalf("the picker did not open before its namespaces:\n%s", s)
	}
	m.drive(m.onMsg(namespacesMsg{names: []string{"alpha", "beta"}}))
	if s := m.screen_(); !strings.Contains(s, "alpha") || !strings.Contains(s, "beta") {
		t.Errorf("the namespaces did not fill the picker:\n%s", s)
	}
}

// ! on a panel opens the lib's App Log (tdp K1, F5): what kbu did is in it,
// and a toast is too, by itself.
func TestTheAppLog(t *testing.T) {
	m := newTestModel(t)
	m.log.info("Compare: anchor set on pod/a")
	m.drive(m.toastErr("Drill failed: forbidden"))
	m.press("!")
	s := m.screen_()
	for _, want := range []string{"APP LOG", "Compare: anchor set on pod/a", "Drill failed: forbidden", "Started kbu"} {
		if !strings.Contains(s, want) {
			t.Errorf("the App Log has no %q:\n%s", want, s)
		}
	}
}
