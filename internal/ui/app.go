package ui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
	"github.com/vulcanshen/kbu/internal/version"
)

// aggregateLogsRetryInterval throttles re-attempts of failed aggregate
// log stream starts. 10s strikes the balance: transient pod-rollover
// errors recover within one or two windows; persistent RBAC denials
// log one warning + one kubectl-equivalent API call per window instead
// of one per watcher tick. Row-cursor change clears the throttle
// (different row may have different RBAC outcomes).
const aggregateLogsRetryInterval = 10 * time.Second

// rowSwitchDebounce coalesces rapid j/k mashing in panel 2: every
// RowSelectedMsg bumps m.rowSeq and schedules a tick this far in the
// future, and the heavy work (fetchResourceDetail, logStreamer.Start,
// startAggregateLogs) only fires when the tick lands on the latest
// rowSeq. Without this, a 10-row scroll would Start/Stop 10 log
// streams and queue 10 detail fetches against the API. Mirrors the
// switchSeq pattern used by resourceSwitchTickMsg for sidebar kind
// switches; 300ms matches that constant for muscle-memory consistency.
const rowSwitchDebounce = 300 * time.Millisecond

type resourceSwitchTickMsg struct {
	seq int
}

// rowSwitchTickMsg lands rowSwitchDebounce after a RowSelectedMsg. The
// handler drops it when seq != m.rowSeq — that's the debounce primitive:
// every subsequent nav (RowSelectedMsg, ResourceSelectedMsg, namespace
// switch, context switch, drill-down, drill-up) bumps m.rowSeq so any
// older in-flight tick falls on the floor. kind+item snapshot the row
// at nav time so the handler doesn't race against intervening kind/ns
// switches. drillContainer is non-nil for container-level drill-down
// rows; kind+item are ignored in that path.
type rowSwitchTickMsg struct {
	seq            int
	kind           k8s.ResourceType
	item           k8s.ResourceItem
	drillContainer *k8s.ContainerInfo
}

type drillDownMsg struct {
	parentType k8s.ResourceType
	parentName string
	childType  k8s.ResourceType
	children   []k8s.ResourceItem
}

// Main view layout — absolute cells, no percentages. Stack model:
//
//	horizontal: hMargin + sidebar(sw) + hSpace + rightSide(rw) + hMargin = m.width
//	vertical:   statusBar(1) + middle + statusLine = m.height
//	right side: table(upperH) + vSpace + detail(detailH) = middleH
//	sidebar:    fills middleH (no internal vSpace)
//
// Only sw and detailH are pinned; everything else falls out by subtraction.
const (
	panelSidebarWidth = 24 // panel 1 (sidebar) — fixed absolute width
	panelDetailHeight = 14 // panel 3 (detail)  — fixed absolute height
	panelHMargin      = 1  // cells between terminal left/right edge and panels
	panelHSpace       = 0  // cells between sidebar and right side (0 = flush borders)
	panelVSpace       = 0  // rows between table and detail (0 = flush borders)
)

type AppModel struct {
	sidebar         SidebarModel
	table           TableModel
	detail          DetailModel
	statusBar       StatusBarModel
	statusLine      StatusLineModel
	namespacePicker NamespacePickerModel
	loadingTicking  bool // a loading-icon tick is in flight (tdp D3)
	contextPicker   ContextPickerModel
	help            HelpModel
	appLog          AppLogModel
	confirm         ConfirmModel
	splash          SplashModel
	toast           ToastModel
	// Dual-slot PTY: the persistent Alterm shell (shellPty) and any
	// transient PTY for kubectl edit / exec (txPty) live independently so
	// the user can keep a long-running shell hidden in the background while
	// editing or exec'ing into a container. tx-on-top is enforced at render
	// + input routing time so only one popup is visible at a time.
	shellPty        *PtyView
	txPty           *PtyView
	yamlPopup       YamlPopupModel
	comparePopup    CompareYamlPopupModel
	breadcrumbPopup BreadcrumbPopupModel
	// spaceMenu is every panel's Space menu (tdp M2); globalMenu is the
	// global operation popup its last row opens (M4).
	spaceMenu  MenuPopupModel
	globalMenu MenuPopupModel
	// listPicker is the sort flow's column step; sortDirPicker is its
	// direction step, stacked on it (tdp F1: one popup per step).
	listPicker    ListPickerModel
	sortDirPicker ListPickerModel
	settingsPopup SettingsPopupModel

	activePanel     Panel
	width           int
	height          int
	theme           *theme.Theme
	cfg             *config.Config // user config — mutated + persisted on pin/unpin
	cfgEditor       string
	editing         bool
	successNotice   string
	successNoticeID int
	k8sClient       *k8s.Client
	watcher         *k8s.Watcher
	logStreamer     *k8s.LogStreamer
	currentResource k8s.ResourceType
	items           []k8s.ResourceItem
	ready           bool
	logsActive      bool
	// nextAggregateRetry throttles automatic re-attempts of aggregate
	// log streaming after a failure (RBAC denial, zero pods, transient
	// API error). Without it, the ResourceDataMsg watcher gate would
	// re-fire startAggregateLogs on every watcher tick until the
	// user navigated away — one duplicated AppLog warning + one
	// kubectl-equivalent API call per tick, sustained for the
	// session. The throttle window is short enough that legitimate
	// transient failures (pods mid-rollout) recover within a few
	// seconds; permanent failures (RBAC) get one entry per window
	// instead of one per tick. Cleared on row change so a new row
	// gets an immediate fresh attempt.
	nextAggregateRetry time.Time
	detailExpanded     bool
	tableExpanded      bool
	switchSeq          int
	// rowSeq is bumped by RowSelectedMsg and every nav handler that
	// resets nextAggregateRetry, so any in-flight rowSwitchTickMsg from
	// before the nav drops on receipt (seq comparison). See rowSwitchTickMsg.
	rowSeq int

	// Drill-down state
	drillDownStack      []drillDownEntry
	drillDownPod        *k8s.ResourceItem // innermost: Pod → Container
	drillDownContainers []k8s.ContainerInfo

	// pendingTableSelect holds the (kind, ns, name) of a resource the
	// user asked to switch to via the Relatives-tab space hotkey. When the
	// next ResourceDataMsg for the matching kind arrives, the table
	// cursor jumps to the row whose name+namespace matches, and the
	// pointer is cleared. nil otherwise.
	pendingTableSelect *k8s.RefTarget

	// Compare mode state. compareLock holds the baseline row the user
	// picked via panel-2 Space → "Lock to compare". Subsequent "Compare
	// to this" picks open the diff popup between the locked item and
	// the new cursor row (same resource type required). Cleared on Exit
	// compare / panel focus leaving panel 2 / locked item disappearing
	// from the watcher stream. Nil = not in compare mode.
	compareLock *compareLockedRef

	// Sort flow in-flight state. The flow is two stacked popups — the
	// column picker, then the direction picker over it; these fields
	// carry the user's column choice across the column → direction
	// step so the direction commit knows which column to persist.
	// sortFlowColumn clears when the direction step ends (commit or
	// Esc); sortFlowKind when the column picker closes. Empty
	// kind/column = no flow in progress.
	sortFlowKind   k8s.ResourceType
	sortFlowColumn string

	// Mouse double-click detection. Bubbletea's MouseMsg doesn't
	// carry a timestamp so we stamp wall-clock at press time and
	// look back N ms on the next press to decide single vs double.
	// Same panel + adjacent cell within doubleClickWindow → emit
	// Enter; otherwise treat as a fresh single click.
	lastLeftPressAt    time.Time
	lastLeftPressX     int
	lastLeftPressY     int
	lastLeftPressPanel Panel
}

// compareLockedRef identifies the panel-2 row currently locked as the
// comparison baseline. UID is the authoritative identity (survives
// renames + per-watcher restarts); type/name/namespace are stamped at
// lock time for status-bar label rendering without re-looking-up the
// row.
type compareLockedRef struct {
	uid          string
	resourceType k8s.ResourceType
	name         string
	namespace    string
}

// inCompareMode reports whether the user has a locked baseline row.
func (m AppModel) inCompareMode() bool { return m.compareLock != nil }

// setCompareLock stamps the currently-pointed item as the comparison
// baseline. Caller must have already verified the item is selectable
// (non-empty list, cursor in range).
func (m *AppModel) setCompareLock(item k8s.ResourceItem, rt k8s.ResourceType) {
	m.compareLock = &compareLockedRef{
		uid:          item.UID,
		resourceType: rt,
		name:         item.Name,
		namespace:    item.Namespace,
	}
	m.syncCompareLockToTable()
}

// clearCompareLock exits compare mode. Idempotent — calling when already
// out of compare mode is a no-op, so the various exit paths (Space-menu
// Exit / focus change / item deletion) can all funnel here without
// pre-checks.
func (m *AppModel) clearCompareLock() {
	m.compareLock = nil
	m.table.SetLockedRow(-1)
}

// compareCtxForMenu assembles the panel2CompareCtx the menu needs to
// decide which of "Mark / Compare to / Unmark" to surface for the
// cursor-pointed row. Centralised here so the gating rules live with
// the lock state itself rather than in the menu file.
func (m AppModel) compareCtxForMenu(cursorItem k8s.ResourceItem) panel2CompareCtx {
	ctx := panel2CompareCtx{
		locked:  m.inCompareMode(),
		canLock: len(m.items) > 1,
	}
	if ctx.locked {
		ctx.cursorOnAnchor = m.compareLock.uid == cursorItem.UID
		ctx.cursorComparable = !ctx.cursorOnAnchor &&
			m.compareLock.resourceType == m.currentResource
	}
	return ctx
}

// popupDepth counts the popups that hold a place in the stack, so a
// newly-opening popup can stamp itself layer = depth + 1 via SetLayer
// before its Open/Show/Toggle call; the layer picks the border colour
// on the lavender → sapphire scale (tdp D2). Same stackOrder and the
// same owns() test the key routing uses: a popup closing on its way
// out no longer counts (tdp F3).
func (m AppModel) popupDepth() int {
	n := 0
	for _, l := range m.stackOrder() {
		if l.owns() {
			n++
		}
	}
	return n
}

// closeAllBlockingPopups batches the close cmds for every popup in the
// stack. Used by context-shift target entry handlers (PTY shell /
// kubectl edit / kubectl exec / drill-down, tdp T1) — the source popup
// that launched the action should not still be sitting underneath when
// the user returns from a minute-long subprocess session or a
// swapped-out panel 2 view. Returns nil when nothing is open so callers
// can unconditionally tea.Batch the result.
//
// Toast is not in the stack: it is non-blocking and auto-dismisses on
// its own timer. PTY slots (shellPty / txPty) are skipped — they have
// their own mutual-exclusion logic (toast warn "Close current edit/exec
// PTY first") and shouldn't cascade-close each other.
func (m *AppModel) closeAllBlockingPopups() tea.Cmd {
	var cmds []tea.Cmd
	for _, l := range m.stackOrder() {
		if _, isPty := l.(*PtyView); isPty {
			continue
		}
		if l.owns() {
			cmds = append(cmds, l.closeLayer())
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// compareHotkeyDispatch routes the "C" hotkey contextually:
//   - no anchor set → mark the given row as the anchor
//   - anchor set, cursor on a different row of the same kind → open
//     the diff popup against the anchor
//   - anchor set, cursor sits on the anchor row itself → cancel the
//     anchor (exit compare mode). Makes C a toggle from any row of
//     the same kind: press C to mark, press C again on the same
//     row to unmark.
//   - anchor kind differs from the current row's kind → silent no-op
//     (the menu hides the C entry in that case too)
//
// Used by BOTH the menu commit handler (case "C") and the direct
// panel-2 C-key path so the two surfaces can't drift on edge cases.
func (m *AppModel) compareHotkeyDispatch(rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	if m.inCompareMode() {
		if m.compareLock.uid == item.UID {
			m.clearCompareLock()
			m.appLog.Info("compare: anchor cleared")
			return nil
		}
		if m.compareLock.resourceType != rt {
			return nil
		}
		return m.openCompareDiff(item)
	}
	if len(m.items) <= 1 {
		return nil
	}
	m.setCompareLock(item, rt)
	m.appLog.Info(fmt.Sprintf("compare: anchor set on %s/%s", rt.KubectlName(), item.Name))
	return nil
}

// openCompareDiff resolves the locked anchor out of the current items
// slice (UID lookup — name/ns are stamped at anchor time but the row
// could have been recreated since), strips both YAMLs to compare-clean
// form, and opens the diff popup.
func (m *AppModel) openCompareDiff(item k8s.ResourceItem) tea.Cmd {
	var anchorItem *k8s.ResourceItem
	for i := range m.items {
		if m.items[i].UID == m.compareLock.uid {
			anchorItem = &m.items[i]
			break
		}
	}
	if anchorItem == nil {
		return m.toast.Show("compare: anchor item gone")
	}
	leftYAML := k8s.MarshalItemYAMLForCompare(*anchorItem)
	rightYAML := k8s.MarshalItemYAMLForCompare(item)
	leftLabel := fmt.Sprintf("%s/%s", anchorItem.Namespace, anchorItem.Name)
	rightLabel := fmt.Sprintf("%s/%s", item.Namespace, item.Name)
	if anchorItem.Namespace == "" {
		leftLabel = anchorItem.Name
	}
	if item.Namespace == "" {
		rightLabel = item.Name
	}
	m.comparePopup.SetSize(m.width, m.height)
	m.comparePopup.SetLayer(m.popupDepth() + 1)
	return m.comparePopup.Open(leftYAML, rightYAML, leftLabel, rightLabel)
}

// togglePinnedKind flips the pin status for the given resource kind:
//   - not pinned → AddPinned (kind moves from its original category to Pinned)
//   - already pinned → RemovePinned (kind moves back to its original category)
//
// Used by both the direct `P` hotkey on panel 1 and the Space-menu
// PinKind / UnpinKind actions — single funnel keeps the two paths
// from drifting on edge cases. Persists the updated list to config
// atomically; on save failure the in-memory state already changed
// but the toast surfaces the error.
//
// Cursor follows the kind: each kind has exactly one location, so
// SnapCursorToKind picks up wherever Pods (say) ended up after the
// move — Pinned section if just pinned, original Workloads if just
// unpinned. No "remember category" bookkeeping needed.
func (m *AppModel) togglePinnedKind(rt k8s.ResourceType) tea.Cmd {
	if m.sidebar.IsPinned(rt) {
		m.sidebar.RemovePinned(rt)
	} else {
		m.sidebar.AddPinned(rt)
	}
	m.sidebar.SnapCursorToKind(rt)
	if err := m.persistPinnedKinds(); err != nil {
		m.appLog.Error("pin save failed: " + err.Error())
		return m.toast.Show("pin save failed")
	}
	return nil
}

// openSortColumnPicker opens the column picker, the first step of the
// Sort flow. Items are the kind's column titles; a column already in the
// sort chain is badged with its priority and direction. Caches kind in
// sortFlowKind so the direction step knows what kind it's committing for
// even if the sidebar cursor drifts mid-flow.
func (m *AppModel) openSortColumnPicker(rt k8s.ResourceType) tea.Cmd {
	items := sortColumnItems(rt, m.cfg)
	if items == nil {
		return nil
	}
	def := sortRegistry().Get(rt)
	m.sortFlowKind = rt
	m.sortFlowColumn = ""
	title := sortPopupIcon + " Sort " + def.DisplayName + " by…"
	m.listPicker.SetSize(m.width, m.height)
	m.listPicker.SetLayer(m.popupDepth() + 1)
	return m.listPicker.Open("sort:column", title, items)
}

// sortColumnItems builds the column picker's rows: the columns under
// "fields" (the cursor's group, tdp M8), then Reset under "all". Reset is
// always listed — dimmed while there is no sort to reset (tdp M6) — so
// the picker's height is fixed from the moment it opens and doesn't grow
// a row when the first tier lands (F7).
func sortColumnItems(rt k8s.ResourceType, cfg *config.Config) []ListPickerItem {
	def := sortRegistry().Get(rt)
	if def == nil || len(def.Columns) == 0 || cfg == nil {
		return nil
	}
	chain := cfg.GetSort(def.KubectlName)
	items := make([]ListPickerItem, 0, len(def.Columns)+4)
	items = append(items, ListPickerItem{Header: true, Label: "fields"})
	for _, c := range def.Columns {
		it := ListPickerItem{Key: c.Title, Label: c.Title}
		// Columns in the chain get a priority+direction badge —
		// single-tier chain shows just the arrow, multi-tier shows
		// "(N) ↑" so the user sees the tier order.
		if idx := chain.IndexOf(c.Title); idx >= 0 {
			it.Badge = sortTierBadge(idx, chain[idx].Direction, len(chain))
		}
		items = append(items, it)
	}
	items = append(items, ListPickerItem{Separator: true})
	items = append(items, ListPickerItem{Header: true, Label: "all"})
	items = append(items, ListPickerItem{Key: sortResetKey, Label: "Reset " + resetIcon, Disabled: len(chain) == 0})
	return items
}

// refreshSortColumnPicker redraws the open column picker's badges (and
// Reset's dimming) in place after a tier lands or the chain resets.
func (m *AppModel) refreshSortColumnPicker(rt k8s.ResourceType) {
	if items := sortColumnItems(rt, m.cfg); items != nil && m.listPicker.owns() {
		m.listPicker.SetItems(items)
	}
}

// sortTierBadge formats the priority + direction marker used in the
// column picker. Mirrors the table header's collapse rule: a
// single-tier chain shows just the direction arrow (no "(1)"), a
// multi-tier chain shows "(N) ↑/↓" so the user sees the tier order.
// chainLen is the total length so the picker and header stay
// visually consistent.
func sortTierBadge(idx int, direction string, chainLen int) string {
	arrow := sortDirectionGlyph(direction)
	if chainLen <= 1 {
		return arrow
	}
	return fmt.Sprintf("(%d) %s", idx+1, arrow)
}

// openSortDirectionPicker opens the second step over the column picker
// (tdp F1, F4): Esc on it returns to the column picker. Always offers
// Ascending / Descending; offers Unset ONLY when the column is already in
// the chain — for a column that isn't sorted there is no tier to unset.
// When the column IS in the chain, its current direction gets badged
// "current" so the user sees their existing pick.
func (m *AppModel) openSortDirectionPicker(rt k8s.ResourceType, column string) tea.Cmd {
	def := sortRegistry().Get(rt)
	if def == nil {
		return nil
	}
	m.sortFlowColumn = column
	chain := m.cfg.GetSort(def.KubectlName)
	items := []ListPickerItem{
		{Key: config.SortDirectionAscending, Label: "Ascending"},
		{Key: config.SortDirectionDescending, Label: "Descending"},
	}
	if idx := chain.IndexOf(column); idx >= 0 {
		for i := range items {
			if items[i].Key == chain[idx].Direction {
				items[i].Badge = "current"
			}
		}
		items = append(items, ListPickerItem{Key: "unset", Label: "Unset"})
	}
	title := sortPopupIcon + " Sort " + def.DisplayName + " by " + column + "…"
	m.sortDirPicker.SetSize(m.width, m.height)
	m.sortDirPicker.SetLayer(m.popupDepth() + 1)
	return m.sortDirPicker.Open("sort:direction", title, items)
}

// commitSortFlow finalises one tier — column + direction. Persists the
// upsert (or removes the tier on "unset"), re-applies the sort to live
// items, closes the direction step and refreshes the column picker's
// badges in place, so the user can stack another tier or Esc out. The
// column picker stays open until the user closes it.
func (m *AppModel) commitSortFlow(direction string) tea.Cmd {
	rt := m.sortFlowKind
	column := m.sortFlowColumn
	m.sortFlowColumn = ""
	closeDir := m.sortDirPicker.Close()
	// Defensive: inconsistent state (missing kind / column / cfg /
	// registry entry) only ends the direction step.
	if rt == "" || column == "" || m.cfg == nil {
		return closeDir
	}
	def := sortRegistry().Get(rt)
	if def == nil {
		return closeDir
	}
	chain := m.cfg.GetSort(def.KubectlName)
	switch direction {
	case "unset":
		// Unset removes just THIS tier from the chain. The direction
		// step only offers Unset for in-chain columns; the guard stays
		// against stale picker state.
		if chain.IndexOf(column) < 0 {
			return closeDir
		}
		m.cfg.UnsetSortColumn(def.KubectlName, column)
	case config.SortDirectionAscending, config.SortDirectionDescending:
		m.cfg.SetSort(def.KubectlName, column, direction)
	default:
		return closeDir
	}
	var saveErrCmd tea.Cmd
	if err := m.cfg.Save(); err != nil {
		// In-memory state already mutated — surface the disk-side
		// failure via both the app log (full error) and a toast.
		m.appLog.Error("sort save failed: " + err.Error())
		saveErrCmd = m.toast.Show("sort save failed")
	}
	// Re-apply sort to whatever's currently in panel 2 if this
	// kind is the one being viewed — user sees the data reshape
	// immediately while still in the picker.
	if rt == m.currentResource {
		m.syncTableSortIndicator()
		m.applySortToItems()
		rows := augmentRowsWithHelm(m.items, m.currentResource)
		m.table.SetRows(rows)
	}
	m.refreshSortColumnPicker(rt)
	return tea.Batch(closeDir, saveErrCmd)
}

// resetSortFlow is the column picker's Reset row: drop the entire chain
// for this kind, re-apply the fallback sort to live items, and refresh
// the column picker in place (Reset dims again). With nothing to reset
// the row is dimmed and never reaches here; the guard stays defensive.
func (m *AppModel) resetSortFlow() tea.Cmd {
	rt := m.sortFlowKind
	m.sortFlowColumn = ""
	if rt == "" || m.cfg == nil {
		return nil
	}
	def := sortRegistry().Get(rt)
	if def == nil || len(m.cfg.GetSort(def.KubectlName)) == 0 {
		return nil
	}
	m.cfg.ResetSort(def.KubectlName)
	var saveErrCmd tea.Cmd
	if err := m.cfg.Save(); err != nil {
		m.appLog.Error("sort save failed: " + err.Error())
		saveErrCmd = m.toast.Show("sort save failed")
	}
	if rt == m.currentResource {
		m.syncTableSortIndicator()
		m.applySortToItems()
		rows := augmentRowsWithHelm(m.items, m.currentResource)
		m.table.SetRows(rows)
	}
	m.refreshSortColumnPicker(rt)
	return saveErrCmd
}

// applySortToItems re-orders m.items per the current kind's saved
// sort config. When no config exists for the kind, falls back to
// (Namespace asc, Name asc) — matches kubectl's default order so a
// cross-namespace Pods list groups by namespace the way users
// expect, and degenerates to Name asc for cluster-scoped kinds
// (Namespace is uniformly empty there). The fallback also catches
// the Unset path: clearing the saved sort needs to actually re-
// order panel 2 immediately, not wait for the next kind switch.
//
// Called on every ResourceDataMsg before the table sees the rows,
// and immediately after a direction commit so the view reflects the
// new order without waiting for the next watcher tick.
func (m *AppModel) applySortToItems() {
	if m.cfg == nil {
		return
	}
	def := sortRegistry().Get(m.currentResource)
	if def == nil {
		return
	}
	chain := m.cfg.GetSort(def.KubectlName)
	if len(chain) == 0 {
		sort.SliceStable(m.items, func(i, j int) bool {
			if m.items[i].Namespace != m.items[j].Namespace {
				return m.items[i].Namespace < m.items[j].Namespace
			}
			return m.items[i].Name < m.items[j].Name
		})
		return
	}
	tiers := make([]k8s.SortTier, len(chain))
	for i, c := range chain {
		tiers[i] = k8s.SortTier{
			Column:    c.Column,
			Ascending: c.Direction == config.SortDirectionAscending,
		}
	}
	k8s.SortItemsChain(m.items, def.Columns, tiers)
}

// syncTableSortIndicator pushes the current kind's saved sort
// (column + direction) to the table so the panel-2 header renders
// the right arrow. Called wherever sort state can change relative
// to the table: kind switch, sort commit, app init. Empty kind /
// empty config clears the indicator.
func (m *AppModel) syncTableSortIndicator() {
	if m.cfg == nil || m.currentResource == "" {
		m.table.SetSortIndicators(nil)
		return
	}
	def := sortRegistry().Get(m.currentResource)
	if def == nil {
		m.table.SetSortIndicators(nil)
		return
	}
	m.table.SetSortIndicators(m.cfg.GetSort(def.KubectlName))
}

// buildSettingsItems snapshots the current config state into the row
// list the SettingsPopup renders. Called at popup Open and again
// after each toggle (via SetItems) so the displayed badge always
// reflects the live config.
//
// ValueOn drives the badge colour. For boolean settings (Mouse) it
// matches the actual on/off state. For multi-value settings
// (Scroll Direction) it stays true — both values are valid choices,
// so the badge stays green-coloured rather than dropping to grey on
// "reverse".
func (m *AppModel) buildSettingsItems() []SettingsItem {
	mouseOn := m.cfg.IsMouseEnabled()
	mouseBadge := "OFF"
	if mouseOn {
		mouseBadge = "ON"
	}
	scrollDir := m.cfg.MouseScrollDirection()
	scrollBadge := "NATURAL"
	if scrollDir == config.MouseScrollReverse {
		scrollBadge = "REVERSE"
	}
	return []SettingsItem{
		{Key: "mouse", Label: "Mouse", ValueText: mouseBadge, ValueOn: mouseOn},
		{Key: "scroll", Label: "Scroll Direction", ValueText: scrollBadge, ValueOn: true},
	}
}

// commitSettingsToggle handles the SettingsToggleMsg the popup emits
// on Enter / mouse click. Switches on the row's Key to apply the
// actual change, persists to disk, then refreshes the popup's items
// so the badge flips visually without close-reopen.
//
// "scroll" cycles between two values (natural ↔ reverse) — same
// commit shape as the binary toggle, just a different mutation
// underneath.
func (m *AppModel) commitSettingsToggle(key string) tea.Cmd {
	if m.cfg == nil {
		return nil
	}
	var cmds []tea.Cmd
	switch key {
	case "mouse":
		m.cfg.SetMouseEnabled(!m.cfg.IsMouseEnabled())
	case "scroll":
		next := config.MouseScrollReverse
		if m.cfg.MouseScrollDirection() == config.MouseScrollReverse {
			next = config.MouseScrollNatural
		}
		m.cfg.SetMouseScrollDirection(next)
	default:
		return nil
	}
	if err := m.cfg.Save(); err != nil {
		m.appLog.Error("settings save failed: " + err.Error())
		cmds = append(cmds, m.toast.Show("settings save failed"))
	}
	m.settingsPopup.SetItems(m.buildSettingsItems())
	return tea.Batch(cmds...)
}

// doubleClickWindow is the max gap between two left presses that
// counts as a double-click. Standard desktop default.
const doubleClickWindow = 500 * time.Millisecond

// handleMousePress is the main mouse dispatcher. Runs only on
// MouseActionPress events (release / motion are no-ops in phase 1).
// Single left-click → focus the hit panel + move cursor to the
// clicked row. Double-left-click → synthesize Enter so the existing
// keyboard path handles drill / focus-into. Right-click →
// synthesize Space so the existing Space-menu paths apply.
//
// Hit-test ignores clicks outside any panel (e.g. on the status bar
// or border gutters) and clicks that land while MouseEnabled is
// false. Popup hit-testing is left to the popup's own MouseMsg
// handler — this dispatcher only fires when no interactive popup is
// in front.
func (m *AppModel) handleMousePress(msg tea.MouseMsg) tea.Cmd {
	if m.cfg == nil || !m.cfg.IsMouseEnabled() {
		return nil
	}
	if msg.Action != tea.MouseActionPress {
		return nil
	}
	panel, ok := m.panelAt(msg.X, msg.Y)
	if !ok {
		return nil
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		now := time.Now()
		isDouble := !m.lastLeftPressAt.IsZero() &&
			now.Sub(m.lastLeftPressAt) <= doubleClickWindow &&
			panel == m.lastLeftPressPanel &&
			abs(msg.X-m.lastLeftPressX) <= 1 &&
			abs(msg.Y-m.lastLeftPressY) <= 1
		m.lastLeftPressAt = now
		m.lastLeftPressX = msg.X
		m.lastLeftPressY = msg.Y
		m.lastLeftPressPanel = panel
		// First click of any pair: always focus + cursor. Idempotent
		// when this turns out to also be the first half of a double.
		m.setPanel(panel)
		selCmd := m.cursorToScreenY(panel, msg.Y)
		// A double-click is Enter on panels 2 and 3. On panel 1 it only
		// selects: Enter there moves focus to panel 2, and a click on
		// panel 1 shouldn't send focus away from where the user
		// pointed (tdp X2 leaves the mapping to the app).
		if isDouble && panel != SidebarPanel {
			// Reset so a third press isn't read as another double.
			m.lastLeftPressAt = time.Time{}
			enterCmd := func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }
			return tea.Batch(selCmd, enterCmd)
		}
		return selCmd
	case tea.MouseButtonRight:
		m.setPanel(panel)
		selCmd := m.cursorToScreenY(panel, msg.Y)
		spaceCmd := func() tea.Msg { return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}} }
		return tea.Batch(selCmd, spaceCmd)
	}
	return nil
}

