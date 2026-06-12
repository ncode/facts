## Why

A deep review of the Go port (2026-06-10) confirmed the engine, CLI, public API, formatters, config, cache, and built-in fact inventory are in strong shape: `go test ./...` passes across all packages and the per-platform fact surface matches Ruby for Linux, macOS/Darwin, Windows, and FreeBSD. The same review found the remaining distance to a releasable migration is not fact parity — it is four structural gaps that `complete-supported-platform-go-port` does not cover:

1. **Custom-fact DSL semantics.** `internal/facter/custom.go` simulates the Ruby DSL with 46 static regexes. Literal setcodes, `Facter::Core::Execution` command strings, basic confines, weights, and aggregates work; arbitrary Ruby in `setcode`, confine blocks with logic beyond `==`/`!=` against a literal, `on_flush`, `require`, gem/`$LOAD_PATH` fact discovery, and `.rb` external facts all fail silently. Real fleets will hit this on day one and get missing facts with no diagnostic.
2. **Ledger integrity.** `tools/parity-ledger` trusts `docs/MIGRATION.md` references without verifying the named Go test functions exist; 30+ specs are "covered" by the blanket command `go test ./... -count=1`; at least one mapping is wrong (`spec/facter/resolvers/freebsd/freebsd_version_spec.rb` points at networking tests); and `classifySpec()` silently drops some `spec/framework/` subdirectories from the 614 in-scope count.
3. **Platform validation gates.** Windows live validation (task 9.6 of the prior change) has never run as a blocking check, and FreeBSD — a release target — has zero CI coverage; it is validated only by a manual local Lima target. `integration_tests.yaml` also cross-compiles out-of-scope platforms (OpenBSD, NetBSD, DragonFly, Solaris, AIX) contrary to the project's own ground rules.
4. **Distribution and cutover.** There is no release artifact pipeline (`dist`/`install`/packaging targets), `bin/facter` is still a shell shim around `go run`, `facter.gemspec`/`install.rb` still describe the Ruby product, the Beaker `acceptance/` suite has no Go-era replacement, the man page has not been checked against the Go CLI, `--puppet` only shells out for `puppet --version` instead of loading plugin facts, and `facter.conf` is parsed with regexes instead of a HOCON library.

## What Changes

- Define an explicit, documented custom-fact DSL compatibility contract: enumerate supported constructs, detect unsupported constructs at load time, and emit actionable warnings instead of silently resolving nothing. Ship a migration guide for fact authors.
- Decide and record the disposition for each unsupported DSL area (`on_flush`, `$LOAD_PATH`/gem discovery, `.rb` external facts, complex confine blocks): implement, diagnose-and-document, or reject with a warning.
- Harden `tools/parity-ledger`: verify referenced Go test functions exist in `*_test.go`, downgrade blanket `./...` coverage references to a distinct disposition that requires per-spec confirmation, fix known mismapped entries, and make the spec classifier account for every `*_spec.rb` file so exclusions are explicit rather than silent.
- Make Windows validation a blocking CI gate (integrate `tools/windows-release-gate.ps1` pass/fail into `unit_tests.yaml`) and add automated FreeBSD validation to CI (FreeBSD VM action or equivalent), so all four release targets have automated gates.
- Remove out-of-scope platforms from the cross-compile matrix in `integration_tests.yaml`.
- Build the distribution and cutover path: `make dist`/`make install`/release packaging targets with versioned artifacts for the four targets, a `bin/facter` cutover plan, explicit disposition for `facter.gemspec`/`install.rb`/`ext/`, a Go-era end-to-end acceptance strategy replacing Beaker for in-scope platforms, and a man page regenerated from or verified against the Go CLI.
- Close framework fidelity gaps: deepen or explicitly document `--puppet` plugin-fact behavior, and replace the regex HOCON reader with a real HOCON parser or document the supported `facter.conf` subset with tests pinning the boundary.

## Capabilities

### New Capabilities

- `go-port-custom-fact-dsl-contract`: The supported custom-fact DSL surface, load-time detection and diagnostics for unsupported constructs, and migration guidance for fact authors.
- `go-port-parity-ledger-integrity`: Verification rules the parity ledger must enforce so "covered" dispositions are auditable and trustworthy.
- `go-port-ci-platform-gates`: Blocking automated validation for all four release targets and a CI build matrix limited to in-scope platforms.
- `go-port-distribution-and-cutover`: Release artifacts, installation, Ruby-entry-point cutover, acceptance testing, and end-user documentation for shipping the Go binary as facter.
- `go-port-framework-fidelity`: Remaining framework behavior gaps with user-visible impact — Puppet plugin-fact integration and HOCON config parsing.

### Modified Capabilities

- None. The capabilities introduced by `complete-supported-platform-go-port` are not yet archived into `openspec/specs/`; this change layers release-readiness requirements beside them rather than amending them.

## Impact

- Affected Go code: `internal/facter/custom.go`, `internal/facter/config.go`, `internal/facter/puppet.go`, `internal/facter/external.go`, `tools/parity-ledger/main.go`, `cmd/facter`.
- Affected build/CI: `Makefile`, `.github/workflows/unit_tests.yaml`, `.github/workflows/integration_tests.yaml`, `tools/windows-release-gate.ps1`.
- Affected packaging/docs: `bin/facter`, `facter.gemspec`, `install.rb`, `ext/`, `man/man8/facter.8`, `docs/MIGRATION.md`, `docs/PARITY_LEDGER.md`, `PORTING.md`, new fact-author migration guide under `docs/`.
- Affected tests: new diagnostics tests in `internal/facter`, ledger verification tests in `tools/parity-ledger`, new Go acceptance smoke suite, CI gate wiring.
- Out of scope: deleting the Ruby implementation (separate, explicitly approved change, unchanged from the prior proposal), adding platforms beyond Linux/macOS/Windows/FreeBSD, and embedding a Ruby interpreter unless the design decision in this change selects that path.
