package k8s

// Helm CLI integration. km8 treats a Helm release as the 27th ResourceType so
// existing graph / drill machinery stays uniform — the only divergence is that
// the fetcher shells out to `helm` instead of using client-go. Registration is
// gated on `helm` being present on PATH; if not found, the entire Helm
// sidebar category never renders.

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	sigsyaml "sigs.k8s.io/yaml"
)

var (
	helmOnce      sync.Once
	helmAvailable bool
)

// HelmAvailable reports whether the `helm` CLI was detected at startup.
// Result is cached after the first detection call.
func HelmAvailable() bool {
	return helmAvailable
}

// RegisterHelmIfAvailable probes for the `helm` CLI on PATH and, if found,
// registers the ResourceReleases definition into DefaultRegistry. Safe to call
// multiple times — detection runs once.
func RegisterHelmIfAvailable() {
	helmOnce.Do(func() {
		if !detectHelm() {
			return
		}
		helmAvailable = true
		DefaultRegistry.Register(&ResourceDefinition{
			Type:            ResourceReleases,
			DisplayName:     "Releases",
			KubectlName:     "release",
			Category:        "Helm",
			CategoryOrder:   7, // after all native K8s categories
			OrderInCategory: 0,
			Columns: []Column{
				{Title: "Name", MinWidth: 20},
				{Title: "Namespace", MinWidth: 12},
				{Title: "Chart", MinWidth: 20},
				{Title: "App Ver", MinWidth: 10},
				{Title: "Rev", MinWidth: 4},
				{Title: "Status", MinWidth: 10},
				{Title: "Updated", MinWidth: 8},
			},
			Fetcher:      fetchReleases,
			Detailer:     detailRelease,
			WatchStarter: helmPollWatch, // helm has no watch API; refresh via 5s polling
		})
	})
}

func detectHelm() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}

// Release is the parsed shape of one entry from `helm list -o json`.
// Field names match the helm CLI's JSON output verbatim.
type Release struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Revision   string `json:"revision"`
	Updated    string `json:"updated"`
	Status     string `json:"status"`
	Chart      string `json:"chart"`
	AppVersion string `json:"app_version"`
}

