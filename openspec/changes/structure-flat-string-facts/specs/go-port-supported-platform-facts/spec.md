## ADDED Requirements

### Requirement: Kernel facts are structured

Kernel facts SHALL be emitted as a structured `kernel` map rather than as flat top-level facts.

#### Scenario: Kernel output shape

- **WHEN** core facts are resolved on any supported release target
- **THEN** `kernel.name` MUST contain the kernel name
- **AND** `kernel.release.full` MUST contain the full kernel release
- **AND** `kernel.release.major` and `kernel.release.minor` MUST contain the parsed release components when available
- **AND** `kernel.release.patch` MUST be present only when a patch component is available
- **AND** `kernel.version.full` MUST contain the kernel version
- **AND** `kernelmajversion`, `kernelrelease`, and `kernelversion` MUST be absent

### Requirement: Collection facts are arrays

Collection facts SHALL be emitted as arrays rather than delimiter-separated strings.

#### Scenario: Filesystems output shape

- **WHEN** filesystem types are resolved on Linux or macOS/Darwin
- **THEN** `filesystems` MUST be an array of filesystem type strings
- **AND** it MUST NOT be a comma-separated string

#### Scenario: PATH output shape

- **WHEN** core facts are resolved on any supported release target
- **THEN** `path` MUST be an array of PATH entries in lookup order
- **AND** platform path-list separators MUST NOT appear inside entries unless they are part of the entry text itself
- **AND** empty path entries MUST be omitted

#### Scenario: ZFS output shape

- **WHEN** usable ZFS command output is available on a supported platform
- **THEN** `zfs.feature_numbers` MUST be an array of supported filesystem version strings
- **AND** `zfs.version` MUST be the latest supported filesystem version string
- **AND** `zfs_featurenumbers` and `zfs_version` MUST be absent

#### Scenario: Zpool output shape

- **WHEN** usable Zpool command output is available on a supported platform
- **THEN** `zpool.feature_numbers` MUST be an array of supported pool version strings
- **AND** `zpool.feature_flags` MUST be an array of supported pool feature flag strings when feature flags are available
- **AND** `zpool.version` MUST be the latest supported pool version string, or `5000` when feature flags are present
- **AND** `zpool_featurenumbers`, `zpool_featureflags`, and `zpool_version` MUST be absent
