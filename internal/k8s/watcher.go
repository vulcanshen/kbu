package k8s

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// WatchMsg is sent when the resource list has been updated.
type WatchMsg struct {
	Items []ResourceItem
}

// WatchErrMsg is sent when the watcher encounters an error.
type WatchErrMsg struct {
	Err error
}

// Watcher manages Watch connection(s) for a single resource type and
// maintains a local cache of items. It integrates with Bubble Tea through
// a channel-based message pattern.
//
// A watcher tracks a NamespaceSelection: "all namespaces" is served by a
// single cluster-wide list + watch (the efficient path); an explicit set
// of namespaces is served by one list + watch per namespace. In the
// multi-namespace case the initial list emits incrementally — one
// namespace at a time in name order — so the table fills in as each
// namespace resolves (requirement [5]).
type Watcher struct {
	clientset kubernetes.Interface
	mu        sync.RWMutex
	items     []ResourceItem
	cancel    context.CancelFunc
	updates   chan WatchMsg
	errors    chan WatchErrMsg
}

// NewWatcher creates a new Watcher for the given clientset.
func NewWatcher(clientset kubernetes.Interface) *Watcher {
	return &Watcher{
		clientset: clientset,
		updates:   make(chan WatchMsg, 1),
		errors:    make(chan WatchErrMsg, 1),
	}
}

// Start cancels any existing watch and starts watching the given resource
// type across the namespace selection. It performs an initial List, then
// starts Watch(es) for incremental updates. Updates are sent to the
// internal channel — use Channels()/waitForWatchUpdate to receive them.
func (w *Watcher) Start(rt ResourceType, sel NamespaceSelection) {
	w.Stop()

	// Close old channels to unblock any stale waiters, then create fresh
	// channels for the new watcher cycle.
	close(w.updates)
	close(w.errors)
	w.updates = make(chan WatchMsg, 1)
	w.errors = make(chan WatchErrMsg, 1)

	ctx, cancel := context.WithCancel(context.Background())
	w.mu.Lock()
	w.cancel = cancel
	w.items = nil
	w.mu.Unlock()

	go w.run(ctx, rt, sel)
}

// Stop cancels the current watch.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
}

// GetItems returns the current cached items.
func (w *Watcher) GetItems() []ResourceItem {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]ResourceItem, len(w.items))
	copy(result, w.items)
	return result
}

// GetItem returns a single item by index, or nil if out of range.
func (w *Watcher) GetItem(index int) *ResourceItem {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if index < 0 || index >= len(w.items) {
		return nil
	}
	item := w.items[index]
	return &item
}

// Updates returns the channel for receiving watch updates.
func (w *Watcher) Updates() <-chan WatchMsg {
	return w.updates
}

// Errors returns the channel for receiving watch errors.
func (w *Watcher) Errors() <-chan WatchErrMsg {
	return w.errors
}

// Channels returns both channels atomically, preventing a TOCTOU race
// where Start() replaces one channel between two separate
// Updates()/Errors() calls.
func (w *Watcher) Channels() (<-chan WatchMsg, <-chan WatchErrMsg) {
	return w.updates, w.errors
}

func (w *Watcher) store(items []ResourceItem) {
	w.mu.Lock()
	w.items = items
	w.mu.Unlock()
}

