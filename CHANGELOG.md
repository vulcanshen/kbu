# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [v1.7.11] - 2026-07-14

A polish release on top of v1.7.10. Four small threads:

1. **YAML popup — h/l cross line boundaries.** In the v1.7.10
   vim-buffer rewrite `h` at column 0 was a dead-end (cursor sat)
   and `l` at end-of-line was the same. v1.7.11 wraps them: `h`
   at col 0 jumps to the previous line's last col, `l` at the
   last col jumps to the next line's col 0. Buffer corners stay
   no-op — `h` at `(0, 0)` and `l` at the last line's last col
   fall through the `switch` without a case match, so the natural
   Go idiom is the natural UI corner behaviour. Visual mode
   inherits the wrap for free (the cursor motion drives the
   selection anchor).

2. **Toast — min inner width for tiny messages.** `Copied!` used
   to render as a five-cell popup that read as a rendering
   glitch rather than a confirmation. `toastMinInnerW = 28` now
   seeds the inner-width calculation before the max-of
   (title / hint / message) pass — short messages float
   centered in a comfortable box, long messages still expand
   past the floor. The change is a single min-guard in
   `renderFullPopup`, no per-toast tuning knob.

3. **Panel 3 tab bar — palette rework for readability.** The
   v1.7.10 tab bar (blue chip on base bg with the same blue
   `E0B1` divider) had two problems: the active chip's `fg`
   competed with the panel body text below it, and the interior
   dividers pulled the eye off the active tab. v1.7.11 splits
   the tab bar's colour roles: `chipFg` (dark text on blue chip)
   / `tabBg` (crust — deeper than the panel base for contrast
   against non-chip tab cells) / `dividerFg` (focus-tiered:
   `surface0` when panel 3 has focus, `base` when it doesn't —
   dividers dim as the panel dims). Non-first tabs also drop
   their leading space regardless of active state, so a tab's
   width stays constant across focus + active-tab changes and
   the bar no longer jitters when you cycle tabs. Same Powerline
   vocabulary (E0B6 round-left cap, E0B0 hard chevron), just
   with per-role hues instead of a single blue-on-base rule.

4. **Panel 3 active tab persistence.** `state.yaml` grows a
   `tab:` field alongside `panel:` from v1.7.10. Quit on the
   Events tab, relaunch, and you land on Events — not the
   per-kind default. Restore is silent-fallback via
   `DetailModel.SwitchToTabByName(name)`: if the recorded tab
   name doesn't exist on the newly-selected Kind (say you were
   on Helm `History` and next launch loads Deployments), it
   returns `false` and the per-kind default wins. No warning,
   no toast — same trust boundary as the rest of state
   restoration.

## [v1.7.10] - 2026-07-13

A backlog-collapsing release: five queued TODO items shipped as
one, plus a large YAML popup rewrite that turns the read-only
viewer into a vim-style buffer. Four threads:

1. **YAML popup → vim-style buffer.** The popup was the last
   surface where cursor semantics didn't exist — you could scroll
   but never select a piece. v1.7.10 turns it into a vim buffer:
   `hjkl` moves a cursor cell (auto-scroll follows), `w`/`b`/`e`
   walks words, `0`/`$` line endpoints, `gg`/`G` buffer endpoints,
   `u`/`d` half-page cursor jumps. `v` enters character-wise
   visual mode — anchor snaps to cursor, hjkl extends selection
   (j/k cross lines naturally), `y` copies the selected substring
   and exits visual, `Esc` peels visual first / locked search
   next / closes popup last. Normal-mode `y` still copies the
   whole YAML for paste-back use cases. Left-side dim gutter
   renders raw line numbers (right-aligned, blank on wrap
   continuation rows) — deliberately outside cursor + selection
   coordinates so line numbers never leak into the clipboard.
   YAML syntax colouring survives every overlay layer: cursor
   line uses `ansi.Cut` to splice a reverse-video cell into the
   styled string; selection block uses lavender bg (per km8's
   color mindset — user-state) while preserving syntax colours
   outside the range.

2. **Session state persistence — where you were is where you
   restart.** Every quit now snapshots `(context, namespace,
   kind, panel-2 row cursor, focused panel)` to `state.yaml`
   alongside `config.yaml`. Next launch loads the file, applies
   context + namespace before the k8s client connects, restores
   the sidebar cursor to the recorded Kind, and populates the
   Relatives-jump-style `pendingTableSelect` so the row cursor
   snaps onto the last-selected object when the first
   `ResourceDataMsg` arrives. Focused panel (sidebar / table /
   detail) also restored, so `q` from panel 2 lands you back on
   panel 2 next launch. Statusbar `[N]amespace` now reflects the
   client's actual namespace on startup — previously it always
   opened as "All Namespaces" even when the client was scoped
   to a specific ns via `$KM8__NAMESPACE` or the recorded state.
   State is kept separate from `config.yaml` on purpose:
   `config.yaml` is user-authored preferences (comments
   preserved, hand-editable), `state.yaml` is auto-rewritten
   every quit — mixing them would break the config trust
   boundary. `$KM8__STATEPATH` env var mirrors `$KM8__CONFIGPATH`
   for sandbox / test overrides.

3. **Panel 3 Logs / Events polish.** Container output using
   carriage-return progress redraws (`apt update`, `npm install`,
   `pip install`, `docker pull`) no longer breaks the Logs tab
   viewport — `sanitizeLogText` collapses the `\r`-separated
   segments to the final visible state before storage. ANSI
   colour escapes untouched. Events tab gains follow-latest
   parity with Logs: same live (▶) / paused (⏸) glyph in the
   tab title, `G` re-arms the tail, `k` / scroll-up pauses,
   bottom-left hint reads `u/d: page  gg: top  G: live`.
   Panel 3 tab bar tightens to one uniform rule — every tab
   renders as `" Label"` (leading space + label, boundary
   chevron acts as the trailing separator). Glyph-carrying
   Logs / Events used to have an extra space before the glyph
   AND a trailing pad on non-glyph siblings; dropping both
   reclaims ~5 cells of horizontal room and eliminates the
   asymmetry between glyph and non-glyph tabs.

4. **Cluster-mutation UX: Namespace delete + focus-content
   yank.** `D` on a Namespace row (previously blocked by
   `resourceAllowsDelete`) now opens a strong confirm popup:
   `!!! Delete namespace "X"? This will remove ALL resources
   inside it. Cannot be undone.` Three leading `!!!` picked over
   the ⚠ glyph the shared delete prompt uses so the two
   warnings are visually distinct at a glance — routine deletes
   get ⚠, namespace deletes get three loud bangs. Both the `D`
   hotkey and the Space menu Delete entry route through the
   same `deleteConfirmSurface` helper, so the text stays in
   one place. The global `y` key pivots from "copy focused
   panel" to "copy focus content": when the focus target has a
   cursor (sidebar Kind, panel-2 row, panel-3 Relatives /
   History row, YAML popup in visual mode), `y` copies just
   that row / selection — tab-separated raw values ready for
   `pbpaste | awk -F$'\t'`. When there's no cursor (Logs /
   Events / Conditions tabs, App Log popup, YAML popup outside
   visual mode), `y` still copies the whole focus content.

## [v1.7.9] - 2026-07-06

A UI-polish release on top of v1.7.8. Two threads:

1. **Panel 3 tab bar → Powerline chip chain.** The v1.7.8 tab-bar
   layout (space-cell interleave with subtle Nerd Font markers on
   interior cells and Powerline caps around the active tab) has
   been replaced with a starship-style Powerline chip chain: only
   the active tab renders as a filled chip (Catppuccin base fg on
   border-color bg), the rest sit on the panel's base bg with thin
    U+E0B1 chevrons between them. The identity chip for `[3]`
   merges with the first tab into a single "[3] Label" chip when
   that tab is active (avoids a double chevron at the join). All
   three panels now speak the same Powerline vocabulary — Panel 1
   `<E0B6>[1] Kinds<E0B0>`, Panel 2 `<E0B6>[2] <breadcrumb><E0B0>`,
   Panel 3 either `<E0B6>[3] FirstLabel<E0B0>rest…` or
   `<E0B6>[3]<E0B0>rest…` depending on which tab is active. Panel 1
   / 2 titles also grew the  U+E0B6 round-left cap for a softer
   opening edge than the previous  U+E0D4 honeycomb.
2. **Splash caption expansion.** The splash easter egg used to
   reveal just the version line after the pixel logo settled.
   v1.7.9 expands that into a three-line identity block matching
   the README intro — `KubeMate` (bold), `v1.7.9`, and
   `A single-pane kubernetes workspace` — all appearing together
   in blue 400ms after the K/8 pixel reveal completes, held for
   500ms before the "Press Esc to close" dim hint fades in.
   Everything users see now says the same thing about km8, whether
   they landed from GitHub or opened the splash directly.

## [v1.7.8] - 2026-07-05

A UI-polish release on top of v1.7.7. Five threads:

