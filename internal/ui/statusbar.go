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

// ViewFull renders the status bar with optional PTY marker. The marker is
// placed immediately left of the error / success badge so both can coexist
// (e.g. KM8erm hidden while a watch error fired).
func (m StatusBarModel) ViewFull(unreadErrors int, successNotice string, pty *PtyMarker) string {
	ctx := m.theme.StatusBarContextStyle().Render(fmt.Sprintf("ctx: %s", m.clusterInfo.ContextName))
	cluster := m.theme.StatusBarClusterStyle().Render(fmt.Sprintf("cluster: %s", m.clusterInfo.ClusterName))
	ns := m.theme.StatusBarNamespaceStyle().Render(fmt.Sprintf("ns: %s", m.namespace))

	left := fmt.Sprintf(" %s  %s  %s", ctx, cluster, ns)
	barStyle := m.theme.StatusBarStyle().Padding(0, 0)

	badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Bold(true)

	var ptyPart string
	if pty != nil {
		color := m.theme.Status.Pending // amber when hidden
		if pty.Visible {
			color = m.theme.Status.Running // green when attached
		}
		ptyPart = badgeStyle.Background(lipgloss.Color(color)).Render(fmt.Sprintf("  %s ", pty.Label))
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

	rightW := lipgloss.Width(ptyPart) + lipgloss.Width(badgePart)
	if rightW == 0 {
		return barStyle.Width(m.width).Render(left)
	}
	leftPart := barStyle.Width(m.width - rightW).Render(left)
	return leftPart + ptyPart + badgePart
}

func (m StatusBarModel) Height() int {
	return lipgloss.Height(m.View())
}
