package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds application-level settings for kbu.
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
	// Flip true when kbu is launched from a NON-login parent shell
	// (Raycast / Alfred / cron / tmux configured non-login) and your
	// PATH lives in .zprofile / .bash_profile rather than .zshrc —
	// without `-l` those dotfiles never run and Alterm sees a
	// stripped PATH that can't find brew/asdf/mise binaries.
	AltermLoginShell bool `yaml:"alterm_login_shell"`

	// InputTabWidth is the terminu family's input_tab_width (tdp E3): how
	// many cells a Tab takes in the viewers (YAML, logs). 0 means the
	// family's default, 4.
	InputTabWidth int `yaml:"input_tab_width,omitempty"`

	// Mouse carries app-level mouse settings (enable toggle + scroll
	// direction). Nested so future mouse options (double-click
	// threshold, edge-pan, ...) drop in here without polluting the
	// top-level Config. nil = use defaults (enabled, natural
	// scrolling) — matches how an existing user's config behaves on
	// upgrade.
	Mouse *MouseConfig `yaml:"mouse_opt_config,omitempty"`

	// ResourceKindConfig is where an older kbu kept the pins and sorts.
	// They are data now, in kinds.yaml (KindData; tdp E3), and
	// MoveKindData moves them there at start. They are read here only
	// while they could not be moved, kbu then using them here
	// (KindDataInConfig), or while kinds.yaml was there already, left
	// for the user to merge; either way a Save keeps them.
	ResourceKindConfig KindData `yaml:"resource_kind_config,omitempty"`

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
const appName = "kbu"

// legacyAppName is the pre-v2.0 app name. Kept for one-shot config
// dir migration (see MigrateLegacyConfigDir).
const legacyAppName = "km8"

// ConfigDir returns kbu's config directory: config.yaml and — unless
// $KBU__STATE says otherwise — state.yaml live in it.
// Priority (tdp E3): $KBU__CONFIG → $XDG_CONFIG_HOME/kbu → ~/.config/kbu,
// the same on every platform (on Windows ~ is %USERPROFILE%).
//
// An older kbu kept the directory where the platform keeps an app's
// settings (macOS ~/Library/Application Support/kbu, Windows
// %APPDATA%\kbu). MoveOldPlaces moves it at start; while it could
// not and that one is the only one there, kbu keeps reading and writing it,
// so nothing the user had is lost (tdp E3).
//
// $KBU__CONFIG is the family's <APP>__CONFIG (tdp E2): a directory — a
// per-project one, a tmpfs path on CI, a sandbox for a test run. It is
// whitespace-trimmed first: a leading space from a copy-pasted .env
// value would otherwise create a literal-space directory.
func ConfigDir() string {
	if d := strings.TrimSpace(os.Getenv("KBU__CONFIG")); d != "" {
		return d
	}
	dir := familyConfigDir(appName)
	if old := platformConfigDir(appName); old != "" && old != dir && !exists(dir) && exists(old) {
		return old
	}
	return dir
}

// ErrThereAlready is why an item was left where it was: its new place has
// one already, for the user to merge.
var ErrThereAlready = errors.New("the new place has one already")

// Moved is one item MoveOldPlaces moved, or left where it was (Err).
type Moved struct {
	What     string // "settings" or "logs"
	From, To string
	Err      error // it stays in From: ErrThereAlready, or why it could not move
}

// MoveOldPlaces moves what an older kbu kept elsewhere to where tdp E3 puts
// it now, item by item, at the first start (tdp E3: a new place): the
// settings from the platform's settings directory (macOS
// ~/Library/Application Support/kbu, Windows %APPDATA%\kbu) to
// ~/.config/kbu, and the logs from the settings directory to the data one
// (~/.kbu/logs). What the new place has already stays where it was; when a
// setting cannot be moved none is, and kbu keeps using the old directory
// (ConfigDir). The old directory goes once it is empty.
func MoveOldPlaces() []Moved {
	var out []Moved
	if strings.TrimSpace(os.Getenv("KBU__CONFIG")) == "" {
		out = moveSettings()
	}
	out = append(out, moveLogs()...)
	if old := platformConfigDir(appName); old != "" && old != familyConfigDir(appName) {
		_ = os.Remove(old) // only when it is empty
	}
	return out
}

