# Go Coding Patterns for Agents

Patterns here draw from Effective Go, Uber's Go Style Guide, Go Code Review Comments, and the Awesome Go catalog. Use them as building blocks when designing solutions.

## Project & Package Layout

- Use a single `cmd/aibar` binary whose subcommands (`daemon`, `refresh`, `next-provider`, `prev-provider`, `cycle-window`) parse config and wire dependencies through `internal/bootstrap`.
- Keep reusable code in `internal/` to prevent accidental external use; optional shared public packages can live in `pkg/`.
- Organize packages by domain or capability (`usage`, `daemon`), not by technical layer alone; group technology-specific code under `internal/adapter/<tech>`.
- Avoid cyclic imports; extract interfaces or helper packages when cycles appear.
- Keep package APIs small and purposeful; export only what consumers need.

## Configuration & Construction

- Prefer constructors named `NewType` that return concrete types and errors.
- When options proliferate, use functional options (`WithX(...)`) or small config structs with defaults.
- Defer heavy initialization until needed; avoid complex `init` logic.

## Error Handling Patterns

- Early returns on errors; wrap with context using `%w` for callers to inspect via `errors.Is/As`.
- Expose sentinel errors or typed errors only when callers must branch. aibar classifies provider failures with `usage.ProviderError`/`usage.ErrorKindOf`.
- Avoid panics for expected failures; reserve panics for programmer errors or truly fatal states.
- Keep logging at the edge; do not double-log the same error at multiple layers.

## Concurrency Patterns

- **Worker Pool:** Bound concurrency with a fixed number of workers reading from a job channel; stop via context or closing the channel.
- **Pipeline:** Stage work through channels; ensure every stage handles context and closes its output channel when done. aibar's daemon is a fan-in: one goroutine per provider feeds a single snapshot channel drained by the central select loop.
- **Fan-out/Fan-in:** Distribute tasks to multiple goroutines and merge results; use `sync.WaitGroup` or context to coordinate.
- Avoid goroutine leaks by canceling contexts and ensuring sends/receives cannot block forever.
- Choose mutexes over channels when sharing structured state; use channels when transferring ownership or events. The `usage.Store` uses a mutex; snapshots flow over channels.

## Collections & Data Handling

- Favor slices over arrays; pre-size with `make` when capacity is known.
- Copy slices when data should not share backing arrays; use `append` carefully to avoid modifying caller data unintentionally. `Snapshot.Clone` copies the `Windows` slice before handing snapshots across goroutines.
- For maps, initialize before writes; check presence with the two-value form; use struct keys only when they are comparable.
- Use DTO structs for crossing boundaries and keep domain models separate when needed; the statefile adapter keeps its `diskState` shape separate from the domain.

## Adapter & Boundary Patterns

- Inbound adapters (the control socket) validate and translate external input into daemon actions; outbound adapters (providers, statefile, waybar) implement core-owned ports.
- Keep cross-cutting concerns (logging) in adapters, not in core business logic.
- Respect `context.Context` for cancelation/deadlines in watchers and downstream calls.
- Close resources with `defer` immediately after acquisition.

## Testing Patterns

- Table-driven tests with subtests (`t.Run`) for coverage across inputs.
- Use fakes/stubs for external systems; prefer in-memory doubles over heavy integration tests when unit testing business logic.
- For concurrency, coordinate with channels or contexts instead of sleeps; run `go test -race` for shared-state code.
- Golden files only when output is stable and reviewed; keep them small and checked in.

## CLI/Tools Patterns

- Use the standard `flag` package or minimal wrappers; avoid complex global state.
- Provide `-help` output and reasonable defaults; exit codes should reflect success/failure.

## Library Selection (Awesome Go)

- Use the Awesome Go catalog to find libraries by category (web, database, testing, logging, crypto, etc.).
- Evaluate libraries for maintenance (recent commits), license, documentation, tests, and API stability before adopting.
- Prefer standard library first; add dependencies only when they provide clear benefit. aibar depends only on `fsnotify` beyond the standard library.

## References

- Paraphrased from: Effective Go (go.dev/doc/effective_go), Uber Go Style Guide (github.com/uber-go/guide), Go Code Review Comments (go.dev/wiki/CodeReviewComments), and Awesome Go (awesome-go.com).
