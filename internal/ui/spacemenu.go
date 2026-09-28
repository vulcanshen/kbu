package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

// MenuPopupModel is a menu popup (tdp F1): rows of a name and a one-line
// hint, j/k to move the cursor, Enter or a row's hotkey to run it. The
// app runs two instances:
//
//   - spaceMenu — the Space menu of the focused panel (tdp M2): the item
//     operation and panel operation regions, then the Global operation
//     row. Space closes it again (K5).
//   - globalMenu — the global operation popup (M4), opened from the Space
//     menu's last row and stacked over it. Space does nothing on it.
//
// A row can be dimmed (tdp M6): it is listed and the cursor can rest on
// it, but Enter and its hotkey do nothing. Running a row does not close
// the menu: the app decides (a row that opens a popup keeps the menu
// beneath it, F4; a row that only changes state closes it, T1).
type MenuPopupModel struct {
	animator PopupAnimator
	name     string // "space" / "global" — sent back in MenuActionMsg
	// spaceToggle: Space closes this menu (only the Space menu itself,
	// tdp K5).
	spaceToggle bool
	title       string
	items       []menuItem
	cursor      int
	screenW     int
	theme       *theme.Theme

	// Captured at Open. A watcher tick mid-popup must not be allowed to
	// shift the target of an item operation underneath the user.
	resource k8s.ResourceType
	item     k8s.ResourceItem

	layer       int
	borderColor lipgloss.Color
}

// menuItem is one row of a menu.
type menuItem struct {
	// label is the row's name. A single-character key is bracketed in
	// place (bracketHotkey); rows whose key is longer write the
	// bracket into the label themselves ("[Alt-S]ort …", "[Enter] …").
	label string
	// name is the label without that written-in bracket, for the key
	// reference; empty when the label has none.
	name string
	// key is the hotkey that runs the row. "" = none; "enter" / "esc"
	// are core keys named in the label (tdp D4) — in the menu they run
	// the row only from the cursor, since Enter runs the cursor row and
	// Esc closes the menu.
	key string
	// action identifies the row in MenuActionMsg; defaults to key.
	action string
	hint   string
	// disabled: the target exists but the action cannot run right now
	// (tdp M6) — dimmed, cursor can rest, Enter and hotkey do nothing.
	disabled bool
	// opens: the row opens a popup (now, or once a fetch lands), so the
	// menu stays beneath it (tdp F4). Other rows close the menu.
	opens bool

	// separator / header are non-selectable chrome: a rule between
	// regions and a region title.
	separator bool
	header    bool
}

func (it menuItem) actionName() string {
	if it.action != "" {
		return it.action
	}
	return it.key
}

func (it menuItem) selectable() bool { return !it.separator && !it.header }

// plainName is the row's name without a written-in key bracket.
func (it menuItem) plainName() string {
	if it.name != "" {
		return it.name
	}
	return it.label
}

// MenuActionMsg is emitted when the user runs a row (cursor + Enter, the
// row's hotkey, or a left click).
type MenuActionMsg struct {
	Menu     string // "space" / "global"
	Action   string
	Opens    bool
	Resource k8s.ResourceType
	Item     k8s.ResourceItem
}

// menuTitleGlyph marks a menu's title (tdp D3: glyph + text).
const menuTitleGlyph = ""

func newMenuPopupModel(t *theme.Theme, name, target string, spaceToggle bool) MenuPopupModel {
	bc := theme.PopupLayerColor(1)
	return MenuPopupModel{
		theme:       t,
		name:        name,
		spaceToggle: spaceToggle,
		animator:    NewPopupAnimator(target, bc),
		borderColor: bc,
		layer:       1,
	}
}

// NewSpaceMenuModel builds the Space menu instance.
func NewSpaceMenuModel(t *theme.Theme) MenuPopupModel {
	return newMenuPopupModel(t, "space", "spacemenu", true)
}

// NewGlobalMenuModel builds the global operation popup instance.
func NewGlobalMenuModel(t *theme.Theme) MenuPopupModel {
	return newMenuPopupModel(t, "global", "spacemenu_global", false)
}

// SetLayer stamps nesting depth + derives border / animator color.
func (m *MenuPopupModel) SetLayer(layer int) {
	m.layer = layer
	m.borderColor = theme.PopupLayerColor(layer)
	m.animator.Color = m.borderColor
}

