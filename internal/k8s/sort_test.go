package k8s

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// columns mirror the Pod column set so the title-based dispatch is
// exercised end-to-end (sort engine looks up colIdx by title).
var podColumns = []Column{
	{Title: "Name"},
	{Title: "Ready"},
	{Title: "Status"},
	{Title: "Restarts"},
	{Title: "Age"},
}

func podItem(name, ready, status, restarts string, age time.Duration) ResourceItem {
	created := time.Now().Add(-age)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(created),
		},
	}
	return ResourceItem{
		Name: name,
		Row:  []string{name, ready, status, restarts, ageCellFor(age)},
		Raw:  pod,
	}
}

// ageCellFor builds a realistic-shaped Age cell. The test cares about
// CreationTimestamp on Raw, not this string, but having a non-empty
// Row keeps fixtures shaped like production.
func ageCellFor(d time.Duration) string {
	if d < time.Minute {
		return "0s"
	}
	if d < time.Hour {
		return "Xm"
	}
	if d < 24*time.Hour {
		return "Xh"
	}
	return "Xd"
}

func names(items []ResourceItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func TestSortItems_StringFallback_Name(t *testing.T) {
	// No special comparator for Name — falls through to lexicographic
	// compare on Row[0]. Sanity check that asc/desc both work.
	items := []ResourceItem{
		podItem("charlie", "1/1", "Running", "0", time.Hour),
		podItem("alpha", "1/1", "Running", "0", time.Hour),
		podItem("bravo", "1/1", "Running", "0", time.Hour),
	}
	SortItems(items, podColumns, "Name", true)
	got := names(items)
	want := []string{"alpha", "bravo", "charlie"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("asc Name[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
	SortItems(items, podColumns, "Name", false)
	if names(items)[0] != "charlie" {
		t.Errorf("desc Name first = %q, want charlie", names(items)[0])
	}
}

func TestSortItems_IntComparator_RestartsBeats10vs2(t *testing.T) {
	// The exact case string sort gets wrong: "10" < "2" lexicographic
	// but "2" < "10" numerically. Restarts column must use the int
	// comparator.
	items := []ResourceItem{
		podItem("a", "1/1", "Running", "10", time.Hour),
		podItem("b", "1/1", "Running", "2", time.Hour),
		podItem("c", "1/1", "Running", "0", time.Hour),
	}
	SortItems(items, podColumns, "Restarts", true)
	want := []string{"c", "b", "a"} // 0, 2, 10
	got := names(items)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("asc Restarts[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestSortItems_ReadyComparator_BeatsStringForN_M(t *testing.T) {
	// "10/10" beats "2/2" by string sort but should rank above
	// numerically.
	items := []ResourceItem{
		podItem("a", "10/10", "Running", "0", time.Hour),
		podItem("b", "2/2", "Running", "0", time.Hour),
		podItem("c", "0/3", "Running", "0", time.Hour),
		podItem("d", "1/3", "Running", "0", time.Hour),
	}
	SortItems(items, podColumns, "Ready", true)
	want := []string{"c", "d", "b", "a"} // (0,3) (1,3) (2,2) (10,10)
	got := names(items)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("asc Ready[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestSortItems_AgeComparator_UsesCreationTime(t *testing.T) {
	// Age column relies on the underlying CreationTimestamp, not the
	// rendered "Xd" / "Xh" string. asc → youngest (most-recent
	// creation) first by virtue of comparing time.Time values.
	items := []ResourceItem{
		podItem("oldest", "1/1", "Running", "0", 30*24*time.Hour),
		podItem("middle", "1/1", "Running", "0", 5*24*time.Hour),
		podItem("newest", "1/1", "Running", "0", time.Hour),
	}
	SortItems(items, podColumns, "Age", true)
	want := []string{"oldest", "middle", "newest"} // oldest creation timestamp first
	got := names(items)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("asc Age[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
	SortItems(items, podColumns, "Age", false)
	if names(items)[0] != "newest" {
		t.Errorf("desc Age first = %q, want newest", names(items)[0])
	}
}

func TestSortItems_UnknownColumnIsNoOp(t *testing.T) {
	// Stale config referencing a column the kind no longer surfaces
	// (renamed / removed) must not panic or reorder — caller sees the
	// items in their original order and the UI silently degrades to
	// "no active sort".
	items := []ResourceItem{
		podItem("c", "1/1", "Running", "0", time.Hour),
		podItem("a", "1/1", "Running", "0", time.Hour),
		podItem("b", "1/1", "Running", "0", time.Hour),
	}
	original := names(items)
	SortItems(items, podColumns, "DoesNotExist", true)
	got := names(items)
	for i := range original {
		if got[i] != original[i] {
			t.Errorf("unknown column must be no-op: items[%d] = %q, want %q", i, got[i], original[i])
		}
	}
}

func TestSortItems_EmptyOrSingleItem_NoOp(t *testing.T) {
	// Defensive: zero / one item slices skip the sort entirely so the
	// stable-sort invariant has nothing to violate.
	SortItems(nil, podColumns, "Name", true)
	single := []ResourceItem{podItem("a", "1/1", "Running", "0", time.Hour)}
	SortItems(single, podColumns, "Name", true)
	if names(single)[0] != "a" {
		t.Errorf("single-item slice corrupted: %v", names(single))
	}
}

func TestSortItems_HelmRevColumn_IntComparator(t *testing.T) {
	// Helm "Rev" column carries a revision number — string sort
	// puts "10" between "1" and "2". The "Rev" → int comparator
	// fixes the order.
	cols := []Column{
		{Title: "Name"}, {Title: "Namespace"}, {Title: "Chart"},
		{Title: "App Ver"}, {Title: "Rev"}, {Title: "Status"}, {Title: "Updated"},
	}
	items := []ResourceItem{
		{Name: "a", Row: []string{"a", "default", "ch", "1.0", "10", "deployed", "x"}},
		{Name: "b", Row: []string{"b", "default", "ch", "1.0", "2", "deployed", "x"}},
		{Name: "c", Row: []string{"c", "default", "ch", "1.0", "1", "deployed", "x"}},
	}
	SortItems(items, cols, "Rev", true)
	want := []string{"c", "b", "a"}
	for i, w := range want {
		if items[i].Name != w {
			t.Errorf("Rev asc[%d] = %q, want %q (full=%v)", i, items[i].Name, w, names(items))
		}
	}
}

func TestSortItems_HelmUpdatedColumn_UsesRawTime(t *testing.T) {
	// "Updated" column reads the time off Raw (*Release.Updated)
	// rather than the rendered age string. The age string ("5d" /
	// "10d") would lex-sort wrong ("10d" < "5d"); the time-parse
	// path keeps order honest.
	cols := []Column{{Title: "Name"}, {Title: "Updated"}}
	items := []ResourceItem{
		{Name: "new", Row: []string{"new", "5d"}, Raw: &Release{Updated: "2026-06-20 10:00:00.000000000 +0000 UTC"}},
		{Name: "old", Row: []string{"old", "10d"}, Raw: &Release{Updated: "2026-06-15 10:00:00.000000000 +0000 UTC"}},
		{Name: "mid", Row: []string{"mid", "7d"}, Raw: &Release{Updated: "2026-06-17 10:00:00.000000000 +0000 UTC"}},
	}
	SortItems(items, cols, "Updated", true)
	want := []string{"old", "mid", "new"}
	for i, w := range want {
		if items[i].Name != w {
			t.Errorf("Updated asc[%d] = %q, want %q (oldest → newest)", i, items[i].Name, w)
		}
	}
}

func TestSortItems_StableOnTies(t *testing.T) {
	// Three pods all with Restarts=5 must preserve the incoming order
	// under SliceStable so a follow-up sort on a different column
	// acts as an effective secondary key.
	items := []ResourceItem{
		podItem("first", "1/1", "Running", "5", time.Hour),
		podItem("second", "1/1", "Running", "5", time.Hour),
		podItem("third", "1/1", "Running", "5", time.Hour),
	}
	SortItems(items, podColumns, "Restarts", true)
	want := []string{"first", "second", "third"}
	got := names(items)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("stable sort broke order at %d: %v", i, got)
		}
	}
}
