package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// loadingFrames is the family loading icon (tdp D3): a circle filled one
// slice at a time, nf-md-circle_slice_1 to _8, one cell wide like the
// glyphs it stands beside. Written as code points (D6).
var loadingFrames = [8]string{
	string(rune(0xf0a9e)), string(rune(0xf0a9f)), string(rune(0xf0aa0)), string(rune(0xf0aa1)),
	string(rune(0xf0aa2)), string(rune(0xf0aa3)), string(rune(0xf0aa4)), string(rune(0xf0aa5)),
}

// loadingStep is how long each frame shows: a turn every 720ms (tdp D3).
const loadingStep = 90 * time.Millisecond

// loadingNow is the clock the icon reads; tests pin it.
var loadingNow = time.Now

// loadingIcon is the frame for now. The clock picks it, not a counter, so
// a late or doubled tick cannot skip, repeat or speed up a frame (tdp D3).
func loadingIcon() string {
	return loadingFrames[(loadingNow().UnixNano()/int64(loadingStep))%int64(len(loadingFrames))]
}

// loadingTickMsg redraws the loading icon.
type loadingTickMsg struct{}

func loadingTick() tea.Cmd {
	return tea.Tick(loadingStep, func(time.Time) tea.Msg { return loadingTickMsg{} })
}
