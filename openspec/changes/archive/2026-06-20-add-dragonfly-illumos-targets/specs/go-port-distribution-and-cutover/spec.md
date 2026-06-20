## MODIFIED Requirements

### Requirement: Reproducible release artifacts
The Go port SHALL produce versioned, installable release artifacts for all supported targets.

#### Scenario: dist target builds the artifact matrix
- **WHEN** `make dist` runs after DragonFly and illumos promotion
- **THEN** it MUST produce checksummed archives named `facts-<version>-<os>-<arch>` for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, freebsd/amd64, openbsd/amd64, netbsd/amd64, dragonfly/amd64, and illumos/amd64, with the version embedded in the binary and reported by `facts --version`

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
