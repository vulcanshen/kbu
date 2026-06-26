package ui

import (
	"strings"
	"testing"

	"github.com/vulcanshen/km8/internal/theme"
)

// TestAlignSplitDiff_ContextPairsUseRightSide pins the v1.7.2 fix:
// previously context (unchanged) lines paired leftLines[i] with itself
// on BOTH sides, so the right column rendered the left content verbatim
// — most visible when comparing two ConfigMaps with the same keys but
// different values (the changed values appeared on the LEFT side of the
// right column instead of the actual right side).
func TestAlignSplitDiff_ContextPairsUseRightSide(t *testing.T) {
	left := []string{"a", "b", "c"}
	right := []string{"a", "b", "c"}
	pairs := alignSplitDiff(left, right)

	if len(pairs) != 3 {
		t.Fatalf("expected 3 pairs, got %d", len(pairs))
	}
	for i, p := range pairs {
		if p.left != left[i] {
			t.Errorf("pair %d: left = %q, want %q", i, p.left, left[i])
		}
		if p.right != right[i] {
			t.Errorf("pair %d: right = %q, want %q (right side must NOT mirror left)", i, p.right, right[i])
		}
		if p.changed() {
			t.Errorf("pair %d: changed = true, want false", i)
		}
	}
}

// TestAlignSplitDiff_NearTwinConfigMaps mirrors the actual user-reported
// regression: two ConfigMaps with the same keys but a few different
// values. The earlier byte-level alignment splintered the changed lines
// into character fragments ("+ CC", "+ f", "+ r", ...). After the fix
// changed lines must pair full-line vs full-line.
func TestAlignSplitDiff_NearTwinConfigMaps(t *testing.T) {
	leftYAML := strings.Join([]string{
		"apiVersion: v1",
		"data:",
		"  accent: '#0066FF'",
		"  feature_flag: enabled",
		"  log_level: info",
		"  theme: ocean",
		"kind: ConfigMap",
	}, "\n")
	rightYAML := strings.Join([]string{
		"apiVersion: v1",
		"data:",
		"  accent: '#00CC66'",
		"  feature_flag: enabled",
		"  log_level: info",
		"  theme: forest",
		"kind: ConfigMap",
	}, "\n")

	leftLines := strings.Split(leftYAML, "\n")
	rightLines := strings.Split(rightYAML, "\n")
	pairs := alignSplitDiff(leftLines, rightLines)

	// Expect 7 pairs (one per line, since both sides have the same line count).
	if got := len(pairs); got != 7 {
		t.Fatalf("expected 7 pairs (full-line alignment), got %d", got)
	}

	// The two changed lines (accent, theme) must surface as `changed`
	// pairs with the FULL left value vs FULL right value — not fragments.
	wantChanged := map[int]struct{ left, right string }{
		2: {"  accent: '#0066FF'", "  accent: '#00CC66'"},
		5: {"  theme: ocean", "  theme: forest"},
	}
	wantContext := map[int]string{
		0: "apiVersion: v1",
		1: "data:",
		3: "  feature_flag: enabled",
		4: "  log_level: info",
		6: "kind: ConfigMap",
	}

	for i, p := range pairs {
		if w, ok := wantChanged[i]; ok {
			if !p.changed() {
				t.Errorf("pair %d (%q vs %q): expected changed=true, got false", i, p.left, p.right)
			}
			if p.left != w.left {
				t.Errorf("pair %d: left = %q, want %q (full-line, not fragment)", i, p.left, w.left)
			}
			if p.right != w.right {
				t.Errorf("pair %d: right = %q, want %q (full-line, not fragment)", i, p.right, w.right)
			}
		} else {
			ctx := wantContext[i]
			if p.changed() {
				t.Errorf("pair %d (%q): expected context (changed=false), got changed", i, p.left)
			}
			if p.left != ctx {
				t.Errorf("pair %d: left = %q, want %q", i, p.left, ctx)
			}
			if p.right != ctx {
				t.Errorf("pair %d: right = %q, want %q", i, p.right, ctx)
			}
		}
	}
}

// TestAlignSplitDiff_PureInsertion verifies that right-only lines emit
// pairs with empty left + the right content (rendered as `+ x` by the
// split renderer).
func TestAlignSplitDiff_PureInsertion(t *testing.T) {
	left := []string{"a", "c"}
	right := []string{"a", "b", "c"}
	pairs := alignSplitDiff(left, right)

	// Expect: (a, a, ctx), (insert b), (c, c, ctx).
	if got := len(pairs); got != 3 {
		t.Fatalf("expected 3 pairs, got %d", got)
	}
	if pairs[1].left != "" || pairs[1].right != "b" || pairs[1].changed() {
		t.Errorf("expected insertion pair {left:\"\", right:\"b\", changed:false}, got %+v", pairs[1])
	}
}

