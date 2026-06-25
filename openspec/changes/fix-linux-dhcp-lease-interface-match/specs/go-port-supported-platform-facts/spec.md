## MODIFIED Requirements

### Requirement: Core fact parity

The Go port SHALL expose Ruby-compatible structured facts for each supported platform where Ruby Facter has comparable behavior, including correct Linux DHCP lease attribution for interface-level networking facts.

#### Scenario: Linux DHCP lease interface declarations match exactly
- **WHEN** Linux DHCP lease files are scanned for `networking.interfaces.<name>.dhcp`
- **AND** a lease file contains one or more explicit dhclient `interface "..."` declarations
- **THEN** Facts MUST use that lease only when one non-comment declaration exactly equals the requested interface name
- **AND** when a file contains multiple lease blocks, Facts MUST extract the DHCP server from the matching interface block rather than from a later block for another interface
- **AND** Facts MUST parse lease block boundaries without treating braces inside comments or quoted strings as lease terminators
- **AND** if a malformed lease block has no terminator or contains an unterminated quoted string, Facts MUST continue scanning later valid lease blocks rather than falling back to a whole-file DHCP server from another interface
- **AND** malformed interface quoted values MUST NOT count as explicit interface declarations or suppress lease filename fallback
- **AND** Facts MUST recognize explicit interface declarations even when dhclient writes multiple statements on one line
- **AND** when an exact interface declaration appears outside the lease block, Facts MUST still use the file-level DHCP server identifier for that interface
- **AND** when a file-level interface declaration matches and lease blocks omit per-block interface declarations, Facts MUST use the latest DHCP server identifier from those historical leases
- **AND** when multiple lease blocks exactly match the requested interface, the latest matching block MUST control the DHCP server value even if it omits `dhcp-server-identifier`
- **AND** commented or quoted `dhcp-server-identifier` text MUST NOT count as a DHCP server option
- **AND** explicit lease blocks for other interfaces MUST NOT override a file-level declaration for the requested interface
- **AND** Facts MUST NOT treat interface names that merely contain the requested name, such as `eth0-backup` for `eth0`, as a match
- **AND** lease filename fallback MAY still apply when a lease file has no explicit interface declaration

#### Scenario: YAML sequence map values preserve all keys
- **WHEN** YAML output renders a sequence item whose value is a map with multiple keys
- **THEN** Facts MUST render that map as valid YAML that preserves every key/value pair in the sequence item
- **AND** the sequence item MUST NOT collapse the map into a scalar value for the first key

#### Scenario: Plan 9 uptime duration fields use shared numeric types
- **WHEN** Plan 9 uptime facts are emitted
- **THEN** `system_uptime.days`, `system_uptime.hours`, and `system_uptime.seconds` MUST use 64-bit integer values
- **AND** those fields MUST match the numeric value types emitted by other supported platforms

#### Scenario: Snapshot accessors clone public mutable values
- **WHEN** a Snapshot contains a mutable public fact value, including maps, slices, pointers, arrays, and exported struct fields
- **THEN** Snapshot construction, value lookup, and copy-returning accessors MUST clone mutable values and pointed-to values in that public graph
- **AND** mutating the source value or a returned value MUST NOT mutate the Snapshot
- **AND** maps with pointer-bearing keys MUST NOT expose original key pointers when a copied key remains valid for the map key type
- **AND** cyclic pointer, map, and slice values MUST preserve cycles inside the copied graph without linking back to the original graph
- **AND** distinct slices that share backing storage MUST remain distinct copies when their visible lengths differ
- **AND** unexported struct fields MAY be preserved by shallow value copy and are not part of the deep-clone guarantee
