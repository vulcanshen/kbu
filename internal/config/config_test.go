package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.DefaultContext != "" {
		t.Errorf("DefaultContext: got %q, want empty string", cfg.DefaultContext)
	}
	if cfg.DefaultNamespace != "" {
		t.Errorf("DefaultNamespace: got %q, want empty string", cfg.DefaultNamespace)
	}
	if cfg.Editor != "" {
		t.Errorf("Editor: got %q, want empty string", cfg.Editor)
	}
}

func TestConfigDir(t *testing.T) {
	dir := ConfigDir()

	if dir == "" {
		t.Fatal("ConfigDir() returned empty string")
	}
	if filepath.Base(dir) != "km8" {
		t.Errorf("ConfigDir() base: got %q, want %q", filepath.Base(dir), "km8")
	}
}

func TestConfigPath(t *testing.T) {
	// Isolate from a possibly-set KM8__CONFIGPATH in the runner env;
	// this test asserts the default-layout shape.
	orig, had := os.LookupEnv("KM8__CONFIGPATH")
	os.Unsetenv("KM8__CONFIGPATH")
	defer func() {
		if had {
			os.Setenv("KM8__CONFIGPATH", orig)
		}
	}()

	path := ConfigPath()

	if path == "" {
		t.Fatal("ConfigPath() returned empty string")
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("ConfigPath() base: got %q, want %q", filepath.Base(path), "config.yaml")
	}
	// ConfigPath should be under ConfigDir
	if filepath.Dir(path) != ConfigDir() {
		t.Errorf("ConfigPath() dir: got %q, want %q", filepath.Dir(path), ConfigDir())
	}
}

func TestConfigPath_RespectsKM8ConfigPathEnv(t *testing.T) {
	// $KM8__CONFIGPATH wins outright — caller is responsible for the
	// path validity, we just thread it through.
	orig, had := os.LookupEnv("KM8__CONFIGPATH")
	defer func() {
		if had {
			os.Setenv("KM8__CONFIGPATH", orig)
		} else {
			os.Unsetenv("KM8__CONFIGPATH")
		}
	}()

	os.Setenv("KM8__CONFIGPATH", "/tmp/custom-km8.yaml")
	if got := ConfigPath(); got != "/tmp/custom-km8.yaml" {
		t.Errorf("ConfigPath() with env: got %q, want %q", got, "/tmp/custom-km8.yaml")
	}
}

func TestConfigPath_EmptyEnvFallsBack(t *testing.T) {
	// Empty string = treat as unset; fall back to the default layout.
	orig, had := os.LookupEnv("KM8__CONFIGPATH")
	defer func() {
		if had {
			os.Setenv("KM8__CONFIGPATH", orig)
		} else {
			os.Unsetenv("KM8__CONFIGPATH")
		}
	}()

	os.Setenv("KM8__CONFIGPATH", "")
	got := ConfigPath()
	if filepath.Base(got) != "config.yaml" || filepath.Dir(got) != ConfigDir() {
		t.Errorf("empty env must fall back to default layout, got %q", got)
	}
}

func TestThemePath(t *testing.T) {
	path := ThemePath()

	if path == "" {
		t.Fatal("ThemePath() returned empty string")
	}
	if filepath.Base(path) != "theme.yaml" {
		t.Errorf("ThemePath() base: got %q, want %q", filepath.Base(path), "theme.yaml")
	}
	// ThemePath should be under ConfigDir
	if filepath.Dir(path) != ConfigDir() {
		t.Errorf("ThemePath() dir: got %q, want %q", filepath.Dir(path), ConfigDir())
	}
}

func TestLoadFrom_FileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent", "config.yaml")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFrom() returned nil config")
	}

	// Should return default values
	if cfg.DefaultContext != "" {
		t.Errorf("DefaultContext: got %q, want empty", cfg.DefaultContext)
	}
	if cfg.DefaultNamespace != "" {
		t.Errorf("DefaultNamespace: got %q, want empty", cfg.DefaultNamespace)
	}
	if cfg.Editor != "" {
		t.Errorf("Editor: got %q, want empty", cfg.Editor)
	}
}

