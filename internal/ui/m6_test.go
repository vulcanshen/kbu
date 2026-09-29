package ui

import (
	"strings"
	"testing"

	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

var helmLabels = map[string]string{"app.kubernetes.io/managed-by": "Helm"}

// refRowFor is the first key-reference row for key k.
func refRowFor(t *testing.T, rows []helpRow, k string) helpRow {
	t.Helper()
	for _, r := range rows {
		if !r.header && r.key == k {
			return r
		}
	}
	t.Fatalf("the key reference has no %q row: %v", k, refKeys(rows))
	return helpRow{}
}

// m6Case builds the app with an action that can or can't run right now.
type m6Case struct {
	name  string
	build func(t *testing.T, cantRun bool) AppModel
	keys  []string // the rows that are dimmed when the action can't run
}

func m6Cases() []m6Case {
	pinned := func(t *testing.T, cantRun bool) AppModel {
		m := menuFixture(t, nil, 0)
		m.activePanel = SidebarPanel
		kinds := []k8s.ResourceType{k8s.ResourcePods, k8s.ResourceDeployments}
		if cantRun {
			kinds = kinds[:1] // one pinned kind: nothing to drag it past
		}
		m.sidebar.SetPinned(kinds)
		m.sidebar.SnapCursorToKind(k8s.ResourcePods)
		return m
	}
	panel2 := func(t *testing.T, items []k8s.ResourceItem) AppModel {
		m := menuFixture(t, items, 0)
		m.activePanel = TablePanel
		m.currentResource = k8s.ResourcePods
		return m
	}
	helm := func(t *testing.T, cantRun bool) AppModel {
		var labels map[string]string
		if cantRun {
			labels = helmLabels
		}
		return panel2(t, []k8s.ResourceItem{podItem("a", labels), podItem("b", nil)})
	}
	oneRow := func(t *testing.T, cantRun bool) AppModel {
		items := []k8s.ResourceItem{podItem("a", nil), podItem("b", nil)}
		if cantRun {
			items = items[:1] // the only row: nothing to compare it with
		}
		return panel2(t, items)
	}
	oneTab := func(t *testing.T, cantRun bool) AppModel {
		m := stackTestApp(t)
		m.setPanel(DetailPanel)
		rt := k8s.ResourcePods
		if cantRun {
			rt = k8s.ResourceContexts // a single Info tab
		}
		m.detail.SetResourceType(rt)
		return m
	}
	spaceMenu := func(t *testing.T, cantRun bool) AppModel {
		m := stackTestApp(t)
		var labels map[string]string
		if cantRun {
			labels = helmLabels
		}
		openPodMenu(&m.spaceMenu, k8s.ResourcePods, podItem("a", labels), panel2CompareCtx{canLock: true})
		return m
	}
	return []m6Case{
		{"panel 1 drag with one pinned kind", pinned, []string{"D"}},
		{"panel 2 edit and delete on a helm-managed row", helm, []string{"E", "D"}},
		{"panel 2 compare anchor on the only row", oneRow, []string{"C"}},
		{"panel 3 tab keys with one tab", oneTab, []string{"l", "h/[", "l/]"}},
		{"Space menu edit and delete on a helm-managed row", spaceMenu, []string{"E", "D"}},
	}
}

// tdp M6: ? lists a key whose target exists but that can't run right now,
// dimmed — the same row the menu dims. When it can run, the same row is
// bright.
func TestM6_KeyReferenceDimsWhatCantRunNow(t *testing.T) {
	for _, c := range m6Cases() {
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t, true)
			_, rows := m.keyRef()
			for _, k := range c.keys {
				if !refRowFor(t, rows, k).dim {
					t.Errorf("%q can't run here: ? must list it dimmed", k)
				}
			}
			m = c.build(t, false)
			_, rows = m.keyRef()
			for _, k := range c.keys {
				if refRowFor(t, rows, k).dim {
					t.Errorf("%q can run here: ? must list it bright", k)
				}
			}
		})
	}
}

// tdp M6 on the namespace picker: while its list loads, only Esc works —
// the list keys are dimmed. The "while typing" section describes another
// surface (v0.1.16) and stays bright. Once the list is in, all are bright.
func TestM6_NamespacePickerLoadingDimsTheListKeys(t *testing.T) {
	m := stackTestApp(t)
	m.namespacePicker.SetSize(m.width, m.height)
	_ = m.namespacePicker.OpenLoading()
	m.namespacePicker.animator.Finalize()

	_, rows := m.keyRef()
	typing := false
	for _, r := range rows {
		if r.header {
			typing = r.desc == "while typing"
			continue
		}
		switch {
		case typing && r.dim:
			t.Errorf("while typing, %q is another surface's key: it must stay bright", r.key)
		case !typing && r.key == "Esc" && r.dim:
			t.Error("Esc closes the loading picker: it must stay bright")
		case !typing && r.key != "Esc" && !r.dim:
			t.Errorf("%q does nothing while the list loads: it must be dimmed", r.key)
		}
	}
	if !typing {
		t.Fatal("the picker's key reference lost its while-typing section")
	}

	m.namespacePicker.SetNamespaces([]string{"default"})
	_, rows = m.keyRef()
	for _, r := range rows {
		if r.dim {
			t.Errorf("the list is in: %q must be bright", r.key)
		}
	}
}

// cellsOf finds the line of a drawn popup holding text and returns its
// cells and where text starts.
func cellsOf(t *testing.T, drawn, text string) ([]cell, int) {
	t.Helper()
	for _, row := range screenCells(drawn) {
		rs := make([]rune, len(row))
		for i, c := range row {
			rs[i] = c.r
		}
		s := string(rs)
		if i := strings.Index(s, text); i >= 0 {
			return row, len([]rune(s[:i]))
		}
	}
	t.Fatalf("%q is not drawn", text)
	return nil, 0
}

// The dimmed row is drawn dimmed, key and description: Overlay0, where a
// bright row has a Blue key and a Text description (tdp D2).
func TestM6_DimmedRowIsDrawnDimmed(t *testing.T) {
	truecolor(t)
	m := pressQuestion(t, m6Cases()[1].build(t, true)) // helm-managed row
	drawn := m.help.renderFullPopup()

	row, at := cellsOf(t, drawn, "Edit — kubectl edit")
	if got := row[at].fg; !near(got, hexRGB(theme.Overlay0)) {
		t.Errorf("dimmed description drawn %v, want Overlay0", got)
	}
	if got := row[4].fg; row[4].r != 'E' || !near(got, hexRGB(theme.Overlay0)) {
		t.Errorf("dimmed key %q drawn %v, want E in Overlay0", row[4].r, got)
	}

	row, at = cellsOf(t, drawn, "YAML — view resource manifest")
	if got := row[at].fg; !near(got, hexRGB("#cdd6f4")) {
		t.Errorf("bright description drawn %v, want Text", got)
	}
	if got := row[4].fg; row[4].r != 'Y' || !near(got, hexRGB("#89b4fa")) {
		t.Errorf("bright key %q drawn %v, want Y in Blue", row[4].r, got)
	}
}
