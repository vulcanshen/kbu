package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// What kbu does to the cluster and to the clipboard. Every kubectl it runs
// carries kbu's own --context; edit, shell, delete, a rollback and a context
// switch ask first, every time (tdp F6).

// copyText copies text to the clipboard with OSC 52, which also works over
// ssh and in tmux (wrapped for it).
func copyText(text string) tea.Cmd {
	return func() tea.Msg {
		if text == "" {
			return copiedMsg{err: fmt.Errorf("no content to copy")}
		}
		seq := osc52.New(text)
		if os.Getenv("TMUX") != "" {
			seq = seq.Tmux()
		}
		if _, err := seq.WriteTo(os.Stderr); err != nil {
			return copiedMsg{err: err}
		}
		n := strings.Count(text, "\n")
		if !strings.HasSuffix(text, "\n") {
			n++
		}
		return copiedMsg{lines: n}
	}
}

// copyRow copies panel 2's row under the cursor, its cells tab-separated
// (the Helm mark left out).
func (m *Model) copyRow() tea.Cmd {
	r, ok := m.list.Current()
	if !ok {
		return nil
	}
	var cells []string
	for i, c := range r.Cells {
		if i < len(m.list.Columns) && m.list.Columns[i].Title == "" {
			continue
		}
		cells = append(cells, strings.TrimSuffix(terminu.StripStyles(c), " *"))
	}
	return copyText(strings.Join(cells, "\t"))
}

func (m *Model) copyTab() tea.Cmd {
	if b, ok := m.detailP.Body.(*textBody); ok {
		return copyText(strings.TrimRight(b.text(), "\n"))
	}
	return nil
}

func (m *Model) copyRelative() tea.Cmd {
	e, ok := m.relAt()
	if !ok {
		return nil
	}
	return copyText(e.label + "\t" + e.value)
}

func (m *Model) copyRevision() tea.Cmd {
	rev, ok := m.revisionAt()
	if !ok {
		return nil
	}
	return copyText(strings.Join([]string{strconv.Itoa(rev.Revision), rev.Updated, rev.Status, rev.Chart, rev.AppVersion, rev.Description}, "\t"))
}

// subject is how a resource is named in a title: kind/name (namespace).
func subject(rt k8s.ResourceType, item k8s.ResourceItem) string {
	s := rt.KubectlName() + "/" + item.Name
	if item.Namespace != "" {
		s += " (" + item.Namespace + ")"
	}
	return s
}

// --- YAML -----------------------------------------------------------------

func (m *Model) yamlHere() tea.Cmd {
	item, ok := m.currentItem()
	if !ok {
		return nil
	}
	return m.openYAML(m.shownKind(), item, k8s.MarshalItemYAML(item))
}

// yamlOfLevel is the YAML of what panel 3 is at: the chain's last resource
// on the Relatives tab, a container, or the row.
func (m *Model) yamlOfLevel() tea.Cmd {
	if n := len(m.det.chain); n > 0 && m.currentTab() == "Relatives" {
		f := m.det.chain[n-1]
		return m.openYAML(f.ref.Type, f.item, f.detail.YAML)
	}
	if c, pod, ok := m.currentContainer(); ok {
		return m.openYAML(k8s.ResourcePods, pod, k8s.MarshalContainerYAML(pod, c.Name))
	}
	return m.yamlHere()
}

// yamlOfRelative fetches the relative under the cursor and opens its YAML.
func (m *Model) yamlOfRelative() tea.Cmd {
	e, ok := m.relAt()
	if !ok || e.ref == nil || m.client == nil {
		return nil
	}
	ref, cs := *e.ref, m.client.Clientset()
	return func() tea.Msg {
		item, err := k8s.FetchResourceByRef(context.Background(), cs, ref)
		return refYAMLMsg{ref: ref, item: item, err: err}
	}
}

// --- edit and shell --------------------------------------------------------
//
// Each asks first (tdp F6). Its question is built for the row under the
// cursor when panel 2's actions are laid out (Action.Ask), so that from the
// Space menu Esc on it goes back to the menu (tdp F4) and Enter clears the
// stack for the long session (tdp T1).

func (m *Model) editArgs(rt k8s.ResourceType, item k8s.ResourceItem) []string {
	return m.kubectlArgs(namespaced([]string{"edit", rt.KubectlName() + "/" + item.Name}, item.Namespace)...)
}