func TestLoad_ReturnsDefaultWhenNoFile(t *testing.T) {
	// Load() should not error even if config file doesn't exist.
	// On a clean system (or CI), the config file won't exist.
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
}

func TestLoadFrom_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`default_context: my-cluster
default_namespace: production
editor: nvim
`)
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	if cfg.DefaultContext != "my-cluster" {
		t.Errorf("DefaultContext: got %q, want %q", cfg.DefaultContext, "my-cluster")
	}
	if cfg.DefaultNamespace != "production" {
		t.Errorf("DefaultNamespace: got %q, want %q", cfg.DefaultNamespace, "production")
	}
	if cfg.Editor != "nvim" {
		t.Errorf("Editor: got %q, want %q", cfg.Editor, "nvim")
	}
}

func TestLoadFrom_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`{{{invalid yaml`)
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom() expected error for invalid YAML, got nil")
	}
}

func TestLoadFrom_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Only set one field — others should remain at defaults
	content := []byte(`editor: code
`)
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	if cfg.Editor != "code" {
		t.Errorf("Editor: got %q, want %q", cfg.Editor, "code")
	}
	if cfg.DefaultContext != "" {
		t.Errorf("DefaultContext: got %q, want empty", cfg.DefaultContext)
	}
	if cfg.DefaultNamespace != "" {
		t.Errorf("DefaultNamespace: got %q, want empty", cfg.DefaultNamespace)
	}
}

func TestLoadFrom_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	// Empty file should yield default config
	if cfg.DefaultContext != "" {
		t.Errorf("DefaultContext: got %q, want empty", cfg.DefaultContext)
	}
	if cfg.DefaultNamespace != "" {
		t.Errorf("DefaultNamespace: got %q, want empty", cfg.DefaultNamespace)
	}
	if cfg.Editor != "" {
		t.Errorf("Editor: got %q, want empty", cfg.Editor)
	}
}

func TestSaveLoadRoundtrip_ResourceKindConfig(t *testing.T) {
	// Round-trip the new resource_kind_config shape: pinned state + the
	// other primitives that share the YAML file must all survive a
	// save→load cycle. Migration is tested separately.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		Editor:  "vim",
		Compare: CompareConfig{Layout: "unified"},
	}
	cfg.SetPinned("pod", 10)
	cfg.SetPinned("namespace", 20)
	cfg.SetPinned("configmap", 30)

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	wantOrdered := []string{"pod", "namespace", "configmap"}
	got := loaded.PinnedOrdered()
	if len(got) != len(wantOrdered) {
		t.Fatalf("PinnedOrdered len = %d, want %d (got %v)", len(got), len(wantOrdered), got)
	}
	for i, kind := range wantOrdered {
		if got[i] != kind {
			t.Errorf("PinnedOrdered[%d] = %q, want %q (Order drives sidebar render)", i, got[i], kind)
		}
	}
	// Order values survive the round-trip — important if the user
	// hand-edits YAML to leave gaps.
	if p := loaded.GetPinned("pod"); p == nil || p.Order != 10 {
		t.Errorf("pod Order lost in roundtrip, got %+v", p)
	}
	if loaded.Editor != "vim" {
		t.Errorf("Editor lost in roundtrip: %q", loaded.Editor)
	}
	if loaded.Compare.Layout != "unified" {
		t.Errorf("Compare.Layout lost in roundtrip: %q", loaded.Compare.Layout)
	}
}

func TestPinnedHelpers_SetUnsetGet(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.IsPinned("pod") {
		t.Error("fresh config should have no pins")
	}
	cfg.SetPinned("pod", 10)
	if !cfg.IsPinned("pod") {
		t.Error("IsPinned should report true after SetPinned")
	}
	if p := cfg.GetPinned("pod"); p == nil || p.Order != 10 {
		t.Errorf("GetPinned returned %+v, want {Order:10}", p)
	}

	// SetPinned overwrites Order on existing entry.
	cfg.SetPinned("pod", 25)
	if p := cfg.GetPinned("pod"); p == nil || p.Order != 25 {
		t.Errorf("SetPinned must overwrite Order, got %+v", p)
	}

	// UnsetPinned drops the kind entry entirely when nothing else
	// occupies it — keeps the YAML clean.
	cfg.UnsetPinned("pod")
	if cfg.IsPinned("pod") {
		t.Error("IsPinned should be false after UnsetPinned")
	}
	if _, ok := cfg.ResourceKindConfig["pod"]; ok {
		t.Error("empty entry must be deleted from the map after UnsetPinned")
	}

	// UnsetPinned on a kind that isn't pinned is a no-op.
	cfg.UnsetPinned("never-pinned")
}

