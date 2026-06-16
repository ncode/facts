# Delta: go-port-framework-fidelity

## MODIFIED Requirements

### Requirement: Bounded and documented --puppet behavior
The Go port SHALL define `--puppet` behavior explicitly. `--puppet` remains an external-fact compatibility bridge for Puppet plugin fact destinations; it SHALL NOT inventory Puppet package versions.

#### Scenario: puppetversion is absent
- **WHEN** `facts --puppet puppetversion` runs on a system with the puppet binary installed
- **THEN** the CLI MUST NOT execute Puppet for version discovery and MUST treat `puppetversion` like any other missing fact

#### Scenario: Plugin external facts are searched
- **WHEN** `facts --puppet` runs
- **THEN** the engine MUST search Puppet's default plugin fact destination paths for external facts on each supported platform

#### Scenario: Ruby plugin custom facts warn
- **WHEN** `facts --puppet` runs and Puppet Ruby plugin custom facts would have been loaded by Ruby Facter
- **THEN** the Go port MUST emit a warning that Ruby plugin custom facts are not loaded, and the deviation MUST be documented in the migration guide (`docs/CUSTOM_FACT_MIGRATION.md`) and man page
