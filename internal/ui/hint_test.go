package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

// bottomHintOf is the hint in a popup's bottom border: what sits between
// the corner (and its lead dash) and the dashes after it.
func bottomHintOf(popup string) string {
	lines := strings.Split(ansi.Strip(popup), "\n")
	last := strings.TrimPrefix(lines[len(lines)-1], "╰")
	last = strings.TrimPrefix(last, "─")
	if i := strings.Index(last, "─"); i >= 0 {
		last = last[:i]
	}
	return last
}

// popupHints draws every popup in each of its states and returns the hint
// its bottom border shows, by name.
func popupHints(t *testing.T) map[string]string {
	t.Helper()
	m := stackTestApp(t)
	w, h := m.width, m.height
	got := map[string]string{}

	openTestSpaceMenu(&m)
	got["menu"] = bottomHintOf(m.spaceMenu.renderFullPopup())

	m.listPicker.SetSize(w, h)
	_ = m.listPicker.Open("sort", "Sort", []ListPickerItem{{Key: "name", Label: "Name"}})
	got["sort"] = bottomHintOf(m.listPicker.renderFullPopup())

	m.settingsPopup.SetSize(w, h)
	_ = m.settingsPopup.Open([]SettingsItem{{Key: "scroll", Label: "Scroll"}})
	got["settings"] = bottomHintOf(m.settingsPopup.renderFullPopup())

	m.breadcrumbPopup.SetSize(w, h)
	_ = m.breadcrumbPopup.Open([]k8s.RefTarget{{Type: k8s.ResourcePods, Name: "a"}, {Type: k8s.ResourcePods, Name: "b"}})
	got["breadcrumb"] = bottomHintOf(m.breadcrumbPopup.renderFullPopup())

	m.namespacePicker.SetSize(w, h)
	_ = m.namespacePicker.OpenLoading()
	m.namespacePicker.SetNamespaces([]string{"default"})
	got["namespace list"] = bottomHintOf(m.namespacePicker.renderFullPopup())
	m.namespacePicker.searching = true
	got["namespace typing"] = bottomHintOf(m.namespacePicker.renderFullPopup())

	m.contextPicker.SetSize(w, h)
	_ = m.contextPicker.Open([]string{"orbstack"}, "orbstack")
	got["context list"] = bottomHintOf(m.contextPicker.renderFullPopup())
	m.contextPicker.searching = true
	got["context typing"] = bottomHintOf(m.contextPicker.renderFullPopup())

	m.help.SetSize(w, h)
	_ = m.help.Open("Keys", []helpRow{{key: "y", desc: "copy"}})
	got["key reference"] = bottomHintOf(m.help.renderFullPopup())

	m.appLog.SetSize(w, h)
	m.appLog.Info("hello")
	got["app log"] = bottomHintOf(m.appLog.renderFullPopup())

	m.comparePopup.SetSize(w, h)
	_ = m.comparePopup.Open("a: 1\n", "a: 2\n", "l", "r")
	got["compare"] = bottomHintOf(m.comparePopup.renderFrame())

	y := yamlOpenFor(t, k8s.ResourcePods, k8s.ResourceItem{Name: "nginx", Namespace: "default"})
	got["yaml"] = bottomHintOf(y.yamlPopup.renderFullPopup())
	y.yamlPopup.visualMode = true
	got["yaml selection"] = bottomHintOf(y.yamlPopup.renderFullPopup())

	for name, kind := range map[string]PtyKind{"alterm": PtyKindShell, "edit": PtyKindEdit, "exec": PtyKindExec} {
		got[name] = bottomHintOf(hookedPtyView(kind).RenderPopup())
	}
	return got
}

// tdp M5: every hint is key:description, no space around the colon,
// entries one space apart; D3 and D4 give the confirm's and the menu's
// word for word. / and Tab are two entries on the pickers: they don't do
// the same thing.
func TestM5_PopupHintsAreKeyColonDescription(t *testing.T) {
	want := map[string]string{
		"menu":             " j/k:move Enter:run Esc:close ",
		"sort":             " j/k:move Enter:pick Esc:cancel ",
		"settings":         " j/k:move Enter:toggle Esc:close ",
		"breadcrumb":       " j/k:move Enter:switch Esc:close ",
		"namespace list":   " Enter:toggle /:new filter Tab:filter Esc:close ",
		"namespace typing": " ↑/↓:move Enter:toggle Tab:list Esc:close ",
		"context list":     " Enter:select /:new filter Tab:filter Esc:close ",
		"context typing":   " ↑/↓:move Enter:select Tab:list Esc:close ",
		"key reference":    " j/k:scroll ?/Esc:close ",
		"app log":          " j/k:scroll u/d:page y:copy D:clear Esc:close ",
		"compare":          " L:layout j/k:scroll Esc:close ",
		"yaml":             " v:visual y:copy E:edit /:search Esc:close ",
		"yaml selection":   " ?:keys y:copy v/Esc:leave ",
		"alterm":           " Alt-t:hide Alt-Esc:end PgUp/Home:scroll ",
		"edit":             " Alt-Esc:leave PgUp/Home:scroll ",
		"exec":             " Alt-Esc:leave PgUp/Home:scroll ",
	}
	got := popupHints(t)
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s hint is %q, want %q", name, got[name], w)
		}
	}
}

