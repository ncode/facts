## 1. Parity Ledger Integrity

- [x] 1.1 Add test-existence verification to `tools/parity-ledger`: every test-name prefix in a `-run` pattern must match at least one `func Test...` in `*_test.go`; failed references get a distinct disposition and fail `parity-ledger-check`.
- [x] 1.2 Add the `blanket-coverage` disposition for references whose command has no `-run` filter; make `parity-ledger-check` fail on in-scope `blanket-coverage` entries without a documented waiver in `docs/MIGRATION.md`.
- [x] 1.3 Replace the blanket `go test ./... -count=1` references (MIGRATION.md bulk-closure entries, ~30 Windows/FreeBSD/framework specs) with focused test references or recorded waivers.
- [x] 1.4 Fix mismapped ledger entries, starting with `spec/facter/resolvers/freebsd/freebsd_version_spec.rb` (currently references networking tests).
- [x] 1.5 Extend the classifier so every `*_spec.rb` under `spec/` and `spec_integration/` lands in exactly one bucket (in-scope, out-of-scope with rule, unclassified); report bucket counts in the summary and fail the check on unclassified files.
- [x] 1.6 Audit the resulting unclassified/out-of-scope lists (especially `spec/framework/` subdirectories) and either add scoping rules or in-scope dispositions for each.
- [x] 1.7 Add unit tests for the new verification, dispositions, and classifier accounting in `tools/parity-ledger`; regenerate `docs/PARITY_LEDGER.md`.

## 2. Custom-Fact DSL Contract And Diagnostics

- [x] 2.1 Write the DSL compatibility contract document (supported constructs, unsupported constructs, migration path per unsupported pattern) under `docs/`.
- [x] 2.2 Add load-time detection in `internal/facter/custom.go` for `Facter.add`/`define_fact` blocks whose setcode matches no supported pattern; emit a warning naming file, fact, and reason; keep loading remaining facts.
- [x] 2.3 Add detection and warnings for unsupported confine blocks; treat the resolution as not suitable.
- [x] 2.4 Add detection and warnings for `on_flush` and `require` in custom fact files.
- [x] 2.5 Warn and skip `.rb` files found in external-fact directories (`internal/facter/external.go`).
- [x] 2.6 Add tests proving supported constructs emit no diagnostics and each unsupported construct emits exactly one actionable warning; run focused loader benchmarks to confirm no hot-path regression.
- [x] 2.7 Write the fact-author migration guide (audit workflow using the new warnings, rewrite recipes to executable external facts, `FACTERLIB`/`--custom-dir` distribution guidance).

## 3. CI Platform Gates

- [x] 3.1 Make `tools/windows-release-gate.ps1` exit status fail the Windows job in `.github/workflows/unit_tests.yaml`; document it as a release gate in `PORTING.md`.
- [x] 3.2 Confirm the Windows gate covers the Windows release-gate fact set through the built CLI binary; extend the script if facts are missing.
- [x] 3.3 Add an automated FreeBSD CI job (hosted VM action, with Cirrus fallback documented) running platform-sensitive package tests plus the FreeBSD release-gate fact-set smoke; make it blocking.
- [x] 3.4 Align the FreeBSD CI smoke with `make lima-freebsd-smoke` so both verify the same fact set from one definition.
- [x] 3.5 Remove openbsd/netbsd/dragonfly/solaris/aix from the cross-compile matrix in `.github/workflows/integration_tests.yaml`.
- [x] 3.6 Record the first green Windows and FreeBSD gate runs in `docs/MIGRATION.md`, closing prior-change task 9.6.

## 4. Distribution And Cutover

- [x] 4.1 Add `make dist` building checksummed `facter-<version>-<os>-<arch>` archives for the seven supported os/arch pairs, with the version embedded and reported by `--version`.
- [x] 4.2 Add `make install` honoring `PREFIX`.
- [x] 4.3 Add a release CI workflow that builds the dist matrix and attaches artifacts and checksums to releases.
- [x] 4.4 Update `bin/facter` to prefer an installed Go binary and emit a deprecation warning on source-based fallback.
- [x] 4.5 Record explicit dispositions for `facter.gemspec`, `install.rb`, and `ext/` in `docs/MIGRATION.md`.
- [x] 4.6 Add a binary-level acceptance test package that builds `cmd/facter` and asserts the release-gate fact set, output formats, and exit codes on the live host; wire it into the four platform CI gates.
- [x] 4.7 Document the Ruby `acceptance/` Beaker suite as historical for the Go port.
- [x] 4.8 Audit/regenerate `man/man8/facter.8` against the Go CLI, noting Go-port deviations.

## 5. Framework Fidelity

- [x] 5.1 Extend `--puppet` (`internal/facter/puppet.go`) to search Puppet default plugin-fact destination paths for external facts per platform, with tests.
- [x] 5.2 Emit the Ruby-plugin-custom-facts warning under `--puppet`; document the deviation in the DSL contract and man page.
- [x] 5.3 Build a `facter.conf` fixture corpus and bake off Go HOCON parser libraries against Ruby `Hocon.load` behavior; record the decision.
- [x] 5.4 Swap `internal/facter/config.go` internals to the chosen parser (or document and test the pinned subset if no library passes the corpus), keeping all existing config tests green.
- [x] 5.5 Run focused config-parsing benchmarks before and after the swap.

## 6. Verification And Closeout

- [x] 6.1 Run `go test ./... -count=1`, `go test -race ./...`, gofmt, `go vet ./...`, and `git diff --check`; record results.
- [x] 6.2 Regenerate the parity ledger and confirm `parity-ledger-check` passes with the hardened rules (no failed references, no unwaived blanket coverage, no unclassified specs).
- [x] 6.3 Verify all four platform CI gates (Linux, macOS, Windows, FreeBSD) are green and blocking on a single change.
- [x] 6.4 Produce a `make dist` artifact set from CI and smoke-test one archive per platform gate.
- [x] 6.5 Update `docs/MIGRATION.md` and `PORTING.md` with the release-readiness summary; close prior-change tasks 10.2/10.3 where satisfied and note what 10.5 still awaits.
