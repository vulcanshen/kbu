package tui

import (
	"fmt"
	"strings"

	udiff "github.com/aymanbagabas/go-udiff"
	terminu "github.com/vulcanshen/terminu-go"
)

// The compare popup's two layouts: a unified diff (- and + lines) and the
// two sides next to each other. Each side is at most diffLineLimit lines:
// the side-by-side alignment is quadratic in the lines.

const diffLineLimit = 2000

// capLines keeps at most diffLineLimit lines of each side.
func capLines(left, right string) (l, r []string, cutL, cutR bool) {
	split := func(s string) []string {
		if s == "" {
			return nil
		}
		return strings.Split(strings.TrimRight(s, "\n"), "\n")
	}
	l, r = split(left), split(right)
	cutL, cutR = len(l) > diffLineLimit, len(r) > diffLineLimit
	if cutL {
		l = l[:diffLineLimit]
	}
	if cutR {
		r = r[:diffLineLimit]
	}
	return l, r, cutL, cutR
}

// cutNote says a side was cut, in the warning colour; "" when none was.
func cutNote(cutL, cutR bool) string {
	which := ""
	switch {
	case cutL && cutR:
		which = "both sides"
	case cutL:
		which = "the left side"
	case cutR:
		which = "the right side"
	default:
		return ""
	}
	return terminu.Fg(terminu.Peach).Render(fmt.Sprintf("%s cut to the first %d lines; [Y] on the row shows all of it", which, diffLineLimit))
}

const identical = "(identical — no config diff)"

// unifiedDiff is the diff with - and + lines.
func unifiedDiff(left, right, ll, rl string, w int) []string {
	l, r, cutL, cutR := capLines(left, right)
	var out []string
	if n := cutNote(cutL, cutR); n != "" {
		out = append(out, n)
	}
	diff := udiff.Unified(ll, rl, strings.Join(l, "\n"), strings.Join(r, "\n"))
	if diff == "" {
		return append(out, terminu.Fg(terminu.Overlay0).Render(identical))
	}
	for _, line := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			out = append(out, terminu.Fg(terminu.Overlay0).Render(line))
		case strings.HasPrefix(line, "@@"):
			out = append(out, terminu.Fg(terminu.Blue).Render(line))
		case strings.HasPrefix(line, "+"):
			out = append(out, terminu.Fg(terminu.Green).Render(line))
		case strings.HasPrefix(line, "-"):
			out = append(out, terminu.Fg(terminu.Red).Render(line))
		default:
			out = append(out, line)
		}
	}
	return out
}

// splitDiff puts the two sides next to each other, line by line: a changed
// line in red on the left and green on the right, a line only one side has
// against an empty one.
func splitDiff(left, right, ll, rl string, w int) []string {
	col := max((w-3)/2, 8)
	l, r, cutL, cutR := capLines(left, right)
	fit := func(s string) string { return terminu.Fit(s, col) }
	sep := terminu.Fg(terminu.Overlay0)
	head := terminu.Fg(terminu.Blue).Bolded()
	out := []string{head.Render(fit(ll)) + sep.Render(" │ ") + head.Render(fit(rl)),
		sep.Render(terminu.Repeat("─", col) + "─┼─" + terminu.Repeat("─", col))}
	if n := cutNote(cutL, cutR); n != "" {
		out = append(out, n)
	}
	pairs := alignDiff(l, r)
	if len(pairs) == 0 || allSame(pairs) {
		return append(out, terminu.Fg(terminu.Overlay0).Render(identical))
	}
	add, del := terminu.Fg(terminu.Green), terminu.Fg(terminu.Red)
	for _, p := range pairs {
		var ls, rs string
		switch p.kind {
		case pairInsert:
			ls, rs = fit(""), add.Render(fit("+ "+p.right))
		case pairDelete:
			ls, rs = del.Render(fit("- "+p.left)), fit("")
		case pairChanged:
			ls, rs = del.Render(fit("- "+p.left)), add.Render(fit("+ "+p.right))
		default:
			ls, rs = fit("  "+p.left), fit("  "+p.right)
		}
		out = append(out, ls+sep.Render(" │ ")+rs)
	}
	return out
}

type pairKind byte

const (
	pairSame pairKind = iota
	pairChanged
	pairInsert
	pairDelete
)

type linePair struct {
	left, right string
	kind        pairKind
}

func allSame(ps []linePair) bool {
	for _, p := range ps {
		if p.kind != pairSame {
			return false
		}
	}
	return true
}

// alignDiff lines the two sides up by their longest common subsequence;
// a run of deletions next to a run of insertions pairs up as changed lines.
func alignDiff(l, r []string) []linePair {
	n, m := len(l), len(r)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			switch {
			case l[i-1] == r[j-1]:
				lcs[i][j] = lcs[i-1][j-1] + 1
			case lcs[i-1][j] >= lcs[i][j-1]:
				lcs[i][j] = lcs[i-1][j]
			default:
				lcs[i][j] = lcs[i][j-1]
			}
		}
	}
	type op struct {
		kind        pairKind // pairSame, pairDelete, pairInsert
		left, right string
	}
	var ops []op
	i, j := n, m
	for i > 0 && j > 0 {
		switch {
		case l[i-1] == r[j-1]:
			ops = append(ops, op{pairSame, l[i-1], r[j-1]})
			i, j = i-1, j-1
		case lcs[i-1][j] >= lcs[i][j-1]:
			ops = append(ops, op{kind: pairDelete, left: l[i-1]})
			i--
		default:
			ops = append(ops, op{kind: pairInsert, right: r[j-1]})
			j--
		}
	}
	for ; i > 0; i-- {
		ops = append(ops, op{kind: pairDelete, left: l[i-1]})
	}
	for ; j > 0; j-- {
		ops = append(ops, op{kind: pairInsert, right: r[j-1]})
	}
	for a, b := 0, len(ops)-1; a < b; a, b = a+1, b-1 {
		ops[a], ops[b] = ops[b], ops[a]
	}
	var out []linePair
	for k := 0; k < len(ops); {
		if ops[k].kind == pairSame {
			out = append(out, linePair{ops[k].left, ops[k].right, pairSame})
			k++
			continue
		}
		var dels, ins []string
		for ; k < len(ops) && ops[k].kind != pairSame; k++ {
			if ops[k].kind == pairDelete {
				dels = append(dels, ops[k].left)
			} else {
				ins = append(ins, ops[k].right)
			}
		}
		both := min(len(dels), len(ins))
		for p := 0; p < both; p++ {
			out = append(out, linePair{dels[p], ins[p], pairChanged})
		}
		for _, d := range dels[both:] {
			out = append(out, linePair{left: d, kind: pairDelete})
		}
		for _, s := range ins[both:] {
			out = append(out, linePair{right: s, kind: pairInsert})
		}
	}
	return out
}
