package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// linkEntry is one row of the Links tab. Two flavours:
//   - section: header row (just a label, not selectable)
//   - drill:   label + value with a ref (Enter fetches & opens YamlPopup)
//
// There's no "info-only selectable row" by design — Links is strictly a
// navigation hub. Pure-info content (labels, annotations, container image
// strings, status fields) lives in the `Y` YAML popup. Cursor only lands on
// rows that have somewhere to go.
type linkEntry struct {
	label   string
	value   string
	ref     *k8s.RefTarget
	section bool
}

func (e linkEntry) isSelectable() bool { return !e.section && e.ref != nil }

// buildPodLinkEntries renders the Pod-specific Links list using the parsed
// PodLinksData stashed in ResourceDetail. Returns an empty slice when no
// PodLinks are present (e.g. detail still loading) — the renderer will then
// show a placeholder hint.
//
// Strict refs only: Owner, Node, ServiceAccount, and Volumes whose source is
// another K8s resource (ConfigMap / Secret / PVC). Volumes with non-K8s
// sources (emptyDir / hostPath / projected / downwardAPI) and container
// images are intentionally excluded — they're not navigable, so they belong
// in the YAML popup, not here.
func buildPodLinkEntries(detail k8s.ResourceDetail) []linkEntry {
	if detail.PodLinks == nil {
		return nil
	}
	po := detail.PodLinks
	var entries []linkEntry

	if po.Owner != nil {
		entries = append(entries, linkEntry{
			label: "Owner",
			value: ownerDisplay(*po.Owner),
			ref:   po.Owner,
		})
	}
	if po.Node != nil {
		entries = append(entries, linkEntry{
			label: "Node",
			value: po.Node.Name,
			ref:   po.Node,
		})
	}
	if po.ServiceAccount != nil {
		entries = append(entries, linkEntry{
			label: "ServiceAccount",
			value: po.ServiceAccount.Name,
			ref:   po.ServiceAccount,
		})
	}

	// Only drillable volumes — others would be unreachable cursor positions.
	var drillVols []k8s.VolumeRef
	for _, v := range po.Volumes {
		if v.Ref != nil {
			drillVols = append(drillVols, v)
		}
	}
	if len(drillVols) > 0 {
		entries = append(entries, linkEntry{section: true, label: "Volumes"})
		for _, v := range drillVols {
			entries = append(entries, linkEntry{
				label: "  " + v.Name,
				value: v.Kind + "/" + v.Ref.Name,
				ref:   v.Ref,
			})
		}
	}

	return entries
}

// buildServiceLinkEntries renders Service Links: the set of Pods selected
// by the Service's label selector. Each pod is drillable to its YAML.
// Empty when the Service has no selector (ExternalName, headless without
// selector) — the placeholder line takes over.
func buildServiceLinkEntries(detail k8s.ResourceDetail) []linkEntry {
	sl := detail.ServiceLinks
	if sl == nil || len(sl.Pods) == 0 {
		return nil
	}
	entries := []linkEntry{
		{section: true, label: fmt.Sprintf("Pods (%d)", len(sl.Pods))},
	}
	for i := range sl.Pods {
		p := &sl.Pods[i]
		entries = append(entries, linkEntry{
			label: "  " + p.Name,
			value: "pod",
			ref:   p,
		})
	}
	return entries
}

// buildGenericLinkEntries converts the generic detail.Links payload
// (populated by per-kind detailXxx + EnrichLinks in the k8s layer) into
// linkEntry rows. Empty input returns nil so the renderer falls back to
// the "no links — press Y" placeholder.
func buildGenericLinkEntries(detail k8s.ResourceDetail) []linkEntry {
	if len(detail.Links) == 0 {
		return nil
	}
	var entries []linkEntry
	for _, sec := range detail.Links {
		if sec.Title != "" {
			entries = append(entries, linkEntry{section: true, label: sec.Title})
		}
		for i := range sec.Entries {
			row := sec.Entries[i]
			entries = append(entries, linkEntry{
				label: row.Label,
				value: row.Value,
				ref:   row.Ref,
			})
		}
	}
	return entries
}

// linksApplicable returns false for kinds where the Links tab has nothing
// meaningful to surface. Such kinds drop the tab entirely (handled by
// SetResourceType) instead of showing an empty pane.
func linksApplicable(rt k8s.ResourceType) bool {
	return rt != k8s.ResourceNamespaces
}

// linksPlaceholderEmpty is shown when the active resource has no refs to
// drill into right now. Every non-Namespace kind has a Links builder, so
// this is the only placeholder users will see — empty means "this
// instance genuinely has nothing", not "we haven't written the code yet."
const linksPlaceholderEmpty = "(no links to show — press Y for full YAML)"

func ownerDisplay(ref k8s.RefTarget) string {
	// Short kind label + name. Use the registry display name when available,
	// otherwise fall back to the raw type string.
	kind := string(ref.Type)
	if def := k8s.DefaultRegistry.Get(ref.Type); def != nil {
		kind = strings.TrimSuffix(def.DisplayName, "s")
	}
	return fmt.Sprintf("%s/%s", kind, ref.Name)
}

