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
