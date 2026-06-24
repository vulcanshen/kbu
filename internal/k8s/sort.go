package k8s

import (
	"sort"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SortItems sorts items in place by the column whose Title matches
// columnTitle. ascending=true → small/early first; false → reversed.
// Unknown column or empty title → no-op (caller stays defensive
// against stale config referencing a column the kind no longer
// surfaces).
//
// The comparator is picked by column Title (not index) so config
// written for "Age" keeps working even if the kind reorders its
// columns between versions. Title-based dispatch also lets the same
// comparator serve every kind that has e.g. a "Ready" column without
// each kind hand-wiring its own SortLess.
//
// Stable sort: rows whose keys tie keep their incoming relative
// order, which is the natural fallback when the user picks a sort
// column that doesn't fully disambiguate.
func SortItems(items []ResourceItem, columns []Column, columnTitle string, ascending bool) {
	if columnTitle == "" || len(items) < 2 {
		return
	}
	colIdx := -1
	for i, c := range columns {
		if c.Title == columnTitle {
			colIdx = i
			break
		}
	}
	if colIdx < 0 {
		return
	}
	less := comparatorForColumn(columnTitle, colIdx)
	sort.SliceStable(items, func(i, j int) bool {
		cmp := less(items[i], items[j])
		if !ascending {
			cmp = -cmp
		}
		return cmp < 0
	})
}

// comparatorForColumn picks the right typed comparator for well-known
// column titles, falling back to lexicographic compare on the
// pre-rendered Row[colIdx] string for everything else.
//
// Well-known titles + their comparators:
//   - "Age"                              → CreationTimestamp (time)
//   - "Ready"                            → "N/M" parsed as ints
//   - "Restarts" / "Desired" / "Current" → Row cell parsed as int
//     "Up-to-date" / "Available" / "Active"
//
// String fallback covers everything else (Name, Status, Roles,
// Version, Type, Reason, Object, Message, IP, Node, Schedule, ...).
// Lex compare reads correctly on these because the values are either
// truly alphabetic or already left-padded by the renderer.
func comparatorForColumn(title string, colIdx int) func(a, b ResourceItem) int {
	switch title {
	case "Age":
		return lessByCreationTime
	case "Ready":
		return lessByReadyCell(colIdx)
	case "Restarts", "Desired", "Current", "Up-to-date", "Available", "Active":
		return lessByIntCell(colIdx)
	}
	return lessByStringCell(colIdx)
}

// lessByCreationTime is the Age comparator: smaller (older) first when
// ascending. Reads metav1.Object out of item.Raw so the underlying
// CreationTimestamp survives renderer formatting tricks like "5d3h"
// vs "10d" that break a lexicographic compare on the Row string.
//
// Helm Releases don't carry a metav1.Object on Raw — they fall back
// to the zero time, which means all releases sort as "tied" on Age
// and the stable sort keeps their incoming order. Better than mis-
// sorting; if Helm Release age sort ever becomes important, this
// branch grows a type assertion for the Helm-specific carrier.
func lessByCreationTime(a, b ResourceItem) int {
	ta := creationTimeOf(a)
	tb := creationTimeOf(b)
	switch {
	case ta.Before(tb):
		return -1
	case ta.After(tb):
		return 1
	}
	return 0
}

func creationTimeOf(item ResourceItem) time.Time {
	if obj, ok := item.Raw.(metav1.Object); ok {
		return obj.GetCreationTimestamp().Time
	}
	return time.Time{}
}

// lessByIntCell parses Row[colIdx] as an int. Used for plain counter
// columns (Restarts, Desired, Current, Up-to-date, Available,
// Active). Parse failure → 0, so non-numeric stragglers sort
// together at the low end; the alternative (string fallback) would
// mix numeric and non-numeric rows in an unpredictable order.
func lessByIntCell(colIdx int) func(a, b ResourceItem) int {
	return func(a, b ResourceItem) int {
		ai := parseIntCell(a, colIdx)
		bi := parseIntCell(b, colIdx)
		switch {
		case ai < bi:
			return -1
		case ai > bi:
			return 1
		}
		return 0
	}
}

func parseIntCell(item ResourceItem, idx int) int {
	n, _ := strconv.Atoi(strings.TrimSpace(rowCell(item, idx)))
	return n
}

// lessByReadyCell parses the "ready/total" form (e.g. "1/1", "3/5")
// kubectl uses for Ready columns. Sort key is (ready, total) so
// "0/3" sorts below "1/3" sorts below "2/3" sorts below "3/3", and
// ties on ready break by total ascending. Parse failure on either
// side → both treated as (0,0), letting stable sort keep order.
func lessByReadyCell(colIdx int) func(a, b ResourceItem) int {
	return func(a, b ResourceItem) int {
		ar, at := parseReadyCell(rowCell(a, colIdx))
		br, bt := parseReadyCell(rowCell(b, colIdx))
		switch {
		case ar < br:
			return -1
		case ar > br:
			return 1
		case at < bt:
			return -1
		case at > bt:
			return 1
		}
		return 0
	}
}

func parseReadyCell(s string) (int, int) {
	parts := strings.SplitN(strings.TrimSpace(s), "/", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	r, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	t, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return r, t
}

// lessByStringCell is the default fallback: lexicographic compare of
// the pre-rendered Row[colIdx]. Works for Name / Namespace / Status /
// most string columns where the displayed form is what the user
// wants to sort by.
func lessByStringCell(colIdx int) func(a, b ResourceItem) int {
	return func(a, b ResourceItem) int {
		return strings.Compare(rowCell(a, colIdx), rowCell(b, colIdx))
	}
}

func rowCell(item ResourceItem, idx int) string {
	if idx < 0 || idx >= len(item.Row) {
		return ""
	}
	return item.Row[idx]
}