// Open shows the menu with the given rows. rt / item are the target of
// the item operation rows, captured now so a watcher tick can't move it.
func (m *MenuPopupModel) Open(title string, items []menuItem, rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	m.title = title
	m.items = items
	m.resource = rt
	m.item = item
	m.cursor = m.firstSelectable()
	return m.animator.Open()
}

func (m *MenuPopupModel) Close() tea.Cmd     { return m.animator.Close() }
func (m *MenuPopupModel) SetSize(w, _ int)   { m.screenW = w }
func (m MenuPopupModel) IsActive() bool      { return m.animator.IsActive() }
func (m MenuPopupModel) IsInteractive() bool { return m.animator.IsInteractive() }

// Items returns the rows the menu was opened with.
func (m MenuPopupModel) Items() []menuItem { return m.items }

func (m *MenuPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m MenuPopupModel) Update(msg tea.Msg) (MenuPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	key := keyMsg.String()
	switch key {
	case "j", "down":
		m.cursor = m.nextSelectable(m.cursor)
		return m, nil
	case "k", "up":
		m.cursor = m.prevSelectable(m.cursor)
		return m, nil
	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.items) {
			return m, nil
		}
		return m, m.run(m.items[m.cursor])
	case "esc":
		return m, m.animator.Close()
	case " ":
		if m.spaceToggle {
			return m, m.animator.Close()
		}
		return m, nil
	}
	// A row's hotkey runs that row. Enter / Esc rows are cursor-only
	// (Enter runs the cursor row, Esc closes the menu).
	for _, it := range m.items {
		if it.selectable() && it.key == key && it.key != "enter" && it.key != "esc" {
			return m, m.run(it)
		}
	}
	return m, nil
}

// run emits the row's action. Chrome and dimmed rows do nothing.
func (m *MenuPopupModel) run(it menuItem) tea.Cmd {
	if !it.selectable() || it.disabled {
		return nil
	}
	msg := MenuActionMsg{
		Menu:     m.name,
		Action:   it.actionName(),
		Opens:    it.opens,
		Resource: m.resource,
		Item:     m.item,
	}
	return func() tea.Msg { return msg }
}

// HandleMouse: left-click on a row runs it (same as cursor + Enter);
// right-click on the popup closes it (mirror of Esc). Clicks outside the
// popup do nothing.
func (m MenuPopupModel) HandleMouse(msg tea.MouseMsg, screenW, screenH int) (MenuPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	popup := m.renderFullPopup()
	if !popupContains(popup, msg, screenW, screenH) {
		return m, nil
	}
	if msg.Button == tea.MouseButtonRight {
		return m, m.animator.Close()
	}
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	row := popupRowAt(popup, msg, screenW, screenH, 2, len(m.items))
	if row < 0 || !m.items[row].selectable() {
		return m, nil
	}
	m.cursor = row
	return m, m.run(m.items[row])
}

func (m MenuPopupModel) firstSelectable() int {
	for i, it := range m.items {
		if it.selectable() {
			return i
		}
	}
	return 0
}

// nextSelectable / prevSelectable move the cursor over chrome rows and
// wrap at the ends (tdp D4).
func (m MenuPopupModel) nextSelectable(from int) int {
	n := len(m.items)
	for step := 1; step <= n; step++ {
		if idx := (from + step) % n; m.items[idx].selectable() {
			return idx
		}
	}
	return from
}

func (m MenuPopupModel) prevSelectable(from int) int {
	n := len(m.items)
	for step := 1; step <= n; step++ {
		if idx := (from - step + n) % n; m.items[idx].selectable() {
			return idx
		}
	}
	return from
}

func (m MenuPopupModel) View() string { return "" }