// ReleaseRevision is the parsed shape of one entry from
// `helm history <rel> -o json`. Note `revision` is JSON-numeric here even
// though `helm list -o json` reports it as a string — keep the int type
// rather than fight the helm CLI's inconsistency.
type ReleaseRevision struct {
	Revision    int    `json:"revision"`
	Updated     string `json:"updated"` // RFC3339 with offset (different from helm list's Go time.String)
	Status      string `json:"status"`
	Chart       string `json:"chart"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
}

// parseReleaseHistory is split from fetchHelmHistory so unit tests can feed
// canned JSON without exec'ing helm.
func parseReleaseHistory(data []byte) ([]ReleaseRevision, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var revisions []ReleaseRevision
	if err := json.Unmarshal(data, &revisions); err != nil {
		return nil, fmt.Errorf("parse helm history output: %w", err)
	}
	return revisions, nil
}

// fetchHelmHistory runs `helm history <rel> -n <ns> -o json`. Returns the
// revision list (newest revision last, matching helm CLI ordering).
func fetchHelmHistory(ctx context.Context, releaseName, namespace string) ([]ReleaseRevision, error) {
	out, err := exec.CommandContext(ctx, "helm", "history", releaseName, "-n", namespace, "-o", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("helm history %s: %w", releaseName, err)
	}
	return parseReleaseHistory(out)
}

// enrichReleaseHistory fills detail.ReleaseHistory by running
// `helm history`. Quiet on error — failing to fetch history shouldn't break
// the rest of the release detail view.
func enrichReleaseHistory(ctx context.Context, _ kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	revs, err := fetchHelmHistory(ctx, item.Name, item.Namespace)
	if err != nil {
		return
	}
	detail.ReleaseHistory = revs
}

// RollbackRelease runs `helm rollback <rel> <rev> -n <ns>` and returns the
// helm CLI's stdout (typically "Rollback was a success! Happy Helming!").
// Errors propagate from helm — caller should surface them to app log.
func RollbackRelease(ctx context.Context, releaseName, namespace string, revision int) (string, error) {
	out, err := exec.CommandContext(ctx, "helm", "rollback", releaseName, fmt.Sprintf("%d", revision), "-n", namespace).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("helm rollback: %w", err)
	}
	return string(out), nil
}

// RollbackCommandString returns the exact shell command that RollbackRelease
// will execute, suitable for showing in a confirm popup so the user sees
// what's about to run.
func RollbackCommandString(releaseName, namespace string, revision int) string {
	return fmt.Sprintf("helm rollback %s %d -n %s", releaseName, revision, namespace)
}

// IsHelmManaged reports whether a K8s resource was created by Helm. Detection
// matches kubectl's own heuristic: either the `app.kubernetes.io/managed-by`
// label set to "Helm" or the `meta.helm.sh/release-name` annotation set by
// the helm renderer. Both are reliable post-helm-v3.
func IsHelmManaged(item ResourceItem) bool {
	if item.Raw == nil {
		return false
	}
	accessor, err := meta.Accessor(item.Raw)
	if err != nil {
		return false
	}
	if accessor.GetLabels()["app.kubernetes.io/managed-by"] == "Helm" {
		return true
	}
	if _, ok := accessor.GetAnnotations()["meta.helm.sh/release-name"]; ok {
		return true
	}
	return false
}

// IsHelmStorageSecret reports whether a Secret is one of helm's per-revision
// release storage blobs (`type: helm.sh/release.v1`). These dominate the
// Secrets list in any cluster running helm — useful by themselves, but
// usually just noise relative to the real workload secrets users want to
// inspect.
func IsHelmStorageSecret(item ResourceItem) bool {
	sec, ok := item.Raw.(*corev1.Secret)
	if !ok {
		return false
	}
	return string(sec.Type) == "helm.sh/release.v1"
}

// helmHideManaged is a session-local atomic toggle for "filter out
// helm-managed items from any resource list". Default false — surface
// every resource the cluster actually holds; users running helm-heavy
// workloads explicitly toggle hide via `.` when they want to focus on
// non-helm objects. Atomic so watcher fetcher goroutine and UI goroutine
// can read/write without a mutex.
var helmHideManaged atomic.Bool

func init() {
	helmHideManaged.Store(false)
}

// HelmHideManaged reports whether the global helm-managed filter is on.
func HelmHideManaged() bool { return helmHideManaged.Load() }

// SetHelmHideManaged sets the global filter state.
func SetHelmHideManaged(v bool) { helmHideManaged.Store(v) }

// ToggleHelmHideManaged flips the filter and returns the new value.
func ToggleHelmHideManaged() bool {
	v := !helmHideManaged.Load()
	helmHideManaged.Store(v)
	return v
}

// HelmIcon returns the HELM SYMBOL ⎈ (U+2388) — a STANDARD Unicode
// codepoint in the Miscellaneous Technical block, NOT a Nerd Font PUA
// glyph. Used as the popup title icon and the panel 2 row marker,
// unified so the helm-managed signal is visually consistent across
// panels. Picked over Nerd Font nf-dev-helm (U+E7FB) and nf-md-ship_wheel
// (U+F0832) because U+2388 renders in any monospace font — no NF
// dependency, no font-fallback gap (e.g. Termius's built-in fonts cover
// it but not PUA-A icons). It is also the de-facto Kubernetes/Helm
// ecosystem symbol — `kubectl version` and Helm docs use it directly.
func HelmIcon() string { return "⎈" }

// HelmRowMark returns the same glyph as HelmIcon. Kept as a separate
// function so the popup vs row call sites can diverge later without
// touching every consumer.
func HelmRowMark() string { return "⎈" }

// MarkHelm returns the helm row marker when the item is helm-managed
// (either by label/annotation, or — for Secrets — as a helm storage
// blob), else "". Used as the cell value for the unlabeled marker
// column right after Name on every resource type.
func MarkHelm(item ResourceItem) string {
	if IsHelmManaged(item) || IsHelmStorageSecret(item) {
		return HelmRowMark()
	}
	return ""
}

// FormatHelmHistoryDate converts the RFC3339 timestamp in
// ReleaseRevision.Updated into the same age string the rest of km8 uses
// (e.g. "5d", "12h"). Falls back to the raw string when parsing fails so
// users still see something rather than an empty cell.
func FormatHelmHistoryDate(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Older helm builds occasionally drop sub-second precision; try the
		// nanosecond layout as a fallback before giving up.
		if t2, err2 := time.Parse("2006-01-02T15:04:05.999999999Z07:00", s); err2 == nil {
			return formatAge(t2)
		}
		return s
	}
	return formatAge(t)
}

// parseReleases is split from fetchReleases so unit tests can feed canned JSON
// without exec'ing helm.
func parseReleases(data []byte) ([]Release, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var releases []Release
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, fmt.Errorf("parse helm list output: %w", err)
	}
	return releases, nil
}

// fetchReleases runs `helm list -o json` for the given namespace and converts
// the parsed releases into ResourceItem rows. An empty namespace falls back to
// `-A` (all namespaces) so caller code that has not yet selected a namespace
// still sees something useful.
func fetchReleases(ctx context.Context, _ kubernetes.Interface, namespace string) ([]ResourceItem, error) {
	args := []string{"list", "-o", "json"}
	if namespace == "" {
		args = append(args, "-A")
	} else {
		args = append(args, "-n", namespace)
	}
	out, err := exec.CommandContext(ctx, "helm", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("helm list: %w", err)
	}
	releases, err := parseReleases(out)
	if err != nil {
		return nil, err
	}
	items := make([]ResourceItem, 0, len(releases))
	for i := range releases {
		r := releases[i]
		items = append(items, ResourceItem{
			Name:      r.Name,
			Namespace: r.Namespace,
			UID:       fmt.Sprintf("helm/%s/%s", r.Namespace, r.Name),
			Raw:       &releases[i],
			Row: []string{
				r.Name,
				r.Namespace,
				r.Chart,
				r.AppVersion,
				r.Revision,
				r.Status,
				formatHelmUpdated(r.Updated),
			},
		})
	}
	return items, nil
}

// formatHelmUpdated converts the helm CLI's `updated` field (Go time.String()
// layout, e.g. "2024-05-19 14:31:22.123456 +0800 CST") into an age string
// matching the rest of km8. Falls back to the raw string on parse failure.
func formatHelmUpdated(s string) string {
	if s == "" {
		return "<unknown>"
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", s)
	if err != nil {
		return s
	}
	return formatAge(t)
}

// Helm document kinds surfaced as Relatives Info rows. The Label string is
// what shows on the row and is also how the UI dispatches Y → helm CLI fetch
// (no extra metadata on RelativeRow needed). Keep in sync with
// IsHelmDocLabel below.
const (
	HelmDocManifest     = "manifest"
	HelmDocNotes        = "notes"
	HelmDocUserValues   = "user values"
	HelmDocMergedValues = "merged values"
	HelmDocHooks        = "hooks"
)

// IsHelmDocLabel reports whether a RelativeRow.Label corresponds to one of
// the five Helm Info documents. Used by the UI to decide whether Y goes
// through the helm CLI path vs. the default kubectl-yaml path.
func IsHelmDocLabel(label string) bool {
	switch label {
	case HelmDocManifest, HelmDocNotes, HelmDocUserValues, HelmDocMergedValues, HelmDocHooks:
		return true
	}
	return false
}

// FetchHelmDoc shells out to `helm get <subcommand> <release> -n <ns>` for
// the given document label. Returns the raw stdout. The caller decides how
// to render it (manifest/values/hooks are YAML; notes is plain text — both
// go through the same YAML popup since notes still reads fine in monospace).
func FetchHelmDoc(ctx context.Context, label, releaseName, namespace string) (string, error) {
	if !IsHelmDocLabel(label) {
		return "", fmt.Errorf("unknown helm doc label: %q", label)
	}
	args := []string{"get"}
	switch label {
	case HelmDocManifest:
		args = append(args, "manifest")
	case HelmDocNotes:
		args = append(args, "notes")
	case HelmDocUserValues:
		args = append(args, "values")
	case HelmDocMergedValues:
		args = append(args, "values", "--all")
	case HelmDocHooks:
		args = append(args, "hooks")
	}
	args = append(args, releaseName, "-n", namespace)
	out, err := exec.CommandContext(ctx, "helm", args...).Output()
	if err != nil {
		return "", fmt.Errorf("helm %v: %w", args, err)
	}
	return string(out), nil
}

// detailRelease is the sync detailer — produces Fields only. The
// Relatives "Deployed Resources" section is filled in asynchronously by
// enrichReleaseRelatives (via EnrichRelatives) because it needs to shell
// out to `helm get manifest` and parse the multi-doc YAML.
func detailRelease(item ResourceItem) ResourceDetail {
	d := ResourceDetail{
		Name:      item.Name,
		Namespace: item.Namespace,
		Kind:      "Release",
		UID:       item.UID,
	}
	rel, ok := item.Raw.(*Release)
	if !ok || rel == nil {
		return d
	}
	d.Fields = []DetailField{
		{Label: "Chart", Value: rel.Chart},
		{Label: "App Version", Value: rel.AppVersion},
		{Label: "Revision", Value: rel.Revision},
		{Label: "Status", Value: rel.Status},
		{Label: "Updated", Value: rel.Updated},
	}
	// Helm Info (manifest / notes / values) is NOT surfaced via Relatives —
	// that tab is the navigation hub for K8s refs, not a document browser.
	// Instead, panel 2 `Space` on a Release row opens a HelmDocMenu popup
	// (see internal/ui/helmdocmenu.go). See PROGRESS.md Helm category design.
	return d
}

// manifestResource is one entry parsed out of `helm get manifest`. It carries
// the minimum needed to build a RelativeRow — kind for label, name + ns to
// look up the resource via client-go later.
type manifestResource struct {
	Kind      string
	Name      string
	Namespace string
}

// parseManifestResources tears the multi-doc YAML output of
// `helm get manifest` into a flat list of {kind, name, namespace} tuples.
// Empty documents and entries missing kind/name are skipped silently —
// helm prepends a leading "---\n# Source: ..." that yields an empty first
// chunk after the split, and helm-rendered comment-only blocks have no
// kind/metadata.name.
//
// Splitting on "\n---\n" rather than running a full streaming YAML parser
// is intentional: helm's renderer never emits an unquoted "---" inside
// values, so the literal separator is safe and the dependency surface
// stays small.
func parseManifestResources(manifestYAML string) []manifestResource {
	if manifestYAML == "" {
		return nil
	}
	parts := strings.Split(manifestYAML, "\n---\n")
	var out []manifestResource
	for _, p := range parts {
		var meta struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
		}
		if err := sigsyaml.Unmarshal([]byte(p), &meta); err != nil {
			continue
		}
		if meta.Kind == "" || meta.Metadata.Name == "" {
			continue
		}
		out = append(out, manifestResource{
			Kind:      meta.Kind,
			Name:      meta.Metadata.Name,
			Namespace: meta.Metadata.Namespace,
		})
	}
	return out
}

// enrichReleaseRelatives runs `helm get manifest` on the release and turns
// each native K8s resource the chart deployed into a drillable
// RelativeRow. Quiet on error — the user already sees the release in
// panel 2 + the Fields panel; an enrichment failure shouldn't break the
// detail view. CRD kinds are dropped (kindToResourceType returns "")
// rather than emitting non-drillable rows.
func enrichReleaseRelatives(ctx context.Context, _ kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	manifest, err := FetchHelmDoc(ctx, HelmDocManifest, item.Name, item.Namespace)
	if err != nil {
		return
	}
	resources := parseManifestResources(manifest)
	if len(resources) == 0 {
		return
	}
	var entries []RelativeRow
	for _, r := range resources {
		rt, ok := kindToResourceType(r.Kind)
		if !ok {
			continue
		}
		ref := RefTarget{Type: rt, Name: r.Name, Namespace: r.Namespace}
		entries = append(entries, RelativeRow{
			Label: r.Kind,
			Value: r.Name,
			Ref:   &ref,
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
		Title:   fmt.Sprintf("Deployed Resources (%d)", len(entries)),
		Entries: entries,
	})
}

// helmPollInterval is how often helm releases get re-listed when sitting on
// the Releases panel. Helm has no watch API, so the only refresh mechanism
// is polling. 3s matches the scout workflow — external `helm install`
// shows up within a couple seconds, cost is one cheap exec per tick.
const helmPollInterval = 3 * time.Second

// helmPollWatch satisfies the WatchStarter signature for the Helm category.
// It periodically fires a watch.Modified event, which makes the existing
// Watcher.run loop re-call FetchResources — reusing the same re-list path the
// k8s informer watch uses on update events. Returning a permanently-closed
// channel would busy-spin the outer loop; this fires once per
// helmPollInterval instead.
func helmPollWatch(ctx context.Context, _ kubernetes.Interface, _ string) (watch.Interface, error) {
	return newPollWatch(ctx, helmPollInterval), nil
}

type pollWatch struct {
	ch     chan watch.Event
	cancel context.CancelFunc
}

func newPollWatch(parent context.Context, interval time.Duration) *pollWatch {
	ctx, cancel := context.WithCancel(parent)
	pw := &pollWatch{
		ch:     make(chan watch.Event, 1),
		cancel: cancel,
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				close(pw.ch)
				return
			case <-t.C:
				select {
				case pw.ch <- watch.Event{Type: watch.Modified}:
				case <-ctx.Done():
					close(pw.ch)
					return
				}
			}
		}
	}()
	return pw
}

func (w *pollWatch) Stop()                          { w.cancel() }
func (w *pollWatch) ResultChan() <-chan watch.Event { return w.ch }
