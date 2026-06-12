# Delta: go-port-ruby-removal

## MODIFIED Requirements

### Requirement: Historical records survive the removal
The porting verification record SHALL remain recoverable and truthful after its inputs are deleted: `docs/HISTORY.md` SHALL summarize the porting approach, the final parity-ledger accounting, and the release-readiness milestones, and SHALL cite the git commit at which the full `docs/PARITY_LEDGER.md`, `docs/MIGRATION.md`, and `PORTING.md` texts were last present.

#### Scenario: Frozen records are recoverable through HISTORY.md
- **WHEN** a maintainer needs the full porting verification record after the frozen files were removed from HEAD
- **THEN** `docs/HISTORY.md` MUST name the commit containing them so `git show <commit>:docs/PARITY_LEDGER.md` (and the migration log and port tracker) reproduces the complete record

#### Scenario: History summary is honest about what was removed
- **WHEN** a reader consults `docs/HISTORY.md`
- **THEN** it MUST state the final ledger bucket counts, the porting method (TDD slices against Ruby specs), the platform gates that verified completion, and the deliberate post-port departures (ADR-0006, ADR-0007, ADR-0008) without rewriting any recorded outcome
