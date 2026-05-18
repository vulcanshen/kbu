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

// EditDoneMsg is sent when the edit flow finishes (editor closed + apply done).
type EditDoneMsg struct {
	Resource  string // e.g. "pods/my-pod"
	Namespace string
	Output    string // kubectl apply output
}

// editFetchFailedMsg is sent when kubectl get or temp file creation fails.
type editFetchFailedMsg struct{ err error }

// editTempReadyMsg is sent after the YAML has been fetched and written to a temp file.
type editTempReadyMsg struct {
	path        string
	original    []byte // original YAML bytes for change detection
	resource    string
	namespace   string
	contextName string // kubeconfig context to pass to kubectl subprocess
}

// editEditorDoneMsg is sent after the editor exits cleanly.
type editEditorDoneMsg struct {
	path        string
	original    []byte
	resource    string
	namespace   string
	contextName string
}

// editEditorCrashedMsg is sent when the editor exits with a non-zero code.
type editEditorCrashedMsg struct {
	path string
}

// editApplyFailedMsg is sent when kubectl apply returns a non-zero exit code.
type editApplyFailedMsg struct {
	resource  string
	namespace string
	output    string
}

// successNoticeClearMsg clears the success badge in the status bar.
// id must match AppModel.successNoticeID to avoid stale timers clearing a newer badge.
type successNoticeClearMsg struct{ id int }

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
