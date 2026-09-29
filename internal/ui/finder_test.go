package ui

import (
	"testing"

	"github.com/vulcanshen/kbu/internal/theme"
)

// tdp F1, D3: a finder lights only the part that takes the keys. While
// typing, the typing line is bright and the list's cursor row keeps the
// quiet highlight; once Tab moves focus to the list, the whole typing line
// is Overlay0 grey — frame, glyph and filter, no background, no cursor —
// and the cursor row takes the popup's own colour, like a menu's.
func TestF1_FinderLightsThePartWithFocus(t *testing.T) {
	truecolor(t)
	m := stackTestApp(t)
	ns := m.namespacePicker
	ns.SetSize(m.width, m.height)
	_ = ns.OpenLoading()
	ns.SetNamespaces([]string{"kube-system", "default"})
	ns.searchQuery = "kube"
	ctx := m.contextPicker
	ctx.SetSize(m.width, m.height)
	_ = ctx.Open([]string{"kube-prod", "orbstack"}, "orbstack")
	ctx.searchQuery = "kube"
	ctx.cursor = 0 // Open put it on orbstack, which the filter hides

	for _, c := range []struct {
		name   string
		draw   func(typing bool) string
		row    string
		border [3]int
	}{
		{"namespace", func(typing bool) string { ns.searching = typing; return ns.renderFullPopup() }, "kube-system", hexRGB(string(ns.borderColor))},
		{"context", func(typing bool) string { ctx.searching = typing; return ctx.renderFullPopup() }, "kube-prod", hexRGB(string(ctx.borderColor))},
	} {
		typing := c.draw(true)
		if got := screenCells(typing)[2][1]; got.r != '╭' || !near(got.fg, hexRGB(m.theme.Sidebar.CategoryFg)) {
			t.Errorf("%s, typing: the typing line's corner %q is %v, want it lit", c.name, got.r, got.fg)
		}
		row, at := cellsOf(t, typing, c.row)
		if !near(row[at].bg, hexRGB(m.theme.Sidebar.SelectedBg)) {
			t.Errorf("%s, typing: the cursor row's background is %v, want the quiet highlight", c.name, row[at].bg)
		}

		list := c.draw(false)
		cells := screenCells(list)
		if cells[2][1].r != '╭' {
			t.Fatalf("%s, list: the typing line is not on row 2", c.name)
		}
		for _, rowCells := range cells[2:5] {
			for _, cl := range rowCells[1 : len(rowCells)-1] {
				switch {
				case cl.r == '█':
					t.Errorf("%s, list: the typing line draws a cursor", c.name)
				case cl.bg != unset:
					t.Errorf("%s, list: %q in the typing line has a background %v", c.name, cl.r, cl.bg)
				case cl.r != ' ' && cl.fg != hexRGB(theme.Overlay0):
					t.Errorf("%s, list: %q in the typing line is %v, want Overlay0", c.name, cl.r, cl.fg)
				}
			}
		}
		row, at = cellsOf(t, list, c.row)
		if !near(row[at].bg, c.border) {
			t.Errorf("%s, list: the cursor row's background is %v, want the popup's colour %v", c.name, row[at].bg, c.border)
		}
	}
}
