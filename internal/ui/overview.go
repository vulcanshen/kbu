package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// overviewEntry is one row of the Overview tab. Three flavours:
//   - section: header row (just a label, no value, not selectable)
//   - info:    label + value, no drill (selectable but Enter is a no-op)
//   - drill:   label + value with a ref (Enter fetches & opens YamlPopup)
type overviewEntry struct {
	label   string
	value   string
	ref     *k8s.RefTarget
	section bool
}

// isSelectable reports whether the cursor can land on this entry.
// Section headers are visual-only.
func (e overviewEntry) isSelectable() bool { return !e.section }

// buildPodOverviewEntries renders the Pod-specific Overview entry list using
// the parsed PodOverviewData stashed in ResourceDetail. Falls back to the
// generic builder if PodOverview is nil (e.g. detail still loading).
func buildPodOverviewEntries(detail k8s.ResourceDetail) []overviewEntry {
	if detail.PodOverview == nil {
		return buildGenericOverviewEntries(detail)
	}
	po := detail.PodOverview
	var entries []overviewEntry

	if po.Owner != nil {
		entries = append(entries, overviewEntry{
			label: "Owner",
			value: ownerDisplay(*po.Owner),
			ref:   po.Owner,
		})
	}
	if po.Node != nil {
		entries = append(entries, overviewEntry{
			label: "Node",
			value: po.Node.Name,
			ref:   po.Node,
		})
	}
	if po.ServiceAccount != nil {
		entries = append(entries, overviewEntry{
			label: "ServiceAccount",
			value: po.ServiceAccount.Name,
			ref:   po.ServiceAccount,
		})
	}

	if len(po.Volumes) > 0 {
		entries = append(entries, overviewEntry{section: true, label: "Volumes"})
		for _, v := range po.Volumes {
			e := overviewEntry{label: "  " + v.Name}
			if v.Ref != nil {
				// Drillable: show "kind/name" so the user sees which resource
				// they'll drill into. Visual:  config-volume   configMap/nginx-config →
				e.value = v.Kind + "/" + v.Ref.Name
				e.ref = v.Ref
			} else {
				// Informational: emptyDir / hostPath / projected / ...
				e.value = "(" + v.Kind + ")"
			}
			entries = append(entries, e)
		}
	}

	if len(po.InitImages) > 0 || len(po.Images) > 0 {
		entries = append(entries, overviewEntry{section: true, label: "Images"})
		for _, img := range po.InitImages {
			entries = append(entries, overviewEntry{label: "  init", value: img})
		}
		for _, img := range po.Images {
			entries = append(entries, overviewEntry{label: "  app", value: img})
		}
	}

	entries = append(entries, labelsAnnotationsEntries(detail)...)
	return entries
}

// buildGenericOverviewEntries is the fallback Overview content used for any
// resource kind that doesn't have a custom Overview builder yet. It surfaces
// the structured fields the resource-specific detailer already populates
// (Strategy, Replicas, etc.) plus labels and annotations.
func buildGenericOverviewEntries(detail k8s.ResourceDetail) []overviewEntry {
	var entries []overviewEntry
	if detail.Name == "" && len(detail.Labels) == 0 && len(detail.Annotations) == 0 && len(detail.Fields) == 0 {
		return entries
	}
	if detail.Name != "" {
		entries = append(entries, overviewEntry{label: "Name", value: detail.Name})
	}
	if detail.Namespace != "" {
		entries = append(entries, overviewEntry{label: "Namespace", value: detail.Namespace})
	}
	if detail.Kind != "" {
		entries = append(entries, overviewEntry{label: "Kind", value: detail.Kind})
	}
	if detail.CreatedAt != "" {
		entries = append(entries, overviewEntry{label: "Created", value: detail.CreatedAt})
	}
	for _, f := range detail.Fields {
		entries = append(entries, overviewEntry{label: strings.TrimSuffix(f.Label, ":"), value: f.Value})
	}
	entries = append(entries, labelsAnnotationsEntries(detail)...)
	return entries
}

