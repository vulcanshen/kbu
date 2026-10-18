package theme

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeTheme(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "theme.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadThemeDefaults(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "none.yaml")} {
		got, err := LoadTheme(path)
		if err != nil {
			t.Fatalf("LoadTheme(%q): %v", path, err)
		}
		if *got != *DefaultTheme() {
			t.Errorf("LoadTheme(%q) = %+v, want the default", path, got)
		}
	}
}

// What the file sets wins; what it leaves out keeps its default.
func TestLoadThemePartial(t *testing.T) {
	got, err := LoadTheme(writeTheme(t, "status:\n  pending: \"#fab387\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Status.Pending != "#fab387" || got.Status.Error != DefaultTheme().Status.Error {
		t.Errorf("status %+v", got.Status)
	}
}

func TestLoadThemeInvalid(t *testing.T) {
	if _, err := LoadTheme(writeTheme(t, "status: [\n")); err == nil {
		t.Error("a broken theme.yaml loaded")
	}
}

// The sections that coloured the screen itself are named, so the App Log
// can say they no longer take effect.
func TestIgnoredSections(t *testing.T) {
	path := writeTheme(t, "sidebar:\n  foreground: \"#fff\"\nstatus:\n  error: \"#f00\"\ntable: {}\n")
	if got := IgnoredSections(path); !slices.Equal(got, []string{"sidebar", "table"}) {
		t.Errorf("IgnoredSections = %v, want sidebar, table", got)
	}
	if got := IgnoredSections(writeTheme(t, "status: {}\n")); len(got) != 0 {
		t.Errorf("only status: %v", got)
	}
	// An old theme.yaml with every section still loads.
	if _, err := LoadTheme(path); err != nil {
		t.Errorf("an old theme.yaml no longer loads: %v", err)
	}
}
