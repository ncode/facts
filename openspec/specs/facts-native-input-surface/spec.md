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
