package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

// paste is a bracketed paste: Bubble Tea hands the whole text over as one
// KeyRunes, line breaks, tabs and escapes included.
func paste(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true}
}

// searchLine is one of kbu's five search lines, typing: a way to send it a
// key and to read its value back.
type searchLine struct {
	name  string
	send  func(tea.KeyMsg)
	value func() string
}

func openSearchLines(t *testing.T) []searchLine {
	t.Helper()
	m := stackTestApp(t)

	sb := NewSidebarModel(m.theme)
	sb.SetFocused(true)
	sb, _ = sb.Update(key("/"))

	tbl := NewTableModel(m.theme)
	tbl.SetFocused(true)
	tbl, _ = tbl.Update(key("/"))

	ns := NewNamespacePickerModel(m.theme)
	openNamespacePicker(&ns)
	ns, _ = ns.Update(key("/"))

	ctx := NewContextPickerModel(m.theme)
	_ = ctx.Open([]string{"a"}, "a")
	ctx.animator.Finalize()
	ctx, _ = ctx.Update(key("/"))

	yp := m.yamlPopup
	_ = yp.Open("image: nginx\n", k8s.ResourcePods, k8s.ResourceItem{Name: "a"})
	yp.animator.Finalize()
	yp, _ = yp.Update(key("/"))

	return []searchLine{
		{"panel 1 search", func(k tea.KeyMsg) { sb, _ = sb.Update(k) }, func() string { return sb.searchQuery }},
		{"panel 2 search", func(k tea.KeyMsg) { tbl, _ = tbl.Update(k) }, func() string { return tbl.searchQuery }},
		{"namespace filter", func(k tea.KeyMsg) { ns, _ = ns.Update(k) }, func() string { return ns.searchQuery }},
		{"context filter", func(k tea.KeyMsg) { ctx, _ = ctx.Update(k) }, func() string { return ctx.searchQuery }},
		{"YAML search", func(k tea.KeyMsg) { yp, _ = yp.Update(k) }, func() string { return yp.searchQuery }},
	}
}

// A pasted value keeps its line breaks and tabs — \r\n as one line break,
// a lone \r as one too — and loses every other control character: ESC,
// DEL, a C1 code. Each of the five search lines.
func TestSearchLine_PasteKeepsBreaksAndTabs(t *testing.T) {
	const want = "a\nb\tcdef\rg"
	for _, l := range openSearchLines(t) {
		l.send(paste("a\r\nb\tc\x1bd\x7fe\u0085f\rg"))
		if got := l.value(); got != want {
			t.Errorf("%s: pasted value is %q, want %q", l.name, got, want)
		}
	}
}

// Backspace — and Alt-Backspace, the same key to Bubble Tea — takes off
// the last character whole: a pasted line break, a Nerd Font icon (4
// bytes), a CJK character (3 bytes), never a byte of one. Each of the five
// search lines.
func TestSearchLine_BackspaceTakesOffACharacter(t *testing.T) {
	for _, l := range openSearchLines(t) {
		l.send(key("a中\U000F0233"))
		l.send(paste("\r\n"))
		for i, want := range []string{"a中\U000F0233", "a中", "a", ""} {
			bs := tea.KeyMsg{Type: tea.KeyBackspace, Alt: i%2 == 1}
			l.send(bs)
			if got := l.value(); got != want {
				t.Errorf("%s: after %d Backspace (alt %v) the value is %q, want %q", l.name, i+1, bs.Alt, got, want)
				break
			}
		}
	}
}

// boxMid is the search box's value row with its styles stripped.
func boxMid(t *testing.T, box string) string {
	t.Helper()
	lines := strings.Split(box, "\n")
	if len(lines) != 3 {
		t.Fatalf("search box has %d rows, want 3:\n%s", len(lines), box)
	}
	return ansi.Strip(lines[1])
}

// A line break or tab is drawn as a red \n or \t, two cells; a typed
// backslash and n stay in the value's own colour. The box keeps its shape.
func TestSearchLine_DrawsBreaksAndTabsInRed(t *testing.T) {
	truecolor(t)
	th := theme.DefaultTheme()
	box := renderSearchBox("a\\n\nb\tc\rd", true, 30, th)
	for i, l := range strings.Split(box, "\n") {
		if w := dispWidth(l); w != 30 {
			t.Errorf("row %d is %d cells, want 30: %q", i, w, ansi.Strip(l))
		}
	}
	mid := boxMid(t, box)
	if want := "│ \U000F0233 a\\n\\nb\\tc\\nd█"; !strings.HasPrefix(mid, want) {
		t.Fatalf("value row %q, want it to start %q", mid, want)
	}
	cells := screenCells(box)[1]
	red := hexRGB(th.Status.Error)
	at := 4 // │, space, glyph, space
	for i, wantRed := range []bool{false, false, false, true, true, false, true, true, false, true, true, false} {
		c := cells[at+i]
		if got := near(c.fg, red); got != wantRed {
			t.Errorf("cell %d %q: red = %v, want %v", i, c.r, got, wantRed)
		}
	}
}

// A finder's typing line, once the list has focus, is grey through and
// through: the \n mark too, not red.
func TestSearchLine_GreyLineGreysTheMarks(t *testing.T) {
	truecolor(t)
	th := theme.DefaultTheme()
	lines := finderSearchBox("a\nb", false, 30, th)
	if got := ansi.Strip(lines[1]); !strings.Contains(got, "a\\nb") {
		t.Fatalf("value row %q, want the line break drawn as \\n", got)
	}
	for _, c := range screenCells(lines[1])[0] {
		if c.r != ' ' && c.fg != hexRGB(theme.Overlay0) {
			t.Errorf("%q is %v, want Overlay0", c.r, c.fg)
		}
	}
}

// A value too long for the box is cut by display width — never through a
// wide character or a \n mark — with … and padding up to the right border,
// the start kept. With icons one cell or two: a value that fits beside a
// one-cell icon is cut beside a two-cell one.
func TestSearchLine_CutsByDisplayWidth(t *testing.T) {
	th := theme.DefaultTheme()
	for _, cells := range []int{1, 2} {
		prev := iconCells
		iconCells = cells
		for _, c := range []struct {
			query string
			want  [2]string // with one-cell icons, with two-cell icons
		}{
			{strings.Repeat("中文", 6), [2]string{"中文中文中文中…", "中文中文中文中…"}},
			{"abcdefghijklmn\nxyz", [2]string{"abcdefghijklmn…", "abcdefghijklmn…"}},
			{"abcdefghijklmnop", [2]string{"abcdefghijklmnop│", "abcdefghijklmn…"}},
		} {
			want := c.want[cells-1]
			box := renderSearchBox(c.query, false, 21, th)
			for i, l := range strings.Split(box, "\n") {
				if w := dispWidth(l); w != 21 {
					t.Errorf("icons %d, %q: row %d is %d cells, want 21: %q", cells, c.query, i, w, ansi.Strip(l))
				}
			}
			mid := boxMid(t, box)
			if raw := strings.Split(box, "\n")[1]; !utf8.ValidString(raw) {
				t.Errorf("icons %d, %q: value row %q has a broken character", cells, c.query, raw)
			}
			if !strings.Contains(mid, " \U000F0233 "+want) || !strings.HasSuffix(mid, "│") {
				t.Errorf("icons %d, %q: value row %q, want %q from the start, up to the border", cells, c.query, mid, want)
			}
		}
		iconCells = prev
	}
}