func (m *Model) editAsk(rt k8s.ResourceType, item k8s.ResourceItem) *terminu.Ask {
	target := rt.KubectlName() + "/" + item.Name
	return &terminu.Ask{Title: "Edit", Subject: target, Details: []string{"kubectl " + strings.Join(m.editArgs(rt, item), " ")},
		Question: "Edit " + target + "?", Verb: "edit"}
}

// confirmEdit asks, then edits: for the YAML viewer's E, where no Action
// carries the question. then runs when the edit ends.
func (m *Model) confirmEdit(rt k8s.ResourceType, item k8s.ResourceItem, then func() tea.Cmd) tea.Cmd {
	return m.app.Confirm(*m.editAsk(rt, item), func() tea.Cmd { return m.runEdit(rt, item, then) })
}

// runEdit runs kubectl edit in a terminal popup; then, when not nil, runs
// when it ends.
func (m *Model) runEdit(rt k8s.ResourceType, item k8s.ResourceItem, then func() tea.Cmd) tea.Cmd {
	if m.tx != nil && m.tx.Running() {
		return m.app.Error("Edit", "An edit or a shell is open already: close it first.")
	}
	_ = config.WriteAuditEntry("edit", rt.KubectlName()+"/"+item.Name, item.Namespace, "started")
	m.log.info("edit: " + rt.KubectlName() + "/" + item.Name)
	var env []string
	if m.cfg.Editor != "" {
		env = append(env, "KUBE_EDITOR="+m.cfg.Editor)
	}
	m.tx = &terminu.Terminal{Title: "Edit", Subject: subject(rt, item), Command: append([]string{"kubectl"}, m.editArgs(rt, item)...),
		Env: env, Ends: true, History: true, OnExit: m.txExited("edit", then)}
	return m.app.OpenTerminal(m.tx)
}

// rereadMsg is a resource read again after an edit, for the viewer that
// show puts it in.
type rereadMsg struct {
	key  string
	item k8s.ResourceItem
	err  error
	show func(yaml string)
}

// rereadYAML reads the resource again for the YAML viewer an edit was
// started from: from the API for the kinds kbu knows, else as the watch has
// it (onReread).
func (m *Model) rereadYAML(rt k8s.ResourceType, item k8s.ResourceItem, show func(string)) tea.Cmd {
	if m.client == nil {
		return nil
	}
	cs, ref := m.client.Clientset(), k8s.RefTarget{Type: rt, Name: item.Name, Namespace: item.Namespace}
	return func() tea.Msg {
		it, err := k8s.FetchResourceByRef(context.Background(), cs, ref)
		return rereadMsg{key: itemKey(item), item: it, err: err, show: show}
	}
}

// onReread puts what was read in the viewer; when it could not be read
// (a CRD), what the watch has now.
func (m *Model) onReread(msg rereadMsg) tea.Cmd {
	if msg.err == nil {
		msg.show(k8s.MarshalItemYAML(msg.item))
		return nil
	}
	if i := indexOfKey(m.shownItems(), msg.key); i >= 0 {
		msg.show(k8s.MarshalItemYAML(m.shownItems()[i]))
		return nil
	}
	m.log.warn("YAML not read again after the edit: " + msg.err.Error())
	return nil
}

func namespaced(args []string, ns string) []string {
	if ns != "" {
		args = append(args, "-n", ns)
	}
	return args
}

// shellTarget is the container a shell goes into: the one under the cursor
// at a pod's containers, or a pod's first.
func (m *Model) shellTarget() (k8s.ResourceItem, string, bool) {
	if c, p, ok := m.currentContainer(); ok {
		return p, c.Name, true
	}
	if item, ok := m.currentItem(); ok && m.shownKind() == k8s.ResourcePods {
		if names := k8s.ContainerNames(item.Raw); len(names) > 0 {
			return item, names[0], true
		}
	}
	return k8s.ResourceItem{}, "", false
}

// shellArgs is the kubectl exec: bash when the image has it, sh otherwise;
// the probe's output is silenced, the shell's own is not (bash writes its
// prompt on stderr).
func (m *Model) shellArgs(pod k8s.ResourceItem, container string) []string {
	args := m.kubectlArgs("exec", "-it", pod.Name, "-n", pod.Namespace, "-c", container)
	return append(args, "--", "/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi")
}

