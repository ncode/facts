# Delta: go-port-supported-platform-facts

## ADDED Requirements

### Requirement: Not-applicable facts are omitted
A fact that cannot resolve a value or does not apply to the host platform SHALL be absent from the canonical tree. Facts MUST NOT be emitted with empty-string values, empty-map values, or platform-inapplicable defaults. Additional accurate structured data beyond Ruby Facter's set MAY be exposed only as a documented deviation.

#### Scenario: Unresolvable facts are absent
- **WHEN** a fact's source cannot produce a value (no augparse binary for `augeas.version`, no enumerable devices for `disks`/`partitions`, unknown `processors.speed`)
- **THEN** the fact (or key) MUST be absent from every output mode, not rendered as an empty string or empty map

#### Scenario: Platform-inapplicable facts are absent
- **WHEN** discovery runs on a platform where Ruby Facter does not resolve a fact (`fips_enabled` outside Linux and Windows, `os.selinux` outside Linux)
- **THEN** that fact MUST be absent from the canonical tree on that platform

#### Scenario: Additional data is a documented deviation
- **WHEN** the Go port exposes accurate structured data Ruby Facter lacks on that platform (e.g. `processors.extensions` on ARM macOS)
- **THEN** the deviation MUST be documented in the man page Go-port notes