1. **Border-title chip system for Panel 1 / 2 / 3.** Every panel's
   border title now renders its "current tab body" as a filled
   Powerline chip when the panel is focused (and dimmer surface2
   chip when it isn't) —  U+E0D4 left cap +  U+E0B0 right cap,
   Catppuccin base fg on border-color bg, bold. The `[N]` panel-
   number prefix stays plain border-color-bold outside the chip
   so it reads as a fixed hotkey marker. Panel 3's tab bar mirrors
   the same treatment on its active tab (space-cell layout: N tabs
   interleaved with N+1 boundary cells, the two cells adjacent to
   the active tab become the caps, interior cells get a subtle
   Nerd Font marker, edge cells stay plain space). The old
   ` label ` / `[label]` / `─` scheme is gone — every panel now
   uses one unified "focused = filled chip" vocabulary.
2. **Panel 1 label: km8 → Kinds.** The sidebar's border title used
   to read `[1] km8` — a product-name identity but not a content
   label. Now `[1] Kinds` to match the K8s three-tier abstraction
   Panel 1 selects: Panel 1 picks a **Kind**, Panel 2 lists
   **Resources** of that Kind, Panel 3 shows one **Object**'s
   detail. Reads flush with `[2] <breadcrumb>` and `[3] <tab bar>`
   as content-describing titles rather than one branded outlier.
3. **Empty state — centered dim placeholder everywhere.** Panel 2
   with zero rows used to render just the column header and blank
   space, indistinguishable from "still loading". Now shows
   `No <kind>` (or `No <kind> matching "query"` when a search
   filter is active) centered horizontally + vertically in the
   row viewport as a dim overlay0 placeholder. Panel 3 gets the
   same treatment: every tab's placeholder (`No events` / `Waiting
   for logs...` / `No conditions` / `No revision history` / the
   Relatives "press Y for full YAML" hint / `No resource selected`)
   now renders centered + dim via `activeTabEmptyMessage()` instead
   of the previous left-aligned first-line style.
4. **Name column middle-truncation.** Kubectl-style pod names
   (`nginx-6d8f7c-xyz`) bury the identity signal at both ends — the
   old tail-only truncation (`nginx-6d…`) dropped the pod-hash
   suffix that distinguishes rows in a rollout. Panel 2's Name
   column now uses middle-truncation (`nginx…-xyz`) so both the
   app-name prefix AND the tail stay visible. All other columns
   keep tail-truncation, which reads better for prefix-heavy
   values like status / age / IP / node. New `truncateMiddle`
   helper walks rune-by-rune via `lipgloss.Width` so wide chars
   and Nerd Font PUA glyphs count correctly.
5. **Splash easter-egg refresh.** The hidden `Ctrl+K K M 8` splash
   reveal was reworked into a four-phase animation: M background
   sweeps top-to-bottom (4 pixels / 10ms), brief boundary beat
   (250ms hold), K/8 foreground shuffles in (2 pixels / 10ms),
   400ms hold before the version caption fades in, 300ms hold
   before the `Press Esc to close` hint. Total runtime ~1.5s —
   deliberate cinematic pacing over the previous single-shot
   reveal. Also: panel 2's helm-toggle hint compacted from
   `.: toggle helm` to `.: helm` to leave more breathing room in
   the bottom-left border.

### Changed

- `internal/ui/app.go` — new `focusedPanelTitle` and
  `plainTitlePrefix` helpers; `renderPanelWithScroll` now treats
  the title as pre-styled by the caller (removed outer
  `tStyle.Render(title)` wrap). Panel 2 tab title composition
  updated for chip caps; Panel 3 caller drops the trailing space
  from the `[3]` prefix so the tab bar's cell 0 provides the
  boundary. `tablePanelBottomLeft` hint compacted.
- `internal/ui/detail.go` — `TabTitle()` rewritten to use the
  space-cell layout with chip caps on the active tab; new
  `activeTabEmptyMessage()` method and centered-dim rendering in
  `View()` for uniform empty-state handling.
- `internal/ui/table.go` — new `truncateMiddle` helper; render
  path uses it for the Name column (title == "Name"), tail
  truncation preserved for other columns; centered dim empty-
  state message when `len(rows) == 0`.
- `internal/ui/splash.go` — reworked to four-phase reveal with
  `splashVersionMsg` / `splashHintMsg` message types for the
  independent caption reveals.

### Tests

- `internal/ui/table_test.go` — `TestTruncateMiddle` (9 cases
  covering fits / edge widths / wide chars / empty input);
  `TestTableModel_EmptyStateRendersMessage` and
  `TestTableModel_EmptyStateWithSearchShowsQuery`.
- `internal/ui/splash_test.go` — 3 tests for the new phase
  transitions and cap glyph rendering; existing tests still pass
  with the reworked state machine.

## [v1.7.7] - 2026-07-01

A UX polish + docs release on top of v1.7.6. Two threads:

1. **Panel 3 border hint — Logs + Relatives.** Panel 2's bottom-left
   border has surfaced `.: toggle helm` (+ `esc: exit compare`) as
   an at-a-glance cheat sheet for tab-contextual hotkeys; panel 3
   was silent. Now:
   - Logs tab → `u/d: page  gg: top  G: live` (`G` says "live"
     rather than "bottom" because `scrollToBottom` on Logs also
     re-attaches the live tail — losing that nuance would mislead).
   - Relatives tab @ depth 1 → `enter: drill`.
   - Relatives tab @ depth > 1 → `enter: drill  esc: back` (Esc
     gains a Relatives-local "pop one drill level" verb layered on
     top of the app-wide dismiss default).
   The `<key>: <verb>` format matches panel-2's chip, so the whole
   panel-border affordance vocabulary stays one shape across
   surfaces.
2. **Design docs consolidation.**
   `docs/zlc-tui-design-principle.md` rewritten as universal,
   app-agnostic guidance — the ZLC score becomes a quantitative
   framework (`X × min(1, 5/Y) × 100%`, `X` = user-perceivable
   operations over total, `Y` = core-key role union with alias
   completeness gating Y+0 vs Y+1). Reference apps table: vim 0%,
   nano ~100%, km8 ~100%. Framework boundary called out: hotkey
   ergonomics and post-learning efficiency are explicitly out of
   ZLC scope. New `docs/km8-zlc-implementation.md` documents km8's
   concrete mapping — km8 ZLC score table (X=1.0, Y=5, ZLC=100%),
   the 5 core-keys (Tab / Space / Esc / Enter / ?), the bottom
   statusline `[X]label` chip as the Layer-1 disclosure surface,
   the full km8 hotkey table, and the popup-convention v2 content
   inlined in §6 (since `.claude/rules/` isn't git-tracked). New
   §6.6.2 codifies the panel-border-hint rule shipped in thread 1:
   hints surface tab-contextual verbs only; core-keys'
   app-wide defaults live in the `?` help + statusbar disclosure
   surfaces, not repeated on panel borders.

### Changed

- `internal/ui/detail.go` `BorderBottomLeftHint()` — added Logs
  case, added Relatives depth=1 case (both were returning "").
- Demo gifs re-recorded against v1.7.7 for panel 3 renders.

### Tests

- `detail_test.go`:
  `TestDetailModel_BorderBottomLeftHint_LogsTab` added;
  `TestDetailModel_BorderBottomLeftHint_RelativesDrillDepth` updated
  to assert the new depth=1 hint and the composed
  `enter: drill  esc: back` at depth > 1.

## [v1.7.6] - 2026-06-29

A bug-fix + UX-polish release on top of v1.7.5. Six threads:

1. **Sort listPicker key routing.** When the sort picker opened from
   inside another menu (sidebar Space → SortKind, panel-2 menu →
   `[Alt-S]ort`), `j/k/g/G` keys went to the source menu underneath
   instead of the picker on top — a §1.8 violation. Routing in
   `app.go`'s Update now checks `listPicker` BEFORE `hintPopup` /
   `panel2Menu` so the topmost popup owns input.
2. **Sort listPicker layer preservation on swap.** The
   column → direction step was re-stamping
   `SetLayer(popupDepth() + 1)` on every swap, but `popupDepth()`
   counts the active picker itself → the border color jumped one
   tier deeper mid-flow (column was layer N, direction became layer
   N+1, then loop-back to column became N+2). Guard
   `openSortColumnPicker` / `openSortDirectionPicker` with
   `!m.listPicker.IsActive()` so first-open stamps the layer and the
   swap path preserves it. Documented as a new general rule in
   popup-convention.md §1.7 ("In-place swaps keep their original
   layer").
3. **PTY layer always 1.** Alterm (via `Alt+T`) and `kubectl edit` /
   `kubectl exec` (via panel-2 menu) rendered different border
   colors because `popupDepth()` at the entry handler ran BEFORE
   `closeAllBlockingPopups`'s staged Close cmds took effect — the
   chain's source popups still counted as active. Per §1.10, a PTY
   context-shift REPLACES the popup tree rather than stacking on it,
   so the layer concept is "this is the single popup on screen".
   Hardcode `SetLayer(1)` at all four PTY entry points (`Alt+T` shell
   spawn / show, `startEditMsg`, `startShellExecMsg`). Added to
   popup-convention.md §1.7 special-cases table.
4. **Panel-2 menu `C` action: close-on-state-mutation.** When the
   user picked "Mark as Compare anchor" / "Unmark Compare anchor"
   from the panel-2 menu, the menu stayed open per §1.8 default —
   but those actions are pure state mutations with no target popup,
   so the menu just hid the resulting anchor-row lavender highlight
   on panel 2 until the user pressed Esc. The handler now closes the
   menu for mark / unmark, while "Compare to anchor" (which opens
   the diff popup) keeps the menu underneath per §1.8.
5. **Panel-2 menu Compare-to-anchor slot promotion.** When in
   compare mode + cursor on a comparable row, "Compare to anchor" is
   now the FIRST entry in the item group — that's the primary intent
   when the user opened the menu on a candidate row, surfacing it
   first saves them scanning past four row-CRUD verbs every time.
   "Mark" and "Unmark" sub-cases stay in the post-Delete slot since
   they're row-action / cleanup verbs, not the "thing you came to do".
6. **Statusbar Alterm + Compare chips.** Replaced the
   glyph + lavender-text chips with bracket-hotkey format:
   `[Alt-t]erm` and `[C]ompare`. The `[C]` segment uses anti-
   correlated panel-aware dimming with `[C]ontext` — on panel 2 the
   table hijacks `C` for compare actions, so `[C]ompare`'s `[C]`
   lights up and `[C]ontext`'s `[C]` dims; on panels 1 / 3 the
   reverse. The chip's PRESENCE alone still signals "anchor set" /
   "Alterm alive"; brightness is the panel-routed handoff signal.
   Panel-2 menu's `[Alt][S]ort` label → `[Alt-S]ort` so one chord
   rendering rule is shared with the bottom statusline's `Alt-t`.

### Changed

- **`statusbar.go` PtyMarker / CompareMarker structs reduced to
  pointer-as-flag.** Removed `Visible` (dead code — never set true
  in production) and `Label` (chip content is now generated by
  statusbar, not supplied by caller). Callers in `app.go` switched
  to `&PtyMarker{}` / `&CompareMarker{}`.
- **`statusbar.go` Compare chip color logic.** Was hardcoded
  lavender Bold via inline lipgloss style; now uses
  `blueStyle` / `greyStyle` derived from `theme.StatusBar.ContextFg`
  + `#7f849c` overlay1, matching `[C]ontext` / `[N]amespace`. The
  lavender-as-user-state convention stays in force for the chip
  VALUES (current context, current namespace, anchor row, pinned
  section) — the chip LABELS shifted to the blue + grey rhythm.
- **`panel2menu.go` Sort entry label.** `[Alt][S]ort panel 2 list`
  → `[Alt-S]ort panel 2 list`. Same Alt+Shift+S hotkey, new chord
  notation.

### Fixed

- Sort listPicker now responds to `j/k/g/G` when stacked on top of
  panel-2 menu or sidebar Space menu (was routing to the source
  menu underneath).
- Sort listPicker border color no longer jumps mid-flow as the user
  steps through column → direction → column loop-back.
- Alterm popup and `kubectl edit` / `kubectl exec` popup now render
  with the same layer-1 border color (lavenphire25) — both are
  context-shift targets that own the whole screen.
- Marking / unmarking a Compare anchor from the panel-2 menu
  immediately closes the menu so the user sees the resulting lock
  highlight without an extra Esc press.

### Tests

13 new test cases added:

- `sortflow_test.go`: `TestUpdateRouting_ListPickerWinsOverPanel2Menu`,
  `TestUpdateRouting_ListPickerWinsOverHintPopup`,
  `TestOpenSortDirectionPicker_PreservesLayerOnSwap`,
  `TestOpenSortColumnPicker_PreservesLayerOnLoopBack`.
- `app_test.go`: `TestStartEditMsg_PtyAlwaysLayer1`,
  `TestStartShellExecMsg_PtyAlwaysLayer1`,
  `TestPanel2MenuC_MarkAnchor_ClosesMenu`,
  `TestPanel2MenuC_UnmarkAnchor_ClosesMenu`,
  `TestPanel2MenuC_OpenDiff_KeepsMenuOpen`.
- `panel2menu_test.go`: `TestPanel2Menu_Items_CompareToAnchor_IsFirstItem`,
  `TestPanel2Menu_Items_MarkAnchor_StaysAfterDelete`,
  `TestPanel2Menu_Items_UnmarkAnchor_StaysAfterDelete`.
- `statusbar_test.go`: `TestStatusBarModel_ViewFull_PtyMarker_RendersAltermChip`,
  `TestStatusBarModel_ViewFull_PtyMarkerCoexistsWithErrorBadge`,
  `TestStatusBarModel_ViewFull_CompareMarker_RendersCompareChip`,
  `TestStatusBarModel_CompareChip_PanelAwareDimming_WidthInvariant`,
  `TestStatusBarModel_CompareChip_RendersOnEveryPanel`.

## [v1.7.5] - 2026-06-28

A polish release on top of v1.7.4's popup design-system overhaul.
Headline threads:

1. **Popup convention §1.8 / §1.9 / §1.10** — codifies who closes whom
   across the popup stack. Opening a new popup no longer tears down the
   source (§1.8); Esc dismisses every popup including auto-dismiss
   toast (§1.9); context-shift targets (PTY / drill-down) own the
   close of every blocking popup beneath them (§1.10).
2. **PTY two-phase stop** — `kubectl edit` / `kubectl exec` lost both
   its close animation on subprocess exit AND its open animation on
   the next launch. Fixed by deferring `Stop()` until the animator
   finishes painting closed, mirroring the Alterm Alt+T hide pattern.
3. **Panel 3 unfocus dim** — Events / Conditions previously rendered
   identically focused or not. They now collapse to the dim treatment
   sidebar / table / history already use. The dim grey scale was
   stepped from overlay0 → overlay1 app-wide (overlay0 read as
   "disabled" on streaming-content tabs). **Logs is the documented
   exception**: streaming content is information actively arriving, and
   dimming it would hide log lines the user is glancing for from the
   corner of the eye. Logs renders identically focused or not — matches
   how Lens / k9s treat streaming logs.
4. **KM8erm → Alterm rename.** The embedded persistent shell popup is
   now called Alterm app-wide — code identifiers, yaml config keys, env
   var names, docs, demo gif filenames. One release of transition for
   the user-facing settings: the legacy `km8erm_shell` /
   `km8erm_login_shell` yaml keys are auto-migrated on load, and the
   legacy `$KM8__SHELL` / `$KM8__LOGIN_SHELL` env vars are still read
   as a fallback. Both paths emit an App Log warning to nudge the user.
   **Removed in the next release** — update your config.yaml and your
   shell rc / launchctl plists this cycle.

### Added

- **Panel 3 unfocus dim for Events / Conditions tabs.**
  `DetailModel.SetFocused()` now rebuilds every tab on focus change,
  not just Relatives / History. Events / Conditions / Relatives /
  History collapse to overlay1 (`#7f849c`) via `TableDimRowStyle`
  when the panel isn't focused. Logs is the streaming exception
  (see below) and does not dim.
- **One-shot legacy config rewrite — with backup.** When the
  deprecation migration fires (legacy `km8erm_*` yaml keys detected),
  AppModel copies your original `config.yaml` to a sibling
  `config.yaml.old.1_7_5` snapshot first, THEN calls `cfg.Save()`
  to rewrite the live file with the new `alterm_*` keys (dropping
  legacy). The deprecation warning surfaces once this session —
  next launch finds new keys and stays silent. Env-var warnings
  keep recurring since km8 can't rewrite the user's shell rc /
  launchctl plists.

  Why the backup: `cfg.Save()` goes through `yaml.Marshal(struct)`
  which can't preserve user-added comments or yaml keys km8 doesn't
  recognize. The `.old.1_7_5` file is the escape hatch for power
  users who hand-edited config — if you wanted those comments back,
  they're there verbatim. Backup failure ABORTS the Save (warning
  recurs next launch) — losing your custom content silently would
  be worse than nagging you again.