func TestSortHelpers_SetUnsetGet(t *testing.T) {
	cfg := DefaultConfig()
	if got := cfg.GetSort("pod"); len(got) != 0 {
		t.Errorf("fresh config GetSort = %+v, want empty", got)
	}

	cfg.SetSort("pod", "Age", SortDirectionDescending)
	chain := cfg.GetSort("pod")
	if len(chain) != 1 {
		t.Fatalf("GetSort returned %d tiers after first SetSort, want 1", len(chain))
	}
	if chain[0].Column != "Age" || chain[0].Direction != SortDirectionDescending {
		t.Errorf("GetSort tier 0 = %+v, want {Column:Age Direction:desc}", chain[0])
	}

	// SetSort on a NEW column appends — chain grows; priority is
	// insertion order.
	cfg.SetSort("pod", "Name", SortDirectionAscending)
	chain = cfg.GetSort("pod")
	if len(chain) != 2 || chain[0].Column != "Age" || chain[1].Column != "Name" {
		t.Errorf("chain after two distinct SetSort = %+v, want [{Age desc} {Name asc}]", chain)
	}

	// SetSort on a column ALREADY in the chain updates direction in
	// place — priority is preserved.
	cfg.SetSort("pod", "Age", SortDirectionAscending)
	chain = cfg.GetSort("pod")
	if len(chain) != 2 || chain[0].Column != "Age" || chain[0].Direction != SortDirectionAscending {
		t.Errorf("in-place direction update lost: chain=%+v", chain)
	}

	// UnsetSortColumn removes a single tier; remaining tiers shift up.
	cfg.UnsetSortColumn("pod", "Age")
	chain = cfg.GetSort("pod")
	if len(chain) != 1 || chain[0].Column != "Name" {
		t.Errorf("after removing Age, chain = %+v, want [{Name asc}]", chain)
	}

	// Removing the last tier on an entry with no other config drops
	// the map entry.
	cfg.UnsetSortColumn("pod", "Name")
	if got := cfg.GetSort("pod"); len(got) != 0 {
		t.Errorf("GetSort after removing last tier = %+v, want empty", got)
	}
	if _, ok := cfg.ResourceKindConfig["pod"]; ok {
		t.Error("entry must be deleted once chain is empty and nothing else lives there")
	}

	// Operations on absent kinds / columns are silent no-ops.
	cfg.UnsetSortColumn("never-sorted", "Age")
	cfg.ResetSort("never-sorted")
}

func TestSortHelpers_ResetClearsEntireChain(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SetSort("pod", "Age", SortDirectionDescending)
	cfg.SetSort("pod", "Name", SortDirectionAscending)
	cfg.SetSort("pod", "Restarts", SortDirectionDescending)
	if got := cfg.GetSort("pod"); len(got) != 3 {
		t.Fatalf("setup: chain should have 3 tiers, got %d", len(got))
	}

	cfg.ResetSort("pod")
	if got := cfg.GetSort("pod"); len(got) != 0 {
		t.Errorf("ResetSort must clear chain, got %+v", got)
	}
}

func TestSortHelpers_EntrySharedWithPinned(t *testing.T) {
	// Both Pinned and Sort live in the same ResourceKindConfigEntry.
	// Unsetting one must NOT drop the entry when the other is still
	// active, otherwise the surviving config would silently disappear.
	cfg := DefaultConfig()
	cfg.SetPinned("pod", 10)
	cfg.SetSort("pod", "Name", SortDirectionAscending)

	cfg.ResetSort("pod")
	if !cfg.IsPinned("pod") {
		t.Error("ResetSort must not drop the entry when Pinned is still set")
	}

	cfg.SetSort("pod", "Age", SortDirectionDescending)
	cfg.UnsetPinned("pod")
	if got := cfg.GetSort("pod"); len(got) == 0 {
		t.Error("UnsetPinned must not drop the entry when Sort chain is still set")
	}

	cfg.ResetSort("pod")
	if _, ok := cfg.ResourceKindConfig["pod"]; ok {
		t.Error("entry must be deleted once both Pinned and Sort are unset")
	}
}

