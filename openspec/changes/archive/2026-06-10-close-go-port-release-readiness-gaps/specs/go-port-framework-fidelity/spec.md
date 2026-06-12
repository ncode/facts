## ADDED Requirements

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
- **THEN** the Go port MUST emit a warning that Ruby plugin custom facts are not loaded, and the deviation MUST be documented in the DSL contract and man page

### Requirement: HOCON configuration parsing fidelity
The Go port SHALL parse `facter.conf` with semantics equivalent to Ruby facter's HOCON handling for the supported configuration surface.

#### Scenario: Parser library or pinned subset
- **WHEN** `facter.conf` parsing is implemented
- **THEN** it MUST either use a HOCON parser library validated against a fixture corpus shared with the Ruby behavior, or document the exact supported configuration subset with tests that pin both accepted and rejected syntax at the boundary

#### Scenario: Existing configuration keys keep working
- **WHEN** the parser implementation changes
- **THEN** all configuration behavior already covered by Go tests (global, cli, fact-groups, blocklist, ttls, repeated entries, quoted values, invalid config diagnostics, CLI/config precedence) MUST continue to pass unchanged