func (m MenuPopupModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

// bracketHotkey marks a row's hotkey in its label (tdp M5, D4): in place
// when the key's letter is in the label ("[Y]AML", "Un[P]in Pods",
// "Cop[y]"), otherwise in front ("[/] Search"). The bracket always
// prints the key as it is pressed — case counts, so "[Y]" is Shift+Y and
// "[y]" is y. Keys longer than one character ("alt+S", "enter") are left
// to the label, which writes its own bracket ("[Alt-S]ort", "[Enter] …").
func bracketHotkey(label, key string) string {
	if label == "" || key == "" || len([]rune(key)) > 1 {
		return label
	}
	if idx := strings.Index(strings.ToUpper(label), strings.ToUpper(key)); idx >= 0 {
		return label[:idx] + "[" + key + "]" + label[idx+len(key):]
	}
	return "[" + key + "] " + label
}

func (m MenuPopupModel) renderFullPopup() string {
	bc := m.borderColor
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Background(bc).Bold(true)
	dimCursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c")).Background(lipgloss.Color("#45475a"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))

	title := menuTitleGlyph + " " + m.title
	hint := " j/k: move  Enter: run  Esc: close "

	// Width: widest of title / bottom hint / rows; clamp to 85% screen.
	maxInnerW := 60
	if m.screenW > 0 {
		maxInnerW = m.screenW * 85 / 100
		if maxInnerW < 40 {
			maxInnerW = 40
		}
	}
	innerW := lipgloss.Width(title) + 4
	if w := lipgloss.Width(hint) + 4; w > innerW {
		innerW = w
	}
	// The hints line up in one column, two cells past the widest name.
	labelCol := 0
	for _, it := range m.items {
		if it.selectable() {
			labelCol = max(labelCol, lipgloss.Width(bracketHotkey(it.label, it.key)))
		}
	}
	for _, it := range m.items {
		if w := 1 + 2 + labelCol + 2 + lipgloss.Width(it.hint) + 1; it.selectable() && w > innerW {
			innerW = w
		}
	}
	if innerW > maxInnerW {
		innerW = maxInnerW
	}

	const gutter = "  "
	var rows []string
	for i, it := range m.items {
		if it.header {
			rows = append(rows, " "+gutter+headerStyle.Render(it.label))
			continue
		}
		if it.separator {
			rows = append(rows, bStyle.Render(strings.Repeat("─", innerW)))
			continue
		}
		labelDisplay := bracketHotkey(it.label, it.key)
		labelW := lipgloss.Width(labelDisplay)
		gap := strings.Repeat(" ", labelCol-labelW+2)
		bodyPlain := " " + gutter + labelDisplay + gap + it.hint
		padW := innerW - 1 - lipgloss.Width(bodyPlain)
		if padW < 0 {
			padW = 0
		}
		pad := strings.Repeat(" ", padW)
		switch {
		case i == m.cursor && it.disabled:
			rows = append(rows, dimCursorStyle.Render(bodyPlain+pad))
		case i == m.cursor:
			rows = append(rows, cursorStyle.Render(bodyPlain+pad))
		case it.disabled:
			rows = append(rows, dimStyle.Render(bodyPlain)+pad)
		default:
			rows = append(rows, " "+gutter+labelDisplay+gap+hintStyle.Render(it.hint)+pad)
		}
	}

	dashesAfter := innerW - 1 - lipgloss.Width(title)
	if dashesAfter < 0 {
		dashesAfter = 0
	}
	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")
	left := bStyle.Render("│")
	right := bStyle.Render("│")
	padRow := left + strings.Repeat(" ", innerW) + right + "\n"
	b.WriteString(padRow)
	for _, line := range rows {
		lw := lipgloss.Width(line)
		if lw > innerW {
			line = ansiTruncate(line, innerW)
			lw = lipgloss.Width(line)
		}
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		b.WriteString(left + line + pad + right + "\n")
	}
	b.WriteString(padRow)
	bottomDashes := innerW - lipgloss.Width(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))
	return b.String()
}

// ── panel 2 item operations ──────────────────────────────────────────

// panel2CompareCtx carries the compare-mode flags into menu construction.
//
//   - locked:           AppModel.inCompareMode()
//   - canLock:          more than one row in panel 2 — marking the only
//     row as the anchor leaves nothing to compare it with, so the Mark
//     row is dimmed (tdp M6)
//   - cursorComparable: cursor row differs from the anchor AND is the
//     same kind. False on the anchor itself, or when the anchor is from
//     a different (drilled-away) kind.
//   - cursorOnAnchor:   cursor sits on the anchor row itself.
type panel2CompareCtx struct {
	locked           bool
	canLock          bool
	cursorComparable bool
	cursorOnAnchor   bool
}

