package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultTheme(t *testing.T) {
	th := DefaultTheme()

	if th == nil {
		t.Fatal("DefaultTheme() returned nil")
	}

	// Sidebar colors (Background is intentionally empty for terminal transparency)
	assertNonEmpty(t, "Sidebar.Foreground", th.Sidebar.Foreground)
	assertNonEmpty(t, "Sidebar.SelectedBg", th.Sidebar.SelectedBg)
	assertNonEmpty(t, "Sidebar.SelectedFg", th.Sidebar.SelectedFg)
	assertNonEmpty(t, "Sidebar.CategoryFg", th.Sidebar.CategoryFg)

	// Table colors
	assertNonEmpty(t, "Table.HeaderBg", th.Table.HeaderBg)
	assertNonEmpty(t, "Table.HeaderFg", th.Table.HeaderFg)
	assertNonEmpty(t, "Table.RowFg", th.Table.RowFg)
	assertNonEmpty(t, "Table.SelectedRowBg", th.Table.SelectedRowBg)
	assertNonEmpty(t, "Table.SelectedRowFg", th.Table.SelectedRowFg)
	// AlternatingBg is intentionally empty for terminal transparency

	// Detail colors
	assertNonEmpty(t, "Detail.BorderColor", th.Detail.BorderColor)
	assertNonEmpty(t, "Detail.LabelFg", th.Detail.LabelFg)
	assertNonEmpty(t, "Detail.ValueFg", th.Detail.ValueFg)
	assertNonEmpty(t, "Detail.TabActiveBg", th.Detail.TabActiveBg)
	assertNonEmpty(t, "Detail.TabActiveFg", th.Detail.TabActiveFg)
	assertNonEmpty(t, "Detail.TabInactiveFg", th.Detail.TabInactiveFg)

	// StatusBar colors (Background intentionally empty for transparency)
	assertNonEmpty(t, "StatusBar.Foreground", th.StatusBar.Foreground)
	assertNonEmpty(t, "StatusBar.ClusterFg", th.StatusBar.ClusterFg)
	assertNonEmpty(t, "StatusBar.NamespaceFg", th.StatusBar.NamespaceFg)
	assertNonEmpty(t, "StatusBar.ContextFg", th.StatusBar.ContextFg)

	// StatusLine colors (Background intentionally empty for transparency)
	assertNonEmpty(t, "StatusLine.Foreground", th.StatusLine.Foreground)

	// Status colors
	assertNonEmpty(t, "Status.Running", th.Status.Running)
	assertNonEmpty(t, "Status.Pending", th.Status.Pending)
	assertNonEmpty(t, "Status.Error", th.Status.Error)
	assertNonEmpty(t, "Status.Unknown", th.Status.Unknown)
}

func TestDefaultTheme_StyleMethods(t *testing.T) {
	th := DefaultTheme()

	// Verify that all style methods return without panicking.
	// Each method returns a lipgloss.Style which is a value type.
	_ = th.SidebarStyle()
	_ = th.SidebarSelectedStyle()
	_ = th.SidebarCategoryStyle()
	_ = th.TableHeaderStyle()
	_ = th.TableRowStyle()
	_ = th.TableSelectedRowStyle()
	_ = th.TableAlternatingRowStyle()
	_ = th.DetailBorderStyle()
	_ = th.DetailLabelStyle()
	_ = th.DetailValueStyle()
	_ = th.DetailTabActiveStyle()
	_ = th.DetailTabInactiveStyle()
	_ = th.StatusBarStyle()
	_ = th.StatusBarClusterStyle()
	_ = th.StatusBarNamespaceStyle()
	_ = th.StatusBarContextStyle()
	_ = th.StatusLineStyle()
	_ = th.StatusRunningStyle()
	_ = th.StatusPendingStyle()
	_ = th.StatusErrorStyle()
	_ = th.StatusUnknownStyle()
}

func TestLoadTheme_EmptyPath(t *testing.T) {
	th, err := LoadTheme("")
	if err != nil {
		t.Fatalf("LoadTheme(\"\") returned error: %v", err)
	}
	if th == nil {
		t.Fatal("LoadTheme(\"\") returned nil theme")
	}

	// Should be identical to default
	def := DefaultTheme()
	if th.Sidebar.Background != def.Sidebar.Background {
		t.Errorf("expected default Sidebar.Background %q, got %q",
			def.Sidebar.Background, th.Sidebar.Background)
	}
}