// renderLinkEntries turns the entry list into display lines, applying
// styles and adding a cursor highlight on `cursor`. The caller picks the
// placeholder text shown when entries is empty — that's how we surface
// "no links to show" vs "kind not yet supported" without renderLinkEntries
// having to know about resource types. Returns:
//   - lines:           rendered display lines
//   - selectableIdxs:  indices into `entries` that the cursor can land on
//   - cursorLine:      display-line index of the cursor row (-1 if none) —
//     used by the caller to auto-scroll the viewport so
//     the cursor stays visible
func renderLinkEntries(entries []linkEntry, cursor int, width int, t *theme.Theme, placeholder string) (lines []string, selectableIdxs []int, cursorLine int) {
	cursorLine = -1
	labelStyle := t.DetailLabelStyle()
	valueStyle := t.DetailValueStyle()
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Sidebar.CategoryFg)).Bold(true)
	drillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Sidebar.CategoryFg)).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	cursorRowStyle := t.TableSelectedRowStyle()

	const labelW = 18
	const indent = "  "

	if len(entries) == 0 {
		lines = append(lines, indent+dimStyle.Render(placeholder))
		return lines, nil, -1
	}

	for i, e := range entries {
		if e.isSelectable() {
			selectableIdxs = append(selectableIdxs, i)
		}
	}

	rowWidth := width
	if rowWidth < 20 {
		rowWidth = 20
	}

	for i, e := range entries {
		if e.section {
			lines = append(lines, indent+sectionStyle.Render(e.label))
			continue
		}
		isCursor := cursor >= 0 && cursor < len(entries) && cursor == i
		labelText := e.label
		if len(labelText) < labelW {
			labelText = labelText + strings.Repeat(" ", labelW-len(labelText))
		}

		// Wrap value to fit (rowWidth - labelPrefixW), with continuation
		// lines indented under the value column. Same behavior for
		// cursor and non-cursor rows — the previous lipgloss.Width()
		// cursor-only wrap left non-cursor rows truncated by the outer
		// panel.
		//
		// Strategy: wrap (value + arrow) together so the arrow lands on
		// the right chunk, then split the arrow back off the last
		// chunk to render it in drillStyle. Keeps the arrow color
		// consistent with the original non-wrap rendering.
		labelPrefix := "  " + labelText + " "
		labelPrefixW := lipgloss.Width(labelPrefix)
		hasArrow := e.ref != nil
		valueAndArrow := e.value
		if hasArrow {
			valueAndArrow += " →"
		}
		valueBudget := rowWidth - labelPrefixW
		if valueBudget < 10 {
			valueBudget = 10
		}
		chunks := wrapPlain(valueAndArrow, valueBudget)
		arrowChunkIdx := -1
		if hasArrow && len(chunks) > 0 {
			last := len(chunks) - 1
			if strings.HasSuffix(chunks[last], " →") {
				chunks[last] = strings.TrimSuffix(chunks[last], " →")
				if chunks[last] == "" && last > 0 {
					// Arrow ended up alone on a continuation line; drop
					// the empty chunk and attach the arrow to the
					// previous line instead.
					chunks = chunks[:last]
					arrowChunkIdx = len(chunks) - 1
				} else {
					arrowChunkIdx = last
				}
			}
		}
		contIndent := strings.Repeat(" ", labelPrefixW)

		for ci, chunk := range chunks {
			withArrow := ci == arrowChunkIdx
			if isCursor {
				if ci == 0 {
					cursorLine = len(lines)
				}
				var plain string
				if ci == 0 {
					plain = labelPrefix + chunk
				} else {
					plain = contIndent + chunk
				}
				if withArrow {
					plain += " →"
				}
				// Pad so the cursor background spans the full row
				// width on every wrapped line.
				if w := lipgloss.Width(plain); w < rowWidth {
					plain = plain + strings.Repeat(" ", rowWidth-w)
				}
				lines = append(lines, cursorRowStyle.Render(plain))
				continue
			}
			var row string
			if ci == 0 {
				row = "  " + labelStyle.Render(labelText) + " " + valueStyle.Render(chunk)
			} else {
				row = contIndent + valueStyle.Render(chunk)
			}
			if withArrow {
				row += " " + drillStyle.Render("→")
			}
			lines = append(lines, row)
		}
	}
	return lines, selectableIdxs, cursorLine
}

// nextSelectableCursor returns the next/prev cursor index that lands on a
// selectable entry (skipping section headers + non-drillable rows).
// dir=+1 → next, -1 → prev. Clamps at the ends of the list.
func nextSelectableCursor(entries []linkEntry, cursor, dir int) int {
	if len(entries) == 0 {
		return -1
	}
	for i := cursor + dir; i >= 0 && i < len(entries); i += dir {
		if entries[i].isSelectable() {
			return i
		}
	}
	return cursor
}

// firstSelectableCursor returns the first selectable entry index, or -1 if
// the list has no selectable entries.
func firstSelectableCursor(entries []linkEntry) int {
	for i, e := range entries {
		if e.isSelectable() {
			return i
		}
	}
	return -1
}
