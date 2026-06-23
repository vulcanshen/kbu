package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds application-level settings for km8.
// Theme settings are intentionally excluded — see internal/theme/.
type Config struct {
	// DefaultContext is the kubeconfig context to use on startup.
	// Empty string means use the current-context from kubeconfig.
	DefaultContext string `yaml:"default_context"`

	// DefaultNamespace is the namespace filter applied on startup.
	// Empty string means all namespaces.
	DefaultNamespace string `yaml:"default_namespace"`

	// Editor overrides $EDITOR for kubectl edit operations.
	// Empty string means fall back to $EDITOR, then platform default.
	Editor string `yaml:"editor"`

	// Compare carries settings for the YAML compare popup.
	Compare CompareConfig `yaml:"compare"`

	// PinnedResourceKinds is the user's "Pinned" sidebar list — each
	// entry is a resource KIND (KubectlName, e.g. "pod", "namespace",
	// "configmap"). Stable across km8 versions and matches the editing
	// format users reach for. The sidebar renders these as a top-level
	// "Pinned" category in insertion order. To reorder: remove + re-pin.
	// Unknown / no-longer-registered entries are dropped silently at
	// startup (e.g. a CRD that was uninstalled).
	//
	// Named "kinds" not "resources" because each entry pins the kind
	// itself (the sidebar navigation target), not a specific named
	// instance.
	PinnedResourceKinds []string `yaml:"pinned_resource_kinds"`
}

// CompareConfig holds settings for the YAML compare popup. Currently
// only the default layout. Read at startup; in-session toggle does
// NOT persist back (the popup's Space-menu toggle is session-local).
type CompareConfig struct {
	// Layout is the default diff layout for new compare popups.
	// Valid values: "split" (side-by-side, default) | "unified"
	// (single column with -/+ markers).  Empty or unrecognised value
	// falls back to "split".
	Layout string `yaml:"layout"`
}

// DefaultConfig returns a Config with sensible defaults.
// All fields default to empty strings, which means:
//   - DefaultContext: use kubeconfig current-context
//   - DefaultNamespace: all namespaces
//   - Editor: use $EDITOR environment variable
func DefaultConfig() *Config {
	return &Config{}
}

// appName is the directory name used under the OS config directory.
const appName = "km8"

// ConfigDir returns the config directory for km8.
// Priority: $XDG_CONFIG_HOME/km8 → platform default → ~/.config/km8
// Platform defaults: macOS=~/Library/Application Support/km8,
// Linux=~/.config/km8, Windows=%APPDATA%/km8
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appName)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", appName)
	}
	return filepath.Join(dir, appName)
}

// ConfigPath returns the full path to the km8 config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// ThemePath returns the full path to the km8 theme file.
// This is provided for the theme package to use.
func ThemePath() string {
	return filepath.Join(ConfigDir(), "theme.yaml")
}

// Load reads the config file from the default path and returns the parsed Config.
// If the config file does not exist, it returns DefaultConfig without error.
// Other I/O or parse errors are returned.
func Load() (*Config, error) {
	return LoadFrom(ConfigPath())
}

// LoadFrom reads the config file at the given path and returns the parsed Config.
// If the file does not exist, it returns DefaultConfig without error.
// Other I/O or parse errors are returned.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	return cfg, nil
}

// Save persists the config to the default path via atomic write
// (write to a tempfile in the same dir, then rename). The atomic step
// matters because km8 mutates the file during use (pin / unpin) and a
// crash mid-write must not leave a half-written config that fails to
// parse on next startup.
//
// Creates the config dir if it doesn't exist (first-ever save case).
func (c *Config) Save() error {
	return c.SaveTo(ConfigPath())
}

// SaveTo writes the config to the given path. Same atomic semantics as
// Save(); separated so tests can target a tempdir.
func (c *Config) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config.*.yaml")
	if err != nil {
		return fmt.Errorf("creating tempfile in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing tempfile: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming tempfile to %s: %w", path, err)
	}
	return nil
}
