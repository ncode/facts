## ADDED Requirements

### Requirement: Schema semantics are shared

Facts SHALL use one internal schema contract for loading, validating, matching, and reporting schema entries used by conformance tests and supported-facts documentation.

#### Scenario: Conformance and docs use the same schema entries

- **WHEN** schema conformance and supported-facts generation read `docs/schema/facts.yaml`
- **THEN** both MUST use the same parsed entry model, platform vocabulary, conditional handling, and path matching semantics

#### Scenario: Unknown platforms fail consistently

- **WHEN** a schema entry lists an unknown platform
- **THEN** schema validation MUST fail for both conformance and documentation generation

### Requirement: Dynamic keyed schema paths are not open subtrees

Facts SHALL require emitted fact leaves under dynamic keyed maps to match documented child paths unless the schema explicitly marks the entry as an open subtree.

#### Scenario: Undocumented dynamic child fails

- **WHEN** discovery emits a leaf such as `disks.sda.serial`
- **AND** the schema only documents `disks.*` and `disks.*.serial_number`
- **THEN** schema conformance MUST report `disks.sda.serial` as undocumented

#### Scenario: Documented dynamic child passes

- **WHEN** discovery emits a leaf matching a documented dynamic child path such as `disks.sda.serial_number`
- **THEN** schema conformance MUST treat it as documented by `disks.*.serial_number`

#### Scenario: Explicit open subtree remains open

- **WHEN** discovery emits provider-shaped metadata under a schema entry explicitly marked as an open subtree
- **THEN** schema conformance MAY accept arbitrary descendants under that subtree
