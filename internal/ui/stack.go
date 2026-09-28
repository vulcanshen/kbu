package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// stackLayer is one popup in the stack. Every popup View composites
// implements it, so the stack order lives in one list (stackOrder) and
// every question about "the popup on top" — which one takes the key,
// which one a click lands on, which one is drawn last, how deep the
// stack is — reads that same list (tdp D3, F3, F4, X2).
type stackLayer interface {
	// owns: opening or open. The popup holds its place in the stack
	// and, when it is the top one, takes keys and clicks. A popup
	// running its close animation does not (tdp F3).
	owns() bool
	// ready: fully open. A key that reaches a popup still opening is
	// dropped rather than handed to the layer beneath.
	ready() bool
	// drawn: still on screen, close animation included.
	drawn() bool
	resize(w, h int)
	render() string
	closeLayer() tea.Cmd
	key(tea.KeyMsg) tea.Cmd
	click(msg tea.MouseMsg, screenW, screenH int) tea.Cmd
}

// stackOrder is the popup stack bottom-first: View draws in this order,
// and Update hands keys and clicks to the last layer that owns its
// place, so the popup drawn on top is the one that answers. The order
// follows who opens whom — a menu sits below the pickers, viewers and
// confirms it opens, the key reference sits above them all, and the
// PTYs (context-shift targets that clear everything beneath, tdp T1)
// sit on top.
func (m *AppModel) stackOrder() []stackLayer {
	return []stackLayer{
		&m.hintPopup, &m.spaceMenu, &m.globalMenu,
		&m.listPicker, &m.sortDirPicker, &m.settingsPopup, &m.namespacePicker, &m.contextPicker, &m.appLog,
		&m.breadcrumbPopup, &m.yamlPopup, &m.comparePopup,
		&m.confirm, &m.help,
		m.shellPty, m.txPty,
	}
}

// topLayer returns the popup that takes keys and clicks: the last one in
// stackOrder that owns its place. nil when no popup is up.
func (m *AppModel) topLayer() stackLayer {
	order := m.stackOrder()
	for i := len(order) - 1; i >= 0; i-- {
		if order[i].owns() {
			return order[i]
		}
	}
	return nil
}

// isMenuLayer reports whether a layer is a short pick-from-a-list popup.
// Those ignore the mouse wheel: half-page scrolling makes no sense on a
// handful of rows, and a synthesized u / d would reach nothing.
func isMenuLayer(l stackLayer) bool {
	switch l.(type) {
	case *MenuPopupModel, *ListPickerModel, *SettingsPopupModel,
		*HintPopupModel, *BreadcrumbPopupModel,
		*NamespacePickerModel, *ContextPickerModel, *ConfirmModel:
		return true
	}
	return false
}

// ── adapters ─────────────────────────────────────────────────────────
//
// Each popup keeps its own value-receiver Update / HandleMouse; these
// thin pointer-receiver wrappers let the stack drive them in place.

