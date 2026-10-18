package tui

import (
	"slices"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// The namespace picker's checks: All, some, back to All.
func TestPickedSelection(t *testing.T) {
	names := []string{"a", "b", "c"}
	all := k8s.AllNamespaces()
	some := k8s.SelectedNamespaces("a")
	cases := []struct {
		name string
		was  k8s.NamespaceSelection
		now  []bool
		want []string // nil for All
	}{
		{"All, then a checked", all, []bool{true, true, false, false}, []string{"a"}},
		{"All unchecked, nothing else", all, []bool{false, false, false, false}, nil},
		{"a, then b too", some, []bool{false, true, true, false}, []string{"a", "b"}},
		{"a, then All checked again", some, []bool{true, true, false, false}, nil},
		{"a, then unchecked", some, []bool{false, false, false, false}, nil},
	}
	for _, c := range cases {
		got := pickedSelection(c.was, names, c.now)
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

// The sort's two steps write the kind's sort to the config and show it on
// the header.
func TestSortTwoSteps(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "Alt-S")
	if !strings.Contains(m.screen_(), "SORT PODS BY") {
		t.Fatalf("Alt-S opened no sort menu:\n%s", m.screen_())
	}
	m.press("Enter") // Namespace, the first column
	if !strings.Contains(m.screen_(), "SORT PODS BY NAMESPACE") {
		t.Fatalf("no direction step:\n%s", m.screen_())
	}
	m.press("j", "Enter") // Descending
	want := config.SortChain{{Column: "Namespace", Direction: config.SortDirectionDescending}}
	if got := m.cfg.GetSort("pod"); !slices.Equal(got, want) {
		t.Errorf("sort %v, want %v", got, want)
	}
	if got := m.list.Sorting(); !slices.Equal(got, []terminu.SortKey{{Col: 0, Desc: true}}) {
		t.Errorf("list sorted by %v", got)
	}
	saved, _ := config.Load()
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
	if m.anchor != nil {
		t.Error("the first Esc kept the anchor")
	}
}

// The Space menu of panel 2 lists what kbu can do to a pod, in Rules M2's
// order, the panel's own after.
func TestPanel2SpaceMenu(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "Space")
	s := m.screen_()
	for _, want := range []string{"ITEM OPERATION", "[Enter] Drill in", "[Y]AML", "[E]dit", "[S]hell", "[D]elete",
		"Mark as [C]ompare anchor", "Cop[y]", "PANEL OPERATION", "[Alt-S]ort panel 2 list", "[/] Search", "[z] Zoom", "Global operation"} {
		if !strings.Contains(s, want) {
			t.Errorf("the Space menu has no %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "Exit compare mode") || strings.Contains(s, "Back ") {
		t.Errorf("rows for an anchor or a drill that are not there:\n%s", s)
	}
}

// The global operation popup has kbu's global actions and Quit (Rules M4).
func TestGlobalOperations(t *testing.T) {
	m := newTestModel(t)
	m.press("Space", "G", "Enter")
	s := m.screen_()
	for _, want := range []string{"[N]amespace", "[C]ontext", "[Alt-t]erm", "[!] App log", "[q] Quit"} {
		if !strings.Contains(s, want) {
			t.Errorf("the global operation popup has no %q:\n%s", want, s)
		}
	}
}