// TestAlignSplitDiff_BlankLineInsertionNotSwallowed pins the v1.7.2 fix
// for the kind-tag refactor: a splitPair whose inserted/deleted line is
// itself the empty string previously fell through to the renderer's
// `default` (context) branch because the disambiguator used `p.left/
// right == ""` predicates. With splitPairKind in place, a blank
// insertion surfaces as splitPairInsert and renders as `+` on the right
// — visible to the user instead of a silent miss.
func TestAlignSplitDiff_BlankLineInsertionNotSwallowed(t *testing.T) {
	left := []string{"a", "b"}
	right := []string{"a", "", "b"}
	pairs := alignSplitDiff(left, right)

	if got := len(pairs); got != 3 {
		t.Fatalf("expected 3 pairs, got %d", got)
	}
	// pairs[1] is the blank-line insert. Must be tagged splitPairInsert,
	// not the default context kind.
	if pairs[1].kind != splitPairInsert {
		t.Errorf("blank-line insert: kind = %d, want splitPairInsert (%d)", pairs[1].kind, splitPairInsert)
	}
	if pairs[1].right != "" {
		t.Errorf("blank-line insert: right = %q, want \"\"", pairs[1].right)
	}
}

// TestAlignSplitDiff_BlankLineDeletionNotSwallowed mirrors the insert
// case for left-only blank.
func TestAlignSplitDiff_BlankLineDeletionNotSwallowed(t *testing.T) {
	left := []string{"a", "", "b"}
	right := []string{"a", "b"}
	pairs := alignSplitDiff(left, right)

	if got := len(pairs); got != 3 {
		t.Fatalf("expected 3 pairs, got %d", got)
	}
	if pairs[1].kind != splitPairDelete {
		t.Errorf("blank-line delete: kind = %d, want splitPairDelete (%d)", pairs[1].kind, splitPairDelete)
	}
	if pairs[1].left != "" {
		t.Errorf("blank-line delete: left = %q, want \"\"", pairs[1].left)
	}
}

// TestAlignSplitDiff_PureDeletion mirrors PureInsertion for left-only.
func TestAlignSplitDiff_PureDeletion(t *testing.T) {
	left := []string{"a", "b", "c"}
	right := []string{"a", "c"}
	pairs := alignSplitDiff(left, right)

	if got := len(pairs); got != 3 {
		t.Fatalf("expected 3 pairs, got %d", got)
	}
	if pairs[1].left != "b" || pairs[1].right != "" || pairs[1].changed() {
		t.Errorf("expected deletion pair {left:\"b\", right:\"\", changed:false}, got %+v", pairs[1])
	}
}

// TestRenderSplitDiff_TruncatesOversizedInput pins the OOM guard: inputs
// past splitDiffLineLimit per side must be capped before the line-LCS
// DP table allocation, and a warning banner must surface to the user.
func TestRenderSplitDiff_TruncatesOversizedInput(t *testing.T) {
	// Build inputs well past the cap (splitDiffLineLimit + N) — at the
	// cap the DP table allocation would be 4M ints. Above it, without
	// truncation, the TUI would OOM on a real cluster's last-applied-
	// configuration annotation.
	makeYAML := func(n int, value string) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteString("key")
			b.WriteString(value)
			b.WriteByte('\n')
		}
		return b.String()
	}
	left := makeYAML(splitDiffLineLimit+500, "L")
	right := makeYAML(splitDiffLineLimit+500, "R")
	th := theme.DefaultTheme()

	out := renderSplitDiff(left, right, "blue", "green", 80, th)

	if len(out) == 0 {
		t.Fatal("expected non-empty diff output")
	}
	// Banner must surface — search every row because lipgloss styling
	// may prepend ANSI escape sequences.
	bannerFound := false
	for _, row := range out {
		if strings.Contains(row, "truncated") {
			bannerFound = true
			break
		}
	}
	if !bannerFound {
		t.Errorf("expected truncation banner row, got %d rows without it", len(out))
	}

	// Body rows must NOT exceed splitDiffLineLimit lines (header +
	// separator + banner + at most splitDiffLineLimit body rows).
	if len(out) > splitDiffLineLimit+10 {
		t.Errorf("expected ≤ %d body rows after cap, got %d total rows",
			splitDiffLineLimit, len(out))
	}
}

// TestRenderUnifiedDiff_TruncatesOversizedInput pins the v1.7.2-post-review
// fix: cap was only on the split path; toggling to unified against the
// same uncapped YAML used to keep allocating udiff's full DP. Now both
// paths share capDiffInput + truncationBanner.
func TestRenderUnifiedDiff_TruncatesOversizedInput(t *testing.T) {
	makeYAML := func(n int, value string) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteString("key")
			b.WriteString(value)
			b.WriteByte('\n')
		}
		return b.String()
	}
	left := makeYAML(splitDiffLineLimit+500, "L")
	right := makeYAML(splitDiffLineLimit+500, "R")
	th := theme.DefaultTheme()

	out := renderUnifiedDiff(left, right, "blue", "green", 80, th)
	if len(out) == 0 {
		t.Fatal("expected non-empty unified diff output")
	}
	bannerFound := false
	for _, row := range out {
		if strings.Contains(row, "truncated") {
			bannerFound = true
			break
		}
	}
	if !bannerFound {
		t.Errorf("unified diff missing truncation banner; rows=%d", len(out))
	}
}

// TestRenderSplitDiff_NoBannerWhenUnderCap guards against false-positive
// truncation banners for normal-sized inputs.
func TestRenderSplitDiff_NoBannerWhenUnderCap(t *testing.T) {
	left := "a\nb\nc"
	right := "a\nB\nc"
	th := theme.DefaultTheme()
	out := renderSplitDiff(left, right, "blue", "green", 80, th)
	for _, row := range out {
		if strings.Contains(row, "truncated") {
			t.Errorf("did not expect truncation banner for small input, got row: %q", row)
		}
	}
}