func moveSettings() []Moved {
	to, from := familyConfigDir(appName), platformConfigDir(appName)
	if from == "" || from == to || !exists(from) {
		return nil
	}
	whole := Moved{What: "settings", From: from, To: to}
	entries, err := os.ReadDir(from)
	if err != nil {
		whole.Err = err
		return []Moved{whole}
	}
	made := !exists(to)
	if err := os.MkdirAll(to, 0o755); err != nil {
		whole.Err = err
		return []Moved{whole}
	}
	var out []Moved
	for _, e := range entries {
		if e.Name() == "logs" {
			continue // moveLogs takes them to the data directory
		}
		mv := Moved{What: "settings", From: filepath.Join(from, e.Name()), To: filepath.Join(to, e.Name())}
		if exists(mv.To) {
			mv.Err = ErrThereAlready
		} else if err := os.Rename(mv.From, mv.To); err != nil {
			if made {
				// None is moved: kbu keeps using the old directory.
				for _, done := range out {
					_ = os.Rename(done.To, done.From)
				}
				_ = os.Remove(to)
				whole.Err = err
				return []Moved{whole}
			}
			mv.Err = err
		}
		out = append(out, mv)
	}
	return out
}

func moveLogs() []Moved {
	to := LogDir()
	var out []Moved
	seen := map[string]bool{}
	for _, dir := range []string{platformConfigDir(appName), ConfigDir()} {
		from := filepath.Join(dir, "logs")
		if dir == "" || seen[from] || from == to || !exists(from) {
			continue
		}
		seen[from] = true
		mv := Moved{What: "logs", From: from, To: to}
		if exists(to) {
			mv.Err = ErrThereAlready
		} else if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			mv.Err = err
		} else if err := os.Rename(from, to); err != nil {
			mv.Err = err
		}
		out = append(out, mv)
	}
	return out
}

// familyConfigDir is where tdp E3 puts an app's settings:
// $XDG_CONFIG_HOME/<app>, falling back to ~/.config/<app>.
func familyConfigDir(app string) string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, app)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", app)
}

// platformConfigDir is where an older kbu put its settings when
// $XDG_CONFIG_HOME was unset: the platform's own settings directory.
// "" when $XDG_CONFIG_HOME is set (both then agree) or there is none.
func platformConfigDir(app string) string {
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		return ""
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, app)
}

// exists reports whether path is there.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// legacyConfigDir is where the pre-v2.0 km8 kept its settings: the
// platform's settings directory, or $XDG_CONFIG_HOME when set. Only used
// by MigrateLegacyConfigDir; every other caller uses ConfigDir directly.
func legacyConfigDir() string {
	if old := platformConfigDir(legacyAppName); old != "" {
		return old
	}
	return familyConfigDir(legacyAppName)
}

// NoLongerUsed is what an older kbu kept that this one no longer reads, for
// the App Log at every start while it is there (tdp E3, files no longer
// used): theme.yaml, whose colours the family's took over, where the
// settings are (the old directory's moved with them); and km8's directory,
// which v2.0 copied to kbu's and left in place. Nothing is deleted: what to
// do with them is the user's call.
func NoLongerUsed() []string {
	dir := ConfigDir()
	paths := []string{filepath.Join(dir, "theme.yaml")}
	if !exists(dir) {
		return paths // km8's is still to be copied (MigrateLegacyConfigDir)
	}
	for _, old := range []string{platformConfigDir(legacyAppName), familyConfigDir(legacyAppName)} {
		if old != "" && old != dir && !slices.Contains(paths, old) {
			paths = append(paths, old)
		}
	}
	return paths
}

// ConfigPath returns the full path to the kbu config file: config.yaml
// in ConfigDir(), which $KBU__CONFIG moves.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
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
				"Config: 'km8erm_shell' is deprecated, rename to 'alterm_shell' (auto-migrated this session — the next kbu save will rewrite your config.yaml with the new key)")
		}

		_, newLoginPresent := raw["alterm_login_shell"]
		if oldLogin, oldLoginPresent := raw["km8erm_login_shell"]; oldLoginPresent {
			if !newLoginPresent {
				if b, ok := oldLogin.(bool); ok {
					cfg.AltermLoginShell = b
				}
			}
			cfg.DeprecationWarnings = append(cfg.DeprecationWarnings,
				"Config: 'km8erm_login_shell' is deprecated, rename to 'alterm_login_shell' (auto-migrated this session — the next kbu save will rewrite your config.yaml with the new key)")
		}
	}

	return cfg, nil
}

// Save persists the config to the default path via atomic write
// (write to a tempfile in the same dir, then rename). The atomic step
// matters because kbu mutates the file during use (the compare layout) and a
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
// `vers` is the running kbu release with dots replaced by
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
