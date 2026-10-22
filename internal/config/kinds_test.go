package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The pins and sorts are their own file, kinds.yaml in the data directory
// (tdp E3: data), read back as they were written; none yet is no error.
func TestKindDataRoundTrip(t *testing.T) {
	t.Setenv("KBU__DATA", t.TempDir())
	if got, want := KindDataPath(), filepath.Join(DataDir(), "kinds.yaml"); got != want {
		t.Errorf("KindDataPath() = %q, want %q", got, want)
	}
	if k, err := LoadKindData(); err != nil || k != nil {
		t.Fatalf("no kinds.yaml: %v, %v", k, err)
	}
	var k KindData
	k.SetPinned("pod", 10)
	k.SetSort("pod", "Age", SortDirectionDescending)
	k.SetSort("deployment", "Name", SortDirectionAscending)
	if err := k.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := LoadKindData()
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsPinned("pod") || !slices.Equal(got.GetSort("pod"), k.GetSort("pod")) || !slices.Equal(got.GetSort("deployment"), k.GetSort("deployment")) {
		t.Errorf("read back %v, wrote %v", got, k)
	}
	if err := os.WriteFile(KindDataPath(), []byte("pod: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKindData(); err == nil {
		t.Error("a broken kinds.yaml read without an error")
	}
}

const oldConfig = `# my settings
editor: vim # for kubectl edit
resource_kind_config:
  pod:
    pinned:
      order: 10
    sort:
      - column: Age
        direction: desc
compare:
  layout: split
`

func writeOld(t *testing.T) (cfgPath, to string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(oldConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath, filepath.Join(dir, "data", "kinds.yaml")
}

// tdp E3, a new place: the pins and sorts an older kbu kept in config.yaml
// move to kinds.yaml; the rest of config.yaml stays, its comments too.
func TestMoveKindDataMovesThemOut(t *testing.T) {
	cfgPath, to := writeOld(t)
	moved := moveKindData(cfgPath, to)
	if len(moved) != 1 || moved[0].Err != nil || moved[0].From != cfgPath || moved[0].To != to {
		t.Fatalf("moved %+v", moved)
	}
	k, err := LoadKindDataFrom(to)
	if err != nil || !k.IsPinned("pod") || len(k.GetSort("pod")) != 1 {
		t.Fatalf("kinds.yaml: %v, %v", k, err)
	}
	b, _ := os.ReadFile(cfgPath)
	if s := string(b); strings.Contains(s, "resource_kind_config") || !strings.Contains(s, "# my settings") || !strings.Contains(s, "# for kubectl edit") || !strings.Contains(s, "layout: split") {
		t.Errorf("config.yaml after the move:\n%s", s)
	}
	cfg, err := LoadFrom(cfgPath)
	if err != nil || cfg.Editor != "vim" || cfg.ResourceKindConfig != nil {
		t.Errorf("config.yaml reads %+v, %v", cfg, err)
	}
	if again := moveKindData(cfgPath, to); again != nil {
		t.Errorf("moved twice: %+v", again)
	}
}

// What kinds.yaml has already, the move leaves in config.yaml, untouched,
// for the user to merge.
func TestMoveKindDataLeavesThemWhenThere(t *testing.T) {
	cfgPath, to := writeOld(t)
	if err := (KindData{}).SaveTo(to); err != nil {
		t.Fatal(err)
	}
	moved := moveKindData(cfgPath, to)
	if len(moved) != 1 || !errors.Is(moved[0].Err, ErrThereAlready) {
		t.Fatalf("moved %+v", moved)
	}
	if b, _ := os.ReadFile(cfgPath); string(b) != oldConfig {
		t.Errorf("config.yaml changed:\n%s", b)
	}
}

// When they cannot be moved, they stay in config.yaml only, and kbu keeps
// using them there.
func TestMoveKindDataThatCannotMove(t *testing.T) {
	cfgPath, _ := writeOld(t)
	file := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KBU__DATA", file) // a file: nothing can be made in it
	moved := moveKindData(cfgPath, KindDataPath())
	if len(moved) != 1 || moved[0].Err == nil || errors.Is(moved[0].Err, ErrThereAlready) {
		t.Fatalf("moved %+v", moved)
	}
	if b, _ := os.ReadFile(cfgPath); string(b) != oldConfig {
		t.Errorf("config.yaml changed:\n%s", b)
	}
	cfg, err := LoadFrom(cfgPath)
	if err != nil || !KindDataInConfig(cfg) {
		t.Errorf("kbu would not keep using config.yaml: %v", err)
	}
}

// Written to kinds.yaml but not taken out of config.yaml (its directory
// will not take a new file): kinds.yaml goes again, they stay in config.yaml.
func TestMoveKindDataUndoneWhenConfigStays(t *testing.T) {
	cfgPath, to := writeOld(t)
	if err := os.Chmod(filepath.Dir(cfgPath), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(cfgPath), 0o755) })
	to = filepath.Join(t.TempDir(), "kinds.yaml")
	moved := moveKindData(cfgPath, to)
	if len(moved) != 1 || moved[0].Err == nil {
		t.Fatalf("moved %+v", moved)
	}
	if _, err := os.Stat(to); err == nil {
		t.Error("kinds.yaml was left beside the pins and sorts still in config.yaml")
	}
	if b, _ := os.ReadFile(cfgPath); string(b) != oldConfig {
		t.Errorf("config.yaml changed:\n%s", b)
	}
}

// A config.yaml with no pins and sorts, or none at all, has nothing to move.
func TestMoveKindDataWithNothingToMove(t *testing.T) {
	dir := t.TempDir()
	cfgPath, to := filepath.Join(dir, "config.yaml"), filepath.Join(dir, "kinds.yaml")
	if moved := moveKindData(cfgPath, to); moved != nil {
		t.Errorf("no config.yaml: %+v", moved)
	}
	if err := os.WriteFile(cfgPath, []byte("editor: vim\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if moved := moveKindData(cfgPath, to); moved != nil {
		t.Errorf("no pins and sorts: %+v", moved)
	}
	if _, err := os.Stat(to); err == nil {
		t.Error("an empty kinds.yaml was made")
	}
}