func TestLoadTheme_NonExistentFile(t *testing.T) {
	th, err := LoadTheme("/nonexistent/path/theme.yaml")
	if err != nil {
		t.Fatalf("LoadTheme with nonexistent file returned error: %v", err)
	}
	if th == nil {
		t.Fatal("LoadTheme with nonexistent file returned nil theme")
	}

	// Should be identical to default
	def := DefaultTheme()
	if th.Status.Running != def.Status.Running {
		t.Errorf("expected default Status.Running %q, got %q",
			def.Status.Running, th.Status.Running)
	}
}

func TestLoadTheme_ValidYAML(t *testing.T) {
	content := []byte(`
sidebar:
  background: "#000000"
  foreground: "#ffffff"
status:
  running: "#00ff00"
  error: "#ff0000"
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test theme file: %v", err)
	}

	th, err := LoadTheme(path)
	if err != nil {
		t.Fatalf("LoadTheme returned error: %v", err)
	}

	// Overridden values
	if th.Sidebar.Background != "#000000" {
		t.Errorf("expected Sidebar.Background #000000, got %q", th.Sidebar.Background)
	}
	if th.Sidebar.Foreground != "#ffffff" {
		t.Errorf("expected Sidebar.Foreground #ffffff, got %q", th.Sidebar.Foreground)
	}
	if th.Status.Running != "#00ff00" {
		t.Errorf("expected Status.Running #00ff00, got %q", th.Status.Running)
	}
	if th.Status.Error != "#ff0000" {
		t.Errorf("expected Status.Error #ff0000, got %q", th.Status.Error)
	}

	// Non-overridden values should retain defaults
	def := DefaultTheme()
	if th.Sidebar.SelectedBg != def.Sidebar.SelectedBg {
		t.Errorf("expected default Sidebar.SelectedBg %q, got %q",
			def.Sidebar.SelectedBg, th.Sidebar.SelectedBg)
	}
	if th.Table.HeaderBg != def.Table.HeaderBg {
		t.Errorf("expected default Table.HeaderBg %q, got %q",
			def.Table.HeaderBg, th.Table.HeaderBg)
	}
	if th.Detail.BorderColor != def.Detail.BorderColor {
		t.Errorf("expected default Detail.BorderColor %q, got %q",
			def.Detail.BorderColor, th.Detail.BorderColor)
	}
	if th.StatusBar.Background != def.StatusBar.Background {
		t.Errorf("expected default StatusBar.Background %q, got %q",
			def.StatusBar.Background, th.StatusBar.Background)
	}
	if th.StatusLine.Background != def.StatusLine.Background {
		t.Errorf("expected default StatusLine.Background %q, got %q",
			def.StatusLine.Background, th.StatusLine.Background)
	}
	if th.Status.Pending != def.Status.Pending {
		t.Errorf("expected default Status.Pending %q, got %q",
			def.Status.Pending, th.Status.Pending)
	}
	if th.Status.Unknown != def.Status.Unknown {
		t.Errorf("expected default Status.Unknown %q, got %q",
			def.Status.Unknown, th.Status.Unknown)
	}
}

func TestLoadTheme_InvalidYAML(t *testing.T) {
	content := []byte(`{{{invalid yaml`)
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test theme file: %v", err)
	}

	_, err := LoadTheme(path)
	if err == nil {
		t.Fatal("LoadTheme with invalid YAML should return error")
	}
}

func TestLoadTheme_PartialOverride(t *testing.T) {
	// Only override one nested field in one section
	content := []byte(`
table:
  header_fg: "#ff00ff"
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test theme file: %v", err)
	}

	th, err := LoadTheme(path)
	if err != nil {
		t.Fatalf("LoadTheme returned error: %v", err)
	}

	if th.Table.HeaderFg != "#ff00ff" {
		t.Errorf("expected Table.HeaderFg #ff00ff, got %q", th.Table.HeaderFg)
	}

	// All other table fields should retain defaults
	def := DefaultTheme()
	if th.Table.HeaderBg != def.Table.HeaderBg {
		t.Errorf("expected default Table.HeaderBg %q, got %q",
			def.Table.HeaderBg, th.Table.HeaderBg)
	}
	if th.Table.RowFg != def.Table.RowFg {
		t.Errorf("expected default Table.RowFg %q, got %q",
			def.Table.RowFg, th.Table.RowFg)
	}
	if th.Table.SelectedRowBg != def.Table.SelectedRowBg {
		t.Errorf("expected default Table.SelectedRowBg %q, got %q",
			def.Table.SelectedRowBg, th.Table.SelectedRowBg)
	}
}

func assertNonEmpty(t *testing.T, field, value string) {
	t.Helper()
	if value == "" {
		t.Errorf("DefaultTheme().%s is empty", field)
	}
}
