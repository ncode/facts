## MODIFIED Requirements

### Requirement: CI build matrix is limited to in-scope platforms
The Go port's CI SHALL build only supported release targets and active candidate release targets that have repeatable validation.

#### Scenario: In-scope cross-compiles
- **WHEN** the cross-compile CI job runs after DragonFly and illumos promotion
- **THEN** it MUST build linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, freebsd/amd64, freebsd/arm, freebsd/arm64, openbsd/amd64, openbsd/arm, openbsd/arm64, netbsd/amd64, netbsd/arm, netbsd/arm64, dragonfly/amd64, and illumos/amd64 targets only
- **AND** it MUST NOT build solaris or aix targets

#### Scenario: Oracle Solaris is not built by illumos validation
- **WHEN** the illumos candidate or supported gate runs
- **THEN** the pipeline MUST build `illumos/amd64`
- **AND** it MUST NOT build or publish `solaris/amd64`
