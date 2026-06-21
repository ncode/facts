## Why

Facts has several shallow internal contracts that now cost us real time: schema interpretation is duplicated, platform vocabulary is repeated across tests/docs/build tooling, CLI option metadata has already drifted, and host probe behavior still leaks through category modules.

This change deepens those contracts without changing the public Facts library API or the canonical structured fact tree.

## What Changes

- Add one internal schema contract used by schema conformance and supported-facts generation.
- Tighten schema matching so dynamic keyed maps do not hide undocumented emitted leaves unless the schema explicitly marks an open subtree.
- Add one internal platform target profile for target identity, support tier, platform vocabulary, and coarse capability policy.
- Keep category-oriented resolver modules intact; do not introduce GOOS-suffixed resolver splits.
- Keep `Session` as the discovery-run module, but extend the host probe seam where category modules currently call host/runtime APIs directly.
- Add one small CLI option contract so validation, help/man text, and runtime option handling share the same supported option vocabulary.
- Fix known CLI option documentation drift, including `--force-dot-resolution`.

## Capabilities

### New Capabilities

- `facts-cli-option-contract`: Covers consistency between accepted `facts` CLI options, validation, help/man surfaces, and option metadata used by the app runner.

### Modified Capabilities

- `facts-schema`: Schema conformance and supported-facts docs must use the same schema semantics, and dynamic keyed maps must not mask undocumented fact leaves.
- `go-port-supported-platform-facts`: Host I/O and platform capability policy must remain testable through the run-scoped Session/profile seams while preserving category-oriented fact assembly.
- `go-port-ci-platform-gates`: Platform target vocabulary used by build, distribution, schema, and native gates must stay aligned while preserving the distinction between compile targets, distribution targets, and validated release targets.

## Impact

- Affected code: `schema_test.go`, `tools/supportedfacts`, `internal/cli`, `internal/app`, `internal/engine/session.go`, selected `internal/engine` category modules, platform/release gate helpers, and generated supported-facts docs.
- No new runtime dependency is planned.
- No public Facts API change is planned.
- User-visible output should remain stable except for corrected help/man documentation and stricter failures for undocumented or overclaimed schema entries.
