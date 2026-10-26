// Package theme holds the colours of a resource's status cells, the one part
// of the screen whose colours are kbu's to choose (the app's content, tdp P4).
// Everything else is drawn in the terminu family's colours (tdp color).
package theme

// Theme is kbu's colours.
type Theme struct {
	Status StatusColors
}

// StatusColors are the colours of a status cell: what needs a look is
// coloured, what is healthy stays plain.
type StatusColors struct {
	Running string
	Pending string
	Error   string
	Unknown string
}

// DefaultTheme is catppuccin-mocha's colours for a status.
func DefaultTheme() *Theme {
	return &Theme{Status: StatusColors{
		Running: "#a6e3a1",
		Pending: "#f9e2af",
		Error:   "#f38ba8",
		Unknown: "#7f849c",
	}}
}
