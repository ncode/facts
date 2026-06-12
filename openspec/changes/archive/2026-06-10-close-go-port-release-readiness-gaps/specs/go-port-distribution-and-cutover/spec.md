## ADDED Requirements

### Requirement: Reproducible release artifacts
The Go port SHALL produce versioned, installable release artifacts for all supported targets.

#### Scenario: dist target builds the artifact matrix
- **WHEN** `make dist` runs
- **THEN** it MUST produce checksummed archives named `facter-<version>-<os>-<arch>` for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, and freebsd/amd64, with the version embedded in the binary and reported by `facter --version`

#### Scenario: install target
- **WHEN** `make install` runs with an optional `PREFIX`
- **THEN** it MUST install the `facter` binary into the standard binary location under that prefix

#### Scenario: Release workflow publishes artifacts
- **WHEN** a release is cut
- **THEN** a CI workflow MUST build the `dist` matrix and attach the artifacts and checksums to the release

### Requirement: Ruby entry-point cutover
The Go port SHALL define and begin the cutover from the Ruby entry points without deleting Ruby sources.

#### Scenario: bin/facter prefers the Go binary
- **WHEN** `bin/facter` runs on a system with an installed Go facter binary
- **THEN** it MUST execute that binary, and when falling back to source-based execution it MUST emit a deprecation warning naming the installation path

#### Scenario: Ruby packaging files are dispositioned
- **WHEN** the cutover plan is documented
- **THEN** `facter.gemspec`, `install.rb`, and `ext/` build metadata MUST each have an explicit recorded disposition (retain for the Ruby gem line, freeze as historical, or replace), and that disposition MUST be reflected in `docs/MIGRATION.md`

### Requirement: Go-era acceptance verification
The Go port SHALL have end-to-end acceptance verification that exercises the real binary on each supported platform.

#### Scenario: Binary-level acceptance suite
- **WHEN** the acceptance suite runs on a supported platform
- **THEN** it MUST build the real `cmd/facter` binary, execute it with representative flag combinations (default, single query, dotted query, `--json`, `--yaml`, `--show-legacy`, `--strict`), and assert the release-gate fact set and exit codes against the live host

#### Scenario: Beaker suite marked historical
- **WHEN** the Go acceptance suite is in place for the four supported platforms
- **THEN** the Ruby `acceptance/` Beaker suite MUST be documented as historical for the Go port and excluded from Go release gates

### Requirement: End-user documentation matches the Go CLI
The Go port SHALL ship user documentation that reflects the Go binary's actual behavior.

#### Scenario: Man page parity
- **WHEN** the man page is regenerated or audited against the Go CLI
- **THEN** every documented flag, default, and exit code MUST match the Go implementation, and Go-port deviations (custom-fact DSL limits, `--puppet` behavior) MUST be noted
