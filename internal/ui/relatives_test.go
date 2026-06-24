package ui

import (
	"testing"

	"github.com/vulcanshen/km8/internal/k8s"
)

func TestEntryAtLine_SingleLineEntries(t *testing.T) {
	entries := []relativeEntry{
		{section: true, label: "Owner"},
		{label: "rs/foo", ref: &k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "foo"}},
		{section: true, label: "Volumes"},
		{label: "cm/bar", ref: &k8s.RefTarget{Type: k8s.ResourceConfigMaps, Name: "bar"}},
	}
	cases := []struct {
		line, want int
	}{
		{0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, -1}, {-1, -1},
	}
	for _, c := range cases {
		got := entryAtLine(entries, c.line)
		if got != c.want {
			t.Errorf("entryAtLine(line=%d) = %d, want %d", c.line, got, c.want)
		}
	}
}

func TestEntryAtLine_NestedDrillableTakesTwoLines(t *testing.T) {
	// Nested drillable entry (label has "  " indent AND ref non-nil)
	// renders across 2 lines — clicking either line must resolve to
	// the same entry. Anything past the entry span is -1.
	entries := []relativeEntry{
		{section: true, label: "Volumes"},
		{label: "  alias", ref: &k8s.RefTarget{Type: k8s.ResourceConfigMaps, Name: "shared"}},
		{label: "footer"},
	}
	if got := entryAtLine(entries, 0); got != 0 {
		t.Errorf("line 0 (section) = %d, want 0", got)
	}
	if got := entryAtLine(entries, 1); got != 1 {
		t.Errorf("line 1 (first half of nested entry) = %d, want 1", got)
	}
	if got := entryAtLine(entries, 2); got != 1 {
		t.Errorf("line 2 (second half of nested entry) = %d, want 1", got)
	}
	if got := entryAtLine(entries, 3); got != 2 {
		t.Errorf("line 3 (footer) = %d, want 2", got)
	}
	if got := entryAtLine(entries, 99); got != -1 {
		t.Errorf("line past end = %d, want -1", got)
	}
}

func TestEntryAtLine_EmptyEntries(t *testing.T) {
	if got := entryAtLine(nil, 0); got != -1 {
		t.Errorf("empty entries = %d, want -1", got)
	}
}
