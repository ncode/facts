# Delta: facts-schema

## ADDED Requirements

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
