package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
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
