# Design: Split core facts into per-category resolver modules

## Context

`core.go` mixes every fact category and every platform in one 6,256-line file (369 functions, 99 `runtime.GOOS` checks). `buildCoreFacts` (`core.go:117-312`) is a 195-line orchestrator: a large sorted `[]ResolvedFact` literal plus ~25 `append(facts, xFacts(...)...)` calls. The codebase already demonstrates the target shape in `virtual.go`/`ec2.go`/`gce.go`/`az.go`: a per-category file with an impure `currentXInput`/probe layer feeding a pure `detectX`/parse layer.

The 2026-06-17 deepen-engine-internals change routed host I/O through the Session seam and explicitly deferred the `commandRunner`/`fileReader` → `s.*` parameter collapse to a later change. This split is orthogonal to that collapse and must not pre-empt it.

## Goals / Non-Goals

**Goals**
- One category-owned module per fact category; each category's platforms live together.
- Each category owns its package-internal assembly (`networkingCoreFacts(s) []ResolvedFact`, `processorsCoreFacts(s)`, etc.); `buildCoreFacts` is their composition.
- A category is resolvable and testable through its own assembly function without running full core-fact discovery.
- Identical resolved fact set on every supported platform — proven by workflow comparison plus the existing contract tests, not assumed.

**Non-Goals**
- Changing function signatures (the `commandRunner`/`fileReader` collapse is the separate deferred change).
- Changing fact names, values, ordering after collection, formats, flags, or schema.
- Converting `runtime.GOOS` checks to build tags — the `goos`-string parameter is the test seam (rejected in the architecture review and ADR-0010).

## Decisions

- **Split axis: by fact category, hybrid by platform only on size.** A category module holds all platforms so a category change touches one place. Split a category by platform only when the file is genuinely unwieldy.
- **Platform-split files use a non-GOOS suffix.** Go's toolchain reads `_linux.go`/`_windows.go`/`_darwin.go`/`_freebsd.go` as implicit build constraints. The cross-platform parse logic (e.g. parsing Windows `ipconfig` output) is tested on Linux/macOS CI through the `goos` parameter; a GOOS-suffixed file would drop it from those builds. Use names like `networking_msft.go` instead. Genuinely syscall-bound, OS-only code already lives in its own tagged files (`statfs_linux.go`) and is unaffected.
- **Per-category assembly, not just helper relocation.** Each category module exposes a package-internal `*CoreFacts(s *Session) []ResolvedFact` function such as `networkingCoreFacts`. It returns only that category's facts and does not call `CoreFacts`, `buildCoreFacts`, or another category assembly function. `buildCoreFacts` calls them in an order that reproduces today's assembly. This gives each category end-to-end ownership and an isolation test surface.
- **Order preservation is verified at the canonical tree, not by literal position.** Core facts are collected into a name-keyed canonical tree, so the fact names and values must be preserved. Use a same-host before/after output comparison during the carve, plus existing deterministic parser/category tests. Do not commit a host-specific `CoreFacts` value golden: `CoreFacts` still reads runtime/env/network state that a fake host does not fully control, and controlling that belongs to the deferred helper-signature collapse.

## Risks / Trade-offs

- **A silent fact drop during the move** → guarded by the same-host before/after core-fact output comparison after each category, plus the existing contract/acceptance suites and moved category tests.
- **A GOOS-suffix slip** reintroducing an implicit build constraint → guarded by `GOOS=linux|darwin|windows|freebsd go build ./...` and by running the full test suite (cross-platform parse tests must still execute on the host CI).
- **Merge churn against in-flight work on `core.go`** → do the carve as pure cut/paste per category with a green `go test ./...` after each, so each category move is an isolated, revertable step.
- **Large diff** → unavoidable for a 6,256-line file; mitigated by per-category commits and the no-signature-change rule keeping each move mechanical.

## Migration Plan

1. Record the pre-carve baseline: current `go test ./...`, supported `GOOS` builds, and a local same-host `facts --json`/`CoreFacts` output capture for before/after comparison. Keep this capture uncommitted.
2. Per category, in order: move its resolver/parse helpers to the category file, extract the category's `*CoreFacts(s)` function, replace that section of `buildCoreFacts` with the call, run `go test ./...`.
3. After all categories: `buildCoreFacts` is the composition; `core.go` holds only orchestration + shared helpers.
4. Split `core_test.go` along the same category lines.
5. Verify: same-host comparison clean except expected volatile values, all GOOS builds compile, `go vet` clean.

## Open Questions

(none)
