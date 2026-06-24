package ui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/vulcanshen/km8/internal/config"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

type resourceSwitchTickMsg struct {
	seq int
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
	contextPicker   ContextPickerModel
	help            HelpModel
	appLog          AppLogModel
	confirm         ConfirmModel
	splash          SplashModel
	toast           ToastModel
	// Dual-slot PTY: the persistent KM8erm shell (shellPty) and any
	// transient PTY for kubectl edit / exec (txPty) live independently so
	// the user can keep a long-running shell hidden in the background while
	// editing or exec'ing into a container. tx-on-top is enforced at render
	// + input routing time so only one popup is visible at a time.
	shellPty        *PtyView
	txPty           *PtyView
	yamlPopup       YamlPopupModel
	comparePopup    CompareYamlPopupModel
	breadcrumbPopup BreadcrumbPopupModel
	helmDocMenu     HelmDocMenuPopupModel
	panel2Menu      Panel2MenuPopupModel
	hintPopup       HintPopupModel
	listPicker      ListPickerModel
	settingsPopup   SettingsPopupModel

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
	detailExpanded  bool
	tableExpanded   bool
	switchSeq       int

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

	// Sort flow in-flight state. The Sort menu is a 3-popup chain
	// (sidebar hint → column picker → direction picker); these fields
	// carry the user's column choice across the column → direction
	// step so the direction commit knows which column to persist.
	// Cleared on direction commit, on cancel at either step, and on
	// any path that closes the listPicker. Empty kind/column = no
	// flow in progress.
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

// openSortColumnPicker opens the listPicker as the first step of the
// Sort flow. Items are the kind's column titles; the column currently
// in use (if any) is badged with its direction arrow so the user
// sees where they are now. Caches kind in sortFlowKind so the
// direction step knows what kind it's committing for even if the
// sidebar cursor drifts mid-flow.
func (m *AppModel) openSortColumnPicker(rt k8s.ResourceType) tea.Cmd {
	def := sortRegistry().Get(rt)
	if def == nil || len(def.Columns) == 0 {
		return nil
	}
	m.sortFlowKind = rt
	m.sortFlowColumn = ""
	current := m.cfg.GetSort(def.KubectlName)
	items := make([]ListPickerItem, 0, len(def.Columns))
	for _, c := range def.Columns {
		it := ListPickerItem{Key: c.Title, Label: c.Title}
		if current != nil && current.Column == c.Title {
			it.Badge = sortDirectionGlyph(current.Direction)
		}
		items = append(items, it)
	}
	title := "Sort " + def.DisplayName + " by…"
	m.listPicker.SetSize(m.width, m.height)
	return m.listPicker.Open("sort:column", title, items)
}

// openSortDirectionPicker is the second step. Items are
// Ascending / Descending / Unset; the current direction is badged
// "current" if the picked column matches the existing sort entry.
// Unset is offered unconditionally — but commit logic treats it as
// no-op when the column isn't currently sorted, matching the
// "selecting Unset on a never-sorted column does nothing"
// agreement.
func (m *AppModel) openSortDirectionPicker(rt k8s.ResourceType, column string) tea.Cmd {
	def := sortRegistry().Get(rt)
	if def == nil {
		return nil
	}
	m.sortFlowColumn = column
	current := m.cfg.GetSort(def.KubectlName)
	items := []ListPickerItem{
		{Key: config.SortDirectionAscending, Label: "Ascending"},
		{Key: config.SortDirectionDescending, Label: "Descending"},
		{Key: "unset", Label: "Unset"},
	}
	if current != nil && current.Column == column {
		for i := range items {
			if items[i].Key == current.Direction {
				items[i].Badge = "current"
			}
		}
	}
	title := "Sort " + def.DisplayName + " by " + column + "…"
	m.listPicker.SetSize(m.width, m.height)
	return m.listPicker.Open("sort:direction", title, items)
}

// commitSortFlow finalises the Sort flow once the user picks a
// direction. Persists to config (or unsets, if the user chose
// "unset" AND the picked column is the currently-sorted column),
// re-applies sort to the live items so panel-2 reflects the change
// immediately, then closes the picker.
//
// The "unset on never-sorted column" no-op gate matches the design
// agreement: picking Unset on a column that wasn't sorted should
// do nothing, not silently clobber any unrelated sort the user
// might have on a different column.
func (m *AppModel) commitSortFlow(direction string) tea.Cmd {
	rt := m.sortFlowKind
	column := m.sortFlowColumn
	m.sortFlowKind = ""
	m.sortFlowColumn = ""
	closeCmd := m.listPicker.Close()
	if rt == "" || column == "" || m.cfg == nil {
		return closeCmd
	}
	def := sortRegistry().Get(rt)
	if def == nil {
		return closeCmd
	}
	current := m.cfg.GetSort(def.KubectlName)
	switch direction {
	case "unset":
		if current == nil || current.Column != column {
			return closeCmd
		}
		m.cfg.UnsetSort(def.KubectlName)
	case config.SortDirectionAscending, config.SortDirectionDescending:
		m.cfg.SetSort(def.KubectlName, column, direction)
	default:
		return closeCmd
	}
	var saveErrCmd tea.Cmd
	if err := m.cfg.Save(); err != nil {
		// In-memory state already mutated — surface the disk-side
		// failure via both the app log (full error) and a toast (so
		// the user actually notices). Matches togglePinnedKind's
		// behaviour; without this the user would think their sort
		// stuck across restarts when it didn't.
		m.appLog.Error("sort save failed: " + err.Error())
		saveErrCmd = m.toast.Show("sort save failed")
	}
	// Re-apply sort to whatever's currently in panel 2 if this
	// kind is the one being viewed — no point waiting for the next
	// watcher tick.
	if rt == m.currentResource {
		m.syncTableSortIndicator()
		m.applySortToItems()
		rows := augmentRowsWithHelm(m.items, m.currentResource)
		m.table.SetRows(rows)
	}
	return tea.Batch(closeCmd, saveErrCmd)
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
	sortCfg := m.cfg.GetSort(def.KubectlName)
	if sortCfg == nil {
		sort.SliceStable(m.items, func(i, j int) bool {
			if m.items[i].Namespace != m.items[j].Namespace {
				return m.items[i].Namespace < m.items[j].Namespace
			}
			return m.items[i].Name < m.items[j].Name
		})
		return
	}
	asc := sortCfg.Direction == config.SortDirectionAscending
	k8s.SortItems(m.items, def.Columns, sortCfg.Column, asc)
}

// syncTableSortIndicator pushes the current kind's saved sort
// (column + direction) to the table so the panel-2 header renders
// the right arrow. Called wherever sort state can change relative
// to the table: kind switch, sort commit, app init. Empty kind /
// empty config clears the indicator.
func (m *AppModel) syncTableSortIndicator() {
	if m.cfg == nil || m.currentResource == "" {
		m.table.SetSortIndicator("", "")
		return
	}
	def := sortRegistry().Get(m.currentResource)
	if def == nil {
		m.table.SetSortIndicator("", "")
		return
	}
	if s := m.cfg.GetSort(def.KubectlName); s != nil {
		m.table.SetSortIndicator(s.Column, s.Direction)
		return
	}
	m.table.SetSortIndicator("", "")
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
		if isDouble {
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
// disappeared mid-session, or were never installed when km8 started
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

func NewAppModel(t *theme.Theme, client *k8s.Client, cfg *config.Config) AppModel {
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

	return AppModel{
		sidebar:         sidebar,
		table:           NewTableModel(t),
		detail:          detail,
		statusBar:       NewStatusBarModel(t, info),
		statusLine:      NewStatusLineModel(t),
		namespacePicker: NewNamespacePickerModel(t),
		contextPicker:   NewContextPickerModel(t),
		help:            NewHelpModel(t),
		appLog:          NewAppLogModel(t),
		confirm:         NewConfirmModel(t),
		splash:          NewSplashModel(),
		toast:           NewToastModel(t),
		shellPty:        NewPtyView(),
		txPty:           NewPtyView(),
		yamlPopup:       NewYamlPopupModel(t),
		comparePopup:    newCompareModel,
		breadcrumbPopup: NewBreadcrumbPopupModel(t),
		helmDocMenu:     NewHelmDocMenuPopupModel(t),
		panel2Menu:      NewPanel2MenuPopupModel(t),
		hintPopup:       NewHintPopupModel(t),
		listPicker:      NewListPickerModel(t),
		settingsPopup:   NewSettingsPopupModel(t),
		activePanel:     SidebarPanel,
		theme:           t,
		cfg:             cfg,
		cfgEditor:       cfg.Editor,
		k8sClient:       client,
		watcher:         watcher,
		logStreamer:     logStreamer,
		currentResource: k8s.ResourcePods,
	}
}

type appInitMsg struct{ info k8s.ClusterInfo }

func (m AppModel) Init() tea.Cmd {
	m.watcher.Start(k8s.ResourcePods, m.k8sClient.GetNamespace())
	info := m.k8sClient.GetClusterInfo()
	return tea.Batch(
		m.sidebar.Init(),
		m.table.Init(),
		waitForWatchUpdate(m.watcher, m.currentResource),
		discoverCRDs(m.k8sClient),
		func() tea.Msg { return appInitMsg{info: info} },
	)
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

	if _, ok := msg.(quitConfirmedMsg); ok {
		m.watcher.Stop()
		m.logStreamer.Stop()
		// Kill any persistent PTY (hidden KM8erm shell, mid-edit, mid-exec)
		// so we don't orphan the subprocess after km8 exits.
		if m.shellPty != nil && m.shellPty.IsAlive() {
			m.shellPty.Stop()
		}
		if m.txPty != nil && m.txPty.IsAlive() {
			m.txPty.Stop()
		}
		return m, tea.Quit
	}

	// detailSpinnerTickMsg drives the panel 3 refetch spinner; routed
	// unconditionally regardless of focus so the spinner keeps animating even
	// while the user is on Sidebar/Table panels.
	if _, ok := msg.(detailSpinnerTickMsg); ok {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
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
		if c := m.helmDocMenu.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.panel2Menu.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.hintPopup.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.listPicker.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.settingsPopup.HandleTick(tickMsg); c != nil {
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
	//     routing — KM8erm-hidden keeps the shell alive in background.
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
		case tea.KeyMsg:
			if m.txPty.IsActive() {
				var cmd tea.Cmd
				m.txPty, cmd = m.txPty.Update(msg)
				return m, cmd
			}
			if m.shellPty.IsActive() {
				var cmd tea.Cmd
				m.shellPty, cmd = m.shellPty.Update(msg)
				return m, cmd
			}
			// All hidden: fall through (Alt+T re-shows KM8erm, etc.)
		}
	}

	if m.confirm.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.confirm, cmd = m.confirm.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.appLog.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			var cmd tea.Cmd
			m.appLog, cmd = m.appLog.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.help.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			var cmd tea.Cmd
			m.help, cmd = m.help.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.contextPicker.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			var cmd tea.Cmd
			m.contextPicker, cmd = m.contextPicker.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.namespacePicker.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			var cmd tea.Cmd
			m.namespacePicker, cmd = m.namespacePicker.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.yamlPopup.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.yamlPopup, cmd = m.yamlPopup.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.comparePopup.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.comparePopup, cmd = m.comparePopup.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.breadcrumbPopup.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.breadcrumbPopup, cmd = m.breadcrumbPopup.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.helmDocMenu.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.helmDocMenu, cmd = m.helmDocMenu.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.hintPopup.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.hintPopup, cmd = m.hintPopup.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.panel2Menu.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.panel2Menu, cmd = m.panel2Menu.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.listPicker.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.listPicker, cmd = m.listPicker.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.settingsPopup.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.settingsPopup, cmd = m.settingsPopup.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
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
		// Mouse routing. Layered:
		//   1. MouseEnabled gate (short-circuit when user has it off)
		//   2. Wheel → synthesize j/k unconditionally. The synthesized
		//      KeyMsg follows the normal keyboard routing, so the
		//      wheel naturally scrolls whichever popup or panel is
		//      currently active — no per-popup wheel handler needed.
		//   3. Settings popup owns its own click (toggle on row).
		//   4. Other interactive popups swallow non-wheel clicks so
		//      a stray click can't dismiss a keyboard-driven modal.
		//   5. Otherwise, handleMousePress for the main 3 panels.
		if m.cfg != nil && !m.cfg.IsMouseEnabled() {
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			// Wheel translates to half-page move (u / d), not single-
			// row j / k. Single-row felt too coarse-grained per-tick
			// and required many spins to traverse a long list. u / d
			// are bound across sidebar / table / detail (and the
			// viewer popups: yamlpopup, comparepopup, applog), so the
			// wheel works wherever the user might land.
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
		// Forward to whichever interactive popup is on top. Each
		// popup's HandleMouse owns its own hit-test (popup rect,
		// row offsets) and decides what a click commits.
		if m.settingsPopup.IsActive() {
			var cmd tea.Cmd
			m.settingsPopup, cmd = m.settingsPopup.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.panel2Menu.IsActive() {
			var cmd tea.Cmd
			m.panel2Menu, cmd = m.panel2Menu.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.listPicker.IsActive() {
			var cmd tea.Cmd
			m.listPicker, cmd = m.listPicker.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.namespacePicker.IsActive() {
			var cmd tea.Cmd
			m.namespacePicker, cmd = m.namespacePicker.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.hintPopup.IsActive() {
			var cmd tea.Cmd
			m.hintPopup, cmd = m.hintPopup.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		// Remaining popups all have HandleMouse now. List-style
		// popups commit on left-click; scroll-only / dialog popups
		// close on right-click; left-click is no-op everywhere a
		// stray click could fire a destructive or surprising
		// action (confirm dialogs especially).
		if m.confirm.IsActive() {
			var cmd tea.Cmd
			m.confirm, cmd = m.confirm.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.help.IsActive() {
			var cmd tea.Cmd
			m.help, cmd = m.help.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.appLog.IsActive() {
			var cmd tea.Cmd
			m.appLog, cmd = m.appLog.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.contextPicker.IsActive() {
			var cmd tea.Cmd
			m.contextPicker, cmd = m.contextPicker.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.yamlPopup.IsActive() {
			var cmd tea.Cmd
			m.yamlPopup, cmd = m.yamlPopup.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.comparePopup.IsActive() {
			var cmd tea.Cmd
			m.comparePopup, cmd = m.comparePopup.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.breadcrumbPopup.IsActive() {
			var cmd tea.Cmd
			m.breadcrumbPopup, cmd = m.breadcrumbPopup.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		if m.helmDocMenu.IsActive() {
			var cmd tea.Cmd
			m.helmDocMenu, cmd = m.helmDocMenu.HandleMouse(msg, m.width, m.height)
			return m, cmd
		}
		cmd := m.handleMousePress(msg)
		return m, cmd

	case tea.KeyMsg:
		// When any panel is in search mode, only ctrl+c passes through.
		searching := (m.activePanel == TablePanel && m.table.IsSearching()) ||
			(m.activePanel == SidebarPanel && m.sidebar.IsSearching()) ||
			(m.activePanel == DetailPanel && m.detail.IsSearching())
		if searching {
			switch msg.String() {
			case "ctrl+c":
				m.watcher.Stop()
				m.logStreamer.Stop()
				return m, tea.Quit
			}
			break
		}
		switch msg.String() {
		case "ctrl+c":
			m.watcher.Stop()
			m.logStreamer.Stop()
			return m, tea.Quit
		case "q":
			quitCmd := func() tea.Msg {
				return quitConfirmedMsg{}
			}
			return m, m.confirm.Show(ConfirmQuit, "Quit km8?", "", quitCmd)
		case "V":
			return m, m.splash.Show()
		case "M":
			// Global Settings popup. Opens from any panel; popups
			// already-open intercept keys earlier so M while inside
			// another popup is naturally a no-op. Items rebuilt
			// from current config every Open so the badge reflects
			// state on each re-entry.
			m.settingsPopup.SetSize(m.width, m.height)
			return m, m.settingsPopup.Open(m.buildSettingsItems())
		case "alt+t", "alt+T", "ctrl+t":
			// Alt+T is the single KM8erm toggle:
			//   - no shell alive   → spawn KM8erm
			//   - alive, hidden    → reattach (show)
			//   - alive, visible   → handled inside PtyView.Update (hides)
			// The "visible" branch never reaches here because PtyView
			// intercepts keys when IsActive() is true. Edit/Exec PTYs alive:
			// refuse, same as table-level edit/shell guard.
			//
			// Ctrl+T is a hidden alias for the demo recorder only: vhs 0.11
			// drops the Alt modifier between Chrome and the PTY (logged
			// keypress = `t` or `ctrl+t`, never `alt+t`), so demo tapes
			// emit Ctrl+T instead. Humans never see this alias in help/UI
			// hints — the cost of accepting it is that pressing Ctrl+T
			// while a KM8erm shell is visible will hide the shell instead
			// of forwarding to zsh's transpose-chars binding.
			// Dual-slot: KM8erm lives in shellPty only. txPty (edit/exec)
			// being alive does NOT block KM8erm — they can coexist; tx
			// visibility takes precedence so we just hide tx? No — tx is
			// transient and the user explicitly launched it; better to
			// surface KM8erm under it. Currently: if tx visible, KM8erm
			// hide/show is harmless (render still picks tx on top).
			if m.shellPty.IsAlive() {
				m.shellPty.Show(m.width, m.height)
				return m, nil
			}
			cmd := buildShellTerminalCmd()
			return m, m.shellPty.Start(cmd, terminalTitle(), m.width, m.height, PtyKindShell)
		case "?":
			m.help.SetSize(m.width, m.height)
			return m, m.help.Toggle()
		case "!":
			m.appLog.SetSize(m.width, m.height)
			return m, m.appLog.Toggle()
		case "1":
			m.detailExpanded = false
			m.setPanel(SidebarPanel)
			return m, nil
		case "2":
			m.detailExpanded = false
			m.setPanel(TablePanel)
			return m, nil
		case "3":
			m.detailExpanded = false
			m.setPanel(DetailPanel)
			return m, nil
		case "tab":
			m.cyclePanel()
			return m, nil
		case "shift+tab":
			m.cyclePanelReverse()
			return m, nil
		case "h":
			// v1.5.x: h/l switch the panel 3 detail tab ONLY when panel 3
			// is the active panel. Panel 1/2 = no-op (panel 2 was the
			// previous owner — moved to panel 3 so tab nav and list nav
			// live on different panels). `l` is no longer a drill key
			// either; Enter is the sole drill / focus path.
			if m.activePanel == DetailPanel {
				m.detail = m.detail.PrevTab()
				return m, nil
			}
		case "l":
			if m.activePanel == DetailPanel {
				m.detail = m.detail.NextTab()
				return m, nil
			}
		case "enter":
			if m.activePanel == TablePanel && m.drillDownPod == nil {
				return m, m.enterDrillDown()
			}
		case "esc":
			filterActive := (m.activePanel == SidebarPanel && m.sidebar.HasActiveFilter()) ||
				(m.activePanel == TablePanel && m.table.HasActiveFilter()) ||
				(m.activePanel == DetailPanel && m.detail.HasActiveFilter())
			if filterActive {
				// Let panel handle Esc to clear filter
			} else {
				// Panel 2 Esc with compare mode active: peel the
				// lock off first and KEEP going — same keypress
				// also pops one drill level if applicable. The
				// alternative (two-press: one for lock, one for
				// drill-back) made Esc feel inconsistent — every
				// other Esc in km8 does its work in one press.
				if m.activePanel == TablePanel && m.inCompareMode() {
					m.clearCompareLock()
				}
				if m.drillDownPod != nil || len(m.drillDownStack) > 0 {
					return m, m.exitDrillDown()
				}
			}
		case "N":
			// Open the picker immediately in its loading state so the
			// user gets zero-lag visual feedback, then fire the
			// LIST namespaces API in parallel. NamespaceListMsg swaps
			// in the real list when it arrives — no flicker because
			// the animator stays in open state across SetNamespaces.
			openCmd := m.namespacePicker.OpenLoading()
			return m, tea.Batch(openCmd, fetchNamespaces(m.k8sClient))
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
					return m, m.compareHotkeyDispatch(m.currentResource, m.items[idx])
				}
			}
			return m, fetchContexts(m.k8sClient)
		case "P":
			// Panel-1 only: toggle pinned status for the cursor's
			// resource kind. Acts on the sidebar's selected row even
			// without opening Space menu first — same UX as N/C which
			// surface globally. No-op when active panel isn't the
			// sidebar or when the cursor is on a category header.
			if m.activePanel != SidebarPanel {
				return m, nil
			}
			rt := m.sidebar.CursorResourceType()
			if rt == "" {
				return m, nil
			}
			return m, m.togglePinnedKind(rt)
		case "E":
			if !m.editing && m.activePanel == TablePanel && m.drillDownPod == nil && len(m.items) > 0 {
				idx := m.table.SelectedRow()
				if idx < 0 || idx >= len(m.items) {
					return m, nil
				}
				item := m.items[idx]
				// Rule A: any helm-managed resource — Release itself OR a
				// K8s object Helm rendered (label
				// app.kubernetes.io/managed-by=Helm or annotation
				// meta.helm.sh/release-name) — is read-only. kubectl edit
				// changes get overwritten on the next helm upgrade /
				// rollback anyway. Use helm upgrade for those.
				if m.currentResource == k8s.ResourceReleases || k8s.IsHelmManaged(item) {
					m.appLog.Info("Helm-managed (read-only) — use helm upgrade / rollback")
					return m, m.toast.Show("Helm-managed (read-only)")
				}
				// Kind-level gate (mirrors panel 2 menu): Events have no
				// editable surface, so E is a silent no-op + toast.
				if !resourceAllowsEdit(m.currentResource) {
					return m, m.toast.Show("Edit not supported on " + m.currentResource.KubectlName())
				}
				detail := fmt.Sprintf("kubectl edit %s/%s", m.currentResource.KubectlName(), item.Name)
				if item.Namespace != "" {
					detail += " -n " + item.Namespace
				}
				startCmd := func() tea.Msg {
					return startEditMsg{resource: m.currentResource, item: item, contextName: m.k8sClient.ContextName()}
				}
				return m, m.confirm.Show(ConfirmEdit, "Edit resource?", detail, startCmd)
			}
		case ".":
			// Toggle visibility of helm-managed items on any panel 2
			// resource list. Helm Releases themselves are excluded (the
			// category IS helm) — there `.` is a no-op. Re-start the
			// watcher to re-emit the cached items so the new filter
			// shows / hides them right away.
			if m.activePanel != TablePanel || m.currentResource == k8s.ResourceReleases {
				return m, nil
			}
			k8s.ToggleHelmHideManaged()
			m.watcher.Start(m.currentResource, m.k8sClient.GetNamespace())
			return m, waitForWatchUpdate(m.watcher, m.currentResource)
		case "S":
			// Panel-1: open Sort flow on the cursor's resource kind
			// (mirror of direct `P` for pin). Cursor on a category
			// header → silent no-op.
			// Panel-2: Shell into the selected pod's container.
			// Each panel owns its own meaning of S — same trade-off
			// as P/N/C globals.
			if m.activePanel == SidebarPanel {
				rt := m.sidebar.CursorResourceType()
				if rt == "" {
					return m, nil
				}
				return m, m.openSortColumnPicker(rt)
			}
			if m.activePanel == TablePanel {
				return m, m.execShell()
			}
		case "D":
			if m.activePanel == TablePanel && m.drillDownPod == nil && len(m.items) > 0 {
				idx := m.table.SelectedRow()
				if idx >= 0 && idx < len(m.items) {
					item := m.items[idx]
					// Rule A: Helm-managed resources are read-only — delete
					// here would be overwritten on next helm upgrade anyway.
					// Mirrors the `E` edit guard above.
					if m.currentResource == k8s.ResourceReleases || k8s.IsHelmManaged(item) {
						m.appLog.Info("Helm-managed (read-only) — use helm uninstall")
						return m, m.toast.Show("Helm-managed (read-only)")
					}
					// Kind-level gate (mirrors panel 2 menu): Events / Nodes /
					// Namespaces are blocked from delete here too — too far
					// from km8's scout-tool scope to gate via "asks for
					// confirmation" alone.
					if !resourceAllowsDelete(m.currentResource) {
						return m, m.toast.Show("Delete not supported on " + m.currentResource.KubectlName())
					}
					detail := fmt.Sprintf("kubectl delete %s %s -n %s", m.currentResource.KubectlName(), item.Name, item.Namespace)
					return m, m.confirm.Show(ConfirmDelete, "⚠ Delete resource? This cannot be undone.", detail,
						deleteResource(m.currentResource, item.Name, item.Namespace, m.k8sClient.ContextName()))
				}
			}
		case "z":
			// Toggle expand on the focused panel. If anything is expanded
			// already, restore. Otherwise expand whichever panel (Table or
			// Detail) currently has focus. Single-key toggle replaces the
			// old `=`/`-` pair.
			if m.detailExpanded || m.tableExpanded {
				m.detailExpanded = false
				m.tableExpanded = false
				return m, nil
			}
			if m.activePanel == DetailPanel {
				m.detailExpanded = true
				return m, nil
			}
			if m.activePanel == TablePanel {
				m.tableExpanded = true
				return m, nil
			}
		case "y":
			return m, copyToClipboardCmd(m.focusedPanelContent())
		case "Y":
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
					return m, func() tea.Msg { return RelativeDrillMsg{Ref: target} }
				}
			}
			yaml := m.detail.CurrentLevelYAML()
			if yaml == "" {
				return m, nil
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
			return m, m.yamlPopup.Open(yaml, resource, item, m.k8sClient.ContextName())
		case " ":
			// Sidebar (panel 1): rows are nav targets, not action targets.
			// Open a read-only cheatsheet popup explaining what the user
			// can do here (j/k move, Enter focus, / search, etc.). Mirrors
			// the panel 2/3 "Space surfaces what's possible" affordance —
			// but informational rather than committable.
			if m.activePanel == SidebarPanel && !m.sidebar.IsSearching() {
				m.hintPopup.SetSize(m.width, m.height)
				title, rows := sidebarHintContent()
				// Contextual Pin / Unpin toggle. Surfaces on any
				// resource row (category headers excluded — they have
				// no kind to act on). Pin vs Unpin is decided purely
				// by IsPinned(rt) so the SAME kind shown in both the
				// Pinned section AND its original category gives the
				// same action — pin status is per-kind, not per-row.
				// Hotkey "P" toggles either direction.
				var actions []hintAction
				if rt := m.sidebar.CursorResourceType(); rt != "" {
					label := string(rt)
					if def := m.k8sClient.Registry().Get(rt); def != nil {
						label = def.DisplayName
					}
					if m.sidebar.IsPinned(rt) {
						actions = append(actions, hintAction{
							label: "Unpin " + label, key: "P", action: "UnpinKind",
						})
					} else {
						actions = append(actions, hintAction{
							label: "Pin " + label, key: "P", action: "PinKind",
						})
					}
					// Sort entry — surfaces for every kind that has at
					// least one column (every registered kind in
					// practice). Commit routes through HintActionMsg
					// → SortKind handler, which opens the column
					// picker.
					if def := m.k8sClient.Registry().Get(rt); def != nil && len(def.Columns) > 0 {
						actions = append(actions, hintAction{
							label: "Sort " + label + "…", key: "S", action: "SortKind",
						})
					}
				}
				return m, m.hintPopup.OpenWithActions(title, actions, rows)
			}
			// Container drill view: panel 2 is showing the containers of
			// the pod we drilled into. Space opens a minimal menu carrying
			// only Shell — containers aren't standalone API objects so
			// YAML/Edit/Delete don't apply. execShell() (driven by the "S"
			// commit) already reads drillDownContainers[cursor], so the
			// menu just needs to surface the action.
			if m.activePanel == TablePanel && m.drillDownPod != nil && len(m.drillDownContainers) > 0 {
				idx := m.table.SelectedRow()
				if idx >= 0 && idx < len(m.drillDownContainers) {
					c := m.drillDownContainers[idx]
					m.panel2Menu.SetSize(m.width, m.height)
					return m, m.panel2Menu.OpenForContainer(m.drillDownPod.Name, m.drillDownPod.Namespace, c.Name)
				}
				return m, nil
			}
			// Panel 2 on a Helm Release row: Space opens the Helm doc
			// menu popup (manifest / notes / values). Branched before
			// the Relatives-tab logic because the activePanel guard
			// below would otherwise reject it.
			if m.activePanel == TablePanel && m.currentResource == k8s.ResourceReleases && !m.editing && m.drillDownPod == nil {
				idx := m.table.SelectedRow()
				if idx >= 0 && idx < len(m.items) {
					item := m.items[idx]
					m.helmDocMenu.SetSize(m.width, m.height)
					return m, m.helmDocMenu.Open(item.Name, item.Namespace)
				}
				return m, nil
			}
			// Panel 2 on a regular (non-Helm-Release) row: Space opens
			// the per-row context menu — YAML/Edit/Shell/Delete items
			// shaped by the resource kind and helm-managed status. The
			// menu surfaces what trigger letters do on this row instead
			// of relying on the user to remember Y/E/S/D in context.
			if m.activePanel == TablePanel && !m.editing && m.drillDownPod == nil && len(m.items) > 0 {
				idx := m.table.SelectedRow()
				if idx >= 0 && idx < len(m.items) {
					item := m.items[idx]
					m.panel2Menu.SetSize(m.width, m.height)
					return m, m.panel2Menu.Open(m.currentResource, item, len(m.drillDownStack) > 0, m.compareCtxForMenu(item))
				}
				return m, nil
			}
			// Panel 2 empty list: surface an explainer popup ("no items —
			// try N to switch ns, / clears filter, . toggles helm hide").
			// Without this Space was a silent no-op when the table happened
			// to be empty, breaking the "Space surfaces what's possible"
			// promise.
			if m.activePanel == TablePanel && !m.editing && m.drillDownPod == nil && len(m.items) == 0 {
				m.hintPopup.SetSize(m.width, m.height)
				title, rows := panel2EmptyHintContent()
				return m, m.hintPopup.Open(title, rows)
			}
			// Panel 3 History tab on a Helm Release: Space picks the
			// cursor row as the rollback target and pops the confirm
			// popup. Current (deployed) row returns nil via
			// SelectedHistoryRevision — silent no-op, no surprise prompt.
			if m.activePanel == DetailPanel && m.detail.ActiveTabName() == "History" {
				if rev := m.detail.SelectedHistoryRevision(); rev != nil {
					root := m.detail.RootRef()
					msg := fmt.Sprintf("Rollback %s to revision %d?", root.Name, rev.Revision)
					cmdStr := k8s.RollbackCommandString(root.Name, root.Namespace, rev.Revision)
					rollback := rollbackReleaseCmd(root.Name, root.Namespace, rev.Revision)
					return m, m.confirm.Show(ConfirmRollback, msg, cmdStr, rollback)
				}
				return m, nil
			}
			// Panel 3 Logs tab: read-only cheatsheet (j/k/u/d/G/y/z).
			// No per-row menu — Logs is a scrollable text buffer, not a
			// list of action targets.
			if m.activePanel == DetailPanel && m.detail.ActiveTabName() == "Logs" {
				m.hintPopup.SetSize(m.width, m.height)
				title, rows := logsHintContent()
				return m, m.hintPopup.Open(title, rows)
			}
			// Panel 3 Events tab: same idea — read-only cheatsheet for the
			// scrollable event list.
			if m.activePanel == DetailPanel && m.detail.ActiveTabName() == "Events" {
				m.hintPopup.SetSize(m.width, m.height)
				title, rows := eventsHintContent()
				return m, m.hintPopup.Open(title, rows)
			}
			// Panel 3 Conditions tab: scrollable table like Events, same nav
			// hint set — Space pops the read-only cheatsheet so user knows
			// j/k/u/d/gg/G/y/z apply here too.
			if m.activePanel == DetailPanel && m.detail.ActiveTabName() == "Conditions" {
				m.hintPopup.SetSize(m.width, m.height)
				title, rows := conditionsHintContent()
				return m, m.hintPopup.Open(title, rows)
			}
			// v1.5.x: Relatives tab Space splits by drill depth.
			//   depth>1 → open breadcrumb popup (chain navigator).
			//   depth=1 → no chain to walk, show the drill cheatsheet
			//             instead (Enter to drill, Y for YAML, etc.).
			if m.activePanel == DetailPanel && m.detail.ActiveTabName() == "Relatives" {
				if m.detail.Depth() <= 1 {
					m.hintPopup.SetSize(m.width, m.height)
					title, rows := relativesDrillHintContent()
					return m, m.hintPopup.Open(title, rows)
				}
				m.breadcrumbPopup.SetSize(m.width, m.height)
				return m, m.breadcrumbPopup.Open(m.detail.DrillChain())
			}
			return m, nil
		}

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
		// If the switch was launched from the breadcrumb popup, the
		// popup's chain is now stale (post-switch the drill chain
		// resets, so listed levels won't reach the same resources
		// anymore). Tear it down.
		if m.breadcrumbPopup.IsActive() {
			if c := m.breadcrumbPopup.Close(); c != nil {
				batch = append(batch, c)
			}
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
		m.watcher.Stop()
		m.detail.ClearDetail()
		m.detail.SetResourceType(msg.Type)
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
		m.syncTableSortIndicator()
		m.switchSeq++
		seq := m.switchSeq
		cmds = append(cmds, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
			return resourceSwitchTickMsg{seq: seq}
		}))
		return m, tea.Batch(cmds...)

	case resourceSwitchTickMsg:
		if msg.seq != m.switchSeq {
			return m, nil
		}
		m.watcher.Start(m.currentResource, m.k8sClient.GetNamespace())
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
		m.table.SetRows(rows)
		m.syncCompareLockToTable()
		m.honorPendingTableSelect(msg.Type, m.items)
		if len(m.items) > 0 {
			idx := m.table.SelectedRow()
			if idx >= 0 && idx < len(m.items) {
				item := m.items[idx]
				cmds = append(cmds, fetchResourceDetail(m.k8sClient, msg.Type, item))
				if c := m.detail.BeginRefetch(); c != nil {
					cmds = append(cmds, c)
				}
				switch {
				case msg.Type == k8s.ResourcePods && !m.logsActive:
					containers := k8s.ContainerNames(item.Raw)
					if len(containers) > 0 {
						m.detail.logLines = nil
						m.logStreamer.Start(item.Name, item.Namespace, containers)
						m.logsActive = true
						cmds = append(cmds, waitForLogLine(m.logStreamer))
					}
				case msg.Type == k8s.ResourceDeployments && !m.logsActive:
					m.detail.logLines = nil
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
		if m.drillDownPod != nil {
			// In drill-down: selected a container
			if msg.Index >= 0 && msg.Index < len(m.drillDownContainers) {
				c := m.drillDownContainers[msg.Index]
				detail := containerToDetail(c, *m.drillDownPod)
				m.detail.SetDetail(detail, nil)
				m.detail.logLines = nil
				m.logStreamer.Start(m.drillDownPod.Name, m.drillDownPod.Namespace, []string{c.Name})
				m.logsActive = true
				cmds = append(cmds, waitForLogLine(m.logStreamer))
			}
			return m, tea.Batch(cmds...)
		}
		if msg.Index >= 0 && msg.Index < len(m.items) && len(m.table.rows) > 0 {
			item := m.items[msg.Index]
			// Reset Relatives-tab drill chain immediately on row change so the
			// user doesn't briefly see the previous row's drill state while
			// the new detail fetch is in flight.
			m.detail.ResetDrillStack()
			cmds = append(cmds, fetchResourceDetail(m.k8sClient, m.currentResource, item))
			if c := m.detail.BeginRefetch(); c != nil {
				cmds = append(cmds, c)
			}
			switch m.currentResource {
			case k8s.ResourcePods:
				containers := k8s.ContainerNames(item.Raw)
				if len(containers) > 0 {
					m.detail.logLines = nil
					m.logStreamer.Start(item.Name, item.Namespace, containers)
					m.logsActive = true
					cmds = append(cmds, waitForLogLine(m.logStreamer))
				}
			case k8s.ResourceDeployments:
				m.logStreamer.Stop()
				m.logsActive = false
				m.detail.logLines = nil
				cmds = append(cmds, startAggregateLogs(m.k8sClient, m.currentResource, item))
			default:
				m.logStreamer.Stop()
				m.logsActive = false
			}
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
		batch := []tea.Cmd{fetchCmd}
		if c := m.detail.BeginRefetch(); c != nil {
			batch = append(batch, c)
		}
		return m, tea.Batch(batch...)

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

	case RelativeBreadcrumbMsg:
		if m.detail.Depth() <= 1 {
			return m, nil
		}
		m.breadcrumbPopup.SetSize(m.width, m.height)
		return m, m.breadcrumbPopup.Open(m.detail.DrillChain())

	case RelativeJumpMsg:
		m.detail.JumpToDrillLevel(msg.Level)
		return m, nil

	case resourceFetchedForDrillMsg:
		if msg.err != nil {
			m.appLog.Warn(fmt.Sprintf("drill %s/%s: %s", msg.ref.Type, msg.ref.Name, msg.err.Error()))
			return m, m.toast.ShowWarn("Drill failed — see App Log (!)")
		}
		if msg.yaml == "" {
			m.appLog.Warn(fmt.Sprintf("drill %s/%s: no YAML", msg.ref.Type, msg.ref.Name))
			return m, nil
		}
		m.yamlPopup.SetSize(m.width, m.height)
		return m, m.yamlPopup.Open(msg.yaml, msg.ref.Type, msg.item, m.k8sClient.ContextName())

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
			return m, nil
		}
		if len(msg.targets) == 0 {
			m.appLog.Info("aggregate logs: no pods running")
			return m, nil
		}
		m.logStreamer.StartMulti(msg.targets)
		m.logsActive = true
		return m, waitForLogLine(m.logStreamer)

	case LogLineMsg:
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

	case NamespaceChangedMsg:
		ns := msg.Namespace
		if ns == "" {
			ns = "All Namespaces"
		}
		m.appLog.Info("namespace switched to " + ns)
		m.k8sClient.SetNamespace(msg.Namespace)
		m.statusBar.SetNamespace(msg.Namespace)
		m.logStreamer.Stop()
		m.logsActive = false
		m.detail.ClearDetail()
		m.watcher.Start(m.currentResource, msg.Namespace)
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		return m, tea.Batch(cmds...)

	case ContextListMsg:
		return m, m.contextPicker.Open(msg.Contexts, msg.Current)

	case ContextChangedMsg:
		newClient, err := k8s.NewClient(msg.Context)
		if err != nil {
			m.appLog.Error("context switch failed: " + err.Error())
			return m, nil
		}
		m.appLog.Info("context switched to " + msg.Context)
		m.watcher.Stop()
		m.logStreamer.Stop()
		m.logsActive = false
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
		m.watcher.Start(m.currentResource, m.k8sClient.GetNamespace())
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		cmds = append(cmds, discoverCRDs(newClient))
		return m, tea.Batch(cmds...)

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
		if len(m.items) > 0 {
			cmds = append(cmds, fetchResourceDetail(m.k8sClient, msg.childType, m.items[0]))
			if c := m.detail.BeginRefetch(); c != nil {
				cmds = append(cmds, c)
			}
			if msg.childType == k8s.ResourcePods {
				containers := k8s.ContainerNames(m.items[0].Raw)
				if len(containers) > 0 {
					m.detail.logLines = nil
					m.logStreamer.Start(m.items[0].Name, m.items[0].Namespace, containers)
					m.logsActive = true
					cmds = append(cmds, waitForLogLine(m.logStreamer))
				}
			}
		}
		return m, tea.Batch(cmds...)

	case startShellExecMsg:
		// Only txPty being alive blocks a new exec — shellPty (KM8erm) is
		// independent and may be hidden in the background.
		if m.txPty.IsAlive() {
			m.appLog.Warn("close active edit/exec PTY before opening shell")
			return m, m.toast.Show("Close current edit/exec PTY first")
		}
		cmd := buildKubectlExecCmd(msg.podName, msg.namespace, msg.container, msg.contextName)
		title := fmt.Sprintf("Shell: pod/%s → %s", msg.podName, msg.container)
		return m, m.txPty.Start(cmd, title, m.width, m.height, PtyKindExec)

	case startEditMsg:
		if m.txPty.IsAlive() {
			m.appLog.Warn("close active edit/exec PTY before editing")
			return m, m.toast.Show("Close current edit/exec PTY first")
		}
		m.editing = true
		title := fmt.Sprintf("Edit: %s/%s", msg.resource.KubectlName(), msg.item.Name)
		if msg.item.Namespace != "" {
			title += " (" + msg.item.Namespace + ")"
		}
		cmd := buildKubectlEditCmd(msg.resource, msg.item, msg.contextName, m.cfgEditor)
		config.WriteAuditEntry("edit", msg.resource.KubectlName()+"/"+msg.item.Name, msg.item.Namespace, "started") //nolint
		m.appLog.Info("edit: " + msg.resource.KubectlName() + "/" + msg.item.Name)
		return m, m.txPty.Start(cmd, title, m.width, m.height, PtyKindEdit)

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
		m.appLog.Info("km8 started")
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
		m.toast.Update(msg)
		return m, nil

	case CRDsDiscoveredMsg:
		if msg.Err != nil {
			m.appLog.Warn("CRD discovery failed: " + msg.Err.Error())
		} else if msg.Count > 0 {
			m.appLog.Info(fmt.Sprintf("discovered %d CRDs", msg.Count))
			m.sidebar.RefreshCategories(m.k8sClient.Registry())
		}
		return m, nil

	case Panel2MenuActionMsg:
		// Panel 2 context menu committed an item (cursor + Enter or
		// direct hotkey). Each action mirrors the corresponding direct
		// keypress on the panel 2 row — kept inline so the trigger-key
		// correspondence stays visible. Rule A guards (helm-managed
		// read-only) match the E/D case statements above.
		resource := msg.Resource
		item := msg.Item
		switch msg.Action {
		case "Enter":
			// Same code path as pressing Enter on the row directly — the
			// menu entry is purely a discoverability surface.
			return m, m.enterDrillDown()
		case "Esc":
			// Mirror of the global Esc behavior — pop one drill level.
			// Surfaced in the container menu so users know they can back
			// out the same way other popups close.
			return m, m.exitDrillDown()
		case "Y":
			yaml := m.detail.CurrentLevelYAML()
			if yaml == "" {
				return m, nil
			}
			m.yamlPopup.SetSize(m.width, m.height)
			return m, m.yamlPopup.Open(yaml, resource, item, m.k8sClient.ContextName())
		case "E":
			if resource == k8s.ResourceReleases || k8s.IsHelmManaged(item) {
				m.appLog.Info("Helm-managed (read-only) — use helm upgrade / rollback")
				return m, m.toast.Show("Helm-managed (read-only)")
			}
			detail := fmt.Sprintf("kubectl edit %s/%s", resource.KubectlName(), item.Name)
			if item.Namespace != "" {
				detail += " -n " + item.Namespace
			}
			startCmd := func() tea.Msg {
				return startEditMsg{resource: resource, item: item, contextName: m.k8sClient.ContextName()}
			}
			return m, m.confirm.Show(ConfirmEdit, "Edit resource?", detail, startCmd)
		case "S":
			return m, m.execShell()
		case "D":
			if resource == k8s.ResourceReleases || k8s.IsHelmManaged(item) {
				m.appLog.Info("Helm-managed (read-only) — use helm uninstall")
				return m, m.toast.Show("Helm-managed (read-only)")
			}
			detail := fmt.Sprintf("kubectl delete %s %s -n %s", resource.KubectlName(), item.Name, item.Namespace)
			return m, m.confirm.Show(ConfirmDelete, "⚠ Delete resource? This cannot be undone.", detail,
				deleteResource(resource, item.Name, item.Namespace, m.k8sClient.ContextName()))
		case "C":
			// Contextual compare action — same letter, dispatches on
			// current state:
			//   - no anchor set → mark this row as the anchor
			//   - anchor set, cursor on different row of same kind →
			//     open the diff popup against the anchor
			// Menu gating hides "C" when the action would be a no-op
			// (cursor on the anchor itself, single-item list, kind
			// switched away from the anchor's), so we don't need to
			// double-guard here beyond the inCompareMode branch.
			return m, m.compareHotkeyDispatch(resource, item)
		}
		return m, nil

	case HintActionMsg:
		// Sidebar Space-menu actions. Pin / Unpin share
		// togglePinnedKind so the menu + direct `P` hotkey can't
		// drift; SortKind kicks off the listPicker chain (column →
		// direction → persist).
		switch msg.Action {
		case "PinKind", "UnpinKind":
			rt := m.sidebar.CursorResourceType()
			if rt == "" {
				return m, nil
			}
			return m, m.togglePinnedKind(rt)
		case "SortKind":
			rt := m.sidebar.CursorResourceType()
			if rt == "" {
				return m, nil
			}
			return m, m.openSortColumnPicker(rt)
		}
		return m, nil

	case ListPickerActionMsg:
		// Sort flow commits routed by PickerID. Column step picks a
		// column → opens the direction step (in-place swap on the
		// same listPicker). Direction step persists the choice and
		// closes the picker.
		switch msg.PickerID {
		case "sort:column":
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

	case ListPickerCancelMsg:
		// Esc at any sort step: drop in-flight kind/column so a
		// later sort flow starts fresh. The picker's own close
		// animation is already queued by the Cancel msg.
		switch msg.PickerID {
		case "sort:column", "sort:direction":
			m.sortFlowKind = ""
			m.sortFlowColumn = ""
		}
		return m, nil

	case HelmDocRequestMsg:
		// Menu picked a doc kind. Fire the helm CLI fetch asynchronously
		// so a slow `helm get manifest` on a big chart doesn't freeze the
		// UI; the result comes back as HelmDocReadyMsg.
		return m, fetchHelmDocCmd(msg.DocKind, msg.ReleaseName, msg.Namespace)

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
		return m, m.yamlPopup.Open(msg.Content, k8s.ResourceReleases, item, m.k8sClient.ContextName())

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

func (m AppModel) View() string {
	if !m.ready {
		return "loading..."
	}

	if m.splash.IsActive() {
		return m.splash.Render(m.width, m.height)
	}

	// KM8erm marker: render ONLY when the shell is alive but hidden — that's
	// the state the user can't otherwise see ("is there a shell waiting for
	// me?"). When the popup is visible, the popup's border already says
	// "KM8erm: hostname" so a status-bar duplicate just adds noise; when no
	// shell is running, nothing to mark.
	var ptyMarker *PtyMarker
	if m.shellPty != nil && m.shellPty.IsAlive() && m.shellPty.IsHidden() {
		ptyMarker = &PtyMarker{Visible: false, Label: " KM8erm"}
	}
	var compareMarker *CompareMarker
	if m.inCompareMode() {
		// Compare anchor only works within panel 2 — the user is
		// already on the kind that matches the anchor's kind. So the
		// kind prefix would just duplicate context already on screen;
		// the bare name is enough.
		compareMarker = &CompareMarker{
			Label: fmt.Sprintf("\U000f08aa %s", m.compareLock.name),
		}
	}
	statusBar := m.statusBar.ViewFull(m.appLog.UnreadErrorCount(), m.successNotice, ptyMarker, compareMarker)
	statusLine := m.statusLine.ViewWithNotice(m.appLog.UnreadErrorCount(), m.appLog.LastErrorMessage(), "")

	var mainView string

	if m.detailExpanded {
		panelH := m.height - 1 - m.statusLine.LineCount()
		panelW := m.width - 2*panelHMargin
		m.detail.SetSize(panelW-2, panelH-2)
		fullPanel := renderPanelWithScroll(m.detail.View(), "[3] "+m.detail.TabTitle()+m.detail.SpinnerSuffix(), panelW, panelH, true, m.theme, m.detail.ScrollInfo(), m.detail.BorderTopRightHint(), m.detail.BorderBottomLeftHint())
		hMargin := blankColumn(panelHMargin, panelH)
		middle := lipgloss.JoinHorizontal(lipgloss.Top, hMargin, fullPanel, hMargin)
		mainView = lipgloss.JoinVertical(lipgloss.Left, statusBar, middle, statusLine)
	} else if m.tableExpanded {
		_, _, upperH, detailH := m.panelSizes()
		panelW := m.width - 2*panelHMargin
		m.table.SetSize(panelW-2, upperH-2)
		m.detail.SetSize(panelW-2, detailH-2)
		tabTitle := "[2] " + m.breadcrumb()
		tablePanel := renderPanelWithScroll(m.table.View(), tabTitle, panelW, upperH, m.activePanel == TablePanel, m.theme, m.table.ScrollInfo(), "", m.tablePanelBottomLeft())
		detailPanel := renderPanelWithScroll(m.detail.View(), "[3] "+m.detail.TabTitle()+m.detail.SpinnerSuffix(), panelW, detailH, m.activePanel == DetailPanel, m.theme, m.detail.ScrollInfo(), m.detail.BorderTopRightHint(), m.detail.BorderBottomLeftHint())
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

		sidebarPanel := renderPanelWithScroll(m.sidebar.View(), "[1] km8", sw, fullH, m.activePanel == SidebarPanel, m.theme, m.sidebar.ScrollInfo(), "", "")
		tabTitle := "[2] " + m.breadcrumb()
		tablePanel := renderPanelWithScroll(m.table.View(), tabTitle, rw, upperH, m.activePanel == TablePanel, m.theme, m.table.ScrollInfo(), "", m.tablePanelBottomLeft())
		detailPanel := renderPanelWithScroll(m.detail.View(), "[3] "+m.detail.TabTitle()+m.detail.SpinnerSuffix(), rw, detailH, m.activePanel == DetailPanel, m.theme, m.detail.ScrollInfo(), m.detail.BorderTopRightHint(), m.detail.BorderBottomLeftHint())

		rightSide := joinTableAndDetail(tablePanel, detailPanel, rw)

		// 1-col gap between sidebar and right side, plus 1-col margins on
		// the outer edges so panel borders sit 1 cell inside the terminal.
		hMargin := blankColumn(panelHMargin, fullH)
		hSpace := blankColumn(panelHSpace, fullH)
		middle := lipgloss.JoinHorizontal(lipgloss.Top, hMargin, sidebarPanel, hSpace, rightSide, hMargin)
		mainView = lipgloss.JoinVertical(lipgloss.Left, statusBar, middle, statusLine)
	}

	if m.appLog.IsActive() {
		m.appLog.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.appLog.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.help.IsActive() {
		m.help.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.help.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.contextPicker.IsActive() {
		mainView = overlay.Composite(m.contextPicker.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.namespacePicker.IsActive() {
		mainView = overlay.Composite(m.namespacePicker.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	// helmDocMenu renders BEFORE yamlPopup so when the menu spawns a YAML
	// view the YAML overlays the menu (matching the input-routing order:
	// yamlPopup catches keys first while it's open, menu sits idle
	// underneath, then takes input back when YAML closes).
	if m.helmDocMenu.IsActive() {
		m.helmDocMenu.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.helmDocMenu.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.panel2Menu.IsActive() {
		m.panel2Menu.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.panel2Menu.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.hintPopup.IsActive() {
		m.hintPopup.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.hintPopup.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.listPicker.IsActive() {
		m.listPicker.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.listPicker.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.settingsPopup.IsActive() {
		m.settingsPopup.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.settingsPopup.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.yamlPopup.IsActive() {
		m.yamlPopup.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.yamlPopup.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.comparePopup.IsActive() {
		m.comparePopup.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.comparePopup.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.breadcrumbPopup.IsActive() {
		m.breadcrumbPopup.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.breadcrumbPopup.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	// Confirm renders LAST among modal popups so it sits on top of any
	// other open popup (breadcrumb especially — space on a breadcrumb
	// row triggers confirm while breadcrumb stays visible underneath).
	// Input routing checks confirm before breadcrumb (top of Update), so
	// the topmost visual popup is also the one receiving keys.
	if m.confirm.IsActive() {
		m.confirm.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.confirm.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.toast.IsActive() {
		mainView = overlay.Composite(m.toast.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	// Composite shellPty under txPty: KM8erm renders first so a visible
	// edit/exec popup overlays it. Hidden shellPty contributes nothing.
	if m.shellPty.IsActive() {
		mainView = overlay.Composite(m.shellPty.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}
	if m.txPty.IsActive() {
		mainView = overlay.Composite(m.txPty.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
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
			return waitForLogLine(m.logStreamer)
		}
		return nil
	}

	// Resource → child resource drill-down — only kinds with a
	// registered DrillDown config (HPA → target workload, etc.). For
	// everything else Enter is now a deliberate no-op (the panel-2 →
	// panel-3 focus shift was removed alongside the broader Enter-
	// as-focus fallback).
	if !m.currentResource.SupportsDrillDown() {
		return nil
	}

	return func() tea.Msg {
		childType, children, err := k8s.FetchChildResources(
			context.Background(), m.k8sClient.Clientset(), m.currentResource, item,
		)
		if err != nil || len(children) == 0 {
			return nil
		}
		return drillDownMsg{
			parentType: m.currentResource,
			parentName: item.Name,
			childType:  childType,
			children:   children,
		}
	}
}

func (m *AppModel) exitDrillDown() tea.Cmd {
	m.logStreamer.Stop()
	m.logsActive = false
	m.detail.logLines = nil

	// If at container level, go back to pod list
	if m.drillDownPod != nil {
		m.drillDownPod = nil
		m.drillDownContainers = nil
		// Restore current resource's table. Rows MUST be helm-augmented to
		// stay in lockstep with ColumnsForResource (which always reserves
		// an index-1 helm marker column for non-Releases kinds). Using raw
		// item.Row here shifts Status one column left, so stylizeCell —
		// which colors by column title — reads the wrong cell and the
		// Running green disappears until the next resource switch.
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
	if c := m.detail.BeginRefetch(); c != nil {
		cmds = append(cmds, c)
	}
	if m.currentResource == k8s.ResourcePods {
		containers := k8s.ContainerNames(item.Raw)
		if len(containers) > 0 {
			m.detail.logLines = nil
			m.logStreamer.Start(item.Name, item.Namespace, containers)
			m.logsActive = true
			cmds = append(cmds, waitForLogLine(m.logStreamer))
		}
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
	showCmd := m.confirm.Show(ConfirmShellExec, "Exec into container?", detail,
		shellExec(podName, namespace, container, m.k8sClient.ContextName()))
	m.appLog.Info("exec shell: " + detail)
	return showCmd
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
// internal terminal popup. Inherits env / cwd from km8 so the user's aliases,
// PATH, and current directory are exactly what they'd see in a regular
// terminal — like `ssh localhost` but embedded.
func buildShellTerminalCmd() *exec.Cmd {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	// -l → login shell, sources .zprofile/.bash_profile/etc. so the user
	// gets the same environment as a fresh terminal tab.
	return exec.Command(sh, "-l")
}

// terminalTitle returns the popup title for the internal terminal — the
// host's name prefixed with the KM8erm tag, mirroring how an ssh prompt
// identifies the connection. Returns os.Hostname() verbatim; mDNS-style
// suffixes like `.home` / `.local` / `.lan` are passed through, because
// the user said so (some routers append `.home` and the user wants to
// keep that visible).
func terminalTitle() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "KM8erm"
	}
	return "KM8erm: " + h
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
//   - `esc` to exit compare mode (only when a compare anchor is set,
//     since Esc otherwise has its standard back-out semantics elsewhere
//     — the hint is the discoverable affordance that "Exit compare
//     mode" used to live in the Space menu)
func (m *AppModel) tablePanelBottomLeft() string {
	var parts []string
	if m.currentResource != k8s.ResourceReleases {
		parts = append(parts, ".: toggle helm")
	}
	if m.inCompareMode() {
		parts = append(parts, "esc: exit compare")
	}
	return strings.Join(parts, "  ")
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

// augmentRowsWithHelm inserts the helm-marker cell right after Name on
// every row. Helm Releases get pass-through rows — their column set is
// already helm-specific (CHART / REV / STATUS / ...). Drill rows from
// container lists etc. don't pass through here, so they're unaffected.
func augmentRowsWithHelm(items []k8s.ResourceItem, rt k8s.ResourceType) [][]string {
	rows := make([][]string, len(items))
	for i, item := range items {
		if rt == k8s.ResourceReleases || len(item.Row) == 0 {
			rows[i] = item.Row
			continue
		}
		out := make([]string, 0, len(item.Row)+1)
		out = append(out, item.Row[0])
		out = append(out, k8s.MarkHelm(item))
		out = append(out, item.Row[1:]...)
		rows[i] = out
	}
	return rows
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
}

// ScrollInfo holds position info for the "X of Y" indicator in a panel border.
type ScrollInfo struct {
	Position int // 1-based current position
	Total    int
}

func renderPanel(content, title string, width, height int, focused bool, t *theme.Theme) string {
	return renderPanelWithScroll(content, title, width, height, focused, t, nil, "", "")
}

func renderPanelWithScroll(content, title string, width, height int, focused bool, t *theme.Theme, scroll *ScrollInfo, topRight, bottomLeft string) string {
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
	b.WriteString(bStyle.Render("╭─"))
	b.WriteString(tStyle.Render(title))
	b.WriteString(bStyle.Render(strings.Repeat("─", dashesAfter)))
	if topRight != "" {
		b.WriteString(bStyle.Render(" "))
		b.WriteString(tStyle.Render(topRight))
		b.WriteString(bStyle.Render("─"))
	}
	b.WriteString(bStyle.Render("╮"))
	b.WriteString("\n")

	leftBorder := bStyle.Render("│")
	rightBorder := bStyle.Render("│")
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

	// Bottom-left optional marker (used by panel 2 for `.helm` filter
	// hint). Style matches the title — same color + bold, with a
	// single dash either side as separator. Kept short by callers; if
	// it doesn't fit alongside the scroll indicator we drop it silently.
	leftHintRendered := ""
	leftHintVis := 0
	if bottomLeft != "" {
		leftHintVis = lipgloss.Width(bottomLeft) + 2 // dash + content + dash
		leftHintRendered = bStyle.Render("─") + tStyle.Render(bottomLeft) + bStyle.Render("─")
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
		b.WriteString(bStyle.Render("╰") + leftHintRendered + bStyle.Render(strings.Repeat("─", dashes)+indicator+"╯"))
	} else {
		dashes := innerW - leftHintVis
		if dashes < 0 {
			dashes = 0
			leftHintRendered = ""
			dashes = innerW
		}
		b.WriteString(bStyle.Render("╰") + leftHintRendered + bStyle.Render(strings.Repeat("─", dashes)+"╯"))
	}

	return b.String()
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
	return func() tea.Msg {
		line, ok := <-ls.Lines()
		if !ok {
			return nil
		}
		return LogLineMsg{Pod: line.Pod, Container: line.Container, Text: line.Text}
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
