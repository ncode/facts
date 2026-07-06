# Design: deepen-cli-intake-and-gating-descriptors

## Context

Three verified locality findings from the 2026-07-06 architecture review, all behavior-preserving consolidations of shipped code:

1. **Disable/gating knowledge is encoded six ways** with no source of truth: the `gate()` string literals in `buildCoreFacts` (core.go:78-104), the `BuiltinFactGroups` group→fact table (groups.go:23-35) whose spellings must agree with the gate literals ("processor"→"processors", "operating system"→"os") with nothing enforcing it, the `FilterDisabledFacts`/`pruneDisabledDescendants` hierarchy walks (groups.go:220-274), two more independent hierarchy walks in the ambient diagnostic (`unionDisabledFacts` descendant walk, discovery_plan.go:111-118; `ambientDisableSource` ancestor walk, engine.go:135-151), and the hard-coded gated-category list in core_gating_test.go:27-30. `add-fact-disable-controls` (ADR-0015, commit 129f7ad5) landed this machinery correctly but distributed.
2. **The option vocabulary is declared three ways**: the `optionDefinitions` registry (options.go:31-245, 27 canonical entries — validation/help/man), `runQuery`'s hand-built FlagSet (app.go:220-255, 28 declarations), and the partial hand walkers `configPathFromArgs`/`externalDirsFromArgs` (app.go:88-140) used by the `--list-*-groups` tasks. The query path has no drift today, but sync is implicit, and the list path silently ignores 20 of 22 options with no test locking either behavior.
3. **The app mirrors engine planning**: app.go:272-291 re-derives external-dir resolution (CLI → config → process defaults) line-for-line from `planDiscovery` (discovery_plan.go:43-67), solely to feed `canUseVersionQueryFastPath`; `factGroups` (app.go:61-86) re-derives config/external-dir planning a second time.

Constraints: the CLI process edge is the only Ruby Facter compatibility boundary (ADR-0001/0009) — every stderr byte, conflict message (British "unrecognised"), exit status, and format shape is contract. The published `facts-cli-option-contract` spec requires the fast-path *decision* to stay CLI-owned. `man/man8/facts.8` is committed and hand-maintained; only option-name presence is tested.

## Goals / Non-Goals

**Goals:**

- One gating descriptor table in `internal/engine` that gating, group expansion, filtering/pruning, and ambient diagnostics all read; one hierarchy-walk helper; agreement enforced by test.
- One option intake: FlagSet derived from the `cli` registry, one parsed-options value consumed by every task path, hand walkers deleted.
- The app consumes engine-resolved planning (external dirs) instead of mirroring the resolution order.
- Pre-refactor pins for every currently-untested behavior the refactor could silently change.
- Absorb the deferred `add-fact-disable-controls` task 1.6 cache regression test.

**Non-Goals:**

- Query-scoped core-fact resolution (`buildCoreFacts` stays query-agnostic).
- Folding the version fast path behind `Discover`, or deleting it.
- Changing list-task option semantics (only `--config`/`--external-dir` act; kept as-is, now pinned).
- Any output-contract or input-contract change; any new user-facing option.
- A man-page generator.

## Decisions

### D1: A gating descriptor table, with `BuiltinFactGroups` as a view over it

One table (new `internal/engine` file, e.g. `descriptors.go`) with a row per core fact category: root name, optional Facter-compat group name, gating class (`standalone` / `multiOutput` / `sharedProbe` / `inlineEager`), assembly function, emitted roots, probe consumers, emits-under. `buildCoreFacts` iterates the table instead of inlining nine `gate()` calls plus eager appends; `BuiltinFactGroups()` becomes a projection of the table so its two other consumers (cache TTL bucketing at cache.go:49, `--list-*-groups` rendering) are untouched. `core_gating_test.go`'s hard-coded list migrates to assert against the table, and a new agreement test fails when gate names, group membership, or emitted roots drift.

*Why not leave the literals and just add an agreement test?* The test would be a seventh copy of the same strings. The table is the only shape where agreement is by construction and ADR-0015's per-probe language is data, not comment prose.

*Rejected:* deriving the table by reflection over resolver outputs at init — runtime cleverness for a compile-time fact; the table is small and explicit.

### D2: One hierarchy helper

