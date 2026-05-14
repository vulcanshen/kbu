package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

type drillDownMsg struct {
	parentType k8s.ResourceType
	parentName string
	childType  k8s.ResourceType
	children   []k8s.ResourceItem
}

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
	appLog          AppLogModel

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
	logsActive      bool
	detailExpanded  bool

	// Drill-down state
	drillDownStack      []drillDownEntry
	drillDownPod        *k8s.ResourceItem   // innermost: Pod → Container
	drillDownContainers []k8s.ContainerInfo
}

type drillDownEntry struct {
	parentType  k8s.ResourceType
	parentName  string
	parentItems []k8s.ResourceItem
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
		appLog:          NewAppLogModel(t),
		activePanel:     SidebarPanel,
		theme:           t,
		k8sClient:       client,
		watcher:         watcher,
		logStreamer:      logStreamer,
		currentResource: k8s.ResourcePods,
	}
}

func (m AppModel) Init() tea.Cmd {
	m.appLog.Info("km8 started")
	m.watcher.Start(k8s.ResourcePods, m.k8sClient.GetNamespace())
	info := m.k8sClient.GetClusterInfo()
	m.appLog.Info(fmt.Sprintf("connected to %s (%s)", info.ContextName, info.ServerURL))
	return tea.Batch(
		m.sidebar.Init(),
		m.table.Init(),
		waitForWatchUpdate(m.watcher),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.appLog.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			var cmd tea.Cmd
			m.appLog, cmd = m.appLog.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

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

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case tea.KeyMsg:
		// When any panel is in search mode, only ctrl+c passes through.
		searching := (m.activePanel == TablePanel && m.table.IsSearching()) ||
			(m.activePanel == SidebarPanel && m.sidebar.IsSearching()) ||
			(m.activePanel == DetailPanel && m.detail.IsSearching())
		if searching {
			switch msg.String() {
			case "ctrl+c":
				m.watcher.Stop()
				m.logStreamer.Stop()
				return m, tea.Quit
			}
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
		case "!":
			m.appLog.SetSize(m.width, m.height)
			m.appLog.Toggle()
			return m, nil
		case "1":
			m.detailExpanded = false
			m.setPanel(SidebarPanel)
			return m, nil
		case "2":
			m.detailExpanded = false
			m.setPanel(TablePanel)
			return m, nil
		case "3":
			m.detailExpanded = false
			m.setPanel(DetailPanel)
			return m, nil
		case "tab":
			m.cyclePanel()
			return m, nil
		case "shift+tab":
			m.cyclePanelReverse()
			return m, nil
		case "h":
			if m.activePanel == TablePanel || m.activePanel == DetailPanel {
				m.detail = m.detail.PrevTab()
				return m, nil
			}
		case "l":
			if m.activePanel == TablePanel || m.activePanel == DetailPanel {
				m.detail = m.detail.NextTab()
				return m, nil
			}
		case "enter":
			if m.activePanel == TablePanel && m.drillDownPod == nil {
				return m, m.enterDrillDown()
			}
		case "esc":
			filterActive := (m.activePanel == SidebarPanel && m.sidebar.HasActiveFilter()) ||
				(m.activePanel == TablePanel && m.table.HasActiveFilter()) ||
				(m.activePanel == DetailPanel && m.detail.HasActiveFilter())
			if filterActive {
				// Let panel handle Esc to clear filter
			} else if m.drillDownPod != nil || len(m.drillDownStack) > 0 {
				return m, m.exitDrillDown()
			}
		case "n":
			return m, fetchNamespaces(m.k8sClient)
		case "c":
			return m, fetchContexts(m.k8sClient)
		case "e":
			if m.activePanel == TablePanel && m.drillDownPod == nil && len(m.items) > 0 {
				idx := m.table.SelectedRow()
				if idx >= 0 && idx < len(m.items) {
					item := m.items[idx]
					return m, editResource(m.currentResource, item.Name, item.Namespace)
				}
			}
		case "+":
			if m.activePanel == DetailPanel {
				m.detailExpanded = true
				return m, nil
			}
		case "-":
			if m.detailExpanded {
				m.detailExpanded = false
				return m, nil
			}
		}

	case ResourceSelectedMsg:
		m.appLog.Info("switched to " + msg.Type.String())
		m.currentResource = msg.Type
		m.drillDownStack = nil
		m.drillDownPod = nil
		m.drillDownContainers = nil
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
		cmds = append(cmds, waitForWatchUpdate(m.watcher))
		if m.drillDownPod != nil {
			// In drill-down mode: update items but don't touch the table
			return m, tea.Batch(cmds...)
		}
		rows := make([][]string, len(msg.Items))
		for i, item := range msg.Items {
			rows[i] = item.Row
		}
		m.table.SetRows(rows)
		if len(msg.Items) > 0 {
			idx := m.table.SelectedRow()
			if idx >= 0 && idx < len(msg.Items) {
				item := msg.Items[idx]
				cmds = append(cmds, fetchResourceDetail(m.k8sClient, m.currentResource, item))
				if m.currentResource == k8s.ResourcePods && !m.logsActive {
					containers := k8s.ContainerNames(item.Raw)
					if len(containers) > 0 {
						m.detail.logLines = nil
						m.logStreamer.Start(item.Name, item.Namespace, containers)
						m.logsActive = true
						cmds = append(cmds, waitForLogLine(m.logStreamer))
					}
				}
			}
		}
		return m, tea.Batch(cmds...)

	case ResourceErrorMsg:
		m.appLog.Error(msg.Err.Error())
		cmds = append(cmds, waitForWatchUpdate(m.watcher))
		return m, tea.Batch(cmds...)

	case RowSelectedMsg:
		if m.drillDownPod != nil {
			// In drill-down: selected a container
			if msg.Index >= 0 && msg.Index < len(m.drillDownContainers) {
				c := m.drillDownContainers[msg.Index]
				detail := containerToDetail(c, m.drillDownPod.Name, m.drillDownPod.Namespace)
				m.detail.SetDetail(detail, nil)
				m.detail.logLines = nil
				m.logStreamer.Start(m.drillDownPod.Name, m.drillDownPod.Namespace, []string{c.Name})
				m.logsActive = true
				cmds = append(cmds, waitForLogLine(m.logStreamer))
			}
			return m, tea.Batch(cmds...)
		}
		if msg.Index >= 0 && msg.Index < len(m.items) {
			item := m.items[msg.Index]
			cmds = append(cmds, fetchResourceDetail(m.k8sClient, m.currentResource, item))
			if m.currentResource == k8s.ResourcePods {
				containers := k8s.ContainerNames(item.Raw)
				if len(containers) > 0 {
					m.detail.logLines = nil
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
		ns := msg.Namespace
		if ns == "" {
			ns = "All Namespaces"
		}
		m.appLog.Info("namespace switched to " + ns)
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
			m.appLog.Error("context switch failed: " + err.Error())
			return m, nil
		}
		m.appLog.Info("context switched to " + msg.Context)
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

	case drillDownMsg:
		if msg.children == nil {
			return m, nil
		}
		m.drillDownStack = append(m.drillDownStack, drillDownEntry{
			parentType:  msg.parentType,
			parentName:  msg.parentName,
			parentItems: m.items,
		})
		m.currentResource = msg.childType
		m.items = msg.children
		m.detail.SetResourceType(msg.childType)
		m.table.SetColumns(ColumnsForResource(msg.childType))
		rows := make([][]string, len(msg.children))
		for i, item := range msg.children {
			rows[i] = item.Row
		}
		m.table.SetRows(rows)
		m.statusLine.SetDrillDown(true)
		if len(msg.children) > 0 {
			cmds = append(cmds, fetchResourceDetail(m.k8sClient, msg.childType, msg.children[0]))
			if msg.childType == k8s.ResourcePods {
				containers := k8s.ContainerNames(msg.children[0].Raw)
				if len(containers) > 0 {
					m.detail.logLines = nil
					m.logStreamer.Start(msg.children[0].Name, msg.children[0].Namespace, containers)
					m.logsActive = true
					cmds = append(cmds, waitForLogLine(m.logStreamer))
				}
			}
		}
		return m, tea.Batch(cmds...)

	case EditDoneMsg:
		m.appLog.Info("kubectl edit completed")
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

	if m.appLog.IsActive() {
		m.appLog.SetSize(m.width, m.height)
		return m.appLog.View()
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

	statusBar := m.statusBar.View()
	statusLine := m.statusLine.View()

	if m.detailExpanded {
		panelH := m.height - 2
		m.detail.SetSize(m.width-2, panelH-2)
		fullPanel := renderPanel(m.detail.View(), "[3] "+m.detail.ActiveTabName(), m.width, panelH, true, m.theme)
		return lipgloss.JoinVertical(lipgloss.Left, statusBar, fullPanel, statusLine)
	}

	sw, rw, upperH, detailH := m.panelSizes()
	fullH := upperH + detailH

	m.sidebar.SetSize(sw-2, fullH-2)
	m.table.SetSize(rw-2, upperH-2)
	m.detail.SetSize(rw-2, detailH-2)

	sidebarPanel := renderPanel(m.sidebar.View(), "[1] km8", sw, fullH, m.activePanel == SidebarPanel, m.theme)

	tabTitle := "[2] " + m.breadcrumb() + "─" + m.detail.TabTitle()
	tablePanel := renderPanel(m.table.View(), tabTitle, rw, upperH, m.activePanel == TablePanel, m.theme)

	detailPanel := renderPanel(m.detail.View(), "[3] "+m.detail.ActiveTabName(), rw, detailH, m.activePanel == DetailPanel, m.theme)

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

func (m AppModel) panelSizes() (sw, rw, upperH, detailH int) {
	sw = m.width * sidebarWidthPercent / 100
	if sw < minSidebarWidth {
		sw = minSidebarWidth
	}
	if sw > m.width/2 {
		sw = m.width / 2
	}
	rw = m.width - sw

	totalH := m.height - 2 // status bar + status line
	if totalH < 6 {
		totalH = 6
	}

	detailH = totalH * detailHeightPercent / 100
	if detailH < 5 {
		detailH = 5
	}
	upperH = totalH - detailH
	if upperH < 4 {
		upperH = 4
		detailH = totalH - upperH
	}
	return
}

func (m *AppModel) enterDrillDown() tea.Cmd {
	idx := m.table.SelectedRow()
	if idx < 0 || idx >= len(m.items) {
		return nil
	}
	item := m.items[idx]

	// Pod → Container drill-down (special case)
	if m.currentResource == k8s.ResourcePods {
		m.drillDownPod = &item
		detail := k8s.GetResourceDetail(k8s.ResourcePods, item)
		m.drillDownContainers = detail.Containers
		m.table.SetColumns(containerColumns())
		m.table.SetRows(containerRows(m.drillDownContainers))
		m.statusLine.SetDrillDown(true)
		if len(m.drillDownContainers) > 0 {
			c := m.drillDownContainers[0]
			d := containerToDetail(c, item.Name, item.Namespace)
			m.detail.SetDetail(d, nil)
			m.detail.logLines = nil
			m.logStreamer.Start(item.Name, item.Namespace, []string{c.Name})
			m.logsActive = true
			return waitForLogLine(m.logStreamer)
		}
		return nil
	}

	// Resource → child resource drill-down
	if !m.currentResource.SupportsDrillDown() {
		return nil
	}

	return func() tea.Msg {
		childType, children, err := k8s.FetchChildResources(
			context.Background(), m.k8sClient.Clientset(), m.currentResource, item,
		)
		if err != nil || len(children) == 0 {
			return nil
		}
		return drillDownMsg{
			parentType: m.currentResource,
			parentName: item.Name,
			childType:  childType,
			children:   children,
		}
	}
}

func (m *AppModel) exitDrillDown() tea.Cmd {
	m.logStreamer.Stop()
	m.logsActive = false
	m.detail.logLines = nil

	// If at container level, go back to pod list
	if m.drillDownPod != nil {
		m.drillDownPod = nil
		m.drillDownContainers = nil
		// Restore current resource's table
		m.table.SetColumns(ColumnsForResource(m.currentResource))
		rows := make([][]string, len(m.items))
		for i, item := range m.items {
			rows[i] = item.Row
		}
		m.table.SetRows(rows)
		m.statusLine.SetDrillDown(len(m.drillDownStack) > 0)
		return m.refreshDetailForCurrent()
	}

	// Pop from resource drill-down stack
	if len(m.drillDownStack) > 0 {
		entry := m.drillDownStack[len(m.drillDownStack)-1]
		m.drillDownStack = m.drillDownStack[:len(m.drillDownStack)-1]
		m.currentResource = entry.parentType
		m.items = entry.parentItems
		m.detail.SetResourceType(m.currentResource)
		m.table.SetColumns(ColumnsForResource(m.currentResource))
		rows := make([][]string, len(m.items))
		for i, item := range m.items {
			rows[i] = item.Row
		}
		m.table.SetRows(rows)
		m.statusLine.SetDrillDown(len(m.drillDownStack) > 0)
		return m.refreshDetailForCurrent()
	}

	return nil
}

func (m *AppModel) refreshDetailForCurrent() tea.Cmd {
	if len(m.items) == 0 {
		return nil
	}
	idx := m.table.SelectedRow()
	if idx < 0 || idx >= len(m.items) {
		return nil
	}
	item := m.items[idx]
	var cmds []tea.Cmd
	cmds = append(cmds, fetchResourceDetail(m.k8sClient, m.currentResource, item))
	if m.currentResource == k8s.ResourcePods {
		containers := k8s.ContainerNames(item.Raw)
		if len(containers) > 0 {
			m.detail.logLines = nil
			m.logStreamer.Start(item.Name, item.Namespace, containers)
			m.logsActive = true
			cmds = append(cmds, waitForLogLine(m.logStreamer))
		}
	}
	return tea.Batch(cmds...)
}

func containerColumns() []Column {
	return []Column{
		{Title: "Name", MinWidth: 15},
		{Title: "Image", MinWidth: 30},
		{Title: "State", MinWidth: 12},
		{Title: "Ready", MinWidth: 7},
		{Title: "Restarts", MinWidth: 10},
	}
}

func containerRows(containers []k8s.ContainerInfo) [][]string {
	rows := make([][]string, len(containers))
	for i, c := range containers {
		ready := "false"
		if c.Ready {
			ready = "true"
		}
		prefix := ""
		if c.Init {
			prefix = "(init) "
		}
		rows[i] = []string{
			prefix + c.Name,
			c.Image,
			c.State,
			ready,
			fmt.Sprintf("%d", c.Restarts),
		}
	}
	return rows
}

func containerToDetail(c k8s.ContainerInfo, podName, namespace string) k8s.ResourceDetail {
	fields := []k8s.DetailField{
		{Label: "Pod", Value: podName},
		{Label: "Image", Value: c.Image},
		{Label: "State", Value: c.State},
	}
	ready := "false"
	if c.Ready {
		ready = "true"
	}
	fields = append(fields, k8s.DetailField{Label: "Ready", Value: ready})
	if c.Restarts > 0 {
		fields = append(fields, k8s.DetailField{Label: "Restarts", Value: fmt.Sprintf("%d", c.Restarts)})
	}
	if c.Ports != "" {
		fields = append(fields, k8s.DetailField{Label: "Ports", Value: c.Ports})
	}
	name := c.Name
	if c.Init {
		name = "(init) " + name
	}
	return k8s.ResourceDetail{
		Name:      name,
		Namespace: namespace,
		Kind:      "Container",
		Fields:    fields,
	}
}

func (m AppModel) breadcrumb() string {
	if m.drillDownPod != nil {
		return truncateName(m.drillDownPod.Name, 20) + " > Containers"
	}
	if len(m.drillDownStack) > 0 {
		last := m.drillDownStack[len(m.drillDownStack)-1]
		return truncateName(last.parentName, 20) + " > " + m.currentResource.String()
	}
	return m.currentResource.String()
}

func ansiTruncate(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	var result []byte
	w := 0
	inEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\x1b' {
			inEscape = true
			result = append(result, c)
			continue
		}
		if inEscape {
			result = append(result, c)
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEscape = false
			}
			continue
		}
		if w >= maxWidth {
			break
		}
		result = append(result, c)
		w++
	}
	result = append(result, "\x1b[0m"...)
	return string(result)
}

func truncateName(name string, max int) string {
	if len(name) <= max {
		return name
	}
	return name[:max-1] + "…"
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
	m.statusLine.SetDrillDown(m.drillDownPod != nil)
}

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

	lines := strings.Split(content, "\n")
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	lines = lines[:innerH]

	var b strings.Builder

	titleVis := lipgloss.Width(title)
	dashesAfter := innerW - 1 - titleVis
	if dashesAfter < 0 {
		dashesAfter = 0
	}
	b.WriteString(bStyle.Render("╭─"))
	b.WriteString(tStyle.Render(title))
	b.WriteString(bStyle.Render(strings.Repeat("─", dashesAfter) + "╮"))
	b.WriteString("\n")

	leftBorder := bStyle.Render("│")
	rightBorder := bStyle.Render("│")
	emptyLine := strings.Repeat(" ", innerW)
	for _, line := range lines {
		lw := lipgloss.Width(line)
		if lw > innerW {
			line = ansiTruncate(line, innerW)
			lw = lipgloss.Width(line)
		}
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		if line == "" {
			b.WriteString(leftBorder + emptyLine + rightBorder)
		} else {
			b.WriteString(leftBorder + line + pad + rightBorder)
		}
		b.WriteString("\n")
	}

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
