## MODIFIED Requirements

### Requirement: Core fact parity

The Go port SHALL expose Ruby-compatible structured facts for each supported platform where Ruby Facter has comparable behavior, except for the intentionally removed Ruby runtime and Puppet package-version built-ins. Legacy alias facts are not part of the surface. Facts MAY expose Facts-native extensions when the native source is stable, the canonical fact spelling is schema-documented, and platform validation covers the behavior. Linux DHCP lease attribution for interface-level networking facts MUST use the exact interface and lease-block semantics specified below.

#### Scenario: Linux fact parity
- **WHEN** Linux facts are resolved for OS/release/distro, SELinux, identity, networking, DHCP, memory, swap, processors, DMI, disks, partitions, filesystems, mountpoints, uptime, load averages, virtualization, hypervisors, cloud metadata, SSH, timezone, path, FIPS, and Augeas
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, fallback precedence, diagnostics, and formatted output for supported Linux behavior, while omitting `ruby`, `aio_agent_version`, and Puppet package-version facts

#### Scenario: macOS fact parity
- **WHEN** macOS/Darwin facts are resolved for OS/release, macOS product/build/version, identity, networking, DHCP, memory, swap, processors, DMI, system profiler hardware/software/ethernet, filesystems, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, and Augeas
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, command parsing, fallback precedence, diagnostics, and formatted output for supported macOS behavior, while omitting `ruby`, `aio_agent_version`, and Puppet package-version facts

#### Scenario: Windows fact parity
- **WHEN** Windows facts are resolved for OS/release/product metadata, system32 path, identity, networking, DHCP, memory, processors, DMI, kernel, FIPS, virtualization, hypervisors, cloud metadata, SSH, uptime, timezone, and path
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, WMI/registry parsing, fallback precedence, and diagnostic messages for supported Windows behavior, while omitting `aio_agent_version` and Puppet package-version facts

#### Scenario: FreeBSD fact parity
- **WHEN** FreeBSD facts are resolved for OS/release, identity, networking, memory, swap, processors, DMI, disks, partitions, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, Augeas, ZFS, and Zpool
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, sysctl/geom/mount/df/parser behavior, fallback precedence, diagnostics, and formatted output for supported FreeBSD behavior (Ruby Facter resolves no `filesystems` fact on FreeBSD, so it is absent per the not-applicable rule), while omitting `ruby`, `aio_agent_version`, and Puppet package-version facts

#### Scenario: OpenBSD fact parity
- **WHEN** OpenBSD facts are resolved for OS/release, identity, networking, memory, swap, processors, DMI when available, disks, partitions, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, and Augeas
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, sysctl/disklabel/mount/df/route/dhcpleasectl parser behavior, fallback precedence, diagnostics, and formatted output for supported OpenBSD behavior, while omitting `ruby`, `aio_agent_version`, Puppet package-version facts, legacy aliases, ZFS/zpool facts, and other platform-inapplicable facts

#### Scenario: NetBSD fact parity
- **WHEN** NetBSD facts are resolved for OS/release, identity, networking, memory, swap, processors, DMI when available, disks, partitions, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, Augeas, and conditional ZFS/zpool command output
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, sysctl/disklabel/dkctl/mount/df/route/parser behavior, fallback precedence, diagnostics, and formatted output for supported NetBSD behavior, while omitting `ruby`, `aio_agent_version`, Puppet package-version facts, legacy aliases, and platform-inapplicable or unusable ZFS/zpool facts

#### Scenario: DragonFly fact coverage
- **WHEN** DragonFly BSD facts are resolved for OS/release, identity, networking, memory, swap, processors, disks, partitions, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, and other audited native sources
- **THEN** the Go port MUST emit only schema-documented DragonFly facts backed by stable DragonFly sources and native validation, reporting `DragonFly` for `os.name`, `os.family`, and `kernel.name`
- **AND** Ruby Facter byte parity MUST NOT block accurate Facts-native extensions when DragonFly has no comparable Facter behavior

#### Scenario: illumos fact coverage
- **WHEN** illumos facts are resolved on OmniOS for OS/release, identity, networking, memory, swap, processors, disks, partitions, mountpoints, uptime, load averages, virtualization/zones where available, SSH, timezone, path, ZFS, Zpool, and other audited native sources
- **THEN** the Go port MUST emit only schema-documented illumos facts backed by stable illumos sources and native validation
- **AND** `os.family` MUST be `illumos`, `kernel.name` MUST be `SunOS`, and `os.name` MUST report the validated distribution such as `OmniOS`
- **AND** Oracle Solaris-specific behavior MUST remain absent until the Oracle Solaris target is separately validated

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
