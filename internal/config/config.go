package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

	// AltermShell overrides $SHELL for the Alterm internal terminal
	// popup. Empty string means fall back to $SHELL, then /bin/sh —
	// the typical case where the user wants the same login shell as
	// their host terminal. Resolved via Go's exec.Command(name, args)
	// — bare names (e.g. `fish`) are looked up on $PATH at popup-open
	// time; absolute paths are used verbatim. Distinct from `Editor`
	// so users can pick e.g. `fish` for an interactive alterm while
	// keeping vim/$EDITOR for kubectl edit.
	AltermShell string `yaml:"alterm_shell"`

	// AltermLoginShell launches the Alterm shell with `-l` so it
	// sources login dotfiles (~/.zprofile, ~/.bash_profile, /etc/
	// profile). Default false because the login path on macOS bash
	// force-sets PS1 from /etc/profile and clobbered the user's clean
	// prompt; the v1.7.2 baseline runs non-login interactive so
	// .bashrc / .zshrc still loads but no PS1 surprise.
	//
	// Flip true when km8 is launched from a NON-login parent shell
	// (Raycast / Alfred / cron / tmux configured non-login) and your
	// PATH lives in .zprofile / .bash_profile rather than .zshrc —
	// without `-l` those dotfiles never run and Alterm sees a
	// stripped PATH that can't find brew/asdf/mise binaries.
	AltermLoginShell bool `yaml:"alterm_login_shell"`

	// Compare carries settings for the YAML compare popup.
	Compare CompareConfig `yaml:"compare"`

	// Mouse carries app-level mouse settings (enable toggle + scroll
	// direction). Nested so future mouse options (double-click
	// threshold, edge-pan, ...) drop in here without polluting the
	// top-level Config. nil = use defaults (enabled, natural
	// scrolling) — matches how an existing user's config behaves on
	// upgrade.
	Mouse *MouseConfig `yaml:"mouse_opt_config,omitempty"`

	// ResourceKindConfig holds per-kind user preferences keyed by
	// KubectlName (e.g. "pod" / "namespace" / "configmap"). Single
	// home for everything km8 lets the user tune about a kind: pin
	// state, sort, future column visibility / filter. Unknown kinds
	// at load time stay in the map but are dropped from the sidebar
	// (CRD uninstalled, etc.) — the entry is preserved so a re-install
	// of the CRD silently restores the user's pin / sort.
	ResourceKindConfig map[string]ResourceKindConfigEntry `yaml:"resource_kind_config,omitempty"`

	// DeprecationWarnings collected during LoadFrom — populated when the
	// loader migrates a deprecated yaml key (e.g. km8erm_shell →
	// alterm_shell from the v1.7.5 rename). Not yaml-tagged: never
	// persisted, lives only for the lifetime of the loaded Config.
	// AppModel emits each entry to the App Log on startup so the user
	// sees the nudge in the `!` popup. Removable next release once the
	// transition period ends.
	DeprecationWarnings []string `yaml:"-"`
}

// ResourceKindConfigEntry is the per-kind config bag. Each sub-field
// is a pointer / collection so YAML round-trips preserve "absent" vs
// "explicitly empty" — important so removing the last pinned kind from
// a kind's entry leaves the YAML clean rather than emitting
// `pinned: null`.
//
// IsEmpty reports whether the entry carries any active config, used
// at unpin/unsort time to decide whether to keep the map entry
// around (any other config present) or delete it entirely (none).
type ResourceKindConfigEntry struct {
	Pinned *PinnedConfig `yaml:"pinned,omitempty"`
	Sort   SortChain     `yaml:"sort,omitempty"`
}

// IsEmpty reports whether the entry has zero active config — caller
// (UnsetPinned / UnsetSortColumn / ResetSort / ...) drops the kind
// from the map when an unset leaves the entry empty so the YAML
// stays tidy.
func (e ResourceKindConfigEntry) IsEmpty() bool {
	return e.Pinned == nil && len(e.Sort) == 0
}

