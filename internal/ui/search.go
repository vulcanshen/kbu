package ui

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vulcanshen/kbu/internal/theme"
)

// renderSearchBox draws the inline search input box, with the border color
// reflecting the input state:
//   - active (`/` pressed, user is typing): default category color (cyan)
//   - inactive with a non-empty query (filter locked, focus back to content):
//     status-pending color (amber/warm yellow) — signals "this is what the
//     view is filtered by, j/k/n navigate within it"
//   - inactive with empty query: default color (rare; caller almost always
//     skips rendering altogether in this state)
func renderSearchBox(query string, active bool, width int, t *theme.Theme) string {
	color := lipgloss.Color(t.Sidebar.CategoryFg)
	if !active && query != "" {
		color = lipgloss.Color(t.Status.Pending)
	}
	return renderSearchBoxWithColor(query, active, width, t, color)
}

// finderSearchBox is the typing line of a finder popup (the namespace and
// context pickers, tdp F1): lit while typing; once Tab has moved focus to
// the list, the whole line — frame, glyph, filter — is Overlay0, the dim
// text colour, with no cursor (D3). A faded copy of its own colours is not
// used: a faded highlight can read brighter than grey.
func finderSearchBox(query string, typing bool, width int, t *theme.Theme) []string {
	lines := strings.Split(renderSearchBox(query, typing, width, t), "\n")
	if typing {
		return lines
	}
	grey := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Overlay0))
	for i, l := range lines {
		lines[i] = grey.Render(ansi.Strip(l))
	}
	return lines
}

// finderCursorStyle is a finder's list cursor row: while the list has
// focus it takes the popup's own colour, like a menu's cursor row; while
// typing (↑/↓ still move it) it keeps the quieter highlight.
func finderCursorStyle(typing bool, bc lipgloss.Color, t *theme.Theme) lipgloss.Style {
	if typing {
		return t.SidebarSelectedStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Background(bc).Bold(true)
}

// renderSearchBoxWithColor renders the search box with a caller-supplied
// border color override. Use when the default active/locked color logic in
// renderSearchBox isn't right for a specific call site.
func renderSearchBoxWithColor(query string, active bool, width int, t *theme.Theme, borderColor lipgloss.Color) string {
	bc := borderColor
	bStyle := lipgloss.NewStyle().Foreground(bc)

	innerW := width - 2
	if innerW < 4 {
		innerW = 4
	}

	top := bStyle.Render("╭" + strings.Repeat("─", innerW) + "╮")
	mid := bStyle.Render("│") + searchLineText(query, active, innerW, t) + bStyle.Render("│")
	bot := bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + bot
}

// searchLineText lays the search glyph, the value and, while typing, the
// cursor out to exactly w cells. A line break in the value is drawn as a
// red \n and a tab as a red \t — two cells, told apart from a typed
// backslash by colour; drawn as they are they would break the box. Text
// wider than w is cut by display width at the edge of a character or a
// mark, never through one, keeping the start, with "…" after the cut and
// padding up to the border.
func searchLineText(query string, active bool, w int, t *theme.Theme) string {
	mark := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Status.Error))
	units := []string{" \U000F0233 "}
	for rest := query; rest != ""; {
		g, _ := ansi.FirstGraphemeCluster(rest, ansi.GraphemeWidth)
		if g == "" {
			break
		}
		rest = rest[len(g):]
		switch g {
		case "\n", "\r", "\r\n":
			g = mark.Render(`\n`)
		case "\t":
			g = mark.Render(`\t`)
		}
		units = append(units, g)
	}
	if active {
		units = append(units, "█")
	}

	total := 0
	for _, u := range units {
		total += dispWidth(u)
	}
	room := w
	if total > w {
		room = w - 1 // the last cell is the "…"
	}
	var b strings.Builder
	used := 0
	for _, u := range units {
		uw := dispWidth(u)
		if used+uw > room {
			break
		}
		b.WriteString(u)
		used += uw
	}
	if total > w {
		b.WriteString("…")
		used++
	}
	return b.String() + strings.Repeat(" ", max(w-used, 0))
}

// typedRunes is what a key types into a search line (tdp K8: while
// typing, every printable key is a character). Bubble Tea delivers the
// space bar as KeySpace rather than as a rune, so it is added here —
// otherwise "image: nginx" can't be searched for. A paste comes as one
// KeyRunes holding whatever was pasted: line breaks and tabs stay in the
// value — \r\n as the one \n — and are drawn as \n and \t
// (searchLineText); every other control character, ESC among them, is
// dropped. Typed keys never carry one, and Tab, Enter or Ctrl-J pressed
// arrive as keys of their own, so they still do what they do.
func typedRunes(msg tea.KeyMsg) []rune {
	if msg.Type == tea.KeySpace {
		return []rune{' '}
	}
	out := make([]rune, 0, len(msg.Runes))
	for i, r := range msg.Runes {
		switch {
		case r == '\r' && i+1 < len(msg.Runes) && msg.Runes[i+1] == '\n':
			// \r\n is one line break: keep the \n that follows
		case r == '\n' || r == '\r' || r == '\t':
			out = append(out, r)
		case unicode.IsControl(r): // the rest of C0, DEL, C1
		default:
			out = append(out, r)
		}
	}
	return out
}
