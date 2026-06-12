## Context

The Go port is complete: Ruby is removed, four platform CI gates are green, and the public surface is a faithful port of Ruby Facter's module-level API — ~58 package-level exports in `package facter` (root) backed by mutex-guarded global state, delegating to `internal/facter` (resolvers, custom/external fact loading, config, cache, query, formatters). The CLI (`cmd/facter` → `internal/app`) consumes those globals. A grilling session (2026-06-11) settled the library direction; decisions are recorded in `CONTEXT.md` and `docs/adr/0001`–`0005`, which this design implements.

Binding constraints:
- **Output contract**: `facter` CLI stdout/stderr — fact names, nesting, value normalization, JSON/YAML/HOCON/legacy-text formatting, diagnostic text/severity/once-only semantics, exit statuses — stays byte-compatible.
- **Input contract**: custom fact DSL files, programmatic registration, external facts (files/executables/env), and `facter.conf` semantics keep working unchanged.
- Everything between the contracts is implementation freedom. No external Go consumers exist (module untagged), so Go-API breakage has zero blast radius.

## Goals / Non-Goals

**Goals:**
- An idiomatic, embeddable Go library: `package facts`, immutable hermetic `Engine`, explicit `Discover(ctx)` → immutable `Snapshot`, generic `As[T]` decode, honest error semantics, `log/slog` diagnostics, zero package-global mutable state.
- The CLI becomes an ordinary library consumer wiring a system-following Engine, with contract behavior unchanged.
- Parity scenarios from `facter_test.go` survive as Engine- and CLI-level contract tests.

**Non-Goals:**
- Non-Go consumers (C ABI, daemon/IPC) — the binary already serves them.
- Predefined typed structs for core facts — later, additive, built on `As[T]` (ADR 0002).
- Exporting the contract formatters (JSON/YAML/HOCON/legacy text) — CLI-internal; additive later if demanded.
- Tagging v1.0.0 — module stays v0 until the API survives real consumers.
- Tracking a newer upstream Facter behavior level (CLI keeps reporting 4.11.0).

## Decisions

1. **Single idiomatic API; CLI is the only Ruby-compat boundary (ADR 0001).** The Ruby-shaped globals are deleted, not facaded — a facade would preserve ~58 Ruby-isms (`Loadfacts`, `PuppetFacts`, …) for a consumer that doesn't exist. Alternatives (keep globals; facade over default engine) rejected in ADR 0001.
2. **Naming (ADR 0004).** Root package renames `facter` → `facts` to match the module path; binary stays `facter`. Mechanical rename in `cmd/` and `internal/app` imports.
3. **Engine: immutable, hermetic, functional options (ADR 0003, 0005).** `facts.New(opts ...Option) (*Engine, error)`. Default: core facts only — no `facter.conf`, no directory scanning, no script execution, no persistent cache. Options: `WithConfigFile`, `WithCustomDirs`, `WithExternalDirs`, `WithCache`, `WithFact(name, resolver)` (programmatic registration, fixed at construction), `WithLogger`, and `WithSystemDefaults()` for CLI-equivalent behavior. All registration happens at construction; nothing mutates after `New`.
4. **Discover → Snapshot (ADR 0005).** `eng.Discover(ctx)` runs resolvers and returns an immutable `Snapshot` (canonical tree + pure queries). No engine-resident memo of results, no `Flush`: freshness = discover again; the caller's Snapshot is the cache. Scoped discovery (e.g. `eng.Discover(ctx, facts.Only("os", "networking"))`) bounds resolution for cheap single-fact use; exact shape is an open question below. Both types are concurrency-safe by immutability.
5. **Canonical tree only; typing is a view (ADR 0002).** Snapshot queries return the same names/nesting/normalized values the formatters consume. `facts.As[T](snap, query)` decodes any subtree into a caller type, failing loudly on shape mismatch — works uniformly for core, custom, and external facts. No parallel typed model (precedence makes typed accessors unsound).
6. **Error semantics.** `var ErrFactNotFound = errors.New(...)`; `snap.Value(query)` distinguishes missing fact (`ErrFactNotFound`) from nil-valued fact (`nil, nil`). `Discover` returns partial Snapshot + `errors.Join` of per-fact failures (documented: snapshot is valid when err != nil; precedent `os.ReadDir`). Not-applicable facts (e.g. EC2 metadata off-cloud) are absent, never errors.
7. **Diagnostics via slog.** Engine emits contract-pinned message text at mapped levels; default handler is `slog.DiscardHandler`; once-only dedup tracked per engine. The CLI installs a private slog handler rendering byte-compatible Ruby-style stderr lines — the stderr contract lives in one place at the boundary.
8. **Internal restructure.** All `internal/facter` global state (session caches, once-only sets, search paths, registration lists) moves onto a per-engine struct threaded through resolution. `context.Context` threads through exec paths (`commandOutput`, custom/external fact execution with timeouts) and cloud-metadata HTTP (`ec2.go`, `gce.go`, `az.go`). Custom-fact `Facter.value()` lookups during DSL evaluation resolve against the owning engine, not a global.
9. **Test strategy.** Contracts first: migrate `facter_test.go` scenarios to (a) CLI-level golden tests driving `internal/app.Run(stdout, stderr, args)` and (b) Engine-level tests for resolution semantics, before deleting the old API. Platform gates stay blocking throughout.

