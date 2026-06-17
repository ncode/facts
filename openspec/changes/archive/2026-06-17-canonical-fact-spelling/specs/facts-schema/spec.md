## ADDED Requirements

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
