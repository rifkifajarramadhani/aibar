# aibar — Waybar AI Usage Monitor

## Purpose

`aibar` is a single long-lived Go daemon that displays timed AI usage limits in
Waybar. The target providers are:

- Claude / Claude Code
- Codex / OpenAI
- Cursor

Each provider may expose a five-hour rolling window and a weekly window. The
bar should make the most constrained available window visible while the
tooltip provides the complete provider/window breakdown.

The first implementation milestone intentionally focuses on Codex, because
Codex writes authoritative rate-limit data into local rollout files and can be
implemented without network requests.

## Design principles

These constraints are architectural requirements:

1. Never block Waybar on a network request. A hung HTTP call must not freeze
   the bar.
2. Local sources are the realtime path. Network requests are reconciliation,
   not the primary UI driver.
3. Respect provider rate limits. Anthropic usage polling must not be more
   frequent than approximately every 300 seconds.
4. Degrade visibly. Keep last-good data, but mark stale/auth/error states
   explicitly rather than displaying old values as fresh.
5. There is no hover-fetch mechanism in a Waybar custom module. Tooltip content
   is emitted with the last JSON line; click-to-refresh is the interaction
   substitute where needed.

## Runtime architecture

```text
Codex fsnotify watcher ─┐
                        │
future provider watches ├─> snapshot channel ─> merged state ─> renderer ─> stdout
                        │                         │                  │
future network fetches ┘                         └─> state.json       └─> Waybar

SIGUSR1 / control socket ───────────────────────────────> refresh/action
```

The daemon is launched by Waybar:

```jsonc
"custom/aibar": {
  "exec": "aibar daemon",
  "restart-interval": 1,
  "return-type": "json",
  "tooltip": true
}
```

It holds stdout open and emits one compact JSON line when the rendered state
changes. A one-second render ticker updates reset countdowns and age text
without performing I/O or network work in the render path.

## Data sources

| Provider | Realtime/local source | Reconciliation source | Minimum network interval |
| --- | --- | --- | --- |
| Codex | `~/.codex/sessions/**/rollout-*.jsonl` with server `rate_limits` | None required | n/a |
| Claude | `~/.claude/projects/**/*.jsonl` token deltas | Claude Code OAuth usage endpoint | 300 seconds |
| Cursor | None currently available | Cursor dashboard endpoint using local session cookie | 300 seconds |

### Codex

Codex rollout events use the shape:

```json
{
  "type": "event_msg",
  "payload": {
    "type": "token_count",
    "rate_limits": {
      "primary": {
        "used_percent": 1,
        "window_minutes": 10080,
        "resets_at": 1784534584
      },
      "secondary": null
    }
  }
}
```

The parser classifies windows by duration rather than assuming that
`primary` is always 5h:

- `300` minutes → `5h`
- `10080` minutes → `weekly`

Missing or unknown windows are ignored. A current account may expose only the
weekly window.

### Claude

Claude reads the OAuth access token from `~/.claude/.credentials.json` and
uses the undocumented `https://api.anthropic.com/api/oauth/usage` endpoint as
the authoritative usage source. The provider accepts `five_hour` and
`seven_day`/`weekly` windows, supports both `utilization` and
`used_percentage`, and keeps the endpoint parser isolated for schema changes.

Claude Code project JSONL files are watched for assistant usage changes. Those
changes trigger a debounced refresh request only; local token counts are not
converted into an estimated account percentage. A successful network fetch
replaces the last anchor. A 429 honors `Retry-After`, all attempts respect the
five-minute minimum interval, and failures preserve last-good data. Optional
`Stop` and `SessionEnd` hooks can signal the daemon after a session completes.

### Cursor

Cursor has no supported individual-account usage API. The planned provider
reads a manually captured `WorkosCursorSessionToken` from a permissions-
restricted secrets file or libsecret. Cookie expiry is expected and must be
reported as `auth-error` with recovery instructions; it must not affect other
providers.

## Core contracts

The provider abstraction is intentionally shared by local and network-backed
providers:

