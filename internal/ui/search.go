package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
// context pickers, tdp F1): lit while typing, dimmed once Tab has moved
// focus to the list, so only the part that takes the keys is bright.
func finderSearchBox(query string, typing bool, width int, t *theme.Theme) []string {
	box := renderSearchBox(query, typing, width, t)
	if !typing {
		box = dimANSI(box)
	}
	return strings.Split(box, "\n")
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

	var text string
	if active {
		text = " \U000F0233 " + query + "█"
	} else {
		text = " \U000F0233 " + query
	}

	textW := lipgloss.Width(text)
	if textW > innerW {
		text = text[:innerW-1] + "…"
		textW = lipgloss.Width(text)
	}
	pad := ""
	if textW < innerW {
		pad = strings.Repeat(" ", innerW-textW)
	}

	top := bStyle.Render("╭" + strings.Repeat("─", innerW) + "╮")
	mid := bStyle.Render("│") + text + pad + bStyle.Render("│")
	bot := bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + bot
}

// typedRunes is what a key types into a search line (tdp K8: while
// typing, every printable key is a character). Bubble Tea delivers the
// space bar as KeySpace rather than as a rune, so it is added here —
// otherwise "image: nginx" can't be searched for.
func typedRunes(msg tea.KeyMsg) []rune {
	if msg.Type == tea.KeySpace {
		return []rune{' '}
	}
	return msg.Runes
}
