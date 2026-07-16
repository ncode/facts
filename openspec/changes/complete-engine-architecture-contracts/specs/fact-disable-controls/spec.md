## MODIFIED Requirements

### Requirement: Disabling skips resolution for a dedicated resolver

Facts SHALL skip a gateable resolver when every top-level fact root it can produce is disabled, and otherwise SHALL resolve it and prune only the disabled output.

#### Scenario: A standalone-resolver fact is resolution-gated

- **WHEN** a fact produced by its own resolver (such as `packages`) is disabled
- **THEN** its resolver MUST NOT run and its collection work MUST be skipped

#### Scenario: Every output of a multi-output resolver is disabled

- **WHEN** every declared top-level root of a multi-output resolver is disabled
- **THEN** that resolver MUST NOT run
- **AND** disabling a sub-fact alone MUST NOT count as disabling its top-level root

#### Scenario: A shared resolver still runs for kept siblings

- **WHEN** a disabled fact shares a resolver that also produces a top-level root not in the disabled set
- **THEN** the resolver MUST run and the disabled output MUST be pruned from the result

#### Scenario: A disabled sub-fact is pruned

- **WHEN** a disabled target names a sub-fact such as `os.release`
- **THEN** its parent resolver MUST run and the sub-fact MUST be pruned from the value

### Requirement: Disable semantics derive from one gating descriptor table

The engine SHALL derive resolver gating, fact-group expansion, disabled-fact filtering and pruning, and ambient-disable diagnostics from a single gating descriptor table describing each core fact category — root fact name, Facter-compatible group name, scheduling policy, and the maximum set of top-level roots it can emit on any supported host. The fact-name hierarchy SHALL be walked through one shared helper wherever ancestor or descendant relationships are evaluated. Agreement between gate names, group membership, emitted-root metadata, and resolver output MUST be structurally enforced by tests, not by convention.

#### Scenario: Descriptor agreement is enforced

- **WHEN** a category's gate name, group membership, scheduling policy, or maximum emitted-root set disagrees with the row-pinned gating descriptor table
- **THEN** an engine test MUST fail rather than silently mis-gating or mis-expanding
- **AND** every root actually emitted on a fixture host MUST be declared, while a platform-inapplicable declared root MAY be absent from that run

#### Scenario: Hierarchy walks agree

- **WHEN** a disabled name is evaluated for post-resolution filtering, descendant pruning, ambient-source attribution, or CLI-disable subsumption
- **THEN** every path MUST reach the same ancestor and descendant conclusions through the shared hierarchy helper

#### Scenario: Gateable descriptor keeps one sibling

- **WHEN** at least one emitted root declared by a gateable descriptor remains enabled
- **THEN** its resolver MUST run
- **AND** disabled roots MUST be removed by the existing pruning behavior

#### Scenario: Gateable descriptor loses every sibling

- **WHEN** every emitted root declared by a gateable descriptor is explicitly disabled
- **THEN** its resolver MUST NOT run
- **AND** the skipped descriptor MUST NOT initiate a shared memoized probe
- **AND** another scheduled descriptor, including an always-eager descriptor, MAY still initiate and share that probe

#### Scenario: Shared cloud roots are scheduled independently

- **WHEN** a metadata provider's provider-specific roots are disabled but `cloud` remains enabled
- **THEN** that provider resolver MUST still run
- **AND** it MUST be skipped only when `cloud` and all of its provider-specific roots are disabled

#### Scenario: Always-eager compatibility behavior remains

- **WHEN** `facterversion`, `is_virtual`, `path`, `virtual`, or `selinux` is named in the disabled set
- **THEN** its inline or compatibility resolver behavior MUST remain eager
- **AND** disabling `selinux` by name remains a no-op because its facts emit under `os.selinux`
