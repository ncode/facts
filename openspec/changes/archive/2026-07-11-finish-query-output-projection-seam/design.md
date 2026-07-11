## Context

`Projection` already owns reverse-precedence query selection, wildcard matching, canonical-tree fallback, dotted external/registered fact mode, output shape, selected-query values, and strict missing-query classification. The remaining seam is raw: the internal Snapshot stores a Projection but exposes `Snapshot.Facts()` so `internal/app` can pass `[]ResolvedFact` to the formatter, inspect query names for timing, and construct another Projection for strict mode. Formatter adapters then construct a Projection from those records.

This is an in-process, behavior-preserving deepening. It must respect four distinct Projection lifetimes rather than pretending one object can serve every contract:

1. Discovery-time Projections select queries before and after persistent cache resolution, using the discovery plan's dotted-fact mode.
2. The canonical Snapshot Projection always uses the canonical non-force-dot tree and memoizes that tree for public `Value` lookups.
3. The normal CLI presentation Projection uses the CLI/config force-dot mode and a defensive copy of the Snapshot's selected records.
4. The version-query fast path has no Snapshot and builds a synthetic one-fact presentation Projection before using the normal formatter-selection seam.

The active discovery-input change keeps formatter selection, timing output, strict diagnostics/status, and the version fast-path decision in `internal/app`. This change preserves that ownership and does not duplicate its capability delta while the change remains active.

## Goals / Non-Goals

**Goals:**

- Hide raw Snapshot records behind one defensive presentation Projection seam.
- Keep raw-returning query selection package-private so `internal/app` cannot recover the retired record seam from that Projection.
- Make formatter adapters consume a prebuilt Projection rather than reconstructing projection semantics from `[]ResolvedFact`.
- Reuse one normal-query presentation Projection for formatting, timing names, and strict missing-query classification.
- Keep Projection-owned shape, selected-value, query-name, and missing-query logic local to the Projection module.
- Preserve Snapshot immutability and every existing public and CLI contract.

**Non-Goals:**

- No attempt to collapse discovery selection, canonical Snapshot, CLI presentation, or version-fast-path Projections into one object or one computation.
- No public Facts API or public Snapshot method change.
- No change to discovery planning, cache ordering, `ResolvedFact` producers, source precedence, canonical-tree semantics, query grammar, or force-dot behavior.
- No move of formatter selection, timing rendering, strict diagnostics/status, or fast-path decisions out of `internal/app`.
- No universal renderer value that erases legitimate JSON/YAML versus HOCON/legacy scalar and container differences.
- No new formatter abstraction, dependency, ADR, format, fact, or schema entry.

## Decisions

**1. Snapshot will expose a defensive output Projection, not raw facts.**

Add an internal-engine method callable by `internal/app`, conceptually `OutputProjection(includeTypedDotted bool)`, that creates a Projection over a deep clone of the Snapshot's selected facts. Remove `Snapshot.Facts()` once its callers and defensive-copy tests migrate.

The output Projection must not return or share the canonical Snapshot's mutable maps, slices, pointers, or exported struct fields. Returning the Snapshot's existing Projection directly was rejected because formatter code and user-defined values must not be able to mutate Snapshot state. Reusing the canonical Projection for force-dot output was also rejected because the canonical tree deliberately has different dotted-fact semantics.

**2. Formatter adapters will consume a Projection.**

Change the formatter seam from `Format([]ResolvedFact)` to formatting a prebuilt Projection. Dotted-fact mode moves out of formatter options because it is already fixed by the presentation Projection. JSON, YAML, HOCON, and legacy adapters retain their format-specific rendering and byte-level quirks; they read shape and values from Projection instead of constructing one.

`Formatter` currently has one concrete `formatterFunc` implementation and a `Name` method used only by tests. Keep `BuildFormatter` as the formatter-selection boundary required by the CLI contract, but return and invoke a concrete formatter function/type instead of retaining the one-implementation interface. Selection tests will pin output and errors rather than a test-only name method.

Keeping raw-fact compatibility overloads was rejected because it would preserve the shallow seam. Moving format-specific scalar/container behavior into Projection was rejected because that would couple the domain module to presentation formats.

**3. Projection will expose only the additional view needed by the CLI adapter.**

