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

// Watcher manages a Watch connection for a single resource type and
// maintains a local cache of items. It integrates with Bubble Tea
// through a channel-based message pattern.
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

// Start cancels any existing watch and starts watching the given resource type.
// It performs an initial List, then starts a Watch for incremental updates.
// Updates are sent to the internal channel — use WaitForUpdate() to receive them.
func (w *Watcher) Start(rt ResourceType, namespace string) {
	w.Stop()

	// Close old channels to unblock any stale waitForWatchUpdate goroutines,
	// then create fresh channels for the new watcher cycle.
	close(w.updates)
	close(w.errors)
	w.updates = make(chan WatchMsg, 1)
	w.errors = make(chan WatchErrMsg, 1)

	ctx, cancel := context.WithCancel(context.Background())
	w.mu.Lock()
	w.cancel = cancel
	w.items = nil
	w.mu.Unlock()

	go w.run(ctx, rt, namespace)
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

// Channels returns both channels atomically, preventing a TOCTOU race where
// Start() replaces one channel between two separate Updates()/Errors() calls.
func (w *Watcher) Channels() (<-chan WatchMsg, <-chan WatchErrMsg) {
	return w.updates, w.errors
}

func (w *Watcher) run(ctx context.Context, rt ResourceType, namespace string) {
	for {
		items, err := FetchResources(ctx, w.clientset, rt, namespace)
		if err != nil {
			if ctx.Err() != nil {
				return // context was cancelled intentionally (e.g. user switched resource)
			}
			select {
			case w.errors <- WatchErrMsg{Err: fmt.Errorf("listing %s: %w", rt, err)}:
			case <-ctx.Done():
			}
			return
		}

		w.mu.Lock()
		w.items = items
		w.mu.Unlock()

		select {
		case w.updates <- WatchMsg{Items: items}:
		case <-ctx.Done():
			return
		}

		watcher, err := w.startWatch(ctx, rt, namespace)
		if err != nil {
			if ctx.Err() != nil {
				return // context was cancelled intentionally
			}
			select {
			case w.errors <- WatchErrMsg{Err: fmt.Errorf("watching %s: %w", rt, err)}:
			case <-ctx.Done():
			}
			return
		}

		// Inner event loop: runs until the watch stream closes or ctx is cancelled.
		// On stream close (!ok), break to the outer loop to re-list and reconnect.
		reconnect := false
		for !reconnect {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					watcher.Stop()
					reconnect = true
				} else {
					w.handleEvent(ctx, event, rt, namespace)
				}
			}
		}
	}
}

func (w *Watcher) handleEvent(ctx context.Context, event watch.Event, rt ResourceType, namespace string) {
	switch event.Type {
	case watch.Added, watch.Modified, watch.Deleted:
		// Re-fetch the full list for simplicity.
		// A production implementation would apply incremental updates.
		items, err := FetchResources(ctx, w.clientset, rt, namespace)
		if err != nil {
			return
		}

		w.mu.Lock()
		w.items = items
		w.mu.Unlock()

		select {
		case w.updates <- WatchMsg{Items: items}:
		case <-ctx.Done():
		}

	case watch.Error:
		// Restart watch
	}
}

func (w *Watcher) startWatch(ctx context.Context, rt ResourceType, namespace string) (watch.Interface, error) {
	return DefaultRegistry.StartWatch(ctx, w.clientset, rt, namespace)
}
