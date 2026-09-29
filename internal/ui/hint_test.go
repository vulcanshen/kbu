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
	if i := strings.IndexAny(last, "─╯"); i >= 0 {
		last = last[:i]
	}
	return last
}

// drawnPopups draws every popup in each of its states on a w-column
// terminal, by name.
func drawnPopups(t *testing.T, w int) map[string]string {
	t.Helper()
	m := stackTestApp(t)
	m.width = w
	h := m.height
	got := map[string]string{}

	openTestSpaceMenu(&m)
	got["menu"] = m.spaceMenu.renderFullPopup()

	m.listPicker.SetSize(w, h)
	_ = m.listPicker.Open("sort", "Sort", []ListPickerItem{{Key: "name", Label: "Name"}})
	got["sort"] = m.listPicker.renderFullPopup()

	m.settingsPopup.SetSize(w, h)
	_ = m.settingsPopup.Open([]SettingsItem{{Key: "scroll", Label: "Scroll"}})
	got["settings"] = m.settingsPopup.renderFullPopup()

	m.breadcrumbPopup.SetSize(w, h)
	_ = m.breadcrumbPopup.Open([]k8s.RefTarget{{Type: k8s.ResourcePods, Name: "a"}, {Type: k8s.ResourcePods, Name: "b"}})
	got["breadcrumb"] = m.breadcrumbPopup.renderFullPopup()

	m.namespacePicker.SetSize(w, h)
	_ = m.namespacePicker.OpenLoading()
	m.namespacePicker.SetNamespaces([]string{"default"})
	got["namespace list"] = m.namespacePicker.renderFullPopup()
	m.namespacePicker.searching = true
	got["namespace typing"] = m.namespacePicker.renderFullPopup()

	m.contextPicker.SetSize(w, h)
	_ = m.contextPicker.Open([]string{"orbstack"}, "orbstack")
	got["context list"] = m.contextPicker.renderFullPopup()
	m.contextPicker.searching = true
	got["context typing"] = m.contextPicker.renderFullPopup()

	m.help.SetSize(w, h)
	_ = m.help.Open("Keys", []helpRow{{key: "y", desc: "copy"}})
	got["key reference"] = m.help.renderFullPopup()

	m.appLog.SetSize(w, h)
	m.appLog.Info("hello")
	got["app log"] = m.appLog.renderFullPopup()

	m.comparePopup.SetSize(w, h)
	_ = m.comparePopup.Open("a: 1\n", "a: 2\n", "l", "r")
	got["compare"] = m.comparePopup.renderFrame()

	m.confirm.SetSize(w, h)
	_ = m.confirm.Show(ConfirmDelete, "Delete pod/a?", "kubectl delete pod a", nil)
	got["confirm"] = m.confirm.renderFullPopup()

	y := yamlOpenFor(t, k8s.ResourcePods, k8s.ResourceItem{Name: "nginx", Namespace: "default"})
	y.yamlPopup.SetSize(w, h)
	got["yaml"] = y.yamlPopup.renderFullPopup()
	y.yamlPopup.visualMode = true
	got["yaml selection"] = y.yamlPopup.renderFullPopup()

	for name, kind := range map[string]PtyKind{"alterm": PtyKindShell, "edit": PtyKindEdit, "exec": PtyKindExec} {
		p := hookedPtyView(kind)
		p.term.Resize(w-4, 20) // F7: a terminal fills W − 2, its frame takes 2 more
		got[name] = p.RenderPopup()
	}
	return got
}

// popupHints is the hint each popup's bottom border shows at 120 columns.
func popupHints(t *testing.T) map[string]string {
	t.Helper()
	got := map[string]string{}
	for name, drawn := range drawnPopups(t, 120) {
		got[name] = bottomHintOf(drawn)
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
		"yaml selection":   " ?:keys h/j/k/l:select y:copy v/Esc:leave ",
		"alterm":           " Alt-Esc:end Alt-t:hide PgUp/Home:scroll ",
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

// tdp D3: a bottom-border hint that doesn't fit drops whole entries from
// the end — never half of one — and the border stays the box's width (L4).
// At 26 columns every popup is at its narrowest and no hint fits whole. A
// terminal's exit key comes first, so it is the last to go (K10).
func TestD3_NarrowHintsDropWholeEntries(t *testing.T) {
	full := popupHints(t)
	for _, w := range []int{40, 26} {
		for name, drawn := range drawnPopups(t, w) {
			lines := strings.Split(ansi.Strip(drawn), "\n")
			top, bottom := lines[0], lines[len(lines)-1]
			if ansi.StringWidth(bottom) != ansi.StringWidth(top) {
				t.Errorf("%d cols, %s: the bottom border is %d wide, the box %d: %q", w, name, ansi.StringWidth(bottom), ansi.StringWidth(top), bottom)
			}
			got, want := strings.TrimSpace(bottomHintOf(drawn)), strings.TrimSpace(full[name])
			switch {
			case got == "":
				t.Errorf("%d cols, %s: no hint entry fits at all", w, name)
			case !strings.HasPrefix(want, got) || (len(got) < len(want) && want[len(got)] != ' '):
				t.Errorf("%d cols, %s: %q is not whole entries of %q", w, name, got, want)
			case w == 26 && got == want:
				t.Errorf("%d cols, %s: %q fits whole; the test no longer measures dropping", w, name, got)
			case (name == "alterm" || name == "edit" || name == "exec") && !strings.HasPrefix(got, "Alt-Esc:"):
				t.Errorf("%d cols, %s: the exit key must stay, got %q", w, name, got)
			}
		}
	}
}

// Panel borders too: entries that don't fit beside the scroll indicator
// drop whole from the end; the indicator stays and the row keeps its width.
func TestD3_PanelHintDropsBesideTheIndicator(t *testing.T) {
	hints := []keyHint{{"u/d", "page"}, {"gg", "top"}, {"G", "live"}}
	drawn := renderPanelWithScroll("x", "t", 30, 5, true, theme.DefaultTheme(), &ScrollInfo{Position: 5, Total: 27}, "", hints, "")
	lines := strings.Split(ansi.Strip(drawn), "\n")
	bottom := lines[len(lines)-1]
	if ansi.StringWidth(bottom) != 30 {
		t.Errorf("the bottom border is %d wide, want 30: %q", ansi.StringWidth(bottom), bottom)
	}
	if !strings.Contains(bottom, "u/d:page gg:top") || strings.Contains(bottom, "G:") || !strings.Contains(bottom, " 5 of 27 ") {
		t.Errorf("want whole entries up to gg:top beside the indicator, got %q", bottom)
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
		drawn := renderPanelWithScroll("x", "t", 30, 5, c.focused, theme.DefaultTheme(), nil, "", hints, "")
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
