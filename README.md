# aibar

`aibar` is a small Waybar usage monitor for AI coding tools. It watches the
local Codex provider and, when Claude Code credentials are available, polls
Claude's OAuth usage endpoint while watching local Claude project files for
refresh triggers.

## Build and run

```sh
go build -o aibar ./cmd/aibar
./aibar daemon
```

The daemon keeps stdout open and emits one Waybar JSON object per changed
render. It reloads last-good data from `~/.cache/aibar/state.json` after a
restart. Runtime control uses a private Unix socket under the same directory.

All runtime paths can be overridden, which is useful for pointing at fixtures
or an isolated runtime during testing:

```sh
aibar daemon \
  --codex-root ~/.codex/sessions \
  --claude-credentials ~/.claude/.credentials.json \
  --claude-projects ~/.claude/projects \
  --state ~/.cache/aibar/state.json \
  --cache-dir ~/.cache/aibar
```

Paths default to the current user's home and XDG cache directories, so the
plain `aibar daemon` needs no flags in a normal installation.

The control subcommands connect to the running daemon over the Unix socket
(`~/.cache/aibar/aibar.sock`, mode `0600`), which also enforces a single
instance — a live socket makes a second daemon exit with "daemon already
running":

```sh
aibar refresh        # rescan all sources now (also triggered by SIGUSR1)
aibar next-provider  # pin the next provider in the tooltip order
aibar prev-provider  # pin the previous provider
aibar cycle-window   # cycle the visible window for the current view
```

The initial view shows the most constrained available window across providers.
Scroll up or down to temporarily pin a provider; reaching either end returns
to aggregate mode. Middle-click cycles that view's windows, while `refresh`
also works through `SIGUSR1` for hooks and other local integrations. View
selection is session-only and resets to aggregate mode after a daemon restart.

## Waybar

The current Omarchy setup keeps the module inside the existing hover-to-reveal
tray drawer. Do not add `custom/aibar` directly to `modules-right`; add it
after `tray` so it appears alongside Cursor, Slack, and Docker when the drawer
is hovered:

```jsonc
"group/tray-expander": {
  "orientation": "inherit",
  "drawer": {
    "transition-duration": 600,
    "children-class": "tray-group-item"
  },
  "modules": ["custom/expand-icon", "tray", "custom/aibar"]
}
```

Then use this module definition:

```jsonc
"custom/aibar": {
  "exec": "/home/overhaul/.local/bin/aibar daemon",
  "format": "{text}",
  "restart-interval": 1,
  "return-type": "json",
  "tooltip": true,
  "on-click": "/home/overhaul/.local/bin/aibar refresh",
  "on-scroll-up": "/home/overhaul/.local/bin/aibar next-provider",
  "on-scroll-down": "/home/overhaul/.local/bin/aibar prev-provider",
  "on-click-middle": "/home/overhaul/.local/bin/aibar cycle-window"
}
```

Hovering the compact icon shows every available provider and usage window.
Each window is rendered with a fixed-width usage bar, a whole-number
percentage, and its reset countdown. Provider names are shown without account
plan labels because those labels are not available from the usage data.

The absolute path matches the current installation because `~/.local/bin` is
not guaranteed to be in Waybar's inherited `PATH`. If the binary is installed
on `PATH`, the shorter `aibar ...` commands are equivalent. The drawer uses
hover-to-reveal behavior; `click-to-reveal` is intentionally not configured.

Suggested CSS:

```css
#custom-aibar {
  min-width: 16px;
  min-height: 16px;
  margin: 0 16px 0 -4px;
  padding: 0;
  font-size: 16px;
}
#custom-aibar.ok           { color: @foreground; }
#custom-aibar.warning      { color: @foreground; opacity: 0.85; }
#custom-aibar.critical     { color: @foreground; font-weight: bold; }
#custom-aibar.stale        { opacity: 0.55; }
#custom-aibar.auth-error   { color: @foreground; font-weight: bold; }

tooltip {
  padding: 8px 10px;
}
tooltip label {
  font-family: 'CaskaydiaMono Nerd Font';
  font-size: 12px;
}
```

After editing Waybar configuration or CSS on Omarchy, apply the changes with:

```sh
omarchy restart waybar
```

The suggested severity styling intentionally uses `@foreground`, opacity, and
font weight so it remains valid for themes that define only the standard
Omarchy foreground/background variables. It does not require `@yellow` or
`@red` to be present.

The current Codex rollout format may expose only a weekly window and may put it
in `primary` while leaving `secondary` null. aibar classifies windows by their
duration and displays only windows that are actually present.

Claude uses the read-only OAuth access token in
`~/.claude/.credentials.json`. The credentials file must remain private
(normally mode `0600`); aibar never refreshes or rewrites it. Missing
credentials leave Claude unconfigured. Malformed or expired credentials are
shown as `auth-error` while healthy providers continue to render.

Claude usage polling uses the undocumented
`https://api.anthropic.com/api/oauth/usage` endpoint with a five-minute minimum
interval. Successful responses provide the `5h` and `weekly` windows. HTTP
429 responses honor `Retry-After`, and transient failures back off while
preserving last-good data. The endpoint may change or disappear.

Local Claude JSONL token usage is used only to request a refresh; token counts
are not converted into an estimated quota percentage. Optional `Stop` and
`SessionEnd` hook configuration is documented in
[`docs/claude-hooks.md`](docs/claude-hooks.md).

## Architecture

aibar follows a domain-first Clean Architecture (full spec in
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)). The core — `internal/usage`
(shared contracts and the merge policy) and `internal/daemon` (the central
select loop) — depends only on the standard library. Everything that touches
the outside world is an adapter under `internal/adapter/` implementing a
core-owned port:

- `adapter/codex` — fsnotify watcher and parser for local Codex sessions.
- `adapter/claude` — OAuth usage polling plus a local-file watcher for refresh
  triggers.
- `adapter/waybar` — the `Renderer` port; formats the Waybar line and tooltip.
- `adapter/control` — the `ControlServer` port; the Unix-socket control plane.
- `adapter/statefile` — the `SnapshotArchive` port; atomic `state.json` writes.

`internal/bootstrap` wires the adapters into the daemon. A single goroutine per
source feeds `usage.Store.Apply`, and a 1-second ticker re-renders without any
I/O so reset countdowns advance while the render path stays free of file and
network work. `depguard` (in `.golangci.yml`) fails the lint if the core
imports an adapter, bootstrap, or config package.

## Development

```sh
make check                 # fmt + lint + test (the pre-submit gate)
go test ./...              # all package tests
go test -race ./...        # race detector for concurrent changes
golangci-lint run ./...    # depguard, errcheck, staticcheck, and friends
```

Each provider keeps all of its parsing and file-watching inside its own adapter
package, and any new provider format ships with a JSONL fixture under
`testdata/<provider>/` and a parser test. See
[`CLAUDE.md`](CLAUDE.md) and [`AGENTS.md`](AGENTS.md) for the full conventions.

## Security and roadmap

The current UX supports aggregate constrained-window selection, temporary
provider pinning, window cycling, complete provider tooltips, and pacing
status calculated from each window's reset time. Cursor support remains the
next provider milestone.

Codex remains local-only. Claude reads its existing local OAuth credential and
uses the undocumented usage endpoint; credentials are never stored by aibar
or placed in Waybar's world-readable configuration. The future Cursor provider
will follow the same isolation rules.

Claude's usage endpoint and Cursor's dashboard endpoints are undocumented and
may change or disappear. They will be isolated behind the provider interface,
with last-good state and visible stale/auth-error status preserved when they
break.