// panelAt does hit-test on the screen coordinate against the
// currently-laid-out panel rects. Returns the panel under (x, y) and
// true; (zero, false) if the point is outside any panel (status bar,
// margins, gaps). The arithmetic mirrors panelSizes() and the
// renderer's positioning so it stays correct across resize +
// expanded modes.
func (m AppModel) panelAt(x, y int) (Panel, bool) {
	sw, rw, upperH, detailH := m.panelSizes()
	middleY := 1 // status bar height
	sidebarX := panelHMargin
	rightX := panelHMargin + sw + panelHSpace
	totalRightH := upperH + panelVSpace + detailH

	// Sidebar fills the entire middle on the left.
	if x >= sidebarX && x < sidebarX+sw && y >= middleY && y < middleY+totalRightH {
		return SidebarPanel, true
	}
	// Right side splits into table (top) + detail (bottom), unless an
	// expanded mode collapses one of them.
	if x >= rightX && x < rightX+rw && y >= middleY && y < middleY+totalRightH {
		if m.tableExpanded {
			return TablePanel, true
		}
		if m.detailExpanded {
			return DetailPanel, true
		}
		if y < middleY+upperH {
			return TablePanel, true
		}
		return DetailPanel, true
	}
	return SidebarPanel, false
}

// cursorToScreenY moves the targeted panel's cursor to the row the
// user clicked and returns the selection-change cmd the panel emits
// (mirrors keyboard j/k behaviour — sidebar fires
// ResourceSelectedMsg, table fires RowSelectedMsg, detail's
// Relatives tab moves m.relativeCursor without an emit). screenY is
// the absolute screen Y; per-panel offsets are computed here.
func (m *AppModel) cursorToScreenY(panel Panel, screenY int) tea.Cmd {
	// Strip the status bar (always at y=0). Sidebar / table top
	// borders sit at the post-stripped y=0.
	screenY -= 1
	switch panel {
	case SidebarPanel:
		return m.sidebar.SetCursorAtScreenY(screenY)
	case TablePanel:
		return m.table.SetCursorAtScreenY(screenY)
	case DetailPanel:
		// Detail's top y depends on layout mode:
		//   - detailExpanded:  detail fills the right side, top at 0
		//   - normal:          detail sits below the table, top at upperH
		//   - tableExpanded:   detail isn't rendered, panelAt won't return DetailPanel
		_, _, upperH, _ := m.panelSizes()
		detailTop := 0
		if !m.detailExpanded {
			detailTop = upperH
		}
		return m.detail.SetCursorAtScreenY(screenY - detailTop)
	}
	return nil
}

// sortRegistry is the registry the sort flow looks up resource
// definitions in. Returns the global DefaultRegistry — which is the
// same instance k8s.Client wraps via .Registry() — so the sort
// helpers don't require a constructed k8s.Client during tests, and
// production code still hits the one and only registry. If a future
// app supports multiple registries (multi-cluster?), wrap this in a
// per-AppModel field.
func sortRegistry() *k8s.Registry { return k8s.DefaultRegistry }

// sortAscendingGlyph / sortDescendingGlyph render the Nerd Font
// arrows the user picked for sort direction indicators (U+F161 up
// / U+F160 down). Centralised here so the listPicker and the
// panel-2 header use the same glyphs.
const (
	sortAscendingGlyph  = ""
	sortDescendingGlyph = ""
)

// sortResetKey is the sentinel ListPickerItem.Key used for the
// "Reset" shortcut row at the bottom of the column picker. Picking
// it bypasses the direction step and unsets this kind's sort
// outright. Internal-only — never persisted, never matched against
// real column titles.
const sortResetKey = "__sort_reset__"

// resetIcon (U+F0E2, nf-fa-undo) is the Nerd Font glyph appended
// to the sort column picker's "Reset" row. Undo arrow signals "back
// to the previous (default) state" — matches Reset's semantic (drop
// the sort entry, fall back to Name asc). Same label-with-trailing-
// icon pattern as panel-2 menu's "Enter <drillDownIcon>" entry.
const resetIcon = "\uF0E2"

// sortPopupIcon (U+F0DC, nf-fa-sort) sits in the sort picker's title
// border so the popup's purpose is recognisable at a glance — same
// surface convention as hintpopup's titleIcon (kbu wheel) and
// settingspopup's cog.
const sortPopupIcon = "\uF0DC"

// sortDirectionGlyph returns the right arrow glyph for a saved
// direction string. Used by the column picker to badge the
// currently-sorted column.
func sortDirectionGlyph(direction string) string {
	switch direction {
	case config.SortDirectionAscending:
		return sortAscendingGlyph
	case config.SortDirectionDescending:
		return sortDescendingGlyph
	}
	return ""
}

// persistPinnedKinds rewrites the config's pinned state to mirror
// the sidebar's current PinnedKinds order, then saves atomically.
// The sidebar is the in-memory source of truth for kinds the
// registry knows about; this flushes the diff out to disk so a
// restart restores the same order.
//
// Critical invariant: pins for unregistered kinds (CRDs that
// disappeared mid-session, or were never installed when kbu started
// but are listed in config) MUST survive this rewrite. The
// ResourceKindConfigEntry contract is "Unknown kinds at load time
// stay in the map but are dropped from the sidebar — the entry is
// preserved so a re-install of the CRD silently restores the user's
// pin / sort." A naive "wipe all + re-add from sidebar" defeats
// that, since unregistered kinds were never in the sidebar to be
// re-added.
//
// Strategy: only clear pin entries for kinds the registry currently
// knows about (i.e. those the sidebar manages); leave everything
// else untouched. The unregistered kind keeps its Order value, so
// when its CRD comes back it slots into its original relative
// position.
func (m *AppModel) persistPinnedKinds() error {
	if m.cfg == nil {
		return nil
	}
	reg := m.k8sClient.Registry()
	knownKubectl := make(map[string]struct{})
	for _, rt := range reg.AllTypes() {
		if def := reg.Get(rt); def != nil {
			knownKubectl[def.KubectlName] = struct{}{}
		}
	}
	for _, kind := range m.cfg.PinnedOrdered() {
		if _, ok := knownKubectl[kind]; ok {
			m.cfg.UnsetPinned(kind)
		}
	}
	for i, rt := range m.sidebar.PinnedKinds() {
		def := reg.Get(rt)
		if def == nil {
			continue
		}
		m.cfg.SetPinned(def.KubectlName, (i+1)*10)
	}
	return m.cfg.Save()
}

// syncCompareLockToTable re-resolves the locked UID into a row index
// against the CURRENT items slice and pushes it to the TableModel.
// Called after any path that changes items (watcher update) or the
// lock itself (set / clear). -1 when not in compare mode or the locked
// UID isn't in the current items (the dropCompareLockIfMissing path
// usually catches this first, but the index helper stays defensive).
func (m *AppModel) syncCompareLockToTable() {
	if !m.inCompareMode() {
		m.table.SetLockedRow(-1)
		return
	}
	for i, it := range m.items {
		if it.UID == m.compareLock.uid {
			m.table.SetLockedRow(i)
			return
		}
	}
	m.table.SetLockedRow(-1)
}

// saveSessionState snapshots the user's current cursor position
// (context / namespace / kind / row) to state.yaml so the next launch
// can restore it. Best-effort: a write failure does NOT block the
// quit path — worst case the user's next launch starts from stale
// state, which the load-side fallback handles cleanly.
//
// Drill-down state is intentionally NOT preserved. The recorded kind
// is m.currentResource, which for a drilled-down view is the CHILD
// kind (Pods inside a Deployment drill); next launch subscribes to
// that kind at top level rather than trying to reconstruct the drill
// chain. If the recorded object still exists next launch it lights up
// in the top-level view.
func (m *AppModel) saveSessionState() {
	s := buildSessionState(
		m.k8sClient.ContextName(),
		m.k8sClient.Selection(),
		m.currentResource,
		m.items,
		m.table.SelectedRow(),
	)
	s.Panel = panelToStateString(m.activePanel)
	s.Tab = m.detail.ActiveTabName()
	if err := s.Save(); err != nil {
		m.appLog.Warn(fmt.Sprintf("state: save on quit failed (%v) — next launch starts from defaults", err))
	}
}

// panelToStateString + panelFromStateString translate between the
// runtime Panel enum and the yaml-friendly string form used in
// state.yaml. Keeping the mapping in one place so a future panel
// reorder (or an added panel) doesn't silently corrupt saved state.
// Unknown / empty strings fall back to SidebarPanel — safest default
// since the sidebar is the natural launch focus.
func panelToStateString(p Panel) string {
	switch p {
	case TablePanel:
		return "table"
	case DetailPanel:
		return "detail"
	default:
		return "sidebar"
	}
}

func panelFromStateString(s string) Panel {
	switch s {
	case "table":
		return TablePanel
	case "detail":
		return DetailPanel
	default:
		return SidebarPanel
	}
}

// buildSessionState turns the app's current cursor position into a
// State struct ready for persistence. Split out from saveSessionState
// so tests can drive it without a real k8s.Client — the I/O half
// (state.yaml write, error surfacing) is covered by state_test.go,
// this function covers the projection from AppModel fields into the
// yaml shape. `cursor` follows m.table.SelectedRow() semantics:
// negative or out-of-range means "no row selected", in which case
// ObjectName / ObjectNamespace stay empty.
func buildSessionState(ctx string, sel k8s.NamespaceSelection, rt k8s.ResourceType, items []k8s.ResourceItem, cursor int) *config.State {
	s := &config.State{
		Context:    ctx,
		Namespaces: sel.List(), // nil for All → omitted from the yaml
		Kind:       rt.KubectlName(),
	}
	if cursor >= 0 && cursor < len(items) {
		s.ObjectNamespace = items[cursor].Namespace
		s.ObjectName = items[cursor].Name
	}
	return s
}

// honorPendingTableSelect snaps the table cursor onto the requested
// name+namespace when a ResourceDataMsg for the matching kind arrives,
// then clears the pending pointer. If the target isn't in the result
// set (different namespace scope, drifted away, ...), the cursor stays
// at its current position; pending still clears so we don't keep
// hunting on every subsequent watcher tick. Split out so tests can
// drive just this slice without the surrounding watcher plumbing.
func (m *AppModel) honorPendingTableSelect(kind k8s.ResourceType, items []k8s.ResourceItem) {
	if m.pendingTableSelect == nil || m.pendingTableSelect.Type != kind {
		return
	}
	for i, item := range items {
		if item.Name == m.pendingTableSelect.Name && item.Namespace == m.pendingTableSelect.Namespace {
			m.table.SetCursor(i)
			break
		}
	}
	m.pendingTableSelect = nil
}

type drillDownEntry struct {
	parentType  k8s.ResourceType
	parentName  string
	parentItems []k8s.ResourceItem
}

// parseCompareLayout maps a config-file string into the typed enum.
// Empty / unknown values fall back to Unified — diff readers grok
// `-`/`+` markers immediately and the unified form survives narrow
// panels without column-wrapping artefacts. Split is opt-in via
// config.
func parseCompareLayout(s string) CompareLayout {
	if s == "split" {
		return CompareLayoutSplit
	}
	return CompareLayoutUnified
}

