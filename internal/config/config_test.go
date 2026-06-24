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
	if got := cfg.GetSort("pod"); got != nil {
		t.Errorf("fresh config GetSort = %+v, want nil", got)
	}

	cfg.SetSort("pod", "Age", SortDirectionDescending)
	got := cfg.GetSort("pod")
	if got == nil {
		t.Fatal("GetSort returned nil after SetSort")
	}
	if got.Column != "Age" || got.Direction != SortDirectionDescending {
		t.Errorf("GetSort = %+v, want {Column:Age Direction:desc}", got)
	}

	// SetSort overwrites the previous sort entry — single-column model.
	cfg.SetSort("pod", "Name", SortDirectionAscending)
	got = cfg.GetSort("pod")
	if got == nil || got.Column != "Name" || got.Direction != SortDirectionAscending {
		t.Errorf("SetSort overwrite = %+v, want {Column:Name Direction:asc}", got)
	}

	// UnsetSort on a kind with no other active config drops the entry.
	cfg.UnsetSort("pod")
	if got := cfg.GetSort("pod"); got != nil {
		t.Errorf("GetSort after UnsetSort = %+v, want nil", got)
	}
	if _, ok := cfg.ResourceKindConfig["pod"]; ok {
		t.Error("empty entry must be deleted after UnsetSort with no other config")
	}

	// UnsetSort on never-sorted kind is a no-op.
	cfg.UnsetSort("never-sorted")
}

func TestSortHelpers_EntrySharedWithPinned(t *testing.T) {
	// Both Pinned and Sort live in the same ResourceKindConfigEntry.
	// Unsetting one must NOT drop the entry when the other is still
	// active, otherwise the surviving config would silently disappear.
	cfg := DefaultConfig()
	cfg.SetPinned("pod", 10)
	cfg.SetSort("pod", "Name", SortDirectionAscending)

	cfg.UnsetSort("pod")
	if !cfg.IsPinned("pod") {
		t.Error("UnsetSort must not drop the entry when Pinned is still set")
	}

	cfg.SetSort("pod", "Age", SortDirectionDescending)
	cfg.UnsetPinned("pod")
	if got := cfg.GetSort("pod"); got == nil {
		t.Error("UnsetPinned must not drop the entry when Sort is still set")
	}

	// Now drop both — entry should disappear from the map.
	cfg.UnsetSort("pod")
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
	cfg.SetSort("deployment", "Name", SortDirectionAscending)

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if got := loaded.GetSort("pod"); got == nil || got.Column != "Age" || got.Direction != SortDirectionDescending {
		t.Errorf("pod sort lost in roundtrip, got %+v", got)
	}
	if got := loaded.GetSort("deployment"); got == nil || got.Column != "Name" || got.Direction != SortDirectionAscending {
		t.Errorf("deployment sort lost in roundtrip, got %+v", got)
	}
	if !loaded.IsPinned("pod") {
		t.Error("pod pin lost when round-tripped alongside sort")
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
