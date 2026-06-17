## Why

Query selection and output projection are spread across `internal/app`, `Snapshot`, `query.go`, `fact.go`, and the formatter module. The same nil-vs-missing, dotted fact, selected-query, and canonical tree rules are reassembled by multiple callers, which keeps the output contract correct only by duplication.

## What Changes

- Add one internal projection module that owns query selection, canonical tree lookup, dotted fact mode, selected-query values, and strict missing-query detection.
- Keep formatter adapters focused on rendering JSON, YAML, HOCON, and legacy text from the projection rather than rebuilding projection rules themselves.
- Route `Snapshot.Value` and the `facts` CLI strict-mode path through the same projection semantics, while preserving the library distinction between missing facts and nil-valued registered/external facts.
- Preserve the existing public Facts interface and CLI output/status behavior. No new user-visible flags, fact names, formats, or dependencies.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-framework-parity`: Query and formatter behavior remains unchanged, but selected-query projection, dotted fact mode, nil rendering, and strict missing-fact handling must be centralized behind one internal module.
- `facts-library-api`: Snapshot query behavior remains unchanged, but Snapshot value lookup must use the same projection semantics as the CLI where their contracts overlap.

## Impact

- **Code**: `internal/engine/query.go`, `internal/engine/fact.go`, `internal/engine/snapshot.go`, `internal/engine/formatter.go`, `internal/app/app.go`, and focused tests around query, formatter, Snapshot, and CLI strict behavior.
- **Behavior**: No intended user-visible behavior change.
- **Docs/schema**: No fact schema or changelog update expected unless implementation exposes an unintended behavior difference.
