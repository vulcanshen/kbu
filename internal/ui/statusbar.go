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

func (m StatusBarModel) View() string {
	return m.ViewWithErrors(0)
}

func (m StatusBarModel) ViewWithErrors(unreadErrors int) string {
	ctx := m.theme.StatusBarContextStyle().Render(fmt.Sprintf("ctx: %s", m.clusterInfo.ContextName))
	cluster := m.theme.StatusBarClusterStyle().Render(fmt.Sprintf("cluster: %s", m.clusterInfo.ClusterName))
	ns := m.theme.StatusBarNamespaceStyle().Render(fmt.Sprintf("ns: %s", m.namespace))

	left := fmt.Sprintf(" %s  %s  %s", ctx, cluster, ns)

	barStyle := m.theme.StatusBarStyle().Padding(0, 0)

	if unreadErrors > 0 {
		badgeText := fmt.Sprintf(" ! %d errors ", unreadErrors)
		badgeWidth := len(badgeText)
		leftPart := barStyle.Width(m.width - badgeWidth).Render(left)
		badgePart := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1e1e2e")).
			Background(lipgloss.Color(m.theme.Status.Error)).
			Bold(true).
			Render(badgeText)
		return leftPart + badgePart
	}

	return barStyle.Width(m.width).Render(left)
}

func (m StatusBarModel) Height() int {
	return lipgloss.Height(m.View())
}