func NewAppModel(t *theme.Theme, client *k8s.Client, cfg *config.Config, state *config.State, stateLoadErr error, configMigrationNotice string) AppModel {
	info := client.GetClusterInfo()

	sidebar := NewSidebarModel(t)
	sidebar.SetFocused(true)
	// Resolve pinned kind strings from config into registered
	// ResourceTypes, preserving the user's chosen Order. Entries that
	// no longer map to a registered kind (CRD uninstalled, etc.) are
	// SKIPPED for sidebar rendering but stay in the config — a
	// re-install of the CRD silently restores the pin.
	if cfg != nil {
		ordered := cfg.PinnedOrdered()
		resolved := make([]k8s.ResourceType, 0, len(ordered))
		for _, kind := range ordered {
			if rt := client.Registry().LookupByKubectlName(kind); rt != "" {
				resolved = append(resolved, rt)
			}
		}
		sidebar.SetPinned(resolved)
	}

	watcher := k8s.NewWatcher(client.Clientset())
	logStreamer := k8s.NewLogStreamer(client.Clientset())

	detail := NewDetailModel(t)
	detail.SetResourceType(k8s.ResourcePods)

	newCompareModel := NewCompareYamlPopupModel(t)
	newCompareModel.SetDefaultLayout(parseCompareLayout(cfg.Compare.Layout))

	// Build appLog up-front so we can surface startup notices — chiefly
	// a $KBU__CONFIGPATH override warning so the user knows pin / sort
	// persistence will land at the override path, not the default
	// config dir. Without this nudge, a leftover env var from a debug
	// session silently writes the user's mutations to /tmp and the
	// next session sees pristine config; the user assumes pins were
	// lost. `!` opens the App Log popup where this message lives.
	//
	// v2.0 rename: prefer $KBU__CONFIGPATH; $KM8__CONFIGPATH still read
	// for one release as a fallback (see EnvDeprecations for the nudge).
	appLog := NewAppLogModel(t)
	overridePath := strings.TrimSpace(os.Getenv("KBU__CONFIGPATH"))
	overrideName := "$KBU__CONFIGPATH"
	if overridePath == "" {
		if legacy := strings.TrimSpace(os.Getenv("KM8__CONFIGPATH")); legacy != "" {
			overridePath = legacy
			overrideName = "$KM8__CONFIGPATH"
		}
	}
	if overridePath != "" {
		// Use Info (not Warn) — Warn increments errorCount and arms
		// the status bar's red `! N errors` badge every launch with
		// the env legitimately set. The point is a discoverable
		// nudge in the App Log popup (`!`), not a recurring error
		// signal for a setup the user chose.
		appLog.Info(fmt.Sprintf("config: %s=%s — loads AND saves redirected here, not the default config dir", overrideName, overridePath))
	}
	// v2.0 rename: main.go called config.MigrateLegacyConfigDir before
	// loading the config; any resulting warning is threaded in here so
	// the user notices in `!` that a one-shot migration happened.
	if configMigrationNotice != "" {
		appLog.Info(configMigrationNotice)
	}
	// v1.7.5 KM8erm → Alterm rename: surface any deprecation warnings
	// the config loader collected (legacy yaml keys present) and any
	// deprecated env vars still set. Warn-level so the user notices in
	// the `!` popup; removable next release when the transition ends.
	//
	// For config keys we ALSO trigger an immediate Save() to rewrite
	// the file with the new keys (dropping the legacy ones) so the
	// warning only fires once — this session. Next launch finds the
	// new keys and stays silent. Best-effort: if Save fails (perms,
	// disk full) the warning will recur next launch, which is correct
	// behavior — the user needs to know migration didn't persist.
	//
	// Env-var warnings keep recurring per launch — they live in the
	// user's shell rc / launchctl plist and kbu can't (and shouldn't)
	// rewrite those. Persistent nudge is appropriate until the user
	// updates their env.
	for _, w := range cfg.DeprecationWarnings {
		appLog.Warn(w)
	}
	if len(cfg.DeprecationWarnings) > 0 {
		// Back up the original file before cfg.Save() rewrites it. The
		// rewrite goes through yaml.Marshal which can't preserve user-
		// added comments or unknown yaml keys — the backup is the
		// escape hatch for power users who hand-edited their config.
		// File suffix carries the kbu release tag (dots → underscores
		// so the suffix sorts cleanly and doesn't confuse path tools).
		// If backup fails we ABORT the Save — losing the user's custom
		// content would be worse than recurring the warning next launch.
		src := config.ConfigPath()
		versTag := strings.ReplaceAll(version.Version, ".", "_")
		bak, bakErr := config.BackupBeforeMigration(src, versTag)
		if bakErr != nil {
			appLog.Warn(fmt.Sprintf("config: backup before migration failed (%v) — Save aborted, warnings will recur next launch", bakErr))
		} else {
			if err := cfg.Save(); err != nil {
				appLog.Warn(fmt.Sprintf("config: failed to rewrite legacy keys this session — warnings will recur next launch (%v)", err))
			} else {
				appLog.Info(fmt.Sprintf("config: original kept at %s before legacy-key migration", bak))
			}
		}
		// Clear so subsequent reads of cfg.DeprecationWarnings (none
		// expected — NewAppModel constructs once per run) don't double-
		// surface the same message.
		cfg.DeprecationWarnings = nil
	}
	// v2.0 km8 → kbu rename: surface any legacy $KM8__* env vars still
	// set. Fires per launch until the user renames them in their shell
	// rc / launchctl plist. The current tier of KM8__ vars (CONFIGPATH,
	// STATEPATH, ALTERM_SHELL, ALTERM_LOGIN_SHELL) is fallback-read this
	// release; scheduled for removal in v2.1.
	//
	// The pre-v2.0 KM8__SHELL / KM8__LOGIN_SHELL tier (deprecated since
	// v1.7.5 in favor of KM8__ALTERM_*) is fully removed in v2.0 — the
	// grace period spanned multiple minor releases.
	for _, w := range config.EnvDeprecations() {
		appLog.Warn(w)
	}

	// Session state resolution. main.go already applied state.Context /
	// state.Namespace before constructing us (both are load-time actions
	// on the k8s client). What remains is the Kind + object cursor: the
	// initial resource kind + the row to select once its ResourceDataMsg
	// arrives. When state came in bad (stateLoadErr non-nil) OR the
	// recorded kind is no longer registered (CRD uninstalled, etc.), we
	// fall back to Pods and INFO the applog so the user notices in `!`.
	//
	// pendingTableSelect ties into the existing Relatives-jump machinery
	// (honorPendingTableSelect fires on ResourceDataMsg): the field is
	// consumed as soon as the first watcher tick lands with a matching
	// item, or dropped silently if the target row isn't in the result
	// set. That "silently dropped" case corresponds to "the recorded
	// object is gone" — we surface an INFO after a short delay only if
	// we can prove it (we don't want to alarm on a slow first tick).
	initialResource := k8s.ResourcePods
	var pendingSelect *k8s.RefTarget
	if stateLoadErr != nil {
		appLog.Warn(fmt.Sprintf("state: failed to load %s (%v) — starting from defaults", config.StatePath(), stateLoadErr))
	}
	if state != nil && state.Kind != "" {
		if rt := client.Registry().LookupByKubectlName(state.Kind); rt != "" {
			initialResource = rt
			if state.ObjectName != "" {
				pendingSelect = &k8s.RefTarget{
					Type:      rt,
					Name:      state.ObjectName,
					Namespace: state.ObjectNamespace,
				}
			}
		} else {
			// Registry lookup miss at startup can also mean "the kind is
			// a CRD not yet discovered" — DiscoverCRDs runs async. Treat
			// as a fallback but don't shout too loudly; if the user's
			// last kind IS a CRD they'll notice the Pods default and the
			// applog line has the context.
			appLog.Info(fmt.Sprintf("state: recorded kind %q not in registry, falling back to Pods", state.Kind))
		}
	}
	sidebar.SetSelected(initialResource)
	detail.SetResourceType(initialResource)

	// Restore the recorded Panel 3 tab. SetResourceType above rebuilt
	// the tab list for initialResource, so the lookup runs against
	// the correct set. A stale tab name (kind switched to one that
	// doesn't have the recorded tab — e.g. was on "Events" for a
	// Pod, now on ConfigMaps which has no Events tab) silently falls
	// back to the current first-tab default.
	if state != nil && state.Tab != "" {
		detail.SwitchToTabByName(state.Tab)
	}

	// Sync the status bar with whatever namespace main.go applied to
	// the client (either state.Namespace or cfg.DefaultNamespace).
	// Without this, NewStatusBarModel defaults to "All Namespaces"
	// even though the client is scoped to a specific ns, and the
	// mismatch persists until the user opens the ns picker or hits a
	// path that fires SetNamespace (e.g. NamespaceChangedMsg).
	statusBar := NewStatusBarModel(t, info)
	statusBar.SetNamespace(namespaceSelectionLabel(client.Selection()))

	// Restore the focused panel from state. Defaults to SidebarPanel
	// when state is empty / unknown — same as the pre-restore behavior.
	// Focus flags for the three panel models must all be reset here
	// so unfocused ones lose the "focused=true" default that sidebar
	// carries from line 1170 (NewSidebarModel + SetFocused(true) is
	// hardcoded for the fresh-launch path).
	table := NewTableModel(t)
	initialPanel := SidebarPanel
	if state != nil {
		initialPanel = panelFromStateString(state.Panel)
	}
	sidebar.SetFocused(initialPanel == SidebarPanel)
	table.SetFocused(initialPanel == TablePanel)
	detail.SetFocused(initialPanel == DetailPanel)

	return AppModel{
		sidebar:            sidebar,
		table:              table,
		detail:             detail,
		statusBar:          statusBar,
		statusLine:         NewStatusLineModel(t),
		namespacePicker:    NewNamespacePickerModel(t),
		contextPicker:      NewContextPickerModel(t),
		help:               NewHelpModel(t),
		appLog:             appLog,
		confirm:            NewConfirmModel(t),
		splash:             NewSplashModel(),
		toast:              NewToastModel(t),
		shellPty:           NewPtyView("ptyview_shell"),
		txPty:              NewPtyView("ptyview_tx"),
		yamlPopup:          NewYamlPopupModel(t),
		comparePopup:       newCompareModel,
		breadcrumbPopup:    NewBreadcrumbPopupModel(t),
		spaceMenu:          NewSpaceMenuModel(t),
		globalMenu:         NewGlobalMenuModel(t),
		listPicker:         NewListPickerModel(t),
		sortDirPicker:      NewSortDirPickerModel(t),
		settingsPopup:      NewSettingsPopupModel(t),
		activePanel:        initialPanel,
		theme:              t,
		cfg:                cfg,
		cfgEditor:          cfg.Editor,
		k8sClient:          client,
		watcher:            watcher,
		logStreamer:        logStreamer,
		currentResource:    initialResource,
		pendingTableSelect: pendingSelect,
	}
}

type appInitMsg struct{ info k8s.ClusterInfo }

func (m AppModel) Init() tea.Cmd {
	m.watcher.Start(m.currentResource, m.k8sClient.Selection())
	info := m.k8sClient.GetClusterInfo()
	cmds := []tea.Cmd{
		m.sidebar.Init(),
		m.table.Init(),
		waitForWatchUpdate(m.watcher, m.currentResource),
		discoverCRDs(m.k8sClient),
		func() tea.Msg { return appInitMsg{info: info} },
	}
	// A persisted multi/single-namespace selection may reference
	// namespaces that no longer exist — reconcile it against the live
	// list once at startup (drop the dead ones, fall back to all if none
	// survive). All-namespaces needs no check.
	if !m.k8sClient.Selection().IsAll() {
		cmds = append(cmds, validateNamespaceSelection(m.k8sClient))
	}
	return tea.Batch(cmds...)
}

