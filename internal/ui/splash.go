package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SplashModel renders the km8 logo as a hidden easter egg.
type SplashModel struct {
	active bool
}

func NewSplashModel() SplashModel {
	return SplashModel{}
}

func (m SplashModel) IsActive() bool { return m.active }

func (m *SplashModel) Show() { m.active = true }
func (m *SplashModel) Hide() { m.active = false }

func (m SplashModel) Render(width, height int) string {
	if !m.active {
		return ""
	}
	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		renderLogo())
}

// km8 logo (reference: .references/logo.txt): 18x18 grid.
// M=navy (background), K/8=gold.
var logoPixels = [18]string{
	"MMMMMMMMMMMMMMMMMM",
	"MMMMMMMMMMMMMMMMMM",
	"MMKK  KKMM888888MM",
	"MMKK  KKMM888888MM",
	"MMKK  KKMM88  88MM",
	"MMKK  KKMM88  88MM",
	"MMKK  KKMM88  88MM",
	"MMKKKKKKMM88  88MM",
	"MMKKKKKKMM88  88MM",
	"MMKK    MM888888MM",
	"MMKK    MM888888MM",
	"MMKKKKKKMM88  88MM",
	"MMKKKKKKMM88  88MM",
	"MMKK  KKMM88  88MM",
	"MMKK  KKMM88  88MM",
	"MMKK  KKMM88  88MM",
	"MMKK  KKMM888888MM",
	"MMKK  KKMM888888MM",
}

const (
	logoNavy = "#1D4685"
	logoGold = "#F0AE49"
)

func pixelColor(p byte) string {
	switch p {
	case 'M':
		return logoNavy
	case 'K', '8':
		return logoGold
	}
	return ""
}

// renderLogo renders each pixel as a "██" block (2 chars wide × 1 char tall),
// which is visually square in standard terminal cells.
func renderLogo() string {
	var lines []string
	for r := 0; r < len(logoPixels); r++ {
		var line strings.Builder
		for c := 0; c < len(logoPixels[r]); c++ {
			color := pixelColor(logoPixels[r][c])
			if color == "" {
				line.WriteString("  ")
			} else {
				line.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("██"))
			}
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

// Update handles key events when splash is active. Esc or q closes it.
func (m SplashModel) Update(msg tea.Msg) (SplashModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "q":
			m.active = false
		}
	}
	return m, nil
}
