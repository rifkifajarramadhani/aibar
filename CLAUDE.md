# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`aibar` is a long-lived Go daemon that emits Waybar-compatible JSON showing AI coding-tool usage limits. See `AGENTS.md` and `docs/PROJECT_OVERVIEW.md` for the full design rationale and phased roadmap; this file covers what's needed to be productive in the code.

## Commands

```sh
make check                           # fmt + lint + test (the pre-submit gate)
go test ./...                        # all package tests
go test -race ./...                  # race detector — run before submitting concurrent changes
go test ./internal/adapter/codex     # single package
go test -run TestName ./internal/...  # single test
go vet ./...
golangci-lint run ./...              # repository linters (depguard, errcheck, staticcheck, etc.)
gofmt -w ./cmd ./internal            # baseline Go formatting
wsl -fix ./cmd/... ./internal/...    # logical-step blank-line spacing (wsl_5)
go build -o aibar ./cmd/aibar
go run ./cmd/aibar daemon             # runs against ~/.codex/sessions
```

The `daemon` subcommand accepts `--codex-root`, `--state`, and `--cache-dir` to point at fixtures or an isolated runtime instead of the real `~/.codex` / `~/.cache/aibar` paths — use these when exercising the daemon in tests or locally.

## Architecture

Domain-first Clean Architecture (full spec in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)): the core (`internal/usage`, `internal/daemon`) depends only on the standard library; providers, presentation, control, and persistence are adapters under `internal/adapter/`, wired by `internal/bootstrap`. `depguard` in `.golangci.yml` forbids core from importing adapters/bootstrap/config.

Single daemon, one goroutine per source feeding a central select loop. `daemon.Daemon.Run` ([internal/daemon/daemon.go](internal/daemon/daemon.go)) owns the loop; `bootstrap.WireDaemon` ([internal/bootstrap/bootstrap.go](internal/bootstrap/bootstrap.go)) constructs and injects its collaborators:

- **Snapshots flow one direction**: the Codex fsnotify watcher (`internal/adapter/codex`) sends `usage.Snapshot` values into a channel → `usage.Store.Apply` merges them → the `waybar` renderer (`internal/adapter/waybar`, behind the daemon's `Renderer` port) formats a Waybar line → stdout. Only changed lines are emitted (deduped against `lastOutput`).
- **A 1-second ticker re-renders** without doing any I/O, so reset countdowns and data-age text advance while the render path stays free of file/network work. This is a hard design rule: never block the render path or Waybar on I/O.
- **Control plane**: `aibar refresh|next-provider|prev-provider|cycle-window` connect over a private Unix socket at `~/.cache/aibar/aibar.sock` (mode 0600) handled by `internal/adapter/control` (behind the daemon's `ControlServer` port). The socket also enforces single-instance (a live socket → "daemon already running"). `SIGUSR1` triggers the same rescan as `refresh`, for Waybar hooks.
- **State persistence**: `usage.Store` holds the merge policy; `internal/adapter/statefile` (the `usage.SnapshotArchive` port) writes last-good snapshots atomically to `state.json` and reloads them on restart, so a Waybar restart or crash never blanks the bar.

`internal/usage` defines the shared contracts (`Provider`, `Snapshot`, `Window`, `Source`, and the `SnapshotArchive` port). The `Provider` interface (`Fetch`/`Watch`/`MinInterval`) is deliberately shaped for network-backed providers (Claude exists; Cursor is planned); preserve these contracts when adding providers, and define new ports in the package that consumes them.

## Conventions specific to this repo

- **Format and lint before submitting.** Run `make check` (or `gofmt`, `wsl -fix ./cmd/... ./internal/...`, and `golangci-lint run ./...`). Keep logical operation blocks separated by blank lines (`wsl_5` style): setup, guards, I/O, mutation, and return paths should not run together without spacing.
- **Respect the layering.** Core (`internal/usage`, `internal/daemon`) must not import adapters, bootstrap, or config; `depguard` fails the lint if it does. Adapters implement core-owned ports and assert it with `var _ Port = (*Impl)(nil)`.
- **Providers must stay isolated.** All parsing and file-watching for a provider lives in its own adapter package under `internal/adapter/`. One provider's failure must never blank the whole bar — errors keep last-good windows and surface a visible stale/error state instead.
- **Codex windows are classified by duration, not position.** Never assume `rate_limits.primary` is the 5h window; classify by `window_minutes` (300 → `5h`, 10080 → `weekly`). A live account may expose only the weekly window, and may put it in `primary` with `secondary` null. Unknown windows are ignored.
- **`next-provider`/`prev-provider` are intentionally inert** in the current milestone — leave them wired but no-op rather than removing them.
- **Never commit credentials, session data, or generated state.** Only the provider adapters that own them read credentials or make network calls (Claude's OAuth usage endpoint); nothing else does. Runtime files (socket, PID, state) go under the cache dir with restrictive (0600/0700) permissions.
- Add a JSONL fixture under `testdata/<provider>/` plus a parser test for any new provider rollout format.
