package theme

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// Popup-layer border color scale — lavender → sapphire gradient.
// Each layer of popup nesting picks the next stop along the scale,
// so the visual stack reads "deeper popups are MORE distinct from
// the panel background". Scale endpoints are catppuccin Mocha
// lavender + sapphire (real palette anchors, not custom blends);
// intermediate stops are linear-RGB lerp of those two. Future
// expansion: subdivide the existing intervals (12.5 / 37.5 / ...)
// or raise the ceiling to sky.
//
// Lavender itself is the SCALE ANCHOR — popups never use it.
// It stays reserved for in-panel user-footprint (sidebar Pinned
// section, settings ON toggle, compare anchor row, unfocused-
// selected chip, KM8erm statusbar marker).
const (
	Lavender     = "#b4befe" // scale anchor — popups DON'T use this
	Lavenphire25 = "#A4C0FA" // L1 — first-tier popup
	Lavenphire50 = "#94C3F5" // L2 — popup over popup (e.g. comparemenu over comparepopup)
	Lavenphire75 = "#84C5F0" // L3 — popup over popup over popup (unused today)
	Sapphire     = "#74c7ec" // L4 ceiling — catppuccin Mocha sapphire
)

// PopupLayerColor maps a popup's 1-based nesting depth to its
// border + animator stroke color. Layer 0/1 → Lavenphire25, layer
// 2 → Lavenphire50, layer 3 → Lavenphire75, layer 4+ → Sapphire.
// Going past 4 clamps at Sapphire — when the app eventually nests
// deeper, extend the scale by raising the ceiling and subdividing.
func PopupLayerColor(layer int) lipgloss.Color {
	switch {
	case layer <= 1:
		return lipgloss.Color(Lavenphire25)
	case layer == 2:
		return lipgloss.Color(Lavenphire50)
	case layer == 3:
		return lipgloss.Color(Lavenphire75)
	default:
		return lipgloss.Color(Sapphire)
	}
}

// Theme defines colors for all UI elements in km8.
type Theme struct {
	Sidebar    SidebarColors    `yaml:"sidebar"`
	Table      TableColors      `yaml:"table"`
	Detail     DetailColors     `yaml:"detail"`
	StatusBar  StatusBarColors  `yaml:"status_bar"`
	StatusLine StatusLineColors `yaml:"status_line"`
	Status     StatusColors     `yaml:"status"`
}

// SidebarColors defines colors for the sidebar panel.
type SidebarColors struct {
	Background          string `yaml:"background"`
	Foreground          string `yaml:"foreground"`
	SelectedBg          string `yaml:"selected_bg"`
	SelectedFg          string `yaml:"selected_fg"`
	UnfocusedSelectedBg string `yaml:"unfocused_selected_bg"`
	UnfocusedSelectedFg string `yaml:"unfocused_selected_fg"`
	CategoryFg          string `yaml:"category_fg"`
}

// TableColors defines colors for the resource table.
type TableColors struct {
	HeaderBg               string `yaml:"header_bg"`
	HeaderFg               string `yaml:"header_fg"`
	RowFg                  string `yaml:"row_fg"`
	SelectedRowBg          string `yaml:"selected_row_bg"`
	SelectedRowFg          string `yaml:"selected_row_fg"`
	UnfocusedSelectedRowBg string `yaml:"unfocused_selected_row_bg"`
	UnfocusedSelectedRowFg string `yaml:"unfocused_selected_row_fg"`
	AlternatingBg          string `yaml:"alternating_bg"`
}

// DetailColors defines colors for the detail panel.
type DetailColors struct {
	BorderColor   string `yaml:"border_color"`
	LabelFg       string `yaml:"label_fg"`
	ValueFg       string `yaml:"value_fg"`
	TabActiveBg   string `yaml:"tab_active_bg"`
	TabActiveFg   string `yaml:"tab_active_fg"`
	TabInactiveFg string `yaml:"tab_inactive_fg"`
}

// StatusBarColors defines colors for the top status bar.
type StatusBarColors struct {
	Background string `yaml:"background"`
	Foreground string `yaml:"foreground"`
	ContextFg  string `yaml:"context_fg"`
}

// StatusLineColors defines colors for the bottom hints bar.
type StatusLineColors struct {
	Background string `yaml:"background"`
	Foreground string `yaml:"foreground"`
}

