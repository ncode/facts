## ADDED Requirements

### Requirement: BSD fact extension schema coverage
Facts SHALL document every newly emitted FreeBSD, OpenBSD, and NetBSD fact path in `docs/schema/facts.yaml`, including accurate platform lists and conditional markers.

#### Scenario: Schema lists newly emitted BSD facts
- **WHEN** this change adds a BSD-emitted fact path or extends an existing fact path to another BSD platform
- **THEN** the schema entry for that dotted path MUST include the newly supported platform in `platforms`
- **AND** the entry MUST be marked `conditional: true` when host state or probe availability controls whether it appears

#### Scenario: Generated supported-fact docs include extensions
- **WHEN** `docs/schema/facts.yaml` changes for a BSD fact extension
- **THEN** the generated pages under `docs/supported-facts/` MUST be regenerated from the schema
- **AND** the generated fact counts MUST reflect the new platform coverage

#### Scenario: Schema conformance validates BSD extensions
- **WHEN** FreeBSD, OpenBSD, or NetBSD schema conformance runs after this change
- **THEN** undocumented emitted BSD paths MUST fail the gate
- **AND** missing non-conditional BSD schema entries MUST fail the gate
