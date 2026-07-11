## 1. Prerequisite and Projection Contract Tests

- [x] 1.1 Confirm `deepen-discovery-input-surface` is archived and use its CLI ownership and input-surface requirements as the implementation baseline.
- [x] 1.2 Add Projection tests proving receiver-owned strict classification treats selected nil registered/external facts as missing, ignores empty-`UserQuery` full output, and preserves missing-query order and duplicates without using public `LookupValue` semantics.
- [x] 1.3 Replace the internal `Snapshot.Facts()` defensive-copy tests with output-Projection tests covering maps, slices, arrays, pointers, exported struct fields, shared aliases, and cycles.
- [x] 1.4 Add tests proving the canonical Snapshot Projection remains non-force-dot while a defensive CLI presentation Projection can use force-dot semantics without changing `Snapshot.Value`, `Snapshot.Tree`, or `Snapshot.All`.
- [x] 1.5 Add Projection tests proving presentation names retain every backing record in order and duplicates with `UserQuery`-then-`Name` fallback, and pin empty, full-tree, one-distinct-query scalar, repeated-query scalar, and multi-distinct-query map shapes.

## 2. Projection and Snapshot Seam

- [x] 2.1 Make raw-returning query selection package-private; make `Projection.MissingQueries` operate on its backing selection; add the narrow ordered-name view required by timing output; and move Projection-owned shape/query-map helpers out of the formatter module.
- [x] 2.2 Add the internal Snapshot output-Projection seam over a deep clone of selected facts, preserving the existing defensive-copy guarantee for mutable and cyclic values.
- [x] 2.3 Keep discovery-time selection, canonical Snapshot, CLI presentation, and version-fast-path Projections distinct, with the dotted-fact mode and lifetime documented for each path.

## 3. Formatter Migration

- [x] 3.1 Migrate formatter factory and adapter tests to construct and pass Projections for empty, full-tree, single-query, repeated-query, multi-query, nil, dotted, colorized, and machine-format cases without adding raw-fact compatibility overloads or asserting a test-only formatter name.
- [x] 3.2 Change the formatter seam to a concrete callable formatter type that consumes a prebuilt Projection; retain `BuildFormatter`, remove the one-implementation `Formatter` interface and dotted-fact formatter option, and keep JSON, YAML, HOCON, and legacy format-specific shape and byte-rendering rules local to their adapters.
- [x] 3.3 Preserve all existing formatter byte pins, error wrapping, factory precedence, map ordering, quoting, scalar rendering, legacy transformations, and color behavior.

## 4. CLI Presentation Wiring

- [x] 4.1 Change the normal query path to obtain one defensive presentation Projection from the Snapshot and reuse it for formatter input, timing names, and strict missing-query classification.
- [x] 4.2 Add focused CLI tests that pin ordered nested-query timing lines, formatted output ordering, strict diagnostics, and strict exit status through the shared presentation Projection.
- [x] 4.3 Adapt the version-query fast path to construct its separate synthetic one-fact Projection and continue routing through `BuildFormatter`, preserving all existing format bytes, eligibility, ignored color/force-dot options, and disabled/external/timing fall-through behavior.
- [x] 4.4 Keep formatter selection, stdout/stderr writes, timing rendering, strict diagnostics/status, and the version-fast-path decision in `internal/app`.

## 5. Cleanup

- [x] 5.1 Remove `Snapshot.Facts()`, raw-fact formatter signatures, obsolete formatter dotted-mode plumbing, and superseded tests only after all production callers use the Projection seam.
- [x] 5.2 Confirm normal `internal/app` and formatter paths contain no `Snapshot.Facts`, app-visible raw selection/iterator, `Formatter` interface, test-only formatter `Name`, or `[]ResolvedFact` seam; allow only the documented synthetic version record and internal-engine discovery selection.
- [x] 5.3 Confirm no production or test-only formatter overload reconstructs Projection from `[]ResolvedFact` merely to preserve the retired seam.
- [x] 5.4 Run `gofmt -w` on every edited Go file and confirm no public Facts API, changelog, fact schema, ADR, dependency, discovery planner, cache ordering, query grammar, or platform code change is present.

## 6. Verification

- [x] 6.1 Run `rtk go test ./internal/engine ./internal/app .` for the focused engine, CLI adapter, and root Snapshot/library contracts.
- [x] 6.2 Run `rtk go test ./...`.
- [x] 6.3 Run `rtk go test -race . ./internal/engine ./internal/app`.
- [x] 6.4 Run `rtk go vet ./...`.
- [x] 6.5 Run `rtk make build`.
- [x] 6.6 Run `rtk openspec validate finish-query-output-projection-seam --strict`.
