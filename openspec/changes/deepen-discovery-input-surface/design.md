## Context

Facts already has deep modules for projection and cache storage, but discovery input planning is still shallow. `internal/engine.Engine.Discover` resolves config, external source loading, registered facts, core facts, blocklists, and cache; `internal/app.runQuery` separately merges CLI flags with config, applies default external dirs, applies cache, then projects queries for formatting and strict mode.

This change deepens the discovery input surface without changing public behavior. The public library remains `facts.Engine` plus immutable `Snapshot`. The CLI remains the compatibility boundary. `Projection` continues to own query semantics, and `FactCache` continues to own persistent cache storage.

## Goals / Non-Goals

**Goals:**

- Put discovery-time source policy, blocklist policy, cache policy, and query selection point behind one package-internal module in `internal/engine`.
- Parse config on each `Discover`, preserving freshness for long-running engines.
- Keep `force-dot-resolution` as a projection/query-selection bit only.
- Make CLI cache/query/source behavior feed the same discovery plan as library system-following behavior.
- Preserve public `facts` interfaces, CLI flags, output, input source precedence, diagnostics, and cache behavior.

**Non-Goals:**

- No public `facts` option for `force-dot-resolution`.
- No formatter rewrite beyond removing discovery/query/cache planning from `internal/app`.
- No change to `Projection` internals, canonical tree semantics, or `FactCache` storage behavior.
- No fact schema, fact name, or supported release target change.

## Decisions

**1. Add an internal discovery planner in `internal/engine`.**

The planner should be package-internal and concrete. It should derive a per-discovery plan from `EngineConfig`, the current context/session logger, and parsed config.

The plan should own:

- effective external fact dirs
- `no-external-facts`
- blocklisted facts
- cache enabled/disabled policy
- cache TTLs and fact groups
- external loader mode and env-fact inclusion
- query list and include-typed-dotted projection mode

Rejected alternative: keep helper functions in `internal/app` and only shorten `Engine.Discover`. That would preserve the duplicated source/cache/query rules and fail the locality goal.

**2. Parse config at discovery time.**

`Engine` remains immutable configuration. Config files, default external dirs, environment facts, executable facts, and cache files remain discovery-time inputs. The planner must parse config on each `Discover` when `ConfigFile`, `SystemDefaults`, or CLI config handling requires it.

Rejected alternative: parse config in `NewEngine`. That would make long-running engines stale and contradict freshness-by-rediscovery.

**3. Move cache policy, not cache storage.**

The planner decides whether cache applies and which TTLs/groups apply. `FactCache` remains the implementation that resolves and writes cache entries.

For CLI behavior, the `--no-cache` flag and config-derived TTL/groups must reach the planner. Timing output still belongs in `internal/app`; the CLI can format timing from the selected facts returned by discovery.

Rejected alternative: absorb `FactCache` into the planner. `FactCache` already has useful depth around file storage, freshness, groups, and diagnostics; merging it would make the new module too broad.

**4. Move query selection point into discovery, keep projection semantics in `Projection`.**

`Discover(ctx, queries...)` already accepts queries. The planner should make that parameter meaningful by applying `Projection.Select` at the end of discovery, before the Snapshot/CLI consumes selected facts. `Projection` still owns reverse precedence, dotted fallback, wildcard matching, missing-query detection, and selected-query value extraction.

Rejected alternative: leave query projection in `internal/app`. That keeps CLI and library discovery paths divergent and makes cache placement depend on CLI code.

**5. Treat `force-dot-resolution` as projection policy only.**

The flag/config value exists so operator-supplied dotted external or registered facts can be interpreted as structured paths for output/query projection. It must not affect source loading, fact precedence, or the canonical Snapshot tree. The planner may carry it as an internal include-typed-dotted bit for query selection and selected CLI output.

Rejected alternative: make `force-dot-resolution` a public library option. That exposes a CLI compatibility escape hatch as a library interface without a demonstrated consumer.

**6. Keep the CLI as an adapter.**

`internal/app` should continue to own process-edge behavior: flag parsing, help/man/version tasks, stdout/stderr, diagnostic rendering, timing lines, formatter selection, strict exit behavior, and fast-path version query handling. It should stop owning discovery source/cache/query planning.

Rejected alternative: move all CLI state into `internal/engine`. That would make the engine own process-edge output behavior and blur the compatibility boundary.

## Risks / Trade-offs

- **Cache ordering drift** -> Add tests that compare cached queried output, full output, TTL group behavior, and `--no-cache` behavior before and after the planner move.
- **Canonical Snapshot tree accidentally changes under force-dot resolution** -> Keep force-dot resolution out of `newSnapshot` canonical collection and pin format/query-only behavior in CLI and projection tests.
- **Config diagnostics change timing or destination** -> Keep config parsing under the engine logger for discovery, and retain CLI log handler behavior for process-edge diagnostics.
- **Strict query behavior changes** -> Keep strict missing-query checks routed through `Projection.MissingQueries` and cover nil external/registered facts separately.
- **Planner grows into a grab bag** -> Keep renderer, diagnostics formatting, and cache storage outside the planner.

## Migration Plan

1. Add planner tests for effective external dirs, config-derived blocklists, cache TTL/groups, no-external-facts, loader mode/env inclusion, and include-typed-dotted projection policy.
2. Extract existing `Engine.Discover` planning code into the internal planner without changing behavior.
3. Move cache policy into the planner while keeping `FactCache` calls in the discovery path.
4. Move query selection into the discovery path using `Projection.Select`.
5. Change `internal/app` to pass CLI/config cache, external source, blocklist, and force-dot values into engine configuration instead of applying discovery planning itself.
6. Keep formatter selection, timing output, and strict error handling in `internal/app`, fed by the selected facts from discovery/projection.
7. Run targeted library, cache, query/projection, config/external, and CLI contract tests before the full Go test/vet pass.

## Open Questions

(none)
