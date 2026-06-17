## Context

Facts currently preserves query and formatter behavior through a cluster of shallow modules:

- `internal/app` asks `Snapshot.Facts()`, calls `SelectWithDottedFacts`, applies cache, renders with `BuildFormatter`, and checks strict missing queries with `ValueForQuery`.
- `Snapshot.Value` separately calls `Select` and applies its own nil-vs-missing interpretation.
- The formatter module repeats the "full canonical tree vs selected-query map vs single scalar" decision for JSON, YAML, HOCON, and legacy text.
- `ResolvedFact` carries discovery data plus projection state (`UserQuery`, `Type: "nil"`, source type, file path), so callers know too much about the implementation.

This is an in-process deepening. It needs no new dependency and no adapter. The public Facts interface and CLI behavior must remain unchanged.

## Goals / Non-Goals

**Goals:**

- Put selected-query projection, canonical tree lookup, dotted fact mode, selected-query value maps, and strict missing-query detection behind one internal module.
- Keep Snapshot lookup, CLI strict mode, and formatter adapters using the same projection semantics where their contracts overlap.
- Preserve output contract behavior for no-query output, one query, multiple queries, nil rendering, dotted facts, and legacy text quirks.
- Leave a small test surface at the projection module's interface, then keep existing CLI/Snapshot/formatter contract tests as regression coverage.

**Non-Goals:**

- No public Facts interface changes.
- No CLI flag, formatter, fact-name, schema, cache, or external-fact behavior changes.
- No broad rewrite of `ResolvedFact` producers in core, external, cloud, or cache code.
- No migration of unrelated diagnostics or input-contract parsing.

## Decisions

**1. Add an internal projection module.**

Create a concrete internal module, likely in `internal/engine`, that accepts resolved facts, user queries, and dotted fact mode, then exposes the query-shaped view needed by Snapshot, CLI strict mode, and formatters.

The module should own:

- reverse-precedence fact selection
- wildcard fact matching
- canonical tree fallback for nested queries
- dotted external/registered fact merge mode
- selected-query value extraction
- strict missing-query detection for the CLI
- Snapshot missing-vs-nil lookup semantics

Rejected alternative: make each formatter own a projection helper. That keeps the same shallow seam and duplicates behavior by output format.

**2. Keep renderer adapters narrow.**

JSON, YAML, HOCON, and legacy text should render values from the projection instead of deciding whether the input is a full tree, query map, or scalar. The legacy renderer can keep its Ruby-compatible string transformations; those are rendering implementation, not projection behavior.

Rejected alternative: preserve `BuildFormatter.Format([]ResolvedFact)` and add more helper functions around it. That leaves callers passing raw discovery records through the formatter seam.

**3. Do not expose projection publicly.**

The projection module is internal implementation. `facts.Snapshot`, `facts.As[T]`, and CLI behavior stay as they are. The public library interface remains the canonical tree and pure query/decode operations.

Rejected alternative: add a public query result type. There is no consumer need, and it would make internal output-contract mechanics part of the public Facts interface.

**4. Keep cache placement conservative.**

The first implementation may keep persistent cache resolution on `[]ResolvedFact` if moving it would broaden the change. The projection module should sit at the point where selected facts are interpreted for output/strict/Snapshot behavior. If cache ordering forces projection leakage back into `internal/app`, revisit the placement during implementation.

## Risks / Trade-offs

- **Formatter output drift** -> Keep existing formatter tests and add projection-level tests before migrating renderer internals.
- **Snapshot nil semantics blur with CLI strict semantics** -> Projection must model both contracts explicitly: Snapshot treats resolved nil registered/external facts as found; CLI strict mode treats nil selected values as missing.
- **Large diff through formatter tests** -> Migrate callers first, then delete duplicate helpers only after tests pass.
- **Over-deepening** -> Do not redesign all `ResolvedFact` producers; concentrate on projection behavior only.

## Migration Plan

1. Add projection tests that pin strict missing-query behavior, Snapshot missing-vs-nil behavior, dotted fact mode, and selected-query formatter shapes.
2. Add the internal projection module with the smallest interface needed by Snapshot, CLI strict mode, and formatter adapters.
3. Migrate `Snapshot.Value` to use projection.
4. Migrate `internal/app` strict missing-query handling to use projection.
5. Migrate formatter factory/renderers to consume projection-shaped values.
6. Delete duplicated projection helpers only when no callers remain.
7. Run targeted query/formatter/app/library tests, then `go test ./...` and `go vet ./...`.

## Open Questions

- Should the cache step receive raw selected facts or a projection value? Default: keep cache raw unless implementation proves it leaks projection semantics back into app code.