func discoverCRDs(client *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				config.WriteCrashLog(r)
			}
		}()
		count, err := k8s.DiscoverCRDs(context.Background(), client)
		return CRDsDiscoveredMsg{Count: count, Err: err}
	}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if _, ok := msg.(quitMsg); ok {
		m.watcher.Stop()
		m.logStreamer.Stop()
		// Kill any persistent PTY (hidden Alterm shell, mid-edit, mid-exec)
		// so we don't orphan the subprocess after kbu exits.
		if m.shellPty != nil && m.shellPty.IsAlive() {
			m.shellPty.Stop()
		}
		if m.txPty != nil && m.txPty.IsAlive() {
			m.txPty.Stop()
		}
		// Session state snapshot on quit. Best-effort: a Save failure
		// (perms, disk full, read-only fs) must not block the exit —
		// worst case is next launch reads a slightly stale state file,
		// which is exactly the fallback path is designed for. Written
		// synchronously here because tea.Quit runs post-return and we
		// need the write to land before the process teardown starts.
		m.saveSessionState()
		return m, tea.Quit
	}

	// The exit key in a kubectl edit / exec PTY (tdp K10): ask before
	// ending the session — an edit in progress would be lost. The
	// confirm stacks over the PTY; Esc returns to it.
	if req, ok := msg.(ptyLeaveRequestMsg); ok {
		action, question, detail := ConfirmEndShell, "End the shell session?", "the kubectl exec session is closed"
		switch req.kind {
		case PtyKindEdit:
			action, question, detail = ConfirmLeaveEdit, "Leave kubectl edit?", "the edit is cancelled; nothing is applied"
		case PtyKindShell:
			question, detail = "End the Alterm shell?", "the shell and anything still running in it stop"
		}
		kind := req.kind
		onConfirm := func() tea.Msg { return ptyKillMsg{kind: kind} }
		m.confirm.SetSize(m.width, m.height)
		m.confirm.SetLayer(m.popupDepth() + 1)
		return m, m.confirm.Show(action, question, detail, onConfirm)
	}
	if kill, ok := msg.(ptyKillMsg); ok {
		if kill.kind == PtyKindShell {
			m.shellPty.Kill()
		} else {
			m.txPty.Kill()
		}
		return m, nil
	}

	// PtyExitMsg arrives AFTER ptyView has already Stop()ed itself, so this
	// handler lives outside the IsActive() guard — it cleans up app-level
	// state when the subprocess finishes.
	if exit, ok := msg.(PtyExitMsg); ok {
		// Only clear the editing flag when the Edit slot exited — Shell /
		// Exec exits don't touch it. With dual-slot routing in place, an
		// exec exit while edit is alive (or vice-versa) shouldn't drop
		// the unrelated state.
		if exit.Kind == PtyKindEdit {
			m.editing = false
		}
		if exit.ExitCode != 0 {
			m.appLog.Warn(fmt.Sprintf("subprocess exited with code %d", exit.ExitCode))
		}
		return m, nil
	}

	if m.splash.IsActive() {
		var cmd tea.Cmd
		m.splash, cmd = m.splash.Update(msg)
		return m, cmd
	}

	if tickMsg, ok := msg.(AnimTickMsg); ok {
		var animCmds []tea.Cmd
		if c := m.help.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.appLog.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.confirm.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.contextPicker.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.namespacePicker.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.yamlPopup.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.comparePopup.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.breadcrumbPopup.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.spaceMenu.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.globalMenu.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.listPicker.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.sortDirPicker.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.settingsPopup.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.toast.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.shellPty.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.txPty.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		return m, tea.Batch(animCmds...)
	}

	// PTY intercepts keys / ticks / resizes while a subprocess is running.
	// Dual-slot routing rules:
	//   - WindowSizeMsg: both slots get the new size; visible popup short-
	//     circuits (no fall-through to underlying panels).
	//   - ptyTickMsg: dispatch to whichever slot is alive (each PtyView's
	//     tick is idempotent: it polls only its own done flag).
	//   - tea.KeyMsg: txPty wins over shellPty (transient on top). If
	//     neither has a visible popup, keys fall through to top-level
	//     routing — Alterm-hidden keeps the shell alive in background.
	anyAlive := m.shellPty.IsAlive() || m.txPty.IsAlive()
	if anyAlive {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.ready = true
			m.layout()
			m.shellPty.SetSize(m.width, m.height)
			m.txPty.SetSize(m.width, m.height)
			if m.txPty.IsActive() || m.shellPty.IsActive() {
				return m, nil
			}
			// Both hidden / inactive: fall through so underlying panels
			// also see the new size.
		case ptyTickMsg:
			// Route each tick to ONLY the slot whose Kind it carries.
			// Double-dispatch caused an exponential tick explosion
			// (each slot returned a new tick cmd, both got re-dispatched
			// next cycle → 2× per tick → visible input lag within seconds).
			tickMsg := msg
			if tickMsg.kind == PtyKindShell {
				if m.shellPty.IsAlive() {
					var c tea.Cmd
					m.shellPty, c = m.shellPty.Update(msg)
					return m, c
				}
				return m, nil
			}
			// PtyKindEdit / PtyKindExec → txPty
			if m.txPty.IsAlive() {
				var c tea.Cmd
				m.txPty, c = m.txPty.Update(msg)
				return m, c
			}
			return m, nil
		}
	}

	// Keys go to the popup on top of the stack — the last layer in
	// stackOrder that owns its place. A popup running its close
	// animation no longer owns it, so the key falls through to the
	// layer beneath instead of being swallowed (tdp F3, D3).
	if k, ok := msg.(tea.KeyMsg); ok {
		top := m.topLayer()
		// A PTY on top gets every key, q and Ctrl+C included (tdp
		// K10); with Alterm hidden, keys fall through to the panels
		// (Alt+t shows it again).
		if _, isPty := top.(*PtyView); isPty {
			return m, top.key(k)
		}
		// tdp K9: q and Ctrl+C are one leave flow, on every surface
		// — a popup, a menu, a mode. Ctrl+C works even while typing;
		// q is a character there (K8).
		switch k.String() {
		case "ctrl+c":
			return m, quitCmd
		case "q":
			if !m.typing() {
				return m, quitCmd
			}
		case "?":
			// tdp K6: ? answers on every surface — a panel, a menu, a
			// popup, a mode — with the key reference of whatever is
			// frontmost, stacked on top of it. On the key reference
			// itself it closes it; while typing it is a character (K8).
			if !m.typing() && !m.help.owns() {
				return m, m.openKeyRef()
			}
		}
		// tdp K4, F3: Esc closes the top popup first, and a toast
		// counts — it is drawn over everything, so it goes before any
		// popup, mode or search beneath it. Toasts never take keys
		// otherwise (F1). One already fading no longer owns Esc: the
		// press goes on to what is under it.
		if k.String() == "esc" && m.toast.Owns() {
			return m, m.toast.Dismiss()
		}
		// tdp K11: in the YAML viewer's selection mode Tab is
		// suspended, but answers.
		if top == &m.yamlPopup && m.yamlPopup.visualMode && k.String() == "tab" {
			return m, m.toast.Show("[Esc] leaves the selection first")
		}
		if top != nil {
			if !top.ready() {
				return m, nil
			}
			return m, top.key(k)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case tea.MouseMsg:
		// Sidebar drag mode: ANY mouse event (click, motion, wheel)
		// is "anything else," which cancels the drag and reverts.
		// Consume the event — don't propagate to focus shift / row
		// selection / popup hit-test, mirroring the same intent as
		// the keyboard cancel path. (A popup over the drag — the
		// key reference — takes its own clicks.)
		if m.activePanel == SidebarPanel && m.sidebar.IsDragging() && m.topLayer() == nil {
			cmd := m.sidebar.CancelDrag()
			return m, cmd
		}
		// Mouse routing, top of the stack first — the same topLayer()
		// the keys go to, so a click lands on the popup the user can
		// see on top, never on a menu row hidden beneath it (tdp X2):
		//   1. MouseEnabled gate — short-circuit when off, EXCEPT
		//      for the Settings popup itself. Users who toggle Mouse
		//      OFF would otherwise be locked out of the surface that
		//      toggles it back on; the popup is its own escape hatch.
		//   2. Wheel → synthesize u/d (half-page) for main panels +
		//      viewer popups (yaml / compare / appLog / help) that
		//      bind u/d natively. With a menu-style popup on top
		//      (short lists) the wheel is swallowed: u/d is unbound
		//      there, and ignoring it keeps the wheel from drifting
		//      the cursor of whatever sits beneath. A PTY on top
		//      takes no mouse action at all.
		//   3. A click goes to the top popup's HandleMouse.
		//   4. Otherwise, handleMousePress for the main 3 panels.
		top := m.topLayer()
		if m.cfg != nil && !m.cfg.IsMouseEnabled() {
			if top == &m.settingsPopup {
				top.resize(m.width, m.height)
				return m, top.click(msg, m.width, m.height)
			}
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			if top != nil && (isMenuLayer(top) || !top.ready()) {
				return m, nil
			}
			if _, isPty := top.(*PtyView); isPty {
				return m, nil
			}
			// Wheel translates to half-page move (u / d). u/d are
			// bound across sidebar / table / detail and the viewer
			// popups (yamlpopup / comparepopup / applog / help), so
			// the wheel works wherever the user might land.
			//
			// Direction:
			//   natural (default): wheel-up = scroll content up =
			//                      cursor / view moves toward TOP = 'u'
			//   reverse:           swap, so wheel-up = 'd'
			up, down := 'u', 'd'
			if m.cfg != nil && m.cfg.MouseScrollDirection() == config.MouseScrollReverse {
				up, down = 'd', 'u'
			}
			r := up
			if msg.Button == tea.MouseButtonWheelDown {
				r = down
			}
			return m, func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }
		}
		// A click goes to the popup on top. Each popup's HandleMouse
		// owns its own hit-test (popup rect, row offsets) and decides
		// what a click commits: list popups commit on left-click,
		// scroll-only and dialog popups close on right-click, and
		// left-click is a no-op wherever a stray click could fire a
		// destructive action (confirm especially).
		if top != nil {
			// Hit-test at the size the popup is drawn at (View resizes
			// before it composites).
			top.resize(m.width, m.height)
			return m, top.click(msg, m.width, m.height)
		}
		cmd := m.handleMousePress(msg)
		return m, cmd

	case tea.KeyMsg:
		return m, m.panelKey(msg)

	case RequestSwitchToResourceMsg:
		// Single confirm-gate for both Relatives space and breadcrumb
		// space. On confirm, fire SwitchToResourceMsg which does the
		// actual sidebar + table + drill-chain rearrangement.
		kindLabel := string(msg.Ref.Type)
		if def := k8s.DefaultRegistry.Get(msg.Ref.Type); def != nil {
			kindLabel = strings.TrimSuffix(def.DisplayName, "s")
		}
		detail := fmt.Sprintf("%s/%s", kindLabel, msg.Ref.Name)
		if msg.Ref.Namespace != "" {
			detail += "  namespace: " + msg.Ref.Namespace
		}
		target := msg.Ref
		onConfirm := func() tea.Msg { return SwitchToResourceMsg{Ref: target} }
		m.confirm.SetLayer(m.popupDepth() + 1)
		return m, m.confirm.Show(ConfirmSwitch, "Switch panel 1 + 2 to this resource?", detail, onConfirm)

	case SwitchToResourceMsg:
		// Confirmed Relatives-tab jump-to-this-resource. Update sidebar
		// state synchronously so panel 1 highlight is correct on the
		// next render, then route through the standard ResourceSelected
		// flow (which clears table/detail/drill state, restarts the
		// watcher, and fetches new items). The pendingTableSelect hook
		// then moves the table cursor onto the target row once
		// ResourceDataMsg arrives for the new kind.
		//
		// Clear search filters on all three panels first — a stale
		// sidebar / table / detail filter from the previous selection
		// could hide the new target. Table's ResourceSelectedMsg
		// handler already self-clears, but sidebar + detail don't
		// consume that message, so we reset them explicitly.
		m.sidebar.ClearSearch()
		m.detail.ClearSearch()
		m.sidebar.SetSelected(msg.Ref.Type)
		ref := msg.Ref
		m.pendingTableSelect = &ref
		batch := []tea.Cmd{func() tea.Msg { return ResourceSelectedMsg{Type: ref.Type} }}
		// The switch finishes the flow that led here (Space menu →
		// breadcrumb → confirm): the chain those popups showed is gone
		// once the drill chain resets, so the whole stack closes
		// (tdp T1).
		if c := m.closeAllBlockingPopups(); c != nil {
			batch = append(batch, c)
		}
		return m, tea.Batch(batch...)

	case ResourceSelectedMsg:
		m.appLog.Info("switched to " + msg.Type.String())
		m.currentResource = msg.Type
		m.drillDownStack = nil
		m.drillDownPod = nil
		m.drillDownContainers = nil
		m.logStreamer.Stop()
		m.logsActive = false
		// Throttle is per-target — kind switch invalidates the prior
		// kind's RBAC / pod-existence outcome, so the new kind gets a
		// fresh attempt. Same rationale as RowSelectedMsg's nextAggregateRetry
		// reset; applies to every navigation handler that flips
		// logsActive=false. Repeated in NamespaceChangedMsg,
		// ContextChangedMsg, drillDownMsg, exitDrillDown.
		m.nextAggregateRetry = time.Time{}
		m.watcher.Stop()
		m.detail.ClearDetail()
		m.detail.SetResourceType(msg.Type)
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
		m.syncTableSortIndicator()
		m.switchSeq++
		m.rowSeq++ // invalidate any in-flight rowSwitchTickMsg from the prior kind
		seq := m.switchSeq
		cmds = append(cmds, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
			return resourceSwitchTickMsg{seq: seq}
		}))
		return m, tea.Batch(cmds...)

	case resourceSwitchTickMsg:
		if msg.seq != m.switchSeq {
			return m, nil
		}
		m.watcher.Start(m.currentResource, m.k8sClient.Selection())
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		return m, tea.Batch(cmds...)

	case ResourceDataMsg:
		if msg.Type != m.currentResource {
			return m, nil
		}
		m.items = filterHelmIfHidden(msg.Items, msg.Type)
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		if m.drillDownPod != nil {
			return m, tea.Batch(cmds...)
		}
		// Apply the user's saved sort BEFORE compare-lock resolution
		// and row augmentation. compare lock is tracked by UID so the
		// reorder doesn't break the lookup, but the table row index
		// it locks onto depends on the post-sort positions — sort
		// first, then sync, so the lockedRow points at the right
		// row. Indicator sync is idempotent and cheap, so refreshing
		// it on every data tick guarantees the header stays in lock-
		// step with the saved config even after kind switches that
		// arrive before the first ResourceSelectedMsg-driven sync.
		m.applySortToItems()
		m.syncTableSortIndicator()
		// Compare mode: if the locked baseline has disappeared from the
		// watcher (deleted / renamed / fell out of namespace scope),
		// drop the lock and flash a toast — otherwise the status-bar
		// marker would hang around pointing at a row that no longer
		// exists in panel 2.
		if c := m.dropCompareLockIfMissing(m.items); c != nil {
			cmds = append(cmds, c)
		}
		rows := augmentRowsWithHelm(m.items, msg.Type)
		if msg.Type == k8s.ResourceContexts {
			markCurrentContextRow(rows, m.k8sClient.ContextName())
		}
		m.table.SetRows(rows)
		m.syncCompareLockToTable()
		m.honorPendingTableSelect(msg.Type, m.items)
		if len(m.items) > 0 {
			idx := m.table.SelectedRow()
			if idx >= 0 && idx < len(m.items) {
				item := m.items[idx]
				cmds = append(cmds, fetchResourceDetail(m.k8sClient, msg.Type, item))
				switch {
				case msg.Type == k8s.ResourcePods && !m.logsActive:
					containers := k8s.ContainerNames(item.Raw)
					if len(containers) > 0 {
						m.detail.logLines = nil
						m.logStreamer.Start(item.Name, item.Namespace, containers)
						m.logsActive = true
						cmds = append(cmds, waitForLogLine(m.logStreamer))
					}
				case isAggregateLogsKind(msg.Type) && !m.logsActive && time.Now().After(m.nextAggregateRetry):
					// Workload kinds funnel Pods through k8s.PodsForWorkload
					// into one aggregate stream — see supportsLogs() for the
					// canonical kind set. Flip logsActive=true at dispatch
					// time (not at aggregateLogsReadyMsg arrival) so a
					// follow-up ResourceDataMsg watcher tick in the
					// dispatch-to-Ready gap doesn't re-fire this case and
					// queue a duplicate startAggregateLogs.
					//
					// nextAggregateRetry throttles re-attempts after a
					// failure (zero pods, RBAC denial, transient API
					// error) — without the throttle, every watcher
					// tick would re-fire the same failing call until
					// the user navigated away.
					m.detail.logLines = nil
					m.logsActive = true
					cmds = append(cmds, startAggregateLogs(m.k8sClient, msg.Type, item))
				}
			}
		}
		return m, tea.Batch(cmds...)

	case ResourceErrorMsg:
		if msg.Err != nil {
			m.appLog.Error(msg.Err.Error())
		}
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		return m, tea.Batch(cmds...)

	case RowSelectedMsg:
		// Immediate dispatch: cheap model mutations + Stop the previous
		// stream + lie-lock logsActive=true so any ResourceDataMsg watcher
		// tick in the rowSwitchDebounce window can't see !m.logsActive and
		// queue a duplicate startAggregateLogs / logStreamer.Start. The
		// expensive work (fetchResourceDetail + new stream Start) defers
		// to the rowSwitchTickMsg handler. See rowSwitchTickMsg doc.
		if m.drillDownPod != nil {
			if msg.Index >= 0 && msg.Index < len(m.drillDownContainers) {
				c := m.drillDownContainers[msg.Index]
				detail := containerToDetail(c, *m.drillDownPod)
				m.detail.SetDetail(detail, nil)
				m.logStreamer.Stop()
				m.detail.logLines = nil
				m.logsActive = true
				m.rowSeq++
				seq := m.rowSeq
				container := c
				cmds = append(cmds, tea.Tick(rowSwitchDebounce, func(time.Time) tea.Msg {
					return rowSwitchTickMsg{seq: seq, drillContainer: &container}
				}))
			}
			return m, tea.Batch(cmds...)
		}
		if msg.Index >= 0 && msg.Index < len(m.items) && len(m.table.rows) > 0 {
			item := m.items[msg.Index]
			m.detail.ResetDrillStack()
			m.nextAggregateRetry = time.Time{}
			m.logStreamer.Stop()
			m.detail.logLines = nil
			m.logsActive = true
			m.rowSeq++
			seq := m.rowSeq
			kind := m.currentResource
			cmds = append(cmds, tea.Tick(rowSwitchDebounce, func(time.Time) tea.Msg {
				return rowSwitchTickMsg{seq: seq, kind: kind, item: item}
			}))
		}
		return m, tea.Batch(cmds...)

	case rowSwitchTickMsg:
		// Stale tick: user has navigated since we scheduled this; drop
		// without side effects. Without the seq compare, rapid j/k
		// would fire one fetchResourceDetail + logStreamer.Start per
		// row instead of one for the row the user actually settled on.
		if msg.seq != m.rowSeq {
			return m, nil
		}
		if msg.drillContainer != nil {
			m.logStreamer.Start(m.drillDownPod.Name, m.drillDownPod.Namespace, []string{msg.drillContainer.Name})
			cmds = append(cmds, waitForLogLine(m.logStreamer))
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, fetchResourceDetail(m.k8sClient, msg.kind, msg.item))
		switch {
		case msg.kind == k8s.ResourcePods:
			containers := k8s.ContainerNames(msg.item.Raw)
			if len(containers) > 0 {
				m.logStreamer.Start(msg.item.Name, msg.item.Namespace, containers)
				cmds = append(cmds, waitForLogLine(m.logStreamer))
			} else {
				// Pod row with no containers (impossible in practice
				// but the producer code guards): the immediate-dispatch
				// lie-lock left logsActive=true; flip it back so the
				// state matches reality.
				m.logsActive = false
			}
		case isAggregateLogsKind(msg.kind):
			cmds = append(cmds, startAggregateLogs(m.k8sClient, msg.kind, msg.item))
		default:
			m.logsActive = false
		}
		return m, tea.Batch(cmds...)

	case RelativeDrillMsg:
		// User pressed Y on a drillable Relatives entry. Fetch the target
		// resource off the Update path and open its YAML in a popup.
		ref := msg.Ref
		client := m.k8sClient
		cmd := func() tea.Msg {
			item, err := k8s.FetchResourceByRef(context.Background(), client.Clientset(), ref)
			if err != nil {
				return resourceFetchedForDrillMsg{ref: ref, err: err}
			}
			yaml := k8s.MarshalItemYAML(item)
			return resourceFetchedForDrillMsg{ref: ref, item: item, yaml: yaml}
		}
		return m, cmd

	case RelativePushMsg:
		// User pressed Enter / l on a drillable entry. Cycle-check
		// against the existing chain (kind+ns+name — k8s makes this
		// triple unique within a kind so it's effectively UID-equivalent
		// without needing the fetch first), then dispatch the drill
		// fetch. Stale guard: sourceUID lets the result-handler drop
		// fetches whose source row has changed.
		sourceUID := m.currentItemUID()
		if sourceUID == "" {
			return m, nil
		}
		for _, existing := range m.detail.DrillChain() {
			if existing.Type == msg.Ref.Type && existing.Name == msg.Ref.Name && existing.Namespace == msg.Ref.Namespace {
				return m, m.toast.ShowWarn(fmt.Sprintf("cycle blocked: %s/%s already in chain", msg.Ref.Type, msg.Ref.Name))
			}
		}
		ref := msg.Ref
		client := m.k8sClient
		fetchCmd := func() tea.Msg {
			ctx := context.Background()
			item, err := k8s.FetchResourceByRef(ctx, client.Clientset(), ref)
			if err != nil {
				return relativeDrillFetchedMsg{ref: ref, sourceUID: sourceUID, err: err}
			}
			detail := k8s.GetResourceDetail(ref.Type, item)
			detail.YAML = k8s.MarshalItemYAML(item)
			k8s.EnrichRelatives(ctx, client.Clientset(), ref.Type, item, &detail)
			return relativeDrillFetchedMsg{ref: ref, sourceUID: sourceUID, item: item, detail: detail}
		}
		return m, fetchCmd

	case relativeDrillFetchedMsg:
		if msg.sourceUID != m.currentItemUID() {
			return m, nil // user moved on
		}
		if msg.err != nil {
			m.appLog.Warn(fmt.Sprintf("drill push %s/%s: %s", msg.ref.Type, msg.ref.Name, msg.err.Error()))
			return m, m.toast.ShowWarn(fmt.Sprintf("drill failed: %s", msg.err.Error()))
		}
		m.detail.PushDrillFrame(msg.ref, msg.item, msg.detail)
		return m, nil

	case RelativeJumpMsg:
		m.detail.JumpToDrillLevel(msg.Level)
		return m, nil

	case resourceFetchedForDrillMsg:
		if msg.err != nil {
			m.appLog.Warn(fmt.Sprintf("drill %s/%s: %s", msg.ref.Type, msg.ref.Name, msg.err.Error()))
			return m, m.toast.ShowWarn("Drill failed — see App Log [!]")
		}
		if msg.yaml == "" {
			m.appLog.Warn(fmt.Sprintf("drill %s/%s: no YAML", msg.ref.Type, msg.ref.Name))
			return m, nil
		}
		m.yamlPopup.SetSize(m.width, m.height)
		m.yamlPopup.SetLayer(m.popupDepth() + 1)
		return m, m.yamlPopup.Open(msg.yaml, msg.ref.Type, msg.item)

	case aggregateLogsReadyMsg:
		// Stale result guard: user may have navigated to a different row
		// while the pod-list call was in flight.
		if msg.resource != m.currentResource {
			return m, nil
		}
		idx := m.table.SelectedRow()
		if idx < 0 || idx >= len(m.items) || m.items[idx].UID != msg.itemUID {
			return m, nil
		}
		if msg.err != nil {
			m.appLog.Warn("aggregate logs: " + msg.err.Error())
			// Reset logsActive + arm the retry throttle. Without
			// throttling, dispatch-time logsActive=true would stay
			// HIGH on failure (blocking the watcher gate forever),
			// and a naive reset to false would let every watcher
			// tick re-fire the same failing call — RBAC-denied
			// rows would spam one warning + one API call per tick.
			m.logsActive = false
			m.nextAggregateRetry = time.Now().Add(aggregateLogsRetryInterval)
			return m, nil
		}
		if len(msg.targets) == 0 {
			m.appLog.Info("aggregate logs: no pods running")
			// Same retry throttle as the err branch — pods may
			// legitimately appear later (mid-rollout) but we don't
			// want to re-list on every watcher tick until they do.
			m.logsActive = false
			m.nextAggregateRetry = time.Now().Add(aggregateLogsRetryInterval)
			return m, nil
		}
		m.logStreamer.StartMulti(msg.targets)
		m.logsActive = true
		return m, waitForLogLine(m.logStreamer)

	case LogLineMsg:
		// Stream-epoch guard: a LogLine in the closed prior stream's
		// buffered residue can still wake the parked reader after Stop
		// closed the channel (Go closed-buffered-channel semantics).
		// Without this check, that residue would AppendLogLine into the
		// new context's logLines buffer — visible as 1-2 stale lines
		// from the previous pod/workload bleeding into the new row's
		// Logs tab on rapid j/k through aggregate kinds. The producer
		// captured its streamID at goroutine spawn; we drop msgs whose
		// epoch doesn't match the streamer's current one.
		if msg.StreamID != m.logStreamer.CurrentStreamID() {
			return m, nil
		}
		m.detail.AppendLogLine(msg.Pod, msg.Container, msg.Text)
		if m.logsActive {
			cmds = append(cmds, waitForLogLine(m.logStreamer))
		}
		return m, tea.Batch(cmds...)

	case ResourceDetailMsg:
		// Drop stale results — a fetch that finished after the user moved
		// on to a different row would otherwise overwrite the right detail
		// with the wrong one. Critical for kinds whose EnrichRelatives does a
		// cluster-wide List (ClusterRole / StorageClass / IngressClass),
		// where latency easily lets order get scrambled. Also drops when
		// currentItemUID is empty (namespace/context change cleared the
		// selection between dispatch and reply).
		if msg.ItemUID == "" || msg.ItemUID != m.currentItemUID() {
			return m, nil
		}
		m.detail.SetDetail(msg.Detail, msg.Events)
		return m, nil

	case NamespaceListMsg:
		// Fetch failed — pull the picker out of its loading state
		// rather than leaving "Loading…" sticky. Toast surfaces the
		// reason so the user knows it wasn't just slow.
		if msg.Err != nil {
			m.appLog.Error("namespace fetch: " + msg.Err.Error())
			closeCmd := m.namespacePicker.Close()
			return m, tea.Batch(closeCmd, m.toast.Show("namespace fetch failed"))
		}
		// Picker was opened in loading state by the N keypress; just
		// swap in the real list. If the user dismissed before this
		// landed, SetNamespaces is a harmless state poke.
		m.namespacePicker.SetNamespaces(msg.Namespaces)
		return m, nil

	case namespaceValidationMsg:
		if msg.Err != nil {
			// Couldn't list namespaces — keep the optimistic selection
			// rather than wiping a valid one on a transient error.
			m.appLog.Warn("namespace validation skipped: " + msg.Err.Error())
			return m, nil
		}
		sel := m.k8sClient.Selection()
		validated := sel.FilterExisting(msg.Namespaces)
		if validated.Equal(sel) {
			return m, nil // every persisted namespace still exists
		}
		if validated.IsAll() {
			m.appLog.Info("saved namespaces no longer exist — showing all namespaces")
		} else {
			m.appLog.Info("dropped deleted namespaces from the saved selection")
		}
		m.k8sClient.SetSelection(validated)
		m.statusBar.SetNamespace(namespaceSelectionLabel(validated))
		m.watcher.Start(m.currentResource, validated)
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		return m, tea.Batch(cmds...)

	case NamespaceChangedMsg:
		m.k8sClient.SetSelection(msg.Selection)
		label := namespaceSelectionLabel(msg.Selection)
		logLabel := label
		if logLabel == "" {
			logLabel = "All Namespaces"
		}
		m.appLog.Info("namespace selection → " + logLabel)
		m.statusBar.SetNamespace(label)
		m.logStreamer.Stop()
		m.logsActive = false
		m.nextAggregateRetry = time.Time{} // per-target throttle; see ResourceSelectedMsg
		m.rowSeq++                         // invalidate any in-flight rowSwitchTickMsg from the prior namespace
		m.detail.ClearDetail()
		m.watcher.Start(m.currentResource, m.k8sClient.Selection())
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		return m, tea.Batch(cmds...)

	case ContextListMsg:
		m.contextPicker.SetLayer(m.popupDepth() + 1)
		return m, m.contextPicker.Open(msg.Contexts, msg.Current)

	case ContextChangedMsg:
		newClient, err := k8s.NewClient(msg.Context)
		if err != nil {
			m.appLog.Error("context switch failed: " + err.Error())
			return m, nil
		}
		m.appLog.Info("context switched to " + msg.Context)
		// Picking a context finishes the flow that opened the picker
		// (Space menu → global operation popup → picker): every popup
		// still up described the old cluster (tdp T1).
		if c := m.closeAllBlockingPopups(); c != nil {
			cmds = append(cmds, c)
		}
		m.watcher.Stop()
		m.logStreamer.Stop()
		m.logsActive = false
		m.nextAggregateRetry = time.Time{} // per-target throttle; see ResourceSelectedMsg
		m.rowSeq++                         // invalidate any in-flight rowSwitchTickMsg from the prior context
		newClient.Registry().ClearDynamic()
		m.k8sClient = newClient
		m.watcher = k8s.NewWatcher(newClient.Clientset())
		m.logStreamer = k8s.NewLogStreamer(newClient.Clientset())
		info := newClient.GetClusterInfo()
		m.statusBar.SetClusterInfo(info)
		m.statusBar.SetNamespace("")
		m.detail.ClearDetail()
		m.items = nil
		m.table.SetRows(nil)
		if m.currentResource.SupportsDrillDown() || k8s.DefaultRegistry.Get(m.currentResource) == nil {
			m.currentResource = k8s.ResourcePods
		}
		m.sidebar.RefreshCategories(newClient.Registry())
		m.watcher.Start(m.currentResource, m.k8sClient.Selection())
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		cmds = append(cmds, discoverCRDs(newClient))
		return m, tea.Batch(cmds...)

	case yamlEditRequestMsg:
		// E in the YAML viewer: the panel's Edit, confirm and all.
		return m, m.confirmEdit(msg.resource, msg.item)

	case CompareLayoutChangedMsg:
		// L in the Compare popup: the layout it switched to becomes
		// compare.layout in config.yaml, so the next start opens with it.
		if m.cfg == nil {
			return m, nil
		}
		m.cfg.Compare.Layout = msg.Layout.String()
		if err := m.cfg.Save(); err != nil {
			m.appLog.Error("compare layout save failed: " + err.Error())
			return m, m.toast.Show("compare layout save failed")
		}
		return m, nil

	case clearStackMsg:
		// An accepted Delete / Rollback finished the flow: the Space
		// menu and anything else under the confirm pointed at the
		// thing just deleted or rolled back (tdp T1).
		return m, m.closeAllBlockingPopups()

	case drillDownFailedMsg:
		if msg.err != nil {
			m.appLog.Warn(fmt.Sprintf("drill %s/%s: %s", msg.parentType.KubectlName(), msg.parentName, msg.err.Error()))
			return m, m.toast.ShowWarn("drill failed: " + msg.err.Error())
		}
		what := "children"
		if msg.childType != "" {
			what = msg.childType.String()
		}
		return m, m.toast.Show("no " + what + " under " + msg.parentName)

	case drillDownMsg:
		if msg.children == nil {
			return m, nil
		}
		m.drillDownStack = append(m.drillDownStack, drillDownEntry{
			parentType:  msg.parentType,
			parentName:  msg.parentName,
			parentItems: m.items,
		})
		m.currentResource = msg.childType
		m.items = filterHelmIfHidden(msg.children, msg.childType)
		m.detail.SetResourceType(msg.childType)
		m.table.SetColumns(ColumnsForResource(msg.childType))
		m.table.SetRows(augmentRowsWithHelm(m.items, msg.childType))
		m.statusLine.SetDrillDown(true)
		// Drilling changes the panel-2 resource kind, which means
		// every piece of m.detail (logs, structured detail body, the
		// Relatives drill chain, events) describes the PARENT row and
		// would render under the child kind's tab list. Clear all of
		// it unconditionally so the child gets a clean slate; the
		// kind-specific re-arm below stays gated on items>0 since it
		// needs m.items[0]. Hoisted OUT of the items>0 block — the
		// previous placement leaked the parent's aggregate stream
		// when child kind had zero rows (CronJob → Jobs with no
		// retained Jobs), and also left the parent's structured
		// detail visible under the new tab list (Relatives entries
		// describing the parent, conditions, events, ...).
		m.logStreamer.Stop()
		m.logsActive = false
		m.nextAggregateRetry = time.Time{} // per-target throttle; see ResourceSelectedMsg
		m.rowSeq++                         // invalidate any in-flight rowSwitchTickMsg from the parent kind
		m.detail.ClearDetail()
		if len(m.items) > 0 {
			cmds = append(cmds, fetchResourceDetail(m.k8sClient, msg.childType, m.items[0]))
			switch {
			case msg.childType == k8s.ResourcePods:
				containers := k8s.ContainerNames(m.items[0].Raw)
				if len(containers) > 0 {
					m.logStreamer.Start(m.items[0].Name, m.items[0].Namespace, containers)
					m.logsActive = true
					cmds = append(cmds, waitForLogLine(m.logStreamer))
				}
			case isAggregateLogsKind(msg.childType):
				// Workload kinds funnel their Pods through
				// PodsForWorkload — same path RowSelectedMsg /
				// ResourceDataMsg use for top-level rows.
				m.logsActive = true
				cmds = append(cmds, startAggregateLogs(m.k8sClient, msg.childType, m.items[0]))
			}
		}
		return m, tea.Batch(cmds...)

	case startShellExecMsg:
		// Only txPty being alive blocks a new exec — shellPty (Alterm) is
		// independent and may be hidden in the background.
		if m.txPty.IsAlive() {
			m.appLog.Warn("close active edit/exec PTY before opening shell")
			return m, m.toast.Show("Close current edit/exec PTY first")
		}
		// tdp T1 — PTY is a context-shift target; entry handler closes
		// every blocking popup that launched this action so the user
		// returns to a clean base view, not a stale source popup.
		closeAll := m.closeAllBlockingPopups()
		cmd := buildKubectlExecCmd(msg.podName, msg.namespace, msg.container, msg.contextName)
		title := fmt.Sprintf("Shell: pod/%s → %s", msg.podName, msg.container)
		// tdp T1 context-shift target — always layer 1; see toggleAlterm
		// for the rationale.
		m.txPty.SetLayer(1)
		return m, tea.Batch(closeAll, m.txPty.Start(cmd, title, m.width, m.height, PtyKindExec))

	case startEditMsg:
		if m.txPty.IsAlive() {
			m.appLog.Warn("close active edit/exec PTY before editing")
			return m, m.toast.Show("Close current edit/exec PTY first")
		}
		// tdp T1 — see startShellExecMsg above.
		closeAll := m.closeAllBlockingPopups()
		m.editing = true
		title := fmt.Sprintf("Edit: %s/%s", msg.resource.KubectlName(), msg.item.Name)
		if msg.item.Namespace != "" {
			title += " (" + msg.item.Namespace + ")"
		}
		cmd := buildKubectlEditCmd(msg.resource, msg.item, msg.contextName, m.cfgEditor)
		config.WriteAuditEntry("edit", msg.resource.KubectlName()+"/"+msg.item.Name, msg.item.Namespace, "started") //nolint
		m.appLog.Info("edit: " + msg.resource.KubectlName() + "/" + msg.item.Name)
		// tdp T1 context-shift target — always layer 1; see toggleAlterm
		// for the rationale.
		m.txPty.SetLayer(1)
		return m, tea.Batch(closeAll, m.txPty.Start(cmd, title, m.width, m.height, PtyKindEdit))

	case DeleteDoneMsg:
		out := strings.TrimSpace(msg.Output)
		if out == "" {
			out = "deleted " + msg.Name
		}
		m.appLog.Info(out)
		config.WriteAuditEntry("delete", msg.Resource, msg.Namespace, msg.Output) //nolint
		return m, nil

	case DeleteErrMsg:
		m.appLog.Error("delete failed: " + msg.Err.Error())
		return m, nil

	case appInitMsg:
		m.appLog.Info("kbu started")
		m.appLog.Info(fmt.Sprintf("connected to %s (%s)", msg.info.ContextName, msg.info.ServerURL))
		if !k8s.HelmAvailable() {
			m.appLog.Info("helm CLI not found — Helm Releases category hidden")
		}
		return m, nil

	case ClipboardCopiedMsg:
		notice := fmt.Sprintf("copied %d lines", msg.Lines)
		if msg.Lines == 1 {
			notice = "copied 1 line"
		}
		m.appLog.Info(notice)
		return m, m.toast.Show("Copied!")

	case ClipboardCopyFailedMsg:
		m.appLog.Warn("copy: " + msg.Reason)
		return m, nil

	case toastDismissMsg:
		return m, m.toast.Update(msg)

	case loadingTickMsg: // redraw the loading icon; re-arm only while something loads (tdp D3)
		m.loadingTicking = false
		return m, m.keepLoading()

	case CRDsDiscoveredMsg:
		if msg.Err != nil {
			m.appLog.Warn("CRD discovery failed: " + msg.Err.Error())
		} else if msg.Count > 0 {
			m.appLog.Info(fmt.Sprintf("discovered %d CRDs", msg.Count))
			m.sidebar.RefreshCategories(m.k8sClient.Registry())
		}
		return m, nil

	case MenuActionMsg:
		// A row of the Space menu or the global operation popup ran
		// (cursor + Enter, its hotkey, or a click).
		return m, m.runMenuAction(msg)

	case ListPickerActionMsg:
		// Sort flow commits routed by PickerID. Column step picks a
		// column → opens the direction step over it. Direction step
		// persists the choice and closes itself, back to the column
		// step.
		switch msg.PickerID {
		case "sort:column":
			if msg.Key == sortResetKey {
				return m, m.resetSortFlow()
			}
			return m, m.openSortDirectionPicker(m.sortFlowKind, msg.Key)
		case "sort:direction":
			return m, m.commitSortFlow(msg.Key)
		}
		return m, nil

	case SettingsToggleMsg:
		// Commit a Settings popup toggle. Currently only "mouse" is
		// wired; commitSettingsToggle ignores unknown keys so a
		// future setting added to the popup can be wired in one
		// place without touching this routing.
		return m, m.commitSettingsToggle(msg.Key)

	case SidebarDragEnterMsg:
		// Drag is a mode (tdp K11): its keys go in the footer for as
		// long as it lasts, with ? for the full list.
		m.statusLine.SetDragMode(true)
		return m, nil

	case SidebarDragCommitMsg:
		m.statusLine.SetDragMode(false)
		if err := m.persistPinnedKinds(); err != nil {
			m.appLog.Error("pin order save failed: " + err.Error())
			return m, m.toast.Show("pin order save failed")
		}
		return m, nil

	case SidebarDragCancelMsg:
		// Sidebar already reverted its pinned slice from the
		// snapshot. No cancellation toast — cancelling shouldn't nag
		// the user about something they decided not to do.
		m.statusLine.SetDragMode(false)
		return m, nil

	case ListPickerCancelMsg:
		// Esc on the direction step returns to the column step (tdp
		// F4): only the chosen column is dropped. Esc on the column
		// step ends the flow. The picker's own close animation is
		// already queued by the Cancel msg.
		switch msg.PickerID {
		case "sort:direction":
			m.sortFlowColumn = ""
		case "sort:column":
			m.sortFlowKind = ""
			m.sortFlowColumn = ""
		}
		return m, nil

	case HelmDocReadyMsg:
		if msg.Err != nil {
			m.appLog.Error(fmt.Sprintf("helm get %s: %s", msg.DocKind, msg.Err.Error()))
			return m, nil
		}
		// Open the YAML popup with the fetched text. notes is plain text
		// rather than YAML, but the popup renders monospace either way
		// and the user gets a uniform "press q / Esc to dismiss" UX.
		item := k8s.ResourceItem{Name: msg.ReleaseName, Namespace: msg.Namespace}
		m.yamlPopup.SetSize(m.width, m.height)
		m.yamlPopup.SetLayer(m.popupDepth() + 1)
		return m, m.yamlPopup.Open(msg.Content, k8s.ResourceReleases, item)

	case RollbackResultMsg:
		if msg.Err != nil {
			m.appLog.Error(fmt.Sprintf("rollback %s rev %d: %s", msg.ReleaseName, msg.Revision, msg.Err.Error()))
			if msg.Output != "" {
				m.appLog.Info(strings.TrimSpace(msg.Output))
			}
			return m, nil
		}
		// Success — helm's stdout is "Rollback was a success! Happy
		// Helming!" but the user-facing toast is shorter. Drop the helm
		// blurb into app log for the record.
		m.appLog.Info(fmt.Sprintf("rolled back %s to revision %d", msg.ReleaseName, msg.Revision))
		if msg.Output != "" {
			m.appLog.Info(strings.TrimSpace(msg.Output))
		}
		return m, m.toast.Show(fmt.Sprintf("Rolled back to rev %d", msg.Revision))
	}

	switch m.activePanel {
	case SidebarPanel:
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		cmds = append(cmds, cmd)
	case TablePanel:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	case DetailPanel:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// panelKey handles a key that no popup took: the app-wide hotkeys and
// then the focused panel. The Space menu runs its rows through here too
// — a menu row is a shell over the hotkey it names (tdp M3), so the two
// can never do different things.
func (m *AppModel) panelKey(msg tea.KeyMsg) tea.Cmd {
	// Sidebar drag-and-drop mode is modal: only j/k (swap),
	// D (commit), and "anything else" (cancel) make sense. Route
	// every key that reaches the panel to the sidebar first so hotkeys
	// like Tab / 1 / 2 / 3 can't slip past — pressing them mid-drag
	// should cancel, not switch focus. (q and Ctrl+C never get here:
	// the leave flow runs before any surface sees the key.)
	if m.activePanel == SidebarPanel && m.sidebar.IsDragging() {
		// tdp K11: Tab and the panel keys are suspended in the mode but
		// answer — the drag stays, the toast says how to leave it.
		switch msg.String() {
		case "tab", "shift+tab", "1", "2", "3":
			return m.toast.Show("[Esc] leaves drag mode first")
		}
		sidebar, cmd := m.sidebar.Update(msg)
		m.sidebar = sidebar
		return cmd
	}
	// While a panel search is typing, every key is the search's.
	if m.panelTyping() {
		return m.dispatchToPanel(msg)
	}
	switch msg.String() {
	case "V":
		return m.splash.Show()
	case ">":
		return m.openSettings()
	case "alt+t", "alt+T", "ctrl+t":
		// Ctrl+T is a hidden alias for the demo recorder only: vhs 0.11
		// drops the Alt modifier between Chrome and the PTY (logged
		// keypress = `t` or `ctrl+t`, never `alt+t`), so demo tapes
		// emit Ctrl+T instead. Humans never see this alias in help/UI
		// hints — the cost of accepting it is that pressing Ctrl+T
		// while a Alterm shell is visible will hide the shell instead
		// of forwarding to zsh's transpose-chars binding.
		return m.toggleAlterm()
	case "!":
		return m.openAppLog()
	case "1":
		m.detailExpanded = false
		m.setPanel(SidebarPanel)
		return nil
	case "2":
		m.detailExpanded = false
		m.setPanel(TablePanel)
		return nil
	case "3":
		m.detailExpanded = false
		m.setPanel(DetailPanel)
		return nil
	case "tab":
		m.cyclePanel()
		return nil
	case "shift+tab":
		m.cyclePanelReverse()
		return nil
	case "h":
		// v1.5.x: h/l switch the panel 3 detail tab ONLY when panel 3
		// is the active panel. Panel 1/2 = no-op (panel 2 was the
		// previous owner — moved to panel 3 so tab nav and list nav
		// live on different panels). `l` is no longer a drill key
		// either; Enter is the sole drill key (focus-shift fallback
		// removed when mouse double-click → Enter synthesis landed).
		if m.activePanel == DetailPanel {
			m.detail = m.detail.PrevTab()
			return nil
		}
	case "l":
		if m.activePanel == DetailPanel {
			m.detail = m.detail.NextTab()
			return nil
		}
	case "enter":
		return m.enterKey()
	case "esc":
		filterActive := (m.activePanel == SidebarPanel && m.sidebar.HasActiveFilter()) ||
			(m.activePanel == TablePanel && m.table.HasActiveFilter()) ||
			(m.activePanel == DetailPanel && m.detail.HasActiveFilter())
		if filterActive {
			// Let panel handle Esc to clear filter
		} else {
			// Panel 2 Esc with compare mode active: one layer
			// per press (tdp K4) — this Esc only drops the
			// compare lock; the next one backs out of the drill.
			// The lock is a panel state like the search filter
			// (ruled 2026-09-28), and the filter is peeled the
			// same way.
			if m.activePanel == TablePanel && m.inCompareMode() {
				m.clearCompareLock()
				return nil
			}
			if m.drillDownPod != nil || len(m.drillDownStack) > 0 {
				return m.exitDrillDown()
			}
		}
	case "N":
		return m.openNamespacePicker()
	case "C":
		// Panel 2 cursor-on-row: C is the contextual Compare
		// hotkey (same path as the panel-2 Space menu's "C"
		// entry). Same trade-off the P pin hotkey makes on
		// panel 1 — panel-context-specific override of a
		// global letter. Everywhere ELSE C still opens the
		// context picker.
		if m.activePanel == TablePanel && !m.editing && m.drillDownPod == nil && len(m.items) > 0 {
			idx := m.table.SelectedRow()
			if idx >= 0 && idx < len(m.items) {
				return m.compareHotkeyDispatch(m.currentResource, m.items[idx])
			}
		}
		return fetchContexts(m.k8sClient)
	case "P":
		// Panel-1 only: toggle pinned status for the cursor's
		// resource kind. Acts on the sidebar's selected row even
		// without opening Space menu first — same UX as N/C which
		// surface globally. No-op when active panel isn't the
		// sidebar or when the cursor is on a category header.
		if m.activePanel != SidebarPanel {
			return nil
		}
		rt := m.sidebar.CursorResourceType()
		if rt == "" {
			return nil
		}
		return m.togglePinnedKind(rt)
	case "E":
		if !m.editing && m.activePanel == TablePanel && m.drillDownPod == nil && len(m.items) > 0 {
			idx := m.table.SelectedRow()
			if idx < 0 || idx >= len(m.items) {
				return nil
			}
			// Kind-level gate (mirrors panel 2 menu): kinds with no
			// editable surface (Events, Contexts, Releases) answer
			// with a toast.
			if !resourceAllowsEdit(m.currentResource) {
				return m.toast.Show("Edit not supported on " + m.currentResource.KubectlName())
			}
			return m.confirmEdit(m.currentResource, m.items[idx])
		}
	case ".":
		// Toggle visibility of helm-managed items on any panel 2
		// resource list. Helm Releases themselves are excluded (the
		// category IS helm) — there `.` is a no-op. Re-start the
		// watcher to re-emit the cached items so the new filter
		// shows / hides them right away.
		if m.activePanel != TablePanel || m.currentResource == k8s.ResourceReleases {
			return nil
		}
		k8s.ToggleHelmHideManaged()
		m.watcher.Start(m.currentResource, m.k8sClient.Selection())
		return waitForWatchUpdate(m.watcher, m.currentResource)
	case "S":
		// Panel-1: open the sort column picker on the cursor's
		// kind. Restores the v1.6 muscle memory — sort lives on S
		// in panel 1 (no conflict with anything panel-1 specific).
		// Panel-2 keeps S as Shell; panel-2 sort moves through O
		// (see case "O" below).
		if m.activePanel == SidebarPanel {
			rt := m.sidebar.CursorResourceType()
			if rt == "" {
				return nil
			}
			return m.openSortColumnPicker(rt)
		}
		if m.activePanel == TablePanel {
			return m.execShell()
		}
	case "alt+S":
		// Panel-2 only sort entry on Alt+Shift+S — bare S is
		// already Shell on panel 2 and reverting wholesale (no
		// sort hotkey here) would force the user back to panel 1
		// just to reorder rows. The modifier carves out a panel-
		// 2 sort gesture without colliding with Shell. Panel 1
		// still uses plain S (v1.6 muscle memory).
		if m.activePanel == TablePanel {
			// Container drill view (panel 2 showing containers of a
			// drilled pod): no-op. Matches E/D/C gating — row-level
			// operations are blocked during drill so the picker
			// title "Sort Pods by…" can't appear while the user is
			// looking at containers.
			if m.drillDownPod != nil {
				return nil
			}
			if m.currentResource == "" {
				return nil
			}
			return m.openSortColumnPicker(m.currentResource)
		}
		return nil
	case "D":
		// Panel-1: enter drag-and-drop reorder mode for the
		// cursor's pinned kind. Mirrors the panel-meaning split
		// used by S (panel 2 only: shell) and C (panel 2 only).
		// Guards in EnterDrag — silent no-op when cursor isn't
		// on a pinned row or there's <2 pins.
		if m.activePanel == SidebarPanel {
			_, cmd := m.sidebar.EnterDrag()
			return cmd
		}
		if m.activePanel == TablePanel && m.drillDownPod == nil && len(m.items) > 0 {
			idx := m.table.SelectedRow()
			if idx >= 0 && idx < len(m.items) {
				// Kind-level gate (mirrors panel 2 menu): Events / Nodes /
				// Contexts / Releases are never deleted from kbu.
				if !resourceAllowsDelete(m.currentResource) {
					return m.toast.Show("Delete not supported on " + m.currentResource.KubectlName())
				}
				return m.confirmDelete(m.currentResource, m.items[idx])
			}
		}
	case "z":
		return m.toggleZoom()
	case "B":
		// Relatives, drilled in: the breadcrumb of the chain walked so
		// far, to switch panels 1 and 2 to any step of it. At the first
		// level there is nothing up the chain.
		if m.activePanel == DetailPanel && m.detail.ActiveTabName() == "Relatives" && m.detail.Depth() > 1 {
			m.breadcrumbPopup.SetSize(m.width, m.height)
			m.breadcrumbPopup.SetLayer(m.popupDepth() + 1)
			return m.breadcrumbPopup.Open(m.detail.DrillChain())
		}
		return nil
	case "y":
		return copyToClipboardCmd(m.focusedPanelContent())
	case "Y":
		// Y is a panel-2 / panel-3 affordance — opens the YAML of
		// the resource currently selected (panel 2) or drilled into
		// (panel 3). Pressing Y from panel 1 with focus elsewhere
		// would silently open the LAST panel-2 selection's YAML,
		// which feels like an out-of-context jump — gate it.
		if m.activePanel == SidebarPanel {
			return nil
		}
		// Cursor-aware on the Relatives tab: if the cursor sits on a
		// drillable entry, fetch + popup THAT entry's YAML (via
		// RelativeDrillMsg). If no drillable cursor (empty / non-link
		// row), fall through to the current level's own YAML — at
		// depth 1 that's the table-selected resource's YAML
		// (existing behavior), at deeper levels it's the resource
		// the user has drilled into.
		if m.activePanel == DetailPanel && m.detail.ActiveTabName() == "Relatives" {
			if ref := m.detail.SelectedRelativeRef(); ref != nil {
				target := *ref
				return func() tea.Msg { return RelativeDrillMsg{Ref: target} }
			}
		}
		yaml := m.detail.CurrentLevelYAML()
		if yaml == "" {
			return nil
		}
		var resource k8s.ResourceType
		var item k8s.ResourceItem
		if m.detail.Depth() > 1 {
			resource = m.detail.currentLevelKind()
			item = m.detail.CurrentLevelItem()
		} else if !m.editing && m.drillDownPod == nil && len(m.items) > 0 {
			idx := m.table.SelectedRow()
			if idx >= 0 && idx < len(m.items) {
				resource = m.currentResource
				item = m.items[idx]
			}
		}
		m.yamlPopup.SetSize(m.width, m.height)
		m.yamlPopup.SetLayer(m.popupDepth() + 1)
		return m.yamlPopup.Open(yaml, resource, item)
	case " ":
		// tdp K5, M2, M7: Space on any panel and any tab opens the
		// Space menu of what can be done here — always, even when only
		// the Global operation row is left.
		return m.openSpaceMenu()
	}
	return m.dispatchToPanel(msg)
}

// enterKey is Enter on a panel (tdp K3): the most obvious action for the
// focused item, the same one for the same kind of item everywhere.
//   - panel 1, a kind: show it in panel 2 (focus moves there)
//   - panel 2: drill in where the kind drills (Pods → containers,
//     workloads → pods, …); a KubeConfig context: switch to it (after a
//     confirm); a container: shell into it; any other kind: its YAML
//   - panel 3: Relatives drills into the entry; History rolls back to the
//     revision (after a confirm); a content tab (Logs, Events,
//     Conditions, Info) — no item to act on — full-screens the panel,
//     the same as z
func (m *AppModel) enterKey() tea.Cmd {
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	switch m.activePanel {
	case SidebarPanel:
		if m.sidebar.CursorResourceType() == "" {
			return m.dispatchToPanel(enter)
		}
		m.setPanel(TablePanel)
		return nil
	case TablePanel:
		if m.drillDownPod != nil {
			return m.execShell()
		}
		idx := m.table.SelectedRow()
		if idx < 0 || idx >= len(m.items) {
			return nil
		}
		item := m.items[idx]
		switch {
		case m.currentResource == k8s.ResourceContexts:
			return m.confirmContextSwitch(item.Name)
		case m.currentResource.SupportsDrillDown():
			return m.enterDrillDown()
		}
		return m.openYamlFor(m.currentResource, item)
	case DetailPanel:
		switch m.detail.ActiveTabName() {
		case "Relatives":
			return m.dispatchToPanel(enter)
		case "History":
			return m.confirmRollback()
		}
		return m.toggleZoom()
	}
	return nil
}

// toggleZoom is z: full-screen the focused panel 2 or 3, or restore.
func (m *AppModel) toggleZoom() tea.Cmd {
	if m.detailExpanded || m.tableExpanded {
		m.detailExpanded = false
		m.tableExpanded = false
		return nil
	}
	switch m.activePanel {
	case DetailPanel:
		m.detailExpanded = true
	case TablePanel:
		m.tableExpanded = true
	}
	return nil
}

// confirmContextSwitch asks before switching kbu to another kubeconfig
// context (Enter on a KubeConfig Contexts row); accepting takes the same
// path as picking it in the context picker. The context kbu is already
// on is not a switch: nothing happens.
func (m *AppModel) confirmContextSwitch(name string) tea.Cmd {
	if name == "" || name == m.k8sClient.ContextName() {
		return nil
	}
	onConfirm := func() tea.Msg { return ContextChangedMsg{Context: name} }
	m.confirm.SetSize(m.width, m.height)
	m.confirm.SetLayer(m.popupDepth() + 1)
	return m.confirm.Show(ConfirmContextSwitch, "Switch kbu to context "+name+"?",
		"~/.kube/config is not changed", onConfirm)
}

// ptyKillMsg: the user confirmed ending a PTY session — Alterm's shell
// (kind PtyKindShell) or the kubectl edit / exec one.
type ptyKillMsg struct{ kind PtyKind }

// quitCmd starts the leave flow (tdp K9). kbu leaves straight away — no
// confirm step; the quitMsg handler does the teardown (streams, PTYs,
// session-state save) in one place.
func quitCmd() tea.Msg { return quitMsg{} }

// panelTyping reports whether the focused panel's search line is taking
// keystrokes.
func (m *AppModel) panelTyping() bool {
	return (m.activePanel == TablePanel && m.table.IsSearching()) ||
		(m.activePanel == SidebarPanel && m.sidebar.IsSearching()) ||
		(m.activePanel == DetailPanel && m.detail.IsSearching())
}

// typing reports whether the frontmost surface is an input taking
// keystrokes (tdp K8): a panel's search line, a picker's filter line, the
// YAML viewer's search line. There, printable keys — q and ? included —
// are characters.
func (m *AppModel) typing() bool {
	switch top := m.topLayer(); top {
	case nil:
		return m.panelTyping()
	case &m.namespacePicker:
		return m.namespacePicker.searching
	case &m.contextPicker:
		return m.contextPicker.searching
	case &m.yamlPopup:
		return m.yamlPopup.IsSearching()
	}
	return false
}

// dispatchToPanel hands a message to the focused panel's own Update.
func (m *AppModel) dispatchToPanel(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.activePanel {
	case SidebarPanel:
		m.sidebar, cmd = m.sidebar.Update(msg)
	case TablePanel:
		m.table, cmd = m.table.Update(msg)
	case DetailPanel:
		m.detail, cmd = m.detail.Update(msg)
	}
	return cmd
}

func (m AppModel) View() string {
	if !m.ready {
		return "loading..."
	}

	if m.splash.IsActive() {
		return m.splash.Render(m.width, m.height)
	}

	// Alterm marker: render ONLY when the shell is alive but hidden — that's
	// the state the user can't otherwise see ("is there a shell waiting for
	// me?"). When the popup is visible, the popup's border already says
	// "Alterm: hostname" so a status-bar duplicate just adds noise; when no
	// shell is running, nothing to mark.
	var ptyMarker *PtyMarker
	if m.shellPty != nil && m.shellPty.IsAlive() && m.shellPty.IsHidden() {
		// Statusbar renders "[Alt-t]erm" — pointer-as-flag, no label.
		ptyMarker = &PtyMarker{}
	}
	var compareMarker *CompareMarker
	if m.inCompareMode() {
		// "[C]ompare" — pointer-as-flag, statusbar renders the chip
		// with panel-aware [C] dimming anti-correlated to [C]ontext
		// (lit on panel 2 where C fires compare, dim on panel 1/3
		// where C means context picker).
		compareMarker = &CompareMarker{}
	}
	statusBar := m.statusBar.ViewFull(m.appLog.UnreadErrorCount(), m.appLog.UnreadWarnCount(), m.successNotice, ptyMarker, compareMarker)
	statusLine := m.statusLine.ViewWithNotice(m.appLog.UnreadErrorCount(), m.appLog.UnreadWarnCount(), m.appLog.LastErrorMessage(), m.appLog.LastWarnMessage(), "")

	var mainView string

	if m.detailExpanded {
		panelH := m.height - 1 - m.statusLine.LineCount()
		panelW := m.width - 2*panelHMargin
		m.detail.SetSize(panelW-2, panelH-2)
		fullPanel := renderPanelWithScroll(m.detail.View(), plainTitlePrefix("[3]", m.theme, true)+m.detail.TabTitle(), panelW, panelH, true, m.theme, m.detail.ScrollInfo(), m.detail.BorderTopRightHint(), m.detail.BorderBottomLeftHint())
		hMargin := blankColumn(panelHMargin, panelH)
		middle := lipgloss.JoinHorizontal(lipgloss.Top, hMargin, fullPanel, hMargin)
		mainView = lipgloss.JoinVertical(lipgloss.Left, statusBar, middle, statusLine)
	} else if m.tableExpanded {
		_, _, upperH, detailH := m.panelSizes()
		panelW := m.width - 2*panelHMargin
		m.table.SetSize(panelW-2, upperH-2)
		m.detail.SetSize(panelW-2, detailH-2)
		tablePanel := renderPanelWithScroll(m.table.View(), focusedPanelTitle("[2]", m.breadcrumb(), m.theme, m.activePanel == TablePanel), panelW, upperH, m.activePanel == TablePanel, m.theme, m.table.ScrollInfo(), "", m.tablePanelBottomLeft())
		detailPanel := renderPanelWithScroll(m.detail.View(), plainTitlePrefix("[3]", m.theme, m.activePanel == DetailPanel)+m.detail.TabTitle(), panelW, detailH, m.activePanel == DetailPanel, m.theme, m.detail.ScrollInfo(), m.detail.BorderTopRightHint(), m.detail.BorderBottomLeftHint())
		middle := joinTableAndDetail(tablePanel, detailPanel, panelW)
		fullH := upperH + panelVSpace + detailH
		hMargin := blankColumn(panelHMargin, fullH)
		middleWithMargins := lipgloss.JoinHorizontal(lipgloss.Top, hMargin, middle, hMargin)
		mainView = lipgloss.JoinVertical(lipgloss.Left, statusBar, middleWithMargins, statusLine)
	} else {
		sw, rw, upperH, detailH := m.panelSizes()
		fullH := upperH + panelVSpace + detailH // sidebar matches right side total height
		m.sidebar.SetSize(sw-2, fullH-2)
		m.table.SetSize(rw-2, upperH-2)
		m.detail.SetSize(rw-2, detailH-2)

		sidebarPanel := renderPanelWithScroll(m.sidebar.View(), focusedPanelTitle("[1]", "Kinds", m.theme, m.activePanel == SidebarPanel), sw, fullH, m.activePanel == SidebarPanel, m.theme, m.sidebar.ScrollInfo(), "", nil)
		tablePanel := renderPanelWithScroll(m.table.View(), focusedPanelTitle("[2]", m.breadcrumb(), m.theme, m.activePanel == TablePanel), rw, upperH, m.activePanel == TablePanel, m.theme, m.table.ScrollInfo(), "", m.tablePanelBottomLeft())
		detailPanel := renderPanelWithScroll(m.detail.View(), plainTitlePrefix("[3]", m.theme, m.activePanel == DetailPanel)+m.detail.TabTitle(), rw, detailH, m.activePanel == DetailPanel, m.theme, m.detail.ScrollInfo(), m.detail.BorderTopRightHint(), m.detail.BorderBottomLeftHint())

		rightSide := joinTableAndDetail(tablePanel, detailPanel, rw)

		// 1-col gap between sidebar and right side, plus 1-col margins on
		// the outer edges so panel borders sit 1 cell inside the terminal.
		hMargin := blankColumn(panelHMargin, fullH)
		hSpace := blankColumn(panelHSpace, fullH)
		middle := lipgloss.JoinHorizontal(lipgloss.Top, hMargin, sidebarPanel, hSpace, rightSide, hMargin)
		mainView = lipgloss.JoinVertical(lipgloss.Left, statusBar, middle, statusLine)
	}

	// The popup stack, bottom first — the same stackOrder the keys and
	// clicks are routed by, so the popup drawn on top is the one that
	// answers them. A popup still running its close animation is drawn
	// in its slot until the animation ends.
	//
	// Only the top popup is bright (tdp F8): just before it is drawn,
	// everything already on screen — the panels, streaming logs, the
	// popups beneath — is redrawn dimmed. A popup that has started to
	// close no longer owns the top, so the one beneath lights up again
	// at once.
	top := m.topLayer()
	for _, l := range m.stackOrder() {
		if l.drawn() {
			if l == top {
				mainView = dimANSI(mainView)
			}
			l.resize(m.width, m.height)
			mainView = overlay.Composite(l.render(), mainView, overlay.Center, overlay.Center, 0, 0)
		}
	}

	// The toast composites AFTER the popup stack so a fresh message
	// (error, save-failed status) shows over whatever popup is on
	// screen. It is not a layer: it never takes keys (tdp F1, F8).
	if m.toast.IsActive() {
		// tdp F7: the toast sits at the bottom, just above the footer.
		m.toast.SetSize(m.width)
		mainView = overlay.Composite(m.toast.RenderPopup(), mainView, overlay.Center, overlay.Bottom, 0, -1)
	}

	return mainView
}

func (m *AppModel) layout() {
	m.statusBar.SetWidth(m.width)
	m.statusLine.SetWidth(m.width)
	m.help.SetSize(m.width, m.height)

	sw, rw, upperH, detailH := m.panelSizes()
	fullH := upperH + detailH
	m.sidebar.SetSize(sw-2, fullH-2)
	m.table.SetSize(rw-2, upperH-2)
	m.detail.SetSize(rw-2, detailH-2)
}

// panelSizes derives panel dimensions purely by subtraction from the terminal
// size, using the absolute constants above.
//
// Horizontal: m.width = hMargin + sw + hSpace + rw + hMargin
// Vertical:   middleH = m.height - statusBar(1) - statusLine.LineCount()
//
//	middleH = upperH + vSpace + detailH      (right side)
//	middleH = sidebar height                 (left side, no vSpace)
func (m AppModel) panelSizes() (sw, rw, upperH, detailH int) {
	sw = panelSidebarWidth
	rw = m.width - 2*panelHMargin - panelHSpace - sw

	middleH := m.height - 1 - m.statusLine.LineCount()
	detailH = panelDetailHeight
	upperH = middleH - panelVSpace - detailH

	// Tiny-terminal sanity clamps. Layout still expressed as
	// `available - reserved`, just rebalanced when reserved exceeds
	// available.
	const (
		minSw      = 12
		minRw      = 20
		minUpperH  = 4
		minDetailH = 4
	)
	if sw > m.width-2*panelHMargin-panelHSpace-minRw {
		sw = m.width - 2*panelHMargin - panelHSpace - minRw
	}
	if sw < minSw {
		sw = minSw
	}
	rw = m.width - 2*panelHMargin - panelHSpace - sw
	if rw < minRw {
		rw = minRw
	}

	if middleH < minUpperH+panelVSpace+minDetailH {
		middleH = minUpperH + panelVSpace + minDetailH
	}
	if detailH > middleH-panelVSpace-minUpperH {
		detailH = middleH - panelVSpace - minUpperH
	}
	if detailH < minDetailH {
		detailH = minDetailH
	}
	upperH = middleH - panelVSpace - detailH
	if upperH < minUpperH {
		upperH = minUpperH
		detailH = middleH - panelVSpace - upperH
	}
	return
}

// blankColumn builds a w×h block of spaces, suitable for use as a horizontal
// spacer column in lipgloss.JoinHorizontal. Returns "" for zero or negative
// dimensions.
func blankColumn(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	line := strings.Repeat(" ", w)
	if h == 1 {
		return line
	}
	return strings.Repeat(line+"\n", h-1) + line
}

// joinTableAndDetail vertically stacks the table + detail panels on the
// right side. When panelVSpace > 0, inserts that many blank rows between
// them; when 0 (current default) the borders sit flush.
func joinTableAndDetail(tablePanel, detailPanel string, w int) string {
	if panelVSpace <= 0 {
		return lipgloss.JoinVertical(lipgloss.Left, tablePanel, detailPanel)
	}
	spacer := blankColumn(w, panelVSpace)
	return lipgloss.JoinVertical(lipgloss.Left, tablePanel, spacer, detailPanel)
}

func (m *AppModel) enterDrillDown() tea.Cmd {
	idx := m.table.SelectedRow()
	if idx < 0 || idx >= len(m.items) {
		// Empty / out-of-range table — no drill target. Used to
		// fall through to "focus panel 3" so the key wasn't silent;
		// removed when the broader Enter-as-focus fallback went
		// away (mouse double-click synthesizes Enter and shifting
		// focus on double-click felt wrong).
		return nil
	}
	item := m.items[idx]

	// Pod → Container drill-down (special case)
	if m.currentResource == k8s.ResourcePods {
		// tdp T1 — drill-down is a context-shift target; entry handler
		// closes every blocking popup that launched it so the user
		// returns to a clean drilled-in view without a stale source
		// popup (e.g. panel 2 menu) floating over swapped columns.
		closeAll := m.closeAllBlockingPopups()
		m.drillDownPod = &item
		detail := k8s.GetResourceDetail(k8s.ResourcePods, item)
		m.drillDownContainers = detail.Containers
		m.table.SetColumns(containerColumns())
		m.table.SetRows(containerRows(m.drillDownContainers))
		m.statusLine.SetDrillDown(true)
		if len(m.drillDownContainers) > 0 {
			c := m.drillDownContainers[0]
			d := containerToDetail(c, item)
			m.detail.SetDetail(d, nil)
			m.detail.logLines = nil
			m.logStreamer.Start(item.Name, item.Namespace, []string{c.Name})
			m.logsActive = true
			return tea.Batch(closeAll, waitForLogLine(m.logStreamer))
		}
		return closeAll
	}

	// Resource → child resource drill-down — only kinds with a
	// registered DrillDown config (HPA → target workload, etc.). For
	// everything else Enter is now a deliberate no-op (the panel-2 →
	// panel-3 focus shift was removed alongside the broader Enter-
	// as-focus fallback).
	if !m.currentResource.SupportsDrillDown() {
		return nil
	}

	// tdp T1 — see Pod branch above.
	closeAll := m.closeAllBlockingPopups()
	parentType := m.currentResource
	clientset := m.k8sClient.Clientset()
	return tea.Batch(closeAll, func() tea.Msg {
		childType, children, err := k8s.FetchChildResources(context.Background(), clientset, parentType, item)
		return drillResultMsg(parentType, item.Name, childType, children, err)
	})
}

// drillDownFailedMsg: a panel 2 drill fetched nothing to show — an error,
// or no children at all.
type drillDownFailedMsg struct {
	parentType k8s.ResourceType
	parentName string
	childType  k8s.ResourceType
	err        error
}

// drillResultMsg turns a child fetch into the message the drill handles:
// the children, or why there are none — a failure must be visible at once
// (tdp F5), and an empty result shouldn't look like Enter was ignored.
func drillResultMsg(parentType k8s.ResourceType, parentName string, childType k8s.ResourceType, children []k8s.ResourceItem, err error) tea.Msg {
	if err != nil || len(children) == 0 {
		return drillDownFailedMsg{parentType: parentType, parentName: parentName, childType: childType, err: err}
	}
	return drillDownMsg{parentType: parentType, parentName: parentName, childType: childType, children: children}
}

func (m *AppModel) exitDrillDown() tea.Cmd {
	m.logStreamer.Stop()
	m.logsActive = false
	m.nextAggregateRetry = time.Time{} // per-target throttle; see ResourceSelectedMsg
	m.rowSeq++                         // invalidate any in-flight rowSwitchTickMsg from the child kind
	m.detail.logLines = nil

	// If at container level, go back to pod list
	if m.drillDownPod != nil {
		m.drillDownPod = nil
		m.drillDownContainers = nil
		// Restore current resource's table. Rows MUST be helm-augmented to
		// stay in lockstep with ColumnsForResource (which always reserves
		// an index-1 helm marker column for non-Releases kinds). Using raw
		// item.Row here shifts every column one to the left and breaks
		// any future per-column treatment that resolves by column title.
		m.table.SetColumns(ColumnsForResource(m.currentResource))
		m.table.SetRows(augmentRowsWithHelm(m.items, m.currentResource))
		m.statusLine.SetDrillDown(len(m.drillDownStack) > 0)
		return m.refreshDetailForCurrent()
	}

	// Pop from resource drill-down stack
	if len(m.drillDownStack) > 0 {
		entry := m.drillDownStack[len(m.drillDownStack)-1]
		m.drillDownStack = m.drillDownStack[:len(m.drillDownStack)-1]
		m.currentResource = entry.parentType
		m.items = entry.parentItems
		m.detail.SetResourceType(m.currentResource)
		m.table.SetColumns(ColumnsForResource(m.currentResource))
		m.table.SetRows(augmentRowsWithHelm(m.items, m.currentResource))
		m.statusLine.SetDrillDown(len(m.drillDownStack) > 0)
		return m.refreshDetailForCurrent()
	}

	return nil
}

// currentItemUID returns the UID of the row currently highlighted in the
// table, or "" when no row is selectable (empty list, cursor out of range).
// Used to drop stale fetch results — async fetches that finish after the
// user has moved on to a different row would otherwise overwrite the
// freshly displayed detail.
func (m AppModel) currentItemUID() string {
	if len(m.items) == 0 {
		return ""
	}
	idx := m.table.SelectedRow()
	if idx < 0 || idx >= len(m.items) {
		return ""
	}
	return m.items[idx].UID
}

func (m *AppModel) refreshDetailForCurrent() tea.Cmd {
	if len(m.items) == 0 {
		return nil
	}
	idx := m.table.SelectedRow()
	if idx < 0 || idx >= len(m.items) {
		return nil
	}
	item := m.items[idx]
	var cmds []tea.Cmd
	cmds = append(cmds, fetchResourceDetail(m.k8sClient, m.currentResource, item))
	switch {
	case m.currentResource == k8s.ResourcePods:
		containers := k8s.ContainerNames(item.Raw)
		if len(containers) > 0 {
			m.detail.logLines = nil
			m.logStreamer.Start(item.Name, item.Namespace, containers)
			m.logsActive = true
			cmds = append(cmds, waitForLogLine(m.logStreamer))
		}
	case isAggregateLogsKind(m.currentResource):
		// Drill-up from a Pods child back into a workload-kind parent
		// must restart the aggregate stream — exitDrillDown stopped
		// whatever was running but never re-armed for non-Pods. Before
		// this branch, drilling Deployment → Pods → Esc left the
		// Deployment row's Logs tab silent until the user nudged the
		// cursor (next RowSelectedMsg restarted). The feat(logs)
		// extension widened the gap from Deployment-only to all 5
		// workload kinds (StatefulSet / DaemonSet / Job / CronJob).
		// dispatch-time logsActive=true mirrors RowSelectedMsg/
		// ResourceDataMsg so a watcher tick in the gap can't double-fire.
		// Throttle clear matches RowSelectedMsg too — user-initiated
		// drill-up deserves a fresh attempt; stale timer from prior
		// failure would otherwise gate subsequent watcher tick.
		m.detail.logLines = nil
		m.logsActive = true
		m.nextAggregateRetry = time.Time{}
		cmds = append(cmds, startAggregateLogs(m.k8sClient, m.currentResource, item))
	}
	return tea.Batch(cmds...)
}

func containerColumns() []Column {
	return []Column{
		{Title: "Name", MinWidth: 15},
		{Title: "Image", MinWidth: 30},
		{Title: "State", MinWidth: 12},
		{Title: "Ready", MinWidth: 7},
		{Title: "Restarts", MinWidth: 10},
	}
}

func containerRows(containers []k8s.ContainerInfo) [][]string {
	rows := make([][]string, len(containers))
	for i, c := range containers {
		ready := "false"
		if c.Ready {
			ready = "true"
		}
		prefix := ""
		if c.Init {
			prefix = "(init) "
		}
		rows[i] = []string{
			prefix + c.Name,
			c.Image,
			c.State,
			ready,
			fmt.Sprintf("%d", c.Restarts),
		}
	}
	return rows
}

func containerToDetail(c k8s.ContainerInfo, pod k8s.ResourceItem) k8s.ResourceDetail {
	name := c.Name
	if c.Init {
		name = "(init) " + name
	}
	yaml := k8s.MarshalContainerYAML(pod, c.Name)

	// Structured fields are kept as a fallback for the rare case where YAML
	// extraction fails (e.g. pod.Raw not a *corev1.Pod).
	fields := []k8s.DetailField{
		{Label: "Pod", Value: pod.Name},
		{Label: "Image", Value: c.Image},
		{Label: "State", Value: c.State},
	}
	ready := "false"
	if c.Ready {
		ready = "true"
	}
	fields = append(fields, k8s.DetailField{Label: "Ready", Value: ready})
	if c.Restarts > 0 {
		fields = append(fields, k8s.DetailField{Label: "Restarts", Value: fmt.Sprintf("%d", c.Restarts)})
	}
	if c.Ports != "" {
		fields = append(fields, k8s.DetailField{Label: "Ports", Value: c.Ports})
	}
	return k8s.ResourceDetail{
		Name:      name,
		Namespace: pod.Namespace,
		Kind:      "Container",
		YAML:      yaml,
		Fields:    fields,
	}
}

func (m AppModel) breadcrumb() string {
	if m.drillDownPod != nil {
		return truncateName(m.drillDownPod.Name, 20) + " > Containers"
	}
	if len(m.drillDownStack) > 0 {
		last := m.drillDownStack[len(m.drillDownStack)-1]
		return truncateName(last.parentName, 20) + " > " + m.currentResource.String()
	}
	return m.currentResource.String()
}

func ansiTruncate(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	var result []byte
	w := 0
	inEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\x1b' {
			inEscape = true
			result = append(result, c)
			continue
		}
		if inEscape {
			result = append(result, c)
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEscape = false
			}
			continue
		}
		if w >= maxWidth {
			break
		}
		result = append(result, c)
		w++
	}
	result = append(result, "\x1b[0m"...)
	return string(result)
}