func TestSortRoundtrip_SurvivesSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.SetPinned("pod", 10)
	cfg.SetSort("pod", "Age", SortDirectionDescending)
	cfg.SetSort("pod", "Name", SortDirectionAscending) // second tier on pods
	cfg.SetSort("deployment", "Name", SortDirectionAscending)

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	got := loaded.GetSort("pod")
	want := SortChain{
		{Column: "Age", Direction: SortDirectionDescending},
		{Column: "Name", Direction: SortDirectionAscending},
	}
	if len(got) != len(want) {
		t.Fatalf("pod chain lost tiers, got %+v want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pod chain tier %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if d := loaded.GetSort("deployment"); len(d) != 1 || d[0].Column != "Name" || d[0].Direction != SortDirectionAscending {
		t.Errorf("deployment sort lost in roundtrip, got %+v", d)
	}
	if !loaded.IsPinned("pod") {
		t.Error("pod pin lost when round-tripped alongside sort")
	}
}

func TestSortChain_NullYAML_LoadsAsEmpty(t *testing.T) {
	// Defensive: user manually editing YAML to clear a sort entry
	// might land on `sort: null` or `sort:` (empty). Both must
	// degrade to an empty chain, not fail load.
	cases := []struct {
		name string
		body string
	}{
		{
			name: "explicit null",
			body: "resource_kind_config:\n  pod:\n    sort: null\n",
		},
		{
			name: "empty value",
			body: "resource_kind_config:\n  pod:\n    sort:\n",
		},
		{
			name: "empty string",
			body: "resource_kind_config:\n  pod:\n    sort: \"\"\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			cfg, err := LoadFrom(path)
			if err != nil {
				t.Fatalf("LoadFrom must tolerate %s, got err: %v", tc.name, err)
			}
			if got := cfg.GetSort("pod"); len(got) != 0 {
				t.Errorf("expected empty chain for %s, got %+v", tc.name, got)
			}
		})
	}
}

func TestSortChain_LegacyYAMLUnmarshal(t *testing.T) {
	// v1.6 wrote sort as a single mapping. v1.7+ writes a sequence
	// but READS both shapes so existing user configs upgrade with
	// no migration step.
	legacy := []byte(`default_context: ""
default_namespace: ""
editor: ""
compare:
  layout: ""
resource_kind_config:
  pod:
    sort:
      column: Age
      direction: desc
`)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom legacy: %v", err)
	}
	chain := cfg.GetSort("pod")
	if len(chain) != 1 {
		t.Fatalf("legacy single-mapping should lift to 1-tier chain, got %d tiers (%+v)", len(chain), chain)
	}
	if chain[0].Column != "Age" || chain[0].Direction != SortDirectionDescending {
		t.Errorf("legacy tier = %+v, want {Age desc}", chain[0])
	}
}

func TestMouse_DefaultsApply(t *testing.T) {
	// Legacy configs leave Mouse=nil. Both helpers fall back to safe
	// defaults: enabled=true, scroll=natural.
	cfg := DefaultConfig()
	if cfg.Mouse != nil {
		t.Errorf("DefaultConfig must leave Mouse nil, got %v", cfg.Mouse)
	}
	if !cfg.IsMouseEnabled() {
		t.Error("default IsMouseEnabled should be true (nil = enabled)")
	}
	if got := cfg.MouseScrollDirection(); got != MouseScrollNatural {
		t.Errorf("default MouseScrollDirection = %q, want %q", got, MouseScrollNatural)
	}
}