// emit pushes items to the updates channel, returning false if the
// context was cancelled before the send could complete.
func (w *Watcher) emit(ctx context.Context, items []ResourceItem) bool {
	select {
	case w.updates <- WatchMsg{Items: items}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (w *Watcher) emitErr(ctx context.Context, err error) {
	select {
	case w.errors <- WatchErrMsg{Err: err}:
	case <-ctx.Done():
	}
}

func (w *Watcher) run(ctx context.Context, rt ResourceType, sel NamespaceSelection) {
	first := true
	for {
		if first {
			// Initial list — emit incrementally, one namespace at a time
			// in name order ([5]).
			if err := w.listAndEmit(ctx, rt, sel); err != nil {
				if ctx.Err() != nil {
					return // cancelled intentionally (user switched resource / namespace)
				}
				w.emitErr(ctx, fmt.Errorf("listing %s: %w", rt, err))
				return
			}
			first = false
		} else {
			// Reconnect re-list — single emit (avoid the incremental
			// shrink-then-grow flicker on every watch reconnect).
			items, err := FetchResourcesMulti(ctx, w.clientset, rt, sel)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				w.emitErr(ctx, fmt.Errorf("listing %s: %w", rt, err))
				return
			}
			w.store(items)
			if !w.emit(ctx, items) {
				return
			}
		}

		events, reconnect, stop, err := w.startWatches(ctx, rt, sel)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.emitErr(ctx, fmt.Errorf("watching %s: %w", rt, err))
			return
		}

		// Inner event loop: runs until a stream closes (reconnect) or ctx
		// is cancelled.
		needReconnect := false
		for !needReconnect {
			select {
			case <-ctx.Done():
				stop()
				return
			case <-reconnect:
				// A watch stream ended — tear all streams down and re-list
				// + re-watch the whole selection.
				stop()
				needReconnect = true
			case <-events:
				// Any change on any stream → re-list the full selection
				// (matches the single-namespace "re-fetch full list for
				// simplicity" approach).
				items, err := FetchResourcesMulti(ctx, w.clientset, rt, sel)
				if err != nil {
					if ctx.Err() != nil {
						stop()
						return
					}
					continue
				}
				w.store(items)
				if !w.emit(ctx, items) {
					stop()
					return
				}
			}
		}
	}
}

// listAndEmit performs the initial list for the selection. For All it
// lists once; for an explicit set it lists each namespace in name order
// and emits the accumulated set after each one so the table fills in
// incrementally ([5]). Each emit sends a defensive copy so later appends
// don't mutate a slice the UI is holding.
func (w *Watcher) listAndEmit(ctx context.Context, rt ResourceType, sel NamespaceSelection) error {
	if sel.IsAll() {
		items, err := FetchResources(ctx, w.clientset, rt, "")
		if err != nil {
			return err
		}
		w.store(items)
		w.emit(ctx, items)
		return nil
	}

	var acc []ResourceItem
	for _, ns := range sel.List() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		items, err := FetchResources(ctx, w.clientset, rt, ns)
		if err != nil {
			return fmt.Errorf("namespace %s: %w", ns, err)
		}
		acc = append(acc, items...)
		snapshot := make([]ResourceItem, len(acc))
		copy(snapshot, acc)
		w.store(snapshot)
		if !w.emit(ctx, snapshot) {
			return ctx.Err()
		}
	}
	return nil
}

// startWatches opens the watch stream(s) for the selection and funnels
// them into a single event channel plus a reconnect signal. For All it is
// one cluster-wide watch; for a specific selection it is one watch per
// namespace. `events` receives a coalesced signal whenever any stream
// reports a change; `reconnect` is closed when any stream ends, telling
// the caller to re-establish everything. `stop` stops all streams and
// waits for the forwarder goroutines to exit (no leak on reconnect /
// cancel).
func (w *Watcher) startWatches(ctx context.Context, rt ResourceType, sel NamespaceSelection) (events <-chan struct{}, reconnect <-chan struct{}, stop func(), err error) {
	nss := sel.List()
	if sel.IsAll() {
		nss = []string{""}
	}

	watches := make([]watch.Interface, 0, len(nss))
	for _, ns := range nss {
		wi, werr := DefaultRegistry.StartWatch(ctx, w.clientset, rt, ns)
		if werr != nil {
			for _, prev := range watches {
				prev.Stop()
			}
			return nil, nil, nil, werr
		}
		watches = append(watches, wi)
	}

	evCh := make(chan struct{}, 1)
	reconnectCh := make(chan struct{})
	var once sync.Once
	closeReconnect := func() { once.Do(func() { close(reconnectCh) }) }

	var wg sync.WaitGroup
	for _, wi := range watches {
		wg.Add(1)
		go func(wi watch.Interface) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case _, ok := <-wi.ResultChan():
					if !ok {
						closeReconnect()
						return
					}
					// Coalesce: a single pending signal is enough to
					// trigger a full re-list; extra events collapse into it.
					select {
					case evCh <- struct{}{}:
					default:
					}
				}
			}
		}(wi)
	}

	stop = func() {
		for _, wi := range watches {
			wi.Stop()
		}
		wg.Wait()
	}
	return evCh, reconnectCh, stop, nil
}