// StatusColors defines colors for resource status indicators.
type StatusColors struct {
	Running string `yaml:"running"`
	Pending string `yaml:"pending"`
	Error   string `yaml:"error"`
	Unknown string `yaml:"unknown"`
}

// DefaultTheme returns a sensible dark theme with Catppuccin-inspired colors.
func DefaultTheme() *Theme {
	return &Theme{
		Sidebar: SidebarColors{
			Background:          "",
			Foreground:          "#cdd6f4",
			SelectedBg:          "#bac2de", // Catppuccin Mocha subtext1 — muted blue-grey, reads "light highlight" without "white block"
			SelectedFg:          "#1e1e2e", // Catppuccin Mocha base — high contrast, palette-native
			UnfocusedSelectedBg: "#b4befe", // Catppuccin Mocha lavender — same accent as the sidebar Pinned section and the statusbar [C]ontext/[N]amespace values, so the "user-selected / user-relevant" marker reads the same across the app
			UnfocusedSelectedFg: "#1e1e2e", // Catppuccin Mocha base — high contrast against the lavender chip
			CategoryFg:          "#89b4fa",
		},
		Table: TableColors{
			HeaderBg:               "", // no bg by default — the bold colored fg carries the header; bg is opt-in via theme.yaml
			HeaderFg:               "#89b4fa",
			RowFg:                  "#cdd6f4",
			SelectedRowBg:          "#bac2de", // Catppuccin Mocha subtext1
			SelectedRowFg:          "#1e1e2e", // Catppuccin Mocha base
			UnfocusedSelectedRowBg: "#b4befe", // Catppuccin Mocha lavender — matches the sidebar unfocused-selected chip so "user-selected" reads identically across the two panels
			UnfocusedSelectedRowFg: "#1e1e2e", // Catppuccin Mocha base — high contrast against the lavender chip
			AlternatingBg:          "",
		},
		Detail: DetailColors{
			BorderColor:   "#585b70",
			LabelFg:       "#89b4fa",
			ValueFg:       "#cdd6f4",
			TabActiveBg:   "#45475a",
			TabActiveFg:   "#cdd6f4",
			TabInactiveFg: "#6c7086",
		},
		StatusBar: StatusBarColors{
			Background: "",
			Foreground: "#cdd6f4",
			ContextFg:  "#89b4fa",
		},
		StatusLine: StatusLineColors{
			Background: "",
			Foreground: "#89b4fa",
		},
		Status: StatusColors{
			Running: "#a6e3a1",
			Pending: "#f9e2af",
			Error:   "#f38ba8",
			Unknown: "#6c7086",
		},
	}
}

// LoadTheme reads a theme.yaml from the given path and merges it with the
// default theme. Fields not present in the YAML file retain their default
// values. If path is empty or the file does not exist, the default theme is
// returned without error.
func LoadTheme(path string) (*Theme, error) {
	t := DefaultTheme()

	if path == "" {
		return t, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return t, nil
		}
		return nil, fmt.Errorf("reading theme file: %w", err)
	}

	if err := yaml.Unmarshal(data, t); err != nil {
		return nil, fmt.Errorf("parsing theme file: %w", err)
	}

	return t, nil
}

// --- Style helper methods ---
// Each method returns a lipgloss.Style configured with the theme's colors.

// SidebarStyle returns the base style for the sidebar panel.
func (t *Theme) SidebarStyle() lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Sidebar.Foreground))
	if t.Sidebar.Background != "" {
		s = s.Background(lipgloss.Color(t.Sidebar.Background))
	}
	return s
}

// SidebarSelectedStyle returns the style for the selected sidebar item
// while the sidebar panel has focus — bg highlight + bold, the classic
// "you're driving this row" cursor look.
func (t *Theme) SidebarSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.Sidebar.SelectedBg)).
		Foreground(lipgloss.Color(t.Sidebar.SelectedFg)).
		Bold(true)
}

// SidebarUnfocusedSelectedStyle returns the style for the currently-active
// resource when the sidebar isn't focused — softer bg than the focused
// cursor plus bold, so the panel still reads as having a "remembered"
// selection without competing visually with the focused panel.
func (t *Theme) SidebarUnfocusedSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.Sidebar.UnfocusedSelectedBg)).
		Foreground(lipgloss.Color(t.Sidebar.UnfocusedSelectedFg)).
		Bold(true)
}

