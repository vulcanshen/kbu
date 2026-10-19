package tui

import (
	_ "embed"

	terminu "github.com/vulcanshen/terminu-go"
)

// icon is kbu's mark, drawn by the splash (tdp S5): a copy of
// docs/icon.svg, which go:embed cannot reach. A test keeps the two the same.
//
//go:embed icon.svg
var icon []byte

// intro is the splash's line under the name and the version.
const intro = "A single-pane Kubernetes workspace"

// newSplash is the family's easter egg, V on a panel (Rules S).
func newSplash(version string) (terminu.Splash, error) {
	if version == "" {
		version = "dev"
	}
	return terminu.NewSplash(icon, "kbu", version, intro)
}
