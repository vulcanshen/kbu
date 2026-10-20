package tui

import (
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/terminu-go/tdptest"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// tdpApp is kbu as main makes it, but with no cluster to reach.
func tdpApp() *terminu.App {
	return New(Options{Config: config.DefaultConfig(), State: config.DefaultState()}).app
}

// tdp's checks of every app (terminu-go's tdptest): the first launch on many
// terminal sizes and icon widths, each panel's Space menu, the global
// operations, the App Log and the key reference, closed one layer an Esc,
// and a terminal under kbu's least size and back. Every frame is checked
// for tdp L4 and for a control character reaching the terminal.
func TestTDP(t *testing.T) {
	tdptest.Check(t, tdptest.App{Name: "kbu", New: tdpApp})
}

// Pods named with tdptest's tricky values, which have broken screens: their
// control characters are drawn by name, never sent to the terminal, on
// panel 2, its menu and its zoom.
func TestTDPTrickyNames(t *testing.T) {
	t.Setenv("KBU__CONFIG", t.TempDir())
	t.Setenv("KBU__STATE", t.TempDir())
	d := tdptest.Start(t, tdpApp(), 120, 40)
	items := make([]k8s.ResourceItem, len(tdptest.Tricky))
	for i, name := range tdptest.Tricky {
		items[i] = pod("tricky", name)
	}
	d.Send(watchDataMsg{kind: k8s.ResourcePods, items: items})
	d.Press("Tab", "Space", "Esc", "z", "z", "G", "g", "g")
	d.Resize(80, 24)
	d.Press("Space", "Esc")
}