// PinnedConfig is the per-kind pin state. Order drives sidebar render
// order (ascending). Sparse — increments of 10 by default so manual
// YAML edits + future insertion-between-existing don't force a
// reindex.
type PinnedConfig struct {
	Order int `yaml:"order"`
}

// Sort direction constants — the YAML strings stored verbatim in the
// config file. Anything else (including empty) is treated as
// "unsorted" by the UI layer, so writing garbage to the file
// harmlessly degrades to default order.
const (
	SortDirectionAscending  = "asc"
	SortDirectionDescending = "desc"
)

// SortConfig is one tier of the sort chain — a column + a direction.
// The chain (SortChain) is the ordered list of these; tier 0 is the
// primary sort, tier 1 the first tiebreaker, etc. Column is the
// column Title as defined in the resource registry (e.g. "Name" /
// "Age" / "Status") — stable per kind, readable in YAML, doesn't
// depend on column order. Direction is one of SortDirection* above.
type SortConfig struct {
	Column    string `yaml:"column"`
	Direction string `yaml:"direction"`
}

// SortChain is the ordered list of sort tiers for a kind. Index 0 is
// the primary sort, 1 is the first tiebreaker, and so on. Persisted
// as a YAML sequence; reads ALSO accept the legacy v1.6 single-
// mapping shape (`sort: {column, direction}`) and transparently lift
// it into a one-element chain — old configs upgrade with no user
// action.
type SortChain []SortConfig

// UnmarshalYAML accepts both the new sequence form and the legacy
// single-mapping form. The legacy form was the v1.6 shape; the
// sequence form is the multi-column shape introduced in this phase.
// Writes always emit the sequence form (default Go yaml behaviour
// for slices), so a load-then-save cleanly migrates old configs.
//
// Defensive: also accepts an empty / null scalar (`sort: null` or
// `sort:` with no value) by treating it as an empty chain. Without
// this, a user manually editing YAML to wipe a sort entry could
// produce a node shape that refuses to load.
func (s *SortChain) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		var list []SortConfig
		if err := node.Decode(&list); err != nil {
			return err
		}
		*s = list
		return nil
	case yaml.MappingNode:
		var one SortConfig
		if err := node.Decode(&one); err != nil {
			return err
		}
		*s = SortChain{one}
		return nil
	case yaml.ScalarNode:
		// Tolerate `sort: null` and `sort: ""` — both surface as
		// scalars and should map to empty chain rather than error.
		// Any other scalar value is rejected.
		if node.Tag == "!!null" || node.Value == "" {
			*s = nil
			return nil
		}
		return fmt.Errorf("sort: scalar value %q is not supported (use mapping or sequence)", node.Value)
	case 0:
		// Absent — empty chain.
		*s = nil
		return nil
	}
	return fmt.Errorf("sort: expected mapping (legacy) or sequence (new), got node kind %d", node.Kind)
}

