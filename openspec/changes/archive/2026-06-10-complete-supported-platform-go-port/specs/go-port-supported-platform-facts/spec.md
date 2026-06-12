## ADDED Requirements

### Requirement: Supported platform fact scope
The Go port SHALL treat Linux, macOS/Darwin, Windows, and FreeBSD as supported release targets.

#### Scenario: In-scope Ruby spec disposition
- **WHEN** the parity audit processes Ruby specs for Linux, macOS/Darwin, Windows, FreeBSD, shared framework behavior, custom facts, external facts, and in-scope Linux distro families
- **THEN** each spec file MUST receive an explicit disposition of covered, newly covered, intentional deviation, or blocked with a concrete reason

#### Scenario: Out-of-scope platform exclusion
- **WHEN** the audit sees Ruby or Go behavior for Solaris, AIX, OpenBSD, NetBSD, DragonFly, or unvalidated generic BSD-family paths
- **THEN** that behavior MUST NOT be treated as a release blocker unless it is needed by a shared parser or helper used by Linux, macOS/Darwin, Windows, or FreeBSD

### Requirement: Core fact and legacy alias parity
The Go port SHALL expose Ruby-compatible structured facts and legacy aliases for each supported platform.

#### Scenario: Linux fact parity
- **WHEN** Linux facts are resolved for OS/release/distro, SELinux, identity, networking, DHCP, memory, swap, processors, DMI, disks, partitions, filesystems, mountpoints, uptime, load averages, virtualization, hypervisors, cloud metadata, SSH, timezone, path, FIPS, Ruby, Augeas, ZFS, Zpool, and legacy aliases
- **THEN** the Go port MUST match Ruby fact names, values, nil behavior, fallback precedence, legacy alias visibility, diagnostics, and formatted output for supported Linux behavior

#### Scenario: macOS fact parity
- **WHEN** macOS/Darwin facts are resolved for OS/release, macOS product/build/version, identity, networking, DHCP, memory, swap, processors, DMI, system profiler hardware/software/ethernet, filesystems, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, Ruby, Augeas, and legacy aliases
- **THEN** the Go port MUST match Ruby fact names, values, nil behavior, command parsing, fallback precedence, legacy alias visibility, diagnostics, and formatted output for supported macOS behavior

#### Scenario: Windows fact parity
- **WHEN** Windows facts are resolved for OS/release/product metadata, system32 path, identity, networking, DHCP, memory, processors, DMI, kernel, FIPS, virtualization, hypervisors, cloud metadata, SSH, uptime, timezone, path, Ruby, and legacy aliases
- **THEN** the Go port MUST match Ruby fact names, values, nil behavior, WMI/registry parsing, fallback precedence, diagnostic messages, legacy alias visibility, and formatted output for supported Windows behavior

#### Scenario: FreeBSD fact parity
- **WHEN** FreeBSD facts are resolved for OS/release, identity, networking, memory, swap, processors, DMI, disks, partitions, filesystems, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, Ruby, Augeas, and legacy aliases
- **THEN** the Go port MUST match Ruby fact names, values, nil behavior, sysctl/geom/mount/df/parser behavior, fallback precedence, legacy alias visibility, diagnostics, and formatted output for supported FreeBSD behavior

### Requirement: Resolver fallback and diagnostic parity
The Go port SHALL preserve supported-platform Ruby resolver fallback order and diagnostics.

#### Scenario: Command and file fallback behavior
- **WHEN** a supported-platform resolver reads files, executes commands, queries WMI or registry data, parses system profiler/sysctl/geom output, or receives missing, empty, malformed, permission-denied, or invalid data
- **THEN** the Go port MUST match Ruby fallback order, nil/default fact shaping, debug/warn/error text, and non-fatal continuation behavior

#### Scenario: Platform-specific command isolation
- **WHEN** facts are resolved on one supported platform
- **THEN** the Go port MUST only execute or read platform-appropriate sources for that platform and MUST keep deterministic Go tests injectable through command, filesystem, time, environment, HTTP, and platform seams

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
- **THEN** the workflow MUST build a FreeBSD binary, run it in a FreeBSD guest, and verify a release-gate fact set that includes at least `os.name`, `os.release`, `kernel`, `virtual`, `is_virtual`, `memory`, `processors`, `mountpoints`, `filesystems`, `dmi`, and representative legacy aliases

#### Scenario: FreeBSD fixture-backed parity
- **WHEN** FreeBSD fact behavior depends on sysctl, geom, mount, df, ifconfig, route, kenv, or other OS command output
- **THEN** deterministic Go tests MUST use fixtures and injectable seams so the behavior remains covered even when the live Lima smoke gate is not running
