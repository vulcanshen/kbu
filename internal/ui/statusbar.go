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
	theme       *theme.Theme
}

func NewStatusBarModel(t *theme.Theme, info k8s.ClusterInfo) StatusBarModel {
	return StatusBarModel{
		clusterInfo: info,
		namespace:   "All Namespaces",
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

// PtyMarker is the right-side indicator that a persistent PTY (KM8erm) is
// alive. The status bar renders it green when the popup is currently visible
// ("attached") and amber when the popup is hidden in the background.
type PtyMarker struct {
	Visible bool
	Label   string // "attached" / "km8erm" / ... — caller controls
}

func (m StatusBarModel) View() string {
	return m.ViewWithErrors(0)
}

func (m StatusBarModel) ViewWithErrors(unreadErrors int) string {
	return m.ViewWithBadge(unreadErrors, "")
}

func (m StatusBarModel) ViewWithBadge(unreadErrors int, successNotice string) string {
	return m.ViewFull(unreadErrors, successNotice, nil)
}

// ViewFull renders the status bar with optional PTY marker. The PTY marker
// sits in the LEFT segment right after `ns:` so it doesn't fight the
// error / success badge on the right for space.
func (m StatusBarModel) ViewFull(unreadErrors int, successNotice string, pty *PtyMarker) string {
	ctx := m.theme.StatusBarContextStyle().Render(fmt.Sprintf("ctx: %s", m.clusterInfo.ContextName))
	cluster := m.theme.StatusBarClusterStyle().Render(fmt.Sprintf("cluster: %s", m.clusterInfo.ClusterName))
	ns := m.theme.StatusBarNamespaceStyle().Render(fmt.Sprintf("ns: %s", m.namespace))

	barStyle := m.theme.StatusBarStyle().Padding(0, 0)
	badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Bold(true)

	left := fmt.Sprintf(" %s  %s  %s", ctx, cluster, ns)
	if pty != nil {
		// Hidden KM8erm: Catppuccin peach (#fab387). Status.Pending defaults
		// to yellow — same hue as ns: in the status bar, so the marker
		// blended in. Peach reads as a distinct "warm reminder" without
		// stealing attention the way an error/red would.
		// Attached (popup visible): green via Status.Running, kept for the
		// rare cases ViewFull is called with Visible=true (current call
		// site only sets pty when hidden).
		color := "#fab387"
		if pty.Visible {
			color = m.theme.Status.Running
		}
		ptyChip := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(pty.Label)
		left = left + "  " + ptyChip
	}

	var badgePart string
	switch {
	case unreadErrors > 0:
		badgeText := fmt.Sprintf(" ! %d errors ", unreadErrors)
		badgePart = badgeStyle.Background(lipgloss.Color(m.theme.Status.Error)).Render(badgeText)
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
