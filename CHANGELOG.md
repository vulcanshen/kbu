# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [v1.5.0] - 2026-05-26

The Helm release. A Helm release becomes km8's 27th resource type and
plugs into the same Relatives / drill / breadcrumb / Y popup machinery
every other resource uses — the divergence is that the fetcher shells
out to `helm` instead of going through client-go. Registered at startup
only when `helm` is on `PATH`; otherwise the entire `Helm > Releases`
sidebar category never renders and an app-log INFO surfaces.

Beyond Helm, this release polishes search semantics across all three
panels (only the source panel's filter clears on focus leave; cursors
restore to last selection) and removes the panel-3 search entirely
(cursor-driven tabs didn't tolerate filter; "find a string in logs"
goes via `Y` + your editor).

### Added

- **Helm Releases category — `Helm > Releases` in the sidebar.** Lists
  every release in the cluster via `helm list -o json`, polled every 3s
  (no Helm watch API; the poller fakes a `watch.Modified` event into
  the existing watcher loop so external `helm install` / `upgrade`
  surfaces within seconds without busy-spinning the CLI). Columns:
  `NAME / NAMESPACE / CHART / APP VER / REV / STATUS / UPDATED`. Follows
  the current namespace selector — `helm list -n <ns>` when a ns is
  picked, `-A` otherwise.
- **Helm doc menu — `Space` on a Release row.** Pops a 5-item picker:
  `Manifest` (rendered chart), `Creator Notes` (post-install
  NOTES.txt), `User Values` (user-supplied), `Merged Values` (incl.
  chart defaults), `Hooks` (install/upgrade hook resources).
  `Enter`/`Space` fires the corresponding `helm get ...` asynchronously
  (10s timeout) and routes the stdout into the YAML popup. The menu
  stays open behind the YAML so consecutive picks flow without
  re-opening — input routing checks `yamlPopup` first while open, the
  menu sits idle underneath, then takes input back when YAML closes.
  `Esc`/`q` dismisses the menu.
- **Deployed Resources section in Release Relatives.** `helm get
  manifest` parsed into per-document `{kind, name, namespace}` tuples;
  each native K8s ref becomes a drillable RelativeRow under a
  `Deployed Resources (N)` section. Drill / `Space` / `Y` all work
  exactly as on any other Relatives row, so Release → Deployment →
  Pod → ConfigMap is a continuous chain. CRD kinds the registry
  doesn't recognize are dropped silently — every visible row stays
  drillable. The chain[0] entry is the Release itself, so the
  breadcrumb popup shows `Release/foo → Deployment/foo → Pod/foo-...`.
- **History tab — Panel 3, Helm releases only.** Replaces Events for
  releases (a release isn't a K8s object; kubectl events don't apply —
  drill into a deployed resource if you want events). Table view:
  `REV / STATUS / DATE / CHART / DESCRIPTION` from `helm history`. The
  current deployed revision is marked with a `●` glyph. `j`/`k`/`g`/`G`
  move the revision cursor; the cursor auto-lands on the deployed rev
  the first time the tab loads.