// SidebarCategoryStyle returns the style for sidebar category headers.
func (t *Theme) SidebarCategoryStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Sidebar.CategoryFg)).
		Bold(true)
}

// SidebarDimRowStyle returns the dim style applied to non-cursor sidebar
// rows while the sidebar is unfocused. Catppuccin overlay0 (#6c7086) —
// dim enough to recede so the cursor row stands out as the single
// "remembered position" marker, light enough to stay legible if the
// user glances over.
func (t *Theme) SidebarDimRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
}

// TableHeaderStyle returns the style for table column headers. Bg is
// opt-in (empty = no bg, header sits on the panel canvas) — same
// pattern as TableAlternatingRowStyle. Default theme leaves bg empty
// so the bold colored fg alone signals "header"; a theme.yaml override
// can set header_bg to bring the bar back.
func (t *Theme) TableHeaderStyle() lipgloss.Style {
	s := lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Table.HeaderFg)).
		Bold(true)
	if t.Table.HeaderBg != "" {
		s = s.Background(lipgloss.Color(t.Table.HeaderBg))
	}
	return s
}

// TableRowStyle returns the style for a normal table row.
func (t *Theme) TableRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Table.RowFg))
}

// TableSelectedRowStyle returns the style for the table cursor row while
// the table panel has focus — bg highlight + bold.
func (t *Theme) TableSelectedRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.Table.SelectedRowBg)).
		Foreground(lipgloss.Color(t.Table.SelectedRowFg)).
		Bold(true)
}

// TableUnfocusedSelectedRowStyle returns the style for the table cursor
// row when the table isn't focused — softer bg than focused + bold,
// matching SidebarUnfocusedSelectedStyle.
func (t *Theme) TableUnfocusedSelectedRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.Table.UnfocusedSelectedRowBg)).
		Foreground(lipgloss.Color(t.Table.UnfocusedSelectedRowFg)).
		Bold(true)
}

// TableDimRowStyle returns the dim style applied to non-cursor non-locked
// table rows and the column header while the table is unfocused. Same
// overlay0 grey as SidebarDimRowStyle so the two panels' unfocused
// treatments read consistently.
func (t *Theme) TableDimRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
}

// TableAlternatingRowStyle returns the style for alternating table rows.
func (t *Theme) TableAlternatingRowStyle() lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Table.RowFg))
	if t.Table.AlternatingBg != "" {
		s = s.Background(lipgloss.Color(t.Table.AlternatingBg))
	}
	return s
}

// DetailBorderStyle returns the style for the detail panel border.
func (t *Theme) DetailBorderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderForeground(lipgloss.Color(t.Detail.BorderColor))
}

// DetailLabelStyle returns the style for labels in the detail panel.
func (t *Theme) DetailLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Detail.LabelFg)).
		Bold(true)
}

// DetailValueStyle returns the style for values in the detail panel.
func (t *Theme) DetailValueStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Detail.ValueFg))
}

// DetailTabActiveStyle returns the style for the active tab.
func (t *Theme) DetailTabActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.Detail.TabActiveBg)).
		Foreground(lipgloss.Color(t.Detail.TabActiveFg)).
		Bold(true).
		Padding(0, 1)
}

// DetailTabInactiveStyle returns the style for inactive tabs.
func (t *Theme) DetailTabInactiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Detail.TabInactiveFg)).
		Padding(0, 1)
}

// StatusBarStyle returns the base style for the status bar.
func (t *Theme) StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.StatusBar.Background)).
		Foreground(lipgloss.Color(t.StatusBar.Foreground))
}

// StatusBarContextStyle returns the style for the context in the status bar.
func (t *Theme) StatusBarContextStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.StatusBar.ContextFg))
}

// StatusLineStyle returns the style for the bottom hints bar.
func (t *Theme) StatusLineStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.StatusLine.Background)).
		Foreground(lipgloss.Color(t.StatusLine.Foreground))
}

// StatusRunningStyle returns the style for running/ready status.
func (t *Theme) StatusRunningStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Status.Running))
}

// StatusPendingStyle returns the style for pending/warning status.
func (t *Theme) StatusPendingStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Status.Pending))
}

// StatusErrorStyle returns the style for error/failed status.
func (t *Theme) StatusErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Status.Error))
}

// StatusUnknownStyle returns the style for unknown status.
func (t *Theme) StatusUnknownStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Status.Unknown))
}
