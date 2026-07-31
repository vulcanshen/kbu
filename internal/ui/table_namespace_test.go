package ui

import (
	"testing"

	"github.com/vulcanshen/kbu/internal/k8s"
)

func TestShouldInjectNamespace(t *testing.T) {
	cases := []struct {
		rt   k8s.ResourceType
		want bool
	}{
		{k8s.ResourcePods, true},        // namespaced, no own Namespace column
		{k8s.ResourceConfigMaps, false}, // ships its own Namespace column
		{k8s.ResourceNodes, false},      // cluster-scoped
		{k8s.ResourceNamespaces, false}, // cluster-scoped
	}
	for _, c := range cases {
		if got := shouldInjectNamespace(c.rt); got != c.want {
			t.Errorf("shouldInjectNamespace(%s) = %v, want %v", c.rt, got, c.want)
		}
	}
}

func countNamespaceColumns(cols []Column) int {
	n := 0
	for _, c := range cols {
		if c.Title == "Namespace" {
			n++
		}
	}
	return n
}

func TestColumnsForResource_NamespaceColumn(t *testing.T) {
	if got := countNamespaceColumns(ColumnsForResource(k8s.ResourcePods)); got != 1 {
		t.Errorf("Pods must gain exactly one Namespace column, got %d", got)
	}
	if got := countNamespaceColumns(ColumnsForResource(k8s.ResourceNodes)); got != 0 {
		t.Errorf("cluster-scoped Nodes must have no Namespace column, got %d", got)
	}
	// ConfigMaps ships its own — must stay at exactly one (no double-inject).
	if got := countNamespaceColumns(ColumnsForResource(k8s.ResourceConfigMaps)); got != 1 {
		t.Errorf("ConfigMaps must keep exactly one Namespace column, got %d", got)
	}
}

// The injected Namespace cell must line up with the injected Namespace
// column — columns and cells share the shouldInjectNamespace predicate, so
// a row built by augmentRowsWithHelm has one cell per display column.
func TestAugmentRows_NamespaceCellAligned(t *testing.T) {
	cols := k8s.DefaultRegistry.ColumnsFor(k8s.ResourcePods)
	row := make([]string, len(cols))
	for i := range row {
		row[i] = "x"
	}
	row[0] = "p1"
	item := k8s.ResourceItem{Name: "p1", Namespace: "prod", Row: row}

	rows := augmentRowsWithHelm([]k8s.ResourceItem{item}, k8s.ResourcePods)
	display := ColumnsForResource(k8s.ResourcePods)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if len(rows[0]) != len(display) {
		t.Errorf("row cells (%d) must equal display columns (%d) — misalignment", len(rows[0]), len(display))
	}
	// Layout is Name, helm marker, Namespace, ...
	if display[2].Title != "Namespace" {
		t.Fatalf("expected Namespace column at index 2, got %q", display[2].Title)
	}
	if rows[0][2] != "prod" {
		t.Errorf("injected namespace cell = %q, want 'prod'", rows[0][2])
	}
}
