## Why

Query semantics are centralized in `Projection`, but the Snapshot-to-CLI formatter seam still exposes `[]ResolvedFact`: `internal/app` reads those records for formatting and timing, formatter adapters reconstruct a Projection, and strict mode constructs another one. This leaves the archived query-output projection design incomplete and exposes discovery metadata where callers need only a presentation view.

## What Changes

- Replace the internal `Snapshot.Facts()` escape with a defensive presentation Projection that hides raw resolved-fact records from the CLI formatter seam.
- Make formatter adapters consume an existing Projection, and let one normal-query presentation Projection provide formatter shape/value data, timing names, and strict missing-query classification.
- Make raw-record query selection package-private and collapse the one-implementation formatter interface while preserving `BuildFormatter` as the CLI construction seam.
- Keep discovery-time query selection, the canonical Snapshot Projection, the CLI presentation Projection, and the synthetic version-fast-path Projection distinct because they have different lifetimes and dotted-fact modes.
- Preserve the public Facts API, Snapshot immutability, canonical-tree behavior, force-dot behavior, formatter bytes, CLI stdout/stderr and status behavior, cache ordering, and the version-query fast path.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-framework-parity`: Formatter adapters must consume projection-shaped input without rebuilding query semantics from raw resolved facts, while preserving every output format contract.
- `facts-library-api`: Internal Snapshot presentation access must preserve the Snapshot's defensive-copy and immutable-result guarantees without changing its public API.

## Impact

- **Code**: `internal/engine/projection.go`, `internal/engine/snapshot.go`, `internal/engine/formatter.go`, `internal/app/app.go`, and focused projection, Snapshot, formatter, and CLI contract tests.
- **Behavior**: Behavior-preserving refactor; any public API, output byte, diagnostic, status, dotted-fact, timing, strict-mode, or version-fast-path difference is a bug.
- **Sequencing**: apply after the completed `deepen-discovery-input-surface` change is archived, because it establishes the CLI ownership boundary and overlaps `internal/app`, force-dot behavior, and query projection.
- **Dependencies/docs/schema**: No new dependency, ADR, changelog entry, fact schema update, or platform-specific validation is expected.
