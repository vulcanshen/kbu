package ui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/vulcanshen/km8/internal/config"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

type resourceSwitchTickMsg struct {
	seq int
}

type drillDownMsg struct {
	parentType k8s.ResourceType
	parentName string
	childType  k8s.ResourceType
	children   []k8s.ResourceItem
}

// Main view layout — absolute cells, no percentages. Stack model:
//
//	horizontal: hMargin + sidebar(sw) + hSpace + rightSide(rw) + hMargin = m.width
//	vertical:   statusBar(1) + middle + statusLine = m.height
//	right side: table(upperH) + vSpace + detail(detailH) = middleH
//	sidebar:    fills middleH (no internal vSpace)
//
// Only sw and detailH are pinned; everything else falls out by subtraction.
const (
	panelSidebarWidth = 28 // panel 1 (sidebar) — fixed absolute width
	panelDetailHeight = 14 // panel 3 (detail)  — fixed absolute height
	panelHMargin      = 1  // cells between terminal left/right edge and panels
	panelHSpace       = 1  // cells between sidebar and right side
	panelVSpace       = 0  // rows between table and detail (0 = flush borders)
)

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
	confirm         ConfirmModel
	splash          SplashModel
	toast           ToastModel
	ptyView         *PtyView
	yamlPopup       YamlPopupModel

	activePanel     Panel
	width           int
	height          int
	theme           *theme.Theme
	cfgEditor       string
	editing         bool
	successNotice   string
	successNoticeID int
	k8sClient       *k8s.Client
	watcher         *k8s.Watcher
	logStreamer     *k8s.LogStreamer
	currentResource k8s.ResourceType
	items           []k8s.ResourceItem
	ready           bool
	logsActive      bool
	detailExpanded  bool
	tableExpanded   bool
	switchSeq       int

	// Drill-down state
	drillDownStack      []drillDownEntry
	drillDownPod        *k8s.ResourceItem // innermost: Pod → Container
	drillDownContainers []k8s.ContainerInfo
}

type drillDownEntry struct {
	parentType  k8s.ResourceType
	parentName  string
	parentItems []k8s.ResourceItem
}

func NewAppModel(t *theme.Theme, client *k8s.Client, cfgEditor string) AppModel {
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
		confirm:         NewConfirmModel(t),
		splash:          NewSplashModel(),
		toast:           NewToastModel(t),
		ptyView:         NewPtyView(),
		yamlPopup:       NewYamlPopupModel(t),
		activePanel:     SidebarPanel,
		theme:           t,
		cfgEditor:       cfgEditor,
		k8sClient:       client,
		watcher:         watcher,
		logStreamer:     logStreamer,
		currentResource: k8s.ResourcePods,
	}
}

type appInitMsg struct{ info k8s.ClusterInfo }

func (m AppModel) Init() tea.Cmd {
	m.watcher.Start(k8s.ResourcePods, m.k8sClient.GetNamespace())
	info := m.k8sClient.GetClusterInfo()
	return tea.Batch(
		m.sidebar.Init(),
		m.table.Init(),
		waitForWatchUpdate(m.watcher, m.currentResource),
		discoverCRDs(m.k8sClient),
		func() tea.Msg { return appInitMsg{info: info} },
	)
}

