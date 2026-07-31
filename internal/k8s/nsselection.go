package k8s

import "sort"

// NamespaceSelection describes which namespaces the UI is currently
// showing. It is either "all namespaces" (served by a single
// cluster-wide list) or an explicit set of namespace names. The two are
// mutually exclusive — the Toggle helper maintains that invariant so the
// picker never has to (see popup requirement [4]: checking a specific
// namespace unchecks All, and vice versa).
//
// There is deliberately no "show nothing" state: deselecting the last
// remaining namespace falls back to All. kbu always shows something.
type NamespaceSelection struct {
	all bool
	set map[string]struct{}
}

// AllNamespaces returns the cluster-wide selection kbu boots into. This
// is the zero-value-equivalent: an empty NamespaceSelection{} also reads
// as All (see IsAll), so callers can rely on the zero value being safe.
func AllNamespaces() NamespaceSelection {
	return NamespaceSelection{all: true}
}

// SelectedNamespaces builds a selection over an explicit set of namespace
// names. Blank names are ignored; if nothing valid remains it collapses
// to AllNamespaces.
func SelectedNamespaces(names ...string) NamespaceSelection {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		if n != "" {
			set[n] = struct{}{}
		}
	}
	if len(set) == 0 {
		return AllNamespaces()
	}
	return NamespaceSelection{set: set}
}

// IsAll reports whether the selection spans every namespace.
func (s NamespaceSelection) IsAll() bool {
	return s.all || len(s.set) == 0
}

// Count returns the number of explicitly selected namespaces, or 0 when
// the selection is All.
func (s NamespaceSelection) Count() int {
	if s.IsAll() {
		return 0
	}
	return len(s.set)
}

// Contains reports whether ns is in the explicit set. Always false when
// the selection is All (All is not "every name checked" — see [4]).
func (s NamespaceSelection) Contains(ns string) bool {
	if s.IsAll() {
		return false
	}
	_, ok := s.set[ns]
	return ok
}

// List returns the selected namespaces sorted by name. This is the fetch
// order used by the multi-namespace list path (requirement [5]: fetch
// per namespace, ordered by name). Returns nil when the selection is All.
func (s NamespaceSelection) List() []string {
	if s.IsAll() {
		return nil
	}
	out := make([]string, 0, len(s.set))
	for n := range s.set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Toggle returns a new selection with ns flipped on/off. Checking a
// specific namespace clears the All state; unchecking the last remaining
// namespace falls back to All. Blank ns is a no-op.
func (s NamespaceSelection) Toggle(ns string) NamespaceSelection {
	if ns == "" {
		return s
	}
	next := make(map[string]struct{}, len(s.set)+1)
	if !s.IsAll() {
		for k := range s.set {
			next[k] = struct{}{}
		}
	}
	if _, ok := next[ns]; ok {
		delete(next, ns)
	} else {
		next[ns] = struct{}{}
	}
	if len(next) == 0 {
		return AllNamespaces()
	}
	return NamespaceSelection{set: next}
}

// Equal reports whether two selections cover the same namespaces. Used to
// skip redundant re-fetches when a toggle results in no net change.
func (s NamespaceSelection) Equal(other NamespaceSelection) bool {
	if s.IsAll() || other.IsAll() {
		return s.IsAll() && other.IsAll()
	}
	if len(s.set) != len(other.set) {
		return false
	}
	for k := range s.set {
		if _, ok := other.set[k]; !ok {
			return false
		}
	}
	return true
}