## Risks / Trade-offs

- [Output-contract regression while rewiring the CLI] → Migrate the formatter/CLI parity scenarios to CLI-level tests *before* swapping the plumbing; keep all four platform gates blocking; ship the rewire as test-first commits.
- [Hidden coupling to global state inside `internal/facter` (session caches, once-only sets, custom-fact cross-lookups)] → Audit every package-level `var` in `internal/facter` during the per-engine restructure; the race detector plus two-engines-in-one-test contract tests catch stragglers.
- [Custom fact DSL evaluation depends on engine-scoped `Facter.value` recursion] → Make the resolution context an explicit parameter of DSL evaluation; covered by existing custom-fact parity scenarios re-homed onto an Engine.
- [Parity ledger references break when `facter_test.go` moves] → Regenerate (`make parity-ledger && make parity-ledger-check`) with the MIGRATION.md checkpoint, per project convention.
- [Discover-everything is heavier than the old lazy single-fact path] → Scoped discovery bounds the work; resolvers already share per-run session caching within one Discover.
- [`As[T]` decode is new code with contract-adjacent semantics] → Decode reads the canonical tree only (never resolves); table-driven tests over core/custom/external shapes incl. mismatches.

## Migration Plan

1. Land contract tests (CLI golden + Engine-level) alongside the existing API — green on all gates.
2. Restructure `internal/facter` to per-engine state + ctx threading; old API still passing.
3. Introduce `facts.Engine`/`Snapshot`/options/errors in the root package; rewire `internal/app` onto a system-following Engine.
4. Delete the Ruby-compat surface and rename the package to `facts`; update README/PORTING, MIGRATION.md checkpoint, regenerate parity ledger.
5. Rollback strategy: steps are independent commits pushed to main; each keeps gates green, so reverting the latest step is always clean.

## Decisions resolved during implementation

- **Scoped discovery is variadic queries: `Discover(ctx, queries ...string)`** (task 3.6). The strings use the CLI's dot-notation matching from `internal/facter/query.go` (case-insensitive, name or name-prefix), and they scope *resolution*: only custom-fact files whose stem matches a queried name are loaded (`LoadCustomFactFilesFromDirs` semantics), so facts outside the scope may be absent from the Snapshot. A `facts.Only(...)` option type was rejected: it would add an option vocabulary for what is naturally a query list, and the strings already carry the contract semantics.
- **`Snapshot` exposes `All() iter.Seq2[string, any]`** (sorted top-level entries, copied values) in addition to `Tree()` (deep copy) — needed by the CLI formatter path and cheap to provide.
- **`WithFact` resolvers are `func(ctx context.Context) (any, error)`**; values get custom-fact normalization (RFC 3339 times, string-keyed maps, null-byte rejection). Resolver errors are partial-discovery failures.
- **`WithFact` facts override core facts and lose to external facts** — same precedence as DSL custom facts at the CLI. The Ruby-era programmatic semantics (zero-weight `Facter.add` defers to a resolving core fact) die with the Ruby API: they emulated Ruby resolution-merging, and CONTEXT.md's stated use case ("register a fact on the engine to override `networking.ip`") requires override semantics.
- **Architecture: the real Engine lives in `internal/facter`** (`EngineConfig`/`NewEngine`/`Discover`/`Snapshot`); the public package wraps it thinly. The CLI consumes the internal face (which can carry CLI-only knobs like legacy facts and `--no-ruby` without growing the public API), satisfying "the CLI constructs its own engine" without leaking CLI surface into `facts`.
- **Hermeticity required loader splits**: `LoadExternalFactsFromDirs` (no `FACTER_*` env vars, partial-failure collection) and `LoadCustomFactsFromDirs`/`LoadCustomFactFilesFromDirs` (no `FACTERLIB`) back the engine; the CLI keeps the system-following wrappers with unchanged semantics.

## Open Questions

*(none remaining)*

- ~~Whether `WithSystemDefaults` also implies the persistent cache~~ — **Resolved (task 4.2): it does not.** The CLI's persistent cache runs *after query selection* (`cache.ResolveFacts` over the selected facts), so it is not a discovery-time engine concern with the same shape: a library engine caching the full discovered set would write different cache files than the CLI for query runs. The cache therefore stays a separate `WithCache` opt-in (which caches the discovered set per `facter.conf` TTLs), and the CLI keeps its own post-selection cache step, byte-identical to before. `WithSystemDefaults` never touches the cache.
