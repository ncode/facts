# Delta: go-port-supported-platform-facts

## MODIFIED Requirements

### Requirement: Core fact parity
The Go port SHALL expose Ruby-compatible structured facts for each supported platform, except for the intentionally removed Ruby runtime and Puppet package-version built-ins. Legacy alias facts are not part of the surface.

#### Scenario: Linux fact parity
- **WHEN** Linux facts are resolved for OS/release/distro, SELinux, identity, networking, DHCP, memory, swap, processors, DMI, disks, partitions, filesystems, mountpoints, uptime, load averages, virtualization, hypervisors, cloud metadata, SSH, timezone, path, FIPS, Augeas, ZFS, and Zpool
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, fallback precedence, diagnostics, and formatted output for supported Linux behavior, while omitting `ruby`, `aio_agent_version`, and Puppet package-version facts

#### Scenario: macOS fact parity
- **WHEN** macOS/Darwin facts are resolved for OS/release, macOS product/build/version, identity, networking, DHCP, memory, swap, processors, DMI, system profiler hardware/software/ethernet, filesystems, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, and Augeas
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, command parsing, fallback precedence, diagnostics, and formatted output for supported macOS behavior, while omitting `ruby`, `aio_agent_version`, and Puppet package-version facts

#### Scenario: Windows fact parity
- **WHEN** Windows facts are resolved for OS/release/product metadata, system32 path, identity, networking, DHCP, memory, processors, DMI, kernel, FIPS, virtualization, hypervisors, cloud metadata, SSH, uptime, timezone, and path
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, WMI/registry parsing, fallback precedence, and diagnostic messages for supported Windows behavior, while omitting `aio_agent_version` and Puppet package-version facts

#### Scenario: FreeBSD fact parity
- **WHEN** FreeBSD facts are resolved for OS/release, identity, networking, memory, swap, processors, DMI, disks, partitions, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, and Augeas
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, sysctl/geom/mount/df/parser behavior, fallback precedence, diagnostics, and formatted output for supported FreeBSD behavior (Ruby Facter resolves no `filesystems` fact on FreeBSD, so it is absent per the not-applicable rule), while omitting `ruby`, `aio_agent_version`, and Puppet package-version facts

### Requirement: Resolver fallback and diagnostic parity
The Go port SHALL preserve supported-platform Ruby resolver fallback order and diagnostics, and SHALL reach all host command execution and file reads through the resolution Session's host seam so that every platform resolver is exercisable with an injected fake host — no resolver reads files or runs commands outside an injectable seam.

#### Scenario: Command and file fallback behavior
- **WHEN** a supported-platform resolver reads files, executes commands, queries WMI or registry data, parses system profiler/sysctl/geom output, or receives missing, empty, malformed, permission-denied, or invalid data
- **THEN** the Go port MUST match Ruby fallback order, nil/default fact shaping, debug/warn/error text, and non-fatal continuation behavior

#### Scenario: Platform-specific command isolation
- **WHEN** facts are resolved on one supported platform
- **THEN** the Go port MUST only execute or read platform-appropriate sources for that platform and MUST keep deterministic Go tests injectable through command, filesystem, time, environment, HTTP, and platform seams

#### Scenario: Host I/O is reachable through the Session
- **WHEN** a core-fact resolver that holds a resolution Session executes a command or reads, stats, or lstats a file
- **THEN** it MUST do so through the Session's host seam (e.g. `Session.commandOutput`/`readFile`), and a test MUST be able to substitute a fake host so the resolver runs to completion without touching the real operating system

## ADDED Requirements

### Requirement: Runtime and package-version facts are Go-native
Facts SHALL NOT expose Ruby runtime or Puppet package-version built-ins. The canonical built-in fact surface is Go-native; operator-supplied external facts remain the compatibility input surface.

#### Scenario: Ruby and puppet-agent package facts are absent
- **WHEN** core facts are resolved on any supported platform, even when Ruby, Puppet, or puppet-agent files are installed
- **THEN** the canonical tree MUST NOT contain `ruby`, `aio_agent_version`, or any Puppet package-version fact emitted by core discovery
