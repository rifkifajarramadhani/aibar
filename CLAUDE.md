# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`aibar` is a long-lived Go daemon that emits Waybar-compatible JSON showing AI coding-tool usage limits. See `AGENTS.md` and `docs/PROJECT_OVERVIEW.md` for the full design rationale and phased roadmap; this file covers what's needed to be productive in the code.

## Commands

```sh
go test ./...                        # all package tests
go test -race ./...                  # race detector — run before submitting concurrent changes
go test ./internal/provider/codex    # single package
go test -run TestName ./internal/...  # single test
go vet ./...
golangci-lint run ./...              # repository linters (errcheck, staticcheck, etc.)
gofmt -w ./cmd ./internal            # baseline Go formatting
wsl -fix ./cmd/... ./internal/...    # logical-step blank-line spacing (wsl_5)
go build -o aibar ./cmd/aibar
go run ./cmd/aibar daemon             # runs against ~/.codex/sessions
```

The `daemon` subcommand accepts `--codex-root`, `--state`, and `--cache-dir` to point at fixtures or an isolated runtime instead of the real `~/.codex` / `~/.cache/aibar` paths — use these when exercising the daemon in tests or locally.

## Architecture

Single daemon, one goroutine per source feeding a central select loop. `daemon.Run` ([internal/daemon/daemon.go](internal/daemon/daemon.go)) owns the loop and wires everything:

- **Snapshots flow one direction**: the Codex fsnotify watcher (`internal/provider/codex`) sends `model.Snapshot` values into a channel → `state.Store.Apply` merges them → `render.JSON` formats a Waybar line → stdout. Only changed lines are emitted (deduped against `lastOutput`).
- **A 1-second ticker re-renders** without doing any I/O, so reset countdowns and data-age text advance while the render path stays free of file/network work. This is a hard design rule: never block the render path or Waybar on I/O.
- **Control plane**: `aibar refresh|next-provider|prev-provider|cycle-window` connect over a private Unix socket at `~/.cache/aibar/aibar.sock` (mode 0600) handled by `internal/control`. The socket also enforces single-instance (a live socket → "daemon already running"). `SIGUSR1` triggers the same rescan as `refresh`, for Waybar hooks.
- **State persistence**: last-good snapshots are written atomically to `state.json` and reloaded on restart, so a Waybar restart or crash never blanks the bar.

`internal/model` defines the shared contracts (`Provider`, `Snapshot`, `Window`, `Source`). The `Provider` interface (`Fetch`/`Watch`/`MinInterval`) is deliberately shaped for future network-backed providers (Claude, Cursor) even though only local Codex exists today; preserve these contracts when adding providers.

## Conventions specific to this repo

- **Format and lint before submitting.** Run `gofmt`, `wsl -fix ./cmd/... ./internal/...`, and `golangci-lint run ./...`. Keep logical operation blocks separated by blank lines (`wsl_5` style): setup, guards, I/O, mutation, and return paths should not run together without spacing.
- **Providers must stay isolated.** All parsing and file-watching for a provider lives in its own package under `internal/provider/`. One provider's failure must never blank the whole bar — errors keep last-good windows and surface a visible stale/error state instead.
- **Codex windows are classified by duration, not position.** Never assume `rate_limits.primary` is the 5h window; classify by `window_minutes` (300 → `5h`, 10080 → `weekly`). A live account may expose only the weekly window, and may put it in `primary` with `secondary` null. Unknown windows are ignored.
- **`next-provider`/`prev-provider` are intentional no-ops** in this Codex-only milestone — leave them wired but inert rather than removing them.
- **Never make network calls, read credentials, or commit session data / generated state** in the current milestone. Runtime files (socket, PID, state) go under the cache dir with restrictive (0600/0700) permissions.
- Add a JSONL fixture under `testdata/<provider>/` plus a parser test for any new provider rollout format.
