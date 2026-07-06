## ADDED Requirements

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
