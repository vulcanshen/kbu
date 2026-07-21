package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	in := &State{
		Context:         "orbstack",
		Namespace:       "default",
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
	if *out != *in {
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
	if *s != (State{}) {
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

func TestStatePath_EnvOverride(t *testing.T) {
	t.Setenv("KM8__STATEPATH", "")
	t.Setenv("KBU__STATEPATH", "/tmp/kbu-test-state.yaml")
	if got := StatePath(); got != "/tmp/kbu-test-state.yaml" {
		t.Errorf("StatePath env override: got %q, want %q", got, "/tmp/kbu-test-state.yaml")
	}
}

func TestStatePath_LeadingWhitespaceIsTrimmed(t *testing.T) {
	t.Setenv("KM8__STATEPATH", "")
	t.Setenv("KBU__STATEPATH", "  /tmp/kbu-test-state.yaml  ")
	if got := StatePath(); got != "/tmp/kbu-test-state.yaml" {
		t.Errorf("StatePath trim: got %q", got)
	}
}

func TestStatePath_LegacyKM8EnvFallback(t *testing.T) {
	// v2.0 rename transition: $KM8__STATEPATH honored when $KBU__STATEPATH
	// is not set. Silent fallback; deprecation surfaced via EnvDeprecations.
	t.Setenv("KBU__STATEPATH", "")
	t.Setenv("KM8__STATEPATH", "/tmp/legacy-km8-state.yaml")
	if got := StatePath(); got != "/tmp/legacy-km8-state.yaml" {
		t.Errorf("legacy KM8 env fallback: got %q, want %q", got, "/tmp/legacy-km8-state.yaml")
	}
}

func TestStatePath_KBUWinsOverKM8(t *testing.T) {
	t.Setenv("KM8__STATEPATH", "/tmp/legacy-km8-state.yaml")
	t.Setenv("KBU__STATEPATH", "/tmp/new-kbu-state.yaml")
	if got := StatePath(); got != "/tmp/new-kbu-state.yaml" {
		t.Errorf("KBU should win over KM8: got %q, want %q", got, "/tmp/new-kbu-state.yaml")
	}
}