func (m *HintPopupModel) owns() bool          { return m.animator.Owns() }
func (m *HintPopupModel) ready() bool         { return m.animator.IsInteractive() }
func (m *HintPopupModel) drawn() bool         { return m.animator.IsActive() }
func (m *HintPopupModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *HintPopupModel) render() string      { return m.RenderPopup() }
func (m *HintPopupModel) closeLayer() tea.Cmd { return m.Close() }
func (m *HintPopupModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *HintPopupModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *MenuPopupModel) owns() bool          { return m.animator.Owns() }
func (m *MenuPopupModel) ready() bool         { return m.animator.IsInteractive() }
func (m *MenuPopupModel) drawn() bool         { return m.animator.IsActive() }
func (m *MenuPopupModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *MenuPopupModel) render() string      { return m.RenderPopup() }
func (m *MenuPopupModel) closeLayer() tea.Cmd { return m.Close() }
func (m *MenuPopupModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *MenuPopupModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *ListPickerModel) owns() bool          { return m.animator.Owns() }
func (m *ListPickerModel) ready() bool         { return m.animator.IsInteractive() }
func (m *ListPickerModel) drawn() bool         { return m.animator.IsActive() }
func (m *ListPickerModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *ListPickerModel) render() string      { return m.RenderPopup() }
func (m *ListPickerModel) closeLayer() tea.Cmd { return m.Close() }
func (m *ListPickerModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *ListPickerModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *SettingsPopupModel) owns() bool          { return m.animator.Owns() }
func (m *SettingsPopupModel) ready() bool         { return m.animator.IsInteractive() }
func (m *SettingsPopupModel) drawn() bool         { return m.animator.IsActive() }
func (m *SettingsPopupModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *SettingsPopupModel) render() string      { return m.RenderPopup() }
func (m *SettingsPopupModel) closeLayer() tea.Cmd { return m.Close() }
func (m *SettingsPopupModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *SettingsPopupModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *NamespacePickerModel) owns() bool          { return m.animator.Owns() }
func (m *NamespacePickerModel) ready() bool         { return m.animator.IsInteractive() }
func (m *NamespacePickerModel) drawn() bool         { return m.animator.IsActive() }
func (m *NamespacePickerModel) resize(int, int)     {}
func (m *NamespacePickerModel) render() string      { return m.RenderPopup() }
func (m *NamespacePickerModel) closeLayer() tea.Cmd { return m.Close() }
func (m *NamespacePickerModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *NamespacePickerModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *ContextPickerModel) owns() bool          { return m.animator.Owns() }
func (m *ContextPickerModel) ready() bool         { return m.animator.IsInteractive() }
func (m *ContextPickerModel) drawn() bool         { return m.animator.IsActive() }
func (m *ContextPickerModel) resize(int, int)     {}
func (m *ContextPickerModel) render() string      { return m.RenderPopup() }
func (m *ContextPickerModel) closeLayer() tea.Cmd { return m.Close() }
func (m *ContextPickerModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *ContextPickerModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *AppLogModel) owns() bool          { return m.animator.Owns() }
func (m *AppLogModel) ready() bool         { return m.animator.IsInteractive() }
func (m *AppLogModel) drawn() bool         { return m.animator.IsActive() }
func (m *AppLogModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *AppLogModel) render() string      { return m.RenderPopup() }
func (m *AppLogModel) closeLayer() tea.Cmd { return m.Close() }
func (m *AppLogModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *AppLogModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *BreadcrumbPopupModel) owns() bool          { return m.animator.Owns() }
func (m *BreadcrumbPopupModel) ready() bool         { return m.animator.IsInteractive() }
func (m *BreadcrumbPopupModel) drawn() bool         { return m.animator.IsActive() }
func (m *BreadcrumbPopupModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *BreadcrumbPopupModel) render() string      { return m.RenderPopup() }
func (m *BreadcrumbPopupModel) closeLayer() tea.Cmd { return m.Close() }
func (m *BreadcrumbPopupModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *BreadcrumbPopupModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *YamlPopupModel) owns() bool          { return m.animator.Owns() }
func (m *YamlPopupModel) ready() bool         { return m.animator.IsInteractive() }
func (m *YamlPopupModel) drawn() bool         { return m.animator.IsActive() }
func (m *YamlPopupModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *YamlPopupModel) render() string      { return m.RenderPopup() }
func (m *YamlPopupModel) closeLayer() tea.Cmd { return m.Close() }
func (m *YamlPopupModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *YamlPopupModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *CompareYamlPopupModel) owns() bool          { return m.animator.Owns() }
func (m *CompareYamlPopupModel) ready() bool         { return m.animator.IsInteractive() }
func (m *CompareYamlPopupModel) drawn() bool         { return m.animator.IsActive() }
func (m *CompareYamlPopupModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *CompareYamlPopupModel) render() string      { return m.RenderPopup() }
func (m *CompareYamlPopupModel) closeLayer() tea.Cmd { return m.Close() }
func (m *CompareYamlPopupModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *CompareYamlPopupModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *ConfirmModel) owns() bool          { return m.animator.Owns() }
func (m *ConfirmModel) ready() bool         { return m.animator.IsInteractive() }
func (m *ConfirmModel) drawn() bool         { return m.animator.IsActive() }
func (m *ConfirmModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *ConfirmModel) render() string      { return m.RenderPopup() }
func (m *ConfirmModel) closeLayer() tea.Cmd { return m.Close() }
func (m *ConfirmModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *ConfirmModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

func (m *HelpModel) owns() bool          { return m.animator.Owns() }
func (m *HelpModel) ready() bool         { return m.animator.IsInteractive() }
func (m *HelpModel) drawn() bool         { return m.animator.IsActive() }
func (m *HelpModel) resize(w, h int)     { m.SetSize(w, h) }
func (m *HelpModel) render() string      { return m.RenderPopup() }
func (m *HelpModel) closeLayer() tea.Cmd { return m.Close() }
func (m *HelpModel) key(k tea.KeyMsg) tea.Cmd {
	var c tea.Cmd
	*m, c = m.Update(k)
	return c
}
func (m *HelpModel) click(msg tea.MouseMsg, w, h int) tea.Cmd {
	var c tea.Cmd
	*m, c = m.HandleMouse(msg, w, h)
	return c
}

// A PTY owns its place while its subprocess is alive, not hidden and the
// frame is not closing. Keys reach it from the first frame of its open
// animation — they belong to the subprocess (tdp K10), and dropping the
// first keystrokes typed into a shell would lose input. A nil slot (test
// fixtures) is simply never up.

func (p *PtyView) owns() bool {
	return p != nil && p.active && !p.hidden && p.animator.Owns()
}
func (p *PtyView) ready() bool { return p.owns() }
func (p *PtyView) drawn() bool { return p != nil && p.IsRendered() }

// resize is a no-op: resizing a PTY resizes the subprocess's terminal,
// which the WindowSizeMsg handler does once per real resize — not on
// every View.
func (p *PtyView) resize(int, int) {}
func (p *PtyView) render() string  { return p.RenderPopup() }
func (p *PtyView) closeLayer() tea.Cmd {
	return nil // a PTY leaves through its own exit key or its subprocess
}
func (p *PtyView) key(k tea.KeyMsg) tea.Cmd {
	_, c := p.Update(k)
	return c
}
func (p *PtyView) click(tea.MouseMsg, int, int) tea.Cmd {
	return nil // the subprocess owns the grid; kbu has no mouse action there
}
