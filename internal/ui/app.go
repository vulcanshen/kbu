package ui

import (
	"context"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

const sidebarWidthPercent = 20
const minSidebarWidth = 24
const detailHeightPercent = 40

type AppModel struct {
	sidebar         SidebarModel
	table           TableModel
	detail          DetailModel
	statusBar       StatusBarModel
	statusLine      StatusLineModel
	namespacePicker NamespacePickerModel
	contextPicker   ContextPickerModel
	help            HelpModel

	activePanel     Panel
	width           int
	height          int
	theme           *theme.Theme
	k8sClient       *k8s.Client
	watcher         *k8s.Watcher
	logStreamer      *k8s.LogStreamer
	currentResource k8s.ResourceType
	items           []k8s.ResourceItem
	ready           bool
	logsActive      bool // true when log streaming is running
}

func NewAppModel(t *theme.Theme, client *k8s.Client) AppModel {
	info := client.GetClusterInfo()

	sidebar := NewSidebarModel(t)
	sidebar.SetFocused(true)

	watcher := k8s.NewWatcher(client.Clientset())
	logStreamer := k8s.NewLogStreamer(client.Clientset())

	detail := NewDetailModel(t)
	detail.SetResourceType(k8s.ResourcePods)

	return AppModel{
		sidebar:         sidebar,
		table:           NewTableModel(t),
		detail:          detail,
		statusBar:       NewStatusBarModel(t, info),
		statusLine:      NewStatusLineModel(t),
		namespacePicker: NewNamespacePickerModel(t),
		contextPicker:   NewContextPickerModel(t),
		help:            NewHelpModel(t),
		activePanel:     SidebarPanel,
		theme:           t,
		k8sClient:       client,
		watcher:         watcher,
		logStreamer:      logStreamer,
		currentResource: k8s.ResourcePods,
	}
}

func (m AppModel) Init() tea.Cmd {
	m.watcher.Start(k8s.ResourcePods, m.k8sClient.GetNamespace())
	return tea.Batch(
		m.sidebar.Init(),
		m.table.Init(),
		waitForWatchUpdate(m.watcher),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Help overlay intercepts all input when active.
	if m.help.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			var cmd tea.Cmd
			m.help, cmd = m.help.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.namespacePicker.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			var cmd tea.Cmd
			m.namespacePicker, cmd = m.namespacePicker.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.contextPicker.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			var cmd tea.Cmd
			m.contextPicker, cmd = m.contextPicker.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case tea.KeyMsg:
		// Don't intercept global keys when table is in search mode
		// (except q/ctrl+c which should always work).
		if m.table.IsSearching() && m.activePanel == TablePanel {
			switch msg.String() {
			case "ctrl+c":
				m.watcher.Stop()
				m.logStreamer.Stop()
				return m, tea.Quit
			}
			// Let the table handle all other keys in search mode
			break
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.watcher.Stop()
			m.logStreamer.Stop()
			return m, tea.Quit
		case "?":
			m.help.SetSize(m.width, m.height)
			m.help.Toggle()
			return m, nil
		case "1":
			m.setPanel(SidebarPanel)
			return m, nil
		case "2":
			m.setPanel(TablePanel)
			return m, nil
		case "3":
			m.setPanel(DetailPanel)
			return m, nil
		case "tab":
			m.cyclePanel()
			return m, nil
		case "shift+tab":
			m.cyclePanelReverse()
			return m, nil
		case "n":
			return m, fetchNamespaces(m.k8sClient)
		case "c":
			return m, fetchContexts(m.k8sClient)
		case "e":
			if m.activePanel == TablePanel && len(m.items) > 0 {
				idx := m.table.SelectedRow()
				if idx >= 0 && idx < len(m.items) {
					item := m.items[idx]
					return m, editResource(m.currentResource, item.Name, item.Namespace)
				}
			}
		}

	case ResourceSelectedMsg:
		m.currentResource = msg.Type
		m.logStreamer.Stop()
		m.logsActive = false
		m.detail.ClearDetail()
		m.detail.SetResourceType(msg.Type)
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
		m.watcher.Start(msg.Type, m.k8sClient.GetNamespace())
		cmds = append(cmds, waitForWatchUpdate(m.watcher))
		return m, tea.Batch(cmds...)

	case ResourceDataMsg:
		m.items = msg.Items
		rows := make([][]string, len(msg.Items))
		for i, item := range msg.Items {
			rows[i] = item.Row
		}
		m.table.SetRows(rows)
		cmds = append(cmds, waitForWatchUpdate(m.watcher))
		return m, tea.Batch(cmds...)

	case ResourceErrorMsg:
		cmds = append(cmds, waitForWatchUpdate(m.watcher))
		return m, tea.Batch(cmds...)

	case RowSelectedMsg:
		if msg.Index >= 0 && msg.Index < len(m.items) {
			item := m.items[msg.Index]
			cmds = append(cmds, fetchResourceDetail(m.k8sClient, m.currentResource, item))

			// Start log streaming for Pods.
			if m.currentResource == k8s.ResourcePods {
				containers := k8s.ContainerNames(item.Raw)
				if len(containers) > 0 {
					m.detail.logLines = nil // clear previous logs
					m.logStreamer.Start(item.Name, item.Namespace, containers)
					m.logsActive = true
					cmds = append(cmds, waitForLogLine(m.logStreamer))
				}
			} else {
				m.logStreamer.Stop()
				m.logsActive = false
			}
		}
		return m, tea.Batch(cmds...)

	case LogLineMsg:
		m.detail.AppendLogLine(msg.Container, msg.Text)
		if m.logsActive {
			cmds = append(cmds, waitForLogLine(m.logStreamer))
		}
		return m, tea.Batch(cmds...)

	case ResourceDetailMsg:
		m.detail.SetDetail(msg.Detail, msg.Events)
		return m, nil

	case NamespaceListMsg:
		m.namespacePicker.Open(msg.Namespaces)
		return m, nil

	case NamespaceChangedMsg:
		m.k8sClient.SetNamespace(msg.Namespace)
		m.statusBar.SetNamespace(msg.Namespace)
		m.logStreamer.Stop()
		m.logsActive = false
		m.detail.ClearDetail()
		m.watcher.Start(m.currentResource, msg.Namespace)
		cmds = append(cmds, waitForWatchUpdate(m.watcher))
		return m, tea.Batch(cmds...)

	case ContextListMsg:
		m.contextPicker.Open(msg.Contexts, msg.Current)
		return m, nil

	case ContextChangedMsg:
		newClient, err := k8s.NewClient(msg.Context)
		if err != nil {
			// Stay on current context if the new one fails.
			return m, nil
		}
		m.watcher.Stop()
		m.logStreamer.Stop()
		m.logsActive = false
		m.k8sClient = newClient
		m.watcher = k8s.NewWatcher(newClient.Clientset())
		m.logStreamer = k8s.NewLogStreamer(newClient.Clientset())
		info := newClient.GetClusterInfo()
		m.statusBar.SetClusterInfo(info)
		m.statusBar.SetNamespace("")
		m.detail.ClearDetail()
		m.items = nil
		m.table.SetRows(nil)
		m.watcher.Start(m.currentResource, m.k8sClient.GetNamespace())
		cmds = append(cmds, waitForWatchUpdate(m.watcher))
		return m, tea.Batch(cmds...)

	case EditDoneMsg:
		// TUI resumes automatically. The watcher will pick up changes.
		return m, nil
	}

	switch m.activePanel {
	case SidebarPanel:
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		cmds = append(cmds, cmd)
	case TablePanel:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	case DetailPanel:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	if !m.ready {
		return "loading..."
	}

	if m.help.IsActive() {
		m.help.SetSize(m.width, m.height)
		return m.help.View()
	}

	if m.contextPicker.IsActive() {
		return m.contextPicker.View()
	}

	if m.namespacePicker.IsActive() {
		return m.namespacePicker.View()
	}

	sw, rw, tableH, detailH, middleH := m.panelSizes()
	// Inner sizes (subtract border: 2 for width, 2 for height)
	siW, siH := sw-2, middleH-2
	tiW, tiH := rw-2, tableH-2
	diW, diH := rw-2, detailH-2

	m.sidebar.SetSize(siW, siH)
	m.table.SetSize(tiW, tiH)
	m.detail.SetSize(diW, diH)

	statusBar := m.statusBar.View()
	statusLine := m.statusLine.View()

	sidebarPanel := renderPanel(m.sidebar.View(), "[1] Sidebar", sw, middleH, m.activePanel == SidebarPanel, m.theme)
	tablePanel := renderPanel(m.table.View(), "[2] "+m.currentResource.String(), rw, tableH, m.activePanel == TablePanel, m.theme)
	detailPanel := renderPanel(m.detail.View(), "[3] "+m.detail.TabTitle(), rw, detailH, m.activePanel == DetailPanel, m.theme)

	rightSide := lipgloss.JoinVertical(lipgloss.Left, tablePanel, detailPanel)
	middle := lipgloss.JoinHorizontal(lipgloss.Top, sidebarPanel, rightSide)

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, middle, statusLine)
}

func (m *AppModel) layout() {
	m.statusBar.SetWidth(m.width)
	m.statusLine.SetWidth(m.width)
	m.namespacePicker.SetSize(m.width, m.height)
	m.contextPicker.SetSize(m.width, m.height)
	m.help.SetSize(m.width, m.height)
}

func (m AppModel) panelSizes() (sw, rw, tableH, detailH, middleH int) {
	sw = m.width * sidebarWidthPercent / 100
	if sw < minSidebarWidth {
		sw = minSidebarWidth
	}
	if sw > m.width/2 {
		sw = m.width / 2
	}
	rw = m.width - sw

	middleH = m.height - 2 // 1 status bar + 1 status line
	if middleH < 6 {
		middleH = 6
	}

	detailH = middleH * detailHeightPercent / 100
	if detailH < 5 {
		detailH = 5
	}
	tableH = middleH - detailH
	if tableH < 4 {
		tableH = 4
		detailH = middleH - tableH
	}
	return
}

func (m *AppModel) setPanel(p Panel) {
	m.activePanel = p
	m.updateFocus()
}

func (m *AppModel) cyclePanel() {
	switch m.activePanel {
	case SidebarPanel:
		m.activePanel = TablePanel
	case TablePanel:
		m.activePanel = DetailPanel
	case DetailPanel:
		m.activePanel = SidebarPanel
	}
	m.updateFocus()
}

func (m *AppModel) cyclePanelReverse() {
	switch m.activePanel {
	case SidebarPanel:
		m.activePanel = DetailPanel
	case TablePanel:
		m.activePanel = SidebarPanel
	case DetailPanel:
		m.activePanel = TablePanel
	}
	m.updateFocus()
}

func (m *AppModel) updateFocus() {
	m.sidebar.SetFocused(m.activePanel == SidebarPanel)
	m.table.SetFocused(m.activePanel == TablePanel)
	m.detail.SetFocused(m.activePanel == DetailPanel)
	m.statusLine.SetActivePanel(m.activePanel)
}

// renderPanel wraps content in a bordered panel with a title, like lazygit.
func renderPanel(content, title string, width, height int, focused bool, t *theme.Theme) string {
	if width < 4 || height < 3 {
		return content
	}

	borderColor := t.Detail.BorderColor
	if focused {
		borderColor = t.Sidebar.CategoryFg
	}
	bc := lipgloss.Color(borderColor)
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)

	innerW := width - 2
	innerH := height - 2

	// Fit content into inner dimensions.
	fitted := lipgloss.NewStyle().Width(innerW).Height(innerH).MaxWidth(innerW).Render(content)
	lines := strings.Split(fitted, "\n")
	for len(lines) < innerH {
		lines = append(lines, strings.Repeat(" ", innerW))
	}
	lines = lines[:innerH]

	var b strings.Builder

	// Top border: ╭─[1] Sidebar────────╮
	titleVis := lipgloss.Width(title)
	dashesAfter := innerW - 1 - titleVis
	if dashesAfter < 0 {
		dashesAfter = 0
	}
	b.WriteString(bStyle.Render("╭─"))
	b.WriteString(tStyle.Render(title))
	b.WriteString(bStyle.Render(strings.Repeat("─", dashesAfter) + "╮"))
	b.WriteString("\n")

	// Content lines with side borders.
	leftBorder := bStyle.Render("│")
	rightBorder := bStyle.Render("│")
	for _, line := range lines {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		b.WriteString(leftBorder)
		b.WriteString(line)
		b.WriteString(pad)
		b.WriteString(rightBorder)
		b.WriteString("\n")
	}

	// Bottom border: ╰────────────────────╯
	b.WriteString(bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯"))

	return b.String()
}

func fetchNamespaces(client *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		items, err := k8s.FetchResources(context.Background(), client.Clientset(), k8s.ResourceNamespaces, "")
		if err != nil {
			return nil
		}
		names := make([]string, len(items))
		for i, item := range items {
			names[i] = item.Name
		}
		return NamespaceListMsg{Namespaces: names}
	}
}

func fetchContexts(client *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		contexts := client.ListContexts()
		current := client.ContextName()
		return ContextListMsg{Contexts: contexts, Current: current}
	}
}

func editResource(rt k8s.ResourceType, name, namespace string) tea.Cmd {
	args := []string{"edit", rt.KubectlName(), name}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	c := exec.Command("kubectl", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return EditDoneMsg{}
	})
}

func waitForWatchUpdate(w *k8s.Watcher) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-w.Updates():
			return ResourceDataMsg{Items: msg.Items}
		case errMsg := <-w.Errors():
			return ResourceErrorMsg{Err: errMsg.Err}
		}
	}
}

func fetchResourceDetail(client *k8s.Client, rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	return func() tea.Msg {
		detail := k8s.GetResourceDetail(rt, item)
		events, _ := k8s.FetchResourceEvents(context.Background(), client.Clientset(), item.Name, item.Namespace)
		return ResourceDetailMsg{Detail: detail, Events: events}
	}
}

func waitForLogLine(ls *k8s.LogStreamer) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ls.Lines()
		if !ok {
			return nil
		}
		return LogLineMsg{Container: line.Container, Text: line.Text}
	}
}
