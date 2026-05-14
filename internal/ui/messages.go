package ui

import "github.com/vulcanshen/km8/internal/k8s"

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
