package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// helmDocRows are the Helm Release documents, listed as item operations
// of a Release row's Space menu (next to [Y]AML). Picking one fetches it
// with `helm get …` and opens it in the YAML viewer over the menu, so the
// menu is still there for the next document (tdp F4).
var helmDocRows = []struct {
	label, docKind, hint string
}{
	{"Manifest", k8s.HelmDocManifest, "rendered chart"},
	{"Creator Notes", k8s.HelmDocNotes, "post-install notes"},
	{"User Values", k8s.HelmDocUserValues, "user-supplied values"},
	{"Merged Values", k8s.HelmDocMergedValues, "incl. chart defaults"},
	{"Hooks", k8s.HelmDocHooks, "install/upgrade hook resources"},
}

// helmDocAction is the menu action of a document row.
const helmDocActionPrefix = "doc:"

func helmDocMenuItems() []menuItem {
	items := make([]menuItem, 0, len(helmDocRows))
	for _, d := range helmDocRows {
		items = append(items, menuItem{label: d.label, action: helmDocActionPrefix + d.docKind, hint: d.hint, opens: true})
	}
	return items
}

// helmDocKindOf returns the document kind a menu action names, or "".
func helmDocKindOf(action string) string {
	if !strings.HasPrefix(action, helmDocActionPrefix) {
		return ""
	}
	return strings.TrimPrefix(action, helmDocActionPrefix)
}

// HelmDocReadyMsg carries the result of an async helm CLI fetch.
// AppModel opens the YAML popup with Content (or surfaces Err to app log).
type HelmDocReadyMsg struct {
	DocKind     string
	ReleaseName string
	Namespace   string
	Content     string
	Err         error
}

// RollbackResultMsg carries the outcome of an async `helm rollback`.
// AppModel surfaces success as a toast and failure as an app-log error.
type RollbackResultMsg struct {
	ReleaseName string
	Namespace   string
	Revision    int
	Output      string
	Err         error
}

// rollbackReleaseCmd runs `helm rollback` asynchronously. 30s timeout is
// chosen on the long side because rollbacks can wait on K8s reconciliation
// (replicasets winding down, jobs running) — well above the doc-fetch
// budget but short enough to surface a hung helm.
func rollbackReleaseCmd(releaseName, namespace string, revision int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := k8s.RollbackRelease(ctx, releaseName, namespace, revision)
		return RollbackResultMsg{
			ReleaseName: releaseName,
			Namespace:   namespace,
			Revision:    revision,
			Output:      out,
			Err:         err,
		}
	}
}

// fetchHelmDocCmd runs `helm get <kind>` for the chosen doc and folds the
// result (or error) into a HelmDocReadyMsg. 10s timeout is generous: even
// `helm get manifest` on a big chart should finish well within that, but
// the hard cap stops a hung helm CLI from wedging the UI forever.
func fetchHelmDocCmd(docKind, releaseName, namespace string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		content, err := k8s.FetchHelmDoc(ctx, docKind, releaseName, namespace)
		return HelmDocReadyMsg{
			DocKind:     docKind,
			ReleaseName: releaseName,
			Namespace:   namespace,
			Content:     content,
			Err:         err,
		}
	}
}
