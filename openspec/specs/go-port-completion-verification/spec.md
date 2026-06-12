# go-port-completion-verification Specification

## Purpose
TBD - created by archiving change complete-supported-platform-go-port. Update Purpose after archive.
## Requirements
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
- **THEN** Windows CI or an approved Windows runner MUST pass Go tests and CLI fact smoke checks for OS, system32, networking, memory, processors, DMI, uptime, virtualization, FIPS, and timezone, using structured fact names only

#### Scenario: FreeBSD validation gates
- **WHEN** maintainers validate FreeBSD behavior
- **THEN** Lima FreeBSD build/smoke coverage MUST pass and MUST be broad enough to exercise the FreeBSD release-gate fact set defined by the supported-platform facts capability

### Requirement: Benchmark and performance evidence
The Go port SHALL preserve performance discipline for hot-path changes.

#### Scenario: Hot-path benchmark requirement
- **WHEN** a change touches core fact collection, query/formatting, cache, config parsing, external fact loading, cloud metadata normalization, networking parsers, memory parsers, processor parsers, or mount/disk/partition parsers
- **THEN** maintainers MUST run repeated focused benchmarks before and after the change, record representative results in the change record (PR description or CHANGELOG), and avoid accepting unjustified regressions

#### Scenario: Cold compatibility exception
- **WHEN** a change only adds cold diagnostic compatibility or an error branch that does not run in normal collection paths
- **THEN** maintainers MAY skip benchmarks if the change record notes why no benchmark is needed