func truncateName(name string, max int) string {
	if len(name) <= max {
		return name
	}
	return name[:max-1] + "…"
}

func (m *AppModel) execShell() tea.Cmd {
	var podName, namespace, container string

	if m.drillDownPod != nil {
		idx := m.table.SelectedRow()
		if idx < 0 || idx >= len(m.drillDownContainers) {
			return nil
		}
		podName = m.drillDownPod.Name
		namespace = m.drillDownPod.Namespace
		container = m.drillDownContainers[idx].Name
	} else {
		if m.currentResource != k8s.ResourcePods || len(m.items) == 0 {
			return nil
		}
		idx := m.table.SelectedRow()
		if idx < 0 || idx >= len(m.items) {
			return nil
		}
		item := m.items[idx]
		containers := k8s.ContainerNames(item.Raw)
		if len(containers) == 0 {
			return nil
		}
		podName = item.Name
		namespace = item.Namespace
		container = containers[0]
	}

	detail := fmt.Sprintf("kubectl exec -it %s -n %s -c %s", podName, namespace, container)
	m.confirm.SetLayer(m.popupDepth() + 1)
	showCmd := m.confirm.Show(ConfirmShellExec, "Exec into container?", detail,
		shellExec(podName, namespace, container, m.k8sClient.ContextName()))
	m.appLog.Info("exec shell: " + detail)
	return showCmd
}

