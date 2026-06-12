# Delta: go-port-framework-fidelity

## MODIFIED Requirements

### Requirement: Bounded and documented --puppet behavior
The Go port SHALL define `--puppet` behavior explicitly instead of silently diverging from Ruby.

#### Scenario: puppetversion resolution
- **WHEN** `facter --puppet` runs on a system with the puppet binary installed
- **THEN** the `puppetversion` fact MUST resolve from the puppet installation

#### Scenario: Plugin external facts are searched
- **WHEN** `facter --puppet` runs
- **THEN** the engine MUST search Puppet's default plugin fact destination paths for external facts on each supported platform

#### Scenario: Ruby plugin custom facts warn
- **WHEN** `facter --puppet` runs and Puppet Ruby plugin custom facts would have been loaded by Ruby facter
- **THEN** the Go port MUST emit a warning that Ruby plugin custom facts are not loaded, and the deviation MUST be documented in the migration guide (`docs/CUSTOM_FACT_MIGRATION.md`) and man page
