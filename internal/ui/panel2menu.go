package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// Panel2MenuPopupModel is the per-row context menu opened by Space on a
// panel 2 resource row (everything that isn't a Helm Release — that has
// its own doc menu). Items are resource-aware:
//
//   - YAML(Y) — always available, view the resource manifest
//   - Edit(E) — if not helm-managed (Rule A read-only block)
//   - Shell(S) — if the resource type has containers
//   - Delete(D) — if not helm-managed (Rule A)
//
// Menu items display the hotkey in parens so the user can either cursor
// + Enter, or hit the letter directly. After commit (either path) the
// popup closes — menu is an ephemeral launcher, not a persistent context.
// Mirrors HelmDocMenuPopupModel's shape; chose a separate type because
// the items are dynamic per-row and the action msg is different.
type Panel2MenuPopupModel struct {
	animator PopupAnimator
	items    []panel2MenuItem
	cursor   int
	screenW  int
	theme    *theme.Theme

	// Captured at Open. A watcher tick mid-popup must not be allowed to
	// shift the action target underneath the user.
	resource    k8s.ResourceType
	item        k8s.ResourceItem
	helmManaged bool

	// Title override — set by OpenForContainer so the popup header reads
	// "container/<name>" instead of the default "pod/<name>". Empty string
	// falls back to the default rendering.
	titleOverride string
}

type panel2MenuItem struct {
	label string // "YAML" / "Edit" / "Shell" / "Delete" (rendered + "(K)")
	key   string // hotkey trigger letter "Y" / "E" / "S" / "D"
	hint  string // short description shown next to the label, helmdocmenu-style
}

// Panel2MenuActionMsg is emitted when the user commits a menu item (cursor
// + Enter or direct hotkey letter). AppModel maps the Action key to the
// same code path as the direct keypress on the underlying panel 2 row.
type Panel2MenuActionMsg struct {
	Action   string // "Y" / "E" / "S" / "D"
	Resource k8s.ResourceType
	Item     k8s.ResourceItem
}

func NewPanel2MenuPopupModel(t *theme.Theme) Panel2MenuPopupModel {
	return Panel2MenuPopupModel{
		theme:    t,
		animator: NewPopupAnimator("panel2menu", lipgloss.Color("#cba6f7")),
	}
}

// Open shows the menu for the given row. Items are computed at Open so
// the same cursor item produces a stable menu (no rebuild on rerender).
// canPop=true appends the "Esc ↖" back entry — used when the table is
// inside a drill chain (e.g. user pressed Enter on a Deployment row and
// is now viewing its Pods, so Esc pops back to the Deployment list).
// compare carries the compare-mode flags so the per-row menu can surface
// "Lock to compare" / "Compare to this resource" / "Exit compare mode"
// in the right combinations.
func (m *Panel2MenuPopupModel) Open(rt k8s.ResourceType, item k8s.ResourceItem, canPop bool, compare panel2CompareCtx) tea.Cmd {
	m.resource = rt
	m.item = item
	m.helmManaged = k8s.IsHelmManaged(item)
	m.items = buildPanel2MenuItems(rt, item, m.helmManaged, compare)
	if canPop {
		m.items = append(m.items, panel2MenuItem{
			label: "Esc " + drillUpIcon,
			key:   "Esc",
			hint:  "back to parent list",
		})
	}
	m.cursor = 0
	m.titleOverride = ""
	return m.animator.Open()
}

// OpenForContainer is the variant used when the user has drilled into a Pod
// and pressed Space on a container row. Surfaces only the Shell action —
// containers aren't standalone API objects, so YAML / Edit / Delete don't
// apply. AppModel.execShell() reads m.drillDownContainers[cursor] to resolve
// which container actually gets the exec, so the Item carried here is the
// parent pod for record-keeping only.
func (m *Panel2MenuPopupModel) OpenForContainer(podName, namespace, containerName string) tea.Cmd {
	m.resource = k8s.ResourcePods
	m.item = k8s.ResourceItem{Name: podName, Namespace: namespace}
	m.helmManaged = false
	m.titleOverride = " container/" + containerName
	m.items = []panel2MenuItem{
		{label: "Shell", key: "S", hint: "kubectl exec -it"},
		{label: "Esc " + drillUpIcon, key: "Esc", hint: "back to pod list"},
	}
	m.cursor = 0
	return m.animator.Open()
}

func (m *Panel2MenuPopupModel) Close() tea.Cmd     { return m.animator.Close() }
func (m *Panel2MenuPopupModel) SetSize(w, _ int)   { m.screenW = w }
func (m Panel2MenuPopupModel) IsActive() bool      { return m.animator.IsActive() }
func (m Panel2MenuPopupModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *Panel2MenuPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m Panel2MenuPopupModel) Update(msg tea.Msg) (Panel2MenuPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, nil
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "g":
		m.cursor = 0
		return m, nil
	case "G":
		m.cursor = len(m.items) - 1
		return m, nil
	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.items) {
			return m, nil
		}
		return m, m.commit(m.items[m.cursor].key)
	case "Y", "E", "S", "D":
		// Direct hotkey trigger — must match an item actually present in
		// the menu (Rule A removed Edit/Delete for helm-managed rows; a
		// hotkey shortcut shouldn't bypass that gate). Unknown hotkey
		// falls through to no-op so users don't accidentally close the
		// popup with a stray press. Compare-mode actions deliberately
		// have no entry here — they're menu-only, see buildPanel2MenuItems.
		key := keyMsg.String()
		for _, it := range m.items {
			if it.key == key {
				return m, m.commit(key)
			}
		}
		return m, nil
	case "esc", "q", " ":
		return m, m.animator.Close()
	}
	return m, nil
}