// openSettings opens the Settings popup (`>`, or the global operation
// popup). Items are rebuilt from the config on every open so the badges
// reflect the current state.
func (m *AppModel) openSettings() tea.Cmd {
	m.settingsPopup.SetSize(m.width, m.height)
	m.settingsPopup.SetLayer(m.popupDepth() + 1)
	return m.settingsPopup.Open(m.buildSettingsItems())
}

// openAppLog opens the App Log popup (`!`, or the global operation popup).
func (m *AppModel) openAppLog() tea.Cmd {
	if m.appLog.owns() {
		return nil
	}
	m.appLog.SetSize(m.width, m.height)
	m.appLog.SetLayer(m.popupDepth() + 1)
	return m.appLog.Toggle()
}

// openNamespacePicker opens the picker immediately in its loading state
// so the user gets zero-lag visual feedback, then fires the LIST
// namespaces API in parallel. NamespaceListMsg swaps in the real list
// when it arrives — no flicker because the animator stays open across
// SetNamespaces.
func (m *AppModel) openNamespacePicker() tea.Cmd {
	m.namespacePicker.SetLayer(m.popupDepth() + 1)
	m.namespacePicker.SetSelection(m.k8sClient.Selection())
	openCmd := m.namespacePicker.OpenLoading()
	return tea.Batch(openCmd, m.keepLoading(), fetchNamespaces(m.k8sClient))
}

