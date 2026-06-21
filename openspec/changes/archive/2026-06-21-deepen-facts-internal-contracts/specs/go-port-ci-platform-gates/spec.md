## ADDED Requirements

### Requirement: Platform target vocabulary is shared

Facts SHALL use one internal platform target vocabulary for schema-visible platform names, supported-facts generation, build target metadata, distribution target metadata, and native gate metadata.

#### Scenario: Target sets remain distinct

- **WHEN** maintainers inspect platform target metadata
- **THEN** compile targets, distribution targets, schema-visible platforms, and native validation gates MUST be represented as distinct target sets

#### Scenario: Unsupported platform names remain excluded

- **WHEN** platform target metadata is validated
- **THEN** unsupported names such as `solaris` and `aix` MUST remain excluded unless a later OpenSpec change promotes them

#### Scenario: Schema and docs use the same platform vocabulary

- **WHEN** supported-facts documentation is generated from the schema
- **THEN** the platform names accepted by schema validation MUST match the platform names used by supported-facts generation

### Requirement: Native gates align with target policy

Facts SHALL keep lab-backed and CI-backed native gates aligned with platform target policy without storing lab-specific secrets or host details in tracked files.

#### Scenario: Gate fact sets follow target policy

- **WHEN** a native gate validates a target with intentionally absent fact groups
- **THEN** the gate MUST validate the target's supported fact set and MUST NOT require facts marked inapplicable by target policy

#### Scenario: Local and CI gates use supported target names

- **WHEN** local or CI gate scripts select a platform target
- **THEN** they MUST use a target name present in the shared platform target vocabulary
