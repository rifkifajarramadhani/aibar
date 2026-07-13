# aibar Progress

Last updated: 2026-07-13

## Current status

The Codex-first vertical slice is implemented and running in Waybar. The
daemon reads local Codex rollout files, emits a compact AI icon, preserves the
usage tooltip, and runs independently of network availability.

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
- Current spacing is `margin: 0 16px 0 -4px`.
- Waybar has been restarted after live configuration changes.

### Documentation/assets

- Root README with build, Waybar, security, and roadmap notes.
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
- State tests cover network-anchor precedence, error preservation, and
  save/load round trips.
- Waybar JSON config parses successfully.
- Live Waybar process and aibar daemon were confirmed running.

## Known behavior

- Current Codex data may contain only a weekly window. aibar displays only
  windows actually supplied by Codex.
- The aibar icon is hidden with the rest of the tray children while the tray
  drawer is collapsed. Hover over the existing expand icon to reveal it.
- `next-provider` and `prev-provider` are safe no-ops until multiple providers
  exist.
- The SVG icon is checked in, but the live compact rendering currently uses
  the Nerd Font glyph so CSS state colors work reliably.
- Current live Waybar config uses an absolute binary path because
  `~/.local/bin` is not guaranteed to be in Waybar's inherited PATH.

## Remaining work

### Claude provider

- Read Claude Code OAuth credentials from the existing local credential file.
- Implement defensive usage endpoint parsing for 5h and weekly windows.
- Enforce a 300-second minimum network interval.
- Add 429 handling and backoff without retry storms.
- Watch Claude project JSONL files for local token deltas.
- Add Stop/SessionEnd hook documentation and integration.

### Cursor provider

- Add restricted secrets-file or libsecret credential loading.
- Implement dashboard endpoint polling at 300 seconds.
- Add schema-change handling and explicit `auth-error` output.
- Document cookie re-capture and the undocumented API caveat.

### Multi-provider UX

- Merge Claude and Cursor snapshots without allowing failures to affect other
  providers.
- Select the most constrained window across providers.
- Add provider pinning and meaningful scroll behavior.
- Add pacing indicator arithmetic.
- Expand tooltip into the complete provider/window grid.
- Integrate theme colors from the active Omarchy theme.

### Distribution

- Add static release workflow/build instructions.
- Create PKGBUILD and prepare AUR packaging.
- Expand README credential setup for Claude and Cursor.
- Add explicit warnings that both future network integrations are
  undocumented and may break.

## Recommended next session

Start with the Claude provider, but first inspect the exact current
`~/.claude/.credentials.json` shape and the local project JSONL token records.
Keep all endpoint parsing behind a provider-specific package and preserve the
existing daemon/state/render contracts. Add fixtures and parser tests before
enabling any live network polling.