Strict missing-query detection will operate on the Projection's own backing selection rather than accepting a second `[]ResolvedFact`. Projection will also provide the ordered presentation names needed by timing output: each selected `UserQuery`, falling back to the fact `Name` for full output. Shape and selected-query-map helpers currently owned by the formatter file will move beside Projection.

The current raw-returning `Projection.Select` operation will become package-private because only discovery and canonical lookup inside `internal/engine` need it. Normal CLI and formatter paths will use only presentation shape, value, name, and missing-query views. A structural search will reject `Snapshot.Facts()`, exported raw selection, or `[]ResolvedFact` in those normal presentation paths; the version fast path remains the sole documented app-side exception because it creates a new synthetic fact rather than escaping Snapshot records.

Receiver-owned missing classification will retain its existing CLI-specific semantics: selected registered/external facts resolved to nil are missing even though public Snapshot lookup treats them as found; missing names preserve selection order and duplicates; full-output records with an empty `UserQuery` are ignored. Ordered presentation names likewise visit every selected record in order, retain duplicates, and use `UserQuery` before falling back to `Name`.

Projection shape will keep the current formatter contract: no records are empty output; full-output records with empty `UserQuery` form a full tree; one distinct non-empty query is scalar even if repeated; and multiple distinct queries form a query map. Format adapters retain their existing empty and scalar bytes.

Exposing raw records or a general record iterator was rejected because timing and strict mode do not need values, source type, or file metadata.

**4. The CLI will create one normal presentation Projection and retain process-edge ownership.**

After `Discover`, `internal/app` obtains one presentation Projection using the effective force-dot setting. It passes that Projection to the selected formatter, reads its ordered names for timing lines, and asks it for missing queries before emitting the existing strict diagnostics and status. Output ordering remains unchanged: timing lines, formatted output, then strict diagnostics/status.

**5. The version fast path remains a separate synthetic adapter.**

The fast path will construct a one-fact Projection for `facterversion` and pass it through `BuildFormatter`. It continues to ignore color and force-dot options, bypass full discovery only under the existing eligibility rules, and preserve all byte and fall-through behavior. A new one-use `VersionProjection` helper was rejected; the synthetic record is not a Snapshot escape and does not justify another interface.

## Risks / Trade-offs

- **Snapshot state is accidentally shared with presentation** -> Build the presentation Projection over the existing deep-clone path and migrate the current map, pointer, alias, and cycle defensive-copy tests.
- **Canonical and force-dot modes are conflated** -> Test that force-dot presentation changes selected/output projection only and never changes `Snapshot.Tree` or `Snapshot.Value`.
- **Formatter output drifts during signature migration** -> Keep byte-pinned JSON, YAML, HOCON, legacy, color, nil, empty, single-query, and multi-query tests at the formatter and CLI contract surfaces.
- **Timing names, duplicate queries, shape, or strict ordering changes** -> Add focused Projection and CLI tests that pin record-order timing names, duplicate handling, empty/full/scalar/map shape, missing-query diagnostics, and status after the shared Projection migration.
- **Tests preserve the retired raw seam through compatibility helpers** -> Migrate formatter tests to construct Projections explicitly; do not add production or test-only raw-fact formatter overloads.
- **The one-implementation formatter interface survives only for its test-only name method** -> Preserve `BuildFormatter`, collapse the interface to a concrete callable type, and make selection tests assert behavior.

## Migration Plan

1. Archive `deepen-discovery-input-surface` so its CLI ownership and input-surface requirements are the baseline for this change.
2. Add Projection and Snapshot tests for ordered presentation names, receiver-owned missing queries, duplicate/shape semantics, distinct dotted modes, and defensive output Projection copies.
3. Make raw-returning selection package-private; change formatter adapters and their tests to consume presentation views from Projection while preserving format-specific rendering; and collapse the one-implementation formatter interface while retaining `BuildFormatter`.
4. Change `internal/app` normal query handling to obtain and reuse one presentation Projection for formatting, timing names, and strict classification.
5. Adapt the synthetic version fast path to the Projection formatter seam and retain its byte and fall-through pins.
6. Remove `Snapshot.Facts()`, exported raw selection, raw formatter signatures, the one-implementation formatter interface and test-only name method, obsolete dotted formatter options, and misplaced Projection helpers after no callers remain.
7. Run focused tests, the full suite, the concurrency-sensitive race suite, `go vet`, `make build`, and strict OpenSpec validation.

Rollback is a source revert because there is no persisted data or external migration.

## Open Questions

(none)
