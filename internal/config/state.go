package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// State captures the user's last-known TUI position across restarts.
//
// Separate from Config on purpose. Config is user-authored preferences
// (mouse, editor, pins) — a human hand-edits the YAML and expects km8
// to leave comments and structure alone. State is auto-recorded runtime
// position (last context / namespace / kind / cursor) written on every
// quit. Mixing them would pollute the user's hand-editable config with
// values km8 mutates automatically and defeat the "config.yaml is my
// document" trust boundary.
//
// Persisted at StatePath() — same config dir as config.yaml but a
// distinct file. YAML for symmetry with the rest of the config layout,
// so a user peeking at it can read / hand-edit if they want.
type State struct {
	// Context is the last-selected kubeconfig context. Empty means no
	// state recorded yet — startup falls through to Config's
	// DefaultContext / kubeconfig current-context.
	Context string `yaml:"context,omitempty"`

	// Namespace is the last-selected namespace scope. Empty is a valid
	// recorded value — it means "all namespaces". The absent/present
	// distinction is carried by whether the whole state.yaml exists at
	// all: a missing file falls through to config default, a present
	// file with an empty Namespace field is an explicit "user was on
	// all-namespaces last time".
	Namespace string `yaml:"namespace,omitempty"`

	// Kind is the last-selected sidebar entry's KubectlName (e.g.
	// "pods", "deployments", "crontabs.stable.example.com"). Empty
	// falls through to the sidebar's first entry. KubectlName is
	// used instead of the km8 ResourceType constant because it's
	// stable across km8 upgrades (a rename of the internal constant
	// wouldn't invalidate a user's saved state) and it also identifies
	// CRDs unambiguously via their fully-qualified name.
	Kind string `yaml:"kind,omitempty"`

	// ObjectNamespace + ObjectName identifies the row cursor in
	// panel 2. Both empty = no object recorded (fresh startup or
	// a kind with no rows at quit time). When Kind is cluster-scoped
	// (Namespace / Node / etc.), ObjectNamespace stays empty.
	ObjectNamespace string `yaml:"object_namespace,omitempty"`
	ObjectName      string `yaml:"object_name,omitempty"`

	// Panel is the focused panel at quit time — "sidebar", "table",
	// or "detail". Empty falls back to "sidebar" (the fresh-launch
	// default). Stored as a string rather than the Panel int enum so
	// the YAML stays readable and the enum can be reordered without
	// breaking existing state files.
	Panel string `yaml:"panel,omitempty"`
}

// DefaultState returns an empty State. Used both as the load-file-missing
// fallback and as the seed value main.go passes into NewAppModel when
// state loading errors out — either way the app runs on config defaults.
func DefaultState() *State {
	return &State{}
}

// StatePath returns the full path to the km8 state file. Sits alongside
// config.yaml in ConfigDir() rather than under a subdirectory so the
// user's config-dir stays flat and both files are equally discoverable
// on `ls`.
//
// $KM8__STATEPATH override mirrors the $KM8__CONFIGPATH pattern —
// useful for tests, sandboxed launches, or a per-project state file.
// Whitespace-trimmed for the same reason ConfigPath is: a leading
// space from a copy-pasted .env value would otherwise create a
// literal-space directory.
func StatePath() string {
	if p := strings.TrimSpace(os.Getenv("KM8__STATEPATH")); p != "" {
		return p
	}
	return filepath.Join(ConfigDir(), "state.yaml")
}

// LoadState reads the state file from the default path. A missing file
// is NOT an error — first-ever launch has no state to restore, and the
// caller falls through to config defaults. Malformed YAML IS an error
// so the user notices when their file is corrupt rather than silently
// resetting their session; callers typically log the error and continue
// with DefaultState() so a bad state file can't lock the user out of
// km8.
func LoadState() (*State, error) {
	return LoadStateFrom(StatePath())
}

// LoadStateFrom reads the state file at the given path. Split from
// LoadState so tests can drive a tempdir without touching the real
// ConfigDir().
func LoadStateFrom(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultState(), nil
		}
		return nil, fmt.Errorf("reading state file %s: %w", path, err)
	}
	s := DefaultState()
	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parsing state file %s: %w", path, err)
	}
	return s, nil
}

// Save persists the state to the default path via the same atomic
// write pattern as Config.Save — tempfile in the target dir, then
// rename. Reason to match Config: state.yaml gets rewritten on every
// quit, and a crash mid-write must not leave a half-written file that
// fails to parse on next startup (which would be worse than losing the
// session position because the LoadStateFrom error path is fatal for
// the caller unless they explicitly ignore it).
func (s *State) Save() error {
	return s.SaveTo(StatePath())
}

// SaveTo writes the state to the given path. Same atomic semantics as
// Save; separated so tests can target a tempdir.
func (s *State) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state dir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".state.*.yaml")
	if err != nil {
		return fmt.Errorf("creating state tempfile in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing state tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing state tempfile: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming state tempfile to %s: %w", path, err)
	}
	return nil
}
