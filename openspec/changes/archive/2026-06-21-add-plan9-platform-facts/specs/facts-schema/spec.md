## ADDED Requirements

### Requirement: Plan 9 schema coverage
Facts SHALL include Plan 9 in `docs/schema/facts.yaml` platform metadata only for facts that Plan 9 discovery can emit through deterministic tests and native lab validation.

#### Scenario: Schema lists Plan 9-supported facts
- **WHEN** a fact can be emitted by Plan 9 discovery and is validated by the Plan 9 native gate
- **THEN** the schema entry for that dotted path MUST include `plan9` in `platforms`

#### Scenario: Schema avoids aspirational Plan 9 facts
- **WHEN** a fact has not been proven on Plan 9 through deterministic tests and a native validation path
- **THEN** the schema entry for that dotted path MUST NOT include `plan9`

#### Scenario: Conditional Plan 9 facts stay conditional
- **WHEN** host state, interface configuration, probe availability, or lab environment controls whether a Plan 9 fact appears
- **THEN** the schema entry MUST be marked `conditional: true`

### Requirement: Plan 9 platform vocabulary
Facts SHALL treat `plan9` as a supported platform vocabulary value after the first Plan 9 fact set is implemented.

#### Scenario: Schema validation accepts Plan 9
- **WHEN** schema validation reads `docs/schema/facts.yaml`
- **THEN** `plan9` MUST be accepted as a valid platform name alongside the existing supported platform names

#### Scenario: Unsupported platform names remain rejected
- **WHEN** schema validation reads an unknown platform name
- **THEN** the validation MUST fail rather than silently accepting the unknown name

### Requirement: Plan 9 supported-facts documentation
Facts SHALL publish generated supported-facts documentation for Plan 9.

#### Scenario: Generated Plan 9 supported-facts page
- **WHEN** `docs/schema/facts.yaml` lists any fact with `plan9` in `platforms`
- **THEN** the generated supported-facts documentation MUST include `docs/supported-facts/plan9.md`

#### Scenario: Generated docs stay in sync
- **WHEN** Plan 9 schema support changes
- **THEN** `go test ./...` MUST fail if `docs/supported-facts/plan9.md` is missing or drifted from the schema

### Requirement: Plan 9 schema conformance
Facts SHALL run schema conformance against native Plan 9 discovery in the Plan 9 release gate.

#### Scenario: Plan 9 undocumented emitted paths fail
- **WHEN** the Plan 9 schema conformance check flattens live Plan 9 discovery
- **THEN** every emitted path MUST match a schema entry whose `platforms` includes `plan9`

#### Scenario: Plan 9 overclaimed paths fail
- **WHEN** the Plan 9 schema conformance check runs
- **THEN** every non-conditional schema entry listing `plan9` MUST be present in live Plan 9 discovery
