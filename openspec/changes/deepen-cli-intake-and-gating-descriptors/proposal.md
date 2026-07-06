# Deepen CLI intake and gating descriptors

## Why

Two vocabularies that must stay internally consistent are each declared several times with nothing enforcing agreement. Disable/gating semantics — which fact names gate which resolvers, how groups expand, how the name hierarchy is walked — are encoded six ways across `core.go`, `groups.go`, `discovery_plan.go`, `engine.go`, `app.go`, and a hard-coded test list; the CLI option vocabulary is declared three ways (the `cli` registry, `runQuery`'s hand-built FlagSet, and partial arg walkers for the list-groups tasks), kept in sync only implicitly. On top of both, `internal/app` re-derives the engine's discovery planning (external-dir resolution, defaults) line-for-line just to feed the version fast path and the list-groups tasks. Every future change to disabling, options, or planning currently pays an N-site synchronization tax with no failing test when a site is missed — the list path already silently ignores 20 of 22 options with zero coverage.

## What Changes

- **One gating descriptor table** (`internal/engine`): a single table describing each core-fact category — root fact name, Facter-compatible group name, gating class (standalone / multi-output / shared-probe / inline-eager), resolver, emitted roots, probe consumers, emits-under — becomes the source `buildCoreFacts` gating, `BuiltinFactGroups`, disabled-fact filtering/pruning, and the ambient-disable diagnostics all read. One hierarchy-walk helper replaces the four independent ancestor/descendant walk implementations. Behavior-preserving: resolved names, gating semantics (per-probe, not per-category), group spellings, and diagnostic bytes are unchanged.
- **Registry-driven option intake** (`internal/cli`, `internal/app`): the `runQuery` FlagSet is derived from the option registry instead of hand-redeclared; all task paths consume one parsed-options value; the hand-rolled `configPathFromArgs`/`externalDirsFromArgs` walkers are deleted. Current list-path semantics (only `--config`/`--external-dir` act on `--list-*-groups`) are kept and pinned by new tests before the refactor.
- **App consumes engine planning instead of mirroring it**: the external-dir/defaults resolution duplicated in `app.go` (feeding the version fast path) and in `factGroups` moves behind engine-owned seams the app queries. The fast-path *decision* stays CLI-owned, per the published `facts-cli-option-contract` requirement — its mechanics are amended, not its ownership.
- **Pre-refactor pins** (tests first): list-path option semantics, walker arity-skip behavior, `--no-json`/`--no-yaml`/`--no-hocon` standalone inertness, runtime-vs-validation error prefix asymmetry, `--color facterversion` bytes, and the deferred `add-fact-disable-controls` task 1.6 regression test (disabled facts never served from or persisted into cache).
- No output-contract or input-contract changes. No new user-facing surface.

## Capabilities

### New Capabilities

_None — this change consolidates the implementation behind existing capabilities._

### Modified Capabilities

- `facts-cli-option-contract`: "CLI option vocabulary is shared" is strengthened — the parser's flag set is derived from the shared registry (single declaration), and list-task option semantics are pinned. "Version fast path reuses engine-owned seams" is amended — the fast-path gate consumes engine-resolved planning inputs instead of re-deriving them; the decision stays in `internal/app`.
- `fact-disable-controls`: gating, group expansion, pruning, and disable diagnostics are required to derive from one descriptor table with agreement structurally enforced by test; the cache-ordering invariant gains its deferred regression test. (This capability's spec lands with `add-fact-disable-controls`, which is code-complete and unarchived — that change must archive before this one; this change builds on its landed code, commit `129f7ad5`.)

## Impact

- `internal/engine`: `core.go` (gate calls → table-driven), `groups.go` (`BuiltinFactGroups`, `FilterDisabledFacts`, `pruneDisabledDescendants`), `discovery_plan.go` (`unionDisabledFacts` ambient walk), `engine.go` (`ambientDisableSource`), `cache.go` (group-table consumer, unchanged semantics), new descriptor table file.
- `internal/cli`: `options.go` grows whatever metadata FlagSet derivation needs; `arguments.go`/`validation.go` unchanged in behavior.
- `internal/app`: `app.go` (`runQuery` FlagSet, `factGroups`, hand walkers deleted, fast-path gate rewired), `loghandler.go` untouched.
- `cmd/facts`: unchanged behavior; error-rendering seam byte-identical.
- Tests: new pins in `internal/app` and `internal/engine`; `core_gating_test.go`'s hard-coded category list migrates to assert against the table; existing ~78 `app_test.go` cases, `disable_test.go`, `disable_diagnostic_test.go`, `builtin_groups_test.go`, `main_test.go` must stay green unmodified.
- Dependency: `add-fact-disable-controls` archives first. Independent of `add-packages-fact` and the other open changes.
