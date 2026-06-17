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

