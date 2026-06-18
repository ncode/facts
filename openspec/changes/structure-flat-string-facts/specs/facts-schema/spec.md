## ADDED Requirements

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
