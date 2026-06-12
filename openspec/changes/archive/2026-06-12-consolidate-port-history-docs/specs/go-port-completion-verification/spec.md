# Delta: go-port-completion-verification

## MODIFIED Requirements

### Requirement: Benchmark and performance evidence
The Go port SHALL preserve performance discipline for hot-path changes.

#### Scenario: Hot-path benchmark requirement
- **WHEN** a change touches core fact collection, query/formatting, cache, config parsing, external fact loading, cloud metadata normalization, networking parsers, memory parsers, processor parsers, or mount/disk/partition parsers
- **THEN** maintainers MUST run repeated focused benchmarks before and after the change, record representative results in the change record (PR description or CHANGELOG), and avoid accepting unjustified regressions

#### Scenario: Cold compatibility exception
- **WHEN** a change only adds cold diagnostic compatibility or an error branch that does not run in normal collection paths
- **THEN** maintainers MAY skip benchmarks if the change record notes why no benchmark is needed

## REMOVED Requirements

### Requirement: Parity ledger completion gate
**Reason**: A one-time porting-completion gate, satisfied before the Ruby removal; the ledger is frozen and no longer maintained.
**Migration**: The final ledger accounting is summarized in `docs/HISTORY.md` with the full record recoverable from git history.

### Requirement: Release completion declaration
**Reason**: The port was declared complete on 2026-06-10 with all gates green; the declaration is a historical milestone, not a living requirement.
**Migration**: The completion milestone and its evidence are recorded in `docs/HISTORY.md`.
