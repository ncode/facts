## ADDED Requirements

### Requirement: Host probes remain Session-injectable

Facts SHALL keep host I/O used for platform fact discovery reachable through the run-scoped Session seam so category behavior can be tested with injected native source data.

#### Scenario: Disk probes are injectable

- **WHEN** disk, partition, or mountpoint facts need command output, file reads, stat data, directory reads, glob matches, or platform identity
- **THEN** tests MUST be able to provide those inputs without reading the developer host directly

#### Scenario: Session command behavior is preserved

- **WHEN** a fact resolver executes a platform command through the Session host seam
- **THEN** command timeout, context cancellation, logging, and sanitized environment behavior MUST remain consistent with current Session command execution

### Requirement: Platform capability policy is explicit

Facts SHALL keep coarse platform capability policy explicit while preserving category-oriented resolver modules.

#### Scenario: Not-applicable fact groups are omitted by policy

- **WHEN** a target profile marks a fact group as inapplicable for the current platform
- **THEN** the relevant category module MUST omit that fact group rather than emitting empty placeholder values

#### Scenario: Category modules own resolver implementation

- **WHEN** platform capability policy is added or changed
- **THEN** parser and resolver bodies MUST remain in the relevant category modules rather than moving into a platform registry
