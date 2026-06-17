# Delta: go-port-custom-fact-dsl-contract

## MODIFIED Requirements

### Requirement: Fact-author migration guide
The repository SHALL provide a migration guide stating that Facts reads no `.rb` fact files and mapping common Facter Ruby DSL patterns to their external-fact equivalents.

#### Scenario: Migration guide content
- **WHEN** an operator with existing Ruby custom facts evaluates Facts
- **THEN** `docs/CUSTOM_FACT_MIGRATION.md` MUST state that `.rb` fact files are not read, reference ADR-0006 for the rationale, and map at least: literal `setcode` values to structured-data external facts, command and `Facter::Core::Execution` `setcode` to executable external facts, `confine` to conditional logic inside the executable, and `weight` to single-source-of-truth external facts
