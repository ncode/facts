## ADDED Requirements

### Requirement: Parity ledger completion gate
The Go port SHALL maintain an auditable ledger for all in-scope Ruby compatibility specs before release completion.

#### Scenario: Ledger coverage
- **WHEN** the Go port is evaluated for completion
- **THEN** the ledger MUST include every in-scope Ruby spec file for Linux, macOS/Darwin, Windows, FreeBSD, shared framework behavior, custom facts, external facts, and in-scope Linux distro families

#### Scenario: Ledger disposition
- **WHEN** a ledger entry is reviewed
- **THEN** it MUST identify the Ruby spec path, compatibility domain, platform, disposition, covering Go test or implementation reference, intentional deviation if any, and remaining blocker if any

### Requirement: Supported-platform verification matrix
The Go port SHALL pass a defined verification matrix before release completion.

#### Scenario: Local deterministic gates
- **WHEN** maintainers validate deterministic Go behavior
- **THEN** `go test ./...`, `go test -race ./...`, gofmt checks, `go vet ./...`, and `git diff --check` MUST pass for the tracked Go port files

#### Scenario: Linux validation gates
- **WHEN** maintainers validate Linux behavior
- **THEN** Linux Go tests, Linux distro/container smoke coverage, and Linux CLI fact smoke checks MUST pass for representative supported Linux families

#### Scenario: macOS validation gates
- **WHEN** maintainers validate macOS/Darwin behavior
- **THEN** macOS host tests and CLI fact smoke checks MUST pass for representative macOS facts, including OS, networking, memory, processors, DMI, system profiler, mountpoints, uptime, and virtualization

#### Scenario: Windows validation gates
- **WHEN** maintainers validate Windows behavior
- **THEN** Windows CI or an approved Windows runner MUST pass Go tests and CLI fact smoke checks for OS, system32, networking, memory, processors, DMI, uptime, virtualization, FIPS, timezone, and legacy aliases

#### Scenario: FreeBSD validation gates
- **WHEN** maintainers validate FreeBSD behavior
- **THEN** Lima FreeBSD build/smoke coverage MUST pass and MUST be broad enough to exercise the FreeBSD release-gate fact set defined by the supported-platform facts capability

### Requirement: Benchmark and performance evidence
The Go port SHALL preserve performance discipline for hot-path changes.

#### Scenario: Hot-path benchmark requirement
- **WHEN** a change touches core fact collection, query/formatting, cache, config parsing, custom fact loading, external fact loading, cloud metadata normalization, networking parsers, memory parsers, processor parsers, or mount/disk/partition parsers
- **THEN** maintainers MUST run repeated focused benchmarks before and after the change, record representative results, and avoid accepting unjustified regressions

#### Scenario: Cold compatibility exception
- **WHEN** a change only adds cold diagnostic compatibility or an error branch that does not run in normal collection paths
- **THEN** maintainers MAY skip benchmarks if the migration log records why no benchmark is needed

### Requirement: Release completion declaration
The Go port SHALL only be declared complete when scope, tests, docs, and cleanup readiness are explicit.

#### Scenario: Completion declaration
- **WHEN** the Go port is marked complete
- **THEN** all in-scope ledger entries MUST be resolved, all verification gates MUST pass, `PORTING.md` and `docs/MIGRATION.md` MUST reflect the final supported platform scope including FreeBSD, and Ruby cleanup MUST be left as a separate explicitly approved change

#### Scenario: Blocked completion
- **WHEN** any in-scope behavior cannot be verified or implemented
- **THEN** the blocker MUST be documented with the affected Ruby spec path, platform, reason, proposed validation path, and whether it blocks release
