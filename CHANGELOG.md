# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- **Overview tab + Lens-style navigable references**: every detail panel now has an Overview tab as its default (post-YAML-migration tab order; YAML lives in the `Y` popup). For Pods, Overview lists Owner / Node / ServiceAccount / Image as a structured view; `j`/`k` move the cursor between drillable refs and `Enter` opens the referenced resource (`Deployment/nginx`, `Node/worker-3`, `ServiceAccount/nginx-sa`, ...) in the YAML popup. Other resource kinds get a generic Overview fallback (Name + structured fields + Labels + Annotations) so the panel never renders empty. This is km8 finally delivering on the CLAUDE.md tagline "Lens IDE terminal alternative" — graph navigation in a TUI.
- New `k8s.PodOverviewData` and `k8s.RefTarget` carry the navigable refs through the existing `ResourceDetail` so the ui package never needs to parse `*corev1.Pod` directly.
- New `k8s.FetchResourceByRef(ctx, cs, ref)` fetches a single resource by km8 type + name + namespace, used by Overview drill-down. Supports Pods, Deployments, DaemonSets, StatefulSets, Jobs, CronJobs, Nodes, ServiceAccounts, ConfigMaps, Secrets, PVCs, PVs, Services.

### Changed
- **Detail tab order: YAML out, Overview in.** YAML moves entirely to the `Y` popup (introduced earlier in this branch). New defaults:
  - Pod: `Logs` / `Overview` / `Events`
  - Deployment: `Logs` / `Overview` / `Events`
  - Events resource: `Overview` alone
  - everything else: `Overview` / `Events`
  Existing users will notice that pressing `1`/`2`/`3` or `h`/`l` no longer cycles to a YAML tab — use `Y` instead.

### Added
- **Aggregate logs for Deployments**: selecting a Deployment row now streams logs from every pod in its current ReplicaSet into a unified Logs tab (also the default tab for Deployment detail, since "which pod is misbehaving during a rollout" is the most common reason to open a Deployment). Lines are prefixed `<pod-hash>│<container>│<text>` — three independent colors derived from FNV hash + the existing 8-entry Catppuccin palette. Pod-hash is the trailing segment of the pod name (last 5 chars of the random suffix). Cross-stream timestamp sorting is intentionally not attempted — clock skew + jitter would make any ordering misleading; lines arrive in the order kubectl's API returns them. Tab order for Deployment is `Logs` → `YAML` → `Events` (was `YAML` → `Events`). Falls back to the Deployment selector when the current-ReplicaSet lookup fails (e.g. RBAC denies ReplicaSet list).
- **Persistent KM8erm**: `Alt+T` is now the single toggle for the embedded shell — spawn / show / hide. The shell never dies on hide; subsequent `Alt+T` presses cycle visibility while cwd, history, env vars, and background jobs are all preserved. The status bar carries a chip showing the state (right after `ns:`) — green `attached` while the popup is visible, amber `km8erm` while the shell is hidden in the background. The shell is killed cleanly when km8 quits (`q` confirm or `Ctrl+C`). `Alt+T` only applies to KM8erm (Shell-kind PTY); `kubectl edit` and `kubectl exec` popups treat it as a normal keypress because their lifecycle is bounded by the subprocess they wrap. **`T` (uppercase) is no longer bound** — Alt+T replaces it.
- Pressing `e` (edit) or `s` (shell exec) while a PTY is alive (visible or hidden) now refuses with a toast / app log warning instead of clobbering the in-flight subprocess — close the current PTY first (`exit` in the shell, or `Alt+T` then `exit`).
- **`Y` opens a full-screen YAML popup** for the currently-selected resource. Supports `j` / `k` line scroll, `u` / `d` half-page scroll, `gg` / `G` jump to top / bottom, `/` search (`Enter` commits; `n` / `N` step through matches with a full-row highlight on the current match like the panel-view selected row), and `e` to dispatch `kubectl edit` on the same resource directly from the popup (skips the confirm step that the table-level `e` uses — by the time you press `e` here, you've already inspected the YAML). `Esc` / `q` close. The search box border shifts color from cyan (actively editing the query) to amber (filter committed, navigating matches) so the locked state is visible at a glance. Solves the "YAML wall in narrow vertical Panel 3 is hard to read" problem without giving up on having YAML at all.
- **Uppercase aliases for namespace / context pickers**: `N` and `C` open the same pickers as `n` and `c`. Transitional — lowercase will be deprecated later. Rationale: lowercase keys felt too easy to misfire ("any letter pops up a popup"), and `n` / `c` clash with vim mental models (`n` = next-search, `c` = change).

### Internal
- New `k8s.PodTarget` + `k8s.PodsForDeployment` / `k8s.PodsForWorkload` helpers resolve a workload to the pod set whose containers should be streamed. `currentOnly=true` filters by the deployment's `deployment.kubernetes.io/revision` annotation matched against owned ReplicaSets; `false` returns all selector-matching pods for rollout-comparison cases.
- `k8s.LogStreamer.StartMulti([]PodTarget)` is the new entry point for aggregate streams; `Start(podName, namespace, containers)` remains as a single-pod wrapper. `LogLine` gains a `Pod` field so the consumer can multiplex per pod, not just per container.
- `internal/ui` follows: `LogLineMsg` and `DetailModel.AppendLogLine` both gain a `pod` parameter (empty for single-pod streams). `detail.go` gains `podLogColor`, `podHashTag`, and `fnvPaletteColor` helpers; `buildLogLines` branches between two-segment (`<container>│<text>`) and three-segment (`<pod-hash>│<container>│<text>`) prefixes based on whether the source pod is set.
- `aggregateLogsReadyMsg` carries `resource` and `itemUID` so AppModel can ignore stale results when the user navigates to a different row while the pod-list call is in flight.
- New `YamlPopupModel` in `internal/ui/yamlpopup.go` modelled on `HelpModel` / `AppLogModel` (PopupAnimator + content lines + scroll offset + search state). Captures edit target at `Open()` time so `e` knows what to dispatch even after the user has scrolled around.
- New `DetailModel.YAMLContent()` accessor exposes the loaded YAML for popup rendering without leaking the internal `k8s.ResourceDetail` field.
- `NamespacePickerModel` and `ContextPickerModel` now accept the uppercase form as a close-key alias.
- `PtyView` gains `hidden bool` + `kind PtyKind` (Shell / Edit / Exec). `IsActive()` now means "alive AND visible"; new `IsAlive()` reports the underlying subprocess state. `Hide()` is a no-op for Edit / Exec kinds (transient — never hide). `Show(w,h)` re-syncs PTY size before un-hiding so a window resize while hidden doesn't leave the shell rendering at stale dimensions. `Start()` takes a `PtyKind` argument.
- `StatusBarModel.ViewFull(unreadErrors, success, *PtyMarker)` is the new render entry point; `ViewWithBadge` keeps working as a thin wrapper for callers that don't pass a marker.

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