- **Peach warn badge — distinct from error.** App Log now tracks
  warn and error counts separately. The status bar's right-side badge
  splits accordingly: red `! N errors` for real failures, Catppuccin
  Peach (`#fab387`) ` N warnings` for non-critical nudges
  (deprecation, transient hiccups). Same Peach as toast warn border
  + App Log WARN entries — one warning colour across the whole app.
  Badge glyph uses Nerd Font U+F071 (`nf-fa-warning`) per design-guide
  §3.2 (glyphs limited to `U+F...` Nerd Font range). The status line
  bottom notice follows the same precedence (error red > warn peach >
  success green).
- **`AppModel.closeAllBlockingPopups()` helper.** Batches `Close()`
  for the 13 blocking popups (toast + PTY slots excluded — toast
  auto-dismisses, PTY slots have their own mutex). Returns `nil`
  when nothing is open so callers can unconditionally
  `tea.Batch(closeAll, ...)`. Used by every context-shift entry
  handler (`startEditMsg` / `startShellExecMsg` / Alt+T /
  `enterDrillDown`). Public `Close()` added to `HelpModel` +
  `AppLogModel` so the helper can drive them.

### Changed

- **Popup §1.8 — opening a popup no longer closes the source.**
  `Panel2MenuPopupModel.commit()` no longer self-closes. The menu
  was the only popup that tore itself down on commit, so Esc on the
  target popup (confirm / yaml / pty) used to drop the user back to
  the panel instead of returning here. Aligns with the canonical
  Relatives → switch-confirm pattern.
- **Popup §1.10 — context-shift targets close the popup stack.**
  Inline action popups (confirm / yaml / diff / sort picker / helm
  docs) still stack on top and return to the source. But context-
  shift targets — PTY shell, kubectl edit, kubectl exec, drill-down
  — take the user out of km8's popup tree for minutes; returning
  to a stale source popup over swapped content is disorientation,
  not anchoring. The target's entry handler now calls
  `closeAllBlockingPopups()` at the top. Every launch site (direct
  hotkey, panel 2 menu commit, future surfaces) gets the right
  behavior for free.
- **Dim grey scale: overlay0 → overlay1.** App-wide step from
  `#6c7086` to `#7f849c` across 17 active call sites. overlay0 read
  as "disabled" on the new panel 3 streaming tabs; overlay1 lands
  at "still there, still updating" without competing for focus. The
  6 popup chrome files that were already on overlay1 by design are
  untouched.

### Fixed

- **PTY close + restart animation lost.** `txPty` (kubectl edit /
  kubectl exec) lost its close animation on subprocess exit AND
  its open animation on the next launch. Root cause: `ptyTickMsg`'s
  done branch called `p.Stop()` synchronously, which nilled
  `term` + `ptmx` before `animator.Close()` could paint over the
  grid, and the animator was never told to close so the next
  `Start` hit the "already open → no-op" guard. Fix: two-phase
  teardown — done branch sets `stopPending = true` + calls
  `animator.Close()` + emits `PtyExitMsg` via `tea.Batch`, but does
  NOT call `Stop()`. `HandleTick()` runs `Stop()` only when state
  settles in `PopupClosed` AND `stopPending` is true. Mirrors the
  Alterm Alt+T hide pattern that always worked.
- **Esc on auto-dismiss toast.** `app.go` case `"esc"` now short-
  circuits to `m.toast.Dismiss()` before its own filter-clear /
  drill-exit work. Blocking popups already handle Esc themselves;
  toast was the gap because it's non-blocking and can't intercept
  keys (§1.9).

### Tests

- `TestDetailModel_EventsConditions_DimOnUnfocus` — locks Events /
  Conditions dim. Forces `lipgloss.TrueColor` profile (default `Ascii`
  in `go test` would strip the diff), asserts SetFocused flips
  contentLines bytes while ANSI strip yields identical plain text.
- `TestDetailModel_Logs_NoDimOnUnfocus` — locks the streaming
  exception: Logs must render identically focused or not, byte-for-byte.
- `TestPtyView_StartEcho_Exits` — unwraps `BatchMsg` to find the
  `PtyExitMsg` payload, asserts `stopPending` mid-flight, finalizes
  the animator via HandleTick, asserts deferred `Stop()` ran.
- `TestPtyView_SecondStartReplaysOpenAnimation` — pins the restart
  fix: Start → exit-detect → finalize close → Start, assert
  animator transitions through `PopupOpeningLine`.
- `TestPanel2Menu_CommitKeepsMenuOpen` — asserts the menu stays
  `IsActive()` after commit and no self-close `AnimTickMsg` slips
  into the batch.
- `TestAppModel_CloseAllBlockingPopups_NilWhenIdle` /
  `_ClosesActiveOnes` — helper contract.

### Renamed (KM8erm → Alterm)

- **Config yaml keys** (legacy keys auto-migrated this release,
  **removed next release**):
  - `km8erm_shell` → `alterm_shell`
  - `km8erm_login_shell` → `alterm_login_shell`
- **Environment variables** (legacy names read as fallback this
  release, **removed next release**):
  - `KM8__SHELL` → `KM8__ALTERM_SHELL`
  - `KM8__LOGIN_SHELL` → `KM8__ALTERM_LOGIN_SHELL`
- **Demo gif**: `docs/demo-km8erm.gif` → `docs/demo-alterm.gif`
  (re-recording pending; the README reference is updated).
- All Go identifiers, popup titles, statusbar marker label, comments,
  and docs swept from `KM8erm` / `km8erm` to `Alterm` / `alterm`.

## [v1.7.4] - 2026-06-27

A popup design-system overhaul. Every popup now follows a single
convention codified in `.claude/rules/popup-convention.md`: 4-class
taxonomy (menu / message / viewport / pty), mandatory glyph+text
title, padded layout for menu+message kinds, animator-driven
open/close, and — the headline change — a **layer-based border
color algorithm**. The deeper a popup nests, the deeper its color
runs along the `lavender → sapphire` scale, so the visual stack
reads "this thing is on top of what's underneath" without having
to think about it.

### Added

