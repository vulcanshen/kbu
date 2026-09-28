package ui

import (
	"testing"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// tdp K8: while typing, Space is a character — in every search line.
func TestK8_SpaceTypesInEverySearchLine(t *testing.T) {
	space := key(" ")

	tbl := NewTableModel(stackTestApp(t).theme)
	tbl.SetFocused(true)
	tbl, _ = tbl.Update(key("/"))
	tbl, _ = tbl.Update(key("a"))
	tbl, _ = tbl.Update(space)
	tbl, _ = tbl.Update(key("b"))
	if tbl.searchQuery != "a b" {
		t.Errorf("panel 2 search: query %q, want \"a b\"", tbl.searchQuery)
	}

	sb := NewSidebarModel(stackTestApp(t).theme)
	sb.SetFocused(true)
	sb, _ = sb.Update(key("/"))
	sb, _ = sb.Update(key("a"))
	sb, _ = sb.Update(space)
	if sb.searchQuery != "a " {
		t.Errorf("panel 1 search: query %q, want \"a \"", sb.searchQuery)
	}

	m := stackTestApp(t)
	_ = m.yamlPopup.Open("image: nginx\n", k8s.ResourcePods, k8s.ResourceItem{Name: "a"}, "")
	m.yamlPopup.animator.Finalize()
	for _, k := range []string{"/", "e", ":", " ", "n"} {
		m.yamlPopup, _ = m.yamlPopup.Update(key(k))
	}
	if got := m.yamlPopup.SearchQuery(); got != "e: n" {
		t.Errorf("YAML search: query %q, want \"e: n\"", got)
	}

	ns := NewNamespacePickerModel(m.theme)
	openNamespacePicker(&ns)
	ns, _ = ns.Update(key("/"))
	ns, _ = ns.Update(space)
	if ns.searchQuery != " " || !ns.animator.Owns() {
		t.Errorf("namespace filter: query %q (open=%v), want a space typed", ns.searchQuery, ns.animator.Owns())
	}

	ctx := NewContextPickerModel(m.theme)
	_ = ctx.Open([]string{"a b"}, "a b")
	ctx.animator.Finalize()
	ctx, _ = ctx.Update(key("/"))
	ctx, _ = ctx.Update(space)
	if ctx.searchQuery != " " {
		t.Errorf("context filter: query %q, want a space typed", ctx.searchQuery)
	}
}
