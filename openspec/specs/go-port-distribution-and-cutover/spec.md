# go-port-distribution-and-cutover Specification

## Purpose
TBD - created by archiving change close-go-port-release-readiness-gaps. Update Purpose after archive.
## Requirements
### Requirement: Reproducible release artifacts
The Go port SHALL produce versioned, installable release artifacts for all supported targets.

#### Scenario: dist target builds the artifact matrix
- **WHEN** `make dist` runs after DragonFly and illumos promotion
- **THEN** it MUST produce checksummed archives named `facts-<version>-<os>-<arch>` for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, freebsd/amd64, freebsd/arm, freebsd/arm64, openbsd/amd64, openbsd/arm, openbsd/arm64, netbsd/amd64, netbsd/arm, netbsd/arm64, dragonfly/amd64, and illumos/amd64, with the version embedded in the binary and reported by `facts --version`

#### Scenario: install target
- **WHEN** `make install` runs with an optional `PREFIX`
- **THEN** it MUST install the `facts` binary into the standard binary location under that prefix, with no `facter` alias

#### Scenario: Release workflow publishes artifacts
- **WHEN** a release is cut after DragonFly and illumos promotion
- **THEN** a CI workflow MUST build the `dist` matrix and attach the artifacts and checksums to the release

#### Scenario: Unsupported Go tuples are not invented
- **WHEN** release artifacts are built
- **THEN** Facts MUST NOT publish DragonFly or illumos architectures that the Go toolchain does not support
- **AND** Facts MUST NOT publish `solaris/amd64` until Oracle Solaris is separately validated and promoted

### Requirement: Go-era acceptance verification
The Go port SHALL have end-to-end acceptance verification that exercises the real binary on each supported platform.

#### Scenario: Binary-level acceptance suite
- **WHEN** the acceptance suite runs on a supported platform
- **THEN** it MUST build the real `cmd/facts` binary, execute it with representative flag combinations (default, single query, dotted query, `--json`, `--yaml`, `--strict`), and assert the release-gate fact set and exit codes against the live host using structured fact names only

#### Scenario: Beaker suite marked historical
- **WHEN** the Go acceptance suite is in place for the supported Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, and NetBSD platforms
- **THEN** the Ruby `acceptance/` Beaker suite MUST be documented as historical for the Go port and excluded from Go release gates

### Requirement: End-user documentation matches the Go CLI
The Go port SHALL ship user documentation that reflects the Go binary's actual behavior.

#### Scenario: Man page parity
- **WHEN** the man page is regenerated or audited against the Go CLI
- **THEN** every documented flag, default, and exit code MUST match the Go implementation, and Go-port deviations (the no-Ruby-DSL input contract) MUST be noted

### Requirement: Plan 9 release artifact promotion is validation-gated
Facts SHALL publish Plan 9 release artifacts only for Plan 9 tuples that have native validation.

#### Scenario: Plan 9 amd64 artifact eligibility
- **WHEN** `plan9/amd64` compile coverage and the Plan 9 native release gate are both passing
- **THEN** `plan9/amd64` MAY be added to the release artifact matrix

#### Scenario: Plan 9 artifact names
- **WHEN** Plan 9 release artifacts are produced
- **THEN** they MUST follow the existing artifact naming scheme `facts-<version>-plan9-<arch>`
- **AND** the embedded version MUST be reported by `facts --version`

#### Scenario: Unvalidated Plan 9 artifacts are not published
- **WHEN** the Go toolchain supports a Plan 9 architecture that lacks native validation
- **THEN** Facts MUST NOT publish an artifact for that tuple

### Requirement: Plan 9 acceptance verification
Facts SHALL run binary-level acceptance verification on Plan 9 before Plan 9 is documented as a release target.

#### Scenario: Plan 9 binary acceptance
- **WHEN** Plan 9 is promoted to a release target
- **THEN** acceptance verification MUST execute the real Plan 9 binary with representative CLI modes supported on Plan 9
- **AND** it MUST assert the Plan 9 release-gate fact set and exit codes against the live Plan 9 guest

#### Scenario: Plan 9 unsupported CLI behavior
- **WHEN** a representative CLI mode depends on an OS feature unavailable on Plan 9
- **THEN** the acceptance verification MUST document the omission and continue to validate the supported CLI modes

### Requirement: Plan 9 documentation matches promotion state
Facts SHALL distinguish lab-validated Plan 9 fact support from published release-target support until Plan 9 artifacts are actually shipped.

#### Scenario: Plan 9 supported facts before artifact promotion
- **WHEN** Plan 9 facts are implemented and native-gated but no Plan 9 artifact is published
- **THEN** documentation MUST describe Plan 9 as lab-validated fact support rather than a published release artifact target

#### Scenario: Plan 9 release target after artifact promotion
- **WHEN** Plan 9 artifacts are added to the release matrix
- **THEN** README and release documentation MUST list Plan 9 with only the validated architectures

