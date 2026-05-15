package theme

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

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
	Background string `yaml:"background"`
	Foreground string `yaml:"foreground"`
	SelectedBg string `yaml:"selected_bg"`
	SelectedFg string `yaml:"selected_fg"`
	CategoryFg string `yaml:"category_fg"`
}

// TableColors defines colors for the resource table.
type TableColors struct {
	HeaderBg       string `yaml:"header_bg"`
	HeaderFg       string `yaml:"header_fg"`
	RowFg          string `yaml:"row_fg"`
	SelectedRowBg  string `yaml:"selected_row_bg"`
	SelectedRowFg  string `yaml:"selected_row_fg"`
	AlternatingBg  string `yaml:"alternating_bg"`
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
	Background  string `yaml:"background"`
	Foreground  string `yaml:"foreground"`
	ClusterFg   string `yaml:"cluster_fg"`
	NamespaceFg string `yaml:"namespace_fg"`
	ContextFg   string `yaml:"context_fg"`
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
			Background: "",
			Foreground: "#cdd6f4",
			SelectedBg: "#45475a",
			SelectedFg: "#cdd6f4",
			CategoryFg: "#89b4fa",
		},
		Table: TableColors{
			HeaderBg:       "#313244",
			HeaderFg:       "#89b4fa",
			RowFg:          "#cdd6f4",
			SelectedRowBg:  "#45475a",
			SelectedRowFg:  "#cdd6f4",
			AlternatingBg:  "",
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
			Background:  "",
			Foreground:  "#cdd6f4",
			ClusterFg:   "#a6e3a1",
			NamespaceFg: "#f9e2af",
			ContextFg:   "#89b4fa",
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

// SidebarSelectedStyle returns the style for the selected sidebar item.
func (t *Theme) SidebarSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.Sidebar.SelectedBg)).
		Foreground(lipgloss.Color(t.Sidebar.SelectedFg)).
		Bold(true)
}

// SidebarCategoryStyle returns the style for sidebar category headers.
func (t *Theme) SidebarCategoryStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Sidebar.CategoryFg)).
		Bold(true)
}

// TableHeaderStyle returns the style for table column headers.
func (t *Theme) TableHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.Table.HeaderBg)).
		Foreground(lipgloss.Color(t.Table.HeaderFg)).
		Bold(true)
}

// TableRowStyle returns the style for a normal table row.
func (t *Theme) TableRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Table.RowFg))
}

// TableSelectedRowStyle returns the style for the selected table row.
func (t *Theme) TableSelectedRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.Table.SelectedRowBg)).
		Foreground(lipgloss.Color(t.Table.SelectedRowFg)).
		Bold(true)
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

// StatusBarClusterStyle returns the style for the cluster name in the status bar.
func (t *Theme) StatusBarClusterStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.StatusBar.ClusterFg)).
		Bold(true)
}

// StatusBarNamespaceStyle returns the style for the namespace in the status bar.
func (t *Theme) StatusBarNamespaceStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.StatusBar.NamespaceFg))
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
