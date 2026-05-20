package ui

import (
	"os"
	"strings"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

// ClipboardCopiedMsg is dispatched after a clipboard write so the app can
// show a transient success notice.
type ClipboardCopiedMsg struct {
	Lines int
}

// ClipboardCopyFailedMsg is dispatched when the clipboard write produced no
// output (empty content). The app uses it to surface a no-op notice rather
// than silently doing nothing.
type ClipboardCopyFailedMsg struct {
	Reason string
}

// copyToClipboardCmd writes text to the system clipboard via OSC 52 and
// returns a Cmd that fires ClipboardCopiedMsg on success. OSC 52 works
// through SSH and tmux without requiring xclip/pbcopy/etc.
//
// The escape sequence is written to os.Stderr because bubbletea owns stdout
// in alt-screen mode. The terminal interprets OSC 52 from any fd.
func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		if text == "" {
			return ClipboardCopyFailedMsg{Reason: "no content to copy"}
		}
		seq := osc52.New(text)
		if os.Getenv("TMUX") != "" {
			seq = seq.Tmux()
		}
		if _, err := seq.WriteTo(os.Stderr); err != nil {
			return ClipboardCopyFailedMsg{Reason: err.Error()}
		}
		return ClipboardCopiedMsg{Lines: countLines(text)}
	}
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
