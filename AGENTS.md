# Repository Guidelines

## Project Structure & Module Organization

`aibar` is a Go module that emits Waybar-compatible JSON for AI usage limits. It
follows domain-first Clean Architecture (see `docs/ARCHITECTURE.md`). The CLI
entrypoint is in `cmd/aibar/main.go`. The core domain is `internal/usage`
(entities, the `Store` merge policy, the `View`, and the `Provider`/`Refreshable`/
`SnapshotArchive` ports); the application loop is `internal/daemon` (which owns the
`Renderer`/`ControlServer` ports and the control-command vocabulary). Adapters
live under `internal/adapter/`: `codex` and `claude` (providers), `waybar`
(presentation), `control` (Unix socket), `statefile` (state.json persistence),
and `logging`. `internal/config` resolves paths and `internal/bootstrap` wires
the adapters into the daemon. Core packages must not import adapters, bootstrap,
or config — `depguard` enforces this. Put integration fixtures in `testdata/`
(currently `testdata/codex/` and `testdata/claude/`), documentation in `docs/`,
and Waybar assets/snippets in `assets/` or the repository root.

## Build, Test, and Development Commands

```sh
make check                           # fmt + lint + test (pre-submit gate)
go test ./...                         # run all package tests
go test -race ./...                  # check concurrent code for races
go vet ./...                         # run Go static checks
golangci-lint run ./...              # run repository linters (includes depguard)
gofmt -w ./cmd ./internal            # format Go sources
wsl -fix ./cmd/... ./internal/...    # apply logical-step blank-line spacing
go build -trimpath ./cmd/aibar       # build the daemon binary
go run ./cmd/aibar daemon             # run against ~/.codex/sessions
```

Use `go build -o aibar ./cmd/aibar` when a named local binary is useful.
Daemon flags such as `--codex-root`, `--state`, and `--cache-dir` make fixture
or isolated-runtime testing straightforward.

## Coding Style & Naming Conventions

Use standard Go formatting (`gofmt`) and idiomatic Go names: exported names
need useful GoDoc where appropriate, while package-private helpers use
camelCase. Keep provider-specific parsing and file-watching logic inside its
adapter package under `internal/adapter/`; preserve the `usage` domain,
snapshot, and renderer contracts when adding providers, and define new ports in
the package that consumes them. Prefer small, error-returning functions and
restrictive permissions for runtime files.

Separate logical steps with blank lines (`wsl` / `wsl_5` style): group related
assignments together, then leave a blank line before the next guard clause,
I/O call, state transition, or return. Example:

```go
file, err := os.Open(path)
if err != nil {
	return usage.Snapshot{}, err
}

defer func() { _ = file.Close() }()

snapshot, err := Parse(file, now)
if err != nil {
	return usage.Snapshot{}, fmt.Errorf("parse %s: %w", path, err)
}

snapshot.Provider = "codex"
return snapshot, nil
```

Before submitting Go changes, run `gofmt`, `wsl -fix ./cmd/... ./internal/...`,
and `golangci-lint run ./...`. Error strings should start lowercase and
intentionally ignored close/remove errors should use `_ =`.

## Testing Guidelines

Tests use Go’s standard `testing` package and live beside the code as
`*_test.go`. Name tests `Test<Behavior>` and use temporary directories for
filesystem behavior. Add JSONL fixtures and parser tests for new provider
formats, then run `go test ./...` and `go test -race ./...` before submitting.

## Commit & Pull Request Guidelines

The history currently contains only an `initial commit`, so no repository
commit convention is established. Use short, imperative subjects (for
example, `add Codex rotation handling`) and keep unrelated changes separate.
Pull requests should explain the behavior change, include test commands and
results, link any relevant issue, and include Waybar screenshots or config
examples when the UI or integration changes.

## Security & Configuration Tips

The Codex milestone reads local rollout files and makes no network calls.
Never commit credentials, session data, or generated state. Keep cache, PID,
and socket files under the user cache directory with restrictive permissions;
future network-backed providers must preserve last-good data and expose stale
or authentication errors visibly.