// toggleAlterm is the single Alterm entry (Alt+t, or the global
// operation popup):
//   - no shell alive → spawn Alterm
//   - alive, hidden  → reattach (show)
//   - alive, visible → handled inside PtyView.Update (hides)
//
// Alterm is a context-shift target (tdp T1): it replaces the popup stack
// rather than stacking on it, so every popup is closed first — from the
// global operation popup that means the Space menu under it too. A PTY
// always takes layer 1 so Alterm and kubectl edit / exec share one
// border color. Alterm and an edit / exec PTY coexist (dual slot); the
// transient one draws on top.
func (m *AppModel) toggleAlterm() tea.Cmd {
	closeAll := m.closeAllBlockingPopups()
	m.shellPty.SetLayer(1)
	if m.shellPty.IsAlive() {
		return tea.Batch(closeAll, m.shellPty.Show(m.width, m.height))
	}
	cmd := buildShellTerminalCmd(m.cfg.AltermShell, m.cfg.AltermLoginShell)
	return tea.Batch(closeAll, m.shellPty.Start(cmd, terminalTitle(), m.width, m.height, PtyKindShell))
}

// confirmEdit asks before running kubectl edit on item. Rule A: a
// helm-managed object is read-only in kbu (an edit would be overwritten
// by the next helm upgrade / rollback) — the Space menu dims its Edit
// row, and the E hotkey, the row's shortcut, does nothing either (tdp
// M6).
func (m *AppModel) confirmEdit(rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	if !resourceAllowsEdit(rt) || k8s.IsHelmManaged(item) {
		return nil
	}
	detail := fmt.Sprintf("kubectl edit %s/%s", rt.KubectlName(), item.Name)
	if item.Namespace != "" {
		detail += " -n " + item.Namespace
	}
	contextName := m.k8sClient.ContextName()
	startCmd := func() tea.Msg {
		return startEditMsg{resource: rt, item: item, contextName: contextName}
	}
	m.confirm.SetSize(m.width, m.height)
	m.confirm.SetLayer(m.popupDepth() + 1)
	return m.confirm.Show(ConfirmEdit, "Edit resource?", detail, startCmd)
}

// confirmDelete asks before running kubectl delete on item. Same Rule A
// as confirmEdit: helm-managed objects are removed with helm uninstall.
func (m *AppModel) confirmDelete(rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	if !resourceAllowsDelete(rt) || k8s.IsHelmManaged(item) {
		return nil
	}
	message, detail := deleteConfirmSurface(rt, item)
	m.confirm.SetSize(m.width, m.height)
	m.confirm.SetLayer(m.popupDepth() + 1)
	return m.confirm.ShowCompleting(ConfirmDelete, message, detail,
		deleteResource(rt, item.Name, item.Namespace, m.k8sClient.ContextName()))
}

// openYamlFor opens the YAML viewer on the detail panel's current level,
// tagged with the resource it belongs to (so E inside it knows what to
// edit).
func (m *AppModel) openYamlFor(rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	yaml := m.detail.CurrentLevelYAML()
	if yaml == "" {
		return nil
	}
	m.yamlPopup.SetSize(m.width, m.height)
	m.yamlPopup.SetLayer(m.popupDepth() + 1)
	return m.yamlPopup.Open(yaml, rt, item)
}

// confirmRollback asks before rolling the Helm release back to the
// revision under the History cursor. The deployed revision can't be a
// rollback target: nothing happens there (its menu row is dimmed).
func (m *AppModel) confirmRollback() tea.Cmd {
	rev, current := m.detail.HistoryCursor()
	if rev == nil || current {
		return nil
	}
	root := m.detail.RootRef()
	msg := fmt.Sprintf("Rollback %s to revision %d?", root.Name, rev.Revision)
	cmdStr := k8s.RollbackCommandString(root.Name, root.Namespace, rev.Revision)
	m.confirm.SetSize(m.width, m.height)
	m.confirm.SetLayer(m.popupDepth() + 1)
	return m.confirm.ShowCompleting(ConfirmRollback, msg, cmdStr, rollbackReleaseCmd(root.Name, root.Namespace, rev.Revision))
}

// shellExec returns a Cmd that asks AppModel to launch a PTY for kubectl exec.
// AppModel cannot start the PTY directly from inside confirm's onConfirm
// closure because the closure has no access to model state — so we round-trip
// through startShellExecMsg.
func shellExec(podName, namespace, container, contextName string) tea.Cmd {
	return func() tea.Msg {
		return startShellExecMsg{
			podName:     podName,
			namespace:   namespace,
			container:   container,
			contextName: contextName,
		}
	}
}

type startShellExecMsg struct {
	podName, namespace, container, contextName string
}

func buildKubectlExecCmd(podName, namespace, container, contextName string) *exec.Cmd {
	args := []string{"exec", "-it", podName, "-n", namespace, "-c", container}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}
	// Prefer bash via PATH lookup (covers /bin/bash, /usr/bin/bash,
	// /usr/local/bin/bash) and fall back to POSIX sh — handles the
	// 95% case (debian/ubuntu/centos+bash, alpine sh, debian minimal)
	// in a single kubectl invocation. Distroless / scratch images
	// without /bin/sh still fail; no probe sidesteps that.
	//
	// `command -v` probes existence with all output silenced; the exec
	// itself runs WITHOUT a stderr redirect, because bash writes its PS1
	// prompt to stderr via readline — redirecting fd 2 to /dev/null on
	// the exec swallows the prompt and the shell looks dead.
	args = append(args, "--", "/bin/sh", "-c",
		"if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi")
	return exec.Command("kubectl", args...)
}

// buildShellTerminalCmd assembles the user's login shell command for the
// internal terminal popup. Inherits env / cwd from kbu so the user's aliases,
// PATH, and current directory are exactly what they'd see in a regular
// terminal — like `ssh localhost` but embedded.
//
// Shell precedence: $KBU__ALTERM_SHELL > $KM8__ALTERM_SHELL (v2.0 legacy)
// > cfgShell (alterm_shell config) > $SHELL > /bin/sh. The env-var slot is
// for ad-hoc overrides (one-shot `KBU__ALTERM_SHELL=... kbu`) without
// editing the config; cfgShell is for persistent per-user preference;
// $SHELL is the host fallback. $KM8__ALTERM_SHELL is the pre-v2.0 name,
// still read this release so existing shell rc / launchctl plists don't
// silently break — EnvDeprecations logs a nudge when it's the value in
// effect. Remove next release.
//
// Login precedence mirrors the same 2-tier fallback: $KBU__ALTERM_LOGIN_SHELL
// > $KM8__ALTERM_LOGIN_SHELL > cfgLogin (alterm_login_shell). Default is
// non-login interactive — sources .bashrc / .zshrc, skips /etc/profile so
// macOS bash doesn't clobber the user's PS1. Flip true when launched from
// a non-login parent (Raycast/Alfred/cron/non-default tmux) and PATH lives
// in .zprofile / .bash_profile — without `-l` those dotfiles don't run and
// Alterm sees a stripped PATH.
//
// All env-derived candidates are TrimSpace'd before the empty-string
// check; otherwise a stray `KBU__ALTERM_SHELL=" /opt/.../fish"` (leading
// space from copy-paste or a sourced .env) would fall through to
// exec.Command verbatim and error with ENOENT.
func buildShellTerminalCmd(cfgShell string, cfgLogin bool) *exec.Cmd {
	sh := strings.TrimSpace(os.Getenv("KBU__ALTERM_SHELL"))
	if sh == "" {
		sh = strings.TrimSpace(os.Getenv("KM8__ALTERM_SHELL"))
	}
	if sh == "" {
		sh = strings.TrimSpace(cfgShell)
	}
	if sh == "" {
		sh = strings.TrimSpace(os.Getenv("SHELL"))
	}
	if sh == "" {
		sh = "/bin/sh"
	}

	login := cfgLogin
	loginEnv := strings.TrimSpace(os.Getenv("KBU__ALTERM_LOGIN_SHELL"))
	if loginEnv == "" {
		loginEnv = strings.TrimSpace(os.Getenv("KM8__ALTERM_LOGIN_SHELL"))
	}
	if loginEnv != "" {
		// strconv.ParseBool covers the Go-canonical truthy set
		// {1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False}
		// — same set kubectl + most Go CLIs accept. The previous
		// literal whitelist {true, 1, yes, TRUE, YES} silently
		// dropped `True`/`Yes`/`t`/`on` etc., which looked like Go-
		// idiomatic spellings to the user. Unrecognized values
		// leave login=cfgLogin (no override).
		if b, err := strconv.ParseBool(loginEnv); err == nil {
			login = b
		}
	}
	if login {
		return exec.Command(sh, "-l")
	}
	return exec.Command(sh)
}

// terminalTitle returns the popup title for the internal terminal — the
// host's name prefixed with the Alterm tag, mirroring how an ssh prompt
// identifies the connection. Returns os.Hostname() verbatim; mDNS-style
// suffixes like `.home` / `.local` / `.lan` are passed through, because
// the user said so (some routers append `.home` and the user wants to
// keep that visible).
func terminalTitle() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "Alterm"
	}
	return "Alterm: " + h
}

func (m AppModel) focusedPanelContent() string {
	switch m.activePanel {
	case SidebarPanel:
		return m.sidebar.CopyableContent()
	case TablePanel:
		return m.table.CopyableContent()
	case DetailPanel:
		return m.detail.CopyableContent()
	}
	return ""
}

// tablePanelBottomLeft returns the bottom-left border hint for panel 2.
// Composes two hotkey hints:
//   - `.` to toggle helm-managed visibility (hidden in Releases since
//     the entire list is helm-managed there)
//   - `Esc` to exit compare mode (only when a compare anchor is set,
//     since Esc otherwise has its standard back-out semantics elsewhere
//     — the hint is the discoverable affordance that "Exit compare
//     mode" used to live in the Space menu)
func (m *AppModel) tablePanelBottomLeft() []keyHint {
	var hints []keyHint
	if m.currentResource != k8s.ResourceReleases {
		hints = append(hints, keyHint{".", "helm"})
	}
	if m.inCompareMode() {
		hints = append(hints, keyHint{"Esc", "exit compare"})
	}
	return hints
}

// filterHelmIfHidden drops helm-managed items (and, for Secrets, also helm
// storage blobs) from the slice when the global helm-hide toggle is on.
// Helm Releases themselves are passed through untouched — the category
// IS helm. Returns the original slice unmodified when nothing is hidden.
func filterHelmIfHidden(items []k8s.ResourceItem, rt k8s.ResourceType) []k8s.ResourceItem {
	if !k8s.HelmHideManaged() || rt == k8s.ResourceReleases {
		return items
	}
	out := make([]k8s.ResourceItem, 0, len(items))
	for _, item := range items {
		if k8s.IsHelmManaged(item) {
			continue
		}
		if rt == k8s.ResourceSecrets && k8s.IsHelmStorageSecret(item) {
			continue
		}
		out = append(out, item)
	}
	return out
}

// augmentRowsWithHelm builds the panel-2 display rows: for namespaced
// resources it leads each row with the item's namespace ([E]), then Name,
// then the helm-marker cell, then the rest — dropping the resource's own
// Namespace cell if it ships one (hoisted to the front). This mirrors
// ColumnsForResource exactly (same namespaced / skip-index logic, same
// Namespace/Name/helm/rest order) so columns and cells stay aligned. Helm
// Releases get pass-through rows — their column set is already helm-
// specific (CHART / REV / STATUS / ...). Drill rows from container lists
// etc. don't pass through here, so they're unaffected.
func augmentRowsWithHelm(items []k8s.ResourceItem, rt k8s.ResourceType) [][]string {
	namespaced := isNamespacedResource(rt)
	skipIdx := -1
	if namespaced {
		skipIdx = ownNamespaceColumnIndex(rt)
	}
	rows := make([][]string, len(items))
	for i, item := range items {
		if rt == k8s.ResourceReleases || len(item.Row) == 0 {
			rows[i] = item.Row
			continue
		}
		out := make([]string, 0, len(item.Row)+2)
		if namespaced {
			out = append(out, item.Namespace) // Namespace leads
		}
		out = append(out, item.Row[0])        // Name
		out = append(out, k8s.MarkHelm(item)) // helm marker
		for j := 1; j < len(item.Row); j++ {
			if j == skipIdx {
				continue // resource's own namespace cell, hoisted to the front
			}
			out = append(out, item.Row[j])
		}
		rows[i] = out
	}
	return rows
}

