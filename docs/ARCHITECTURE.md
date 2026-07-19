# Go Architecture Playbook

aibar uses domain-first Clean Architecture. Source dependencies point from the
command and adapters toward the core domain. The daemon never blocks Waybar on
I/O, and one provider's failure never blanks the whole bar.

## Package Map

- `cmd/aibar` is the single binary. Its `daemon` subcommand owns the process
  lifecycle (signals, logger) and wires dependencies through `internal/bootstrap`;
  the `refresh`, `next-provider`, `prev-provider`, and `cycle-window` subcommands
  are control clients that talk to a running daemon over the Unix socket.
- `internal/usage` is the core domain: the `Window`/`Snapshot`/`Source` entities,
  the `Store` merge policy, the `View` selection state, and the ports every
  collaborator implements (`Provider`, `Refreshable`, `SnapshotArchive`).
- `internal/daemon` is the application service: the central select loop that
  merges snapshots, renders the Waybar line, and handles control commands. It
  owns the `Renderer` and `ControlServer` ports and the control-command vocabulary.
- `internal/adapter/codex` and `internal/adapter/claude` are outbound provider
  adapters implementing `usage.Provider`/`usage.Refreshable`. Codex is local-only
  (fsnotify over rollout files); Claude is network-authoritative with a local
  watcher used as a refresh trigger.
- `internal/adapter/waybar` is the presentation adapter implementing the daemon's
  `Renderer` port (Waybar JSON, class thresholds, tooltip grid, navigation).
- `internal/adapter/control` is the inbound adapter implementing `ControlServer`:
  the private Unix socket, the single-instance guard, and the runtime files.
- `internal/adapter/statefile` is the persistence adapter implementing
  `usage.SnapshotArchive`: atomic, `0600` writes of `state.json`.
- `internal/adapter/logging` builds the stderr `slog` logger.
- `internal/bootstrap` assembles the adapters into the daemon. It owns no process
  resources.
- `internal/config` resolves the runtime paths from the home and cache directories.

## Dependency Rules

- Core capabilities (`internal/usage`, `internal/daemon`) may depend on the
  standard library and other core capabilities only.
- Core capabilities must not import `internal/adapter`, `internal/bootstrap`, or
  `internal/config`.
- Adapters may import core capabilities and third-party frameworks (`fsnotify`).
- The command creates the logger and cancellation context and runs the daemon
  that bootstrap returns.
- Interfaces are defined by the package that consumes them and remain small.
  `Provider`/`Refreshable`/`SnapshotArchive` live in `internal/usage`;
  `Renderer`/`ControlServer` live in `internal/daemon`. Adapters satisfy them
  structurally and assert it with `var _ Port = (*Impl)(nil)`.
- Contexts enter at the command and propagate through all I/O.

These rules are enforced by the `depguard` configuration in `.golangci.yml`.

## Data Flow

```text
codex fsnotify watcher ─┐
                        │
claude network fetch ───├─> snapshot channel ─> usage.Store (merge) ─> waybar renderer ─> stdout
                        │                            │                        │
(future providers) ─────┘                            └─> statefile (state.json) └─> Waybar

SIGUSR1 / control socket ─────────────────────────────────────────────> refresh / navigate
```

1. The `daemon` subcommand loads configuration, builds the logger, and calls
   `bootstrap.WireDaemon`, which binds the control socket and returns the daemon.
2. Each provider adapter streams `usage.Snapshot` values into the daemon's
   snapshot channel; the `usage.Store` merges them under the anchor/last-good
   policy and persists good snapshots through the `SnapshotArchive` port.
3. The daemon renders through the `Renderer` port and writes the line to stdout
   only when it changes. A one-second ticker re-renders so reset countdowns
   advance without any I/O in the render path — this is a hard rule.
4. Control commands and `SIGUSR1` drive on-demand refreshes and view navigation
   through the same loop; `next-provider`/`prev-provider` are wired but inert in
   the current milestone.

## Design Rules

- Never block Waybar on a network request, and never do file or network work in
  the render path. Local sources are the realtime path; network requests are
  reconciliation.
- Keep providers isolated. All parsing and watching for a provider lives in its
  adapter package. An error keeps the last-good windows and surfaces an explicit
  stale/auth/error state instead of blanking the bar.
- Classify Codex windows by duration, never by position: `300` → `5h`,
  `10080` → `weekly`. Unknown windows are ignored.
- Runtime files (socket, PID, state) live under the cache directory with
  restrictive (`0600`/`0700`) permissions. Make no network calls and read no
  credentials outside the provider adapters that own them.

## Testing

- Core packages use deterministic fakes and table-driven tests; the store's
  merge policy and the view selection are tested without any I/O.
- Adapters test translation and error mapping: the Codex/Claude parsers, the
  Waybar renderer's Waybar contract, and the statefile round-trip and permissions.
- Providers add a JSONL fixture under `testdata/<provider>/` plus a parser test
  for any new rollout format.
- The command stays thin and is validated through builds and smoke runs against
  the fixtures in `testdata/`.
