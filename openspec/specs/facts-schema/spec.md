# facts-schema Specification

## Purpose
TBD - created by archiving change facts-schema. Update Purpose after archive.
## Requirements
### Requirement: Published fact schema
Facts SHALL publish `docs/schema/facts.yaml` describing every supported fact: dotted path (with `*` patterns for dynamic key segments), value type, description, supported platforms, and a `conditional` marker for facts whose presence depends on host state.

#### Scenario: Schema answers what is supported
- **WHEN** a user consults `docs/schema/facts.yaml`
- **THEN** every fact the `facts` CLI can emit on a supported platform MUST be described there with its type, description, and platform list

### Requirement: Schema is machine-verified on every platform gate
A conformance test in the standard suite SHALL verify the schema against a live discovery on each platform gate.

#### Scenario: No undocumented facts
- **WHEN** the conformance test flattens a host discovery into leaf paths
- **THEN** every emitted path MUST match a schema entry whose platforms include the host platform, and the test MUST fail naming any unmatched path

#### Scenario: No overclaimed facts
- **WHEN** the conformance test runs on a platform
- **THEN** every non-conditional schema entry listing that platform MUST be present in the discovery, and the test MUST fail naming any missing entry

#### Scenario: Authoring report
- **WHEN** a contributor adds a fact and runs the conformance test's report mode
- **THEN** the output MUST list exactly the undocumented paths so the schema entry can be written

### Requirement: Contributors must keep the schema current
`CONTRIBUTING.md` SHALL state that new facts require a schema entry, enforced by the conformance test on the platform gates.

#### Scenario: Contributor guidance
- **WHEN** a contributor reads `CONTRIBUTING.md`
- **THEN** it MUST link `docs/schema/facts.yaml` and state that the platform gates fail on undocumented facts

### Requirement: OpenBSD and NetBSD schema coverage
Facts SHALL include OpenBSD and NetBSD in `docs/schema/facts.yaml` platform metadata for every fact those supported platforms can emit.

#### Scenario: Schema lists BSD-supported facts
- **WHEN** a fact can be emitted by OpenBSD or NetBSD discovery
- **THEN** the schema entry for that dotted path MUST include `openbsd`, `netbsd`, or both in `platforms` as appropriate

#### Scenario: Schema conformance runs in BSD gates
- **WHEN** the OpenBSD or NetBSD live platform gate runs
- **THEN** schema conformance MUST fail on undocumented emitted paths and on missing non-conditional schema entries for that platform

#### Scenario: Supported platform vocabulary includes BSDs
- **WHEN** a contributor reads the schema file or generates a conformance report
- **THEN** the supported-platform vocabulary MUST include `openbsd` and `netbsd` alongside `linux`, `darwin`, `windows`, and `freebsd`

#### Scenario: Supported fact pages are generated from schema
- **WHEN** `docs/schema/facts.yaml` changes platform support or fact metadata
- **THEN** the generated pages under `docs/supported-facts/` MUST be updated from the schema and `go test ./...` MUST fail if they drift

### Requirement: Schema-owned canonical fact spelling

Facts SHALL document one canonical dot-notation path for each supported fact concept across supported release targets. The schema MUST NOT document platform aliases for the same concept.

#### Scenario: Disk serial number has one schema path

- **WHEN** a contributor reads `docs/schema/facts.yaml`
- **THEN** disk serial numbers MUST be documented as `disks.*.serial_number`
- **AND** `disks.*.serial` MUST NOT be documented as a supported fact

#### Scenario: Generated supported-facts pages use canonical spelling

- **WHEN** supported-facts pages are generated from `docs/schema/facts.yaml`
- **THEN** Linux and FreeBSD disk serial numbers MUST appear as `disks.*.serial_number`
- **AND** no generated page MUST list `disks.*.serial`

### Requirement: BSD fact extension schema coverage
Facts SHALL document every newly emitted FreeBSD, OpenBSD, and NetBSD fact path in `docs/schema/facts.yaml`, including accurate platform lists and conditional markers.

#### Scenario: Schema lists newly emitted BSD facts
- **WHEN** this change adds a BSD-emitted fact path or extends an existing fact path to another BSD platform
- **THEN** the schema entry for that dotted path MUST include the newly supported platform in `platforms`
- **AND** the entry MUST be marked `conditional: true` when host state or probe availability controls whether it appears

#### Scenario: Generated supported-fact docs include extensions
- **WHEN** `docs/schema/facts.yaml` changes for a BSD fact extension
- **THEN** the generated pages under `docs/supported-facts/` MUST be regenerated from the schema
- **AND** the generated fact counts MUST reflect the new platform coverage

#### Scenario: Schema conformance validates BSD extensions
- **WHEN** FreeBSD, OpenBSD, or NetBSD schema conformance runs after this change
- **THEN** undocumented emitted BSD paths MUST fail the gate
- **AND** missing non-conditional BSD schema entries MUST fail the gate

### Requirement: DragonFly and illumos schema coverage
Facts SHALL include DragonFly and illumos in `docs/schema/facts.yaml` platform metadata only for facts those targets can emit through fixture-backed and native-validated discovery.

#### Scenario: Schema lists DragonFly and illumos facts
- **WHEN** a fact can be emitted by DragonFly or illumos discovery
- **THEN** the schema entry for that dotted path MUST include `dragonfly`, `illumos`, or both in `platforms` as appropriate

#### Scenario: Schema avoids aspirational target claims
- **WHEN** a fact has not been proven on DragonFly or illumos through deterministic tests and a native validation path
- **THEN** the schema entry for that dotted path MUST NOT include the unproven platform

#### Scenario: Conditional facts stay conditional
- **WHEN** host state, installed tools, VM metadata, disk layout, cloud metadata, zones, ZFS/Zpool availability, or privilege controls whether a DragonFly or illumos fact appears
- **THEN** the schema entry MUST be marked `conditional: true`

