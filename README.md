# aibar

`aibar` is a small Waybar usage monitor for AI coding tools. The first
milestone implements the local Codex provider: it watches
`~/.codex/sessions/**/rollout-*.jsonl` and reads the server-provided rate-limit
windows without making network calls.

## Build and run

```sh
go build -o aibar ./cmd/aibar
./aibar daemon
```

The daemon keeps stdout open and emits one Waybar JSON object per changed
render. It reloads last-good data from `~/.cache/aibar/state.json` after a
restart. Runtime control uses a private Unix socket under the same directory.

```sh
aibar refresh
aibar next-provider
aibar prev-provider
aibar cycle-window
```

`next-provider` and `prev-provider` are safe no-ops until the Claude and Cursor
providers are added. `refresh` also works through `SIGUSR1` for hooks and other
local integrations.

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

## Security and roadmap

The Codex-first milestone does not read or store credentials and does not use
the network. Future Claude and Cursor providers will read existing local
credentials only; credentials must never be committed or placed in Waybar's
world-readable configuration.

Claude's usage endpoint and Cursor's dashboard endpoints are undocumented and
may change or disappear. They will be isolated behind the provider interface,
with last-good state and visible stale/auth-error status preserved when they
break.
