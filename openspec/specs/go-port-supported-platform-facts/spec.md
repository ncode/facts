# go-port-supported-platform-facts Specification

## Purpose
TBD - created by archiving change complete-supported-platform-go-port. Update Purpose after archive.
## Requirements
### Requirement: Supported platform fact scope
The Go port SHALL treat Linux, macOS/Darwin, Windows, and FreeBSD as supported release targets.

#### Scenario: In-scope Ruby spec disposition
- **WHEN** the parity audit processes Ruby specs for Linux, macOS/Darwin, Windows, FreeBSD, shared framework behavior, custom facts, external facts, and in-scope Linux distro families
- **THEN** each spec file MUST receive an explicit disposition of covered, newly covered, intentional deviation, or blocked with a concrete reason

#### Scenario: Out-of-scope platform exclusion
- **WHEN** the audit sees Ruby or Go behavior for Solaris, AIX, OpenBSD, NetBSD, DragonFly, or unvalidated generic BSD-family paths
- **THEN** that behavior MUST NOT be treated as a release blocker unless it is needed by a shared parser or helper used by Linux, macOS/Darwin, Windows, or FreeBSD

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

### Requirement: Virtualization and cloud parity
The Go port SHALL match Ruby-compatible virtualization, hypervisor, and cloud metadata behavior on supported platforms.

#### Scenario: Virtualization and hypervisor facts
- **WHEN** supported-platform virtualization facts are resolved from Linux `virt-what`, cgroups, DMI, VMware, Xen, OpenVZ, Windows OEM/netkvm/WMI indicators, macOS indicators, or FreeBSD virtualization indicators
- **THEN** the Go port MUST match Ruby `virtual`, `is_virtual`, `hypervisors.*`, Xen, container, and nil/unknown behavior for supported detection paths

#### Scenario: Cloud metadata facts
- **WHEN** EC2, GCE, or Azure metadata is available, invalid, blocked by virtualization/provider checks, empty, or unavailable
- **THEN** the Go port MUST match Ruby cloud provider gating, request headers, metadata normalization, user-data behavior, nil values, and diagnostic behavior on supported platforms

### Requirement: FreeBSD live validation
The Go port SHALL have a repeatable FreeBSD validation path before the port is declared complete.

#### Scenario: Lima FreeBSD smoke gate
- **WHEN** maintainers run the FreeBSD validation target from macOS using Lima
- **THEN** the workflow MUST build a FreeBSD binary, run it in a FreeBSD guest, and verify a release-gate fact set that includes at least `os.name`, `os.release`, `kernel`, `virtual`, `is_virtual`, `memory`, `processors`, `mountpoints`, and `dmi`

#### Scenario: FreeBSD fixture-backed parity
- **WHEN** FreeBSD fact behavior depends on sysctl, geom, mount, df, ifconfig, route, kenv, or other OS command output
- **THEN** deterministic Go tests MUST use fixtures and injectable seams so the behavior remains covered even when the live Lima smoke gate is not running

### Requirement: Not-applicable facts are omitted
A fact that cannot resolve a value or does not apply to the host platform SHALL be absent from the canonical tree. Facts MUST NOT be emitted with empty-string values, empty-map values, or platform-inapplicable defaults. Additional accurate structured data beyond Ruby Facter's set MAY be exposed only as a documented deviation.

#### Scenario: Unresolvable facts are absent
- **WHEN** a fact's source cannot produce a value (no augparse binary for `augeas.version`, no enumerable devices for `disks`/`partitions`, unknown `processors.speed`)
- **THEN** the fact (or key) MUST be absent from every output mode, not rendered as an empty string or empty map

#### Scenario: Platform-inapplicable facts are absent
- **WHEN** discovery runs on a platform where Ruby Facter does not resolve a fact (`fips_enabled` outside Linux and Windows, `os.selinux` outside Linux)
- **THEN** that fact MUST be absent from the canonical tree on that platform

#### Scenario: Additional data is a documented deviation
- **WHEN** the Go port exposes accurate structured data Ruby Facter lacks on that platform (e.g. `processors.extensions` on ARM macOS)
- **THEN** the deviation MUST be documented in the man page Go-port notes

### Requirement: Primary IPv6 selection prefers routable addresses
When selecting the primary IPv6 address for `networking.ip6`, `networking.network6`, and `networking.scope6`, the Go port SHALL prefer routable addresses (global scope, then unique-local) over link-local addresses on the primary interface. This is a deliberate, documented deviation from Ruby Facter's first-bound-address rule, which can surface `fe80::` link-locals.

#### Scenario: Routable address wins over link-local
- **WHEN** the primary interface carries both a link-local (`fe80::/10`) and a routable (global or unique-local) IPv6 address
- **THEN** `networking.ip6` MUST report the routable address and `networking.scope6` its scope, regardless of binding order

#### Scenario: Link-local only
- **WHEN** the primary interface carries only link-local IPv6 addresses
- **THEN** `networking.ip6` MUST report the link-local address with `networking.scope6` of `link`

#### Scenario: Deviation is documented
- **WHEN** an operator reads the man page Go-port notes
- **THEN** the IPv6 selection deviation from Ruby Facter MUST be stated there

### Requirement: Runtime and package-version facts are Go-native
Facts SHALL NOT expose Ruby runtime or Puppet package-version built-ins. The canonical built-in fact surface is Go-native; operator-supplied external facts remain the compatibility input surface.

#### Scenario: Ruby and puppet-agent package facts are absent
- **WHEN** core facts are resolved on any supported platform, even when Ruby, Puppet, or puppet-agent files are installed
- **THEN** the canonical tree MUST NOT contain `ruby`, `aio_agent_version`, or any Puppet package-version fact emitted by core discovery