func TestMouse_SetAndRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.SetMouseEnabled(false)
	cfg.SetMouseScrollDirection(MouseScrollReverse)
	if cfg.IsMouseEnabled() {
		t.Error("SetMouseEnabled(false) → IsMouseEnabled should be false")
	}
	if cfg.MouseScrollDirection() != MouseScrollReverse {
		t.Errorf("ScrollDirection mismatch: %q", cfg.MouseScrollDirection())
	}

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if loaded.IsMouseEnabled() {
		t.Error("enabled=false should survive roundtrip")
	}
	if loaded.MouseScrollDirection() != MouseScrollReverse {
		t.Errorf("scroll_direction lost in roundtrip: %q", loaded.MouseScrollDirection())
	}
	if loaded.Mouse == nil || loaded.Mouse.Enabled == nil {
		t.Error("explicit Enabled must persist as non-nil, distinguishing from legacy")
	}
}

func TestMouse_PartialMouseConfig(t *testing.T) {
	// User sets only scroll_direction → enabled stays nil and falls
	// back to true. Mirror: setting only enabled leaves scroll at
	// its default. Ensures the nested struct's fields are
	// independently settable.
	cfg := DefaultConfig()
	cfg.SetMouseScrollDirection(MouseScrollReverse)
	if !cfg.IsMouseEnabled() {
		t.Error("scroll-only set should leave enabled at default true")
	}
	if cfg.Mouse.Enabled != nil {
		t.Error("setting only scroll must not touch Enabled")
	}
}

func TestNextPinOrder_MonotonicSparse(t *testing.T) {
	cfg := DefaultConfig()
	if got := cfg.NextPinOrder(); got != 10 {
		t.Errorf("NextPinOrder on empty config = %d, want 10", got)
	}
	cfg.SetPinned("pod", 10)
	if got := cfg.NextPinOrder(); got != 20 {
		t.Errorf("NextPinOrder after one pin = %d, want 20", got)
	}
	cfg.SetPinned("namespace", 55) // simulate user hand-edit
	if got := cfg.NextPinOrder(); got != 65 {
		t.Errorf("NextPinOrder must be max+10 = 65, got %d", got)
	}
}

// TestBackupBeforeMigration_PreservesContent — v1.7.5 deprecation-key
// migration must save user content verbatim before triggering
// cfg.Save() (which loses comments + unknown fields). Backup file
// should match the source byte-for-byte and live at the documented
// sibling path.
func TestBackupBeforeMigration_PreservesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte("# user-added comment about fish\n" +
		"km8erm_shell: /opt/homebrew/bin/fish\n" +
		"future_field: 42  # km8 doesn't know this yet\n" +
		"editor: vim\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("seeding source: %v", err)
	}

	bak, err := BackupBeforeMigration(path, "1_7_5")
	if err != nil {
		t.Fatalf("BackupBeforeMigration: %v", err)
	}
	wantPath := path + ".old.1_7_5"
	if bak != wantPath {
		t.Errorf("backup path = %q, want %q", bak, wantPath)
	}
	got, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("backup content differs from source:\n--- source ---\n%s\n--- backup ---\n%s", original, got)
	}
}

// TestBackupBeforeMigration_OverwritesExisting — re-running migration
// in the same km8 release overwrites a prior backup of the same name
// so the user has the latest pre-migration snapshot, not a stale one
// from a previous failed attempt.
func TestBackupBeforeMigration_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	bakPath := path + ".old.1_7_5"
	if err := os.WriteFile(bakPath, []byte("stale prior backup\n"), 0o644); err != nil {
		t.Fatalf("seeding stale backup: %v", err)
	}
	fresh := []byte("fresh config to back up\n")
	if err := os.WriteFile(path, fresh, 0o644); err != nil {
		t.Fatalf("seeding source: %v", err)
	}

	if _, err := BackupBeforeMigration(path, "1_7_5"); err != nil {
		t.Fatalf("BackupBeforeMigration: %v", err)
	}
	got, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(got) != string(fresh) {
		t.Errorf("backup must reflect FRESH source, got stale:\n%s", got)
	}
}

func TestSaveTo_Atomic_NoTempfileLeak(t *testing.T) {
	// Save should not leave .config.*.yaml stragglers around after a
	// successful write — they would confuse a user listing the config
	// dir, and indicate the rename failed.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := &Config{Editor: "vim"}
	cfg.SetPinned("pod", 10)
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "config.yaml" {
			t.Errorf("unexpected leftover file in config dir: %q", e.Name())
		}
	}
}
