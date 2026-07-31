package k8s

import (
	"reflect"
	"testing"
)

func TestNamespaceSelection_ZeroValueAndAllAreAll(t *testing.T) {
	var zero NamespaceSelection
	if !zero.IsAll() {
		t.Error("zero-value NamespaceSelection must read as All")
	}
	if !AllNamespaces().IsAll() {
		t.Error("AllNamespaces() must be All")
	}
	if AllNamespaces().Count() != 0 {
		t.Errorf("All Count must be 0, got %d", AllNamespaces().Count())
	}
	if AllNamespaces().List() != nil {
		t.Error("All List() must be nil")
	}
}

func TestSelectedNamespaces(t *testing.T) {
	s := SelectedNamespaces("prod", "dev")
	if s.IsAll() {
		t.Error("explicit selection must not be All")
	}
	if s.Count() != 2 {
		t.Errorf("Count = %d, want 2", s.Count())
	}
	if !s.Contains("prod") || !s.Contains("dev") {
		t.Error("must contain both selected namespaces")
	}
	if s.Contains("staging") {
		t.Error("must not contain an unselected namespace")
	}

	// Blank names filtered; all-blank collapses to All.
	if !SelectedNamespaces("", "").IsAll() {
		t.Error("all-blank input must collapse to All")
	}
	if SelectedNamespaces("a", "").Count() != 1 {
		t.Error("blank names must be filtered out")
	}
	// Duplicates deduped.
	if SelectedNamespaces("a", "a", "a").Count() != 1 {
		t.Error("duplicate namespaces must dedupe")
	}
	// No args → All.
	if !SelectedNamespaces().IsAll() {
		t.Error("SelectedNamespaces() with no args must be All")
	}
}

func TestNamespaceSelection_ListSorted(t *testing.T) {
	got := SelectedNamespaces("prod", "alpha", "dev").List()
	want := []string{"alpha", "dev", "prod"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want sorted %v", got, want)
	}
}

func TestNamespaceSelection_ToggleFromAllClearsAll(t *testing.T) {
	// [4]: checking a specific namespace from the All state switches to a
	// single-namespace selection (All becomes unchecked).
	s := AllNamespaces().Toggle("prod")
	if s.IsAll() {
		t.Error("toggling a specific ns from All must clear All")
	}
	if !s.Contains("prod") || s.Count() != 1 {
		t.Errorf("expected {prod}, got Count=%d contains(prod)=%v", s.Count(), s.Contains("prod"))
	}
}

func TestNamespaceSelection_ToggleOffLastFallsBackToAll(t *testing.T) {
	// Unchecking the last remaining namespace returns to All (no empty state).
	s := SelectedNamespaces("prod").Toggle("prod")
	if !s.IsAll() {
		t.Error("unchecking the last namespace must fall back to All")
	}
}

func TestNamespaceSelection_ToggleAddsAndRemoves(t *testing.T) {
	s := SelectedNamespaces("prod")
	s = s.Toggle("dev") // add
	if s.Count() != 2 || !s.Contains("dev") {
		t.Errorf("Toggle add: Count=%d contains(dev)=%v", s.Count(), s.Contains("dev"))
	}
	s = s.Toggle("prod") // remove one of two — stays specific
	if s.IsAll() || s.Count() != 1 || !s.Contains("dev") || s.Contains("prod") {
		t.Errorf("Toggle remove: expected {dev}, got IsAll=%v Count=%d", s.IsAll(), s.Count())
	}
}

func TestNamespaceSelection_ToggleBlankNoop(t *testing.T) {
	s := SelectedNamespaces("prod")
	if !s.Toggle("").Equal(s) {
		t.Error("toggling a blank namespace must be a no-op")
	}
}

func TestNamespaceSelection_Equal(t *testing.T) {
	cases := []struct {
		a, b NamespaceSelection
		want bool
	}{
		{AllNamespaces(), AllNamespaces(), true},
		{AllNamespaces(), NamespaceSelection{}, true}, // zero value == All
		{AllNamespaces(), SelectedNamespaces("prod"), false},
		{SelectedNamespaces("prod", "dev"), SelectedNamespaces("dev", "prod"), true}, // order-independent
		{SelectedNamespaces("prod"), SelectedNamespaces("prod", "dev"), false},
		{SelectedNamespaces("prod"), SelectedNamespaces("dev"), false},
	}
	for i, c := range cases {
		if got := c.a.Equal(c.b); got != c.want {
			t.Errorf("case %d: Equal = %v, want %v", i, got, c.want)
		}
	}
}