The four walk implementations (root-cut filter, descendant prune, ambient descendant subsumption, ambient ancestor attribution) collapse onto one helper answering "is name X disabled by set S, and through which entry". Existing `disable_diagnostic_test.go` cases become the parity matrix; behavior byte-identical.

### D3: FlagSet derived from the registry; binding lives in `internal/app`

The `cli` registry stays pure metadata (its package doc promises "without replacing the existing parser"). `internal/app` gains a builder that iterates `cli` option metadata and binds each canonical option (and aliases) into one `parsedOptions` struct — bool/value/repeated binding chosen by the existing arity metadata. A parity test asserts every non-task registry entry has a binding and the FlagSet accepts nothing the registry doesn't name. `runQuery`'s 28 hand declarations are deleted.

*Rejected:* moving parsing into `internal/cli` — it would swallow `flag` semantics the app owns (task dispatch, config interplay) and widen the `cli` interface for one consumer; the registry-drives-the-parser property is what the spec needs, not a package move.

### D4: List tasks route through the same intake; semantics frozen

`factGroups` receives the same `parsedOptions` (it reads only config path and external dirs from it); `configPathFromArgs`/`externalDirsFromArgs` are deleted. Current semantics — every other option inert on list tasks — are preserved and pinned by new tests *before* the walkers are removed.

*Rejected:* honoring all options uniformly on list tasks — an observable behavior change (e.g. `--no-external-facts` would start dropping external groups from `--list-cache-groups`), out of scope for a behavior-preserving change; if wanted later it is its own proposal.

### D5: The fast-path gate asks the engine for resolved planning

The engine exports one pure planning seam (mirroring the existing `DisabledUnion` precedent) that answers "given CLI dirs, config, and no-external-facts, which external dirs would discovery load" — the same code path `planDiscovery` uses. app.go:272-291 is deleted; `canUseVersionQueryFastPath` consumes the seam's answer. The decision and formatter selection stay in `internal/app`, so the published requirement's ownership sentence stands; only its mechanics are amended (delta spec).

*Rejected:* folding the fast path behind `Discover` — `buildCoreFacts` is not query-scoped; any query resolves every category including cloud-metadata HTTP (100ms–5s timeouts on cloud hosts), so the fold would need query-scoped resolution first, which is out of scope. *Rejected:* deleting the fast path — same latency reason; `facts facterversion` is the documented cheap path.

### D6: Pins land first (tests-first ordering)

Every behavior the refactor could silently change gets a pinning test before any production edit: list-path inertness and walker arity-skip, `--no-json`/`--no-yaml`/`--no-hocon` standalone acceptance, runtime-vs-validation error prefix asymmetry at the binary layer, `--color facterversion` bytes, and the task 1.6 cache regression (disabled fact never served from cache; pruned sub-fact never persisted into a cached group).

## Risks / Trade-offs

- [Ruby-parity bytes drift during intake consolidation] → all conflict messages, prefixes, and format shapes are already or newly byte-pinned; `app_test.go` (~78 cases), `disable_test.go`, `main_test.go` must pass unmodified.
- [Descriptor table over-skips a shared probe] → gating class `sharedProbe` keeps `dmi`/`identity` eager exactly as today; `TestBuildCoreFacts_resolutionGatingSkipsProbeWork` and the preserved-behavior spec scenario guard it.
- [`--no-block` regression] → the empty-non-nil `DisabledFacts` override (app.go:294-298 → discovery_plan.go:59) is exercised by existing tests; the table must not re-introduce defaults ahead of the override.
- [Cache ordering disturbed by table-driven build] → the build/gate → filter → select → cache order in `Discover` (engine.go:248-260) is untouched; the new task 1.6 regression test locks it.
- [List-path pin reveals a disagreement about intended semantics] → the pin records today's behavior; changing it is deliberately deferred to its own change.
- [Config parse errors must still precede the fast path] → sequencing in `runQuery` (parse config → gate → discover) is preserved and covered by existing config-error tests.
- [Archive-order coupling] → `add-fact-disable-controls` must archive before this change so the `fact-disable-controls` capability exists to receive the delta.

## Open Questions

- None blocking. The one judgment call — freeze vs. unify list-task option semantics — is decided (freeze, D4) and reversible by a future change.
