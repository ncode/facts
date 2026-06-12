# Delta: go-port-ruby-removal

## REMOVED Requirements

### Requirement: Historical records survive the removal
**Reason**: The repository history was flattened to a single initial commit by deliberate owner decision (2026-06-12); the commits the recoverability guarantee pointed at no longer exist in this repository.
**Migration**: `docs/HISTORY.md` remains the surviving summary of the porting record (method, final ledger accounting, milestones) and states the flattening; the Ruby implementation's own history remains available in upstream puppetlabs/facter.
