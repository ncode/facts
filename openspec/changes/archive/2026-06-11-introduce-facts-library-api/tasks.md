## 1. Contract test baseline (before any surgery)

- [x] 1.1 Inventory `facter_test.go` scenarios and classify each as CLI-contract, engine-contract, or Ruby-API-only (dropped with the old surface); record the mapping in the change dir
- [x] 1.2 Add CLI-level contract tests driving `internal/app.Run(stdout, stderr, args)` covering the output-contract scenarios (formatters, queries, legacy facts, option validation, exit statuses) against the current implementation
- [x] 1.3 Extend CLI contract tests to stderr diagnostics (message text, severity, once-only semantics) and verify green on all four platform gates

## 2. Per-engine state and context threading (`internal/facter`)

- [x] 2.1 Audit every package-level `var` in `internal/facter`; move session caches, once-only sets, loaded config, and search/registration state onto a per-engine state struct threaded through resolution
- [x] 2.2 Thread `context.Context` through command-execution paths (core resolvers, custom `setcode` execution, external-fact scripts, timeouts)
- [x] 2.3 Thread `context.Context` through cloud-metadata HTTP paths (`ec2.go`, `gce.go`, `az.go`)
- [x] 2.4 Scope custom-fact DSL evaluation (`Facter.value` recursion, confine lookups) to the owning engine state instead of globals
- [x] 2.5 Keep the existing public API and all tests green through the restructure; add `go test -race` over the restructured paths

## 3. Public library API (`package facts`)

- [x] 3.1 Implement `Engine` and `facts.New(opts ...Option)` with hermetic defaults plus options: `WithConfigFile`, `WithCustomDirs`, `WithExternalDirs`, `WithCache`, `WithFact`, `WithLogger`, `WithSystemDefaults`
- [x] 3.2 Implement `Discover(ctx)` returning an immutable `Snapshot`: partial results with `errors.Join` aggregation, context cancellation honored, not-applicable facts excluded from errors
- [x] 3.3 Implement Snapshot queries: dotted `Value` with `ErrFactNotFound` vs nil-valued distinction, tree access, and iteration (`iter.Seq2`) for formatter consumption
- [x] 3.4 Implement `facts.As[T]` generic decode over the canonical tree with loud shape-mismatch errors; table-driven tests across core/custom/external shapes
- [x] 3.5 Wire engine diagnostics to `log/slog` with per-engine once-only dedup and `slog.DiscardHandler` default
- [x] 3.6 Implement scoped discovery (decide `Discover(ctx, queries...)` vs `facts.Only(...)` against `internal/facter/query.go` semantics) and document the choice in design.md
- [x] 3.7 Add Engine-level contract tests from the 1.1 mapping, including the two-isolated-engines concurrency scenario under `-race`

## 4. CLI rewiring

- [x] 4.1 Implement the CLI's private slog handler rendering Ruby-compatible stderr lines from engine diagnostics
- [x] 4.2 Rewire `internal/app` onto a system-following Engine (flags/config → options), resolving whether `WithSystemDefaults` implies the persistent cache per `facter.conf` TTL semantics
- [x] 4.3 Verify all CLI contract tests from group 1 still pass unchanged on all four platform gates

## 5. Remove the Ruby-compat API and rename

- [x] 5.1 Delete the Ruby-compat package-level exports and their global state from the root package, demoting still-needed machinery (e.g. execution helpers used by custom facts) to internal
- [x] 5.2 Rename the root package `facter` → `facts`; update `cmd/facter`, `internal/app`, and remaining imports
- [x] 5.3 Retire `facter_test.go` per the 1.1 mapping and verify zero package-global mutable state remains in the public package (vet/staticcheck + review)

## 6. Docs, ledger, and release hygiene

- [x] 6.1 Update README.md and PORTING.md for the facts identity with library usage examples; write package godoc for `facts`
- [x] 6.2 Add the MIGRATION.md checkpoint and regenerate the parity ledger (`make parity-ledger && make parity-ledger-check`)
- [x] 6.3 Update CHANGELOG.md, confirm v0 (untagged) posture, and run the full verification suite (`go test -race ./...`, `go vet ./...`, `staticcheck ./...`) plus all platform gates
