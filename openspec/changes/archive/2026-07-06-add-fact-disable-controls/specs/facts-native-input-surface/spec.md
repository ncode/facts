## ADDED Requirements

### Requirement: disable is the native disabled-set key with blocklist compatibility

Facts SHALL accept `disable` as the facts-native `facts.conf` key for the disabled set while continuing to read the Facter `blocklist` key as its compatibility alias, with the native key taking precedence.

#### Scenario: Native disable key honored with blocklist compat

- **WHEN** `facts.conf` sets `disable` and the facter-compatible config sets `blocklist`
- **THEN** discovery MUST honor `disable`
- **AND** discovery MUST still honor `blocklist` when it is the only key present

### Requirement: FACTS_DISABLE is a reserved control key, not an environment fact

Facts SHALL treat an environment variable whose resolved fact name is `disable` as the disabled-set control input rather than an external environment fact, across every accepted prefix.

#### Scenario: FACTS_DISABLE controls disabling, not a fact

- **WHEN** `FACTS_DISABLE=packages` is set
- **THEN** `packages` MUST be added to the disabled set
- **AND** no external fact named `disable` MUST be created

#### Scenario: All prefix spellings are reserved

- **WHEN** any of `FACTS_DISABLE`, `FACTSDISABLE`, `FACTER_DISABLE`, or `FACTERDISABLE` is set
- **THEN** it MUST be treated as the disable control
- **AND** it MUST NOT create a `disable` fact