// panel2ItemOps builds the item operation rows for a panel 2 resource
// row. Gating:
//
//  1. Kind-level: resourceAllowsEdit / resourceAllowsDelete leave Edit /
//     Delete out for kinds where they never apply (Events, Contexts,
//     Releases, Node delete) — the action does not exist there.
//  2. Rule A — a helm-managed object is read-only in kbu while Helm
//     manages it: Edit / Delete are listed but dimmed (tdp M6).
//  3. resourceHasContainer — Shell only on Pods.
//
// "Compare to anchor" goes first when it applies: the user set an anchor
// and opened the menu on a candidate row, so it is what they came to do.
func panel2ItemOps(rt k8s.ResourceType, item k8s.ResourceItem, helmManaged bool, compare panel2CompareCtx) []menuItem {
	var items []menuItem
	if compare.locked && compare.cursorComparable {
		items = append(items, menuItem{label: "Compare to anchor", key: "C", hint: "open the YAML diff popup", opens: true})
	}
	items = append(items, menuItem{label: "YAML", key: "Y", hint: "view resource manifest", opens: true})
	if resourceAllowsEdit(rt) {
		items = append(items, menuItem{label: "Edit", key: "E", hint: "kubectl edit", opens: true, disabled: helmManaged})
	}
	if resourceHasContainer(rt) {
		items = append(items, menuItem{label: "Shell", key: "S", hint: "kubectl exec -it", opens: true})
	}
	if resourceAllowsDelete(rt) {
		items = append(items, menuItem{label: "Delete", key: "D", hint: "kubectl delete", opens: true, disabled: helmManaged})
	}
	switch {
	case compare.locked && compare.cursorOnAnchor:
		items = append(items, menuItem{label: "Unmark Compare anchor", key: "C", hint: "cancel anchor, exit compare mode"})
	case !compare.locked:
		items = append(items, menuItem{label: "Mark as Compare anchor", key: "C", hint: "set this row as the diff baseline", disabled: !compare.canLock})
	}
	if rt.SupportsDrillDown() {
		hint := "into its children"
		if target := panel2DrillLabel(rt, item); target != "" {
			hint = "into its " + target
		}
		items = append(items, menuItem{label: "[Enter] Drill in", name: "Drill in", key: "enter", action: "drill", hint: hint})
	}
	items = append(items, menuItem{label: "Copy", key: "y", hint: "this row, tab-separated"})
	return items
}

// panel2DrillLabel returns the human-readable plural for what Enter drills
// into. Pod → "containers" (special-cased: containers aren't a K8s API
// resource, just a Pod sub-component). Other kinds resolve via registry's
// ChildTypeFor + KubectlName + "s".
func panel2DrillLabel(rt k8s.ResourceType, item k8s.ResourceItem) string {
	if rt == k8s.ResourcePods {
		return "containers"
	}
	def := k8s.DefaultRegistry.Get(rt)
	if def == nil || def.DrillDown == nil {
		return ""
	}
	childType := def.DrillDown.ChildTypeFor(item)
	if childType == "" {
		return ""
	}
	return childType.KubectlName() + "s"
}

// resourceAllowsEdit returns false for kinds where kbu never runs
// `kubectl edit`: Events (system-generated immutable records), Contexts
// (a read-only kubeconfig view, not a cluster object) and Helm Releases
// (changed through helm upgrade / rollback, not kubectl).
func resourceAllowsEdit(rt k8s.ResourceType) bool {
	switch rt {
	case k8s.ResourceEvents, k8s.ResourceContexts, k8s.ResourceReleases:
		return false
	}
	return true
}

// resourceAllowsDelete returns false for kinds where `kubectl delete` is
// blocked by kbu's scout-tool stance. Events (no point — they're
// system-generated immutable records), Nodes (admin infra action, not
// kbu's audience), Contexts (not a cluster object) and Helm Releases
// (removed with helm uninstall).
//
// Namespaces is intentionally NOT blocked here — the cascading destruction
// of every resource in the namespace makes it the single most dangerous
// delete kbu exposes, but blocking it entirely also makes kbu useless
// for "kubectl delete ns test-XYZ" cleanup which IS a normal dev-workflow
// action. The tradeoff resolves through the confirm popup: the delete-
// triggering paths (d hotkey + Space menu Delete) build a stronger
// warning message for the Namespace kind via deleteConfirmSurface —
// "will remove ALL resources in it" — so the user cannot fat-finger
// past a generic "delete resource?" prompt when the target is a ns.
func resourceAllowsDelete(rt k8s.ResourceType) bool {
	switch rt {
	case k8s.ResourceEvents, k8s.ResourceNodes, k8s.ResourceContexts, k8s.ResourceReleases:
		return false
	}
	return true
}

// resourceHasContainer returns true for kinds where `kubectl exec` is
// directly meaningful on the row. Currently only Pod — Deployment / STS /
// DS / Job / CronJob require a pod-selection step that execShell doesn't
// yet support. Users wanting a shell into a Deployment's pod drill in
// (Enter on the row) to the pod list, then S there.
func resourceHasContainer(rt k8s.ResourceType) bool {
	return rt == k8s.ResourcePods
}