- **Rollback — `Space` on a History row.** On any non-current revision,
  `Space` pops a confirm popup whose `detail` row prints the exact
  `helm rollback <rel> <rev> -n <ns>` that will run. `Enter` runs it
  asynchronously (30s timeout, `CombinedOutput` so stderr surfaces);
  success fires a toast `Rolled back to rev N` plus an app-log info
  line, failure routes to app-log error with helm's stderr. On the
  current row, `Space` is a silent no-op (no surprise re-deploy of the
  state you're already on).
- **Rule A — helm-managed read-only guard.** Pressing `e` on any
  resource carrying `app.kubernetes.io/managed-by: Helm` (label) or
  `meta.helm.sh/release-name` (annotation) — or on a Release row
  itself — surfaces a "Helm-managed (read-only) — use helm upgrade /
  rollback" toast instead of opening `kubectl edit`. Stops users from
  editing fields the next helm reconcile would overwrite.
- **Helm storage secret filter — `.` on the Secrets panel.** The
  per-revision `sh.helm.release.v1.*` Secrets that helm uses for
  release storage are hidden from the Secrets list by default — they
  dominate the list otherwise. `.` on the Secrets table flips
  visibility; a `.helm` chip in the panel-2 bottom-left border
  confirms when the filter is OFF (secrets shown). Enricher lookups
  bypass the filter so SA → token-Secret links still work regardless.

### Changed

- **Confirm popup also dismisses on `Space`** (same as `Esc` / `n` /
  `q`). The same key that opens the confirm (Relatives-tab space-jump,
  History-tab rollback) re-pressed by reflex now cancels rather than
  re-fires.
- **Search clears only on the source panel when focus moves away** —
  the panel you're leaving loses its filter, every other panel keeps
  whatever filter it had. Sidebar / Table both restore the cursor to
  `selected` after dropping the filter so the unfocus highlight lands
  on the last picked item, not on whichever row the filtered index
  happened to point at.
- **`.helm` marker moved to panel-2 bottom-left border.** Earlier
  iteration during 1.5 development surfaced helm-secret filter state
  as `.hidden` in the status bar. Final form uses the unused
  bottom-left corner of the affected panel + an unambiguous `.helm`
  label, since the marker only matters while looking at the Secrets
  list.
- **Hidden KM8erm chip relabeled `KM8erm`** (was `km8erm` lowercase)
  to match the popup border title.
- **Breadcrumb + helm doc menu popups grew one row of top/bottom
  padding** so title/hint don't sit flush against the first/last
  content row.

### Removed

- **Panel 3 search.** The `/` hotkey on the detail panel and all
  associated filter rendering are gone. Cursor-driven tabs (Relatives,
  History) don't tolerate row filtering — the cursor index becomes
  meaningless once rows are hidden. Logs follow-tail breaks under
  filter (new lines that don't match silently vanish). Events are
  short enough to scroll. For "find this string in logs", press `Y`
  to copy the content and grep / search in your editor.

### Fixed

- **Helm watcher busy-spin.** The first cut returned a permanently-
  closed `watch.Interface` for releases, which made the watcher's
  outer loop reconnect-and-re-list as fast as the CPU could go — a
  single km8 sitting on the Releases panel would have pegged the
  helm CLI. Replaced with a polling `watch.Interface` that fires
  one `watch.Modified` event per interval and properly blocks
  between ticks.
- **History tab cursor lit only on focus.** The cursor row picked
  the focused/unfocused style at `buildContentLines` time but
  `SetFocused` only rebuilt for the Relatives tab. Switching focus
  to a panel-3 History view left the cursor in unfocused-dim style
  until the next 3s poll forced a rebuild. Fixed by including
  History in `SetFocused`'s rebuild list.
- **History cursor stuck across releases.** Cursor-position state
  travelled across `panel 2` row changes; switching from a release
  with 5 revs to one with 2 left the cursor on a now-invisible
  index. `SetDetail` now resets `historyCursor` when the underlying
  UID changes (panel 2 row swap) but preserves it when the same UID
  re-arrives via polling refresh (so user-typed `j`/`k` survives).
- **Sidebar search list "1 of 1" but empty.** `resetCursorToFirstMatch`
  set `m.cursor` to the first matching index but never reset
  `scrollOffset` — a stale offset from the previous wider list
  could push the only match off the visible window. Now resets to
  0 and `ensureCursorVisible`s after.
- **Table search filter survived focus leave.** Earlier ClearSearch
  cleared the flags but didn't recompute `m.rows` from `m.allRows`,
  so the panel could appear filtered after the search box was gone.
  Now mirrors the in-panel Esc path: convert filtered cursor to its
  unfiltered position, drop the filter, restore the cursor.
- **Sidebar focus-leave parked the cursor on a stale row.** After
  ClearSearch the filtered cursor index pointed at an unrelated
  row in the now-larger visible list. ClearSearch now calls
  `SetSelected(m.selected)` to put the cursor back on whatever the
  user actually picked.
- **Long Relatives value wrap lost arrow color.** A
  `harbor-registry-htpasswd ↘` value that wrapped onto two lines
  rendered the arrow as plain text on row 2 — `wrapPlain` trimmed
  the leading space before `↘`, so the suffix-match that decided
  which chunk owned the arrow style missed. Now the arrow is
  stripped before wrapping and re-appended (styled) to the last
  chunk, with reserved width in the wrap budget.

## [v1.4.0] - 2026-05-25

The Relatives release. The graph navigation tab that v1.3.0 named "Links"
gets the right name (Relatives — it's about what's related to this
resource, not what this resource points at) and the right hotkey
vocabulary to round it out: `Space` jumps the table cursor to whatever
ref you're highlighting, in either the Relatives tab or the breadcrumb
popup. Pod tab order swaps so Relatives is first — when you space-jump
to a Pod you land where you came from instead of being teleported to
Logs. ServiceAccount and Secret grow bidirectional links (RBAC
subjects + token-secret annotations). Selection styling gains a focused
vs unfocused distinction so you can always see which panel "remembers"
the cursor. Two KM8erm/drill bug fixes from v1.3.0 hotfixes promoted in.

### Added

- **`Space` — jump panels 1+2 to a related resource.** On the Relatives
  tab, pressing `Space` while the cursor is on a drillable ref pops a
  confirm popup; pressing `y` / `Enter` switches the sidebar selection
  and the table row to that resource (drill chain reset, panel 3
  rescoped). Works at any depth, on any drillable entry — including
  nested rows like `Volumes / config / configMap/harbor-core`. No
  round-trip through `Enter` to drill first. From inside the breadcrumb
  popup, `Space` does the same thing for the cursor-selected level;
  the confirm popup stacks visually above the breadcrumb so cancelling
  returns to the breadcrumb instead of dismissing it. (`Y` is still
  cursor-aware YAML preview; the two hotkeys complement.)
- **ServiceAccount ↔ Secret bidirectional links.** SA Relatives now
  carry three new sections: `RoleBindings (N)` (namespace-scoped
  bindings naming this SA as a subject), `ClusterRoleBindings (N)`
  (cluster-wide same), and `Token Secrets (N)` (Secrets whose
  `kubernetes.io/service-account.name` annotation references this SA —
  catches legacy auto-created token Secrets that aren't in `sa.Secrets`).
  Secret Relatives now show a `ServiceAccount` section back when that
  annotation is set, completing the round trip. RBAC subject queries
  are how you'd actually debug "why can / can't this SA do X" — they
  needed a first-class surface, not a guess.
- **Focus-shift hotkeys (`l` / `Enter`).** Sidebar `l` and `Enter` used
  to re-fire `ResourceSelectedMsg` for the cursor row, duplicating what
  `j`/`k` already auto-emitted. They now shift focus to panel 2 — the
  natural "I've picked the resource, now show me the rows" motion.
  Table `Enter` on rows without drill capability (Deployments without
  child config, ConfigMaps, etc.) likewise shifts focus to panel 3
  instead of being a silent no-op. Resources that DO drill (Pods →
  containers, HPAs → workloads) keep their drill semantics. Status line
  drops the "Enter drill" hint — focus-shift is the obvious
  adjacent-panel motion and not worth a slot.
- **Panel-aware selection styling.** Focused panel cursor: reverse-
  video — Catppuccin subtext1 (`#bac2de`) bg + base (`#1e1e2e`) fg +
  bold. Unfocused panel selected: softer bg (`#353648`, between surface0
  and surface1) + text fg + bold. So the panel you're driving and the
  panel that "remembers" your selection are both visible at a glance.
  Pod STATUS column gets a Catppuccin Latte (darker) palette variant
  when the row has the focused light bg — Mocha pastel `#a6e3a1` washes
  out on cream, Latte `#40a02b` reads cleanly.

### Changed

- **Detail tab renamed: Links → Relatives.** The new name describes
  the relationship, not the implementation. The tab title at depth ≥ 2
  shows `Relatives N`. Internal Go identifiers also renamed
  (`LinkSection` → `RelativeSection`, `EnrichLinks` → `EnrichRelatives`,
  `internal/ui/links.go` → `relatives.go`, etc.) so the source vocabulary
  matches the UI. Pure mechanical rename, no behavior change.
- **Pod / Deployment tab order: Relatives first.** Was
  `[Logs, Relatives, Events]`, now `[Relatives, Logs, Events]`. Space-
  jumping to a Pod lands on Relatives — same tab the user came from, no
  visual whiplash. Logs is one `]` away.
- **Nested Relatives rows wrap to two lines.** Section children
  (`Volumes / config / configMap/foo`) used to render as a single row
  `alias  configMap/very-long-name `, which truncated badly on narrow
  terminals. Now: alias on one line, indented `resourceKind/name `
  on the next. Top-level entries (Owner / Node / ServiceAccount) keep
  the single-line layout — short relationship words don't benefit from
  splitting.
- **Glyph vocabulary unified.** The Relatives drill arrow `→` becomes
   (Nerd Font chain glyph). Breadcrumb middle rows carry the same
  glyph; the bottom row keeps its `●` you-are-here dot. Three surfaces
  (drill arrow, breadcrumb middle, breadcrumb current) now read as
  consistent vocabulary.
- **Search filters clear on `Space`-switch.** Stale sidebar / table /
  detail filters from before the switch can't hide the freshly
  selected resource anymore.

### Fixed

- **KM8erm: Alt+letter / Shift+Tab / Ctrl-arrows / F-keys forwarded
  to the embedded shell.** `ptyKeyBytes` was dropping these — zsh
  hotkeys like `Alt+.` / `Alt+f` / `Alt+Backspace`, Shift+Tab reverse
  completion, Ctrl+Left/Right word jump, and F1–F12 all silently
  no-op'd inside KM8erm. Now they serialize to the right escape
  sequences (meta convention ESC prefix for Alt, xterm CSI for
  modified arrows, DEC SS3 / CSI `~` for F-keys).
- **Drill chain survives background watcher refresh.** While drilled
  into a deeper Relatives level, the watcher's periodic
  `ResourceDataMsg` would re-fire `fetchResourceDetail` for the
  still-selected root row; the result's `SetDetail` wiped the drill
  stack and snapped the user back to level 1 just as their fetch
  finished. `SetDetail` no longer touches the drill stack; the
  row-change path resets it explicitly, namespace/context switches go
  through `ClearDetail` (which still resets).
- **Selected-row highlight spans full row width.** A long-standing
  rendering bug where the Pod STATUS column's inner ANSI reset killed
  the row style for every column after it, leaving Restarts / Age /
  Node uncolored on the selected row. Per-cell style application
  (separator + trailing pad row-styled too) fixes it; the row
  highlight now reaches the right edge.
- **Detail panel cursor row honors focus state.** Was always
  rendering with `TableSelectedRowStyle` regardless of panel focus,
  so the unfocused-panel softer style only ever applied to the table.
  Now the Relatives cursor row picks the unfocused style when panel 3
  isn't focused; `SetFocused` rebuilds content lines on focus change
  so the highlight refreshes immediately.

### Internal

- New AppModel messages: `RelativePushMsg`, `RelativeDrillMsg`,
  `RelativeBreadcrumbMsg`, `RelativeJumpMsg`, `relativeDrillFetchedMsg`,
  `FocusTableMsg`, `FocusDetailMsg`, `SwitchToResourceMsg`,
  `RequestSwitchToResourceMsg`.
- New helpers: `SidebarModel.ClearSearch`, `SidebarModel.SetSelected`,
  `TableModel.SetCursor`, `DetailModel.ClearSearch`,
  `DetailModel.CurrentLevelRef`, `AppModel.honorPendingTableSelect`.
- New k8s enrichers: `enrichServiceAccountBindings`,
  `enrichServiceAccountTokenSecrets`, `enrichSecretServiceAccount`.
- New theme fields: `Sidebar.UnfocusedSelectedBg/Fg`,
  `Table.UnfocusedSelectedRowBg/Fg`.

## [v1.3.0] - 2026-05-24

The big one. km8 becomes a graph navigator — the Links tab lets you
chase ownership / consumer / ref chains by repeatedly drilling
(Deployment → Pods → ConfigMap → consumer Pods → ...) without ever
leaving panel 3. 25 of 26 resource kinds carry Links data; every drill
respects a cycle pre-check; a breadcrumb popup lets you jump back to
any ancestor level in one step. Alongside that: a persistent embedded
shell (KM8erm), aggregate Deployment logs, a full-screen `Y` YAML
popup, and a layout refactor that ditched percentage-math heuristics
for absolute stacking.

### Added

- **Links tab — Lens-style graph navigation.** Every detail panel
  (except Namespaces, which has no meaningful refs) carries a Links
  tab listing the resource's navigable references. `Enter` / `l`
  drills into a ref — the panel re-renders showing *that* resource's
  Links, building a navigation chain (Deployment → Pod → ConfigMap →
  consumer Pods, ...). `h` / `Esc` pops one level. `b` opens a
  breadcrumb popup listing the full chain so you can jump back to any
  ancestor in one step (`j` / `k` to pick, `Enter` to commit). `Y` on
  the cursor-pointed entry opens its YAML popup. The tab label
  surfaces depth as `Links ↳N` and the panel border carries a
  `[b]readcrumbs` hint at the top-right whenever you're deeper than
  the root. Cycle detection (`kind+ns+name`) blocks revisiting an
  ancestor; fetch failures show a peach `ShowWarn` toast and don't
  push a frame. Stale-drop guards (source item UID) keep async fetch
  results from clobbering the panel when you've moved on to a
  different row.
- **Links coverage for 25 of 26 resource kinds.** Pods / Services /
  Deployments / StatefulSets / DaemonSets / Jobs / CronJobs / Ingresses
  / HPAs / PVCs each surface their kind-specific refs (owners,
  selected pods, scaleTargetRef, claimRef, ...). ConfigMaps / Secrets
  / ServiceAccounts / PVs surface *reverse* refs (which pods mount me
  / use me as their SA / are bound to me). Nodes /
  PodDisruptionBudgets / NetworkPolicies / EndpointSlices / Roles /
  RoleBindings / ClusterRoles / ClusterRoleBindings / StorageClasses /
  IngressClasses all wired. Namespace hides the Links tab entirely
  (no concrete drill target).
- **Aggregate Logs for Deployments.** Selecting a Deployment row
  streams logs from every pod in its current ReplicaSet into one Logs
  tab (also Deployment's default tab — "which pod is misbehaving
  during a rollout" is the question that opens 90% of Deployment
  details). Lines are prefixed `<pod-hash>│<container>│<text>` with
  three independent FNV-derived colors from the 8-entry Catppuccin
  palette so any pod / container combination stays visually distinct.
  Cross-stream timestamp sorting deliberately not attempted (clock
  skew + jitter would make any ordering misleading). Falls back to
  the Deployment's full selector when the current-ReplicaSet lookup
  fails (RBAC denies RS list, etc.).
- **Persistent KM8erm (`Alt+t`).** The embedded shell survives
  visibility toggling. First `Alt+t` spawns it; subsequent presses
  hide / show while cwd, history, env vars, and background jobs all
  persist. Status bar carries a chip in the `ns:` row showing state —
  green `attached` while visible, peach `km8erm` while hidden. Shell
  exits cleanly on km8 quit. `Alt+t` only applies to the Shell-kind
  PTY; `kubectl edit` and `kubectl exec` popups treat it as a regular
  key (their lifecycle is bound to the subprocess). `e` / `s` while
  any PTY is alive refuse with a `ShowWarn` toast instead of
  clobbering the in-flight subprocess.
- **`Y` YAML popup.** Full-screen popup of the currently-selected
  resource's YAML with `j` / `k` line scroll, `u` / `d` half-page,
  `gg` / `G` top / bottom, `/` search (`Enter` commits; `n` / `N`
  step through matches with full-row highlight; search-box border
  flips cyan → amber when the filter locks), `e` to dispatch
  `kubectl edit` directly from the popup (skips the table-level
  confirm), and `y` to OSC-52-copy the full YAML to your clipboard.
  Solves the "YAML wall in narrow Panel 3 is hard to read" friction
  without dropping YAML access. On the Links tab, `Y` follows the
  cursor — opens the YAML of the link entry you're pointing at, so
  previewing a drill target's YAML doesn't require drilling into it
  first.
- **App Log `y` to copy.** Press `y` inside the App Log popup (`!`)
  to OSC-52-copy the full log (newest-first, matching display order).
  Makes "paste the error into Slack / GitHub issue" one key away.
- **Toast levels — `Show` / `ShowWarn`.** Info-level (`Show`) stays
  1s sky-blue (Copied!, PTY hints); warning-level (`ShowWarn`) is 2s
  peach with a warning glyph (`󰀦`) for cycle-blocked / drill-failed
  messages. Longer duration means you actually get to read what
  blocked. `ShowError` reserved for when the first error caller
  appears.
- **Per-popup distinct icons.** Each popup (toast, confirm, help, app
  log, context picker, namespace picker, YAML popup, breadcrumb,
  PTY view) gets its own Nerd Font glyph in the title.
- **`N` / `C` uppercase aliases for namespace / context pickers.**
  Lowercase still works but feels too easy to misfire (`n` is
  vim-search-next muscle memory). Lowercase will be deprecated later.
- **Sidebar category-name search.** Typing `/` followed by a category
  name (`cluster`) expands matching categories and shows all their
  children, not only items whose own label matches.
- **Detail panel refetch spinner.** Panel 3 border shows an animated
  braille spinner while `fetchResourceDetail` is in flight.

### Changed

- **Detail tab order: YAML moved out, Links is the default tab.**
  YAML lives in the `Y` popup now. New defaults:
  - Pod: `Logs` / `Links` / `Events`
  - Deployment: `Logs` / `Links` / `Events`
  - Events: `Links` alone
  - everything else: `Links` / `Events`

  Existing users who pressed `1`/`2`/`3` to cycle to a YAML tab — use
  `Y` instead.
- **`h` / `l` no longer switches detail tabs from inside Panel 3.**
  On the Links tab those keys belong to the drill chain (push / pop),
  and dual-purposing them was confusing. To switch detail tabs while
  reading panel 3, move focus to panel 2 first. From Panel 2 `h` /
  `l` still cycle tabs as before.
- **Tab label format.** Drilled-into Links tab shows `Links ↳N` (was
  `Links(N)`); the down-arrow reads as "you've gone N levels deep" at
  a glance.
- **Panel layout uses absolute stacking math.** Replaced percentage
  heuristics (`*N/100`) with named constants (`panelSidebarWidth =
  24`, `panelDetailHeight = 14`, ...) and pure subtraction. Side
  benefit: predictable behavior on any terminal width. Panel 1
  narrowed 28 → 24. Panel 2 ↔ Panel 3 vertical space dropped to 0
  (borders themselves act as the separator). Sidebar ↔ Table
  horizontal space also dropped to 0.
- **Status line is fixed 1 row.** Removed the dynamic two-row mode.
  Hints are condensed (`?`, `q`, panel-specific keys, `Y`, `M-t`) —
  no more vim-convention reminders, no overflow.
- **YAML popup spans full terminal width.** Was sized to a percentage
  of the screen; now matches the panel-border alignment. Same for
  the help popup.
- **Help popup is two-column.** Counts wrap rows per group to balance
  the columns; padding distributes across inter-section gaps so the
  columns terminate at the same height.

### Fixed

- **Panic on quit when KM8erm was hidden.** `Stop()` nil'd `p.cmd`
  while `readLoop` was still doing `cmd.Wait()`; the loop now
  captures local pointer copies before the wait so the nil
  reassignment can't race the in-flight wait.
- **Pod STATUS column lost its color when truncated.** A
  `CrashLoopBackOff` clipped to `CrashL…` no longer matched the color
  lookup switch — color logic now reads the pre-truncation value
  while the renderer keeps the clipped string.
- **Pod owner drill resolves past the ReplicaSet layer.** A Pod's
  `OwnerReferences[0]` is the auto-created ReplicaSet, but
  `kindToResourceType` mapped it to `Deployments`. The Name was the
  RS's (`<deployment>-<hash>`), so drilling into Owner failed with
  `deployments.apps "..." not found`. `EnrichLinks` now looks up the
  RS to find its owning Deployment and rewrites `PodLinks.Owner` in
  place. Also fixes cycle detection for the Deployment → Pod → Owner
  round trip.
- **Stale `ResourceDetailMsg` drops by UID.** Rapid row switching
  used to let a slow fetch overwrite the current row's detail after
  the user had moved on. `ResourceDetailMsg` now carries the source
  item UID; the handler ignores mismatches.
- **Help popup right border on odd-width terminals.** Off-by-one
  from integer-truncated column split — fixed by letting the middle
  gutter absorb the leftover column.
- **KM8erm hidden status-bar marker uses peach (`#fab387`).** The
  previous yellow was identical to the `ns:` text; the new color
  matches the panel-border palette and is unambiguous.
- **`Alt+t` hint everywhere is lowercase.** The keymap is
  case-sensitive; help / status line / KM8erm border hints now match
  the actual key.
- **Long Links values wrap consistently for cursor and non-cursor
  rows.** Cursor row used `lipgloss.Width()` (which wraps); non-
  cursor rows had no width constraint and got `ansi`-truncated by
  the outer panel — and the drill arrow disappeared from the
  truncated rows, hiding the fact that the row was drillable. Both
  branches now share an explicit `wrapPlain` path; the drill arrow
  `→` is split back off the last wrap chunk so its color stays in
  `drillStyle`.
- **Breadcrumb cursor row aligns with non-cursor rows.** The
  cursor's highlight wrapped both the prefix's leading space *and*
  an outer wrap-space, doubling it up; `2.` was shifted right by one
  cell. Now both render with a single leading space inside the same
  content frame.
- **Various popup margin and padding tightening.** Top/bottom
  padding rows dropped from the YAML popup, breadcrumb popup, and
  overview cursor — the borders alone provide enough visual
  separation.

### Internal

- New `k8s.LinkSection` / `k8s.LinkRow` generic Links payload on
  `ResourceDetail`; Pod and Service keep their typed `PodLinks` /
  `ServiceLinks` for richer per-kind structure. Per-kind builders
  live in `internal/k8s/links.go`; `EnrichLinks(ctx, cs, rt, item,
  *detail)` is the extension point AppModel calls after the
  synchronous `Detailer` returns — the place to put API-needing
  resolution (RS-skip, selector→pods, reverse refs).
- `k8s.FetchResourceByRef(ctx, cs, ref)` fetches any supported kind
  by `(Type, Name, Namespace)`, used by both the YAML popup drill
  (`Y`) and the chain drill (`Enter` / `l`). Supports 21 kinds.
- `DetailModel.drillStack []drillFrame` carries the Links navigation
  chain (level 2+); the root is implicit in `m.detail`. `Depth()`,
  `RootRef()`, `DrillChain()`, `PushDrillFrame`, `PopDrillFrame`,
  `JumpToDrillLevel`, `ResetDrillStack`, `BorderTopRightHint`,
  `CurrentLevelYAML` give AppModel + the breadcrumb popup the API
  surface they need.
- New `LinkPushMsg` / `linkDrillFetchedMsg` / `LinkBreadcrumbMsg` /
  `LinkJumpMsg` messages. The fetched message carries `sourceUID`
  for the same stale-drop guard `ResourceDetailMsg` uses.
- New `BreadcrumbPopupModel` (PopupAnimator-based, follows the
  ConfirmModel pattern): `j` / `k` move cursor, `Enter` jumps back
  to that level, `Esc` / `q` / `b` close. Long resource names wrap
  with continuation indented under the label start; the cursor
  highlight spans every wrapped line as one block.
- New `ToastModel` levels — `toastLevel` enum + per-level duration
  / glyph / color helpers.
- `k8s.PodTarget` + `k8s.PodsForDeployment` /
  `k8s.PodsForWorkload`. `LogStreamer.StartMulti([]PodTarget)` is
  the aggregate entry point; the single-pod `Start` is a thin
  wrapper. `LogLine.Pod` is populated only in aggregate mode, so
  single-pod streams stay free of the `<pod-hash>│` prefix.
- `YamlPopupModel` in `internal/ui/yamlpopup.go` mirrors
  `HelpModel` / `AppLogModel` structure. Captures edit target at
  `Open()` so `e` knows what to dispatch even after scroll.
- `PtyView` gains `hidden bool` + `kind PtyKind` (Shell / Edit /
  Exec). `IsActive()` means "alive AND visible"; `IsAlive()`
  reports the subprocess state. `Hide()` is a no-op for Edit / Exec
  (transient by design). `Show(w,h)` re-syncs PTY size on un-hide.
- Panel layout constants in one block at the top of `app.go`:
  `panelSidebarWidth`, `panelDetailHeight`, `panelHMargin`,
  `panelHSpace`, `panelVSpace`. `panelSizes()` is pure subtraction.
- `aggregateLogsReadyMsg` / `resourceFetchedForDrillMsg` /
  `linkDrillFetchedMsg` all carry the source item UID for
  stale-drop. AppModel's `currentItemUID()` helper centralizes the
  lookup.

### Known trade-offs

- **Cluster-wide Links enrichers** (ClusterRole bindings,
  StorageClass PVCs, IngressClass Ingresses) issue cluster-wide List
  calls. On large clusters this can push the Links tab populate time
  into multiple seconds. OrbStack-scale clusters are unaffected. If
  it matters in your environment, file an issue — the simplest fix
  is making these specific enrichers opt-in via config.
- **Bare ReplicaSets** (RS without a parent Deployment, rare in
  practice) still hit `not found` on Owner drill —
  `enrichPodOwner` has no Deployment to resolve to. Would need
  ReplicaSet as a first-class km8 resource to fix; not in scope
  here.

## [v1.2.0] - 2026-05-22

### Added
- **KM8erm — embedded shell terminal** (`T` key). Opens the user's login shell (`$SHELL -l`, fallback `/bin/sh`) inside a PtyView popup with the user's full env and cwd intact — essentially `ssh localhost` embedded in km8. The popup title shows the short hostname (`.local` mDNS suffix stripped) so it's clear which machine you're connected to. Solves the "I need to drop out of km8 to run `kubectl apply -f foo.yaml`" friction without re-implementing every kubectl verb inside the TUI.
- **PTY scrollback** — 10,000-line ring buffer captures every output line that flows through any PtyView popup (KM8erm, `s` shell exec, `e` edit). Navigate with `PgUp` / `PgDn` (page) and `Home` / `End` (top / live). Typing any other key snaps the view back to live. ANSI color codes are preserved so the rendered history looks exactly like the live output. Scrollback automatically resets when the subprocess clears the screen (`clear` / `\x1b[2J` / `\x1b[H\x1b[J` / `\x1b[3J` / `\x1bc`).
- **Per-container colored log labels** — multi-container pods are now visually distinguishable line-by-line. Each container name gets a stable color (FNV hash → 8-entry Catppuccin palette).
- **Colored Pod STATUS column** — green for `Running` / `Succeeded` / `Completed`, yellow for `Pending` / `ContainerCreating` / `PodInitializing`, red for `CrashLoopBackOff` / `ImagePullBackOff` / `OOMKilled` / `Evicted` / `Error` / `Failed` / `Init:<reason>`, gray for `Terminating` / `Unknown`.
- **Sidebar category-name search** — typing `/` followed by a category name (e.g. `cluster`) now expands the matching category and shows all its children, not only resource items whose own label contains the query.
- **Detail panel refetch spinner** — Panel 3 border title shows an animated braille spinner while `fetchResourceDetail` is in flight (after edits, on row select, etc.). Effectively invisible on fast clusters; useful on remote clusters with noticeable API round-trips.
- **Per-popup distinct icons** — toast, confirm, help, applog, context picker, namespace picker, and PtyView each get their own Nerd Font glyph in the title bar, replacing the single shared `󰵅` from v1.1.0.

### Changed
- **Pod STATUS column now shows the kubectl-equivalent reason** rather than the raw `Pod.Status.Phase`. A pod stuck in CrashLoopBackOff used to display `Running` (because Phase remains Running while containers fail); it now displays `CrashLoopBackOff`. Matches `kubectl get pods` exactly. New `k8s.PodStatus(p *corev1.Pod) string` helper.
- **Scrollback keys (`PgUp` / `PgDn` / `Home` / `End`) only intercept in non-alt-screen mode.** When a full-screen app like vim / nvim / less / htop switches to alt screen, these keys forward to the app as usual so paging inside the app keeps working. Plain shells (zsh / bash) don't bind these keys by default, so users with shell line-editing habits use the readline-native `Ctrl+A` / `Ctrl+E` for beginning / end of line — unchanged.

### Fixed
- **PTY scrollback CRLF handling** — macOS shells (and Windows) emit `\r\n` as line terminator. Previous `\r`-as-reset logic was clearing pending line content before `\n` could commit it, so scrollback was always empty. Now a `pendingCR` flag distinguishes `\r\n` (line terminator) from a lone `\r` (progress-bar in-place reset).
- **PTY scrollback ANSI-only filter** — zsh prompts emit dozens of pure-ANSI repaint sequences per command. These were being stored as empty scrollback entries that looked like "blank lines I scrolled into". Now `commitScrollbackLine` filters entries whose ANSI-stripped form is whitespace-only.
- **nvim shutdown lag in edit popup** — was caused by inherited terminal-program env vars (`TERM_PROGRAM`, `KITTY_*`, `ITERM_*`) telling nvim to probe for terminal-specific features that the embedded PTY can't answer. Editor subprocess env is now sanitized; `TERM=xterm-256color` is forced. Remaining lag is nvim's own LSP / plugin teardown (use `editor: nvim --noplugin` in config.yaml to skip).

### Internal
- New `terminalTitle()` + `buildShellTerminalCmd()` helpers in `internal/ui/app.go` for the `T` key flow.
- `PtyView` gains `scrollback []string`, `pendingLine *strings.Builder`, `pendingCR bool`, `scrollOffset int` fields and `captureToScrollback` / `commitScrollbackLine` / `scrollPage` / `scrollToEnd` methods.
- `DetailModel` gains `refetching` / `spinnerFrame` state + `BeginRefetch` / `SpinnerSuffix` / `advanceSpinner` methods and a `detailSpinnerTickMsg` routed unconditionally (focus-agnostic) so the spinner keeps ticking while the user is on Sidebar / Table panels.
- `AppModel.ptyView` switched from value to pointer (`*PtyView`) — the background readLoop writes to scrollback fields concurrently, and value-receiver copies of PtyView triggered race-detector hits in `tea.KeyMsg` / `tea.MouseMsg` paths.
- Sidebar `visibleItems` extended with `catMatch` short-circuit so category-name matches preserve all children.

## [v1.1.0] - 2026-05-22

### Added
- **Embedded PTY for `e` (edit) and `s` (shell exec)** — kubectl runs inside an in-app virtual terminal (`creack/pty` + `hinshun/vt10x`) rather than commandeering the host terminal via `tea.ExecProcess`. The popup is rendered as a centered overlay with a titled border (`╭─ 󰵅 Edit: deployment/nginx (default) ─╮`); 24-bit RGB colors, attributes, and the cursor are all preserved so editors like nvim render exactly as they would standalone. Closes the long-standing "kubectl edit leaves residue in scrollback after quitting km8" issue — subprocess output never touches the host terminal buffer.
- **Confirm popup before `e` (edit)** — pressing `e` now opens the same confirmation dialog used by `s` and `D`, showing the exact `kubectl edit <kind>/<name> -n <ns>` invocation. Prevents accidental edits and keeps the action key UX consistent across edit / shell / delete.
- **Confirm popup before `q` (quit)** — pressing `q` no longer exits immediately; it asks `Quit km8?` with Enter to confirm. `Ctrl+C` still bypasses confirmation for the emergency case.

### Changed
- **`e` (edit) now runs `kubectl edit` directly**, not `kubectl get -o yaml → temp file → editor → kubectl apply`. The patch strategy and validation behavior now match `kubectl edit` exactly; users no longer see apply-semantics surprises (e.g. server-side apply behavior diverging from strategic merge patch).
- **`s` (shell exec) replaced its `sh -c "clear; ...; clear"` wrapper** with a direct `kubectl exec -it ... -- /bin/sh` inside the PTY popup. Fixes Windows shell exec which was previously broken (no `sh` binary on Windows).
- **Popup title decoration unified** — confirm, namespaces, contexts, toast, and PTY overlay all use the same `popupGlyph` (Nerd Font 󰵅) inside the top border.

### Fixed
- **PTY popup symmetric centering** — fixed margin (1 row top/bottom, 2 cols left/right) so left/right and top/bottom margins are always equal regardless of host terminal size. Previous percentage-based sizing produced off-by-one asymmetry on odd-width terminals.
- **PTY popup border alignment** — top border `╮` was rendered off-screen because `len("──")` returns byte count (6 for UTF-8 box-drawing chars) instead of visual width (2). Now uses an explicit visual-width constant.

### Internal
- New `internal/ui/ptyview.go` — embedded VT100/xterm terminal renderer with key forwarding, SIGWINCH-aware resize, and exit detection.
- New dependencies: `github.com/creack/pty v1.1.24`, `github.com/hinshun/vt10x v0.0.0-20220301184237-5011da428d02`.
- Editor subprocess environment is sanitized — strips `TERM_PROGRAM`, `KITTY_*`, `ITERM_*`, `LC_TERMINAL`, `WEZTERM_*`, `GHOSTTY_*`, `COLORTERM` and forces `TERM=xterm-256color`. nvim was probing for terminal-program features and timing out on exit when the embedded PTY did not respond; the strip eliminates the wait.
- Cell rendering fast-path — default-styled cells skip lipgloss style allocation entirely, cutting ~30k Render calls per second on a typical 80×24 popup at 20 ticks/s.
- vt10x 24-bit RGB colors (encoded as `r<<16|g<<8|b`) are now correctly translated to lipgloss `#RRGGBB` instead of being emitted as uninterpretable integer strings (which made nvim render entirely white).
- Removed: `editResource`, `editTempReadyMsg`, `editEditorDoneMsg`, `editApplyFailedMsg`, `editEditorCrashedMsg`, `editFetchFailedMsg`, `EditDoneMsg`, `successNoticeClearMsg`, `resolveEditor` — replaced by the PTY pipeline.

## [v1.0.10] - 2026-05-21

### Changed
- **Splash easter egg now shows the build version** instead of the `Hi! It's KubeMate.` tagline. Tagged releases display `v1.0.10`, local `go build` output displays `dev`. Quick way to confirm what release is running without quitting + `km8 --version`.
- **`--version` output prefixes tagged releases with `v`** (`km8 v1.0.10`); `dev` builds remain `km8 dev`. The `v` is added by `version.Display()` so the on-disk constant matches the goreleaser convention of stripping the tag's `v` prefix.

### Internal
- Build version moved from `cmd/main.go` to a new `internal/version` package so both `cmd/` and `internal/ui/splash.go` can read it without import cycles. goreleaser ldflags target updated to `github.com/vulcanshen/km8/internal/version.Version`.

## [v1.0.9] - 2026-05-21

### Added
- **9 new built-in resource types** (17 → 26 total):
  - **Storage** (new category): `PersistentVolumes` (cluster-scoped), `PersistentVolumeClaims`, `StorageClasses` (cluster-scoped)
  - **Autoscaling** (new category): `HorizontalPodAutoscalers` (autoscaling/v2), `PodDisruptionBudgets`
  - **Network**: `NetworkPolicies`, `EndpointSlices` (replaces legacy `Endpoints` — K8s 1.21+ primary type), `IngressClasses` (cluster-scoped)
  - **RBAC**: `ServiceAccounts` (completes the Pod → SA → RoleBinding lookup chain)
- **Three new drill-down chains**:
  - **PVC → mounting Pods** — filters pods by `spec.volumes[].persistentVolumeClaim.claimName`
  - **HPA → target workload** — resolves `spec.scaleTargetRef` to Deployment / StatefulSet / DaemonSet; child type adapts per HPA target kind
  - **PDB → protected Pods** — pods matching `spec.selector`
- **Sidebar label truncation with ellipsis** — long names like `PersistentVolumeClaims` and `HorizontalPodAutoscalers` clip with `…` instead of overflowing the panel. Full name is recoverable from panel 2 border title on selection. Clipboard copy (`y`) keeps untruncated labels for paste.

### Changed
- **Drill-down child type is now resolved per item** (internal): `DrillDownConfig.ChildType` (fixed field) replaced with `ChildTypeFor` (resolver function). Existing drill-downs use a `StaticChildType(t)` wrapper — no behavior change. HPA's resolver returns the actual target kind (e.g. `ResourceDeployments` when targeting a Deployment, `ResourceStatefulSets` when targeting a StatefulSet) so the drilled-into table uses the correct column schema.
- **HPA drill-down now goes to the target workload, not its pods.** Pressing Enter on an HPA lists the Deployment / StatefulSet / DaemonSet it controls (as a single-row list); a second Enter then descends into that workload's pods. The previous behavior jumped straight to leaf pods via the target's selector — pragmatic but lossy and inconsistent with how every other "follow the reference" drill-down works (Ingress → Service, RoleBinding → Role).

### Internal
- Dropped unused `ResourceType.ChildResourceType()` method (no callers).

## [v1.0.8] - 2026-05-21

### Added
- **Log follow-tail**: the Logs tab now sticks to the bottom as new lines arrive — no need to press `G` to keep up with `kubectl logs -f`-style output. A `▼` marker in Panel 3's title (`[3] Logs ▼`) shows when follow is active.
- **Auto-pause on scroll up**: pressing `k` / `↑` / `u` / `gg` / mouse-wheel-up while on the Logs tab pauses the auto-scroll so you can read history without being yanked back. `G` jumps to the bottom AND resumes follow in one motion. `kj` suffices to pause in place — no dedicated toggle key needed.

## [v1.0.7] - 2026-05-20

### Added
- **Detail tab is now YAML** (renamed from "Detail"). Renders the resource's serialized YAML — equivalent to `kubectl get -o yaml` but sourced from the informer cache so display is instant. `apiVersion` / `kind` are restored on the deep-copied object before marshaling (client-go strips TypeMeta from cached typed objects); `managedFields` are stripped.
- **Container drill-down shows YAML too**: extracts `spec.containers[i]` and `status.containerStatuses[i]` (or init equivalents) into a `{spec, status}` document.
- **YAML syntax highlighting**: keys, list dashes, comments, and `---` separators are colored per line; safe round-trip (strip ANSI = original text).
- **Clipboard copy (`y`)**: global key copies the focused panel's content via OSC 52 (works through tmux/SSH, no `xclip`/`pbcopy` needed). Sidebar copies the navigation tree, Table copies header + filtered rows, Detail/YAML copies raw unwrapped YAML so it is paste-ready. A bordered toast popup (`Copied!`) confirms for 1 s. New `y copy` hint added to the status line.
- **Search (`/`) in namespace and context popups**: type to filter; `Enter` releases input focus (filter kept) so `j/k` can navigate; `Esc` clears filter; `Backspace` deletes a character. Empty result shows `(no matches)`.
- **Content reflow on panel resize**: expanding (`=`) and restoring (`-`) the panel now re-wraps Detail/YAML, Events, and Logs to the new width. Logs are stored as raw `(container, text)` pairs and wrapped at render time.

### Changed
- **Long values now wrap instead of truncating with `…`** — applies to Detail tab values, Events message column, Log lines, and YAML lines. Continuation lines indent to the value column.
- **Detail tab spacing tightened**: removed blank lines between Labels / Annotations / Fields / Containers sections; label column shrunk from 14 to 12 chars; container field column from 10 to 8.
- **Panel expand key `+` → `=`** so it works without Shift. `-` still restores.
- **Search input fixed in panel 1/2/3 + popups**: `j`/`k` are now typed as characters in search mode (previously hijacked as navigation, blocking inputs like "kafka" or "jenkins"). `↑`/`↓` remain for navigation while searching.
- **Namespace and context popup titles updated** with a Nerd Font glyph: `󱧌 Namespaces` and `󱧌 Contexts`.
- **Toast popup style aligned with picker popups** (same `#74c7ec` border, `󱧌` title glyph).
- **Easter egg hotkey `K` → `V`**; `K M 8` caption now followed by `Hi! It's KubeMate.` tagline.

### Fixed
- **YAML output was missing `apiVersion` / `kind`**: client-go strips TypeMeta on objects pulled from the informer cache; km8 now restores GVK via `scheme.Scheme.ObjectKinds` before marshaling.

### Documentation
- README key bindings updated for `=`/`-` expand, `y` copy, YAML tab; Features list refreshed.

## [v1.0.6] - 2026-05-18

### Added
- Easter egg (`K`): logo now reveals with a random pixel-by-pixel animation (~500 ms); each trigger produces a different order. "K M 8" caption and dim close hint appear after animation completes. `K` is intentionally absent from the help overlay.

### Fixed
- **kubectl context safety**: edit (`e`), delete (`D`), and shell exec (`s`) now pass `--context <name>` to every `kubectl` subprocess. Previously they used the ambient kubeconfig `current-context`, which could differ from what km8 was showing after a context switch.
- **Watch reconnection goroutine leak**: each namespace or context switch previously leaked one goroutine that persisted until the next switch; after N switches there were N+1 goroutines waiting on the same channel. Fixed by checking `ok` on channel receive — closed-channel wakeups return `nil` and break the chain.
- **Watch recursive stack growth**: the watch reconnection loop was implemented as a recursive `run()` call; K8s closes watch streams every ~5–10 minutes, so the goroutine stack grew by one frame per reconnect indefinitely. Replaced with an explicit outer loop.
- **Context-canceled log noise**: rapidly switching resources cancelled the previous watch context mid-flight, which non-deterministically logged a spurious `ERR watching <resource>: context canceled`. Now silently discarded when the context was intentionally cancelled.
- **Init log messages discarded**: "km8 started" and "connected to …" were written inside `Init()` which has a value receiver — the mutations were silently thrown away. Moved to an `appInitMsg` dispatched through `Update()`.
- **Shell exec success badge**: exiting a shell (`s`) showed a green "✓ applied" badge and wrote an empty audit entry. Fixed by returning `EditDoneMsg{Output: "no changes"}` to suppress both side-effects.
- **Splash animation never started**: `SplashModel.Show()` was changed to return a `tea.Cmd` for the first tick, but the call site in `app.go` discarded the return value, so the pixel-reveal animation never ran.
- **Duplicate edit / stale badge / double renderAllLines** (carried from main, not previously released): `editing` flag prevents concurrent edits; success badge timer carries a generation ID to prevent stale timers from clearing a newer badge; `maxScrollOffset()` no longer calls `renderAllLines()` to avoid double computation.

### Documentation
- Added **Editing Resources** section to README: three-step flow (get → editor → apply), editor resolution order, comparison table vs `kubectl edit` (declarative apply vs strategic merge patch, no `resourceVersion` check, `last-applied-configuration` annotation caveat, Helm/operator warning).
- Added **Context Isolation** section to README: km8 context is session-local, does not modify `~/.kube/config`, all kubectl subprocesses use `--context`.

## [v1.0.5] - 2026-05-18

### Changed
- **kubectl edit rewritten as get → edit → apply flow**: the editor now opens a local temp file instead of running `kubectl edit`, so vim/nvim works correctly with no "Edit cancelled" message leaking into the terminal. Changes are applied with `kubectl apply -f` after the editor exits; if the file is unchanged the apply is skipped entirely.
- Editor resolution order: `config.yaml editor:` → `$VISUAL` → `$EDITOR` → `vi` (macOS/Linux) / `notepad` (Windows). Editor strings with arguments (e.g. `code --wait`) are handled correctly.
- Audit and crash logs now rotate daily (`audit-YYYY-MM-DD.log`, `crash-YYYY-MM-DD.log`) instead of per-session; multiple events in a day append to the same file.
- App log entries are now word-wrapped at the popup width instead of truncated; scrolling is line-based so wrapped entries scroll smoothly. Maximum entry length capped at 300 characters; log buffer increased to 1000 entries.

### Added
- Status bar shows a green `✓ applied` badge for 2 seconds after a successful `kubectl apply`
- App log `OK` level (green) for successful operations; `WARN` (yellow) for editor crash or apply failure
- Apply failure triggers both an app log warning and a status bar error notification

### Fixed
- Editor crash (non-zero exit code) is now caught: the temp file is cleaned up and no apply is attempted

## [v1.0.4] - 2026-05-17

### Added
- Audit log: each session writes `audit-YYYY-MM-DD_HH-MM-SS.log` under the km8 log directory, recording every `kubectl edit` and `kubectl delete` operation with timestamp, resource, namespace, and kubectl output
- `XDG_CONFIG_HOME` support: if set, overrides the platform default config directory (useful for keeping config under `~/.config` on macOS)

### Fixed
- **Critical**: replaced `klog.NewKlogr()` with `logr.Discard()` — the previous logger caused infinite recursion (stack overflow) on certain client-go error paths, crashing the app and corrupting the terminal
- Crash recovery now uses `tea.ErrProgramPanic` so bubbletea correctly restores the terminal before handling the panic
- Fixed TOCTOU race in `Watcher`: both channels are now fetched atomically via `Channels()` to prevent stale channel references after `Start()` recreates them
- Pressing Esc after search in Panel 1 (Sidebar) now returns the cursor to the previously selected resource instead of jumping to index 0
- Pressing Esc after search in Panel 2 (Table) now returns the cursor to the previously selected row instead of jumping to index 0
- Pressing Enter while searching now immediately activates the selected item in both Panel 1 and Panel 2 (previously required two Enter presses)
- Added `go test -race ./...` to CI to catch race conditions automatically

## [v1.0.3] - 2026-05-17

### Added
- Open/close animation for all overlay popups (horizontal line expanding then vertical reveal; reversed on close)
- Hidden km8 logo easter egg — press `K` to show a centered pixel-art logo, `Esc`/`q` to dismiss
- Navigation hotkey hints (`j/k scroll`, `u/d page`, `gg/G top/bot`) added to status line for all panels
- `/ search` hint added to Panel 3 (Detail)

### Changed
- Status line redesigned: removed panel numbers (`[1]/[2]/[3]`), separators (`|`), and colons; keys now styled in bold sapphire with dim descriptions
- Status line auto-wraps to a second row when hints exceed terminal width (drops trailing hints if still too narrow); panel heights adjust accordingly
- Confirm popup width now adapts to content (capped at 70% of screen); long detail strings (e.g. `kubectl exec` commands) word-wrap inside the popup

### Fixed
- Confirm popup right-edge padding when content nearly fills the box

## [v1.0.2] - 2026-05-16

### Changed
- Confirm, namespace picker, and context picker modals now render as overlay popups (background remains visible, consistent with help and app log)
- All popup border color changed to sapphire (#74c7ec, Catppuccin Mocha)
- All popups now have vertical padding for less cramped appearance

## [v1.0.1] - 2026-05-15

### Changed
- Status bar and status line backgrounds removed (terminal transparent)
- Status line text color changed to blue (#89b4fa, matching panel focus color)
- Status line simplified: removed panel titles, kept panel number + hotkeys only
- Panel 2 status line now includes +/- expand hint

### Fixed
- Status line was using StatusBarStyle (white) instead of StatusLineStyle

### Docs
- Added screenshot, badges, and removed ASCII layout from README

## [v1.0.0] - 2026-05-15

### Added
- **CRD support** -- dynamic discovery of Custom Resource Definitions at startup via API server, automatic registration into sidebar under "Custom Resources" category
- **Resource registry pattern** -- centralized ResourceDefinition struct replaces 6 scattered switch statements; adding a new resource type requires a single Register() call
- **Dynamic client** -- k8s.Client now includes dynamic.Interface for CRD fetch/watch/detail
- **Error notifications** -- status bar shows red error badge with unread count; status line shows latest error message; clears on viewing app log
- **Scroll position indicator** -- "X of Y" in bottom-right border of all panels (sidebar, table, detail)
- **Page scroll (u/d)** -- half-page up/down in sidebar, table, detail, and app log
- **Help overlay popup** -- `?` overlays keybinding reference on top of main UI using bubbletea-overlay library
- **App log overlay popup** -- `!` overlays app log on top of main UI with unified format (timestamp + level + message)
- **Sidebar viewport scrolling** -- cursor stays within visible viewport, auto-scrolls at edges; category headers stay visible when scrolling up
- **Debounced API requests** -- 300ms debounce on sidebar resource switching to avoid flooding the API server
- **Crash logging** -- panics captured and written to `~/.config/km8/logs/crash-TIMESTAMP.log` with stack trace
- **Version flag** -- `km8 --version` / `km8 -v`
- **Release pipeline** -- goreleaser v2 config for multi-platform builds (linux/darwin/windows × amd64/arm64), Homebrew tap, Scoop bucket
- **GitHub Actions** -- tag-triggered release workflow with cross-platform tests
- **GPL-3.0 license**

### Changed
- **ResourceType** changed from `int` (iota) to `string` for CRD compatibility
- **Delete key** changed from `d` to `D` (shift+D) to free `d` for page-down
- **Detail tab keys** changed from `[`/`]` to `h`/`l` for consistency with vim motion
- **Search icon** changed from `/` character to nerd font filter icon (U+F0233)
- **App log order** reversed -- newest entries at the top
- **API rate limit** increased from default QPS 5/Burst 10 to QPS 50/Burst 100
- **Watcher lifecycle** -- channels are closed and recreated on each Start() to prevent stale goroutines from stealing data

### Fixed
- Crash on rapid resource switching (stale ResourceDataMsg with wrong type causing type assertion panic)
- Nil pointer crash when ResourceErrorMsg.Err is nil
- Sidebar scroll not working (height was 0 before first View render; now set in layout on WindowSizeMsg)
- Help page content outdated (missing c/!/D/s keys, stale `[`/`]` reference)

## [v0.1.0] - 2026-05-14

### Added
- Go module with Bubble Tea (bubbletea), Bubbles, Lipgloss, and client-go
- K8s client layer: kubeconfig loading, context listing, namespace filtering
- Watch-based real-time resource updates via List+Watch pattern
- Resource fetchers and table columns for 13 resource types (Namespaces, Nodes, Pods, Deployments, DaemonSets, StatefulSets, Jobs, CronJobs, Services, Ingress, ConfigMaps, Secrets, Events)
- RBAC resources: ClusterRoles, ClusterRoleBindings, Roles, RoleBindings
- Sidebar with resource tree categories and vim navigation (j/k/gg/G)
- Table panel with column headers, vim-style scrolling, and auto-updated detail
- Detail panel with structured tabs (Detail/Events/Logs)
- Lazygit-style panel borders with numbered titles [1]/[2]/[3]
- Panel focus system: Tab to cycle, 1/2/3 for direct panel switching
- Namespace picker overlay (n key)
- Context switching with full screen redraw (c key)
- Pod log streaming: multi-container format, Follow mode with TailLines 100
- Search (/) in all 3 panels
- Drill-down navigation with stack-based multi-level support
- Container detail view: image, state, ready, restarts, ports
- YAML edit via `kubectl edit` (e key)
- Container shell exec with confirm popup (s key)
- Resource deletion with confirm popup (d key)
- Help overlay (? key)
- App log overlay (! key)
- +/- to expand/restore detail panel
- Theme system with YAML override
- Config loader with cross-platform paths
- Cross-platform build support (macOS/Linux/Windows)
- 88 programmatic model tests

### Fixed
- Search box width calculation for UTF-8 characters
- ANSI-aware line truncation in panel rendering
- Suppressed k8s client-go klog output corrupting TUI
- Clear screen after shell session instead of on km8 exit