// markCurrentContextRow appends a " *" marker to the Name cell of the row
// whose context name matches the live client's active context. "Current" is
// app runtime state — the C picker rebinds the client without writing the
// kubeconfig, so the disk current-context can differ from what kbu is
// actually connected to — hence this can't be derived in the registry
// fetcher and is stamped here where the active context is known. For the
// cluster-scoped Contexts resource, Name is column 0 (no leading Namespace
// column). No-op when nothing matches. Marks after sort so the "*" never
// affects row ordering.
func markCurrentContextRow(rows [][]string, current string) {
	if current == "" {
		return
	}
	for i := range rows {
		if len(rows[i]) > 0 && rows[i][0] == current {
			rows[i][0] += " *"
			return
		}
	}
}

// clearSearchOnLeave drops the search state of `from` when focus moves
// away from it. Other panels' search states are untouched — only the
// panel being left loses its filter, on the theory that search is a
// short-lived nav aid the user has already finished using once they've
// changed focus.
func (m *AppModel) clearSearchOnLeave(from Panel) {
	switch from {
	case SidebarPanel:
		m.sidebar.ClearSearch()
	case TablePanel:
		m.table.ClearSearch()
	case DetailPanel:
		m.detail.ClearSearch()
	}
}

func (m *AppModel) setPanel(p Panel) {
	if p != m.activePanel {
		m.clearSearchOnLeave(m.activePanel)
		m.exitCompareOnLeave(m.activePanel, p)
	}
	m.activePanel = p
	m.updateFocus()
}

func (m *AppModel) cyclePanel() {
	from := m.activePanel
	switch m.activePanel {
	case SidebarPanel:
		m.activePanel = TablePanel
	case TablePanel:
		m.activePanel = DetailPanel
	case DetailPanel:
		m.activePanel = SidebarPanel
	}
	m.clearSearchOnLeave(from)
	m.exitCompareOnLeave(from, m.activePanel)
	m.updateFocus()
}

func (m *AppModel) cyclePanelReverse() {
	from := m.activePanel
	switch m.activePanel {
	case SidebarPanel:
		m.activePanel = DetailPanel
	case TablePanel:
		m.activePanel = SidebarPanel
	case DetailPanel:
		m.activePanel = TablePanel
	}
	m.exitCompareOnLeave(from, m.activePanel)
	m.updateFocus()
}

// exitCompareOnLeave drops compare mode the instant focus moves out of
// panel 2 — compare actions only make sense while the user is
// navigating the list, so leaving for sidebar / detail releases the
// lock without ceremony. Hook is also fed the destination so other
// future "leaving X for Y" rules can attach here.
func (m *AppModel) exitCompareOnLeave(from, to Panel) {
	if from == TablePanel && to != TablePanel && m.inCompareMode() {
		m.clearCompareLock()
	}
}

// dropCompareLockIfMissing scans the freshly delivered watcher items
// for the locked UID. If absent (delete event / namespace change /
// row simply scrolled out of scope), drops the lock and returns a
// toast Cmd to notify the user. Returns nil otherwise.
func (m *AppModel) dropCompareLockIfMissing(items []k8s.ResourceItem) tea.Cmd {
	if !m.inCompareMode() {
		return nil
	}
	for _, it := range items {
		if it.UID == m.compareLock.uid {
			return nil
		}
	}
	missing := fmt.Sprintf("%s/%s", m.compareLock.resourceType.KubectlName(), m.compareLock.name)
	m.clearCompareLock()
	m.appLog.Info("compare: locked item gone — " + missing)
	return m.toast.Show("compare: locked item gone")
}

func (m *AppModel) updateFocus() {
	m.sidebar.SetFocused(m.activePanel == SidebarPanel)
	m.table.SetFocused(m.activePanel == TablePanel)
	m.detail.SetFocused(m.activePanel == DetailPanel)
	m.statusLine.SetActivePanel(m.activePanel)
	m.statusLine.SetDrillDown(m.drillDownPod != nil)
	m.statusBar.SetActivePanel(m.activePanel)
}

// ScrollInfo holds position info for the "X of Y" indicator in a panel border.
type ScrollInfo struct {
	Position int // 1-based current position
	Total    int
}

// focusedPanelTitle builds a Panel 1/2 border title. Layout (both focus
// states): <E0B6>[N] body<E0B4> — round left cap + single filled chip
// wrapping prefix & body + round right cap. Chip fg = Catppuccin base; bg
// = border color (blue when focused, surface2 when not).
func focusedPanelTitle(prefix, body string, t *theme.Theme, focused bool) string {
	borderHex := t.Detail.BorderColor
	if focused {
		borderHex = t.Sidebar.CategoryFg
	}
	bc := lipgloss.Color(borderHex)
	capStyle := lipgloss.NewStyle().Foreground(bc)
	chipStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1e1e2e")).
		Background(bc).
		Bold(true)
	return capStyle.Render("\uE0B6") +
		chipStyle.Render(prefix+" "+body) +
		capStyle.Render("\uE0B4")
}

// plainTitlePrefix wraps the "[N]" panel-id in a Powerline chip matching
// focusedPanelTitle's shape: <E0B6>[N]<E0B0>. Panel 3 uses this because
// its body (the tab bar from DetailModel.TabTitle) carries its own per-tab
// chip system; sharing the same panel-id chip keeps the visual language
// unified across all three panels.
func plainTitlePrefix(prefix string, t *theme.Theme, focused bool) string {
	borderHex := t.Detail.BorderColor
	if focused {
		borderHex = t.Sidebar.CategoryFg
	}
	bc := lipgloss.Color(borderHex)
	capStyle := lipgloss.NewStyle().Foreground(bc)
	chipStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1e1e2e")).
		Background(bc).
		Bold(true)
	// Open the [N] chip with a round left cap; TabTitle handles closing
	// (either merges with the first tab when active, or emits its own
	// close cap into the base tab area).
	return capStyle.Render("\uE0B6") + chipStyle.Render(prefix)
}

func renderPanel(content, title string, width, height int, focused bool, t *theme.Theme) string {
	return renderPanelWithScroll(content, title, width, height, focused, t, nil, "", nil)
}

func renderPanelWithScroll(content, title string, width, height int, focused bool, t *theme.Theme, scroll *ScrollInfo, topRight string, bottomLeft []keyHint) string {
	if width < 4 || height < 3 {
		return content
	}

	borderColor := t.Detail.BorderColor
	if focused {
		borderColor = t.Sidebar.CategoryFg
	}
	bc := lipgloss.Color(borderColor)
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)

	// Focused panel uses double-line box-drawing chars; unfocused uses
	// light rounded chars. Same cell count, no layout shift — just a
	// different glyph weight as a stronger focus signal. Both sets are
	// widely supported by Nerd-Font monospaced terminal fonts.
	tl, tr, bl, br, horiz, vert := "╭", "╮", "╰", "╯", "─", "│"
	if focused {
		tl, tr, bl, br, horiz, vert = "╔", "╗", "╚", "╝", "═", "║"
	}

	innerW := width - 2
	innerH := height - 2

	lines := strings.Split(content, "\n")
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	lines = lines[:innerH]

	var b strings.Builder

	titleVis := lipgloss.Width(title)
	// Top-right hint format: " <hint>─" (leading space + hint + 1 dash
	// before the corner). Drop the hint silently if title+hint+1 dash
	// would overflow innerW — small terminals get plain border.
	hintVis := 0
	if topRight != "" {
		hintVis = lipgloss.Width(topRight) + 2
		if titleVis+hintVis+1 > innerW {
			hintVis = 0
			topRight = ""
		}
	}
	dashesAfter := innerW - 1 - titleVis - hintVis
	if dashesAfter < 0 {
		dashesAfter = 0
	}
	b.WriteString(bStyle.Render(tl + horiz))
	b.WriteString(title)
	b.WriteString(bStyle.Render(strings.Repeat(horiz, dashesAfter)))
	if topRight != "" {
		b.WriteString(bStyle.Render(" "))
		b.WriteString(tStyle.Render(topRight))
		b.WriteString(bStyle.Render(horiz))
	}
	b.WriteString(bStyle.Render(tr))
	b.WriteString("\n")

	leftBorder := bStyle.Render(vert)
	rightBorder := bStyle.Render(vert)
	emptyLine := strings.Repeat(" ", innerW)
	for _, line := range lines {
		lw := lipgloss.Width(line)
		if lw > innerW {
			line = ansiTruncate(line, innerW)
			lw = lipgloss.Width(line)
		}
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		if line == "" {
			b.WriteString(leftBorder + emptyLine + rightBorder)
		} else {
			b.WriteString(leftBorder + line + pad + rightBorder)
		}
		b.WriteString("\n")
	}

	// Bottom-left optional hint (panel 2's `.:helm`, panel 3's tab keys),
	// with a single dash either side as separator. Two colours like every
	// hint (tdp M5): the family pair on the focused panel, a darker pair
	// that recedes with an unfocused one. Kept short by callers; if it
	// doesn't fit alongside the scroll indicator we drop it silently.
	leftHintRendered := ""
	leftHintVis := 0
	if len(bottomLeft) > 0 {
		colours := recededHint()
		if focused {
			colours = brightHint()
		}
		leftHintVis = lipgloss.Width(hintText(bottomLeft)) + 2 // dash + content + dash
		leftHintRendered = bStyle.Render(horiz) + renderHints(bottomLeft, colours) + bStyle.Render(horiz)
	}

	if scroll != nil && scroll.Total > 0 {
		indicator := fmt.Sprintf(" %d of %d ", scroll.Position, scroll.Total)
		dashes := innerW - len(indicator) - leftHintVis
		if dashes < 0 {
			dashes = 0
			// Indicator + leftHint overflowed innerW. Drop the hint
			// rather than truncating the more-useful scroll indicator.
			leftHintRendered = ""
			dashes = innerW - len(indicator)
			if dashes < 0 {
				dashes = 0
			}
		}
		b.WriteString(bStyle.Render(bl) + leftHintRendered + bStyle.Render(strings.Repeat(horiz, dashes)+indicator+br))
	} else {
		dashes := innerW - leftHintVis
		if dashes < 0 {
			dashes = 0
			leftHintRendered = ""
			dashes = innerW
		}
		b.WriteString(bStyle.Render(bl) + leftHintRendered + bStyle.Render(strings.Repeat(horiz, dashes)+br))
	}

	return b.String()
}

// namespaceSelectionLabel formats a namespace selection for the status
// bar: "" for All (SetNamespace renders that as "All Namespaces"), the
// single namespace name for a one-namespace selection, or "N selected"
// for a multi-namespace selection ([G]). "N selected" rather than a name
// list keeps the status bar width stable and reads cleanly after the
// "Namespace:" prefix.
func namespaceSelectionLabel(sel k8s.NamespaceSelection) string {
	if sel.IsAll() {
		return ""
	}
	if sel.Count() == 1 {
		return sel.List()[0]
	}
	return fmt.Sprintf("%d selected", sel.Count())
}

func fetchNamespaces(client *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		items, err := k8s.FetchResources(context.Background(), client.Clientset(), k8s.ResourceNamespaces, "")
		if err != nil {
			return NamespaceListMsg{Err: err}
		}
		names := make([]string, len(items))
		for i, item := range items {
			names[i] = item.Name
		}
		return NamespaceListMsg{Namespaces: names}
	}
}

// validateNamespaceSelection lists the live namespaces once at startup so
// a persisted selection can be reconciled against what still exists. Same
// query as fetchNamespaces but tagged with namespaceValidationMsg so it
// drives selection validation rather than populating the picker.
func validateNamespaceSelection(client *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		items, err := k8s.FetchResources(context.Background(), client.Clientset(), k8s.ResourceNamespaces, "")
		if err != nil {
			return namespaceValidationMsg{Err: err}
		}
		names := make([]string, len(items))
		for i, item := range items {
			names[i] = item.Name
		}
		return namespaceValidationMsg{Namespaces: names}
	}
}

func fetchContexts(client *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		contexts := client.ListContexts()
		current := client.ContextName()
		return ContextListMsg{Contexts: contexts, Current: current}
	}
}

// buildKubectlEditCmd assembles the `kubectl edit` command to run inside the
// PtyView. cfgEditor (from config.yaml) is exposed as $KUBE_EDITOR so kubectl
// honors the user's choice; if empty, kubectl falls back to $EDITOR / $VISUAL
// / vi / notepad on its own.
//
// The env is sanitized: vt10x is a basic VT100/xterm emulator and doesn't
// respond to advanced queries (DA1, color reports, etc.) that editors like
// nvim send when they detect terminal-program env vars (TERM_PROGRAM,
// KITTY_*, ITERM_*). Inheriting those values causes nvim to wait for query
// responses and time out on exit, producing a noticeable lag.
func buildKubectlEditCmd(rt k8s.ResourceType, item k8s.ResourceItem, contextName, cfgEditor string) *exec.Cmd {
	args := []string{"edit", rt.KubectlName() + "/" + item.Name}
	if item.Namespace != "" {
		args = append(args, "-n", item.Namespace)
	}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}
	c := exec.Command("kubectl", args...)
	c.Env = sanitizeEditorEnv(cfgEditor)
	return c
}

func sanitizeEditorEnv(cfgEditor string) []string {
	strip := []string{
		"TERM_PROGRAM",
		"TERM_PROGRAM_VERSION",
		"TERM_SESSION_ID",
		"KITTY_WINDOW_ID",
		"KITTY_PUBLIC_KEY",
		"ITERM_SESSION_ID",
		"ITERM_PROFILE",
		"LC_TERMINAL",
		"LC_TERMINAL_VERSION",
		"WEZTERM_EXECUTABLE",
		"WEZTERM_PANE",
		"GHOSTTY_RESOURCES_DIR",
		"COLORTERM",
		"TERM",
		"KUBE_EDITOR",
	}
	stripSet := make(map[string]struct{}, len(strip))
	for _, k := range strip {
		stripSet[k] = struct{}{}
	}
	env := make([]string, 0, len(os.Environ()))
	for _, v := range os.Environ() {
		eq := strings.IndexByte(v, '=')
		if eq < 0 {
			env = append(env, v)
			continue
		}
		if _, drop := stripSet[v[:eq]]; drop {
			continue
		}
		env = append(env, v)
	}
	env = append(env, "TERM=xterm-256color")
	if cfgEditor != "" {
		env = append(env, "KUBE_EDITOR="+cfgEditor)
	}
	return env
}

// deleteConfirmSurface returns the confirm popup's (message, detail)
// pair for a kubectl delete target. Namespace deletes get a stronger
// warning because the action cascades to every resource in the ns —
// a generic "delete resource?" prompt would understate the blast
// radius. Cluster-scoped kinds also drop the "-n <namespace>" tail
// from the detail line so the popup mirrors the actual kubectl call
// (which deleteResource() constructs from the same empty-ns signal).
//
// Kept as a package-level function (not a method on AppModel) so both
// delete-trigger sites — d hotkey (2100s) and Space menu Delete
// (3020s) — share one text source and tests can hit it directly.
func deleteConfirmSurface(rt k8s.ResourceType, item k8s.ResourceItem) (message, detail string) {
	if rt == k8s.ResourceNamespaces {
		return fmt.Sprintf("!!! Delete namespace %q? This will remove ALL resources inside it. Cannot be undone.", item.Name),
			fmt.Sprintf("kubectl delete namespace %s", item.Name)
	}
	if item.Namespace == "" {
		// Other cluster-scoped kinds (ClusterRoles / CRDs / PVs / …) —
		// no -n flag. Message stays the shared default.
		return "⚠ Delete resource? This cannot be undone.",
			fmt.Sprintf("kubectl delete %s %s", rt.KubectlName(), item.Name)
	}
	return "⚠ Delete resource? This cannot be undone.",
		fmt.Sprintf("kubectl delete %s %s -n %s", rt.KubectlName(), item.Name, item.Namespace)
}

func deleteResource(rt k8s.ResourceType, name, namespace, contextName string) tea.Cmd {
	return func() tea.Msg {
		args := []string{"delete", rt.KubectlName(), name}
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		if contextName != "" {
			args = append(args, "--context", contextName)
		}
		c := exec.Command("kubectl", args...)
		var buf bytes.Buffer
		c.Stdout = &buf
		c.Stderr = &buf
		if err := c.Run(); err != nil {
			return DeleteErrMsg{Err: err}
		}
		return DeleteDoneMsg{
			Name:      name,
			Namespace: namespace,
			Resource:  string(rt.KubectlName()) + "/" + name,
			Output:    buf.String(),
		}
	}
}

func waitForWatchUpdate(w *k8s.Watcher, rt k8s.ResourceType) tea.Cmd {
	updates, errors := w.Channels()
	return func() tea.Msg {
		select {
		case msg, ok := <-updates:
			if !ok {
				return nil // channel closed by watcher.Start(); caller must not re-register
			}
			return ResourceDataMsg{Type: rt, Items: msg.Items}
		case errMsg, ok := <-errors:
			if !ok {
				return nil
			}
			return ResourceErrorMsg{Err: errMsg.Err}
		}
	}
}

func fetchResourceDetail(client *k8s.Client, rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				config.WriteCrashLog(r)
			}
		}()
		ctx := context.Background()
		detail := k8s.GetResourceDetail(rt, item)
		detail.YAML = k8s.MarshalItemYAML(item)
		// Kind-specific Relatives data that needs an API call (Service →
		// selector→pods, ClusterRole → bindings, StorageClass → PVCs, ...).
		// EnrichRelatives is a no-op for kinds without extra resolution.
		k8s.EnrichRelatives(ctx, client.Clientset(), rt, item, &detail)
		detail.Conditions = k8s.ExtractConditions(item)
		events, _ := k8s.FetchResourceEventsAggregated(ctx, client.Clientset(), item)
		return ResourceDetailMsg{ItemUID: item.UID, Detail: detail, Events: events}
	}
}

// startAggregateLogs resolves a workload item to its current pod set and emits
// aggregateLogsReadyMsg with the targets. Runs off the Update path so the API
// list call doesn't block the UI. Includes the source item's UID so a stale
// result (e.g. user navigated to a different row in the meantime) can be
// filtered out by the handler.
func startAggregateLogs(client *k8s.Client, resource k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	return func() tea.Msg {
		targets, err := k8s.PodsForWorkload(context.Background(), client.Clientset(), item, true)
		return aggregateLogsReadyMsg{
			resource: resource,
			itemUID:  item.UID,
			targets:  targets,
			err:      err,
		}
	}
}

func waitForLogLine(ls *k8s.LogStreamer) tea.Cmd {
	// Capture the channel reference at Cmd-construction time, NOT inside
	// the goroutine body. Bubble Tea may process another message (e.g.
	// RowSelectedMsg's default branch, NamespaceChangedMsg, exitDrillDown)
	// between this Cmd's dispatch and its goroutine first instruction;
	// that Update can call ls.Stop() which closes the channel AND nils
	// ls.lines. If we read ls.Lines() inside the goroutine, we'd get
	// nil and `<-nil` blocks forever (Go spec) — one parked goroutine
	// leaked per race occurrence. Reading at construction-time binds
	// the closed-old-channel; the receive then unblocks with !ok and
	// we return nil cleanly.
	//
	// Nil-channel short-circuit: if Stop has already nilled ls.lines
	// (Stop→StartMulti async gap during a LogLineMsg re-arm), return
	// a nil Cmd. Bubble Tea skips nil Cmds without spawning a
	// goroutine; equivalent to a no-op without the unnecessary
	// goroutine + channel send + msg-drop cycle the closure form
	// would introduce.
	ch := ls.Lines()
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil
		}
		return LogLineMsg{
			StreamID:  line.StreamID,
			Pod:       line.Pod,
			Container: line.Container,
			Text:      line.Text,
		}
	}
}

// wrapWords wraps s at word boundaries to fit within width. Words longer than
// width are broken mid-word.
func wrapWords(s string, width int) []string {
	if width <= 0 || s == "" {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	var current string
	for _, w := range words {
		for len(w) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			lines = append(lines, w[:width])
			w = w[width:]
		}
		if current == "" {
			current = w
		} else if len(current)+1+len(w) <= width {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
