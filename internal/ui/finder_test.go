package ui

import (
	"testing"
)

// tdp F1, F8: a finder lights only the part that takes the keys. While
// typing, the typing line is bright and the list's cursor row keeps the
// quiet highlight; once Tab moves focus to the list, the typing line dims
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
		if got := screenCells(list)[2][1]; got.r != '╭' || !near(got.fg, dimRGB(hexRGB(m.theme.Status.Pending))) {
			t.Errorf("%s, list: the typing line's corner %q is %v, want it dimmed", c.name, got.r, got.fg)
		}
		row, at = cellsOf(t, list, c.row)
		if !near(row[at].bg, c.border) {
			t.Errorf("%s, list: the cursor row's background is %v, want the popup's colour %v", c.name, row[at].bg, c.border)
		}
	}
}
