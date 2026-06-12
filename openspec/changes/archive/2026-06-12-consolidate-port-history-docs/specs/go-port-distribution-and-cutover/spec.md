# Delta: go-port-distribution-and-cutover

## REMOVED Requirements

### Requirement: Ruby entry-point cutover
**Reason**: A completed migration milestone: the `bin/facter` shim was removed entirely by ADR-0008 (no facter alias ships), and the Ruby packaging files it dispositioned were deleted with the Ruby implementation.
**Migration**: The cutover history and the packaging dispositions are recorded in `docs/HISTORY.md`.
