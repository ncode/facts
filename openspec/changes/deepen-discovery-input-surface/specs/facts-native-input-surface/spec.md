## ADDED Requirements

### Requirement: Input source planning is shared
Facts SHALL plan facts-native and facter-compatible input sources through one discovery-time module for CLI-equivalent and library system-following discovery. The plan MUST preserve native-name precedence, configured external dirs, default external dirs, environment fact precedence, `no-external-facts`, and blocklist semantics.

#### Scenario: Library system defaults use the shared input plan
- **WHEN** an Engine constructed with `WithSystemDefaults` discovers facts on a host with native and facter-compatible config/input sources
- **THEN** discovery applies the same native-before-compatible input precedence as the `facts` CLI

#### Scenario: Explicit library dirs remain explicit
- **WHEN** an Engine constructed with `WithExternalDirs` discovers facts
- **THEN** the shared input plan loads exactly the supplied external dirs and does not add default external dirs or environment facts

#### Scenario: No external facts suppresses all external inputs
- **WHEN** discovery policy has `no-external-facts` enabled from CLI or config
- **THEN** the input plan omits external fact directories and external environment facts
- **AND** core and registered facts can still resolve
