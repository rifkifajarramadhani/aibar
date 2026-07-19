# aibar Progress

Last updated: 2026-07-19

## Current status

### 2026-07-19 — Clean Architecture refactor

The package layout was restructured into domain-first Clean Architecture,
mirroring the `golang-clean-architecture` reference project, with no change to
runtime behavior or the Waybar output (verified byte-for-byte against the
`testdata/` fixtures). Summary of the move:

- Core domain consolidated into `internal/usage` (entities, `Store` merge policy,
  `View`, and the `Provider`/`Refreshable`/`SnapshotArchive` ports). The former
  `internal/model` and the in-memory half of `internal/state` were folded in.
- Application loop kept in `internal/daemon`, now an injected `Daemon` struct that
  consumes `Renderer`/`ControlServer` ports (defined there) and owns the
  control-command vocabulary. `bootstrap.WireDaemon` constructs its collaborators.
- Providers moved to `internal/adapter/{codex,claude}`; the renderer to
  `internal/adapter/waybar`; the control socket to `internal/adapter/control`; the
  `state.json` persistence split out to `internal/adapter/statefile`
  (implementing `usage.SnapshotArchive`). Added `internal/adapter/logging` (slog),
  `internal/config`, and `internal/bootstrap`.
- Layering is enforced by `depguard` in a new `.golangci.yml`; a `Makefile`
  provides `make check`. New doc playbooks added under `docs/`
  (`ARCHITECTURE.md`, `PATTERNS.md`, `STYLE_GUIDE.md`, `LINTING.md`).
- All existing tests moved with their packages and pass under `-race`; the
  statefile round-trip and permission tests replace the old `state` persistence
  tests, and the store merge tests live with `internal/usage`.

The Codex and Claude vertical slices plus the multi-provider UX are
implemented. The daemon reads local Codex rollout files, reads Claude Code
OAuth credentials when configured, polls Claude usage no more than once every
five minutes, emits a compact AI icon, preserves last-good usage, and keeps
provider failures isolated.

## Completed

### Project foundation

- Standalone Git repository at `/home/overhaul/Personal/aibar`.
- Go module using `github.com/fsnotify/fsnotify`.
- Command entrypoint with:
  - `daemon`
  - `refresh`
  - `next-provider`
  - `prev-provider`
  - `cycle-window`

### Codex provider

- Recursively discovers the newest `rollout-*.jsonl` under
  `~/.codex/sessions/`.
- Watches session directories and handles newly created date directories.
- Re-resolves the newest file after writes, creates, renames, removes, and
  rotation events.
- Parses the latest complete `event_msg` / `token_count` event.
- Reads `payload.rate_limits.primary` and `.secondary` defensively.
- Maps 300-minute windows to `5h` and 10080-minute windows to `weekly`.
- Handles the observed weekly-only Codex payload where `secondary` is null.
- Does not make network calls.

### Claude provider

- Reads the access token from `~/.claude/.credentials.json` without rewriting
  or refreshing the file.
- Polls the undocumented OAuth usage endpoint with a five-minute minimum.
- Parses 5h and weekly windows with defensive reset and percentage handling.
- Handles auth errors, 429 retry hints, transient backoff, and bounded bodies.
- Watches project JSONL assistant usage and debounces local refresh triggers.
- Includes fixture-backed credentials, parser, HTTP, and watcher tests.

### Daemon and state

- Long-lived stdout JSON stream for Waybar.
- One-second render ticker for countdown/age refreshes.
- Emits only when rendered JSON changes.
- SIGUSR1 local refresh support.
- Private Unix socket for refresh and navigation commands.
- Atomic last-good state persistence at `~/.cache/aibar/state.json`.
- PID/socket/state files use restrictive permissions.
- Provider errors preserve last-good usage data and expose stale state.

### Waybar integration

