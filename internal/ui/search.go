package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

func renderSearchBox(query string, active bool, width int, t *theme.Theme) string {
	return renderSearchBoxWithColor(query, active, width, t, lipgloss.Color(t.Sidebar.CategoryFg))
}

// renderSearchBoxWithColor renders the search box with a caller-supplied
// border color. Use to express "filter locked" state (e.g. amber/orange after
// the user commits a query with Enter and focus shifts back to content).
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
