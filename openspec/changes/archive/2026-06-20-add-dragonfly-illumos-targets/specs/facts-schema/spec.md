## ADDED Requirements

### Requirement: DragonFly and illumos schema coverage
Facts SHALL include DragonFly and illumos in `docs/schema/facts.yaml` platform metadata only for facts those targets can emit through fixture-backed and native-validated discovery.

#### Scenario: Schema lists DragonFly and illumos facts
- **WHEN** a fact can be emitted by DragonFly or illumos discovery
- **THEN** the schema entry for that dotted path MUST include `dragonfly`, `illumos`, or both in `platforms` as appropriate

#### Scenario: Schema avoids aspirational target claims
- **WHEN** a fact has not been proven on DragonFly or illumos through deterministic tests and a native validation path
- **THEN** the schema entry for that dotted path MUST NOT include the unproven platform

#### Scenario: Conditional facts stay conditional
- **WHEN** host state, installed tools, VM metadata, disk layout, cloud metadata, zones, ZFS/Zpool availability, or privilege controls whether a DragonFly or illumos fact appears
- **THEN** the schema entry MUST be marked `conditional: true`

#### Scenario: Supported platform vocabulary includes promoted targets
- **WHEN** DragonFly and illumos complete promotion to supported release targets
- **THEN** the supported-platform vocabulary MUST include `dragonfly` and `illumos` alongside `linux`, `darwin`, `windows`, `freebsd`, `openbsd`, and `netbsd`

#### Scenario: Generated supported-fact docs include promoted targets
- **WHEN** `docs/schema/facts.yaml` changes platform support for DragonFly or illumos
- **THEN** the generated pages under `docs/supported-facts/` MUST be updated from the schema
- **AND** generated fact counts MUST reflect the new platform coverage

#### Scenario: Schema conformance validates promoted targets
- **WHEN** DragonFly or illumos schema conformance runs after promotion
- **THEN** undocumented emitted paths MUST fail the gate
- **AND** missing non-conditional schema entries MUST fail the gate
