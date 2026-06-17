## ADDED Requirements

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
