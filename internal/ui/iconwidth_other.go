//go:build !darwin && !linux

package ui

// DetectIconWidth off unix only reads KBU__ICON_WIDTH: the CPR probe needs a
// raw-mode read with a deadline (unix.Poll), so without the override an icon
// stays one cell (tdp D6).
func DetectIconWidth() {
	if n, ok := iconWidthOverride(); ok {
		iconCells = n
	}
}
