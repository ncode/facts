## MODIFIED Requirements

### Requirement: Reproducible release artifacts
The Go port SHALL produce versioned, installable release artifacts for all supported targets.

#### Scenario: dist target builds the artifact matrix
- **WHEN** `make dist` runs
- **THEN** it MUST produce checksummed archives named `facts-<version>-<os>-<arch>` for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, freebsd/amd64, openbsd/amd64, and netbsd/amd64, with the version embedded in the binary and reported by `facts --version`

#### Scenario: install target
- **WHEN** `make install` runs with an optional `PREFIX`
- **THEN** it MUST install the `facts` binary into the standard binary location under that prefix, with no `facter` alias

#### Scenario: Release workflow publishes artifacts
- **WHEN** a release is cut
- **THEN** a CI workflow MUST build the `dist` matrix and attach the artifacts and checksums to the release

### Requirement: Go-era acceptance verification
The Go port SHALL have end-to-end acceptance verification that exercises the real binary on each supported platform.

#### Scenario: Binary-level acceptance suite
- **WHEN** the acceptance suite runs on a supported platform
- **THEN** it MUST build the real `cmd/facts` binary, execute it with representative flag combinations (default, single query, dotted query, `--json`, `--yaml`, `--strict`), and assert the release-gate fact set and exit codes against the live host using structured fact names only

#### Scenario: Beaker suite marked historical
- **WHEN** the Go acceptance suite is in place for the supported Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, and NetBSD platforms
- **THEN** the Ruby `acceptance/` Beaker suite MUST be documented as historical for the Go port and excluded from Go release gates
