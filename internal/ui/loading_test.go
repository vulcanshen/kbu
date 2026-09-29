package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// pinClock fixes the loading icon's clock at d past the epoch.
func pinClock(t *testing.T, d time.Duration) {
	t.Helper()
	old := loadingNow
	loadingNow = func() time.Time { return time.Unix(0, int64(d)) }
	t.Cleanup(func() { loadingNow = old })
}

// tdp D3: the loading icon is nf-md-circle_slice_1..8 (U+F0A9E–U+F0AA5),
// 90ms a frame, picked by the clock — frames[(now / 90ms) % 8]. Code
// points written out.
func TestD3_LoadingIconFollowsTheClock(t *testing.T) {
	for _, tc := range []struct {
		at   time.Duration
		want rune
	}{
		{0, 0xf0a9e},
		{89 * time.Millisecond, 0xf0a9e},
		{90 * time.Millisecond, 0xf0a9f},
		{630 * time.Millisecond, 0xf0aa5},
		{720 * time.Millisecond, 0xf0a9e}, // a full turn
	} {
		pinClock(t, tc.at)
		if got := loadingIcon(); got != string(tc.want) {
			t.Errorf("at %v the icon is %U, want %U", tc.at, []rune(got)[0], tc.want)
		}
	}
}

// tdp F7, D3: the namespace picker shows the loading icon after its title
// while the list is on its way, a space once it is in; the top border is
// as wide either way, and no braille spinner is left.
func TestF7_NamespacePickerShowsLoadingInItsTitle(t *testing.T) {
	pinClock(t, 0)
	icon := string(rune(0xf0a9e))
	m := stackTestApp(t)
	p := m.namespacePicker
	p.SetSize(m.width, m.height)
	_ = p.OpenLoading()
	loading := ansi.Strip(strings.SplitN(p.renderFullPopup(), "\n", 2)[0])
	if !strings.Contains(loading, "Namespaces "+icon) {
		t.Errorf("a loading picker shows the icon after its title: %q", loading)
	}

	p.SetNamespaces([]string{"default"})
	loaded := ansi.Strip(strings.SplitN(p.renderFullPopup(), "\n", 2)[0])
	if strings.Contains(loaded, icon) || !strings.Contains(loaded, "Namespaces  ") {
		t.Errorf("the icon goes once the list is in, leaving a space: %q", loaded)
	}
	if lipgloss.Width(loading) != lipgloss.Width(loaded) {
		t.Errorf("the top border changed width with the icon: %d → %d", lipgloss.Width(loading), lipgloss.Width(loaded))
	}
	for _, r := range loading + loaded {
		if r >= 0x2800 && r <= 0x28ff {
			t.Errorf("a braille spinner frame %U is still drawn", r)
		}
	}
}

// tdp D3: the tick runs only while something loads — the app re-arms it
// while the namespace list is on its way and stops once it is in.
func TestD3_LoadingTickStopsWhenLoaded(t *testing.T) {
	m := stackTestApp(t)
	_ = m.namespacePicker.OpenLoading()
	if _, cmd := m.Update(loadingTickMsg{}); cmd == nil {
		t.Error("while loading, the tick must re-arm")
	}
	m.namespacePicker.SetNamespaces([]string{"default"})
	if _, cmd := m.Update(loadingTickMsg{}); cmd != nil {
		t.Error("once the list is in, the tick must stop")
	}
}
