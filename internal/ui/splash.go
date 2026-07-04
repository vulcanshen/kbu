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
type splashVersionMsg struct{}
type splashHintMsg struct{}

// SplashModel renders the km8 logo as a hidden easter egg.
type SplashModel struct {
	active         bool
	pixelOrder     []int // colored pixel indices (row*cols + col): row-major M background, then shuffled K/8 foreground
	revealedCount  int
	bgCount        int // step-size boundary — first bgCount indices are M pixels
	boundaryPaused bool
	versionVisible bool
	hintVisible    bool
}

func NewSplashModel() SplashModel {
	return SplashModel{}
}

func (m SplashModel) IsActive() bool { return m.active }

func (m *SplashModel) Show() tea.Cmd {
	m.active = true
	m.revealedCount = 0
	m.boundaryPaused = false
	m.versionVisible = false
	m.hintVisible = false

	// Five-phase reveal:
	// (1) M background — row-major top-to-bottom sweep.
	// (2) beat — brief hold at the M→K/8 boundary.
	// (3) K/8 foreground shuffled — identity emerging from noise (accelerated).
	// (4) hold — version caption appears.
	// (5) hold — esc hint appears.
	rows, cols := len(logoPixels), len(logoPixels[0])
	var bg, fg []int
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			switch logoPixels[r][c] {
			case 'M':
				bg = append(bg, r*cols+c)
			case 'K', '8':
				fg = append(fg, r*cols+c)
			}
		}
	}
	// bg stays in row-major order for top-to-bottom sweep; fg shuffled.
	rand.Shuffle(len(fg), func(i, j int) { fg[i], fg[j] = fg[j], fg[i] })
	m.pixelOrder = append(bg, fg...)
	m.bgCount = len(bg)

	return tea.Tick(10*time.Millisecond, func(time.Time) tea.Msg {
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
	"MMKK KKKMM88  88MM",
	"MMKKKKKKMM88  88MM",
	"MMKKKKK MM88  88MM",
	"MMKK    MM888888MM",
	"MMKK    MM888888MM",
	"MMKKKKK MM88  88MM",
	"MMKKKKKKMM88  88MM",
	"MMKK KKKMM88  88MM",
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

func pixelGlyph(p byte) string {
	switch p {
	case 'K', '8':
		return "" // nf-fa-paw
	case 'M':
		return "\U000f011b" // nf-md-cat
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
				glyph := pixelGlyph(logoPixels[r][c])
				line.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(glyph + " "))
			} else {
				line.WriteString("  ")
			}
		}
		logoLines = append(logoLines, line.String())
	}
	logo := strings.Join(logoLines, "\n")

	// Caption space is always reserved so the logo doesn't shift when text appears.
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	blueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa"))
	logoW := cols * 2
	versionText, hintText := " ", " "
	if m.versionVisible {
		versionText = blueStyle.Render(version.Display())
	}
	if m.hintVisible {
		hintText = dimStyle.Render("Press Esc to close")
	}
	caption := "\n\n" +
		lipgloss.PlaceHorizontal(logoW, lipgloss.Center, versionText) +
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
		case "esc", "enter", " ":
			m.active = false
			m.revealedCount = 0
			m.pixelOrder = nil
			m.bgCount = 0
			m.boundaryPaused = false
			m.versionVisible = false
			m.hintVisible = false
		}
	case splashTickMsg:
		// Beat at the M→K/8 boundary — brief hold so the M reveal registers
		// before the K/8 shuffle starts.
		if m.bgCount > 0 && m.revealedCount == m.bgCount && !m.boundaryPaused {
			m.boundaryPaused = true
			return m, tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
				return splashTickMsg{}
			})
		}
		if m.revealedCount < len(m.pixelOrder) {
			// Stage 1 (M background): 4 pixels/tick @ 10ms — top-to-bottom sweep.
			// Stage 2 (K/8 foreground): 2 pixels/tick @ 10ms — accelerated shuffle.
			step, delay := 4, 10*time.Millisecond
			if m.revealedCount >= m.bgCount {
				step, delay = 2, 10*time.Millisecond
			}
			newCount := m.revealedCount + step
			// Clamp to the boundary so the M→K/8 beat fires cleanly.
			if m.revealedCount < m.bgCount && newCount > m.bgCount {
				newCount = m.bgCount
			}
			if newCount > len(m.pixelOrder) {
				newCount = len(m.pixelOrder)
			}
			m.revealedCount = newCount
			return m, tea.Tick(delay, func(time.Time) tea.Msg {
				return splashTickMsg{}
			})
		}
		// K/8 done — schedule the version caption reveal after a brief hold.
		if !m.versionVisible {
			return m, tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
				return splashVersionMsg{}
			})
		}
	case splashVersionMsg:
		m.versionVisible = true
		return m, tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
			return splashHintMsg{}
		})
	case splashHintMsg:
		m.hintVisible = true
	}
	return m, nil
}
