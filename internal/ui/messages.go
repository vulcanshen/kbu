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

// ResourceDetailMsg carries detail data for the selected resource. ItemUID
// is the k8s UID of the item that triggered the fetch — the handler
// compares it against the currently selected item and drops stale
// results (slow fetch finishing after the user moved on). Required:
// out-of-order arrivals are otherwise indistinguishable from current ones.
type ResourceDetailMsg struct {
	ItemUID string
	Detail  k8s.ResourceDetail
	Events  []k8s.EventItem
}

// NamespaceListMsg carries the list of available namespaces.
type NamespaceListMsg struct {
	Namespaces []string
}

// LinkDrillMsg is emitted when the user presses Y on a Links-tab entry —
// it asks AppModel to fetch the cursor-pointed resource and open its
// YAML in a popup. (Y replaces the Enter-opens-YAML behavior that used to
// exist before Enter was reassigned to drill-into-Links.)
type LinkDrillMsg struct {
	Ref k8s.RefTarget
}

// LinkPushMsg is emitted when the user presses Enter / l on a drillable
// Links-tab entry — it asks AppModel to fetch the target and push it onto
// the Links-tab drill chain (so the panel re-renders showing the target's
// links). AppModel does a cycle pre-check (kind+ns+name against the
// existing chain) before dispatching the fetch; cycle hit → toast + drop.
type LinkPushMsg struct {
	Ref k8s.RefTarget
}

// linkDrillFetchedMsg carries the fetched resource for a LinkPushMsg.
// SourceUID is the table-selected item's UID at dispatch time — the
// handler drops the message when the table selection has moved on,
// mirroring the stale-drop guard on ResourceDetailMsg.
type linkDrillFetchedMsg struct {
	ref       k8s.RefTarget
	sourceUID string
	item      k8s.ResourceItem
	detail    k8s.ResourceDetail
	err       error
}

// LinkBreadcrumbMsg is emitted when the user presses `i` on the Links
// tab at depth>1 — opens the breadcrumb popup so they can jump back to
// any ancestor level.
type LinkBreadcrumbMsg struct{}

// SwitchToResourceMsg is emitted when the user confirms a Relatives-tab
// "jump to this resource" action. AppModel routes it by updating sidebar
// selection, recording a pending row-select for the next ResourceDataMsg,
// and emitting ResourceSelectedMsg so the watcher restarts on the new
// kind. This is the CONFIRMED step — see RequestSwitchToResourceMsg for
// the pre-confirm request.
type SwitchToResourceMsg struct {
	Ref k8s.RefTarget
}

// RequestSwitchToResourceMsg is emitted from any UI surface that wants to
// initiate a panel-1/2 jump (Relatives tab space, breadcrumb popup
// space). AppModel handles it by opening the confirm popup with
// SwitchToResourceMsg{Ref} queued as the on-confirm callback — same
// gating regardless of caller, so users don't get surprising silent
// switches from one entry-point but a confirm dialog from another.
type RequestSwitchToResourceMsg struct {
	Ref k8s.RefTarget
}

// FocusTableMsg is emitted when the user presses l / Enter on the
// sidebar (outside search mode). Sidebar's j/k already auto-selects
// the cursor row, so l / Enter would just re-fire the same
// ResourceSelectedMsg and waste a watcher restart. Instead, treat
// l / Enter as "I've picked the resource, now move me into the table"
// and let AppModel shift active panel.
type FocusTableMsg struct{}

// FocusDetailMsg is the table-panel counterpart of FocusTableMsg —
// pressing Enter on a row shifts focus to panel 3 instead of re-firing
// the row-selection effect that j/k already triggered.
type FocusDetailMsg struct{}

// LinkJumpMsg is emitted by the breadcrumb popup when the user picks a
// level to jump back to. Level=1 means root; values >Depth are clamped
// by the handler.
type LinkJumpMsg struct {
	Level int
}

// resourceFetchedForDrillMsg carries a resource fetched in response to an
// LinkDrillMsg, ready to populate a YamlPopup. err non-nil = fetch
// failed; caller should toast + skip popup.
type resourceFetchedForDrillMsg struct {
	ref  k8s.RefTarget
	item k8s.ResourceItem
	yaml string
	err  error
}

// aggregateLogsReadyMsg carries the resolved pod targets for a workload's
// aggregate-log stream. Emitted by startAggregateLogs after the pod-list API
// call completes off the Bubble Tea Update path. err non-nil = no targets;
// caller should log + skip stream start.
type aggregateLogsReadyMsg struct {
	resource k8s.ResourceType
	itemUID  string
	targets  []k8s.PodTarget
	err      error
}

// LogLineMsg carries a single log line from a container. Pod is empty when
// streaming from a single pod (single-pod mode — Pod identity is implicit);
// populated when streaming from a workload's multiple pods (aggregate mode)
// so the detail panel can render `<pod-hash>│<container>│<text>` prefixes.
type LogLineMsg struct {
	Pod       string
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
