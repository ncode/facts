## 1. Projection Test Surface

- [x] 1.1 Add internal projection tests for selected-query values, full canonical tree output, and dotted fact mode
- [x] 1.2 Add projection tests for CLI strict missing-query behavior, including resolved nil external/registered facts
- [x] 1.3 Add projection or Snapshot tests proving Snapshot keeps missing facts distinct from resolved nil registered/external facts

## 2. Projection Module

- [x] 2.1 Add an internal projection module that accepts resolved facts, queries, and dotted fact mode
- [x] 2.2 Move reverse-precedence selection, wildcard matching, canonical tree fallback, and selected-query value extraction behind the projection module
- [x] 2.3 Add projection methods for formatter-ready full-tree, multi-query, and single-query shapes
- [x] 2.4 Add projection method for CLI strict missing-query detection

## 3. Caller Migration

- [x] 3.1 Migrate `Snapshot.Value` to use projection semantics while preserving `ErrFactNotFound` and resolved-nil behavior
  - [x] 3.1a Acceptance: `Snapshot.Value` resolves a query without rebuilding the canonical tree per call. Today `Value` (`snapshot.go`) routes through `Select` -> `CollectionWithDottedFacts` (`query.go`), which rebuilds the whole tree on every call while the tree built once at `newSnapshot` (`snapshot.go`) goes unused. The projection must build (or reuse) the tree once per Snapshot, not once per query. Cover with a test or benchmark asserting that repeated `Value` calls on one Snapshot do not re-run tree construction.
- [x] 3.2 Migrate `internal/app` query selection and strict missing-query handling to use projection
- [x] 3.3 Migrate `BuildFormatter` and formatter internals to render from projection-shaped values
- [x] 3.4 Keep persistent cache behavior unchanged; only adjust placement if selected-query projection still leaks through `internal/app`

## 4. Cleanup

- [x] 4.1 Delete duplicated projection helpers once callers no longer use them
- [x] 4.2 Keep legacy rendering string transformations local to the legacy formatter
- [x] 4.3 Run `gofmt -w` on edited Go files

## 5. Verification

- [x] 5.1 Run targeted tests for `./internal/engine`, `./internal/app`, and root package Snapshot behavior
- [x] 5.2 Run `go test ./...`
- [x] 5.3 Run `go vet ./...`
- [x] 5.4 Confirm no user-visible output, schema, or changelog changes are required