- **Popup layer color algorithm.** Border + animator stroke color
  is derived from a popup's nesting depth: L1 → `lavenphire25`
  (#A4C0FA), L2 → `lavenphire50` (#94C3F5), L3 → `lavenphire75`
  (#84C5F0), L4+ → `sapphire` (#74c7ec, catppuccin Mocha
  ceiling). `theme.PopupLayerColor(layer int)` is the single
  source of truth; every popup model has a `SetLayer(int)` method
  that updates both the border and the animator stroke in
  lockstep. `AppModel.popupDepth()` counts currently-rendered
  popups so `SetLayer(popupDepth()+1)` Just Works at every open
  site. Sub-popups (comparemenu inside comparepopup) use
  `parent.layer+1` so the menu always reads as one notch deeper
  than the host. Future expansion subdivides the scale further or
  raises the ceiling beyond sapphire.
- **PTY popup open/close animation.** `shellPty` and `txPty` now
  have their own PopupAnimator (targets `ptyview_shell` /
  `ptyview_tx`) so Alterm and kubectl edit/exec popups fade in
  and out like every other popup. Previously they snapped in,
  which read as a frame drop.
- **PopupAnimator on Compare's Diff menu.** The Space-triggered
  layout-toggle menu inside comparepopup grew its own animator —
  it was the last popup-shaped surface in the codebase still
  toggling synchronously.
- **Namespace picker animated spinner.** Replaced the "Loading
  namespaces…" body row with a 10-frame braille spinner
  (⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏ @ 80ms) tucked into a fixed-width slot in the
  title bar. lipgloss.Width(title) stays constant across
  loading↔loaded so border-shake is impossible.

### Changed

- **Toast: 3-row → 5-row, layer color, fixed title text.** Toast
  used to be a minimal glyph-only frame. Per the popup-convention
  v2 rule that every title must be `glyph + text`, toasts now
  render as a 5-row message-class popup: `{level glyph} km8` in
  the title, padRow top + body + padRow bottom, and a hint bar
  (`auto-dismiss` for transient, `Esc: close` for sticky). Border
  follows the layer scale for info / sticky; warn keeps its
  Catppuccin Peach (#fab387) — warning signal takes precedence
  over the layer color.
- **Popup convention v2 — 4-class taxonomy.** Every popup is now
  one of `menu` (interactive selector, cursor + Enter),
  `message` (short text + binary or scroll action),
  `viewport` (long content, maximize vertical), or `pty`
  (embedded subprocess frame). Each class has a spec table —
  padding, title, hint — codified in
  `.claude/rules/popup-convention.md` §2.
- **Idiom B → A: named `padRow` recipe across the board.** Five
  popups (applog / confirm / context / namespace / help) used to
  get top/bottom padding via the inline
  `bodyLines := []string{""}` + `append("")` trick. Converted
  all to the canonical
  `padRow := left + strings.Repeat(" ", innerW) + right` recipe
  + explicit `b.WriteString(padRow)` bracketing the content loop.
  Visual output unchanged; reads as "this is the convention" now.
- **Periwinkle removed.** The custom `#A4BAFC` mid-purple that
  every popup used to share is gone — replaced everywhere by the
  layer color. Comments mentioning periwinkle as a border
  reference cleaned up; the constant itself no longer exists in
  `theme/theme.go`.
- **PTY Shell border joins the layer system.** Alterm popups used
  to render with Lavender (`#b4befe`) borders to mark "this is
  YOUR persistent shell". The Alterm identity now lives only on
  the statusbar marker (still lavender — user footprint); the
  popup border itself follows the layer scale like every other
  popup, so a popup over Alterm reads one notch deeper than
  Alterm regardless of kind.
- **comparemenu extracted to its own file.** The Space-triggered
  Diff menu (`Switch view` / `Close`) used to live inside
  `comparepopup.go` as `overlayMenu`. Moved to `comparemenu.go`
  with its own `CompareMenuPopupModel` so it falls under the
  popup-convention filename audit (one file = one popup) just
  like every other popup.
- **Top/bottom padding row added to comparemenu.** The newly-
  extracted Diff menu was the only menu-class popup without the
  one-row breathing space between title border and items.
  Brought into line with every other bordered menu popup.
- **`settingspopup` animator target renamed.** Was `"settings"`,
  now `"settingspopup"` — every other popup's animator target
  matches its filename. Convention cleanup, no functional change.

### Fixed

- **PTY close animation now plays.** `app.go`'s view-overlay gate
  used `IsActive()`, which flipped false the moment `hidden=true`
  during hide — the close-animation tick fired but `RenderPopup`
  was never called for it. Added `IsRendered() = IsActive() ||
  animator.IsActive()` to `PtyView` and gated rendering on it;
  input routing still goes through `IsActive()` so keys don't
  leak into a popup that's fading out.
- **PtyView Shell animator stroke color matches border.** Pre-
  v1.7.4 the animator was constructed with periwinkle but the
  rendered border drew with lavender, so the open-line stroke
  was a different color than the expanded popup it became. With
  Shell joining the layer system, animator stroke + border are
  now always in lockstep via `SetLayer`.
- **Detail panel scroll preserved across watcher refresh.** The
  v1.7.3 same-UID guard fix carried forward without regression.

### Removed

- **`theme.Periwinkle` constant** (replaced by the layer-color
  scale; popups that need a border color now use
  `theme.PopupLayerColor(layer)`).

## [v1.7.3] - 2026-06-26

Polish pass on top of v1.7.2. Three real additions — **row switch
debounce**, **Logs as first tab for workload kinds**, and
**abnormal-only status coloring** — plus a comprehensive color
mindset cleanup, two visible bug fixes (popup shake on watcher
tick / scroll snap-to-top on refresh), and the K-character icon
re-chamfered.

### Added

- **Row switch debounce (300ms).** Mashing j/k through panel 2 used
  to fire one detail fetch + one log-stream Start per row, even
  for the 49 rows the cursor flew past. Each RowSelectedMsg now
  bumps a `rowSeq` counter and schedules a tea.Tick for 300ms; only
  the latest tick (matching seq) actually dispatches the fetch +
  stream Start. The immediate dispatch still does the cheap state
  mutations (Stop previous stream, lock `logsActive` against
  ResourceDataMsg double-fire, clear the aggregate-retry throttle)
  so the lie-as-lock invariant is preserved across the debounce
  window. Constant `rowSwitchDebounce` matches the existing
  `switchSeq` sidebar-debounce window for muscle-memory
  consistency.
- **Logs is the first tab on workload kinds.** Pods / Deployments /
  StatefulSets / DaemonSets / Jobs / CronJobs reorder their panel-3
  tabs from `[Relatives, Logs, Events]` to `[Logs, Relatives,
  Events]`. Switching rows in panel 2 is most often a "what is this
  thing doing right now" gesture — Relatives is a deliberate drill
  action that warrants the extra tab-switch. Non-workload kinds
  (ConfigMap / Service / Node / etc.) keep Relatives first; the
  Relatives-tab Space-jump still lands on the same tab the user
  came from.
- **Status coloring (abnormal-only, yellow + red).** Restored for
  Status column on every kind that has one (Pod, Node, Namespace,
  PVC, PV, Helm Release) plus Events Type column. Pending /
  Terminating / SchedulingDisabled / Released / pending-*
  → yellow; Failed / Error / CrashLoopBackOff / ImagePullBackOff /
  NotReady / Lost / Warning / Init:* → red. Healthy states
  (Running / Bound / Active / Deployed / Normal / ...) stay at
  the row's base foreground — "color is signal", not decoration.
  Cursor / lock rows pick the darker Latte variant so pastel Mocha
  doesn't wash out on the reverse-video bg.

### Changed

- **Color mindset — three accent colors, one role each:**
  - **Blue** `#89b4fa` — structural (panel category headers,
    popup-key hints, statusbar `[C]ontext` / `[N]amespace`).
  - **Lavender** `#b4befe` — in-panel user-set state (pinned items,
    Settings ON toggle, **compare anchor row + statusbar chip**,
    unfocused-cursor chip).
  - **Periwinkle** `#A4BAFC` — floating-overlay layer (every
    popup's border + animator: hintpopup, breadcrumb, helmdocmenu,
    panel2menu, listpicker, **comparepopup**).
  
  The pre-v1.7.3 compare cyan `#9DDAEA` was the only non-palette
  accent in active use; it's gone from the codebase entirely.
- **Compare popup title — `Compare` word removed.** Was `<icon>
  Compare — left vs right (unified)`; now just `<icon> left vs
  right`. The statusbar chip already says `<icon> Compare`, and the
  layout (split vs unified) is visible from the diff content
  itself.
- **Statusbar compare chip — fixed-width "Compare" label.** Was
  `<icon> <resource-name>`; now `<icon> Compare`. Resource names
  are unbounded (some pod names easily exceed the available chip
  width); the popup itself carries the names when the user engages.
- **Follow-tail marker on the Logs tab — color → glyph.**
  v1.5.x – v1.7.2 painted the active `Logs` label green when
  auto-follow was on. Color is now reserved for "this row / cell
  needs your attention" — the live / paused state moves to a Nerd
  Font glyph (U+F0753 mdi-play / U+F0754 mdi-pause) inline in the
  tab label. The glyph shows on the Logs tab regardless of active
  state so the tab bar width stays constant across tab changes.
- **Compare anchor row picks up Bold.** Switching the anchor color
  to lavender visually demoted the row from "highlighted" to
  "highlighted but thinner" without the Bold to match the cursor
  styles — added back.
- **K icon: arm corners chamfered.** `docs/icon.svg` re-cut so the
  K's upper / lower arms get a 1-cell taper at the outer corner,
  with a single interior pixel a row above / below. Synced to
  `internal/ui/splash.go logoPixels` (the easter egg) and the
  1280×640 GitHub social preview.

### Fixed

- **Panel 3 viewport snap-to-top on watcher refresh.** SetDetail
  unconditionally reset `scrollOffset` to 0, which the watcher's
  periodic ResourceDataMsg ticked into every few seconds — Logs
  viewing at tail / Relatives scrolled mid-list / Events scrolled
  down all silently snapped back to top on every refresh. Most
  visible on Logs of an idle pod where no incoming line arrived
  to push scroll back to bottom. Fixed via a same-UID guard:
  different-UID SetDetail (= actual row change) still resets,
  same-UID refresh preserves scroll position + `historyCursor`.
- **Popup shake on watcher tick.** Centered popups (Compare /
  YAML / AppLog / breadcrumb) appeared to "vibrate" by 1 cell on
  each watcher refresh in narrow terminals. Two causes:
  - The refetch spinner (` ⠋`, 2 cells) was appended to panel 3's
    border title and toggled on / off with each fetch dispatch
    and arrival, intermittently inflating panel 3's rendered
    width by 2 cells. This propagated through the horizontal
    join to `mainView`'s width, shifting `overlay.Composite`'s
    Center calculation by 1 cell per render.
  - Symmetrical bug in the tab bar: the Logs tab glyph was only
    attached when Logs was the active tab, so switching to
    Relatives / Events / Conditions contracted the tab bar by
    2 cells and switching back expanded it — same shake
    mechanism via tab bar instead of spinner.
  Both fixed by removing the spinner outright and pinning the
  Logs glyph always-on.
- **Narrow-terminal popup title truncation gets `…`.** Both
  comparepopup and yamlpopup truncated long titles by hard cut
  without an ellipsis indicator — `... default/nginx vs demo/demo-
  app-…-nwx78 (u─╮` read as a literal label fragment rather than
  a cut. Now appends `…` in the title style so the cut is
  obvious.

### Removed

- **Panel 3 refetch spinner.** The braille spinner that appeared in
  the panel 3 border title during fetchResourceDetail dispatch is
  gone (along with `BeginRefetch` / `SpinnerSuffix` /
  `IsRefetching` / `advanceSpinner` / `detailSpinnerTickMsg` /
  `detailSpinnerFrames`). The per-row detail refresh either
  arrives or it doesn't — the user doesn't need a "loading"
  affordance for it. Side benefit: panel 3's title reported width
  no longer flickers on each fetch toggle, which is what fixes
  the popup shake above.

## [v1.7.2] - 2026-06-26

Feature pass on top of v1.7.1. Three real additions — **Compare
diff alignment rewrite**, **aggregate logs widened to all 5
workload kinds**, and **shell / config env-var overrides** — plus
README repositioning around the Zero Learning Curve + Tab/Space/
Enter/Esc + Compose-don't-Replace mindset, and a sidebar dead-state
fix. The Compare tab gets a noticeable upgrade: near-twin ConfigMaps
now line up vs each other instead of splintering into character
fragments.

### Added

- **Compare diff: line-LCS alignment.** Compare's split-view used to
  run a byte-level edit list against the two YAMLs, which on
  near-twin inputs (two ConfigMaps with the same keys but a few
  different values) splintered changed lines into `+ CC`, `+ f`,
  `+ r`, ... character fragments — visually unreadable. Replaced
  with a line-LCS alignment pass: changed lines surface as
  full-line-left vs full-line-right, context lines pair line-by-
  line, pure insertions/deletions render in their own column.
  Blank-line inserts no longer get swallowed by the context branch
  (previously fell through because the disambiguator used
  `left/right == ""` predicates — replaced with an explicit
  `splitPairKind` enum: Context / Changed / Insert / Delete).
- **Compare diff: OOM cap.** Both split and unified diff paths cap
  per-side input at 2000 lines before allocating the DP table.
  Above the cap, the renderer surfaces a truncation banner row so
  the user knows the diff was clipped — at the cap, the DP table is
  ~4M ints, which is the largest size that's comfortable on a
  laptop without risking a TUI OOM on a Deployment with a giant
  last-applied-configuration annotation.
- **Aggregate log streaming widened to all 5 workload kinds.**
  Previously Deployment-only; now StatefulSet, DaemonSet, Job, and
  CronJob also stream their managed pods' logs into the Logs tab.
  Owner-ref walk via `k8s.PodsForWorkload` handles the kind
  uniformly. Per-pod color tag carries through so multi-replica
  workloads stay readable.
- **Env-var config overrides.**
  - `$KM8__CONFIGPATH` — point km8 at a config file outside
    `~/.config/km8/config.yaml`. Useful for split per-cluster configs
    or running a custom config without touching the default.
  - `$KM8__SHELL` — override Alterm's shell (skip the default
    detection, strip the `-l` login flag so the prompt stays clean).
  - `$KM8__LOGIN_SHELL` — opt back into login mode (`true` /
    `1` / `yes`); default is non-login for a faster, quieter prompt.
- **`alterm_shell` + `alterm_login_shell` config keys.** Persistent
  equivalents of the env vars above. Empty string = fall through to
  platform default (`$SHELL` → `/bin/zsh` → `/bin/bash`).
- **`docs/demo-compare.gif`.** Walks Space + Enter + Space + Enter
  to lock two ConfigMaps and toggles between unified and split.

### Changed

- **README repositioned around Zero Learning Curve + Tab/Space/
  Enter/Esc + Compose-don't-Replace.** Tagline rewrites to
  "Single-pane Kubernetes workspace — Tab/Space/Enter/Esc drive
  everything". New hero quote: 「遇事不決，就按 Space」/ "When in
  doubt, hit Space". Features list leads with ZLC and Compose
  (Alterm runs other TUIs — k9s, btop, lazygit — not a replacement
  pitch). Env-var section added in both EN and 繁中 READMEs. Pinned
  drag glyph swapped from `⇅` to `󰩐` (U+F0A50). Alterm border +
  edit/exec pty colors documented as lavender + periwinkle (matches
  v1.7.1 color mindset). Panel expand hotkey corrected from `=`/
  `-` to `z`. Toast info color corrected from sky-blue to
  periwinkle. Stale "Colored Pod status" bullet removed (was
  dropped in v1.7.1). NF Mono variant recommendation added to
  Requirements.
- **Helm + Keybindings popup icons → NF Material Design glyphs.**
  Helm: `󰠳` → `󰠳` (U+F0833, helm wheel). Keybindings:
  retired the cap+key composite for `󰘢` (U+F0633, keyboard
  shortcuts). Both popups now match the cross-popup icon pattern
  (system-relevant glyph in the title chip).
- **Compare overlay menu title: `Diff` + family icon.** Compare's
  toggle popup used to render without a title; now shows `󰢫 Diff`
  (U+F08AB) so the popup's purpose reads at a glance, matching
  every other popup family.
- **Helm popup polish.** Cursor `▶` arrow dropped (the row chip
  alone signals selection), icon synced to the Material Design
  glyph, highlight overflow contained so long release names don't
  bleed past the popup edge.
- **Demo gifs re-recorded.** All 5 prior demos (basics, relatives,
  helm, yaml-edit, alterm) re-recorded for v1.7.1's visual changes,
  plus the new compare gif. Tape recording got a Sleep + Wait +
  Sleep pacing pattern so panel-1 actions that trigger panel-2
  reload wait on the actual content marker, not the border-title
  flicker.

### Fixed

- **Sidebar dead state when filter empties all visible items.**
  Pressing `/` and typing a filter string that matched nothing
  used to leave the sidebar in a stuck state — cursor pointed at
  no row, no `Esc` or backspace path. Now `/` and `Esc` both
  remain as escape hatches when the visible-items list is empty;
  the stale `gg` pending state also clears so a follow-up `gg`
  doesn't mis-fire after the filter is dismissed.
- **Internal log-stream + aggregate-retry hardening.** Multi-round
  sweep of log streamer lifecycle, stale-message handling on
  rapid row changes, and aggregate-logs retry throttling across
  navigation. No user-facing behavior change vs v1.7.1; if you
  weren't hitting these edges, you won't notice anything.

## [v1.7.1] - 2026-06-25

Polish pass on top of v1.7.0 — no new features, just a sweep of
visual + UX tightening that rolled into a coherent color mindset
across the whole app: **blue = app frame / structure, lavender =
user footprint / persistent preference, periwinkle = overlay
layer**. Plus a hotkey cleanup, dim-on-unfocused-panel treatment,
one Y bug fix, and a removed cross-kind Pod-Status special case.

### Color mindset (shared across all the changes below)

| Role | Color | Where |
|---|---|---|
| App frame / structure | catppuccin blue `#89b4fa` | Panel border (focused), Detail tab bar (focused), `[ ]` brackets in statusbar, table HeaderFg, Detail field labels, Relatives section header + drill arrow, sidebar categories (Pinned exception below) |
| User footprint / persistent preference | catppuccin lavender `#b4befe` | Pinned category, statusbar `<ctx>` / `<ns>` values, unfocused-selected chip (all 3 panels), Listpicker "current" badge, Settings ON, Alterm pty border + statusbar chip |
| User current hand | catppuccin subtext1 `#bac2de` | Focused-selected chip (sidebar + table) |
| Overlay layer (transient) | custom periwinkle `#A4BAFC` (new `theme.Periwinkle`) | Every generic popup (panel2menu, hintpopup, listpicker, breadcrumb, helmdocmenu, settings, namespace, context, confirm, applog, help, yamlpopup) + info-level toast + kubectl edit + kubectl exec pty borders |
| Compare feature | cyan `#9DDAEA` | Locked row, statusbar compare chip, compare popup chrome |

The 3-tier popup palette (mauve menus / blue Settings / sapphire
pickers) collapses into one. Sapphire `#74c7ec` and mauve `#cba6f7`
no longer carry chrome meaning — sapphire is gone, mauve only
appears as one slot in the container-log color palette.

### Added

- **`theme.Periwinkle = "#A4BAFC"`.** Named constant for the
  custom overlay accent (no catppuccin original sits between blue
  and lavender on the blue-purple axis). 14 previously hardcoded
  literals collapsed onto the constant.

### Changed

- **Panel border focused: double-line chars.** Unfocused panels
  keep `╭─╮│╰╯`; focused swaps to `╔═╗║╚╝`. Same cell count, no
  layout shift; pairs with the focused-color flip so panel focus
  reads at a glance.
- **Unfocused panels dim non-cursor content.** Panel 1 sidebar,
  panel 2 table, and the Relatives + History tabs of panel 3 all
  drop non-cursor non-locked rows to overlay0 grey when the panel
  isn't focused. The cursor row stays as a single lavender chip —
  the "remembered position" you can navigate back to. Category
  headers (sidebar) and column header (table) also dim. Table
  alternating-row striping flattens to a single dim color so the
  cursor chip is the only signal that survives.
- **Statusbar redesigned.** Cluster slot dropped (context binds
  cluster + user — duplicate signal). The two NF glyphs (U+F0237
  context, U+F51E namespace) migrated to the corresponding popup
  titles so the icon-to-concept association stays. New layout is
  `[C]ontext: <ctx>  [N]amespace: <ns>` with width panel-invariant
  (no jitter on focus change). Coloring is consistent with the
  cross-app mindset: blue brackets + lavender values + grey filler.
  On panel 2, the whole `[C]` collapses to grey because `C` is
  the Compare-anchor hotkey there.
- **Sidebar selected chip → lavender.** Both the sidebar and the
  table now render the unfocused-selected row as a lavender bg +
  base-dark fg chip (matching Pinned, statusbar `<ctx>`/`<ns>`,
  and the "user-selected / user-relevant" mindset accent).
- **Settings popup ON badge → lavender.** Was catppuccin green;
  now lavender to align with "user-set persistent state". OFF
  stays overlay1 grey. Cursor row collapses to one uniform
  dark-on-periwinkle chip; the "ON" / "OFF" word itself carries
  the state. Drops the duplicate cursor-row styles.
- **Alterm pty border + statusbar chip: amber → lavender.**
  Alterm is your persistent personal shell, the only pty kind
  that outlives its popup; same conceptual bucket as the other
  user-state accents.
- **kubectl exec pty border: green → periwinkle.** Edit + Exec
  are both transient subprocess overlays — same color group. The
  title bar (`kubectl edit pod/foo` vs `kubectl exec -it pod/foo`)
  carries the kind distinction.
- **Bottom hint line slimmed.** From `?:help q:quit N:ns C:ctx
  space:menu enter:into Alt-t:Alterm [/:filter]` to `? Esc Space
  Enter Tab Alt-t > settings`. The retired keys all live elsewhere
  now (`N` / `C` on the statusbar, `q` is a special "no popup
  open" gesture, `/` is the per-panel search hint).
- **Help popup reorganized.** New structure: **Core** (Tab /
  Enter / Esc / Space — the four cross-app gestures), **Navigation**
  (cursor + panel keys), **Global** (app-level letters), **Alterm**.
  Dropped the "Vim Navigation" framing — the cursor keys are
  universal-tui, not vim-specific.
- **Settings hotkey: `M` → `>` (shift+.).** `M` was an arbitrary
  letter; `>` doesn't collide with anything and reads as "open
  app preferences from here forward". Wired through statusline
  cheatsheet + help popup + both READMEs.
- **`q` reserved for app quit only.** Previously did double duty
  as a popup-close alias alongside Esc across 13 popups; now Esc
  is the universal close gesture and `q` only fires when no popup
  is open, where it still triggers the ConfirmQuit dialog. Inside
  any popup, `q` is a silent no-op (Esc out first, then `q`).
  Also stripped from splash (`Esc` / `Enter` / `Space` still
  dismiss it) and confirm dialogs (`Esc` / `n` / `Space` still
  cancel).
- **Sidebar categories: stay blue.** Briefly tried overlay1 grey
  to match Relatives section headers; reverted after the
  blue/lavender mindset settled — system categories are app
  structure (blue), only Pinned takes the user-curated lavender
  accent.
- **Context / namespace popup titles got distinct icons.** Both
  used to share `` (U+F4F3); now context popup uses `󰈷`
  (U+F0237, identity-card glyph inherited from the statusbar) and
  namespace popup uses `` (U+F51E, namespace glyph also
  inherited).
- **Container drill menu lost its `Esc` entry.** "Esc back to pod
  list" duplicated the universal Esc gesture — removed per the
  popup-design mindset (no redundant entries).
- **Container drill menu title got an icon.** `container/<name>`
  → ` container/<name>` (U+F4B7).
- **Pod-Status semantic coloring removed.** Was the only kind
  where the Status cell got a Running-green / Pending-yellow /
  Error-red color; the other 26 kinds rendered Status / Ready /
  Available / etc. in plain row color. Either every kind gets
  diagnostic coloring or none — none won on simplicity. `kubectl
  edit` / fetch errors / etc. still surface via the status-bar
  badge.
- **Orphan theme fields cleaned up.** `Sidebar.ClusterFg` +
  `StatusBar.NamespaceFg` + their getters dropped; both became
  dead code after the statusbar refactor.

### Fixed

- **Pressing `Y` from panel 1 (sidebar) no longer silently opens
  the LAST panel-2-selected row's YAML.** Y is a panel-2 /
  panel-3 affordance; now gated to those panels. E / D / S /
  Alt+S already had the gate.
- **`HintPopup` no longer draws a stray horizontal divider when
  there's only one region.** Drop-only menu (Space mid-drag) had
  actions but no cheatsheet rows; the divider was rendered
  unconditionally.

## [v1.7.0] - 2026-06-25

Polish release on top of v1.6.0. Two real new features — **multi-
column sort** and **pinned drag-and-drop reorder** — plus a sweep
of popup-design unification (region headers, region-aware
selectable navigation), sort-flow UX overhaul (loop, swap
animation, conditional Unset, configurable Reset), and one
breaking hotkey swap on panel 2.

### Added

- **Multi-column sort (chain).** Sort config now stores a list of
  tiers instead of a single column. Pick a column → direction →
  popup loops back to the column picker so additional tiers can be
  stacked without re-invoking the flow. Each tier in the chain
  renders its priority and direction in the table header
  (`Name (1) ↑ · Restarts (2) ↓ …`); single-tier chains collapse
  to just the arrow to preserve the v1.6 look. Comparator chain
  walks tiers in order, first non-zero wins; unknown columns
  silently skip so a stale config doesn't break the sort. YAML
  back-compat: the v1.6 single-mapping shape (`sort: {column,
  direction}`) lifts to a one-tier chain on load; a load-then-save
  cleanly migrates to the new sequence form. `sort: null` /
  `sort:` / `sort: ""` also tolerated (clears the chain).
- **Pinned drag-and-drop reorder (`D`).** Press `D` on a pinned
  sidebar row (with two or more pinned kinds) to enter modal drag
  mode: `j` / `k` swap the locked kind with its neighbour,
  `Enter` or `D` commits the new order, `Esc` and anything else
  cancels back to the snapshot taken at entry. The "Pinned" header
  shows `Pinned ⇅ [D]rop` while dragging, the dragged row paints
  in lavender reverse, and a sticky toast carries the keyboard
  contract throughout. `Space` mid-drag opens a trimmed drop-only
  menu — useful when the keyboard contract slips out of memory.
- **Sort picker `Reset` shortcut.** Column picker grows a Reset
  row at the bottom (with an undo glyph) once a chain exists.
  Selecting Reset drops the entire chain, re-applies the
  `(namespace, name)` fallback to live items, and loops back to
  the now-empty column picker — never closes the popup. Direction
  picker also gains a per-column `Unset` row that only surfaces
  when that column is already in the chain.
- **Sort picker swap animation.** Switching content between chain
  steps (column → direction → loop back) no longer flashes; the
  popup compresses to 50% vertical height, content swaps at the
  midpoint, and expands back. Total ~120ms. Same visual vocabulary
  as the existing open/close animation; new
  `PopupSwappingCompress` / `PopupSwappingExpand` states.
- **Popup region headers.** Three popup families (listpicker,
  panel-2 menu, hintpopup) gain non-selectable `Header` and
  `Separator` rows so cursor navigation, Enter, hotkey dispatch,
  and mouse-click all skip them uniformly. Used wherever a popup
  mixes operation kinds:
  - **Sort column picker**: "fields" above the columns, "all"
    above Reset (when chain exists).
  - **Panel-2 popup menu**: "item operation" above
    Y/E/S/D/C+Enter, "panel operation" above the Sort entry.
  - **Panel-1 Space menu**: "item operation" above
    Pin/Unpin/Sort, "panel operation" above Drag (when the
    cursor row qualifies for drag-and-drop).
- **Sort picker title icon.** Border title gains the nf-fa-sort
  glyph (`U+F0DC`) so the picker's purpose reads at a glance —
  matches hintpopup's wheel icon and settingspopup's cog.

### Changed

- **Panel-2 sort hotkey: `Alt+Shift+S`.** Bare `S` on panel 2 is
  Shell, so the modifier carves out a panel-2 sort gesture
  without breaking that. The popup menu entry reads "[Alt][S]ort
  panel 2 list". Panel 1 keeps plain `S` (v1.6 muscle memory).
  AeroSpace users on macOS: this collides with the default
  `alt-shift-s` workspace binding; rebind in AeroSpace if you
  want km8's hotkey.
- **Panel-1 sidebar Space menu Sort entry**: now reads "Sort
  panel 2 list" (hotkey `S`). The cross-panel effect is explicit
  in the wording instead of being inferred from "Set Order in
  …".
- **Panel-1 Space cheatsheet drops the standalone `P` row.** The
  contextual Pin / Unpin entry surfaces in the action region
  above; the cheatsheet row was a duplicate.
- **Sort picker loops back instead of closing on commit.**
  Direction commit re-opens the column picker (swap animation
  plays) so additional tiers can be added without re-invoking the
  flow. `Esc` on the looped picker is the canonical "I'm done"
  exit. Reset behaves the same way — drops the chain, refreshes
  the picker, stays open.
- **Direction picker hides `Unset` for never-sorted columns.**
  Surfacing a guaranteed no-op just clutters the picker; matches
  the column-picker's "Reset hidden when no chain" gate.
- **Popup menus universally cycle on `j` / `k`.** Eight popups
  (panel-2 menu, helmdocmenu, listpicker, settingspopup, context,
  namespace, breadcrumb, hintpopup) now wrap navigation past the
  ends instead of clamping. Main panel `hjkl` unchanged.
- **Toast layering split sticky vs non-sticky.** Sticky toasts
  (background reminders like drag mode's keyboard contract)
  composite UNDER the popup stack so a user-summoned popup wins.
  Non-sticky toasts (transient interrupts like save errors) keep
  compositing ABOVE the popup stack.

### Fixed

- **Sort chain comparator silently skips unknown tiers** so a
  stale chain entry (column removed from the kind's registry
  between sessions) doesn't break the sort or crash. Stale entries
  also render invisibly in the table header.
- **`O` in container drill view** is now a silent no-op (mirrors
  E/D/C drill-mode gating). Previously the picker would open
  titled "Sort Pods by…" while the user was looking at containers.
- **`sort: null` / `sort:` / `sort: ""` in YAML** no longer fail
  the config load — they degrade to an empty chain.
- **`commitSortFlow` / `resetSortFlow` defensive paths** no longer
  close the popup on inconsistent state. Reset must never close
  the popup unilaterally, so even recovery paths now no-op and let
  the user `Esc` out.

### Removed

- **`O` as a sort hotkey.** Panel 1 went `S → O → S` during
  development; panel 2 went `O → Alt+Shift+S`. Net for an end
  user: `O` is now unbound on both panels. v1.6 users continue
  pressing `S` on panel 1 just like before; the new option is
  `Alt+Shift+S` on panel 2.

## [v1.6.0] - 2026-06-24

Four big features land together: **Pinned** sidebar kinds, **YAML
Compare**, per-kind **Sort**, and full **Mouse support** with a new
Settings popup. The keyboard model stays unchanged outside two
deliberate trims (Enter no longer forwards focus — see
**Changed**); everything else extends existing surfaces rather than
replacing them.

### Added

- **Pinned resource kinds (`P`).** Panel 1 grows a Pinned section
  at the top. `P` on any sidebar row toggles pin / unpin, and the
  pin order persists per-context into the config file. Pins move
  rather than duplicate — a pinned kind disappears from its original
  category and reappears under Pinned, so each kind always has
  exactly one home. CRD-managed kinds preserved across uninstall /
  reinstall: if a pinned CRD goes away, its pin stays in the config
  silently and restores the moment the CRD comes back.
- **YAML Compare popup.** New per-kind diff workflow on panel 2.
  Press `C` on a row to mark it as the **compare anchor** (status
  bar shows a glyph + row name), then `C` on a different row of the
  same kind opens a side-by-side or unified YAML diff. `C` on the
  anchor itself cancels — the same key toggles all three states
  (mark / diff / cancel) so muscle memory is consistent. The diff
  popup carries its own action menu (Space) for live layout
  switching, and the default layout is persisted in config.
- **List-view sort.** New three-step popup chain on panel 1 (`Sort
  <Kind>…` action via Space-menu, or direct `S` hotkey): column
  picker → direction picker (Ascending / Descending / Unset) →
  persist. Sort is per-kind and lives in the same config block as
  Pinned. Panel 2 header marks the sorted column with ↑ / ↓
  arrows (NF U+F161 / U+F160). Comparators dispatch by column title:
  `Age` / `Updated` use the underlying timestamp instead of the
  rendered "5d3h" string (which lex-sorts wrong at unit boundaries);
  `Ready` parses the "N/M" form; `Restarts`, `Desired`, `Current`,
  `Up-to-date`, `Available`, `Active`, `Rev` use the int form. No
  saved sort = `(namespace, name)` ascending — matches kubectl's
  default for cross-namespace listings.
- **Mouse support.** km8 had `tea.WithMouseCellMotion` enabled at
  the program level since v1.5.x but only scroll-wheel handlers on
  table/detail (gated on keyboard focus, so wheel never actually
  worked the way users expected). v1.6.0 wires real mouse coverage:
  - **Left-click** on a panel: focus that panel + move cursor to
    the clicked row. Sidebar fires `ResourceSelectedMsg`, table
    fires `RowSelectedMsg`, detail's Relatives tab moves the row
    cursor.
  - **Double-click**: synthesizes Enter (drill).
  - **Right-click**: synthesizes Space (opens the row's context
    menu / hint cheatsheet).
  - **Wheel**: synthesizes `u` / `d` (half-page move) on main
    panels and on viewer popups (yamlpopup, comparepopup, applog,
    help). Menu-style popups swallow wheel since their content is
    short and half-page semantics don't fit.
  - **13 popups** all gain `HandleMouse`: list popups commit on
    left-click, scroll-only popups close on right-click. The
    confirm dialog deliberately makes left-click a no-op so a
    stray click can't trigger a destructive delete / quit /
    rollback.
- **Settings popup (`M`).** New app-level config surface with a
  Catppuccin-blue accent and a cog glyph in the title. Currently
  carries two rows — Mouse on/off, Scroll Direction natural/reverse —
  with a green/grey badge that toggles on Enter or click. Persists
  per-toggle into the new `mouse_opt_config` block. The popup is
  its own escape hatch: even when Mouse is off, clicking it
  remains possible so users can turn it back on.
- **Relatives tab cursor on click.** Detail panel's Relatives tab
  now responds to mouse clicks — landing on a drillable row moves
  `relativeCursor` to it. Wrapped multi-line entries (nested
  drillable refs) collapse correctly back to a single cursor row
  whichever of their lines you clicked.
- **`mouse_opt_config` + `resource_kind_config` config blocks.**
  New nested schema:
  ```yaml
  resource_kind_config:
    pod:
      pinned: { order: 10 }
      sort: { column: Age, direction: desc }
  mouse_opt_config:
    enabled: true
    scroll_direction: natural
  ```
  Both blocks are entirely optional — legacy configs keep working
  unchanged.

### Changed

- **Enter no longer forwards focus across panels.** Panel 1 Enter
  used to focus panel 2; panel 2 Enter on non-drillable kinds used
  to focus panel 3. Both removed: with mouse double-click → Enter
  synthesis, the focus-shift fallback would hijack focus every
  double-click. Panel 1 Enter is now no-op (search-mode Enter still
  locks the filter); panel 2 Enter only drills. Keyboard users
  keep `Tab` / `Shift+Tab` / `1` / `2` / `3` for focus switching.
- **Default compare layout is now Unified.** Side-by-side `Split`
  required more horizontal room than narrow terminals could spare
  and lex-jumped at line-wrap boundaries; Unified survives narrow
  widths and reads like `git diff`. Users on wider terminals can
  switch back via the Compare popup's Space-menu (and the choice
  persists into config).
- **Status bar marker `\U000F08AA`.** Replaces the prior compare-
  mode marker; cleaner glyph that doesn't compete with the helm
  icon family.
- **`N` namespace picker opens immediately** with a "Loading…"
  placeholder and fetches in parallel — no more visible lag on
  cold connections. Esc works during loading; fetch errors close
  the popup with a toast.

### Fixed

- **Sidebar / table click in search mode was offset by 2–3 rows**
  because `renderSearchBox` emits a 3-line bordered box and the
  cursor-mapping math stepped over only 1 line (sidebar) or none
  (table). Fixed both.
- **`persistPinnedKinds` was dropping pins for unregistered kinds.**
  Naïve "wipe all and re-add from sidebar" defeated the
  `ResourceKindConfigEntry` contract that says unknown kinds stay
  in the map so a CRD reinstall silently restores their pin. Now
  the wipe scopes to kubectl-names the registry currently knows
  about; CRD entries that disappeared mid-session survive intact.
- **Helm Releases sort comparators.** `Rev` is now routed through
  the int comparator (lex sort put "10" between "1" and "2");
  `Updated` reads the time off `Raw` (`*Release.Updated` Go
  `time.String()`) instead of the rendered age string.
- **MouseMsg routing.** Four older `IsActive` blocks (appLog,
  help, contextPicker, namespacePicker) intercepted `tea.MouseMsg`
  and forwarded it to their `Update()` methods which didn't accept
  mouse events. The new `HandleMouse` dispatcher's coverage for
  those popups was unreachable — click and wheel silently dropped.
  Now trimmed to `KeyMsg` only; mouse properly reaches each popup's
  `HandleMouse`.
- **Compare anchor stale lock after re-fetch.** Watcher tick now
  re-resolves the locked UID against the fresh items slice and
  drops the lock (with a toast) if the anchored row disappeared.
- **Panel 3 Relatives scroll past last cursor.** Drill chain
  trailing rows could fall below the viewport; `relativeMoveOrScroll`
  now scrolls past the cursor when at the boundary, and the panel's
  `ScrollInfo` carries a bottom indicator so the user sees there's
  more content. (v1.5.x carry-over from 25a12ec.)
- **Sidebar Enter on no-match search clears the filter** instead
  of leaving the sidebar in a dead state with no visible items and
  no way to keystroke out. (v1.5.x carry-over from 3ce5ece.)

### Removed

- **`l` as a focus-forward key** and the **`FocusTableMsg` /
  `FocusDetailMsg`** message types — see "Enter no longer forwards
  focus" above. No keyboard alternative needed: existing
  `Tab` / `1` / `2` / `3` already covered the use case.

## [v1.5.7] - 2026-06-10

Two small `kubectl`-parity additions to the panel-2 list view: Pods
gain the `IP` column from `kubectl get pods -o wide`, and Ingresses
gain the `Address` column that `kubectl get ingress` shows by default
but km8 was previously dropping.

### Added

- **Pod IP column.** New `IP` cell between `Age` and `Node` carrying
  `.status.podIP`, matching the `kubectl get pods -o wide` layout.
  Shows `<none>` while the kubelet hasn't reported one, again matching
  kubectl. All three Pod row producers (`fetchPods`,
  `fetchPodsWithSelector`, `fetchPodsForPVC`) were updated so
  drill-downs from Deployments / ReplicaSets / Jobs / PVCs render the
  IP column too — not just the top-level Pods view.
- **Ingress Address column.** New `Address` cell between `Hosts` and
  `Ports` carrying `.status.loadBalancer.ingress[*].ip|hostname`,
  joined by commas when multiple entries exist (same as kubectl's
  default). Empty when no ingress controller has written status —
  clusters without a controller will see the same empty cell that
  `kubectl get ingress` shows.

## [v1.5.6] - 2026-05-29

Bugfix patch — single UI alignment fix, no feature change.

### Fixed

- **Pod status color disappeared after exiting a Pod → containers
  drill-down.** Exiting container view rebuilt the pod table from raw
  `item.Row` instead of the helm-augmented form. `ColumnsForResource(Pods)`
  reserves index 1 for the helm marker, so raw rows shifted Status one
  column left — `stylizeCell` then read the wrong cell and the
  `Running` / `Pending` / `Error` coloring fell silent until the user
  switched resources. Both exit branches (container-level pop, resource
  drill-stack pop) now go through `augmentRowsWithHelm`. Regression
  guarded by `TestAppModel_ExitDrillDownFromContainers_RowsStayHelmAligned`.

### Changed

- README license badge pinned to a static `GPL-3.0` shield instead of
  shields.io's dynamic `github/license/*` endpoint, which has been
  intermittently returning "Unable to select next GitHub token from pool"
  for the past week. No user-visible change when shields.io is healthy.

## [v1.5.5] - 2026-05-29

Debug visibility. Events tab and a new Conditions tab carry the
"why is this resource sad" story for every workload kind — no more
jumping back to `kubectl describe`. Events aggregation now walks the
full controller chain (CronJob → Jobs → Pods, Deployment → current
ReplicaSet → Pods, StatefulSet/DaemonSet/Job → Pods). Panel 2 menu
gains Enter (drill) and Esc (back) entries for discoverability. Logs
auto-follow indicator switched from a ▼ marker to coloring the active
`[Logs]` label green.

### Added

- **Conditions tab.** New detail-panel tab showing the resource's
  `.status.conditions` as a `TYPE / STATUS / REASON / MESSAGE / AGE`
  table — same content as the Conditions section of `kubectl describe`.
  Status `False` rows highlighted in red. Tab appears for kinds that
  populate conditions: Pod / Node / PVC / Deployment / StatefulSet /
  DaemonSet / Job / HorizontalPodAutoscaler / Ingress. Hidden for
  kinds without conditions (ConfigMap, Secret, Service, ServiceAccount,
  etc.). Why it matters: events expire after the cluster's TTL
  (default 1h), but conditions reflect the resource's current state
  — `PodScheduled: False, Insufficient cpu` stays visible until the
  Pod is actually scheduled.
- **Aggregate child events for all workload kinds.** Selecting a
  workload's row and switching to the Events tab now merges events
  from the workload itself AND its child Pods, sorted newest first.
  The Object column distinguishes source kind so you see the chain
  inline. Covers Deployment (via current ReplicaSet), StatefulSet,
  DaemonSet, Job, ReplicaSet, CronJob. Mirrors the existing aggregate-
  logs pattern; same `PodsForWorkload` helper drives both.
- **CronJob 3-tier aggregate.** CronJob's Events tab additionally
  pulls in events from every owned Job (so you see Job-level
  `SuccessfulCreate` / `BackoffLimitExceeded` / `Completed` alongside
  the CronJob's `SuccessfulCreate` / `MissingJob` and the Pods'
  `Scheduled` / `Pulled` / `Started`). Three layers in one view —
  the killer feature for "why did last night's cron fail" debug.
- **Conditions tab Space hint.** Same cheatsheet pattern as Logs /
  Events tabs — `j/k/u/d/gg/G/y/z` for scroll/copy/expand.
- **Panel 2 menu Enter (drill) entry.** When the selected kind
  supports drill-down, the per-row `Space` menu now ends with an
  `Enter ↘` row whose hint names the child kind ("drill into pods" /
  "drill into containers" / etc.). Cursor + Enter on it triggers
  the same drill as pressing Enter on the row directly — visible
  in the menu for discoverability.
- **Panel 2 menu Esc (back) entry.** When the table is inside a drill
  chain (e.g. you pressed Enter on a Deployment and are now viewing
  its Pods), the menu appends an `Esc ↖ back to parent list` row.
  Cursor + Enter triggers `exitDrillDown`.
- **Container menu Esc entry.** Same back row appended to the Pod →
  containers context menu (Shell + Esc).
- **Panel 3 bottom-left hint.** On the Relatives tab at depth > 1,
  panel 3's border shows `esc: back` in the bottom-left, mirroring
  panel 2's `.: toggle helm` pattern. Surfaces the pop-one-level
  shortcut without requiring the help popup.

### Changed

- **Logs follow indicator.** Active `[Logs]` tab label rendered in
  Status.Running green when auto-follow is engaged. Replaces the
  prior `▼` text marker. Same semantic ("alive stream") expressed
  visually instead of textually.
- **Popup bottom hints trimmed.** All popup border bottom legends
  (panel 2 menu, helm doc menu, breadcrumb, hint popup, app log,
  YAML popup, confirm, namespace picker, context picker) now show
  just `Space: close` / `Space: cancel`. The Esc / q / n / ! keys
  still work, just no longer advertised. Reflects the v1.5.x
  mental model: Space is the primary popup verb.
- **CronJob added to demo fixtures.** `.local/demos/demo-app.yaml`
  now includes a `demo-cron` CronJob firing every minute (busybox
  heartbeat) for verifying the 3-tier aggregate path locally.

### Fixed

- **Stale-events workaround framing.** Empty Events tab on a healthy
  resource is no longer the only signal — the new Conditions tab
  fills the diagnostic gap when events have expired past TTL.

## [v1.5.4] - 2026-05-28

Universal Space coverage. v1.5.3 closed panel 1; v1.5.4 closes the
remaining no-op corners: panel 3 Logs / Events / Relatives-at-depth-1
tabs each get their own cheatsheet, and empty panel 2 surfaces an
explainer instead of swallowing the keypress.

### Added

- **Panel 3 tab cheatsheets via Space.** Logs / Events tabs each get a
  read-only popup listing j/k/u/d/G/y/z (scroll, copy, full-screen).
  Relatives tab at depth=1 (no drill chain yet) shows a drill-into
  cheatsheet (Enter to drill, Y for YAML, Esc to pop). At depth>1 the
  existing breadcrumb popup still opens.
- **Empty panel 2 Space hint.** When the table is empty (e.g., a
  namespace with no Deployments), Space no longer no-ops — it opens a
  popup naming the likely cause (filter on, helm-managed hidden, wrong
  ns/context) and the keys that fix each.

### Changed

- **SidebarHelpPopupModel → HintPopupModel.** Refactored the v1.5.3
  sidebar-only popup into a generic title + rows model. One instance
  serves all six call sites (sidebar, container drill — via separate
  panel 2 menu — Logs / Events / Relatives-depth-1 / empty panel 2).
  No user-visible change beyond more places Space works.

## [v1.5.3] - 2026-05-27

Closes the "Space surfaces what's possible here" loop. v1.5.1 wired it
on panel 2 and panel 3; v1.5.2 added it on container drill; v1.5.3
brings it to panel 1 too, so the rule is now universal: anywhere the
user can land focus, `Space` shows what they can do.

### Added

- **Panel 1 (sidebar) Space cheatsheet.** Sidebar rows are nav targets,
  not action targets — so a per-row menu wouldn't make sense. Instead
  Space opens a read-only popup listing the keys that drive the sidebar
  (j/k move, Enter focus, / search, search-mode Enter to lock, Esc to
  clear, N/C global pickers). Search-mode Enter/Esc are visually nested
  under `/` with the same drill-into arrow used in the Relatives tab.
  Esc/q/Space/Enter all close. Long hints wrap onto continuation lines
  with the key column left empty.

### Changed

- **`Space` scope in README now spelled out per panel.** Three-key blurb
  + Key Bindings table both updated to enumerate which popup opens on
  which panel (sidebar cheatsheet / panel 2 menu / Relatives breadcrumb).

## [v1.5.2] - 2026-05-27

Dual-slot PTY + status bar styling pass. The headline fix: Alterm
(persistent embedded shell) can now coexist with `kubectl exec` /
`kubectl edit` — previously a hidden Alterm shell blocked any new PTY,
forcing the user to exit it. Container drill gets a Space menu too, so
the v1.5.1 "Space = right-click menu" model now reaches the bottom of
the drill chain.

### Added

- **Dual-slot PTY architecture.** Split the single `m.ptyView` into
  `m.shellPty` (Alterm, persistent) and `m.txPty` (kubectl edit / exec,
  transient). They run independently — hide Alterm in the background,
  then exec into a container without closing the shell session. Render
  layers tx on top of shell; input routing prefers tx; tick + exit
  messages carry `Kind` so the right slot cleans up.
- **Container drill Space menu.** Pressing `Space` while drilled into a
  pod's container list now opens a single-item menu (`Shell`) instead
  of doing nothing. `S` direct hotkey on the container row still works
  the same way — the menu just surfaces it so the v1.5.1 "Space =
  contextual menu" rule applies at every drill depth.

### Changed

- **Status bar text labels → Nerd Font icons.** `ctx:` / `cluster:` /
  `ns:` replaced with `\U+F0237` / `\U+F1856` / `\U+F51E` — more compact,
  matches the Alterm chip style. Hidden Alterm chip color synced to
  the popup border (`#F0AE49`).
- **PTY popup borders tri-color.** With two PTYs able to coexist, each
  kind needs its own border so the active popup's provenance is
  unambiguous: Alterm shell stays Catppuccin peach `#F0AE49`, `kubectl
  exec` switches to green `#a6e3a1`, `kubectl edit` keeps sky blue
  `#74c7ec`. Title (bold) shares each popup's border color.

### Fixed

- **Hidden Alterm no longer blocks edit/exec.** Old single-slot guard
  refused `startShellExecMsg` and `startEditMsg` whenever any PTY was
  alive — including a backgrounded Alterm shell. Guards now check only
  the txPty slot, so the persistent shell stays out of the way.

## [v1.5.1] - 2026-05-27

The keybinding UX pass. v1.5.0 shipped Helm; v1.5.1 steps back and
collapses the accumulated key-binding choices into one consistent mental
model. Four navigation keys with disjoint meanings:

- **`Enter` = into** — the sole drill / focus / commit-popup key
- **`Space` = right-click menu** — opens contextual menus, mirror-closes popups
- **`h`/`l` = panel 3 tab switch** — only when panel 3 is active
- **`Esc` = back** — pop drill frame / close popup (LIFO)

Trigger letters (`Y`/`E`/`S`/`D`/`N`/`C`) are uppercase so they require
Shift — the modifier exists to prevent accidental key presses, not to
signal danger. The mental-model anchor is a desktop GUI analogue: Enter
= double-click, Space = right-click.

### Added

- **Per-row context menu on panel 2.** Press `Space` on a regular row
  to open a menu listing `YAML(Y)` / `Edit(E)` / `Shell(S)` / `Delete(D)`.
  Items are resource-aware: Shell is hidden for resources without
  containers (Service / ConfigMap / Secret / ...). Items can be committed
  via cursor + `Enter` or by hitting the letter directly while the menu
  is open. Trigger closes the menu either way — three paths (direct
  hotkey at row / menu + cursor / menu + hotkey) reach the same final
  state.
- **Rule A read-only — Helm-managed Delete.** Pressing `D` on a Helm-
  managed K8s resource (label `app.kubernetes.io/managed-by=Helm` or
  annotation `meta.helm.sh/release-name`) now surfaces a "Helm-managed
  (read-only)" toast and refuses, matching `E` edit. Closes the v1.5.0
  leak where `D` skipped the guard.
- **`z` toggle expand panel.** Single key toggles the focused panel
  (table or detail) between expanded and normal — replaces the
  `=`/`-` pair, which were ergonomically awkward (different keys for
  open vs close).

### Changed

- **Trigger letters uppercase across the board.** `e` → `E` (edit),
  `s` → `S` (shell). Aligns with `D`/`Y`/`N`/`C` and the Shift = anti-
  accidental modifier rule. Also affects the YAML popup's edit hotkey
  (now `E`).
- **`n`/`c` lowercase aliases removed.** Namespace / context pickers
  only respond to `N` / `C` now — the lowercase aliases were a leak
  in the Shift = intentional rule.
- **`h`/`l` purely panel 3 tab switch.** Previously `h`/`l` switched
  the panel 3 tab while panel 2 was focused; now they only fire when
  panel 3 is the active panel. Panel 2 = pure list (cursor + Enter +
  Space). Panel 3 = tab navigation (`h`/`l`).
- **`l` retired as a drill key.** Enter is now the sole drill / focus-
  next-panel key throughout km8 (sidebar `l`/`Enter` → `Enter` only;
  Relatives tab `Enter`/`l` → `Enter` only).
- **`h` retired as drill-frame pop.** `Esc` was always an alias; now
  it's the only key for back-out.
- **`b` key retired.** The breadcrumb popup is now reachable via
  `Space` on the Relatives tab — folds into the universal "Space =
  open menu" rule. The `[b]readcrumbs` panel-border hint is removed.
- **Breadcrumb popup: `Enter` commits, `Space` closes.** Inside the
  popup, `Enter` now commits the cursor row as a panel 1+2 switch
  (replaces the old jump-to-drill-level behavior). `Space` mirrors
  open and closes the popup without committing.
- **Any menu popup: `Space` = close** (mirror open). Breadcrumb popup,
  Helm doc menu, per-row context menu — uniform rule. Confirm dialogs
  already accepted Space as cancel; behavior unchanged.
- **Status line trimmed.** Now shows only `?` / `q` / `N` / `C` /
  `space` / `enter` — plus `/` filter when panel 1/2 is active
  (hidden on panel 3 since in-panel search was retired in v1.5.0).
  Trigger letters (`Y`/`E`/`S`/`D`) live in the per-row context menu
  instead of being duplicated on the status bar.
- **Help popup (`?`) rewritten** as the single source of truth for the
  full key map under the new mental model.

### Mental model summary

| Key | Meaning |
|---|---|
| `Enter` | Into — drill / focus next / commit popup |
| `Space` | Right-click menu — open context menu / mirror-close popup |
| `h`/`l` | Tab switch (panel 3 active only) |
| `Esc` | Back — pop drill / close popup |
| `Y`/`E`/`S`/`D`/`N`/`C` | Triggers, uppercase = needs Shift = anti-accidental |
| `j`/`k`/`u`/`d`/`gg`/`G` | Vim navigation (unchanged baseline) |

### Fixed

- **Table cell truncation no longer slices UTF-8 mid-codepoint.** The
  table renderer used byte-length truncation (`val[:w-1]`), which broke
  any multi-byte cell whose byte count exceeded the column width — the
  Nerd Font helm glyph (3 bytes / 1 cell) rendered as `◇◇` in a 2-cell
  column. Now uses visual-width truncation via `ansi.Truncate` +
  `lipgloss.Width`, so any multi-byte content survives narrow columns
  intact.
- **Pod STATUS color column-index lookup is dynamic.** The hard-coded
  `colIdx == 2` check broke when the helm-marker column was inserted at
  index 1; STATUS color stopped applying. Status column lookup is now
  by column title, not position.

### Changed (polish)

- **Helm-managed visibility defaults to shown.** Previously hidden by
  default with the rationale that helm objects are "noise" on a scout
  workflow. Reverted — the cluster's actual contents should be the
  default surface. `.` on any non-Releases panel 2 list toggles hide.
- **Helm marker column on every resource type.** A dedicated unlabeled
  column right after Name shows the `` (Nerd Font nf-dev-helm) glyph
  on helm-managed rows, blank otherwise. Same glyph used for popup
  title icons. Previously only Secrets list filtered helm storage blobs;
  this universalizes the visual signal across all 26 resource types.
- **Panel 2 bottom-left always shows the `.: toggle helm` hotkey hint.**
  Previously the chip only appeared when helm filter was off, as a
  state indicator. Now it's a permanent hotkey advert (Releases panel
  is the only exception — toggle is a no-op there).
- **Space closes every popup uniformly.** YAML popup, Help popup, App
  log popup, Splash screen all now accept `Space` to close, matching
  the universal "Space mirror-closes the popup that opened" rule from
  the v1.5.1 mental model.
- **README rewritten around zero-learning-curve framing.** Three keys
  (`Enter` / `Space` / `Esc`) cover the primary interaction; layout
  navigation (`1`/`2`/`3`, `h`/`l`) and accelerators (`Y`/`E`/`S`/`D`/...)
  are framed as optional. Honest about where `Space` works vs not
  (sidebar has no per-row menu — every row is itself a navigation
  target).

### Demo

- 5 demo gifs re-recorded against the v1.5.1 mental model:
  `demo-basics` (three-key tour + Space menu → Y → YAML),
  `demo-relatives` (chain drill + Space breadcrumb popup + confirm
  switch), `demo-yaml-edit` (Space menu → Edit → confirm → vim, the
  v1.5.1-correct path to `kubectl edit`), `demo-helm` (new — Space
  doc menu → Manifest YAML popup), `demo-alterm` (two scale cycles
  showing hide/show persistence).

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
- **Hidden Alterm chip relabeled `Alterm`** (was `alterm` lowercase)
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
the cursor. Two Alterm/drill bug fixes from v1.3.0 hotfixes promoted in.

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

- **Alterm: Alt+letter / Shift+Tab / Ctrl-arrows / F-keys forwarded
  to the embedded shell.** `ptyKeyBytes` was dropping these — zsh
  hotkeys like `Alt+.` / `Alt+f` / `Alt+Backspace`, Shift+Tab reverse
  completion, Ctrl+Left/Right word jump, and F1–F12 all silently
  no-op'd inside Alterm. Now they serialize to the right escape
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
shell (Alterm), aggregate Deployment logs, a full-screen `Y` YAML
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
- **Persistent Alterm (`Alt+t`).** The embedded shell survives
  visibility toggling. First `Alt+t` spawns it; subsequent presses
  hide / show while cwd, history, env vars, and background jobs all
  persist. Status bar carries a chip in the `ns:` row showing state —
  green `attached` while visible, peach `alterm` while hidden. Shell
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

- **Panic on quit when Alterm was hidden.** `Stop()` nil'd `p.cmd`
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
- **Alterm hidden status-bar marker uses peach (`#fab387`).** The
  previous yellow was identical to the `ns:` text; the new color
  matches the panel-border palette and is unambiguous.
- **`Alt+t` hint everywhere is lowercase.** The keymap is
  case-sensitive; help / status line / Alterm border hints now match
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
- **Alterm — embedded shell terminal** (`T` key). Opens the user's login shell (`$SHELL -l`, fallback `/bin/sh`) inside a PtyView popup with the user's full env and cwd intact — essentially `ssh localhost` embedded in km8. The popup title shows the short hostname (`.local` mDNS suffix stripped) so it's clear which machine you're connected to. Solves the "I need to drop out of km8 to run `kubectl apply -f foo.yaml`" friction without re-implementing every kubectl verb inside the TUI.
- **PTY scrollback** — 10,000-line ring buffer captures every output line that flows through any PtyView popup (Alterm, `s` shell exec, `e` edit). Navigate with `PgUp` / `PgDn` (page) and `Home` / `End` (top / live). Typing any other key snaps the view back to live. ANSI color codes are preserved so the rendered history looks exactly like the live output. Scrollback automatically resets when the subprocess clears the screen (`clear` / `\x1b[2J` / `\x1b[H\x1b[J` / `\x1b[3J` / `\x1bc`).
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
