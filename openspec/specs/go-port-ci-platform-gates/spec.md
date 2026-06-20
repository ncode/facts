# go-port-ci-platform-gates Specification

## Purpose
TBD - created by archiving change close-go-port-release-readiness-gaps. Update Purpose after archive.
## Requirements
### Requirement: Windows validation is a blocking CI gate
The Go port SHALL treat the Windows release gate as a blocking, automated pass/fail criterion.

#### Scenario: Release gate failure fails the workflow
- **WHEN** `tools/windows-release-gate.ps1` exits non-zero on a Windows CI runner
- **THEN** the unit-test workflow job MUST fail, and the gate MUST be documented in `CONTRIBUTING.md` as a release-blocking check

#### Scenario: Gate covers the Windows release fact set
- **WHEN** the Windows release gate runs
- **THEN** it MUST build the real `cmd/facts` binary and verify the Windows release-gate fact set (including OS, system32, networking, memory, processors, DMI, uptime, virtualization, FIPS, and timezone) through the CLI using structured fact names only

### Requirement: FreeBSD validation is automated in CI
The Go port SHALL validate FreeBSD — a release target — through an automated CI job, not only a manual local target.

#### Scenario: FreeBSD CI job
- **WHEN** the CI pipeline runs for a change to Go sources or workflows
- **THEN** an automated job MUST execute the Go test suite for platform-sensitive packages and the FreeBSD release-gate fact-set smoke on a FreeBSD environment (hosted VM action or an equivalent external CI integration whose status the pipeline asserts), and its failure MUST fail the pipeline

#### Scenario: Local and CI smoke stay aligned
- **WHEN** the FreeBSD release-gate fact set changes
- **THEN** the CI smoke and the local `make lima-freebsd-smoke` (or successor) target MUST verify the same fact set

### Requirement: CI build matrix is limited to in-scope platforms
The Go port's CI SHALL build only supported release targets and active candidate release targets that have repeatable validation.

#### Scenario: In-scope cross-compiles
- **WHEN** the cross-compile CI job runs after DragonFly and illumos promotion
- **THEN** it MUST build linux, darwin, windows, freebsd, openbsd, netbsd, dragonfly, and illumos targets only
- **AND** it MUST NOT build solaris or aix targets

#### Scenario: Oracle Solaris is not built by illumos validation
- **WHEN** the illumos candidate or supported gate runs
- **THEN** the pipeline MUST build `illumos/amd64`
- **AND** it MUST NOT build or publish `solaris/amd64`

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

### Requirement: DragonFly and illumos validation is automated after promotion
The Go port SHALL validate DragonFly and illumos release targets through automated or asserted native gates after promotion, not only manual cross-compilation.

#### Scenario: DragonFly native gate
- **WHEN** the pipeline validates DragonFly after promotion
- **THEN** an automated job or asserted external lab status MUST execute platform-sensitive Go tests and the DragonFly release-gate fact-set smoke on a DragonFly environment
- **AND** its failure MUST fail the pipeline

#### Scenario: illumos native gate
- **WHEN** the pipeline validates illumos after promotion
- **THEN** an automated job or asserted external lab status MUST execute platform-sensitive Go tests and the illumos release-gate fact-set smoke on an illumos environment
- **AND** its failure MUST fail the pipeline

#### Scenario: Local and CI smokes stay aligned
- **WHEN** the DragonFly or illumos release-gate fact set changes
- **THEN** the CI smoke and local smoke target MUST verify the same fact set by running the same tracked release-gate script

#### Scenario: Lab details stay out of git
- **WHEN** local smoke targets invoke DragonFly, illumos, or amd64 BSD lab guests
- **THEN** tracked files MUST reference configurable wrapper variables only
- **AND** lab hostnames, addresses, keys, and private helper commands MUST remain outside tracked files
