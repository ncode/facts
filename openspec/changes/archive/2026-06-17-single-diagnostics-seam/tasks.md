## 1. Logger plumbing — Tests First
<!-- all tasks complete; verified by gates (vet, 6/6 tests, -race, 4 GOOS builds), diff review, and deep Codex review -->


- [x] 1.1 Add a test that an Engine built with `WithLogger`, `WithConfigFile` (unreadable/invalid file), and `WithCache` (forced write failure) emits the config-read, cache-write, and TTL-parse diagnostics to the supplied logger with their mapped severities (engine_test.go / cache_test.go / config_test.go)
- [x] 1.2 Add a test that a fact-value/dotted-child collision (`os` scalar + `os.name`) emits exactly one error-class record to the Engine logger during `Discover`, and emits nothing further when the Snapshot is formatted in every format (engine_test.go)
- [x] 1.3 Add a test that `DetectOSHierarchy` fallback paths emit their debug diagnostics to the passed logger (detector_test.go)
- [x] 1.4 Add a CLI test asserting stderr is byte-identical for warn/debug, and that an error-class engine diagnostic produces no stderr line (internal/app/contract_test.go)

## 2. Thread the logger

- [x] 2.1 `NewSessionContext` defaults `s.logger` to `slog.New(slog.DiscardHandler)`; drop the package-global fallback from `s.warn`/`s.debug`/`s.reportError` (session.go)
- [x] 2.2 `ParseConfig(path string, log *slog.Logger) (Config, error)` — route the two `warn` sites to `log`; update `Engine.Discover` (passes `s.logger`) and `app.runQuery` (passes the CLI logger) (config.go, engine.go, app.go)
- [x] 2.3 `NewFactCache(dir string, ttls []FactTTL, groups []FactGroup, log *slog.Logger)` — store `log`, route the `reportError`/`warn`/`debug` sites to it; thread it into `GroupTTLSeconds`/`ttlSeconds` so `groups.go:117` uses `log`; update both call sites (cache.go, groups.go, engine.go, app.go)
- [x] 2.4 `DetectOSHierarchy(..., log *slog.Logger)` — route the two `debug` sites to `log`; pass `s.logger` from the caller (detector.go + caller in core.go)
- [x] 2.5 Convert `core.go`'s ~10 in-scope `debug()`/`warn()` calls to `s.debug()`/`s.warn()`

## 3. Collisions at discovery

- [x] 3.1 Report collisions in `newSnapshot` (thread `*slog.Logger` from `Discover`), at error severity, once per discovery (snapshot.go, engine.go)
- [x] 3.2 Make `CollectionWithDottedFacts` (and `Collection`) diagnostic-silent — remove the `reportCollectionCollision` calls from the format/query path (fact.go)

## 4. Delete the global sink

- [x] 4.1 Delete `SetDebugHandler`/`SetWarningHandler`/`SetErrorHandler` and the package-level `warn`/`debug`/`reportError` (external.go)
- [x] 4.2 Remove the `engine.SetWarningHandler`/`engine.SetDebugHandler` wiring from `app.runQuery` and `app.Run` (app.go)
- [x] 4.3 Build to confirm no remaining caller of the deleted symbols

## 5. Spec, docs, verification

- [x] 5.1 `CHANGELOG.md`: note that `WithLogger` Engines now receive cache/config/group/collision/OS-hierarchy diagnostics (including error-class); CLI output unchanged
- [x] 5.2 `go test ./...` green against the recorded baseline; `go vet ./...` clean
- [x] 5.3 `go test -race . ./internal/engine ./internal/app` (concurrent `Discover` no longer touches global handler state)
- [x] 5.4 `GOOS=linux`/`darwin`/`windows`/`freebsd go build ./...` all compile
- [x] 5.5 `openspec validate 2026-06-17-single-diagnostics-seam --strict`
