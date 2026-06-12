# Design: Remove the Ruby custom-fact DSL layer

## Context

`internal/facter/custom.go` implements `.rb` custom-fact loading as ~40 hand-written regexes that statically extract literal values and command strings from a documented subset of the Facter Ruby DSL (no interpreter, no shell-out to Ruby). `dsl_diagnostics.go` warns on unsupported constructs. The layer feeds the engine through `LoadCustomFactsFromDirs`, which returns resolved facts plus a per-fact weight map consumed by engine precedence. The CLI exposes it through `--custom-dir`, `FACTERLIB`, the `facter.conf` `custom-dir` key, and the `--no-ruby`/`--no-custom-facts` disable flags; the library exposes it through `WithCustomDirs`. ADR-0006 decided to remove all of it; CONTEXT.md has already retired the "custom fact" term in favor of "registered fact" (programmatic `WithFact` registration, which shares no code with the parser).

## Goals / Non-Goals

**Goals:**
- Zero `.rb` reading anywhere in the codebase; the DSL parser, its diagnostics, and its tests are gone.
- The CLI argument surface matches the new reality: removed flags fail as unknown options, help/man text no longer mentions them.
- The output contract and the remaining input contract (external facts, `facter.conf` semantics for surviving keys) are provably untouched — existing external-fact and formatter tests pass unmodified.
- One honest operator-facing doc (`CUSTOM_FACT_MIGRATION.md`) explains the stance and the migration path.

**Non-Goals:**
- No replacement Ruby support of any kind (no mruby, no shell-out, no "lite" subset).
- No changes to external-fact loading, precedence, caching, formatting, or `facter.conf` parsing beyond deleting the three key patterns.
- No removal of `facter --puppet` warning behavior (the flag still warns that Puppet plugin facts are not loaded).
- No rewrite of `docs/PARITY_LEDGER.md` — it is a frozen historical record per `go-port-ruby-removal`.
- No renaming sweep of "custom" identifiers that are private and external-fact-related (e.g. variable names in tests that still read clearly).

## Decisions

**1. Hard removal of flags, not accepted no-ops.**
`--custom-dir`, `--no-ruby`, and `--no-custom-facts` are deleted from the flag parser along with their mutual-conflict checks. Rationale (ADR-0006): with zero adoption there is nobody to protect from a usage error, and zombie flags misrepresent what the binary does. Alternative considered: keep flags as accepted no-ops ("permanent `--no-ruby` mode," which upstream documents) — rejected as dishonest surface for a days-old project.

**2. `facter.conf` keys become inert by deletion, not by special-casing.**
`internal/facter/config.go` extracts known keys via regex and ignores everything else silently. Deleting the `customDirPattern`, `noRubyPattern`, and `noCustomFactsPattern` extractions makes the keys behave exactly like any other unrecognized key — no new warning path, no error path, no code.

**3. Weight/confine machinery goes with the parser.**
The weight map and confine evaluation exist only to arbitrate between competing `.rb` resolutions. `WithFact` registers exactly one resolver per name, and external facts win wholesale, so no surviving feature needs weights or confines. Engine plumbing that threads the weight map (`LoadCustomFactsFromDirs` return values into `internal/facter/engine.go` precedence) is removed rather than stubbed.

**4. `WithCustomDirs` is removed from the public API without deprecation.**
The module is v0 and unreleased; Go has no deprecation obligation here. `WithFact` and `WithExternalDirs` are untouched. `WithSystemDefaults` no longer wires `FACTERLIB` or default custom directories.

**5. Docs collapse to one page.**
`CUSTOM_FACT_COMPATIBILITY.md` is deleted (its subject no longer exists). `CUSTOM_FACT_MIGRATION.md` is rewritten: a statement that Facts reads no `.rb` fact files, why (ADR-0006 link), and a pattern-mapping table — literal `setcode` → YAML/JSON external fact; command/`Facter::Core::Execution` `setcode` → executable external fact; `confine` → conditional logic inside the executable; `weight` → not needed (one source of truth per fact). The `--puppet` deviation note moves here and to the man page (it previously lived in the deleted contract doc, pinned by `go-port-framework-fidelity`).

**6. Test strategy: delete, retarget, add three negatives.**
Delete `custom_test.go` and `dsl_diagnostics_test.go` wholesale. Retarget engine/app/CLI tests that used `.rb` fixtures for non-DSL behavior (precedence ordering, logger wiring, dir normalization) onto external-fact fixtures so the behavior stays covered. Add negative tests: (a) `facter --custom-dir x` exits with a usage error, (b) a `facter.conf` containing `custom-dir`/`no-ruby` keys loads without error or effect, (c) a `.rb` file present in an external-fact directory is skipped exactly as before (unchanged behavior, now load-bearing as the only `.rb` touchpoint).

## Risks / Trade-offs

- [A Facter refugee evaluates the project, finds `.rb` facts ignored, files an issue] → The migration page is the documented landing spot; README positions the stance up front; the CLI fails loudly on removed flags rather than silently dropping facts.
- [Deleting shared helpers used by both custom and external paths breaks external loading] → Identify shared symbols before deleting (`rtk grep` each exported/internal symbol in `custom.go` for uses outside it); the external-fact test suite (44 tests) must pass unmodified as the guard.
- [Engine/app tests that incidentally used `.rb` fixtures lose coverage of non-DSL behavior] → Retarget those tests to external-fact fixtures in the same commit, never just delete them.
- [Spec/ledger tooling references deleted test names] → `tools/parity-ledger` was already removed by `go-port-ruby-removal`; no generator runs against current tests. Verified: ledger is frozen.
- [Future demand for `.rb` support materializes after release] → ADR-0006 records the rejected alternatives (regex subset, embedded interpreter); reinstating would be a deliberate new decision, not a revert.

## Migration Plan

Single PR, ordered so each commit builds and tests green:
1. Specs/docs first (this change's delta specs, migration page rewrite, contract doc deletion).
2. CLI surface removal (`internal/app`, `internal/cli`, config patterns) with retargeted and negative tests.
3. Library surface removal (`WithCustomDirs`, engine plumbing).
4. Parser deletion (`custom.go`, `dsl_diagnostics.go`, their tests).
5. README/CHANGELOG/man page sweep; `go test ./...`, `go test -race ./...`, `go vet ./...`, gofmt, and all platform CI gates green on the final commit.

Rollback: revert the PR; no data, schema, or on-disk format is involved.

## Open Questions

None — all decision points were resolved in the 2026-06-11 grilling session and recorded in ADR-0006.