func discoverCRDs(client *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				config.WriteCrashLog(r)
			}
		}()
		count, err := k8s.DiscoverCRDs(context.Background(), client)
		return CRDsDiscoveredMsg{Count: count, Err: err}
	}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if _, ok := msg.(quitConfirmedMsg); ok {
		m.watcher.Stop()
		m.logStreamer.Stop()
		// Kill any persistent PTY (hidden KM8erm shell, mid-edit, mid-exec)
		// so we don't orphan the subprocess after km8 exits.
		if m.ptyView != nil && m.ptyView.IsAlive() {
			m.ptyView.Stop()
		}
		return m, tea.Quit
	}

	// detailSpinnerTickMsg drives the panel 3 refetch spinner; routed
	// unconditionally regardless of focus so the spinner keeps animating even
	// while the user is on Sidebar/Table panels.
	if _, ok := msg.(detailSpinnerTickMsg); ok {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}

	// PtyExitMsg arrives AFTER ptyView has already Stop()ed itself, so this
	// handler lives outside the IsActive() guard — it cleans up app-level
	// state when the subprocess finishes.
	if exit, ok := msg.(PtyExitMsg); ok {
		m.editing = false
		if exit.ExitCode != 0 {
			m.appLog.Warn(fmt.Sprintf("subprocess exited with code %d", exit.ExitCode))
		}
		return m, nil
	}

	if m.splash.IsActive() {
		var cmd tea.Cmd
		m.splash, cmd = m.splash.Update(msg)
		return m, cmd
	}

	if tickMsg, ok := msg.(AnimTickMsg); ok {
		var animCmds []tea.Cmd
		if c := m.help.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.appLog.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.confirm.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.contextPicker.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.namespacePicker.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		if c := m.yamlPopup.HandleTick(tickMsg); c != nil {
			animCmds = append(animCmds, c)
		}
		return m, tea.Batch(animCmds...)
	}

	// PtyView intercepts keys / ticks / resizes while a subprocess is running.
	// Other messages (watch updates, detail fetches, animation ticks) fall
	// through to the normal handlers — they update underlying state silently
	// behind the PTY popup so that when the subprocess exits, the table /
	// detail panels are already current.
	//
	// When the popup is *hidden* (alive KM8erm shell after Alt+T), ticks still
	// route into PtyView so exit detection keeps polling, but key input falls
	// through to the top-level handlers so the user can navigate km8 panels
	// without typing characters into the backgrounded shell.
	if m.ptyView.IsAlive() {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.ready = true
			m.layout()
			m.ptyView.SetSize(m.width, m.height)
			if m.ptyView.IsActive() {
				return m, nil
			}
			// Hidden: also let the underlying panels see the new size.
			// Fall through.
		case ptyTickMsg:
			var cmd tea.Cmd
			m.ptyView, cmd = m.ptyView.Update(msg)
			return m, cmd
		case tea.KeyMsg:
			if m.ptyView.IsActive() {
				var cmd tea.Cmd
				m.ptyView, cmd = m.ptyView.Update(msg)
				return m, cmd
			}
			// Hidden: fall through to top-level key routing (T re-shows,
			// q quits, n/c open pickers, etc.)
		}
	}

	if m.confirm.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.confirm, cmd = m.confirm.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

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

	if m.yamlPopup.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.yamlPopup, cmd = m.yamlPopup.Update(msg)
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
		case "ctrl+c":
			m.watcher.Stop()
			m.logStreamer.Stop()
			return m, tea.Quit
		case "q":
			quitCmd := func() tea.Msg {
				return quitConfirmedMsg{}
			}
			return m, m.confirm.Show(ConfirmQuit, "Quit km8?", "", quitCmd)
		case "V":
			return m, m.splash.Show()
		case "alt+t", "alt+T":
			// Alt+T is the single KM8erm toggle:
			//   - no shell alive   → spawn KM8erm
			//   - alive, hidden    → reattach (show)
			//   - alive, visible   → handled inside PtyView.Update (hides)
			// The "visible" branch never reaches here because PtyView
			// intercepts keys when IsActive() is true. Edit/Exec PTYs alive:
			// refuse, same as table-level edit/shell guard.
			if m.ptyView.IsAlive() {
				if m.ptyView.Kind() != PtyKindShell {
					m.appLog.Warn("close active PTY before opening KM8erm")
					return m, m.toast.Show("Close PTY (exit to close)")
				}
				m.ptyView.Show(m.width, m.height)
				return m, nil
			}
			cmd := buildShellTerminalCmd()
			return m, m.ptyView.Start(cmd, terminalTitle(), m.width, m.height, PtyKindShell)
		case "?":
			m.help.SetSize(m.width, m.height)
			return m, m.help.Toggle()
		case "!":
			m.appLog.SetSize(m.width, m.height)
			return m, m.appLog.Toggle()
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
		case "n", "N":
			return m, fetchNamespaces(m.k8sClient)
		case "c", "C":
			return m, fetchContexts(m.k8sClient)
		case "e":
			if !m.editing && m.activePanel == TablePanel && m.drillDownPod == nil && len(m.items) > 0 {
				idx := m.table.SelectedRow()
				if idx >= 0 && idx < len(m.items) {
					item := m.items[idx]
					detail := fmt.Sprintf("kubectl edit %s/%s", m.currentResource.KubectlName(), item.Name)
					if item.Namespace != "" {
						detail += " -n " + item.Namespace
					}
					startCmd := func() tea.Msg {
						return startEditMsg{resource: m.currentResource, item: item, contextName: m.k8sClient.ContextName()}
					}
					return m, m.confirm.Show(ConfirmEdit, "Edit resource?", detail, startCmd)
				}
			}
		case "s":
			if m.activePanel == TablePanel {
				return m, m.execShell()
			}
		case "D":
			if m.activePanel == TablePanel && m.drillDownPod == nil && len(m.items) > 0 {
				idx := m.table.SelectedRow()
				if idx >= 0 && idx < len(m.items) {
					item := m.items[idx]
					detail := fmt.Sprintf("kubectl delete %s %s -n %s", m.currentResource.KubectlName(), item.Name, item.Namespace)
					return m, m.confirm.Show(ConfirmDelete, "⚠ Delete resource? This cannot be undone.", detail,
						deleteResource(m.currentResource, item.Name, item.Namespace, m.k8sClient.ContextName()))
				}
			}
		case "=":
			if m.activePanel == DetailPanel {
				m.detailExpanded = true
				return m, nil
			}
			if m.activePanel == TablePanel {
				m.tableExpanded = true
				return m, nil
			}
		case "-":
			if m.detailExpanded || m.tableExpanded {
				m.detailExpanded = false
				m.tableExpanded = false
				return m, nil
			}
		case "y":
			return m, copyToClipboardCmd(m.focusedPanelContent())
		case "Y":
			yaml := m.detail.YAMLContent()
			if yaml == "" {
				return m, nil
			}
			var resource k8s.ResourceType
			var item k8s.ResourceItem
			if !m.editing && m.drillDownPod == nil && len(m.items) > 0 {
				idx := m.table.SelectedRow()
				if idx >= 0 && idx < len(m.items) {
					resource = m.currentResource
					item = m.items[idx]
				}
			}
			m.yamlPopup.SetSize(m.width, m.height)
			return m, m.yamlPopup.Open(yaml, resource, item, m.k8sClient.ContextName())
		}

	case ResourceSelectedMsg:
		m.appLog.Info("switched to " + msg.Type.String())
		m.currentResource = msg.Type
		m.drillDownStack = nil
		m.drillDownPod = nil
		m.drillDownContainers = nil
		m.logStreamer.Stop()
		m.logsActive = false
		m.watcher.Stop()
		m.detail.ClearDetail()
		m.detail.SetResourceType(msg.Type)
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
		m.switchSeq++
		seq := m.switchSeq
		cmds = append(cmds, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
			return resourceSwitchTickMsg{seq: seq}
		}))
		return m, tea.Batch(cmds...)

	case resourceSwitchTickMsg:
		if msg.seq != m.switchSeq {
			return m, nil
		}
		m.watcher.Start(m.currentResource, m.k8sClient.GetNamespace())
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		return m, tea.Batch(cmds...)

	case ResourceDataMsg:
		if msg.Type != m.currentResource {
			return m, nil
		}
		m.items = msg.Items
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		if m.drillDownPod != nil {
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
				cmds = append(cmds, fetchResourceDetail(m.k8sClient, msg.Type, item))
				if c := m.detail.BeginRefetch(); c != nil {
					cmds = append(cmds, c)
				}
				switch {
				case msg.Type == k8s.ResourcePods && !m.logsActive:
					containers := k8s.ContainerNames(item.Raw)
					if len(containers) > 0 {
						m.detail.logLines = nil
						m.logStreamer.Start(item.Name, item.Namespace, containers)
						m.logsActive = true
						cmds = append(cmds, waitForLogLine(m.logStreamer))
					}
				case msg.Type == k8s.ResourceDeployments && !m.logsActive:
					m.detail.logLines = nil
					cmds = append(cmds, startAggregateLogs(m.k8sClient, msg.Type, item))
				}
			}
		}
		return m, tea.Batch(cmds...)

	case ResourceErrorMsg:
		if msg.Err != nil {
			m.appLog.Error(msg.Err.Error())
		}
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		return m, tea.Batch(cmds...)

	case RowSelectedMsg:
		if m.drillDownPod != nil {
			// In drill-down: selected a container
			if msg.Index >= 0 && msg.Index < len(m.drillDownContainers) {
				c := m.drillDownContainers[msg.Index]
				detail := containerToDetail(c, *m.drillDownPod)
				m.detail.SetDetail(detail, nil)
				m.detail.logLines = nil
				m.logStreamer.Start(m.drillDownPod.Name, m.drillDownPod.Namespace, []string{c.Name})
				m.logsActive = true
				cmds = append(cmds, waitForLogLine(m.logStreamer))
			}
			return m, tea.Batch(cmds...)
		}
		if msg.Index >= 0 && msg.Index < len(m.items) && len(m.table.rows) > 0 {
			item := m.items[msg.Index]
			cmds = append(cmds, fetchResourceDetail(m.k8sClient, m.currentResource, item))
			if c := m.detail.BeginRefetch(); c != nil {
				cmds = append(cmds, c)
			}
			switch m.currentResource {
			case k8s.ResourcePods:
				containers := k8s.ContainerNames(item.Raw)
				if len(containers) > 0 {
					m.detail.logLines = nil
					m.logStreamer.Start(item.Name, item.Namespace, containers)
					m.logsActive = true
					cmds = append(cmds, waitForLogLine(m.logStreamer))
				}
			case k8s.ResourceDeployments:
				m.logStreamer.Stop()
				m.logsActive = false
				m.detail.logLines = nil
				cmds = append(cmds, startAggregateLogs(m.k8sClient, m.currentResource, item))
			default:
				m.logStreamer.Stop()
				m.logsActive = false
			}
		}
		return m, tea.Batch(cmds...)

	case LinkDrillMsg:
		// User pressed Enter on an Links ref. Fetch the target resource
		// off the Update path so the API call doesn't freeze the UI; the
		// fetched item lands as resourceFetchedForDrillMsg and opens YamlPopup.
		ref := msg.Ref
		client := m.k8sClient
		cmd := func() tea.Msg {
			item, err := k8s.FetchResourceByRef(context.Background(), client.Clientset(), ref)
			if err != nil {
				return resourceFetchedForDrillMsg{ref: ref, err: err}
			}
			yaml := k8s.MarshalItemYAML(item)
			return resourceFetchedForDrillMsg{ref: ref, item: item, yaml: yaml}
		}
		return m, cmd

	case resourceFetchedForDrillMsg:
		if msg.err != nil {
			m.appLog.Warn(fmt.Sprintf("drill %s/%s: %s", msg.ref.Type, msg.ref.Name, msg.err.Error()))
			return m, m.toast.Show("Drill failed — see App Log (!)")
		}
		if msg.yaml == "" {
			m.appLog.Warn(fmt.Sprintf("drill %s/%s: no YAML", msg.ref.Type, msg.ref.Name))
			return m, nil
		}
		m.yamlPopup.SetSize(m.width, m.height)
		return m, m.yamlPopup.Open(msg.yaml, msg.ref.Type, msg.item, m.k8sClient.ContextName())

	case aggregateLogsReadyMsg:
		// Stale result guard: user may have navigated to a different row
		// while the pod-list call was in flight.
		if msg.resource != m.currentResource {
			return m, nil
		}
		idx := m.table.SelectedRow()
		if idx < 0 || idx >= len(m.items) || m.items[idx].UID != msg.itemUID {
			return m, nil
		}
		if msg.err != nil {
			m.appLog.Warn("aggregate logs: " + msg.err.Error())
			return m, nil
		}
		if len(msg.targets) == 0 {
			m.appLog.Info("aggregate logs: no pods running")
			return m, nil
		}
		m.logStreamer.StartMulti(msg.targets)
		m.logsActive = true
		return m, waitForLogLine(m.logStreamer)

	case LogLineMsg:
		m.detail.AppendLogLine(msg.Pod, msg.Container, msg.Text)
		if m.logsActive {
			cmds = append(cmds, waitForLogLine(m.logStreamer))
		}
		return m, tea.Batch(cmds...)

	case ResourceDetailMsg:
		m.detail.SetDetail(msg.Detail, msg.Events)
		return m, nil

	case NamespaceListMsg:
		return m, m.namespacePicker.Open(msg.Namespaces)

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
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		return m, tea.Batch(cmds...)

	case ContextListMsg:
		return m, m.contextPicker.Open(msg.Contexts, msg.Current)

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
		newClient.Registry().ClearDynamic()
		m.k8sClient = newClient
		m.watcher = k8s.NewWatcher(newClient.Clientset())
		m.logStreamer = k8s.NewLogStreamer(newClient.Clientset())
		info := newClient.GetClusterInfo()
		m.statusBar.SetClusterInfo(info)
		m.statusBar.SetNamespace("")
		m.detail.ClearDetail()
		m.items = nil
		m.table.SetRows(nil)
		if m.currentResource.SupportsDrillDown() || k8s.DefaultRegistry.Get(m.currentResource) == nil {
			m.currentResource = k8s.ResourcePods
		}
		m.sidebar.RefreshCategories(newClient.Registry())
		m.watcher.Start(m.currentResource, m.k8sClient.GetNamespace())
		cmds = append(cmds, waitForWatchUpdate(m.watcher, m.currentResource))
		cmds = append(cmds, discoverCRDs(newClient))
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
			if c := m.detail.BeginRefetch(); c != nil {
				cmds = append(cmds, c)
			}
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

	case startShellExecMsg:
		if m.ptyView.IsAlive() {
			m.appLog.Warn("close active PTY before opening shell")
			return m, m.toast.Show("Close PTY (Alt+T to show, exit to close)")
		}
		cmd := buildKubectlExecCmd(msg.podName, msg.namespace, msg.container, msg.contextName)
		title := fmt.Sprintf("Shell: pod/%s → %s", msg.podName, msg.container)
		return m, m.ptyView.Start(cmd, title, m.width, m.height, PtyKindExec)

	case startEditMsg:
		if m.ptyView.IsAlive() {
			m.appLog.Warn("close active PTY before editing")
			return m, m.toast.Show("Close PTY (Alt+T to show, exit to close)")
		}
		m.editing = true
		title := fmt.Sprintf("Edit: %s/%s", msg.resource.KubectlName(), msg.item.Name)
		if msg.item.Namespace != "" {
			title += " (" + msg.item.Namespace + ")"
		}
		cmd := buildKubectlEditCmd(msg.resource, msg.item, msg.contextName, m.cfgEditor)
		config.WriteAuditEntry("edit", msg.resource.KubectlName()+"/"+msg.item.Name, msg.item.Namespace, "started") //nolint
		m.appLog.Info("edit: " + msg.resource.KubectlName() + "/" + msg.item.Name)
		return m, m.ptyView.Start(cmd, title, m.width, m.height, PtyKindEdit)

	case DeleteDoneMsg:
		out := strings.TrimSpace(msg.Output)
		if out == "" {
			out = "deleted " + msg.Name
		}
		m.appLog.Info(out)
		config.WriteAuditEntry("delete", msg.Resource, msg.Namespace, msg.Output) //nolint
		return m, nil

	case DeleteErrMsg:
		m.appLog.Error("delete failed: " + msg.Err.Error())
		return m, nil

	case appInitMsg:
		m.appLog.Info("km8 started")
		m.appLog.Info(fmt.Sprintf("connected to %s (%s)", msg.info.ContextName, msg.info.ServerURL))
		return m, nil

	case ClipboardCopiedMsg:
		notice := fmt.Sprintf("copied %d lines", msg.Lines)
		if msg.Lines == 1 {
			notice = "copied 1 line"
		}
		m.appLog.Info(notice)
		return m, m.toast.Show("Copied!")

	case ClipboardCopyFailedMsg:
		m.appLog.Warn("copy: " + msg.Reason)
		return m, nil

	case toastDismissMsg:
		m.toast.Update(msg)
		return m, nil

	case CRDsDiscoveredMsg:
		if msg.Err != nil {
			m.appLog.Warn("CRD discovery failed: " + msg.Err.Error())
		} else if msg.Count > 0 {
			m.appLog.Info(fmt.Sprintf("discovered %d CRDs", msg.Count))
			m.sidebar.RefreshCategories(m.k8sClient.Registry())
		}
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

	if m.splash.IsActive() {
		return m.splash.Render(m.width, m.height)
	}

	// KM8erm marker: render ONLY when the shell is alive but hidden — that's
	// the state the user can't otherwise see ("is there a shell waiting for
	// me?"). When the popup is visible, the popup's border already says
	// "KM8erm: hostname" so a status-bar duplicate just adds noise; when no
	// shell is running, nothing to mark.
	var ptyMarker *PtyMarker
	if m.ptyView != nil && m.ptyView.IsAlive() && m.ptyView.Kind() == PtyKindShell && m.ptyView.IsHidden() {
		ptyMarker = &PtyMarker{Visible: false, Label: " km8erm"}
	}
	statusBar := m.statusBar.ViewFull(m.appLog.UnreadErrorCount(), m.successNotice, ptyMarker)
	statusLine := m.statusLine.ViewWithNotice(m.appLog.UnreadErrorCount(), m.appLog.LastErrorMessage(), "")

	var mainView string

	if m.detailExpanded {
		panelH := m.height - 1 - m.statusLine.LineCount()
		panelW := m.width - 2*panelHMargin
		m.detail.SetSize(panelW-2, panelH-2)
		fullPanel := renderPanelWithScroll(m.detail.View(), "[3] "+m.detail.ActiveTabTitle()+m.detail.SpinnerSuffix(), panelW, panelH, true, m.theme, m.detail.ScrollInfo())
		hMargin := blankColumn(panelHMargin, panelH)
		middle := lipgloss.JoinHorizontal(lipgloss.Top, hMargin, fullPanel, hMargin)
		mainView = lipgloss.JoinVertical(lipgloss.Left, statusBar, middle, statusLine)
	} else if m.tableExpanded {
		_, _, upperH, detailH := m.panelSizes()
		panelW := m.width - 2*panelHMargin
		m.table.SetSize(panelW-2, upperH-2)
		m.detail.SetSize(panelW-2, detailH-2)
		tabTitle := "[2] " + m.breadcrumb() + "─" + m.detail.TabTitle()
		tablePanel := renderPanelWithScroll(m.table.View(), tabTitle, panelW, upperH, m.activePanel == TablePanel, m.theme, m.table.ScrollInfo())
		detailPanel := renderPanelWithScroll(m.detail.View(), "[3] "+m.detail.ActiveTabTitle()+m.detail.SpinnerSuffix(), panelW, detailH, m.activePanel == DetailPanel, m.theme, m.detail.ScrollInfo())
		middle := joinTableAndDetail(tablePanel, detailPanel, panelW)
		fullH := upperH + panelVSpace + detailH
		hMargin := blankColumn(panelHMargin, fullH)
		middleWithMargins := lipgloss.JoinHorizontal(lipgloss.Top, hMargin, middle, hMargin)
		mainView = lipgloss.JoinVertical(lipgloss.Left, statusBar, middleWithMargins, statusLine)
	} else {
		sw, rw, upperH, detailH := m.panelSizes()
		fullH := upperH + panelVSpace + detailH // sidebar matches right side total height
		m.sidebar.SetSize(sw-2, fullH-2)
		m.table.SetSize(rw-2, upperH-2)
		m.detail.SetSize(rw-2, detailH-2)

		sidebarPanel := renderPanelWithScroll(m.sidebar.View(), "[1] km8", sw, fullH, m.activePanel == SidebarPanel, m.theme, m.sidebar.ScrollInfo())
		tabTitle := "[2] " + m.breadcrumb() + "─" + m.detail.TabTitle()
		tablePanel := renderPanelWithScroll(m.table.View(), tabTitle, rw, upperH, m.activePanel == TablePanel, m.theme, m.table.ScrollInfo())
		detailPanel := renderPanelWithScroll(m.detail.View(), "[3] "+m.detail.ActiveTabTitle()+m.detail.SpinnerSuffix(), rw, detailH, m.activePanel == DetailPanel, m.theme, m.detail.ScrollInfo())

		rightSide := joinTableAndDetail(tablePanel, detailPanel, rw)

		// 1-col gap between sidebar and right side, plus 1-col margins on
		// the outer edges so panel borders sit 1 cell inside the terminal.
		hMargin := blankColumn(panelHMargin, fullH)
		hSpace := blankColumn(panelHSpace, fullH)
		middle := lipgloss.JoinHorizontal(lipgloss.Top, hMargin, sidebarPanel, hSpace, rightSide, hMargin)
		mainView = lipgloss.JoinVertical(lipgloss.Left, statusBar, middle, statusLine)
	}

	if m.appLog.IsActive() {
		m.appLog.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.appLog.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.help.IsActive() {
		m.help.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.help.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.confirm.IsActive() {
		m.confirm.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.confirm.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.contextPicker.IsActive() {
		mainView = overlay.Composite(m.contextPicker.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.namespacePicker.IsActive() {
		mainView = overlay.Composite(m.namespacePicker.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.yamlPopup.IsActive() {
		m.yamlPopup.SetSize(m.width, m.height)
		mainView = overlay.Composite(m.yamlPopup.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.toast.IsActive() {
		mainView = overlay.Composite(m.toast.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	if m.ptyView.IsActive() {
		mainView = overlay.Composite(m.ptyView.RenderPopup(), mainView, overlay.Center, overlay.Center, 0, 0)
	}

	return mainView
}

func (m *AppModel) layout() {
	m.statusBar.SetWidth(m.width)
	m.statusLine.SetWidth(m.width)
	m.help.SetSize(m.width, m.height)

	sw, rw, upperH, detailH := m.panelSizes()
	fullH := upperH + detailH
	m.sidebar.SetSize(sw-2, fullH-2)
	m.table.SetSize(rw-2, upperH-2)
	m.detail.SetSize(rw-2, detailH-2)
}

// panelSizes derives panel dimensions purely by subtraction from the terminal
// size, using the absolute constants above.
//
// Horizontal: m.width = hMargin + sw + hSpace + rw + hMargin
// Vertical:   middleH = m.height - statusBar(1) - statusLine.LineCount()
//
//	middleH = upperH + vSpace + detailH      (right side)
//	middleH = sidebar height                 (left side, no vSpace)
func (m AppModel) panelSizes() (sw, rw, upperH, detailH int) {
	sw = panelSidebarWidth
	rw = m.width - 2*panelHMargin - panelHSpace - sw

	middleH := m.height - 1 - m.statusLine.LineCount()
	detailH = panelDetailHeight
	upperH = middleH - panelVSpace - detailH

	// Tiny-terminal sanity clamps. Layout still expressed as
	// `available - reserved`, just rebalanced when reserved exceeds
	// available.
	const (
		minSw      = 12
		minRw      = 20
		minUpperH  = 4
		minDetailH = 4
	)
	if sw > m.width-2*panelHMargin-panelHSpace-minRw {
		sw = m.width - 2*panelHMargin - panelHSpace - minRw
	}
	if sw < minSw {
		sw = minSw
	}
	rw = m.width - 2*panelHMargin - panelHSpace - sw
	if rw < minRw {
		rw = minRw
	}

	if middleH < minUpperH+panelVSpace+minDetailH {
		middleH = minUpperH + panelVSpace + minDetailH
	}
	if detailH > middleH-panelVSpace-minUpperH {
		detailH = middleH - panelVSpace - minUpperH
	}
	if detailH < minDetailH {
		detailH = minDetailH
	}
	upperH = middleH - panelVSpace - detailH
	if upperH < minUpperH {
		upperH = minUpperH
		detailH = middleH - panelVSpace - upperH
	}
	return
}

// blankColumn builds a w×h block of spaces, suitable for use as a horizontal
// spacer column in lipgloss.JoinHorizontal. Returns "" for zero or negative
// dimensions.
func blankColumn(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	line := strings.Repeat(" ", w)
	if h == 1 {
		return line
	}
	return strings.Repeat(line+"\n", h-1) + line
}

// joinTableAndDetail vertically stacks the table + detail panels on the
// right side. When panelVSpace > 0, inserts that many blank rows between
// them; when 0 (current default) the borders sit flush.
func joinTableAndDetail(tablePanel, detailPanel string, w int) string {
	if panelVSpace <= 0 {
		return lipgloss.JoinVertical(lipgloss.Left, tablePanel, detailPanel)
	}
	spacer := blankColumn(w, panelVSpace)
	return lipgloss.JoinVertical(lipgloss.Left, tablePanel, spacer, detailPanel)
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
			d := containerToDetail(c, item)
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
	if c := m.detail.BeginRefetch(); c != nil {
		cmds = append(cmds, c)
	}
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

func containerToDetail(c k8s.ContainerInfo, pod k8s.ResourceItem) k8s.ResourceDetail {
	name := c.Name
	if c.Init {
		name = "(init) " + name
	}
	yaml := k8s.MarshalContainerYAML(pod, c.Name)

	// Structured fields are kept as a fallback for the rare case where YAML
	// extraction fails (e.g. pod.Raw not a *corev1.Pod).
	fields := []k8s.DetailField{
		{Label: "Pod", Value: pod.Name},
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
	return k8s.ResourceDetail{
		Name:      name,
		Namespace: pod.Namespace,
		Kind:      "Container",
		YAML:      yaml,
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

func (m *AppModel) execShell() tea.Cmd {
	var podName, namespace, container string

	if m.drillDownPod != nil {
		idx := m.table.SelectedRow()
		if idx < 0 || idx >= len(m.drillDownContainers) {
			return nil
		}
		podName = m.drillDownPod.Name
		namespace = m.drillDownPod.Namespace
		container = m.drillDownContainers[idx].Name
	} else {
		if m.currentResource != k8s.ResourcePods || len(m.items) == 0 {
			return nil
		}
		idx := m.table.SelectedRow()
		if idx < 0 || idx >= len(m.items) {
			return nil
		}
		item := m.items[idx]
		containers := k8s.ContainerNames(item.Raw)
		if len(containers) == 0 {
			return nil
		}
		podName = item.Name
		namespace = item.Namespace
		container = containers[0]
	}

	detail := fmt.Sprintf("kubectl exec -it %s -n %s -c %s", podName, namespace, container)
	showCmd := m.confirm.Show(ConfirmShellExec, "Exec into container?", detail,
		shellExec(podName, namespace, container, m.k8sClient.ContextName()))
	m.appLog.Info("exec shell: " + detail)
	return showCmd
}

// shellExec returns a Cmd that asks AppModel to launch a PTY for kubectl exec.
// AppModel cannot start the PTY directly from inside confirm's onConfirm
// closure because the closure has no access to model state — so we round-trip
// through startShellExecMsg.
func shellExec(podName, namespace, container, contextName string) tea.Cmd {
	return func() tea.Msg {
		return startShellExecMsg{
			podName:     podName,
			namespace:   namespace,
			container:   container,
			contextName: contextName,
		}
	}
}

type startShellExecMsg struct {
	podName, namespace, container, contextName string
}

func buildKubectlExecCmd(podName, namespace, container, contextName string) *exec.Cmd {
	args := []string{"exec", "-it", podName, "-n", namespace, "-c", container}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}
	args = append(args, "--", "/bin/sh")
	return exec.Command("kubectl", args...)
}

// buildShellTerminalCmd assembles the user's login shell command for the
// internal terminal popup. Inherits env / cwd from km8 so the user's aliases,
// PATH, and current directory are exactly what they'd see in a regular
// terminal — like `ssh localhost` but embedded.
func buildShellTerminalCmd() *exec.Cmd {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	// -l → login shell, sources .zprofile/.bash_profile/etc. so the user
	// gets the same environment as a fresh terminal tab.
	return exec.Command(sh, "-l")
}

// terminalTitle returns the popup title for the internal terminal — the
// host's name prefixed with the KM8erm tag, mirroring how an ssh prompt
// identifies the connection. Returns os.Hostname() verbatim; mDNS-style
// suffixes like `.home` / `.local` / `.lan` are passed through, because
// the user said so (some routers append `.home` and the user wants to
// keep that visible).
func terminalTitle() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "KM8erm"
	}
	return "KM8erm: " + h
}

func (m AppModel) focusedPanelContent() string {
	switch m.activePanel {
	case SidebarPanel:
		return m.sidebar.CopyableContent()
	case TablePanel:
		return m.table.CopyableContent()
	case DetailPanel:
		return m.detail.CopyableContent()
	}
	return ""
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

// ScrollInfo holds position info for the "X of Y" indicator in a panel border.
type ScrollInfo struct {
	Position int // 1-based current position
	Total    int
}

func renderPanel(content, title string, width, height int, focused bool, t *theme.Theme) string {
	return renderPanelWithScroll(content, title, width, height, focused, t, nil)
}

func renderPanelWithScroll(content, title string, width, height int, focused bool, t *theme.Theme, scroll *ScrollInfo) string {
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

	if scroll != nil && scroll.Total > 0 {
		indicator := fmt.Sprintf(" %d of %d ", scroll.Position, scroll.Total)
		dashes := innerW - len(indicator)
		if dashes < 0 {
			dashes = 0
		}
		b.WriteString(bStyle.Render("╰" + strings.Repeat("─", dashes) + indicator + "╯"))
	} else {
		b.WriteString(bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯"))
	}

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

// buildKubectlEditCmd assembles the `kubectl edit` command to run inside the
// PtyView. cfgEditor (from config.yaml) is exposed as $KUBE_EDITOR so kubectl
// honors the user's choice; if empty, kubectl falls back to $EDITOR / $VISUAL
// / vi / notepad on its own.
//
// The env is sanitized: vt10x is a basic VT100/xterm emulator and doesn't
// respond to advanced queries (DA1, color reports, etc.) that editors like
// nvim send when they detect terminal-program env vars (TERM_PROGRAM,
// KITTY_*, ITERM_*). Inheriting those values causes nvim to wait for query
// responses and time out on exit, producing a noticeable lag.
func buildKubectlEditCmd(rt k8s.ResourceType, item k8s.ResourceItem, contextName, cfgEditor string) *exec.Cmd {
	args := []string{"edit", rt.KubectlName() + "/" + item.Name}
	if item.Namespace != "" {
		args = append(args, "-n", item.Namespace)
	}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}
	c := exec.Command("kubectl", args...)
	c.Env = sanitizeEditorEnv(cfgEditor)
	return c
}

func sanitizeEditorEnv(cfgEditor string) []string {
	strip := []string{
		"TERM_PROGRAM",
		"TERM_PROGRAM_VERSION",
		"TERM_SESSION_ID",
		"KITTY_WINDOW_ID",
		"KITTY_PUBLIC_KEY",
		"ITERM_SESSION_ID",
		"ITERM_PROFILE",
		"LC_TERMINAL",
		"LC_TERMINAL_VERSION",
		"WEZTERM_EXECUTABLE",
		"WEZTERM_PANE",
		"GHOSTTY_RESOURCES_DIR",
		"COLORTERM",
		"TERM",
		"KUBE_EDITOR",
	}
	stripSet := make(map[string]struct{}, len(strip))
	for _, k := range strip {
		stripSet[k] = struct{}{}
	}
	env := make([]string, 0, len(os.Environ()))
	for _, v := range os.Environ() {
		eq := strings.IndexByte(v, '=')
		if eq < 0 {
			env = append(env, v)
			continue
		}
		if _, drop := stripSet[v[:eq]]; drop {
			continue
		}
		env = append(env, v)
	}
	env = append(env, "TERM=xterm-256color")
	if cfgEditor != "" {
		env = append(env, "KUBE_EDITOR="+cfgEditor)
	}
	return env
}

func deleteResource(rt k8s.ResourceType, name, namespace, contextName string) tea.Cmd {
	return func() tea.Msg {
		args := []string{"delete", rt.KubectlName(), name}
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		if contextName != "" {
			args = append(args, "--context", contextName)
		}
		c := exec.Command("kubectl", args...)
		var buf bytes.Buffer
		c.Stdout = &buf
		c.Stderr = &buf
		if err := c.Run(); err != nil {
			return DeleteErrMsg{Err: err}
		}
		return DeleteDoneMsg{
			Name:      name,
			Namespace: namespace,
			Resource:  string(rt.KubectlName()) + "/" + name,
			Output:    buf.String(),
		}
	}
}

func waitForWatchUpdate(w *k8s.Watcher, rt k8s.ResourceType) tea.Cmd {
	updates, errors := w.Channels()
	return func() tea.Msg {
		select {
		case msg, ok := <-updates:
			if !ok {
				return nil // channel closed by watcher.Start(); caller must not re-register
			}
			return ResourceDataMsg{Type: rt, Items: msg.Items}
		case errMsg, ok := <-errors:
			if !ok {
				return nil
			}
			return ResourceErrorMsg{Err: errMsg.Err}
		}
	}
}

func fetchResourceDetail(client *k8s.Client, rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				config.WriteCrashLog(r)
			}
		}()
		detail := k8s.GetResourceDetail(rt, item)
		detail.YAML = k8s.MarshalItemYAML(item)
		events, _ := k8s.FetchResourceEvents(context.Background(), client.Clientset(), item.Name, item.Namespace)
		return ResourceDetailMsg{Detail: detail, Events: events}
	}
}

// startAggregateLogs resolves a workload item to its current pod set and emits
// aggregateLogsReadyMsg with the targets. Runs off the Update path so the API
// list call doesn't block the UI. Includes the source item's UID so a stale
// result (e.g. user navigated to a different row in the meantime) can be
// filtered out by the handler.
func startAggregateLogs(client *k8s.Client, resource k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	return func() tea.Msg {
		targets, err := k8s.PodsForWorkload(context.Background(), client.Clientset(), item, true)
		return aggregateLogsReadyMsg{
			resource: resource,
			itemUID:  item.UID,
			targets:  targets,
			err:      err,
		}
	}
}

func waitForLogLine(ls *k8s.LogStreamer) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ls.Lines()
		if !ok {
			return nil
		}
		return LogLineMsg{Pod: line.Pod, Container: line.Container, Text: line.Text}
	}
}

// wrapWords wraps s at word boundaries to fit within width. Words longer than
// width are broken mid-word.
func wrapWords(s string, width int) []string {
	if width <= 0 || s == "" {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	var current string
	for _, w := range words {
		for len(w) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			lines = append(lines, w[:width])
			w = w[width:]
		}
		if current == "" {
			current = w
		} else if len(current)+1+len(w) <= width {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
