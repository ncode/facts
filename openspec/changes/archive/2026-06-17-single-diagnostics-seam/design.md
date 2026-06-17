# Design: Single diagnostics seam

## Context

`Session` is the per-run carrier of the Engine logger (`session.go:44-141`). Its `warn`/`debug`/`reportError` methods route to `s.logger`, falling back to a package-global handler when `s.logger == nil`. That fallback exists because several diagnostics are raised by code that has no `Session` in scope — `ParseConfig`, `FactCache`, `GroupTTLSeconds`, `DetectOSHierarchy`, and the `CollectionWithDottedFacts` collision path — so they call the package-global `warn`/`debug`/`reportError` directly (`external.go:126-152`), installed by the CLI via `SetWarningHandler`/`SetDebugHandler` with `defer …(nil)`.

The deepen-engine-internals change (archived 2026-06-17) removed the external-loader's package-global *host* hooks (`externalFactGOOS`/`externalFactRunCommand`/`externalFactOpen`) but deliberately left the diagnostic sink in place. This change finishes that direction for diagnostics.

## Goals / Non-Goals

**Goals**
- One diagnostic seam: the Engine logger. No package-global diagnostic state.
- A library `WithLogger` Engine observes every diagnostic discovery raises.
- CLI stderr output unchanged, byte-for-byte.

**Non-Goals**
- Changing the `stderrLogHandler` level mapping or the `WARN Facts -`/`DEBUG Facts -` stderr contract.
- Collapsing the `commandRunner`/`fileReader` parameter threading into `s.*` (that is the deepen-engine-internals follow-on, separate).
- Adding structured attributes/groups to records — diagnostics stay message-only, as the stderr contract requires.

## Decisions

- **Sink shape: thread `*slog.Logger`, not `*Session` or a new interface.** The contract is already slog-based; `Discover` has `s.logger` and the CLI has `slog.New(stderrLogHandler)`, so both call sites supply a logger directly. Threading `*Session` fails because the CLI path (`app.runQuery`) has no Session; a bespoke `diag` interface adds an adapter over what slog already provides.
- **Collisions are a discovery diagnostic, reported once at `newSnapshot`.** `reportCollectionCollision` currently lives in `CollectionWithDottedFacts`, which the formatter re-runs on every render (`formatter.go:70,90,111,139`, `query.go:22`) — so a collision is reported multiple times and from a path with no logger. The canonical tree is built once at `newSnapshot` (`snapshot.go:26`), which is reached only through `Discover` (logger in scope). Reporting there fixes both the missing logger and the redundant firing; the formatter's collection becomes a pure render.
- **`Session.logger` is guaranteed non-nil.** `Engine` already sets `e.logger` to a `DiscardHandler` when none is supplied (`engine.go:88-91`); `NewSessionContext` will default the same way so `s.warn`/`s.debug`/`s.reportError` need no nil branch once the globals are gone.
- **Error-class reaches the library logger.** `reportError`-class diagnostics (collision, unsupported cache group, unparseable TTL) map to `slog.Error`. Library consumers can filter; the CLI's `stderrLogHandler.Enabled` returns false for `LevelError`, so CLI output is unchanged.

## Risks / Trade-offs

- **[Library consumers gain error-class diagnostics they didn't see before]** → accepted: the spec mandates mapped severities, and dropping them was a side effect of the global sink, not a decision. Recorded in the proposal and CHANGELOG so it isn't read as accidental.
- **[CLI output regression]** → guarded by the existing CLI contract test plus a direct assertion that error-class produces no stderr line. The `stderrLogHandler` is untouched.
- **Signature churn** on `ParseConfig`/`NewFactCache`/`DetectOSHierarchy`/`GroupTTLSeconds` → these are `internal/engine` symbols; no public `facts` package signature changes.

## Migration Plan

1. Add the non-nil logger default to `Session`; thread `*slog.Logger` into the four free functions/types, supplied by `Discover` and the CLI.
2. Move collision reporting to `newSnapshot`; make the formatter collection silent.
3. Convert `core.go`'s in-scope `debug()`/`warn()` to `s.*`.
4. Delete `Set*Handler` and package `warn`/`debug`/`reportError`. Compiler flags any remaining caller.

## Open Questions

(none)
