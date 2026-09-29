//go:build !darwin && !linux

package ui

// DetectIconWidth off unix only reads KBU__ICON_WIDTH, then TERMINU__ICON_WIDTH
// (iconWidthOverride): the CPR probe needs a raw-mode read with a deadline
// (unix.Poll), so without either an icon stays one cell (tdp D6).
func DetectIconWidth() {
	if n, ok := iconWidthOverride(); ok {
		iconCells = n
	}
}
