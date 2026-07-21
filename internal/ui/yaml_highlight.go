package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/kbu/internal/theme"
)

// highlightYAMLLine returns the line with ANSI color codes applied to YAML
// tokens: keys, list dashes, comments, document separators. Designed for
// lines emitted by sigs.k8s.io/yaml (canonical YAML, no anchors/aliases).
//
// Leading whitespace is preserved unstyled so indentation stays intact.
// Returns the original line when no recognized token is present.
func highlightYAMLLine(line string, t *theme.Theme) string {
	if strings.TrimSpace(line) == "" {
		return line
	}
	keyStyle := t.DetailLabelStyle()
	valStyle := t.DetailValueStyle()
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))

	leadingLen := len(line) - len(strings.TrimLeft(line, " \t"))
	leading := line[:leadingLen]
	rest := line[leadingLen:]

	if strings.HasPrefix(rest, "#") {
		return leading + mutedStyle.Render(rest)
	}
	if rest == "---" {
		return leading + mutedStyle.Render(rest)
	}

	listPrefix := ""
	switch {
	case strings.HasPrefix(rest, "- "):
		listPrefix = mutedStyle.Render("-") + " "
		rest = rest[2:]
	case rest == "-":
		return leading + mutedStyle.Render("-")
	}

	keyEnd := -1
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c == ':' {
			if i == len(rest)-1 || rest[i+1] == ' ' {
				keyEnd = i
			}
			break
		}
		if !isYAMLKeyChar(c) {
			break
		}
	}
	if keyEnd > 0 {
		key := rest[:keyEnd]
		afterColon := rest[keyEnd+1:] // strip the colon itself
		return leading + listPrefix + keyStyle.Render(key) + mutedStyle.Render(":") + valStyle.Render(afterColon)
	}
	return leading + listPrefix + valStyle.Render(rest)
}

func isYAMLKeyChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '_' || c == '-' || c == '.' || c == '/':
		return true
	}
	return false
}
