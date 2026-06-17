# Single diagnostics seam

## Why

Engine diagnostics flow through two seams, not one. The `Session` carries a per-Engine logger (`session.go:119-141`, `s.warn`/`s.debug`/`s.reportError` → `s.logger`), but six classes of diagnostic bypass it and call a **package-global** sink instead — `warn`/`debug`/`reportError`, installed via `SetWarningHandler`/`SetDebugHandler`/`SetErrorHandler` (`external.go:106-152`):

- `core.go` (~10 sites: `core.go:1527,2749,3202,4088,4094,4099,4123,4178,4248,4251,…`) calls the package `debug()` **while the `Session` is already in scope** — it should call `s.debug()`.
- `config.go:110,115` (`ParseConfig`) calls `warn()`.
- `cache.go:164,217,291,295,300,331` (`FactCache`) calls `reportError`/`warn`/`debug`.
- `groups.go:117` (`ttlSeconds`, reached only via `FactCache`→`GroupTTLSeconds`) calls `reportError`.
- `fact.go:83` (`reportCollectionCollision`, inside `CollectionWithDottedFacts`) calls `reportError`.
- `detector.go:53,61` (`DetectOSHierarchy`) calls `debug()`.

This costs three things:

1. **A leak.** A library Engine built with `WithLogger` does *not* receive cache, config, group, collision, or OS-hierarchy diagnostics — they go to the package-global handler (nil → silently dropped unless the CLI installed one). The `facts-library-api` spec already requires that *"Engine diagnostics SHALL flow through `log/slog`"* (`spec.md:67`); the implementation only half-honors it.
2. **Global mutable state.** The CLI swaps handlers with `defer engine.SetWarningHandler(nil)` (`app.go:239-242,321-324`). CONTEXT.md pins *"no global mutable state; every consumer constructs its own"* — process-global handlers contradict that and are racy under concurrent `Discover`.
3. **Stale rationale.** `session.go:48-49` justifies the globals as backing *"the CLI and the Ruby-compat API."* The Ruby-compat API was removed in ADR-0001; the only remaining caller is the CLI, which already constructs a `slog` logger (`stderrLogHandler`).

This is conformance + deletion: make every diagnostic honor the existing seam, then delete the redundant global one.

## What Changes

- **Route `core.go` package calls to the Session.** Replace the ~10 `debug()`/`warn()` calls that already hold an `s *Session` with `s.debug()`/`s.warn()`.
- **Thread `*slog.Logger` into the free functions/types that have no Session.** `ParseConfig(path, log)`, `NewFactCache(dir, ttls, groups, log)` (carrying `log` into `GroupTTLSeconds`/`ttlSeconds`), and `DetectOSHierarchy(hierarchy, identifier, family, log)`. `Engine.Discover` passes `s.logger`; the CLI passes its own `slog.New(stderrLogHandler)`.
- **Report canonical-tree collisions once, at discovery.** Move collision reporting out of `CollectionWithDottedFacts` (re-run on every format render) to snapshot construction (`newSnapshot`, which has the Engine logger via `Discover`). The formatter's collection becomes diagnostic-silent — a pure render.
- **Delete the package-global sink.** Remove `SetDebugHandler`/`SetWarningHandler`/`SetErrorHandler` and the package-level `warn`/`debug`/`reportError`. Make `Session.logger` guaranteed non-nil (default `slog.DiscardHandler`) so the Session methods need no nil fallback.

## Capabilities

### New Capabilities

(none) — this is internal architecture plus a tightening of an existing library contract; no new fact, flag, format, or dependency.

### Modified Capabilities

- `facts-library-api`: the "Diagnostics via structured logging" requirement is **tightened**, not merely implemented. It already requires diagnostics to flow through `log/slog`; this change adds that *all* engine diagnostics — config parsing, the persistent cache, fact-group TTL parsing, canonical-tree collisions, and OS-hierarchy detection — flow through the Engine's logger with their mapped severities (including error-class), and that there is **no** package-global diagnostic sink. A library consumer with `WithLogger` now observes these diagnostics where today they are dropped.

## Impact

- **Code**: `internal/engine/external.go` (delete `Set*Handler` + package `warn`/`debug`/`reportError`), `session.go` (non-nil logger default; drop the global fallback in `s.warn`/`s.debug`/`s.reportError`), `core.go` (~10 calls → `s.*`), `cache.go` (`NewFactCache` gains `*slog.Logger`), `config.go` (`ParseConfig` gains `*slog.Logger`), `groups.go` (`GroupTTLSeconds`/`ttlSeconds` gain `*slog.Logger`), `detector.go` (`DetectOSHierarchy` gains `*slog.Logger`), `fact.go` + `snapshot.go` (collision reported at `newSnapshot`; formatter collection silent), `internal/engine/engine.go` (`Discover` passes `s.logger` to the threaded functions), `internal/app/app.go` (pass the CLI logger to `ParseConfig`/`NewFactCache`; remove the `SetWarningHandler`/`SetDebugHandler` wiring).
- **Behavior**: library `WithLogger` consumers now receive cache/config/group/collision/OS-hierarchy diagnostics, including error-class. **CLI stderr output is unchanged** — `stderrLogHandler` maps slog levels to the same `WARN Facts -`/`DEBUG Facts -` lines and still drops error-class (`loghandler.go:22-46`), so the output contract holds.
- **Docs**: `CHANGELOG.md` records the library-visible diagnostics change. No `docs/schema/facts.yaml` change (no fact added or removed).
