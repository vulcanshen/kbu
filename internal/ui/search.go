package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

func renderSearchBox(query string, active bool, width int, t *theme.Theme) string {
	bc := lipgloss.Color(t.Sidebar.CategoryFg)
	bStyle := lipgloss.NewStyle().Foreground(bc)

	innerW := width - 2
	if innerW < 4 {
		innerW = 4
	}

	var text string
	if active {
		text = " / " + query + "█"
	} else {
		text = " filter: " + query
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
