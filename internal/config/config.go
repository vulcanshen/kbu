package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

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

	// ResourceKindConfig holds per-kind user preferences keyed by
	// KubectlName (e.g. "pod" / "namespace" / "configmap"). Single
	// home for everything km8 lets the user tune about a kind: pin
	// state, sort, future column visibility / filter. Unknown kinds
	// at load time stay in the map but are dropped from the sidebar
	// (CRD uninstalled, etc.) — the entry is preserved so a re-install
	// of the CRD silently restores the user's pin / sort.
	ResourceKindConfig map[string]ResourceKindConfigEntry `yaml:"resource_kind_config,omitempty"`
}

// ResourceKindConfigEntry is the per-kind config bag. Each sub-field
// is a pointer so YAML round-trips preserve "absent" vs "explicitly
// empty" — important so removing the last pinned kind from a kind's
// entry leaves the YAML clean rather than emitting `pinned: null`.
//
// IsEmpty reports whether the entry carries any active config, used
// at unpin time to decide whether to keep the map entry around (any
// other config present) or delete it entirely (none).
type ResourceKindConfigEntry struct {
	Pinned *PinnedConfig `yaml:"pinned,omitempty"`
}

// IsEmpty reports whether the entry has zero active config — caller
// (UnsetPinned, future UnsetSort, ...) drops the kind from the map
// when an unset leaves the entry empty so the YAML stays tidy.
func (e ResourceKindConfigEntry) IsEmpty() bool {
	return e.Pinned == nil
}

// PinnedConfig is the per-kind pin state. Order drives sidebar render
// order (ascending). Sparse — increments of 10 by default so manual
// YAML edits + future insertion-between-existing don't force a
// reindex.
type PinnedConfig struct {
	Order int `yaml:"order"`
}

// CompareConfig holds settings for the YAML compare popup. Currently
// only the default layout. Read at startup; in-session toggle does
// NOT persist back (the popup's Space-menu toggle is session-local).
type CompareConfig struct {
	// Layout is the default diff layout for new compare popups.
	// Valid values: "unified" (single column with -/+ markers,
	// default) | "split" (side-by-side). Empty or unrecognised value
	// falls back to "unified".
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

// GetPinned returns the pin state for kind, or nil when not pinned.
func (c *Config) GetPinned(kind string) *PinnedConfig {
	if c.ResourceKindConfig == nil {
		return nil
	}
	entry, ok := c.ResourceKindConfig[kind]
	if !ok {
		return nil
	}
	return entry.Pinned
}

// IsPinned is the bool convenience wrapper.
func (c *Config) IsPinned(kind string) bool {
	return c.GetPinned(kind) != nil
}

// SetPinned upserts the pin entry for kind. Caller chooses Order —
// typically via NextPinOrder() when appending, or any int when the
// user manually moves a pin around. Pre-existing pin is overwritten.
func (c *Config) SetPinned(kind string, order int) {
	if c.ResourceKindConfig == nil {
		c.ResourceKindConfig = make(map[string]ResourceKindConfigEntry)
	}
	entry := c.ResourceKindConfig[kind]
	entry.Pinned = &PinnedConfig{Order: order}
	c.ResourceKindConfig[kind] = entry
}

// UnsetPinned drops the pin state for kind. When the entry has no
// other active config left (IsEmpty), the entire map entry is removed
// so the YAML stays tidy. Without this housekeeping, unpinning the
// only thing on a kind would leave `kind: {}` in the file.
func (c *Config) UnsetPinned(kind string) {
	if c.ResourceKindConfig == nil {
		return
	}
	entry, ok := c.ResourceKindConfig[kind]
	if !ok {
		return
	}
	entry.Pinned = nil
	if entry.IsEmpty() {
		delete(c.ResourceKindConfig, kind)
	} else {
		c.ResourceKindConfig[kind] = entry
	}
}

// PinnedOrdered returns kinds in display order — ascending Pinned.Order.
// Empty slice when no pins exist. Stable across calls.
func (c *Config) PinnedOrdered() []string {
	if c.ResourceKindConfig == nil {
		return nil
	}
	type kv struct {
		kind  string
		order int
	}
	pairs := make([]kv, 0, len(c.ResourceKindConfig))
	for kind, entry := range c.ResourceKindConfig {
		if entry.Pinned != nil {
			pairs = append(pairs, kv{kind, entry.Pinned.Order})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].order < pairs[j].order
	})
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.kind
	}
	return out
}

// NextPinOrder returns the next sparse Order to use when appending a
// new pin (max existing + 10). Sparse increment lets the user wedge a
// kind between two existing pins by manually editing YAML without
// having to renumber the rest.
func (c *Config) NextPinOrder() int {
	max := 0
	if c.ResourceKindConfig == nil {
		return 10
	}
	for _, entry := range c.ResourceKindConfig {
		if entry.Pinned != nil && entry.Pinned.Order > max {
			max = entry.Pinned.Order
		}
	}
	return max + 10
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
