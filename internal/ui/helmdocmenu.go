package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// HelmDocMenuPopupModel is the 4-item picker that opens on panel 2 `Space`
// when the selected resource is a Helm Release. It lets the user choose
// which Helm document (manifest / creator notes / user values / merged
// values) to fetch via `helm get ...`. After Enter/Space the popup closes
// and emits HelmDocRequestMsg with the target release + chosen doc kind;
// AppModel handles the async fetch + opens YamlPopup with the result.
//
// Kept separate from BreadcrumbPopupModel because the data shape is
// fixed (4 known items) and the action target (a release rather than a
// chain) is different — sharing a generic list popup would mean adding a
// callback indirection for a single second use case.
type HelmDocMenuPopupModel struct {
	animator PopupAnimator
	items    []helmDocMenuItem
	cursor   int
	screenW  int
	theme    *theme.Theme

	// Release snapshot at Open time. Captured here so a watcher tick
	// arriving mid-popup doesn't shift the target underneath the user.
	releaseName string
	releaseNS   string
}

type helmDocMenuItem struct {
	label   string // "Manifest" / "Creator Notes" / ... (user-facing)
	docKind string // k8s.HelmDoc* identifier used for dispatch
	hint    string // short description rendered next to the label
}

var helmDocMenuItems = []helmDocMenuItem{
	{label: "Manifest", docKind: k8s.HelmDocManifest, hint: "rendered chart"},
	{label: "Creator Notes", docKind: k8s.HelmDocNotes, hint: "post-install notes"},
	{label: "User Values", docKind: k8s.HelmDocUserValues, hint: "user-supplied values"},
	{label: "Merged Values", docKind: k8s.HelmDocMergedValues, hint: "incl. chart defaults"},
	{label: "Hooks", docKind: k8s.HelmDocHooks, hint: "install/upgrade hook resources"},
}

func NewHelmDocMenuPopupModel(t *theme.Theme) HelmDocMenuPopupModel {
	return HelmDocMenuPopupModel{
		theme:    t,
		items:    helmDocMenuItems,
		animator: NewPopupAnimator("helmdocmenu", lipgloss.Color("#cba6f7")),
	}
}

// Open shows the menu for the given release. Cursor resets to the first
// item — short list, no benefit to remembering a previous position.
func (m *HelmDocMenuPopupModel) Open(releaseName, releaseNS string) tea.Cmd {
	m.releaseName = releaseName
	m.releaseNS = releaseNS
	m.cursor = 0
	return m.animator.Open()
}

func (m *HelmDocMenuPopupModel) Close() tea.Cmd  { return m.animator.Close() }
func (m *HelmDocMenuPopupModel) SetSize(w, _ int) { m.screenW = w }

func (m HelmDocMenuPopupModel) IsActive() bool      { return m.animator.IsActive() }
func (m HelmDocMenuPopupModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *HelmDocMenuPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

// HelmDocRequestMsg is emitted when the user picks a menu item. AppModel
// handles it by firing fetchHelmDocCmd and routing the eventual
// HelmDocReadyMsg into YamlPopup.
type HelmDocRequestMsg struct {
	DocKind     string // k8s.HelmDoc* constant
	ReleaseName string
	Namespace   string
}

// HelmDocReadyMsg carries the result of an async helm CLI fetch.
// AppModel opens the YAML popup with Content (or surfaces Err to app log).
type HelmDocReadyMsg struct {
	DocKind     string
	ReleaseName string
	Namespace   string
	Content     string
	Err         error
}

// RollbackResultMsg carries the outcome of an async `helm rollback`.
// AppModel surfaces success as a toast and failure as an app-log error.
type RollbackResultMsg struct {
	ReleaseName string
	Namespace   string
	Revision    int
	Output      string
	Err         error
}

// rollbackReleaseCmd runs `helm rollback` asynchronously. 30s timeout is
// chosen on the long side because rollbacks can wait on K8s reconciliation
// (replicasets winding down, jobs running) — well above the doc-fetch
// budget but short enough to surface a hung helm.
func rollbackReleaseCmd(releaseName, namespace string, revision int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := k8s.RollbackRelease(ctx, releaseName, namespace, revision)
		return RollbackResultMsg{
			ReleaseName: releaseName,
			Namespace:   namespace,
			Revision:    revision,
			Output:      out,
			Err:         err,
		}
	}
}

