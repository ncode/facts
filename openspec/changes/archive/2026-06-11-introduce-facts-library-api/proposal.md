## Why

The Go port is complete and Ruby is gone, but the only public Go surface is a port of Ruby Facter's module-level API: package-global mutable state, no `context.Context`, no error returns. Programs that want to embed fact discovery (monitoring agents, provisioning tools) can't hold independent configurations in one process, can't bound or cancel resolution, and can't observe failures programmatically — they're stuck shelling out to the binary. With no external Go consumers yet, this is the last cheap moment to replace that surface with an idiomatic, embeddable library API.

## What Changes

- **BREAKING (Go API only)**: Remove the Ruby-compat global API (`Value`, `ToHash`, `Loadfacts`, `PuppetFacts`, `Search`, `Reset`, and the rest of the ~58 package-level exports) and all package-global mutable state. Ruby compatibility is promised only at the `facter` CLI process boundary.
- **BREAKING (Go API only)**: Rename the root package `facter` → `facts` so the package identifier matches the module path `github.com/ncode/facts`. The shipped binary keeps the name `facter`.
- Introduce the library API: immutable, hermetic-by-default `facts.Engine` built via `facts.New(opts ...Option)`; explicit `Discover(ctx)` returning an immutable `Snapshot` over the canonical fact tree; pure snapshot queries plus generic `facts.As[T]` decode; `ErrFactNotFound` sentinel and partial-results-with-joined-errors semantics; `log/slog` diagnostics (discarded by default).
- Rewire the CLI (`internal/app`) onto a system-following Engine (config file, default fact directories, persistent cache, script execution — all the things a hermetic engine does not do implicitly).
- Rebirth the API-level parity tests in `facter_test.go` as Engine- and CLI-level contract tests; CLI stdout/stderr behavior (output contract) and custom/external/`facter.conf` loading (input contract) remain byte-/behavior-identical.
- Amend the `go-port-framework-parity` spec to drop Go-API parity while keeping CLI, custom-fact, external-fact, config, cache, query, formatter, and diagnostics parity.
- No new dependencies; contract formatters (JSON/YAML/HOCON/legacy text) stay internal to the CLI path.

Binding constraints (per `CONTEXT.md` and `docs/adr/0001-0005`): the **output contract** and **input contract** must not change; everything between them is implementation freedom.

## Capabilities

### New Capabilities

- `facts-library-api`: The embeddable Go library surface — Engine construction and options, hermetic defaults, Discover/Snapshot lifecycle, canonical-tree queries and generic decode, error semantics, concurrency guarantees, and slog diagnostics.

### Modified Capabilities

- `go-port-framework-parity`: The "Public API and CLI parity" requirement narrows to CLI parity only — the Ruby-compatible Go function surface is removed; all CLI, custom/external fact, config, cache, query, formatter, and diagnostics parity requirements remain unchanged.

## Impact

- **Code**: root package (`facter.go`, `facter_test.go`) rewritten as `facts` (Engine/Snapshot/options/errors); `internal/facter` gains `context.Context` threading through exec and cloud-metadata HTTP paths and per-engine (instead of global) state; `internal/app` and `cmd/facter` rewired onto an Engine; `internal/cli` unchanged in behavior.
- **Tests**: `facter_test.go` parity scenarios migrate to Engine/CLI contract tests; CLI golden behavior must stay green on all four platform gates (Linux x64/arm64, macOS, Windows, FreeBSD).
- **Docs/process**: `docs/MIGRATION.md` checkpoint plus parity-ledger regeneration (`make parity-ledger && make parity-ledger-check`); README/PORTING updated for the `facts` identity and library usage.
- **Versioning**: module stays untagged/v0 through this change; v1.0.0 is a deliberate later event. CLI continues to report Facter behavior level 4.11.0.
- **Consumers**: none outside this repo (no tags, no known importers) — the breaking changes have no external blast radius.
