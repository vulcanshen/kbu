package k8s

import (
	"testing"
	"time"
)

// recvUpdate reads one WatchMsg, failing on error or timeout.
func recvUpdate(t *testing.T, w *Watcher) []ResourceItem {
	t.Helper()
	updates, errs := w.Channels()
	select {
	case msg := <-updates:
		return msg.Items
	case e := <-errs:
		t.Fatalf("unexpected watch error: %v", e.Err)
		return nil
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch update")
		return nil
	}
}

func TestWatcher_MultiNamespaceIncrementalList(t *testing.T) {
	// alpha×2, beta×1, gamma×1
	w := NewWatcher(fakeClientsetWithPods())
	defer w.Stop()

	w.Start(ResourcePods, SelectedNamespaces("alpha", "beta"))

	// First emit: alpha only (incremental step 1, sorted ns order [5]).
	if got := namespacesOf(recvUpdate(t, w)); len(got) != 2 || got[0] != "alpha" || got[1] != "alpha" {
		t.Fatalf("first incremental emit = %v, want [alpha alpha]", got)
	}

	// Second emit: alpha + beta accumulated (gamma excluded).
	got := namespacesOf(recvUpdate(t, w))
	want := []string{"alpha", "alpha", "beta"}
	if len(got) != len(want) {
		t.Fatalf("second emit = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("second emit = %v, want %v", got, want)
		}
	}
}

func TestWatcher_AllNamespacesSingleEmit(t *testing.T) {
	w := NewWatcher(fakeClientsetWithPods())
	defer w.Stop()

	w.Start(ResourcePods, AllNamespaces())

	// All is served by one cluster-wide list — every pod in one emit.
	if got := recvUpdate(t, w); len(got) != 4 {
		t.Errorf("All emit = %d items, want 4 (ns: %v)", len(got), namespacesOf(got))
	}
}