// fetchHelmDocCmd runs `helm get <kind>` for the chosen doc and folds the
// result (or error) into a HelmDocReadyMsg. 10s timeout is generous: even
// `helm get manifest` on a big chart should finish well within that, but
// the hard cap stops a hung helm CLI from wedging the UI forever.
func fetchHelmDocCmd(docKind, releaseName, namespace string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		content, err := k8s.FetchHelmDoc(ctx, docKind, releaseName, namespace)
		return HelmDocReadyMsg{
			DocKind:     docKind,
			ReleaseName: releaseName,
			Namespace:   namespace,
			Content:     content,
			Err:         err,
		}
	}
}

func (m HelmDocMenuPopupModel) Update(msg tea.Msg) (HelmDocMenuPopupModel, tea.Cmd) {
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
		it := m.items[m.cursor]
		req := HelmDocRequestMsg{
			DocKind:     it.docKind,
			ReleaseName: m.releaseName,
			Namespace:   m.releaseNS,
		}
		// Leave the menu open so the user can browse multiple docs in
		// one session — YamlPopup stacks on top via render order, and
		// input routing checks yamlPopup before helmDocMenu (see app.go
		// Update routing). When the user closes the YAML view, the
		// menu is still there to pick the next doc. Esc/q/Space dismisses
		// the menu itself (Space = mirror open).
		return m, func() tea.Msg { return req }
	case "esc", "q", " ":
		return m, m.animator.Close()
	}
	return m, nil
}

func (m HelmDocMenuPopupModel) View() string { return "" }

func (m HelmDocMenuPopupModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m HelmDocMenuPopupModel) renderFullPopup() string {
	bc := lipgloss.Color("#cba6f7")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Background(bc).Bold(true)

	// Title shows the targeted release so the user can't lose track when
	// the popup overlays a busy panel.  is the Nerd Font Helm glyph.
	title := " Helm: " + m.releaseName
	if m.releaseNS != "" {
		title += " (" + m.releaseNS + ")"
	}
	hint := " j/k: move  Enter: open  Esc/q/Space: close "

	// Width: pick the widest of title, hint, and rows; clamp to 85% screen.
	maxInnerW := 60
	if m.screenW > 0 {
		maxInnerW = m.screenW * 85 / 100
		if maxInnerW < 40 {
			maxInnerW = 40
		}
	}
	innerW := lipgloss.Width(title) + 4
	if w := len(hint) + 4; w > innerW {
		innerW = w
	}
	for _, it := range m.items {
		// " > Manifest    rendered chart "
		w := 1 + 2 + lipgloss.Width(it.label) + 4 + lipgloss.Width(it.hint) + 1
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
		labelW := lipgloss.Width(it.label)
		gap := strings.Repeat(" ", max(2, 16-labelW))
		bodyPlain := " " + marker + it.label + gap + it.hint
		padW := innerW - 1 - lipgloss.Width(bodyPlain)
		if padW < 0 {
			padW = 0
		}
		pad := strings.Repeat(" ", padW)
		if isCursor {
			rows = append(rows, cursorStyle.Render(bodyPlain+pad))
			continue
		}
		// Non-cursor rows: dim the hint so the label dominates.
		rows = append(rows, " "+marker+it.label+gap+hintStyle.Render(it.hint)+pad)
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
	b.WriteString(padRow) // top padding row
	for _, line := range rows {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		b.WriteString(left + line + pad + right + "\n")
	}
	b.WriteString(padRow) // bottom padding row
	bottomDashes := innerW - len(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))
	return b.String()
}

