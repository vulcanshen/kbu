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

// ConfigDir returns the platform-appropriate config directory for km8.
// On Linux: $XDG_CONFIG_HOME/km8 or ~/.config/km8
// On macOS: ~/Library/Application Support/km8
// On Windows: %APPDATA%/km8
func ConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to ~/.config on error
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
