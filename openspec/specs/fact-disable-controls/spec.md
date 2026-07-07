# fact-disable-controls Specification

## Purpose
Define the all-on fact model with subtractive disabling: the disabled-set inputs (`--disable`, `FACTS_DISABLE`, config `disable`/`blocklist`), resolution-gating semantics, the `--no-block` override, and the cache interaction.
## Requirements
### Requirement: Facts are on by default and removed only by disabling

Facts SHALL resolve every applicable fact by default and SHALL remove a fact only when it is named in the disabled set, with no opt-in, allowlist, or default-off tier.

#### Scenario: Default discovery resolves everything

- **WHEN** discovery runs with an empty disabled set
- **THEN** every applicable fact MUST resolve, including voluminous facts such as `packages`

#### Scenario: A disabled fact is absent

- **WHEN** a fact name or group name is in the disabled set
- **THEN** that fact, or the group's member facts, MUST be absent from the result

### Requirement: The disabled set unions the CLI, environment, and config inputs

Facts SHALL build the disabled set as the union of `--disable`, `FACTS_DISABLE`, and the `facts.conf` `disable` key (with the Facter `blocklist` key as its compatibility alias), accepting fact names and group names.

#### Scenario: Three sources union

- **WHEN** `--disable a`, `FACTS_DISABLE=b`, and config `disable: [c]` are all present
- **THEN** the disabled set MUST contain `a`, `b`, and `c`
- **AND** a group name MUST expand to its member facts

#### Scenario: Native disable wins over the compat alias

- **WHEN** both the `disable` key and the Facter `blocklist` key are present in config
- **THEN** the `disable` key MUST take precedence
- **AND** the `blocklist` key MUST still be honored when it is the only one present

### Requirement: Disabling skips resolution for a dedicated resolver

Facts SHALL skip a fact's resolution when every fact its resolver produces is disabled, and otherwise fall back to resolve-then-prune.

#### Scenario: A standalone-resolver fact is resolution-gated

- **WHEN** a fact produced by its own resolver (such as `packages`) is disabled
- **THEN** its resolver MUST NOT run and its collection work MUST be skipped

#### Scenario: A shared resolver still runs for kept siblings

- **WHEN** a disabled fact shares a resolver that also produces a fact not in the disabled set
- **THEN** the resolver MUST run and the disabled output MUST be pruned from the result

#### Scenario: A disabled sub-fact is pruned

- **WHEN** a disabled target names a sub-fact such as `os.release`
- **THEN** its parent resolver MUST run and the sub-fact MUST be pruned from the value

### Requirement: --no-block clears the disabled set and disable beats query

Facts SHALL treat `--no-block` as a master override that resolves everything, and SHALL let a disable override an explicit query while surfacing the cause.

#### Scenario: --no-block resolves everything

- **WHEN** `--no-block` is given alongside any disable inputs
- **THEN** the disabled set MUST be empty for that run and all facts MUST resolve

#### Scenario: A disabled fact named in a query returns nothing

- **WHEN** a fact is in the disabled set and is also named as a query
- **THEN** the result MUST be empty for that fact

#### Scenario: An ambient disable is diagnosed

- **WHEN** an explicitly queried fact is suppressed by a disable sourced from `FACTS_DISABLE` or config rather than the same command line
- **THEN** a one-line stderr diagnostic MUST name the fact and the disabling source
- **AND** stdout MUST stay empty

### Requirement: Disabled facts are never served from cache

Facts SHALL subtract the disabled set before the cache is consulted and before any cache write.

#### Scenario: Cache does not serve a disabled fact

- **WHEN** a fact is disabled and a cached value for it exists
- **THEN** discovery MUST NOT serve the cached value
- **AND** a pruned sub-fact MUST NOT be written into a cached group

### Requirement: Disable semantics derive from one gating descriptor table

The engine SHALL derive resolver gating, fact-group expansion, disabled-fact filtering and pruning, and ambient-disable diagnostics from a single gating descriptor table describing each core fact category — root fact name, Facter-compatible group name, gating class (standalone, multi-output, shared-probe, inline-eager), emitted roots, probe consumers, and emits-under root. The fact-name hierarchy SHALL be walked through one shared helper wherever ancestor or descendant relationships are evaluated. Agreement between gate names, group membership, and emitted roots MUST be structurally enforced by tests, not by convention.

#### Scenario: Descriptor agreement is enforced

- **WHEN** a category's gate name, group membership, or emitted roots disagree with the gating descriptor table
- **THEN** an engine test MUST fail rather than silently mis-gating or mis-expanding

#### Scenario: Hierarchy walks agree

- **WHEN** a disabled name is evaluated for post-resolution filtering, descendant pruning, ambient-source attribution, or CLI-disable subsumption
- **THEN** every path MUST reach the same ancestor and descendant conclusions through the shared hierarchy helper

#### Scenario: Gating behavior is preserved

- **WHEN** discovery runs with any disabled set
- **THEN** resolved fact names, resolution-gating decisions, group expansion, and diagnostic bytes MUST be identical to the pre-table behavior
- **AND** per-probe semantics hold: a disabled fact that shares a memoized probe with a kept fact skips only its own resolution while the shared probe still runs for the kept consumer
- **AND** disabling `selinux` by name remains a no-op because its facts emit under `os.selinux`
