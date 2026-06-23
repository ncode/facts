## Why

Discovery input behavior is split between `internal/engine.Engine.Discover` and `internal/app.runQuery`: config selection, external fact source planning, blocklists, cache policy, query selection, and `--force-dot-resolution` are assembled in more than one place. This keeps the output and input contracts correct by duplication instead of locality.

## What Changes

- Add one package-internal discovery planning module in `internal/engine` that resolves discovery-time input policy from `EngineConfig` and parsed config on each `Discover`.
- Move cache policy ownership into the planner: it decides when persistent cache applies and which TTLs/groups are used, while `FactCache` remains the cache storage implementation.
- Move the query-selection point into the engine discovery path so `Discover(ctx, queries...)` is handled where facts are discovered, while `Projection` keeps owning query semantics.
- Keep `--force-dot-resolution` as an internal projection/query-selection bit only; it must not affect source loading, fact precedence, or the canonical Snapshot tree.
- Make `internal/app` translate CLI flags/config into engine configuration and keep stdout/stderr, timing output, strict error handling, and formatter selection at the CLI adapter.
- Preserve public `facts` APIs, CLI flags, input contract, output contract, and cache behavior. No breaking change is intended.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `facts-library-api`: Discovery must use one internal discovery plan for source policy, cache policy, and query selection while preserving `facts.Engine`, `Snapshot`, and `Discover(ctx, queries...)` behavior.
- `facts-native-input-surface`: Facts-native and facter-compatible input discovery must be planned once per discovery and shared by CLI-equivalent and library system-following engines.
- `facts-cli-option-contract`: CLI flags and config values for external facts, cache, and force-dot query projection must feed the shared discovery plan instead of duplicating discovery rules in `internal/app`.

## Impact

- **Code**: `internal/engine/engine.go`, new package-internal discovery planning code, `internal/app/app.go`, and focused tests in root library contracts, `internal/app`, cache, query/projection, and config/external fact paths.
- **Behavior**: Behavior-preserving refactor. Any observed public API, CLI output/status, input source precedence, cache TTL/group behavior, or `--force-dot-resolution` difference is a bug unless explicitly captured in a follow-up change.
- **Docs/schema**: No fact schema or changelog update expected because no user-visible behavior change is intended.