// IndexOf returns the position of the given column in the chain, or
// -1 when absent. Used by SetSort/UnsetSortColumn to upsert in place
// without scanning twice.
func (s SortChain) IndexOf(column string) int {
	for i, t := range s {
		if t.Column == column {
			return i
		}
	}
	return -1
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

// Mouse scroll direction constants — written verbatim to YAML and
// matched in the wheel translator. Anything not equal to
// MouseScrollReverse is treated as natural so a typo / future value
// degrades gracefully.
const (
	MouseScrollNatural = "natural"
	MouseScrollReverse = "reverse"
)

// MouseConfig is the nested config bag for everything mouse-related.
// Lives behind a *MouseConfig in Config so legacy/empty configs
// stay nil-friendly without leaking yaml `null`s into the file.
//
// Enabled is a pointer so the helper can distinguish "never set"
// (legacy, treat as enabled) from "explicit false" (user turned it
// off). ScrollDirection is a string so future values (e.g. a
// per-axis variant) can be added without breaking the YAML shape.
type MouseConfig struct {
	Enabled         *bool  `yaml:"enabled,omitempty"`
	ScrollDirection string `yaml:"scroll_direction,omitempty"`
}

// IsMouseEnabled reports the current mouse-enabled state. nil
// MouseConfig or nil Enabled both fall back to true so an existing
// user's config "just works" the moment this feature ships.
func (c *Config) IsMouseEnabled() bool {
	if c == nil || c.Mouse == nil || c.Mouse.Enabled == nil {
		return true
	}
	return *c.Mouse.Enabled
}

// SetMouseEnabled writes the explicit on/off value, lazily creating
// the nested MouseConfig if it's the first mouse setting touched.
// Callers Save() to persist.
func (c *Config) SetMouseEnabled(v bool) {
	if c.Mouse == nil {
		c.Mouse = &MouseConfig{}
	}
	c.Mouse.Enabled = &v
}

// MouseScrollDirection returns the configured wheel-direction string,
// defaulting to natural when unset. Validity isn't enforced here —
// the wheel translator only special-cases "reverse" and treats
// everything else (including unrecognised values) as natural.
func (c *Config) MouseScrollDirection() string {
	if c == nil || c.Mouse == nil || c.Mouse.ScrollDirection == "" {
		return MouseScrollNatural
	}
	return c.Mouse.ScrollDirection
}

// SetMouseScrollDirection writes the explicit scroll direction
// string. Lazily creates MouseConfig if needed.
func (c *Config) SetMouseScrollDirection(d string) {
	if c.Mouse == nil {
		c.Mouse = &MouseConfig{}
	}
	c.Mouse.ScrollDirection = d
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
//
// $KM8__CONFIGPATH wins outright — lets the user point km8 at a config
// file outside the normal config-dir layout (e.g. a per-project YAML
// committed to a repo, or a tmpfs path on CI). Theme file is NOT
// affected — it still lives at ConfigDir()/theme.yaml. Absolute path
// recommended; a relative value resolves against CWD at load/save time.
//
// Leading / trailing whitespace is TrimSpace'd before the empty
// check — without that, `KM8__CONFIGPATH=" /path/cfg.yaml"` (leading
// space from copy-paste or a sourced .env) would slip through and
// reach os.ReadFile verbatim → ENOENT → silent fallback to defaults
// on load, plus a literal-space directory created under CWD on save.
// The trim is the single source of truth so other readers of the env
// (e.g. the NewAppModel startup notice) don't have to repeat it.
func ConfigPath() string {
	if p := strings.TrimSpace(os.Getenv("KM8__CONFIGPATH")); p != "" {
		return p
	}
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

	// v1.7.5 KM8erm → Alterm rename transition: read the legacy keys
	// (km8erm_shell / km8erm_login_shell) when the new keys are absent
	// so existing config.yaml files keep working. Second yaml pass into
	// a raw map detects key PRESENCE — needed because bool's zero value
	// (false) is indistinguishable from "key absent" without it. Save()
	// never writes the deprecated keys, so the migration is one-shot:
	// next save rewrites the file with new keys only and the warning
	// stops on subsequent loads. Removable next release.
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err == nil {
		_, newShellPresent := raw["alterm_shell"]
		if oldShell, oldShellPresent := raw["km8erm_shell"]; oldShellPresent {
			if !newShellPresent {
				if s, ok := oldShell.(string); ok {
					cfg.AltermShell = s
				}
			}
			cfg.DeprecationWarnings = append(cfg.DeprecationWarnings,
				"config: 'km8erm_shell' is deprecated, rename to 'alterm_shell' (auto-migrated this session — the next km8 save will rewrite your config.yaml with the new key)")
		}

		_, newLoginPresent := raw["alterm_login_shell"]
		if oldLogin, oldLoginPresent := raw["km8erm_login_shell"]; oldLoginPresent {
			if !newLoginPresent {
				if b, ok := oldLogin.(bool); ok {
					cfg.AltermLoginShell = b
				}
			}
			cfg.DeprecationWarnings = append(cfg.DeprecationWarnings,
				"config: 'km8erm_login_shell' is deprecated, rename to 'alterm_login_shell' (auto-migrated this session — the next km8 save will rewrite your config.yaml with the new key)")
		}
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

// GetSort returns the full sort chain for kind — nil/empty when no
// sort is set. The returned slice is shared with the config struct,
// callers MUST treat it as read-only.
func (c *Config) GetSort(kind string) SortChain {
	if c.ResourceKindConfig == nil {
		return nil
	}
	entry, ok := c.ResourceKindConfig[kind]
	if !ok {
		return nil
	}
	return entry.Sort
}

// SetSort upserts a tier into the kind's chain. If `column` is
// already in the chain, its direction is updated in place — priority
// is preserved (no reorder). Otherwise the tier is appended as the
// lowest-priority entry. Empty direction is rejected silently to
// keep the chain well-formed.
func (c *Config) SetSort(kind, column, direction string) {
	if column == "" || direction == "" {
		return
	}
	if c.ResourceKindConfig == nil {
		c.ResourceKindConfig = make(map[string]ResourceKindConfigEntry)
	}
	entry := c.ResourceKindConfig[kind]
	if idx := entry.Sort.IndexOf(column); idx >= 0 {
		entry.Sort[idx].Direction = direction
	} else {
		entry.Sort = append(entry.Sort, SortConfig{Column: column, Direction: direction})
	}
	c.ResourceKindConfig[kind] = entry
}

// UnsetSortColumn removes a single tier from the kind's chain.
// Other tiers shift up to fill the gap (priority N+1 becomes N).
// When the chain becomes empty AND the entry has no other active
// config, the entire map entry is dropped — same housekeeping as
// UnsetPinned.
func (c *Config) UnsetSortColumn(kind, column string) {
	if c.ResourceKindConfig == nil {
		return
	}
	entry, ok := c.ResourceKindConfig[kind]
	if !ok {
		return
	}
	idx := entry.Sort.IndexOf(column)
	if idx < 0 {
		return
	}
	entry.Sort = append(entry.Sort[:idx], entry.Sort[idx+1:]...)
	if len(entry.Sort) == 0 {
		entry.Sort = nil
	}
	if entry.IsEmpty() {
		delete(c.ResourceKindConfig, kind)
	} else {
		c.ResourceKindConfig[kind] = entry
	}
}

// ResetSort clears the entire sort chain for kind. Same housekeeping
// as UnsetSortColumn (drops the map entry when nothing else lives
// there). Backs the "Reset" shortcut in the column picker.
func (c *Config) ResetSort(kind string) {
	if c.ResourceKindConfig == nil {
		return
	}
	entry, ok := c.ResourceKindConfig[kind]
	if !ok {
		return
	}
	entry.Sort = nil
	if entry.IsEmpty() {
		delete(c.ResourceKindConfig, kind)
	} else {
		c.ResourceKindConfig[kind] = entry
	}
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

// BackupBeforeMigration copies the current config file to a sibling
// `<srcPath>.old.<vers>` file so a user-edited config (comments,
// custom fields, hand-written formatting) is preserved verbatim
// before any Save-triggered rewrite that goes through yaml.Marshal
// — which can't preserve those. Returns the backup path on success.
//
// `vers` is the running km8 release with dots replaced by
// underscores so the suffix sorts cleanly and doesn't confuse
// path-splitters that special-case dot (e.g. `1_7_5`, or `dev` for
// local builds).
//
// If the target backup already exists, it is overwritten — the most
// recent pre-migration snapshot is the most relevant for the user
// asking "what did my config look like just before this run rewrote
// it". Same-version migrations are idempotent, so re-running doesn't
// produce a confusingly stale backup.
func BackupBeforeMigration(srcPath, vers string) (string, error) {
	bakPath := srcPath + ".old." + vers
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("reading config for backup: %w", err)
	}
	if err := os.WriteFile(bakPath, data, 0o644); err != nil {
		return "", fmt.Errorf("writing backup %s: %w", bakPath, err)
	}
	return bakPath, nil
}
