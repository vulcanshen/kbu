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
		animator: NewPopupAnimator("panel2menu", lipgloss.Color("#a6e3a1")),
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
// for resource kinds that actually have containers.
func buildPanel2MenuItems(rt k8s.ResourceType, helmManaged bool) []panel2MenuItem {
	items := []panel2MenuItem{{label: "YAML", key: "Y"}}
	if !helmManaged {
		items = append(items, panel2MenuItem{label: "Edit", key: "E"})
	}
	if resourceHasContainer(rt) {
		items = append(items, panel2MenuItem{label: "Shell", key: "S"})
	}
	if !helmManaged {
		items = append(items, panel2MenuItem{label: "Delete", key: "D"})
	}
	return items
}

// resourceHasContainer returns true for kinds where `kubectl exec` is
// meaningful — i.e. they manage pods with containers. Service / ConfigMap
// / Secret etc. would just return an error if `S` were available.
func resourceHasContainer(rt k8s.ResourceType) bool {
	switch rt {
	case k8s.ResourcePods, k8s.ResourceDeployments,
		k8s.ResourceStatefulSets, k8s.ResourceDaemonSets,
		k8s.ResourceJobs, k8s.ResourceCronJobs:
		return true
	}
	return false
}

func (m Panel2MenuPopupModel) renderFullPopup() string {
	bc := lipgloss.Color("#a6e3a1")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387")).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Background(bc).Bold(true)

	title := " " + m.resource.KubectlName() + "/" + m.item.Name
	if m.helmManaged {
		title += " [helm]"
	}
	hint := " enter / esc "

	// Width: pick the widest of title / hint / rows, leave breathing room
	// on each side. No max cap — items are short fixed strings, the
	// natural max width fits easily inside any reasonable terminal.
	innerW := lipgloss.Width(title) + 2
	if w := lipgloss.Width(hint) + 2; w > innerW {
		innerW = w
	}
	for _, it := range m.items {
		w := 1 + 2 + lipgloss.Width(it.label) + 3 + 2 // " ▶ Label(K) "
		if w > innerW {
			innerW = w
		}
	}
	// Cap at terminal width minus margins so the popup never overflows.
	if m.screenW > 0 {
		cap := m.screenW * 85 / 100
		if innerW > cap {
			innerW = cap
		}
	}

	var rows []string
	for i, it := range m.items {
		isCursor := i == m.cursor
		marker := "  "
		if isCursor {
			marker = "▶ "
		}
		keyPart := "(" + it.key + ")"
		bodyW := 1 + 2 + lipgloss.Width(it.label) + lipgloss.Width(keyPart)
		padW := innerW - bodyW
		if padW < 0 {
			padW = 0
		}
		pad := strings.Repeat(" ", padW)
		if isCursor {
			rows = append(rows, cursorStyle.Render(" "+marker+it.label+keyPart+pad))
			continue
		}
		rows = append(rows, " "+marker+it.label+keyStyle.Render(keyPart)+pad)
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
