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
