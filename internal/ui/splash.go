package ui

import (
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/km8/internal/version"
)

type splashTickMsg struct{}

// SplashModel renders the km8 logo as a hidden easter egg.
type SplashModel struct {
	active        bool
	pixelOrder    []int // shuffled indices of colored pixels (row*18 + col)
	revealedCount int
}

func NewSplashModel() SplashModel {
	return SplashModel{}
}

func (m SplashModel) IsActive() bool { return m.active }

func (m *SplashModel) Show() tea.Cmd {
	m.active = true
	m.revealedCount = 0

	// Collect all colored pixel indices.
	var colored []int
	for r := 0; r < len(logoPixels); r++ {
		for c := 0; c < len(logoPixels[r]); c++ {
			if pixelColor(logoPixels[r][c]) != "" {
				colored = append(colored, r*len(logoPixels[r])+c)
			}
		}
	}
	rand.Shuffle(len(colored), func(i, j int) { colored[i], colored[j] = colored[j], colored[i] })
	m.pixelOrder = colored

	return tea.Tick(30*time.Millisecond, func(time.Time) tea.Msg {
		return splashTickMsg{}
	})
}

// km8 logo (reference: .references/logo.txt): 18x18 grid.
// M=navy (background), K/8=gold, space=transparent.
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

func (m SplashModel) Render(width, height int) string {
	if !m.active {
		return ""
	}

	// Build set of revealed pixel indices for O(1) lookup.
	revealed := make(map[int]bool, m.revealedCount)
	for _, idx := range m.pixelOrder[:m.revealedCount] {
		revealed[idx] = true
	}

	cols := len(logoPixels[0])
	var logoLines []string
	for r := 0; r < len(logoPixels); r++ {
		var line strings.Builder
		for c := 0; c < cols; c++ {
			color := pixelColor(logoPixels[r][c])
			if color != "" && revealed[r*cols+c] {
				line.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("██"))
			} else {
				line.WriteString("  ")
			}
		}
		logoLines = append(logoLines, line.String())
	}
	logo := strings.Join(logoLines, "\n")

	// Caption space is always reserved so the logo doesn't shift when text appears.
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	logoW := cols * 2
	done := m.revealedCount >= len(m.pixelOrder)
	titleText, taglineText, hintText := " ", " ", " "
	if done {
		titleText = "K M 8"
		taglineText = version.Display()
		hintText = dimStyle.Render("press q or Esc to close")
	}
	caption := "\n\n" +
		lipgloss.PlaceHorizontal(logoW, lipgloss.Center, titleText) +
		"\n" +
		lipgloss.PlaceHorizontal(logoW, lipgloss.Center, taglineText) +
		"\n\n" +
		lipgloss.PlaceHorizontal(logoW, lipgloss.Center, hintText)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, logo+caption)
}

// Update handles key events and animation ticks when splash is active.
func (m SplashModel) Update(msg tea.Msg) (SplashModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.active = false
			m.revealedCount = 0
			m.pixelOrder = nil
		}
	case splashTickMsg:
		if m.revealedCount < len(m.pixelOrder) {
			m.revealedCount += 15
			if m.revealedCount > len(m.pixelOrder) {
				m.revealedCount = len(m.pixelOrder)
			}
			return m, tea.Tick(30*time.Millisecond, func(time.Time) tea.Msg {
				return splashTickMsg{}
			})
		}
	}
	return m, nil
}
