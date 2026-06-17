## MODIFIED Requirements

### Requirement: CI build matrix is limited to in-scope platforms
The Go port's CI SHALL build only the supported release targets.

#### Scenario: Out-of-scope cross-compiles removed
- **WHEN** the cross-compile CI job runs
- **THEN** it MUST build linux, darwin, windows, freebsd, openbsd, and netbsd targets only, and MUST NOT build dragonfly, solaris, or aix targets

## ADDED Requirements

### Requirement: OpenBSD and NetBSD validation is automated in CI
The Go port SHALL validate OpenBSD and NetBSD release targets through automated CI jobs, not only manual local targets.

#### Scenario: OpenBSD CI job
- **WHEN** the CI pipeline runs for a change to Go sources or workflows
- **THEN** an automated job MUST execute the Go test suite for platform-sensitive packages and the OpenBSD release-gate fact-set smoke on an OpenBSD environment (hosted VM action or an equivalent external CI integration whose status the pipeline asserts), and its failure MUST fail the pipeline

#### Scenario: NetBSD CI job
- **WHEN** the CI pipeline runs for a change to Go sources or workflows
- **THEN** an automated job MUST execute the Go test suite for platform-sensitive packages and the NetBSD release-gate fact-set smoke on a NetBSD environment (hosted VM action or an equivalent external CI integration whose status the pipeline asserts), and its failure MUST fail the pipeline

#### Scenario: Local and CI smoke stay aligned
- **WHEN** the OpenBSD or NetBSD release-gate fact set changes
- **THEN** the CI smoke and the local BSD smoke target MUST verify the same fact set