- Installed binary: `/home/overhaul/.local/bin/aibar`.
- Live config: `/home/overhaul/.config/waybar/config.jsonc`.
- Live stylesheet: `/home/overhaul/.config/waybar/style.css`.
- aibar is inside the original hover-to-reveal `group/tray-expander`:

  ```jsonc
  "modules": ["custom/expand-icon", "tray", "custom/aibar"]
  ```

- `custom/aibar` is not listed directly in `modules-right`.
- Tray reveal remains hover-based; `click-to-reveal` is not configured.
- The visible text is the Nerd Font AI/robot icon `󰚩`.
- Hovering the icon shows all provider windows with fixed-width usage bars,
  whole-number percentages, and reset countdowns.
- Current spacing is `margin: 0 16px 0 -4px`.
- Waybar has been restarted after live configuration changes.

### Multi-provider UX

- Aggregate mode selects the most constrained window across visible providers.
- Scroll navigation pins providers in deterministic order and returns to
  aggregate mode at either boundary.
- Providers with auth/errors remain selectable so their recovery status can be
  inspected directly.
- Middle-click cycles stable window order; selection resets after navigation
  and is not persisted across daemon restarts.
- Tooltips use a screenshot-style grouped layout with provider-scoped stale and
  auth status lines.
- Severity styling remains compatible with themes that expose only
  `@foreground` and `@background`.

### Documentation/assets

- Root README with build, Waybar, security, and roadmap notes.
- Opt-in Claude Code hook documentation.
- Checked-in Waybar snippet.
- Custom SVG asset at `assets/aibar.svg` for future icon/theme use.
- This project overview and progress handoff.

## Validation completed

The following checks pass:

```sh
go test ./...
go test -race ./...
go vet ./...
go build -trimpath -ldflags='-s -w' ./cmd/aibar
```

Additional validation performed:

- Fixture-backed daemon smoke test produced valid Waybar JSON.
- Parser tests cover weekly-only, dual-window, malformed, unknown, and reset
  formats.
- Watcher tests cover initial discovery and newer rollout rotation.
- Claude tests cover credential validation, usage payload parsing, HTTP error
  classification, OAuth headers, local JSONL refresh triggers, and interval
  gating.
- An isolated live daemon smoke test successfully rendered Claude 5h/weekly
  data alongside Codex without exposing credentials in output.
- State tests cover network-anchor precedence, error preservation, and
  save/load round trips.
- Waybar JSON config parses successfully.
- Live Waybar process and aibar daemon were confirmed running.

## Known behavior

- Current Codex data may contain only a weekly window. aibar displays only
  windows actually supplied by Codex.
- The aibar icon is hidden with the rest of the tray children while the tray
  drawer is collapsed. Hover over the existing expand icon to reveal it.
- Provider and window selection is session-only; restarting the daemon returns
  to aggregate mode.
- The SVG icon is checked in, but the live compact rendering currently uses
  the Nerd Font glyph so CSS state colors work reliably.
- Current live Waybar config uses an absolute binary path because
  `~/.local/bin` is not guaranteed to be in Waybar's inherited PATH.

## Remaining work

### Cursor provider

- Add restricted secrets-file or libsecret credential loading.
- Implement dashboard endpoint polling at 300 seconds.
- Add schema-change handling and explicit `auth-error` output.
- Document cookie re-capture and the undocumented API caveat.

### Multi-provider UX

- Extend the current UX to the future Cursor provider without allowing its
  failures to affect Codex or Claude.

### Distribution

- Add static release workflow/build instructions.
- Create PKGBUILD and prepare AUR packaging.
- Expand README credential setup for Claude and Cursor.
- Add explicit warnings that both future network integrations are
  undocumented and may break.

## Recommended next session

Continue with the Cursor provider: restricted credential loading, isolated
dashboard polling, explicit auth-error recovery guidance, and fixture-backed
schema/error tests. Keep undocumented endpoints isolated and treat
schema/auth failures as expected provider-local states.
