package ui

import "github.com/vulcanshen/km8/internal/k8s"

// quitConfirmedMsg is emitted by the quit confirm dialog when the user
// confirms exit. AppModel handles it by stopping streams and calling tea.Quit.
type quitConfirmedMsg struct{}

// startEditMsg is emitted by the edit confirm dialog when the user confirms
// editing a resource. AppModel handles it by launching kubectl edit in PTY.
type startEditMsg struct {
	resource    k8s.ResourceType
	item        k8s.ResourceItem
	contextName string
}

// Panel identifies which UI panel has focus.
type Panel int

const (
	SidebarPanel Panel = iota
	TablePanel
	DetailPanel
)

// ResourceSelectedMsg is sent when a resource type is selected in the sidebar.
type ResourceSelectedMsg struct {
	Type k8s.ResourceType
}

// NamespaceChangedMsg is sent when the namespace filter changes.
type NamespaceChangedMsg struct {
	Namespace string
}

// ResourceDataMsg carries updated resource data from the watcher.
type ResourceDataMsg struct {
	Type  k8s.ResourceType
	Items []k8s.ResourceItem
}

// ResourceErrorMsg reports a watcher error.
type ResourceErrorMsg struct {
	Err error
}

// WatchEventMsg signals that a watch event was processed and more may follow.
type WatchEventMsg struct{}

// RowSelectedMsg is sent when the user selects a row in the table.
type RowSelectedMsg struct {
	Index int
}

// ResourceDetailMsg carries detail data for the selected resource.
type ResourceDetailMsg struct {
	Detail k8s.ResourceDetail
	Events []k8s.EventItem
}

// NamespaceListMsg carries the list of available namespaces.
type NamespaceListMsg struct {
	Namespaces []string
}

// LogLineMsg carries a single log line from a container.
type LogLineMsg struct {
	Container string
	Text      string
}

// ContextListMsg carries the list of available contexts and the current one.
type ContextListMsg struct {
	Contexts []string
	Current  string
}

// ContextChangedMsg is sent when the user selects a different kubeconfig context.
type ContextChangedMsg struct {
	Context string
}

// EditResourceMsg requests opening kubectl edit for a resource.
type EditResourceMsg struct {
	ResourceType k8s.ResourceType
	Name         string
	Namespace    string
}

// DeleteDoneMsg is sent when kubectl delete finishes.
type DeleteDoneMsg struct {
	Name      string
	Namespace string
	Resource  string // e.g. "pods/my-pod"
	Output    string
}

// DeleteErrMsg is sent when kubectl delete fails.
type DeleteErrMsg struct {
	Err error
}

// CRDsDiscoveredMsg is sent when CRD discovery completes.
type CRDsDiscoveredMsg struct {
	Count int
	Err   error
}
