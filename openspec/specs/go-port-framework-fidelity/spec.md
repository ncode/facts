# go-port-framework-fidelity Specification

## Purpose
TBD - created by archiving change close-go-port-release-readiness-gaps. Update Purpose after archive.
## Requirements
### Requirement: HOCON configuration parsing fidelity
The Go port SHALL parse `facter.conf` with semantics equivalent to Ruby facter's HOCON handling for the supported configuration surface.

#### Scenario: Parser library or pinned subset
- **WHEN** `facter.conf` parsing is implemented
- **THEN** it MUST either use a HOCON parser library validated against a fixture corpus shared with the Ruby behavior, or document the exact supported configuration subset with tests that pin both accepted and rejected syntax at the boundary

#### Scenario: Existing configuration keys keep working
- **WHEN** the parser implementation changes
- **THEN** all configuration behavior already covered by Go tests (global, cli, fact-groups, blocklist, ttls, repeated entries, quoted values, invalid config diagnostics, CLI/config precedence) MUST continue to pass unchanged

