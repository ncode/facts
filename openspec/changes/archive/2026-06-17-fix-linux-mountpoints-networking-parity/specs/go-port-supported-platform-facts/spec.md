## ADDED Requirements

### Requirement: Linux mountpoint size and capacity parity

Linux `mountpoints.<path>` size, used, available, and capacity SHALL match `df`/Ruby Facter, including on filesystems where the statfs fundamental block size (`f_frsize`) differs from the I/O block size (`f_bsize`). Block-count totals MUST be multiplied by `f_frsize` (falling back to `f_bsize` only when `f_frsize` is zero), and `capacity` MUST be computed as `used / (used + available)` using `f_bavail` for available space, not `used / size` using `f_bfree`. macOS/Darwin and FreeBSD mountpoint values, where `f_frsize` is unavailable and `f_bsize` is the fundamental block size, MUST remain unchanged.

#### Scenario: Frsize differs from Bsize

- **WHEN** Linux `mountpoints` is resolved for a filesystem whose `f_bsize` is larger than its `f_frsize` (for example a virtiofs mount reporting `f_bsize` 256× `f_frsize`)
- **THEN** `size_bytes`, `used_bytes`, and `available_bytes` MUST equal the block counts multiplied by `f_frsize`, matching the bytes `df` reports for that mount

#### Scenario: Capacity uses available, not free

- **WHEN** Linux `mountpoints` capacity is computed for a filesystem with root-reserved blocks (`f_bfree` greater than `f_bavail`)
- **THEN** `capacity` MUST equal `used / (used + available)` using `f_bavail`, matching the percentage `df` and Facter report rather than `used / size`

#### Scenario: Full read-only mount reports 100 percent

- **WHEN** Linux `mountpoints` capacity is computed for a fully used read-only mount where `f_bavail` is zero
- **THEN** `capacity` MUST be `100%`, matching Facter, not `0%`

#### Scenario: macOS and FreeBSD mountpoints unchanged

- **WHEN** `mountpoints` is resolved on macOS/Darwin or FreeBSD, whose `Statfs_t` exposes `f_bsize` as the fundamental block size and no `f_frsize`
- **THEN** the `mountpoints` size and byte values MUST be identical to the prior `f_bsize`-based behavior, while `capacity` MUST follow the same `used / (used + available)` definition Facter and `df` use

### Requirement: Linux interface-level binding fields parity

Linux `networking.<interface>` SHALL expose the interface-level address summary keys that Ruby Facter and the other supported POSIX platforms expose, flattened from the interface's first IPv4 and IPv6 bindings.

#### Scenario: IPv4 binding fields are flattened

- **WHEN** a Linux interface has at least one IPv4 binding carrying `address`, `netmask`, and `network`
- **THEN** `networking.<interface>` MUST also expose `ip`, `netmask`, and `network` taken from that first IPv4 binding, matching Facter

#### Scenario: IPv6 binding fields are flattened

- **WHEN** a Linux interface has at least one IPv6 binding carrying `address`, `netmask`, `network`, and `scope6`
- **THEN** `networking.<interface>` MUST also expose `ip6`, `netmask6`, `network6`, and `scope6` taken from that first IPv6 binding, matching Facter

#### Scenario: Address-less interface gains no summary keys

- **WHEN** a Linux interface has no usable bindings
- **THEN** `networking.<interface>` MUST NOT emit empty `ip`/`netmask`/`network`/`scope6` keys, preserving the not-applicable-facts-are-omitted rule
