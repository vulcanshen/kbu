package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	in := &State{
		Context:         "orbstack",
		Namespace:       "default",
		Namespaces:      []string{"default", "monitoring"},
		Kind:            "pods",
		ObjectNamespace: "kube-system",
		ObjectName:      "coredns-abc123",
		Panel:           "detail",
		Tab:             "Events",
	}
	if err := in.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	out, err := LoadStateFrom(path)
	if err != nil {
		t.Fatalf("LoadStateFrom: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round trip mismatch: got %+v, want %+v", *out, *in)
	}
}

func TestLoadStateFrom_MissingFileReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yaml")

	s, err := LoadStateFrom(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil default state")
	}
	if !reflect.DeepEqual(*s, State{}) {
		t.Errorf("expected empty state, got %+v", *s)
	}
}

func TestLoadStateFrom_MalformedIsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")
	if err := os.WriteFile(path, []byte("kind: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadStateFrom(path)
	if err == nil {
		t.Fatal("expected parse error on malformed yaml")
	}
}

func TestState_SaveToCreatesMissingDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "dir", "state.yaml")

	s := &State{Kind: "deployments"}
	if err := s.SaveTo(nested); err != nil {
		t.Fatalf("SaveTo nested: %v", err)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("state file missing after save: %v", err)
	}
}

func TestState_OmitEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	s := &State{} // all fields empty
	if err := s.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// An entirely empty state should serialize to "{}\n" (yaml empty mapping).
	// The omitempty tags mean no keys emit — confirms the file stays tidy
	// after a save from a fresh session that touched nothing.
	if got := string(data); got != "{}\n" {
		t.Errorf("expected empty state to marshal to %q, got %q", "{}\n", got)
	}
}

// tdp D6: $KBU__STATE is the state directory; state.yaml goes in it,
// trimmed of surrounding whitespace.
func TestStatePath_KBUStateIsTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kbu-state")
	for _, v := range []string{dir, "  " + dir + "  "} {
		t.Setenv("KBU__STATE", v)
		if got, want := StatePath(), filepath.Join(dir, "state.yaml"); got != want {
			t.Errorf("KBU__STATE=%q: StatePath() = %q, want %q", v, got, want)
		}
	}
}

// Unset, the state follows the config directory — $KBU__CONFIG included.
func TestStatePath_FollowsTheConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kbu-cfg")
	t.Setenv("KBU__STATE", "")
	t.Setenv("KBU__CONFIG", dir)
	if got, want := StatePath(), filepath.Join(dir, "state.yaml"); got != want {
		t.Errorf("StatePath() = %q, want %q", got, want)
	}
}

// tdp D6: a rename keeps no old name. $KBU__STATEPATH (a file, v2.x) and
// the pre-v2.0 $KM8__STATEPATH are not read.
func TestStatePath_OldNamesAreNotRead(t *testing.T) {
	t.Setenv("KBU__STATE", "")
	t.Setenv("KBU__CONFIG", "")
	t.Setenv("KBU__STATEPATH", "/tmp/old-kbu-state.yaml")
	t.Setenv("KM8__STATEPATH", "/tmp/legacy-km8-state.yaml")
	if got, want := StatePath(), filepath.Join(ConfigDir(), "state.yaml"); got != want {
		t.Errorf("an old name was read: StatePath() = %q, want %q", got, want)
	}
}