func labelsAnnotationsEntries(detail k8s.ResourceDetail) []overviewEntry {
	var entries []overviewEntry
	if len(detail.Labels) > 0 {
		entries = append(entries, overviewEntry{section: true, label: "Labels"})
		for _, k := range sortedKeys(detail.Labels) {
			entries = append(entries, overviewEntry{label: "  " + k, value: detail.Labels[k]})
		}
	}
	if len(detail.Annotations) > 0 {
		entries = append(entries, overviewEntry{section: true, label: "Annotations"})
		for _, k := range sortedKeys(detail.Annotations) {
			entries = append(entries, overviewEntry{label: "  " + k, value: detail.Annotations[k]})
		}
	}
	return entries
}

func ownerDisplay(ref k8s.RefTarget) string {
	// Short kind label + name. Use the registry display name when available,
	// otherwise fall back to the raw type string.
	kind := string(ref.Type)
	if def := k8s.DefaultRegistry.Get(ref.Type); def != nil {
		kind = strings.TrimSuffix(def.DisplayName, "s")
	}
	return fmt.Sprintf("%s/%s", kind, ref.Name)
}

// renderOverviewEntries turns the entry list into display lines, applying
// styles and adding a cursor marker on `cursor`. Returns:
//   - lines:           rendered display lines
//   - selectableIdxs:  indices into `entries` that the cursor can land on
//   - cursorLine:      display-line index of the cursor row (-1 if none) —
//     used by the caller to auto-scroll the viewport so
//     the cursor stays visible
func renderOverviewEntries(entries []overviewEntry, cursor int, width int, t *theme.Theme) (lines []string, selectableIdxs []int, cursorLine int) {
	cursorLine = -1
	labelStyle := t.DetailLabelStyle()
	valueStyle := t.DetailValueStyle()
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Sidebar.CategoryFg)).Bold(true)
	drillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Sidebar.CategoryFg)).Bold(true)
	// Cursor row uses a full-row background highlight (same treatment as
	// panel-view selected rows and YAML-popup match cursor) — far more
	// visible than the previous ▸ marker, especially on dense panels.
	cursorRowStyle := t.TableSelectedRowStyle()

	const labelW = 18
	const indent = "  "

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
			// No blank separator above section headers — keeps Overview
			// compact (per UX feedback). The section's bold colored label
			// is enough visual break.
			lines = append(lines, indent+sectionStyle.Render(e.label))
			continue
		}
		isCursor := cursor >= 0 && cursor < len(entries) && cursor == i
		labelText := e.label
		if len(labelText) < labelW {
			labelText = labelText + strings.Repeat(" ", labelW-len(labelText))
		}
		if isCursor {
			cursorLine = len(lines)
			// Strip styles by re-building from plain text — lipgloss
			// Background composes poorly with the existing per-segment
			// foregrounds (resets leak default bg through the row).
			plain := "  " + labelText + " " + e.value
			if e.ref != nil {
				plain += " →"
			}
			lines = append(lines, cursorRowStyle.Width(rowWidth).Render(plain))
			continue
		}
		row := "  " + labelStyle.Render(labelText) + " " + valueStyle.Render(e.value)
		if e.ref != nil {
			row += " " + drillStyle.Render("→")
		}
		lines = append(lines, row)
	}
	return lines, selectableIdxs, cursorLine
}

// nextSelectableCursor returns the next/prev cursor index that lands on a
// selectable entry, skipping over section headers. dir=+1 → next, -1 → prev.
// Clamps at the ends of the list.
func nextSelectableCursor(entries []overviewEntry, cursor, dir int) int {
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
// the list contains only section headers.
func firstSelectableCursor(entries []overviewEntry) int {
	for i, e := range entries {
		if e.isSelectable() {
			return i
		}
	}
	return -1
}

// keep sortedKeys re-used (already defined in detail.go); ensure package compiles.
var _ = sort.Strings
