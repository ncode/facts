## 1. Contract Tests

- [x] 1.1 Add library tests proving config-derived external dirs, blocklists, and cache TTL/group policy are recomputed on each `Discover`.
- [x] 1.2 Add library tests proving `Discover(ctx, queries...)` returns selected facts through the same projection semantics as current CLI query projection.
- [x] 1.3 Add cache tests covering full output, queried output, TTL/group policy, and disabled cache behavior through the engine discovery path.
- [x] 1.4 Add CLI contract tests proving `--external-dir`, `--no-external-facts`, `--config`, blocklists, `--no-cache`, cache TTL/groups, and `--force-dot-resolution` still produce identical output/status behavior.

## 2. Discovery Planner

- [x] 2.1 Add a package-internal discovery planner in `internal/engine` that derives effective external dirs, no-external-facts, blocklisted facts, cache policy, cache TTLs/groups, loader mode/env inclusion, queries, and include-typed-dotted projection policy.
- [x] 2.2 Move config-file parsing and default input-source resolution from `Engine.Discover` into the planner while preserving per-discovery freshness.
- [x] 2.3 Move external fact loader mode/env inclusion and recursion warning decisions behind the planner.
- [x] 2.4 Keep `EngineConfig` immutable by cloning slices/maps needed by the planner and avoiding resolved fact state on `Engine`.

## 3. Cache Policy

- [x] 3.1 Move persistent cache enablement, TTLs, and fact groups into the discovery plan.
- [x] 3.2 Apply `FactCache.ResolveFacts` and `FactCache.CacheFacts` from the engine discovery path according to the plan.
- [x] 3.3 Preserve cache diagnostics through the engine logger and CLI diagnostic handler.
- [x] 3.4 Remove cache policy application from `internal/app` once engine discovery owns it.

## 4. Query Projection

- [x] 4.1 Apply `Projection.Select` in the engine discovery path when queries are supplied.
- [x] 4.2 Carry `force-dot-resolution` only as an internal include-typed-dotted projection bit.
- [x] 4.3 Preserve the canonical Snapshot tree by keeping force-dot behavior out of discovery-time canonical collection.
- [x] 4.4 Keep strict missing-query detection routed through `Projection.MissingQueries`.

## 5. CLI Wiring

- [x] 5.1 Change `internal/app` to translate CLI/config discovery inputs into `engine.EngineConfig` instead of duplicating source, blocklist, cache, and query planning.
- [x] 5.2 Keep CLI-owned behavior in `internal/app`: help/man/version tasks, stdout/stderr, formatter selection, timing output, diagnostic rendering, strict exit behavior, and option validation.
- [x] 5.3 Preserve the version-query fast path behavior before full discovery.
- [x] 5.4 Delete obsolete `internal/app` helpers for config path, external dirs, cache, or projection only after no callers remain.

## 6. Verification

- [x] 6.1 Run focused tests for library discovery, cache, config/external facts, projection/query, and CLI contracts.
- [x] 6.2 Run `gofmt -w` on edited Go files.
- [x] 6.3 Run `go test ./...`.
- [x] 6.4 Run `go vet ./...`.
- [x] 6.5 Run `openspec validate deepen-discovery-input-surface --strict`.
