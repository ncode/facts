# Delta: go-port-parity-ledger-integrity

## REMOVED Requirements

### Requirement: Coverage references are machine-verified
**Reason**: The ledger generator (`tools/parity-ledger`) was deleted when the Ruby implementation was removed; the ledger is a frozen, never-regenerated record, so there is no generator left to constrain.
**Migration**: The porting verification outcome is summarized in `docs/HISTORY.md`, which cites the git commit containing the full frozen `docs/PARITY_LEDGER.md`.

### Requirement: Every spec file has an explicit scoping decision
**Reason**: The Ruby `spec/` trees this requirement inventoried were deleted with the Ruby implementation; the final accounting (614 in-scope, 281 out-of-scope, 0 unclassified) is a completed historical fact.
**Migration**: The final bucket counts are recorded in `docs/HISTORY.md`; the full ledger remains in git history at the cited commit.
