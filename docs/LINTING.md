# Go Linting & Quality Checklist

Use this checklist to keep Go codebases healthy. Content is drawn from Effective Go, Go Code Review Comments, and common Go tooling docs.

## Baseline Tools

- `gofmt`: Mandatory formatting; run before committing.
- `goimports`: Formats and prunes/organizes imports.
- `wsl` / `wsl_5`: Logical-step blank-line spacing (`wsl -fix ./cmd/... ./internal/...`).
- `go vet`: Static analysis for suspicious constructs (format string mistakes, unreachable code, copylocks, etc.).
- `golangci-lint`: Aggregates linters (depguard, errcheck, errorlint, govet, ineffassign, revive, staticcheck, unused). Configure per repo to balance signal vs noise.
- `go test -race`: Detects data races; run on packages with concurrency.

## Common Lint Rules

- Unused: remove unused variables/imports; do not keep dead code.
- Errors: check returned errors; wrap with `%w`; avoid swallowing errors. Do not use panics for normal errors. `errorlint` requires `errors.Is`/`errors.As` over `==`/type assertions on errors.
- Naming: avoid stutter; keep exported names meaningful; follow standard initialisms (`ID`, `HTTP`).
- Imports: avoid blank imports unless for side effects (document why); avoid dot-imports in production code.
- Concurrency: guard shared maps; close channels only from senders; avoid goroutine leaks (tie to context).
- Defer: close resources immediately after open (`defer f.Close()`); beware of defers in loops for large iterations.
- Range pitfalls: avoid taking the address of loop variables; use index-based capture.
- Struct tags: keep JSON/DB tags consistent; avoid unused tags; ensure backticks are balanced.
- Composite literals: use field names for structs when many fields or exported fields are involved.
- Context: place `context.Context` as the first parameter when meaningful; do not store contexts inside structs.

## Layering Enforcement

- `depguard` (configured in `.golangci.yml`) enforces the dependency direction: files in `internal/usage` and `internal/daemon` may not import `internal/adapter`, `internal/bootstrap`, or `internal/config`. A layering violation fails the lint gate — see `docs/ARCHITECTURE.md`.

## Suggested CI Gates

- `gofmt -w ./cmd ./internal`
- `wsl -fix ./cmd/... ./internal/...`
- `go vet ./...`
- `golangci-lint run ./...`
- `go test ./...` (add `-race` for critical paths)

This repository provides these gates through `make check` and `.golangci.yml`.

## References

- Paraphrased from: Effective Go (go.dev/doc/effective_go), Go Code Review Comments (go.dev/wiki/CodeReviewComments), and common Go tooling practices.
