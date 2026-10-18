// Package theme reads theme.yaml: the colours of a resource's status cells,
// the one part of the screen whose colours are kbu's to choose (the app's
// content, tdp Principle P4). Everything else is drawn in the terminu
// family's colours (components/color).
package theme

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// Theme is what theme.yaml sets.
type Theme struct {
	Status StatusColors `yaml:"status"`
}

// StatusColors are the colours of a status cell: what needs a look is
// coloured, what is healthy stays plain.
type StatusColors struct {
	Running string `yaml:"running"`
	Pending string `yaml:"pending"`
	Error   string `yaml:"error"`
	Unknown string `yaml:"unknown"`
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

// LoadTheme reads a theme.yaml from the given path over the default theme:
// what the file does not set keeps its default. An empty path or a missing
// file is the default theme.
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

// IgnoredSections are the sections of the theme.yaml at path that no longer
// take effect: sidebar, table, detail, status_bar and status_line coloured
// the screen itself, which is the terminu family's now. TODO(tdp KB3):
// whether theme.yaml stays.
func IgnoredSections(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var top map[string]any
	if yaml.Unmarshal(data, &top) != nil {
		return nil
	}
	var out []string
	for k := range top {
		if k != "status" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
