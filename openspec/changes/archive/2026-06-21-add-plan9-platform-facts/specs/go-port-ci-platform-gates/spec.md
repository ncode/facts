## ADDED Requirements

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
