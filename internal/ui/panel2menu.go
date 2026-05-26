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
func (m *Panel2MenuPopupModel) Open(rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	m.resource = rt
	m.item = item
	m.helmManaged = k8s.IsHelmManaged(item)
	m.items = buildPanel2MenuItems(rt, m.helmManaged)
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
		// popup with a stray press.
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

// buildPanel2MenuItems builds the per-row menu. Rule A — helm-managed
// resources are read-only — drops Edit and Delete. Shell only appears
// for resource kinds that actually have containers (Pod only — see
// resourceHasContainer).
func buildPanel2MenuItems(rt k8s.ResourceType, helmManaged bool) []panel2MenuItem {
	items := []panel2MenuItem{
		{label: "YAML", key: "Y", hint: "view resource manifest"},
	}
	if !helmManaged {
		items = append(items, panel2MenuItem{label: "Edit", key: "E", hint: "kubectl edit"})
	}
	if resourceHasContainer(rt) {
		items = append(items, panel2MenuItem{label: "Shell", key: "S", hint: "kubectl exec -it"})
	}
	if !helmManaged {
		items = append(items, panel2MenuItem{label: "Delete", key: "D", hint: "kubectl delete"})
	}
	return items
}

// bracketHotkey wraps the hotkey letter inside the label with square
// brackets (vim-help convention). "YAML" + "Y" → "[Y]AML". Falls back
// to the unmodified label when the hotkey isn't a substring (case-
// insensitive match), preserving label readability over hint correctness.
func bracketHotkey(label, key string) string {
	if label == "" || key == "" {
		return label
	}
	upperLabel := strings.ToUpper(label)
	upperKey := strings.ToUpper(key)
	idx := strings.Index(upperLabel, upperKey)
	if idx < 0 {
		return label
	}
	return label[:idx] + "[" + string(label[idx]) + "]" + label[idx+1:]
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

	title := " " + m.resource.KubectlName() + "/" + m.item.Name
	if m.helmManaged {
		title += " [helm]"
	}
	hint := " enter / esc "

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

	var rows []string
	for i, it := range m.items {
		isCursor := i == m.cursor
		marker := "  "
		if isCursor {
			marker = "▶ "
		}
		labelDisplay := bracketHotkey(it.label, it.key)
		labelW := lipgloss.Width(labelDisplay)
		gap := strings.Repeat(" ", max(2, 16-labelW))
		bodyPlain := " " + marker + labelDisplay + gap + it.hint
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
			" "+marker+labelDisplay+gap+hintStyle.Render(it.hint)+pad)
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
