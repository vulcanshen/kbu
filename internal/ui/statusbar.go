package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

type StatusBarModel struct {
	clusterInfo k8s.ClusterInfo
	namespace   string
	width       int
	activePanel Panel
	theme       *theme.Theme
}

func NewStatusBarModel(t *theme.Theme, info k8s.ClusterInfo) StatusBarModel {
	return StatusBarModel{
		clusterInfo: info,
		namespace:   "All Namespaces",
		activePanel: SidebarPanel,
		theme:       t,
	}
}

func (m *StatusBarModel) SetClusterInfo(info k8s.ClusterInfo) {
	m.clusterInfo = info
}

func (m *StatusBarModel) SetNamespace(ns string) {
	if ns == "" {
		m.namespace = "All Namespaces"
	} else {
		m.namespace = ns
	}
}

func (m *StatusBarModel) SetWidth(width int) {
	m.width = width
}

// SetActivePanel lets the status bar adapt its bracket-hotkey markers to
// the currently focused panel. The `C` accelerator is panel-aware (panel 2
// hijacks it for Compare), so on panel 2 the `[C]ontext:` label drops its
// brackets — `C` won't open the context picker from there. Other labels
// stay bracketed because their hotkeys are global.
func (m *StatusBarModel) SetActivePanel(p Panel) {
	m.activePanel = p
}

// PtyMarker is the right-side indicator that a persistent PTY (Alterm) is
// alive. The status bar renders it green when the popup is currently visible
// ("attached") and amber when the popup is hidden in the background.
type PtyMarker struct {
	Visible bool
	Label   string // "attached" / "Alterm" / ... — caller controls
}

// CompareMarker is the indicator that compare mode is active. Label
// follows the icon+text status-bar convention; AppModel fills it with
// "󰕜 kind/name" so the user can see at a glance which row is locked.
type CompareMarker struct {
	Label string
}

func (m StatusBarModel) View() string {
	return m.ViewWithErrors(0)
}

func (m StatusBarModel) ViewWithErrors(unreadErrors int) string {
	return m.ViewWithBadge(unreadErrors, "")
}

func (m StatusBarModel) ViewWithBadge(unreadErrors int, successNotice string) string {
	return m.ViewFull(unreadErrors, 0, successNotice, nil, nil)
}

// ViewFull renders the status bar with optional PTY + compare markers.
// Markers sit in the LEFT segment after the namespace label so they don't
// fight the error / warn / success badge on the right for space.
//
// Badge precedence: error (red) > warn (peach) > success (green) > none.
// Splitting warn from error lets non-critical nudges (deprecation, transient
// hiccups) surface a Catppuccin Peach ` N warnings` badge without
// firing the red `! N errors` signal that should mean real failure. The
// `` glyph is the Nerd Font Font-Awesome warning triangle, picked
// over the unicode `⚠` (U+26A0) per design-guide §3.2 (glyphs limited
// to the U+F... Nerd Font private-use range).
func (m StatusBarModel) ViewFull(unreadErrors, unreadWarns int, successNotice string, pty *PtyMarker, compare *CompareMarker) string {
	// `[C]ontext: <ctx>  [N]amespace: <ns>` — multi-segment rendering
	// with semantic coloring instead of layout-shifting bracket markers:
	//   • brackets `[ ]`: blue (theme ContextFg)
	//   • hotkey letter `C` / `N` when ACTIVE: catppuccin green
	//   • filler text `ontext:` / `amespace:`: catppuccin overlay1 grey
	//   • hotkey letter when INACTIVE: same grey as filler
	//   • values <ctx>/<ns>: catppuccin lavender — same accent as the
	//     sidebar Pinned section, so the user-relevant identifiers
	//     (this kube context, this namespace, your pinned kinds)
	//     share a visual signature across the app
	// `C` is panel-aware (panel 2 hijacks it for Compare). On panel 2
	// the whole `[C]` (brackets + letter) collapses to grey — signals
	// "not a shortcut here" without any width change. `N` is global,
	// stays bright blue everywhere.
	//
	// Cluster slot dropped because context already binds (cluster, user)
	// — surfacing both was duplicating the same signal. Previous NF
	// glyphs (U+F0237 context, U+F51E namespace) migrated to the popup
	// titles so the icon-to-concept association stays.
	blueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusBar.ContextFg))
	greyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#b4befe"))

	cBracketStyle := blueStyle
	if m.activePanel == TablePanel {
		cBracketStyle = greyStyle
	}
	ctx := cBracketStyle.Render("[C]") +
		greyStyle.Render("ontext: ") + valueStyle.Render(m.clusterInfo.ContextName)
	ns := blueStyle.Render("[N]") +
		greyStyle.Render("amespace: ") + valueStyle.Render(m.namespace)

	barStyle := m.theme.StatusBarStyle().Padding(0, 0)
	badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Bold(true)

	left := fmt.Sprintf(" %s  %s", ctx, ns)
	if pty != nil {
		// Hidden Alterm: same orange (#F0AE49) as the Alterm popup border so
		// the user can visually link "this chip" to "that popup".
		// Lavender matches the popup border (Alterm is your persistent
		// shell — a user-state thing that outlives every other popup,
		// same conceptual bucket as Pinned / statusbar values / the
		// unfocused-selected chip).
		// Attached (popup visible): green via Status.Running, kept for the
		// rare cases ViewFull is called with Visible=true (current call
		// site only sets pty when hidden).
		color := "#b4befe"
		if pty.Visible {
			color = m.theme.Status.Running
		}
		ptyChip := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(pty.Label)
		left = left + "  " + ptyChip
	}
	if compare != nil && compare.Label != "" {
		// Compare-mode chip: Mocha lavender, matching the locked
		// row's bg in panel 2 and the rest of the "in-panel user-set
		// state" surfaces (pinned, ON toggle). Reads as one signal
		// with the panel-2 lock highlight.
		compareChip := lipgloss.NewStyle().Foreground(lipgloss.Color("#b4befe")).Bold(true).Render(compare.Label)
		left = left + "  " + compareChip
	}

	var badgePart string
	switch {
	case unreadErrors > 0:
		badgeText := fmt.Sprintf(" ! %d errors ", unreadErrors)
		badgePart = badgeStyle.Background(lipgloss.Color(m.theme.Status.Error)).Render(badgeText)
	case unreadWarns > 0:
		// Catppuccin Peach — same hue as toast warn border. The warn
		// badge is visually distinct from the error red so deprecation
		// nudges / non-critical events don't get conflated with real
		// failures.
		label := "warnings"
		if unreadWarns == 1 {
			label = "warning"
		}
		badgeText := fmt.Sprintf("  %d %s ", unreadWarns, label)
		badgePart = badgeStyle.Background(lipgloss.Color(toastWarnColor)).Render(badgeText)
	case successNotice != "":
		badgeText := fmt.Sprintf(" ✓ %s ", successNotice)
		badgePart = badgeStyle.Background(lipgloss.Color(m.theme.Status.Running)).Render(badgeText)
	}

	if badgePart == "" {
		return barStyle.Width(m.width).Render(left)
	}
	leftPart := barStyle.Width(m.width - lipgloss.Width(badgePart)).Render(left)
	return leftPart + badgePart
}

func (m StatusBarModel) Height() int {
	return lipgloss.Height(m.View())
}
