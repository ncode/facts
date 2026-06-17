## ADDED Requirements

### Requirement: Linux disk serial uses canonical spelling

Linux disk serial numbers SHALL be emitted as `disks.*.serial_number`, matching the schema-owned canonical spelling for the same concept on other supported release targets.

#### Scenario: Linux disk serial key

- **WHEN** Linux disk discovery finds a disk serial number
- **THEN** the disk entry MUST contain `serial_number`
- **AND** the disk entry MUST NOT contain `serial`
