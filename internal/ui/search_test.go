package ui

import (
	"strings"
	"testing"

	"github.com/vulcanshen/km8/internal/theme"
)

func newTestTheme() *theme.Theme {
	t := theme.DefaultTheme()
	return t
}

// ── renderSearchBox ────────────────────────────────────────────────────────

func TestRenderSearchBox_HasThreeLines(t *testing.T) {
	th := newTestTheme()
	out := renderSearchBox("", false, 30, th)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (top/mid/bot), got %d", len(lines))
	}
}

func TestRenderSearchBox_ActiveShowsCursor(t *testing.T) {
	th := newTestTheme()
	out := renderSearchBox("foo", true, 30, th)
	if !strings.Contains(out, "█") {
		t.Error("active search box must show cursor block █")
	}
}

func TestRenderSearchBox_InactiveNoCursor(t *testing.T) {
	th := newTestTheme()
	out := renderSearchBox("foo", false, 30, th)
	if strings.Contains(out, "█") {
		t.Error("inactive search box must not show cursor block")
	}
}

func TestRenderSearchBox_ContainsQuery(t *testing.T) {
	th := newTestTheme()
	out := renderSearchBox("nginx", false, 40, th)
	if !strings.Contains(out, "nginx") {
		t.Errorf("search box must display the query string, got %q", out)
	}
}

func TestRenderSearchBox_LongQueryTruncated(t *testing.T) {
	th := newTestTheme()
	long := strings.Repeat("x", 200)
	out := renderSearchBox(long, false, 20, th)
	lines := strings.Split(out, "\n")
	// Middle line must not exceed inner width (width-2 = 18 chars of visible content).
	// Since lipgloss adds ANSI escapes, we can't measure raw len — just check no panic
	// and that output contains ellipsis.
	if !strings.Contains(lines[1], "…") {
		t.Error("long query must be truncated with ellipsis")
	}
}

func TestRenderSearchBox_EmptyQuery(t *testing.T) {
	th := newTestTheme()
	out := renderSearchBox("", false, 30, th)
	// Should render without panic and produce 3 lines.
	if strings.Count(out, "\n") != 2 {
		t.Errorf("expected 2 newlines (3 lines), got %d", strings.Count(out, "\n"))
	}
}