#### Scenario: Supported platform vocabulary includes promoted targets
- **WHEN** DragonFly and illumos complete promotion to supported release targets
- **THEN** the supported-platform vocabulary MUST include `dragonfly` and `illumos` alongside `linux`, `darwin`, `windows`, `freebsd`, `openbsd`, and `netbsd`

#### Scenario: Generated supported-fact docs include promoted targets
- **WHEN** `docs/schema/facts.yaml` changes platform support for DragonFly or illumos
- **THEN** the generated pages under `docs/supported-facts/` MUST be updated from the schema
- **AND** generated fact counts MUST reflect the new platform coverage

#### Scenario: Schema conformance validates promoted targets
- **WHEN** DragonFly or illumos schema conformance runs after promotion
- **THEN** undocumented emitted paths MUST fail the gate
- **AND** missing non-conditional schema entries MUST fail the gate

### Requirement: Plan 9 schema coverage
Facts SHALL include Plan 9 in `docs/schema/facts.yaml` platform metadata only for facts that Plan 9 discovery can emit through deterministic tests and native lab validation.

#### Scenario: Schema lists Plan 9-supported facts
- **WHEN** a fact can be emitted by Plan 9 discovery and is validated by the Plan 9 native gate
- **THEN** the schema entry for that dotted path MUST include `plan9` in `platforms`

#### Scenario: Schema avoids aspirational Plan 9 facts
- **WHEN** a fact has not been proven on Plan 9 through deterministic tests and a native validation path
- **THEN** the schema entry for that dotted path MUST NOT include `plan9`

#### Scenario: Conditional Plan 9 facts stay conditional
- **WHEN** host state, interface configuration, probe availability, or lab environment controls whether a Plan 9 fact appears
- **THEN** the schema entry MUST be marked `conditional: true`

### Requirement: Plan 9 platform vocabulary
Facts SHALL treat `plan9` as a supported platform vocabulary value after the first Plan 9 fact set is implemented.

#### Scenario: Schema validation accepts Plan 9
- **WHEN** schema validation reads `docs/schema/facts.yaml`
- **THEN** `plan9` MUST be accepted as a valid platform name alongside the existing supported platform names

#### Scenario: Unsupported platform names remain rejected
- **WHEN** schema validation reads an unknown platform name
- **THEN** the validation MUST fail rather than silently accepting the unknown name

### Requirement: Plan 9 supported-facts documentation
Facts SHALL publish generated supported-facts documentation for Plan 9.

#### Scenario: Generated Plan 9 supported-facts page
- **WHEN** `docs/schema/facts.yaml` lists any fact with `plan9` in `platforms`
- **THEN** the generated supported-facts documentation MUST include `docs/supported-facts/plan9.md`

#### Scenario: Generated docs stay in sync
- **WHEN** Plan 9 schema support changes
- **THEN** `go test ./...` MUST fail if `docs/supported-facts/plan9.md` is missing or drifted from the schema

### Requirement: Plan 9 schema conformance
Facts SHALL run schema conformance against native Plan 9 discovery in the Plan 9 release gate.

#### Scenario: Plan 9 undocumented emitted paths fail
- **WHEN** the Plan 9 schema conformance check flattens live Plan 9 discovery
- **THEN** every emitted path MUST match a schema entry whose `platforms` includes `plan9`

#### Scenario: Plan 9 overclaimed paths fail
- **WHEN** the Plan 9 schema conformance check runs
- **THEN** every non-conditional schema entry listing `plan9` MUST be present in live Plan 9 discovery

### Requirement: Flat grouped facts are represented as structured schema paths

Facts SHALL document grouped fact concepts as nested schema paths, not as multiple top-level flat names with embedded group meaning.

#### Scenario: Kernel schema is structured

- **WHEN** a contributor reads `docs/schema/facts.yaml`
- **THEN** kernel data MUST be documented under `kernel.*`
- **AND** the schema MUST include `kernel.name`, `kernel.release.full`, `kernel.release.major`, `kernel.release.minor`, and `kernel.version.full`
- **AND** `kernel.release.patch` MUST be documented as conditional
- **AND** `kernel`, `kernelmajversion`, `kernelrelease`, and `kernelversion` MUST NOT be documented as supported facts

### Requirement: String collections are represented as arrays

Facts SHALL document collection-valued facts as arrays, not as delimiter-separated strings.

#### Scenario: Filesystems and PATH schema are arrays

- **WHEN** a contributor reads `docs/schema/facts.yaml`
- **THEN** `filesystems` MUST have type `array`
- **AND** `path` MUST have type `array`

#### Scenario: ZFS schema is structured

- **WHEN** ZFS facts are documented
- **THEN** the schema MUST include `zfs.feature_numbers` as an array
- **AND** the schema MUST include `zfs.version` as a string
- **AND** `zfs_featurenumbers` and `zfs_version` MUST NOT be documented as supported facts

#### Scenario: Zpool schema is structured

- **WHEN** Zpool facts are documented
- **THEN** the schema MUST include `zpool.feature_numbers` as an array
- **AND** the schema MUST include `zpool.feature_flags` as an array
- **AND** the schema MUST include `zpool.version` as a string
- **AND** `zpool_featurenumbers`, `zpool_featureflags`, and `zpool_version` MUST NOT be documented as supported facts

#### Scenario: Supported fact pages reflect structured schema

- **WHEN** `docs/supported-facts/` pages are generated
- **THEN** the pages MUST show the structured kernel, ZFS, and Zpool paths
- **AND** the pages MUST show `filesystems` and `path` as arrays
- **AND** the pages MUST NOT list the removed flat facts

