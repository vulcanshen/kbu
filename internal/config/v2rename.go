package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// v2.0 km8 → kbu rename transition helpers.
//
// Two concerns handled here:
//  1. Config directory migration — a v1.7.x user relaunches on v2.0 and
//     finds ~/.config/kbu/ empty. MigrateLegacyConfigDir copies the
//     contents of ~/.config/km8/ into it (one-shot; subsequent launches
//     skip because ~/.config/kbu/ now exists).
//  2. Env var deprecation — $KBU__* is the new name for $KM8__*.
//     EnvDeprecations returns any warnings for legacy env vars still set
//     while the new name isn't. Fires per-launch until the user updates
//     their shell rc / launchctl plist.
//
// Removable when the transition tier is retired (planned v2.1).

// MigrateLegacyConfigDir copies the pre-v2.0 config directory
// (~/.config/km8/) into the current one (~/.config/kbu/) when the
// current directory doesn't exist but the legacy one does. Called
// once at startup, before Load / LoadState.
//
// Returns (warning, nil) after a successful copy — caller logs the
// warning so the user knows migration happened and can clean up the
// old dir at their leisure. Returns ("", nil) when no migration is
// needed (fresh install, or already migrated). Returns ("", err)
// when the copy fails; caller decides whether to continue with an
// empty config dir or bail.
//
// Copy semantics: recursive file copy, no symlinks or special files.
// The pre-v2.0 config dir only contains YAML files and log
// subdirectories, so this is safe.
func MigrateLegacyConfigDir() (string, error) {
	newDir := ConfigDir()
	oldDir := legacyConfigDir()

	// Guard 1: new dir already exists → migration already done or
	// user created it manually. Skip either way (never overwrite).
	if _, err := os.Stat(newDir); err == nil {
		return "", nil
	}

	// Guard 2: old dir doesn't exist → fresh install, nothing to
	// migrate. Skip silently.
	if _, err := os.Stat(oldDir); err != nil {
		return "", nil
	}

	// Guard 3: same path (edge case — someone rebuilt with a legacy
	// appName). No-op copy.
	if newDir == oldDir {
		return "", nil
	}

	if err := copyDirTree(oldDir, newDir); err != nil {
		return "", fmt.Errorf("migrating config dir %s → %s: %w", oldDir, newDir, err)
	}

	return fmt.Sprintf(
		"config: migrated %s → %s (v2.0 km8 → kbu rename). The old directory is left in place — delete it once you're satisfied the new setup works.",
		oldDir, newDir,
	), nil
}

// EnvDeprecations returns warnings for any legacy $KM8__* env vars
// still set while their $KBU__* replacement isn't. One warning per
// deprecated var. Empty slice = user has already migrated (or never
// used the legacy names).
//
// Fires per-launch on purpose: env vars live in shell rc / launchctl
// plists / systemd units that kbu can't rewrite. A persistent nudge
// is the right pressure until the user updates their environment.
func EnvDeprecations() []string {
	var out []string

	if os.Getenv("KBU__CONFIGPATH") == "" && strings.TrimSpace(os.Getenv("KM8__CONFIGPATH")) != "" {
		out = append(out, "env: $KM8__CONFIGPATH is deprecated, rename to $KBU__CONFIGPATH (the old name is still read this release; remove next release)")
	}
	if os.Getenv("KBU__STATEPATH") == "" && strings.TrimSpace(os.Getenv("KM8__STATEPATH")) != "" {
		out = append(out, "env: $KM8__STATEPATH is deprecated, rename to $KBU__STATEPATH (the old name is still read this release; remove next release)")
	}
	if os.Getenv("KBU__ALTERM_SHELL") == "" && strings.TrimSpace(os.Getenv("KM8__ALTERM_SHELL")) != "" {
		out = append(out, "env: $KM8__ALTERM_SHELL is deprecated, rename to $KBU__ALTERM_SHELL (the old name is still read this release; remove next release)")
	}
	if os.Getenv("KBU__ALTERM_LOGIN_SHELL") == "" && strings.TrimSpace(os.Getenv("KM8__ALTERM_LOGIN_SHELL")) != "" {
		out = append(out, "env: $KM8__ALTERM_LOGIN_SHELL is deprecated, rename to $KBU__ALTERM_LOGIN_SHELL (the old name is still read this release; remove next release)")
	}

	return out
}

// copyDirTree recursively copies src into dst. dst is created if
// missing. Files use 0o644, dirs 0o755 — matches the perms the config
// package uses when it creates its own files (Save / MkdirAll).
// Fails on the first error.
func copyDirTree(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDirTree(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
