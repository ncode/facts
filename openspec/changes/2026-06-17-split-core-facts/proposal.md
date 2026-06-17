# Split core facts into per-category resolver modules

## Why

`internal/engine/core.go` is one file of 6,256 lines, 369 functions, and 99 `runtime.GOOS` comparisons, fronted by a 195-line `buildCoreFacts` that inlines ~50 resolver calls and a long `[]ResolvedFact` literal (`core.go:117-312`). Every fact category and every platform's logic for it share that one file, so:

- Changing networking means scrolling past memory, ssh, dmi, Windows registry parsing, and macOS system_profiler code. Locality is zero.
- The orchestrator is a second place every category touches — adding a fact edits both a helper and the 195-line literal.

The project already proved the better shape: `virtual.go`, `ec2.go`, `gce.go`, and `az.go` are per-category files using the `currentXInput` (impure probe) + `detectX` (pure logic) seam. Core's other categories never got it. This change extends that existing convention to the rest of core. It is **behavior-preserving** — no fact, value, ordering, format, or flag changes — recorded as ADR-0010.

## What Changes

- **Carve `core.go` into category-owned modules** — `networking.go`, `processors.go`, `memory.go`, `os.go`, `dmi.go`, `disks.go`, `ssh.go`, `selinux.go`, `identity.go`, `uptime.go`, plus the small standalone categories (`augeas.go`, `xen.go`, `fips.go`, `timezone.go`). Each module holds that category's logic for **all** supported platforms, including the pure `parse*`/`*ForPlatform` functions that are the cross-platform test surface. `core.go` keeps orchestration, `facterversion`, `path`, and genuinely shared helpers.
- **Give each category a package-internal assembly function** — `networkingCoreFacts(s *Session) []ResolvedFact`, `processorsCoreFacts(s)`, etc. Each function returns only its category's `ResolvedFact` values and does not call `CoreFacts`, `buildCoreFacts`, or another category's assembly function. `buildCoreFacts` becomes the ordered composition of those calls.
- **Hybrid only when needed**: split a category by platform *for size* using a **non-GOOS** suffix (e.g. `networking_msft.go`), never `*_windows.go`/`*_linux.go`/`*_darwin.go`/`*_freebsd.go` — those impose implicit build constraints and would exclude the cross-platform parse logic from other platforms' builds and tests, breaking the `goos`-string test seam (ADR-0010).
- **No signature changes.** The `commandRunner`/`fileReader` → `s.*` parameter collapse stays the separate deferred follow-on (deepen-engine-internals design); this change only relocates functions and adds the per-category assembly funcs.

## Capabilities

### New Capabilities

(none observable) — the structural constraint is added under an existing capability so the refactor is verifiable; no new fact, format, flag, or dependency.

### Modified Capabilities

- `go-port-supported-platform-facts`: a new requirement, "Core facts are assembled by category for independent testing", pins that core-fact resolution is organized into per-category modules each exposing an assembly function, so a single category resolves and is testable in isolation — while the resolved fact set, names, values, ordering after collection, and platform behavior remain identical.

## Impact

- **Code**: `internal/engine/core.go` shrinks to `buildCoreFacts` (the per-category composition) plus shared helpers; ~360 resolver/parse helpers relocate into the new category files; new package-internal `*CoreFacts(s)` assembly functions per category. `core_test.go` splits along the same category lines (tests move with their functions). No public `facts` package change.
- **Behavior**: none. The output contract and input contract are untouched; verified by the existing contract/acceptance tests, a same-host before/after core-fact output comparison during the carve, and `GOOS=linux|darwin|windows|freebsd go build ./...`. No committed host-specific golden is added; volatile host values make that flaky without the separate helper-signature refactor.
- **Docs**: ADR-0010 records the convention. No `CHANGELOG.md` or `docs/schema/facts.yaml` change — nothing user-visible changes.
