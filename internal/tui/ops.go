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
// switch ask first, every time (Rules F6).

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

func (m *Model) editHere() tea.Cmd {
	item, ok := m.currentItem()
	if !ok || !allowsEdit(m.shownKind()) || k8s.IsHelmManaged(item) {
		return nil
	}
	return m.confirmEdit(m.shownKind(), item)
}

// confirmEdit asks, then runs kubectl edit in a terminal popup.
func (m *Model) confirmEdit(rt k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	args := m.kubectlArgs(namespaced([]string{"edit", rt.KubectlName() + "/" + item.Name}, item.Namespace)...)
	return m.app.Confirm(terminu.Ask{Title: "Edit", Subject: rt.KubectlName() + "/" + item.Name,
		Details: []string{"kubectl " + strings.Join(args, " ")}, Question: "Edit " + rt.KubectlName() + "/" + item.Name + "?",
		Verb: "edit"}, func() tea.Cmd {
		if m.tx != nil && m.tx.Running() {
			return m.toastWarn("Close the current edit or shell first")
		}
		_ = config.WriteAuditEntry("edit", rt.KubectlName()+"/"+item.Name, item.Namespace, "started")
		m.log.info("edit: " + rt.KubectlName() + "/" + item.Name)
		var env []string
		if m.cfg.Editor != "" {
			env = append(env, "KUBE_EDITOR="+m.cfg.Editor)
		}
		m.tx = &terminu.Terminal{Title: "Edit", Subject: subject(rt, item), Command: append([]string{"kubectl"}, args...),
			Env: env, Ends: true, History: true, OnExit: m.txExited("edit")}
		return m.app.OpenTerminal(m.tx)
	})
}

func namespaced(args []string, ns string) []string {
	if ns != "" {
		args = append(args, "-n", ns)
	}
	return args
}

// shellHere asks, then runs kubectl exec into the pod's first container, or
// the container under the cursor.
func (m *Model) shellHere() tea.Cmd {
	var pod k8s.ResourceItem
	var container string
	if c, p, ok := m.currentContainer(); ok {
		pod, container = p, c.Name
	} else if item, ok := m.currentItem(); ok && m.shownKind() == k8s.ResourcePods {
		names := k8s.ContainerNames(item.Raw)
		if len(names) == 0 {
			return nil
		}
		pod, container = item, names[0]
	} else {
		return nil
	}
	shown := fmt.Sprintf("kubectl exec -it %s -n %s -c %s", pod.Name, pod.Namespace, container)
	return m.app.Confirm(terminu.Ask{Title: "Shell", Subject: "pod/" + pod.Name, Details: []string{shown},
		Question: "Exec into " + container + "?", Verb: "exec"}, func() tea.Cmd {
		if m.tx != nil && m.tx.Running() {
			return m.toastWarn("Close the current edit or shell first")
		}
		m.log.info("exec shell: " + shown)
		args := m.kubectlArgs("exec", "-it", pod.Name, "-n", pod.Namespace, "-c", container)
		// bash when the image has it, sh otherwise; the probe's output is
		// silenced, the shell's own is not (bash writes its prompt on
		// stderr).
		args = append(args, "--", "/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi")
		m.tx = &terminu.Terminal{Title: "Shell", Subject: "pod/" + pod.Name + " → " + container,
			Command: append([]string{"kubectl"}, args...), Ends: true, History: true, OnExit: m.txExited("shell")}
		return m.app.OpenTerminal(m.tx)
	})
}

// txExited is what an edit's or a shell's end says when it failed.
func (m *Model) txExited(what string) func(error) tea.Cmd {
	return func(err error) tea.Cmd {
		if err != nil {
			// The popup closes with the program: what failed is said at
			// once (Rules F5) and kept in the App Log.
			return m.toastErr(what + " ended: " + err.Error())
		}
		return nil
	}
}

// --- delete ----------------------------------------------------------------

func (m *Model) deleteHere() tea.Cmd {
	item, ok := m.currentItem()
	rt := m.shownKind()
	if !ok || !allowsDelete(rt) || k8s.IsHelmManaged(item) {
		return nil
	}
	args := m.kubectlArgs(namespaced([]string{"delete", rt.KubectlName(), item.Name}, item.Namespace)...)
	details := []string{"kubectl " + strings.Join(args, " ")}
	warn := []int{}
	if rt == k8s.ResourceNamespaces {
		details = append(details, "This removes ALL resources inside it.")
		warn = append(warn, len(details)-1)
	}
	details = append(details, "This cannot be undone.")
	warn = append(warn, len(details)-1)
	target := rt.KubectlName() + "/" + item.Name
	return m.app.Confirm(terminu.Ask{Title: "Delete", Subject: target, Details: details, Warn: warn,
		Question: "Delete " + target + "?", Verb: "delete"}, func() tea.Cmd {
		ns := item.Namespace
		return func() tea.Msg {
			out, err := m.run("kubectl", args...)
			return commandDoneMsg{what: "delete", target: target + "\x00" + ns, out: out, err: err}
		}
	})
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

// confirmRollback asks, then rolls the release back to the revision under
// the History tab's cursor.
func (m *Model) confirmRollback() tea.Cmd {
	rev, ok := m.revisionAt()
	item, ok2 := m.currentItem()
	if !ok || !ok2 || rev.Revision == currentRevision(m.det.detail.ReleaseHistory) {
		return nil
	}
	return m.app.Confirm(terminu.Ask{Title: "Rollback", Subject: item.Name,
		Details:  []string{k8s.RollbackCommandString(item.Name, item.Namespace, rev.Revision)},
		Question: fmt.Sprintf("Roll %s back to revision %d?", item.Name, rev.Revision), Verb: "roll back"}, func() tea.Cmd {
		target := fmt.Sprintf("%s to revision %d", item.Name, rev.Revision)
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			out, err := k8s.RollbackRelease(ctx, item.Name, item.Namespace, rev.Revision)
			return commandDoneMsg{what: "rollback", target: target, out: out, err: err}
		}
	})
}

// --- contexts ----------------------------------------------------------------

// confirmContextHere asks, then switches kbu to the context under the
// cursor of the Contexts list.
func (m *Model) confirmContextHere() tea.Cmd {
	item, ok := m.currentItem()
	if !ok || m.client == nil || item.Name == m.client.ContextName() {
		return nil
	}
	return m.app.Confirm(terminu.Ask{Title: "Switch context", Subject: item.Name,
		Details: []string{"~/.kube/config is not changed"}, Question: "Switch kbu to context " + item.Name + "?",
		Verb: "switch"}, func() tea.Cmd { return m.switchContext(item.Name) })
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
