## ADDED Requirements

### Requirement: Validated BSD fact extensions
Facts SHALL extend FreeBSD, OpenBSD, and NetBSD structured fact coverage only when a native platform source is stable, fixture-backed, and represented in the schema. Unsupported or unstable BSD fields MUST remain absent from the canonical tree.

#### Scenario: NetBSD mountpoint byte facts
- **WHEN** NetBSD mountpoints are resolved from `mount` and `df -P` output
- **THEN** each mountpoint with matching `df` data MUST include `available`, `available_bytes`, `capacity`, `size`, `size_bytes`, `used`, and `used_bytes`
- **AND** mountpoints without matching size data MUST omit those byte and capacity fields

#### Scenario: BSD interface operational state
- **WHEN** FreeBSD, OpenBSD, or NetBSD `ifconfig` output reports an interface status
- **THEN** `networking.interfaces.<name>.operational_state` MUST be emitted with the normalized status value for that interface

#### Scenario: FreeBSD interface speed and duplex
- **WHEN** FreeBSD `ifconfig -m` output reports negotiated media speed and duplex for an interface
- **THEN** `networking.interfaces.<name>.speed` MUST be emitted in Mbit/s
- **AND** `networking.interfaces.<name>.duplex` MUST be emitted as `full` or `half`

#### Scenario: FreeBSD interface DHCP
- **WHEN** a FreeBSD DHCP lease file for an interface contains a `dhcp-server-identifier`
- **THEN** `networking.interfaces.<name>.dhcp` MUST report that server address
- **AND** `networking.dhcp` MUST report the primary interface DHCP server when known

#### Scenario: FreeBSD partition type metadata
- **WHEN** FreeBSD GEOM XML reports partition type metadata for a partition provider
- **THEN** `partitions.<name>.parttype` MUST report the platform partition type identifier

#### Scenario: Conditional FreeBSD disk type
- **WHEN** FreeBSD disk metadata reports a known rotation rate
- **THEN** `disks.<name>.type` MUST be emitted as `ssd` or `hdd`
- **AND** the field MUST be absent when the platform reports an unknown rotation rate

#### Scenario: BSD DMI product version disposition
- **WHEN** OpenBSD or NetBSD exposes a stable system product version source
- **THEN** Facts MUST either expose it as `dmi.product.version` with schema coverage or document and test why it remains absent
- **AND** Facts MUST NOT silently move an existing `dmi.bios.version` value without a compatibility test covering the changed shape

#### Scenario: Unstable NetBSD DHCP source remains absent
- **WHEN** NetBSD DHCP data is available only from an unstable or binary lease source
- **THEN** `networking.interfaces.<name>.dhcp` and `networking.dhcp` MUST remain absent unless a stable text source is selected and tested
