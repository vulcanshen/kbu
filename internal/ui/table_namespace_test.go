package ui

import (
	"testing"

	"github.com/vulcanshen/kbu/internal/k8s"
)

func TestIsNamespacedResource(t *testing.T) {
	cases := []struct {
		rt   k8s.ResourceType
		want bool
	}{
		{k8s.ResourcePods, true},
		{k8s.ResourceConfigMaps, true},  // namespaced even though it ships its own ns column
		{k8s.ResourceNodes, false},      // cluster-scoped
		{k8s.ResourceNamespaces, false}, // cluster-scoped
	}
	for _, c := range cases {
		if got := isNamespacedResource(c.rt); got != c.want {
			t.Errorf("isNamespacedResource(%s) = %v, want %v", c.rt, got, c.want)
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

func TestColumnsForResource_NamespaceLeadsForNamespaced(t *testing.T) {
	// Injected (Pods): exactly one Namespace column, and it leads.
	pods := ColumnsForResource(k8s.ResourcePods)
	if countNamespaceColumns(pods) != 1 {
		t.Errorf("Pods must have exactly one Namespace column, got %d", countNamespaceColumns(pods))
	}
	if pods[0].Title != "Namespace" {
		t.Errorf("Pods first column must be Namespace, got %q", pods[0].Title)
	}

	// Legacy own-column (ConfigMaps): hoisted to the front, not doubled.
	cm := ColumnsForResource(k8s.ResourceConfigMaps)
	if countNamespaceColumns(cm) != 1 {
		t.Errorf("ConfigMaps must keep exactly one Namespace column, got %d", countNamespaceColumns(cm))
	}
	if cm[0].Title != "Namespace" {
		t.Errorf("ConfigMaps first column must be Namespace, got %q", cm[0].Title)
	}

	// Cluster-scoped (Nodes): no Namespace column, Name still leads.
	nodes := ColumnsForResource(k8s.ResourceNodes)
	if countNamespaceColumns(nodes) != 0 {
		t.Errorf("cluster-scoped Nodes must have no Namespace column, got %d", countNamespaceColumns(nodes))
	}
	if nodes[0].Title != "Name" {
		t.Errorf("Nodes first column must be Name, got %q", nodes[0].Title)
	}
}

// Both injected and legacy-own-column resources align — one row cell per
// display column, with the item namespace leading and Name right after it.
func TestAugmentRows_NamespaceLeadsAndAligns(t *testing.T) {
	check := func(rt k8s.ResourceType) {
		t.Helper()
		cols := k8s.DefaultRegistry.ColumnsFor(rt)
		row := make([]string, len(cols))
		for i := range row {
			row[i] = "x"
		}
		row[0] = "the-name"
		item := k8s.ResourceItem{Name: "the-name", Namespace: "prod", Row: row}

		rows := augmentRowsWithHelm([]k8s.ResourceItem{item}, rt)
		display := ColumnsForResource(rt)
		if len(rows) != 1 {
			t.Fatalf("%s: expected 1 row, got %d", rt, len(rows))
		}
		if len(rows[0]) != len(display) {
			t.Errorf("%s: row cells (%d) must equal display columns (%d)", rt, len(rows[0]), len(display))
		}
		if display[0].Title != "Namespace" {
			t.Fatalf("%s: first display column must be Namespace, got %q", rt, display[0].Title)
		}
		if rows[0][0] != "prod" {
			t.Errorf("%s: first cell must be the namespace 'prod', got %q", rt, rows[0][0])
		}
		if rows[0][1] != "the-name" {
			t.Errorf("%s: second cell must be the name, got %q", rt, rows[0][1])
		}
	}
	check(k8s.ResourcePods)       // injected
	check(k8s.ResourceConfigMaps) // legacy own-column, hoisted to front
}