func (m *Model) shellAsk() *terminu.Ask {
	pod, container, ok := m.shellTarget()
	if !ok {
		return nil
	}
	return &terminu.Ask{Title: "Shell", Subject: "pod/" + pod.Name,
		Details:  []string{"kubectl " + strings.Join(m.shellArgs(pod, container), " ")},
		Question: "Exec into " + container + "?", Verb: "exec"}
}

// runShell runs kubectl exec in a terminal popup.
func (m *Model) runShell() tea.Cmd {
	pod, container, ok := m.shellTarget()
	if !ok {
		return nil
	}
	if m.tx != nil && m.tx.Running() {
		return m.app.Error("Shell", "An edit or a shell is open already: close it first.")
	}
	args := m.shellArgs(pod, container)
	m.log.info("exec shell: kubectl " + strings.Join(args, " "))
	m.tx = &terminu.Terminal{Title: "Shell", Subject: "pod/" + pod.Name + " → " + container,
		Command: append([]string{"kubectl"}, args...), Ends: true, History: true, OnExit: m.txExited("shell", nil)}
	return m.app.OpenTerminal(m.tx)
}

// txExited is what an edit's or a shell's end says when it failed.
func (m *Model) txExited(what string, then func() tea.Cmd) func(error) tea.Cmd {
	return func(err error) tea.Cmd {
		var cmds []tea.Cmd
		if err != nil {
			// The popup closes with the program: what failed is said at
			// once (tdp F5) and kept in the App Log.
			cmds = append(cmds, m.toastErr(what+" ended: "+err.Error()))
		}
		if then != nil {
			cmds = append(cmds, then())
		}
		return tea.Batch(cmds...)
	}
}

// --- delete ----------------------------------------------------------------

func (m *Model) deleteArgs(rt k8s.ResourceType, item k8s.ResourceItem) []string {
	return m.kubectlArgs(namespaced([]string{"delete", rt.KubectlName(), item.Name}, item.Namespace)...)
}

// deleteAsk is the delete's question: the exact command, and that it cannot
// be undone; a namespace's also that everything in it goes (in Peach).
func (m *Model) deleteAsk(rt k8s.ResourceType, item k8s.ResourceItem) *terminu.Ask {
	details := []string{"kubectl " + strings.Join(m.deleteArgs(rt, item), " ")}
	var warn []int
	if rt == k8s.ResourceNamespaces {
		details = append(details, "This removes ALL resources inside it.")
		warn = append(warn, len(details)-1)
	}
	details = append(details, "This cannot be undone.")
	warn = append(warn, len(details)-1)
	target := rt.KubectlName() + "/" + item.Name
	return &terminu.Ask{Title: "Delete", Subject: target, Details: details, Warn: warn,
		Question: "Delete " + target + "?", Verb: "delete"}
}

// runDelete runs kubectl delete off the update loop.
func (m *Model) runDelete(rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	if !allowsDelete(rt) || k8s.IsHelmManaged(item) {
		return nil
	}
	args := m.deleteArgs(rt, item)
	target := rt.KubectlName() + "/" + item.Name
	return func() tea.Msg {
		out, err := m.run("kubectl", args...)
		return commandDoneMsg{what: "delete", target: target + "\x00" + item.Namespace, out: out, err: err}
	}
}

// onCommandDone says how a delete or a rollback went.
func (m *Model) onCommandDone(msg commandDoneMsg) tea.Cmd {
	target, ns, _ := strings.Cut(msg.target, "\x00")
	out := strings.TrimSpace(msg.out)
	if msg.err != nil {
		if out != "" {
			out = ": " + out
		}
		return m.toastErr(fmt.Sprintf("%s %s failed: %v%s", msg.what, target, msg.err, out))
	}
	switch msg.what {
	case "delete":
		_ = config.WriteAuditEntry("delete", target, ns, msg.out)
		if out == "" {
			out = "deleted " + target
		}
		m.log.success(out)
		return m.app.Toast(terminu.Info, out)
	case "rollback":
		m.log.success(out)
		return m.app.Toast(terminu.Info, "Rolled back "+target)
	}
	return nil
}

// --- Helm --------------------------------------------------------------------

