package k8s

import (
	"sort"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SortTier is one rung in a multi-column sort. Column is matched
// against the kind's registered Column.Title (same dispatch as
// single-column SortItems). Ascending toggles direction within the
// tier only — each tier is independent.
type SortTier struct {
	Column    string
	Ascending bool
}

// SortItems sorts items in place by a single column. Thin wrapper
// over SortItemsChain — kept for callers that haven't migrated yet
// and for the test surface where single-column is the natural case.
func SortItems(items []ResourceItem, columns []Column, columnTitle string, ascending bool) {
	SortItemsChain(items, columns, []SortTier{{Column: columnTitle, Ascending: ascending}})
}

// SortItemsChain sorts items in place by the ordered tier list.
// Tier 0 is the primary sort, tier 1 the first tiebreaker, and so
// on. A tier whose Column doesn't match any of the kind's columns
// is silently skipped (defensive against stale config). Empty
// chain, or a chain where every tier is unknown / non-discriminating,
// degrades to the stable sort's natural fallback (incoming order).
//
// Comparator dispatch is per-tier — Age uses the time comparator,
// Ready parses N/M, etc. — so mixing typed tiers (e.g. Restarts
// desc + Age asc) works without per-call configuration.
func SortItemsChain(items []ResourceItem, columns []Column, tiers []SortTier) {
	if len(items) < 2 || len(tiers) == 0 {
		return
	}
	type compiled struct {
		less      func(a, b ResourceItem) int
		ascending bool
	}
	chain := make([]compiled, 0, len(tiers))
	for _, t := range tiers {
		if t.Column == "" {
			continue
		}
		colIdx := -1
		for i, c := range columns {
			if c.Title == t.Column {
				colIdx = i
				break
			}
		}
		if colIdx < 0 {
			continue
		}
		chain = append(chain, compiled{
			less:      comparatorForColumn(t.Column, colIdx),
			ascending: t.Ascending,
		})
	}
	if len(chain) == 0 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		for _, c := range chain {
			cmp := c.less(items[i], items[j])
			if !c.ascending {
				cmp = -cmp
			}
			if cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})
}

// comparatorForColumn picks the right typed comparator for well-known
// column titles, falling back to lexicographic compare on the
// pre-rendered Row[colIdx] string for everything else.
//
// Well-known titles + their comparators:
//   - "Age"                              → CreationTimestamp (time)
//   - "Updated"                          → Helm release Updated (time)
//   - "Ready"                            → "N/M" parsed as ints
//   - "Restarts" / "Desired" / "Current" → Row cell parsed as int
//     "Up-to-date" / "Available" / "Active" / "Rev"
//
// String fallback covers everything else (Name, Status, Roles,
// Version, Type, Reason, Object, Message, IP, Node, Schedule, ...).
// Lex compare reads correctly on these because the values are either
// truly alphabetic or already left-padded by the renderer.
func comparatorForColumn(title string, colIdx int) func(a, b ResourceItem) int {
	switch title {
	case "Age":
		return lessByCreationTime
	case "Updated":
		return lessByHelmUpdatedTime
	case "Ready":
		return lessByReadyCell(colIdx)
	case "Restarts", "Desired", "Current", "Up-to-date", "Available", "Active", "Rev":
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

// lessByHelmUpdatedTime is the comparator for the Helm Releases
// "Updated" column. Raw carries a *Release whose Updated field is
// the helm CLI's Go time.String() form ("2024-05-19 14:31:22.123
// +0800 CST"); the displayed Row cell is an age string ("5d" /
// "2h") that lex-sorts wrong ("10d" < "2d"). Parsing back from Raw
// dodges that entirely.
//
// Non-helm items, parse failures, and missing data all fall back to
// the zero time. The stable sort keeps their incoming order.
func lessByHelmUpdatedTime(a, b ResourceItem) int {
	ta := helmUpdatedTimeOf(a)
	tb := helmUpdatedTimeOf(b)
	switch {
	case ta.Before(tb):
		return -1
	case ta.After(tb):
		return 1
	}
	return 0
}

func helmUpdatedTimeOf(item ResourceItem) time.Time {
	r, ok := item.Raw.(*Release)
	if !ok || r.Updated == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", r.Updated)
	if err != nil {
		return time.Time{}
	}
	return t
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
