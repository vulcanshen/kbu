package ui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/vulcanshen/km8/internal/theme"
)

func TestHighlightYAMLLine_PreservesPlainText(t *testing.T) {
	th := theme.DefaultTheme()
	tests := []string{
		"",
		"   ",
		"name: nginx",
		"  - image: nginx:1.21",
		"# a comment",
		"---",
		"  containers:",
		"  ports:",
		"    - containerPort: 80",
	}
	for _, line := range tests {
		got := highlightYAMLLine(line, th)
		plain := ansi.Strip(got)
		if plain != line {
			t.Errorf("highlight changed plain text:\n  input:  %q\n  output: %q (stripped: %q)", line, got, plain)
		}
	}
}

func TestHighlightYAMLLine_BlankLineUnchanged(t *testing.T) {
	th := theme.DefaultTheme()
	got := highlightYAMLLine("", th)
	if got != "" {
		t.Errorf("expected empty line unchanged, got %q", got)
	}
}
