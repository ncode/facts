## Context

The Ruby implementation has been frozen since the port began; the prior changes' ground rules kept it in place until the Go CLI and public API matched Ruby behavior with green release gates. That condition is met (gates green on commit a984084, runs recorded in `docs/MIGRATION.md`; both porting changes archived 2026-06-10). A dependency survey found exactly three couplings from the Go side into the Ruby tree: `internal/facter/core_test.go` reads `spec/fixtures/zpool` and `spec/fixtures/zpool-with-featureflags`; `facter_test.go` pins `facter.Version` to `facter.gemspec`; and `tools/parity-ledger` walks `spec/`/`spec_integration/` to generate the parity ledger. Nothing in CI or the runtime touches Ruby.

## Goals / Non-Goals

**Goals:**

- Remove every Ruby source, spec, acceptance, packaging, and lint file from the repository in one auditable change.
- Keep the Go test suite self-contained (fixtures owned by `internal/facter/testdata/`).
- Preserve the historical record: the frozen parity ledger, migration log, and archived OpenSpec changes remain readable and truthful after the Ruby tree is gone.
- Leave the user-facing surface untouched: CLI flags (including `--no-ruby`), output, public API, release artifacts.

**Non-Goals:**

- No renaming or restructuring of the Go packages.
- No removal of `bin/facter` (it is a Go-binary shim now and still serves PATH-compatibility).
- No changes to the eight synced capabilities in `openspec/specs/`; they describe the Go product and stay as-is.
- No history rewrite: the Ruby code remains recoverable from git history.

## Decisions

1. **Fixtures move, tests do not weaken.** The two zpool fixtures are byte-exact Ruby-parity inputs (protected by `.gitattributes` against CRLF). They move verbatim to `internal/facter/testdata/` with `git mv` so history follows, and `core_test.go` repoints. No assertion changes.

2. **The gemspec version test is deleted, not repointed.** Its purpose was keeping the Go version synchronized with the Ruby gem version during coexistence. With the gemspec gone, `internal/facter/core.go` is the single version source, already pinned by `TestVersionString_returnsPublicFacterVersion` and the CLI-level `TestFacterCommand_version`/`TestRun_version`.

3. **The parity ledger retires with its inputs; the document freezes.** Regenerating the ledger without `spec/` would produce an empty, meaningless file. Instead `tools/parity-ledger` and its make targets are removed in the same commit that removes `spec/`, and `docs/PARITY_LEDGER.md` keeps its final generated content with a replacement header marking it a frozen historical record (generated 2026-06-10 from the last commit that contained the Ruby specs, named for auditability). The ledger requirements in `openspec/specs/go-port-parity-ledger-integrity` were satisfied by that final run; the spec remains as the record of the verification rules that were enforced.

4. **Docs are corrected, not rewritten.** `docs/MIGRATION.md` ground rules that mandate keeping Ruby and regenerating the ledger get a closing note (rules completed/obsolete as of this change) rather than deletion, preserving the log's append-only character. `PORTING.md`'s compatibility-sources and ledger-regeneration sections are replaced with pointers to the frozen ledger and archived changes.

5. **Single commit, ordinary CI validation.** The deletion, fixture move, test/tooling adjustments, and doc updates land as one commit so the tree is never in a broken intermediate state. Validation is the standard battery (full tests, race, vet, gofmt) locally plus a green run of all four platform gates after push — the same gates that protect any other change.

## Risks / Trade-offs

- **Undiscovered references to the Ruby tree.** Mitigated by the pre-survey greps (quoted paths, relative paths, symlinks) and by the fact that any miss fails loudly in the full local test run or the platform gates.
- **Loss of the Ruby reference for future parity questions.** Accepted: git history retains every file, the archive directory retains the porting specs, and the frozen ledger maps every Ruby spec to its covering Go test by name.
- **Frozen ledger drifts from reality as Go tests evolve.** Accepted and documented in its header: it is a record of the porting verification at completion, not a living artifact.