```go
type Window struct {
    Label    string
    UsedPct  float64
    ResetsAt time.Time
}

type Snapshot struct {
    Provider  string
    Windows   []Window
    FetchedAt time.Time
    Source    Source
    Err       error
}

type Provider interface {
    Name() string
    Fetch(context.Context) (Snapshot, error)
    MinInterval() time.Duration
    Watch(context.Context, chan<- Snapshot) error
}
```

Merge rules:

- A network snapshot supersedes the current provider snapshot.
- A local snapshot supersedes a network anchor only when it is newer.
- An error keeps the last-good windows and adds visible error/stale state.
- A local-only provider such as Codex is not marked stale merely because its
  `MinInterval` is zero; watcher/parser failure is the meaningful stale signal.

## Waybar contract and UX

The daemon emits:

```json
{"text":"󰚩","tooltip":"...","class":"ok","percentage":6}
```

The visible bar uses the compact AI/robot icon. Hovering it shows a grouped
plain-text usage card containing:

- every available provider and window in stable order;
- `Rolling Usage` and `Weekly Usage` labels for the supported windows;
- a fixed-width 20-cell `#`/`-` usage bar and whole-number percentage;
- the reset countdown below each bar;
- provider-scoped stale/auth status when applicable.

CSS states are:

- `ok`: below 75%;
- `warning`: 75% or higher;
- `critical`: 90% or higher;
- `stale`: last-good data is accompanied by a current error.

Current live Waybar placement keeps the original hover-to-reveal tray drawer:

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

The aibar module is not listed directly in `modules-right`; it is a child of
the tray group. Its current CSS spacing is:

```css
#custom-aibar {
  margin: 0 16px 0 -4px;
}
```

## Commands

```sh
aibar daemon
aibar refresh
aibar next-provider
aibar prev-provider
aibar cycle-window
```

The daemon stores its PID, private Unix control socket, and last-good state
under `~/.cache/aibar/`. State is written atomically with restrictive file
permissions.

## Implementation phases

### Phase 0 — skeleton

- Standalone Go module and command entrypoint.
- Waybar JSON output and one-second render ticker.
- Fake/static provider or fixture-driven renderer before network code.

### Phase 1 — Codex provider

- Resolve the newest rollout file.
- Watch session directories and file rotation with fsnotify.
- Parse the last complete `token_count` event.
- Emit authoritative 5h/weekly local snapshots.

### Phase 2 — daemon plumbing

- Central snapshot channel and merged state.
- Restart-safe last-good state persistence.
- SIGUSR1 refresh handling.
- Control commands for provider/window actions.

### Phase 3 — Claude provider

- Read-only OAuth credential loading.
- 300-second usage polling with defensive parsing and backoff.
- Local token-usage watcher used as a refresh trigger.
- Opt-in Claude Code refresh hook documentation.

### Phase 4 — Cursor provider

- Restricted cookie/secrets loading.
- 300-second dashboard polling.
- Explicit auth-error state and recovery tooltip.

### Phase 5 — UX polish (implemented)

- Multi-provider constrained-window selection.
- Full tooltip grid and pacing indicator.
- Provider/window pinning and scroll actions.
- Theme-safe styling through standard Omarchy foreground variables.

### Phase 6 — distribution

- Static release binary.
- PKGBUILD/AUR packaging.
- Credential setup and undocumented-endpoint warnings in the README.

## Security and failure handling

- Never commit credentials.
- Never put Cursor cookies in Waybar `config.jsonc`.
- Use `~/.config/aibar/secrets.env` with mode `0600` or libsecret for Cursor.
- Treat undocumented endpoint/schema changes as expected provider failures.
- Keep providers isolated so one provider cannot blank the whole bar.
- Preserve last-good state across daemon crashes and Waybar restarts.

## Prior art

Useful references for future phases:

- `gelzinn/ai-status` — daemon, provider plugin, and Waybar restart pattern.
- `steipete/CodexBar` and `codexbar-waybar` — provider coverage and fallback
  behavior.
- `ccusage` — Claude JSONL/token-window parsing.
- `mryll/claudebar` — Omarchy theming integration.