// helmDoc is one of a release's documents, read with helm get.
type helmDoc struct{ kind, label, desc string }

var helmDocs = []helmDoc{
	{k8s.HelmDocManifest, "Manifest", "rendered chart"},
	{k8s.HelmDocNotes, "Creator Notes", "post-install notes"},
	{k8s.HelmDocUserValues, "User Values", "user-supplied values"},
	{k8s.HelmDocMergedValues, "Merged Values", "incl. chart defaults"},
	{k8s.HelmDocHooks, "Hooks", "install/upgrade hook resources"},
}

func (m *Model) fetchHelmDoc(d helmDoc) tea.Cmd {
	item, ok := m.currentItem()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		content, err := k8s.FetchHelmDoc(ctx, d.kind, item.Name, item.Namespace)
		return helmDocMsg{label: d.label, release: item.Name, namespace: item.Namespace, content: content, err: err}
	}
}

func (m *Model) onHelmDoc(msg helmDocMsg) tea.Cmd {
	if msg.err != nil {
		return m.toastErr(fmt.Sprintf("helm %s of %s: %v", strings.ToLower(msg.label), msg.release, msg.err))
	}
	lang := "yaml"
	if msg.label == "Creator Notes" {
		lang = ""
	}
	return m.app.Viewer(&terminu.Viewer{Title: msg.label, Subject: "release/" + msg.release, Glyph: glyphYAML,
		Lines: strings.Split(strings.TrimRight(msg.content, "\n"), "\n"), Lang: lang,
		Keys:   []terminu.ViewerKey{{Key: "y", Desc: "copy", Do: func() tea.Cmd { return copyText(msg.content) }}},
		Select: m.selection()})
}

// rollbackAsk is the rollback's question for the revision under the
// History tab's cursor: the exact helm command. Nil for no revision.
func (m *Model) rollbackAsk() *terminu.Ask {
	rev, ok := m.revisionAt()
	item, ok2 := m.currentItem()
	if !ok || !ok2 {
		return nil
	}
	return &terminu.Ask{Title: "Rollback", Subject: item.Name,
		Details:  []string{k8s.RollbackCommandString(item.Name, item.Namespace, rev.Revision)},
		Question: fmt.Sprintf("Roll %s back to revision %d?", item.Name, rev.Revision), Verb: "roll back"}
}

// runRollback rolls the release back to the revision under the cursor, off
// the update loop.
func (m *Model) runRollback() tea.Cmd {
	rev, ok := m.revisionAt()
	item, ok2 := m.currentItem()
	if !ok || !ok2 || rev.Revision == currentRevision(m.det.detail.ReleaseHistory) {
		return nil
	}
	target := fmt.Sprintf("%s to revision %d", item.Name, rev.Revision)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := k8s.RollbackRelease(ctx, item.Name, item.Namespace, rev.Revision)
		return commandDoneMsg{what: "rollback", target: target, out: out, err: err}
	}
}

// --- contexts ----------------------------------------------------------------

// contextAsk is the question before switching to the context under the
// cursor of the Contexts list.
func contextAsk(item k8s.ResourceItem) *terminu.Ask {
	return &terminu.Ask{Title: "Switch context", Subject: item.Name,
		Details: []string{"~/.kube/config is not changed"}, Question: "Switch kbu to context " + item.Name + "?",
		Verb: "switch"}
}

// switchContext points kbu at another context of the kubeconfig — kbu only;
// the kubeconfig is not written.
func (m *Model) switchContext(name string) tea.Cmd {
	c, err := k8s.NewClient(name)
	if err != nil {
		return m.toastErr("context switch failed: " + err.Error())
	}
	m.log.info("context switched to " + name)
	if m.watcher != nil {
		m.watcher.Stop()
	}
	m.stopLogs()
	k8s.DefaultRegistry.ClearDynamic()
	m.client = c
	m.watcher = k8s.NewWatcher(c.Clientset())
	m.logs = k8s.NewLogStreamer(c.Clientset())
	rt := m.current
	if rt.SupportsDrillDown() || k8s.DefaultRegistry.Get(rt) == nil {
		rt = k8s.ResourcePods
	}
	m.refreshKinds()
	m.kinds.Select(string(rt))
	m.current, m.kindSeq = "", 0
	return tea.Batch(m.selectKind(rt, false), discoverCRDs(c))
}