// lowerKeyName is a key cap written in lower case (esc, enter).
var lowerKeyName = regexp.MustCompile(`\b(esc|enter|tab|space)\b`)

// tdp M5 across every hint kbu draws — the popups, the panel borders and
// the footer: no "key: desc", no double spaces, no " · ", no key cap in
// lower case.
func TestM5_NoHintInTheOldShape(t *testing.T) {
	var all []string
	for _, h := range popupHints(t) {
		all = append(all, strings.TrimSpace(h))
	}
	m := stackTestApp(t)
	all = append(all, strings.TrimSpace(m.statusLine.layoutLine()))
	m.statusLine.SetDragMode(true)
	all = append(all, strings.TrimSpace(m.statusLine.layoutLine()))
	m.currentResource = k8s.ResourcePods
	all = append(all, hintText(m.tablePanelBottomLeft()))
	for _, tab := range []string{"Logs", "Relatives", "Events"} {
		m.detail.SetResourceType(k8s.ResourcePods)
		m.detail.SwitchToTabByName(tab)
		all = append(all, hintText(m.detail.BorderBottomLeftHint()))
	}
	for _, s := range all {
		switch {
		case s == "":
			t.Error("a hint came out empty")
		case strings.Contains(s, ": "):
			t.Errorf("%q: nothing goes between the colon and the description", s)
		case strings.Contains(s, "  "):
			t.Errorf("%q: entries are one space apart", s)
		case strings.Contains(s, "·"):
			t.Errorf("%q: entries are separated by a space, not a dot", s)
		case lowerKeyName.MatchString(s):
			t.Errorf("%q: key caps are written Esc, Enter, Tab, Space", s)
		}
	}
}

// tdp D2 in a popup: the key Blue, the colon and description Overlay0.
func TestD2_PopupHintKeyAndDescriptionColours(t *testing.T) {
	truecolor(t)
	m := stackTestApp(t)
	openTestSpaceMenu(&m)
	row, at := cellsOf(t, m.spaceMenu.renderFullPopup(), "j/k:move")
	checkHintCells(t, "menu", row, at, "j/k", theme.Blue, theme.Overlay0)
}

// checkHintCells checks the entry starting at cell at: the key in key,
// then the colon and the first letter of the description in desc.
func checkHintCells(t *testing.T, where string, row []cell, at int, k, key, desc string) {
	t.Helper()
	if got := row[at].fg; !near(got, hexRGB(key)) {
		t.Errorf("%s: the key %q is drawn %v, want %s", where, k, got, key)
	}
	colon := at + len([]rune(k))
	if row[colon].r != ':' {
		t.Fatalf("%s: no colon after %q", where, k)
	}
	for _, c := range []cell{row[colon], row[colon+1]} {
		if !near(c.fg, hexRGB(desc)) {
			t.Errorf("%s: %q after the key is drawn %v, want %s", where, c.r, c.fg, desc)
		}
	}
}

// The panel border hint: the family pair on the focused panel, a darker
// pair on an unfocused one — two colours either way.
func TestD2_PanelHintColoursFollowFocus(t *testing.T) {
	truecolor(t)
	hints := []keyHint{{"Enter", "drill"}}
	for _, c := range []struct {
		focused   bool
		key, desc string
	}{{true, theme.Blue, theme.Overlay0}, {false, theme.Overlay0, theme.Surface2}} {
		drawn := renderPanelWithScroll("x", "t", 30, 5, c.focused, theme.DefaultTheme(), nil, "", hints)
		row, at := cellsOf(t, drawn, "Enter:drill")
		checkHintCells(t, "panel border", row, at, "Enter", c.key, c.desc)
	}
}

// The footer: the key keeps the theme's footer colour (Blue by default),
// the colon and description Overlay0.
func TestD2_FooterColours(t *testing.T) {
	truecolor(t)
	m := stackTestApp(t)
	row, at := cellsOf(t, m.statusLine.layoutLine(), "Space:menu")
	checkHintCells(t, "footer", row, at, "Space", theme.Blue, theme.Overlay0)
}
