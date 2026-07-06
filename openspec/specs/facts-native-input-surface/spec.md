# facts-native-input-surface Specification

## Purpose
TBD - created by archiving change rename-binary-to-facts. Update Purpose after archive.
## Requirements
### Requirement: Facts-native input names with facter compatibility
Facts SHALL accept its operator input surface under facts-native names — `facts.conf`, `FACTS_<name>` environment facts, and facts-native default external-fact directories — while continuing to read the facter-named equivalents for compatibility. Native names SHALL take precedence when both are present.

#### Scenario: Native config discovered first
- **WHEN** no `--config` is given and both `/etc/facts/facts.conf` and the facter-compatible default config exist
- **THEN** Facts MUST load `/etc/facts/facts.conf` and ignore the facter-named file; when only the facter-named file exists it MUST be loaded with unchanged semantics

#### Scenario: Native environment facts win collisions
- **WHEN** both `FACTS_site_role` and `FACTER_site_role` are set
- **THEN** discovery MUST resolve `site_role` from `FACTS_site_role`; a name set through only one prefix resolves from that prefix

#### Scenario: Native fact directories searched ahead of compat paths
- **WHEN** default external-fact directories are searched
- **THEN** the facts-native locations (`/etc/facts/facts.d`, `~/.facts/facts.d`, Windows `ProgramData/facts/facts.d`) MUST be searched ahead of the facter-compatible locations, and facts in both follow normal directory precedence

#### Scenario: Compat surface is documented
- **WHEN** an operator reads the configuration compatibility document
- **THEN** it MUST state the native names, the facter-named compat reads, and the precedence between them

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
