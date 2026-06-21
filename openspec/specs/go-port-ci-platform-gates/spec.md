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
- **THEN** it MUST build linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, freebsd/amd64, freebsd/arm, freebsd/arm64, openbsd/amd64, openbsd/arm, openbsd/arm64, netbsd/amd64, netbsd/arm, netbsd/arm64, dragonfly/amd64, and illumos/amd64 targets only
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

### Requirement: Plan 9 validation is lab-backed
The Go port SHALL validate Plan 9 support with a native facts-lab gate before treating Plan 9 facts as supported.

#### Scenario: Plan 9 native release gate
- **WHEN** the Plan 9 release gate runs
- **THEN** it MUST execute the real `cmd/facts` binary on the Plan 9 guest
- **AND** it MUST verify the tracked Plan 9 release-gate fact set through structured fact names only

#### Scenario: Plan 9 release gate uses rc
- **WHEN** the Plan 9 release-gate script is added to the repository
- **THEN** it MUST be written for Plan 9 `rc`
- **AND** it MUST NOT require POSIX `sh`

#### Scenario: Plan 9 gate excludes unsupported facts
- **WHEN** the first Plan 9 release gate runs
- **THEN** it MUST NOT require OS release facts, kernel release facts, filesystems, mountpoint capacity, disk inventory, partitions, DMI, cloud facts, FIPS, exact virtualization classification, DHCP server facts, or load averages

### Requirement: Plan 9 compile coverage
The Go port SHALL add Plan 9 compile coverage only for validated Plan 9 targets.

#### Scenario: Plan 9 amd64 compile
- **WHEN** the Plan 9 first-slice native gate is passing
- **THEN** the compile/build verification MUST include `plan9/amd64`

#### Scenario: Unsupported Plan 9 tuples are not added
- **WHEN** the Go toolchain lists additional Plan 9 tuples such as `plan9/386` or `plan9/arm`
- **THEN** the CI build matrix MUST NOT include those tuples until they have an equivalent native validation path

### Requirement: Plan 9 lab details stay out of tracked files
Tracked Facts files SHALL describe configurable Plan 9 validation entry points without committing private lab details.

#### Scenario: Plan 9 local gate configuration
- **WHEN** tracked files document or invoke Plan 9 validation
- **THEN** they MUST use configurable commands, variables, or generic facts-lab documentation
- **AND** they MUST NOT commit private host addresses, SSH keys, generated passwords, or host-specific helper internals

#### Scenario: Plan 9 lab command is documented
- **WHEN** a contributor wants to run the Plan 9 gate locally
- **THEN** the repository documentation MUST explain the expected high-level command flow for copying the Plan 9 binary and running `tools/plan9-release-gate.rc` through the lab

### Requirement: Plan 9 gate and schema stay aligned
The Plan 9 native gate SHALL validate the same fact set that the schema documents as non-conditional for Plan 9.

#### Scenario: Plan 9 schema conformance in gate
- **WHEN** the Plan 9 release gate runs
- **THEN** it MUST include schema conformance or an equivalent check that fails on undocumented emitted paths and missing non-conditional Plan 9 schema entries

#### Scenario: Plan 9 gate fact set changes
- **WHEN** the Plan 9 release-gate fact set changes
- **THEN** the schema and generated Plan 9 supported-facts documentation MUST be updated in the same change

### Requirement: Vulnerability scanning is automated in CI
The Go port SHALL run Go vulnerability analysis as a blocking CI check.

#### Scenario: Vulnerability scan failure fails the workflow
- **WHEN** the Go checks workflow runs
- **THEN** it MUST run the repository-pinned `govulncheck` tool against `./...`, and any reported vulnerability or scanner failure MUST fail the workflow

### Requirement: Platform target vocabulary is shared

Facts SHALL use one internal platform target vocabulary for schema-visible platform names, supported-facts generation, build target metadata, distribution target metadata, and native gate metadata.

#### Scenario: Target sets remain distinct

- **WHEN** maintainers inspect platform target metadata
- **THEN** compile targets, distribution targets, schema-visible platforms, and native validation gates MUST be represented as distinct target sets

#### Scenario: Unsupported platform names remain excluded

- **WHEN** platform target metadata is validated
- **THEN** unsupported names such as `solaris` and `aix` MUST remain excluded unless a later OpenSpec change promotes them

#### Scenario: Schema and docs use the same platform vocabulary

- **WHEN** supported-facts documentation is generated from the schema
- **THEN** the platform names accepted by schema validation MUST match the platform names used by supported-facts generation

### Requirement: Native gates align with target policy

Facts SHALL keep lab-backed and CI-backed native gates aligned with platform target policy without storing lab-specific secrets or host details in tracked files.

#### Scenario: Gate fact sets follow target policy

- **WHEN** a native gate validates a target with intentionally absent fact groups
- **THEN** the gate MUST validate the target's supported fact set and MUST NOT require facts marked inapplicable by target policy

#### Scenario: Local and CI gates use supported target names

- **WHEN** local or CI gate scripts select a platform target
- **THEN** they MUST use a target name present in the shared platform target vocabulary

