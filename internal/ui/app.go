package ui

import (
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

const sidebarWidthPercent = 20
const minSidebarWidth = 22

type AppModel struct {
	sidebar    SidebarModel
	table      TableModel
	statusBar  StatusBarModel
	statusLine StatusLineModel

	activePanel Panel
	width       int
	height      int
	theme       *theme.Theme
	k8sClient   *k8s.Client
	ready       bool
}

func NewAppModel(t *theme.Theme, client *k8s.Client) AppModel {
	info := client.GetClusterInfo()

	sidebar := NewSidebarModel(t)
	sidebar.SetFocused(true)

	return AppModel{
		sidebar:     sidebar,
		table:       NewTableModel(t),
		statusBar:   NewStatusBarModel(t, info),
		statusLine:  NewStatusLineModel(t),
		activePanel: SidebarPanel,
		theme:       t,
		k8sClient:   client,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.sidebar.Init(),
		m.table.Init(),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.cyclePanel()
			return m, nil
		}

	case ResourceSelectedMsg:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case tea.MouseMsg:
		// route mouse events based on position
	}

	// Route key/mouse events to the focused panel
	switch m.activePanel {
	case SidebarPanel:
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		cmds = append(cmds, cmd)
	case TablePanel:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	if !m.ready {
		return "loading..."
	}

	statusBar := m.statusBar.View()
	statusLine := m.statusLine.View()

	sidebarView := m.sidebar.View()
	tableView := m.table.View()

	middle := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, tableView)

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, middle, statusLine)
}

func (m *AppModel) layout() {
	sw := m.sidebarWidth()
	tw := m.width - sw

	barHeight := 1
	lineHeight := 1
	middleHeight := m.height - barHeight - lineHeight

	m.statusBar.SetWidth(m.width)
	m.statusLine.SetWidth(m.width)
	m.sidebar.SetSize(sw, middleHeight)
	m.table.SetSize(tw, middleHeight)
}

func (m *AppModel) sidebarWidth() int {
	w := m.width * sidebarWidthPercent / 100
	if w < minSidebarWidth {
		w = minSidebarWidth
	}
	if w > m.width/2 {
		w = m.width / 2
	}
	return w
}

func (m *AppModel) cyclePanel() {
	switch m.activePanel {
	case SidebarPanel:
		m.activePanel = TablePanel
	case TablePanel:
		m.activePanel = SidebarPanel
	}
	m.sidebar.SetFocused(m.activePanel == SidebarPanel)
	m.table.SetFocused(m.activePanel == TablePanel)
	m.statusLine.SetActivePanel(m.activePanel)
}