// commit closes the popup AND emits the action msg. Trigger 後 menu popup
// 一律關閉 — A 方案: cursor+Enter and direct hotkey paths reach the same
// final state, popup stack哲學不破 (menu is a launcher, not a context).
func (m *Panel2MenuPopupModel) commit(key string) tea.Cmd {
	closeCmd := m.animator.Close()
	resource := m.resource
	item := m.item
	actionCmd := func() tea.Msg {
		return Panel2MenuActionMsg{
			Action:   key,
			Resource: resource,
			Item:     item,
		}
	}
	return tea.Batch(closeCmd, actionCmd)
}

func (m Panel2MenuPopupModel) View() string { return "" }

func (m Panel2MenuPopupModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

// buildPanel2MenuItems builds the per-row menu. Three layers of gating:
//
//  1. Rule A — helm-managed resources are read-only — drops Edit and Delete.
//  2. resourceAllowsEdit / resourceAllowsDelete — opinionated dev-workflow
//     gates that hide kubectl actions which technically work but have no
//     practical purpose for a scout tool (e.g. Events can't be edited
//     meaningfully; Node deletion is admin-scope, not km8's audience).
//  3. resourceHasContainer — Shell only on kinds with containers (Pod).
//
// The Enter entry is appended at the END (with a separator above) when the
// kind supports drill-down — surfaces the drill action for discoverability,
// visually separated from the hotkey-driven Y/E/S/D group since Enter has
// no single-letter hotkey. Multi-char key "Enter" skips bracketHotkey
// (no false "[E]nter" rendering) and bypasses direct-hotkey dispatch
// (cursor+Enter only — direct Enter from elsewhere in the menu commits
// the cursor's own item, not the drill entry).
func buildPanel2MenuItems(rt k8s.ResourceType, item k8s.ResourceItem, helmManaged bool, compare panel2CompareCtx) []panel2MenuItem {
	canEdit := !helmManaged && resourceAllowsEdit(rt)
	canDelete := !helmManaged && resourceAllowsDelete(rt)

	items := []panel2MenuItem{
		{label: "YAML", key: "Y", hint: "view resource manifest"},
	}
	if canEdit {
		items = append(items, panel2MenuItem{label: "Edit", key: "E", hint: "kubectl edit"})
	}
	if resourceHasContainer(rt) {
		items = append(items, panel2MenuItem{label: "Shell", key: "S", hint: "kubectl exec -it"})
	}
	if canDelete {
		items = append(items, panel2MenuItem{label: "Delete", key: "D", hint: "kubectl delete"})
	}
	// Compare mode entries. Three states drive what appears:
	//   - not locked AND >1 selectable items → "Lock to compare"
	//   - locked AND cursor on a different item of the same kind →
	//       "Compare to this resource"
	//   - locked (in any cursor position)            → "Exit compare mode"
	// Single-item lists hide "Lock to compare" since locking with no
	// alternative target is a dead end.
	// Compare actions deliberately use multi-char keys so:
	//   - bracketHotkey skips them (no misleading "[L]ock to compare"
	//     render — these are menu-only, no direct hotkey)
	//   - the direct-hotkey case list below ignores them — pressing L /
	//     C / X anywhere does nothing on its own; the user MUST cursor
	//     onto the row and Enter to commit. Same pattern as the "Enter"
	//     drill-down entry.
	if compare.locked {
		if compare.cursorComparable {
			items = append(items, panel2MenuItem{label: "Compare to this resource", key: "CompareTo", hint: "diff against the locked item"})
		}
		items = append(items, panel2MenuItem{label: "Exit compare mode", key: "ExitCompare", hint: "release the locked item"})
	} else if compare.canLock {
		items = append(items, panel2MenuItem{label: "Lock to compare", key: "LockCompare", hint: "pick this row as the diff baseline"})
	}
	if rt.SupportsDrillDown() {
		target := panel2DrillLabel(rt, item)
		hint := "drill into " + target
		if target == "" {
			hint = "drill into children"
		}
		items = append(items, panel2MenuItem{
			label: "Enter " + drillDownIcon,
			key:   "Enter",
			hint:  hint,
		})
	}
	return items
}

// panel2CompareCtx carries the compare-mode flags into menu construction.
// Held as a struct (rather than 3 bool args) so the call site reads more
// purposefully — "the menu cares about compare context", not "three
// random bool flags".
//
//   - locked:           AppModel.inCompareMode()
//   - canLock:          len(panel-2 items) > 1 (single-item lists hide
//     the Lock entry — locking with no alternative
//     target is a dead end)
//   - cursorComparable: cursor row is a different UID from the locked
//     row AND same resource type. False when cursor
//     is on the locked row itself, or when locked
//     item is from a different (now-switched) kind.
type panel2CompareCtx struct {
	locked           bool
	canLock          bool
	cursorComparable bool
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

// resourceAllowsEdit returns false for kinds where `kubectl edit` is
// technically allowed but has no dev-workflow value. Currently only
// Events — they're system-generated immutable records.
func resourceAllowsEdit(rt k8s.ResourceType) bool {
	return rt != k8s.ResourceEvents
}

// resourceAllowsDelete returns false for kinds where `kubectl delete` is
// blocked by km8's scout-tool stance. Events (no point), Nodes (admin
// infra action), Namespaces (cascades to every workload in the ns — too
// destructive for a list-row hotkey).
func resourceAllowsDelete(rt k8s.ResourceType) bool {
	switch rt {
	case k8s.ResourceEvents, k8s.ResourceNodes, k8s.ResourceNamespaces:
		return false
	}
	return true
}

// bracketHotkey wraps the hotkey letter inside the label with square
// brackets (vim-help convention). "YAML" + "Y" → "[Y]AML",
// "Unpin Pods" + "P" → "Un[P]in Pods". The bracketed letter is rendered
// in the key's casing (always uppercase since hotkeys are case-
// sensitive uppercase) — without this, "Unpin"'s lowercase p inside
// the label would render "Un[p]in" while the actual hotkey is Shift+P,
// which reads as a key-mismatch hint to users. Falls back to the
// unmodified label when the hotkey isn't a substring (case-insensitive
// match) or when the key is multi-character (e.g. "Enter" — bracketing
// would be misleading since Enter isn't a single-letter shortcut),
// preserving label readability over hint correctness.
func bracketHotkey(label, key string) string {
	if label == "" || key == "" || len(key) > 1 {
		return label
	}
	upperLabel := strings.ToUpper(label)
	upperKey := strings.ToUpper(key)
	idx := strings.Index(upperLabel, upperKey)
	if idx < 0 {
		return label
	}
	return label[:idx] + "[" + upperKey + "]" + label[idx+1:]
}

// resourceHasContainer returns true for kinds where `kubectl exec` is
// directly meaningful on the row. Currently only Pod — Deployment / STS /
// DS / Job / CronJob require a pod-selection step that execShell doesn't
// yet support. Users wanting a shell into a Deployment's pod drill in
// (Enter on the row) to the pod list, then S there.
func resourceHasContainer(rt k8s.ResourceType) bool {
	return rt == k8s.ResourcePods
}

func (m Panel2MenuPopupModel) renderFullPopup() string {
	bc := lipgloss.Color("#cba6f7")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Background(bc).Bold(true)

	// Title icon: helm glyph when cursor row is helm-managed (so the
	// popup signals its provenance the same way the row's helm column
	// does), else the default popup glyph.
	icon := ""
	if m.helmManaged {
		icon = k8s.HelmIcon()
	}
	title := icon + " " + m.resource.KubectlName() + "/" + m.item.Name
	if m.titleOverride != "" {
		title = m.titleOverride
	}
	hint := " j/k: move  Space: close "

	// Width: pick widest of title / bottom hint / rows; clamp to 85% screen.
	// Row shape: " ▶ [K]rest   hint " — vim-help style hotkey bracketing,
	// no separate color since the [] already marks the key.
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
	// Calc must match render's gap formula (max(2, 16-labelW)) — using a
	// fixed gap=4 here used to under-count innerW and cause hint text to
	// punch through the right border.
	for _, it := range m.items {
		labelDisplay := bracketHotkey(it.label, it.key)
		labelW := lipgloss.Width(labelDisplay)
		gap := max(2, 16-labelW)
		w := 1 + 2 + labelW + gap + lipgloss.Width(it.hint) + 1
		if w > innerW {
			innerW = w
		}
	}
	if innerW > maxInnerW {
		innerW = maxInnerW
	}

	// Constant 2-space gutter on every row — cursor row is differentiated
	// by reverse-highlight alone (no leading arrow), so non-cursor rows
	// align cleanly without a phantom marker column.
	const gutter = "  "
	var rows []string
	for i, it := range m.items {
		isCursor := i == m.cursor
		labelDisplay := bracketHotkey(it.label, it.key)
		labelW := lipgloss.Width(labelDisplay)
		gap := strings.Repeat(" ", max(2, 16-labelW))
		bodyPlain := " " + gutter + labelDisplay + gap + it.hint
		padW := innerW - 1 - lipgloss.Width(bodyPlain)
		if padW < 0 {
			padW = 0
		}
		pad := strings.Repeat(" ", padW)
		if isCursor {
			rows = append(rows, cursorStyle.Render(bodyPlain+pad))
			continue
		}
		// Non-cursor: label keeps default color, hint dimmed.
		rows = append(rows,
			" "+gutter+labelDisplay+gap+hintStyle.Render(it.hint)+pad)
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
