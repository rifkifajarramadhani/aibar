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

Claude paths can be overridden for isolated testing:

```sh
aibar daemon \
  --claude-credentials ~/.claude/.credentials.json \
  --claude-projects ~/.claude/projects
```

```sh
aibar refresh
aibar next-provider
aibar prev-provider
aibar cycle-window
```

`next-provider` and `prev-provider` remain safe no-ops until the multi-provider
UX phase adds provider pinning. `refresh` also works through `SIGUSR1` for hooks
and other local integrations.

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
```

After editing Waybar configuration or CSS on Omarchy, apply the changes with:

```sh
omarchy restart waybar
```

If your selected Omarchy theme defines `@yellow` and `@red`, those variables
can be used for the warning, critical, and auth-error colors instead.

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

## Security and roadmap

Codex remains local-only. Claude reads its existing local OAuth credential and
uses the undocumented usage endpoint; credentials are never stored by aibar
or placed in Waybar's world-readable configuration. The future Cursor provider
will follow the same isolation rules.

Claude's usage endpoint and Cursor's dashboard endpoints are undocumented and
may change or disappear. They will be isolated behind the provider interface,
with last-good state and visible stale/auth-error status preserved when they
break.
