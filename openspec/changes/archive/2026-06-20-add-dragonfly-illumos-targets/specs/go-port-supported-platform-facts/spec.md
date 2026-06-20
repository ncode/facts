## MODIFIED Requirements

### Requirement: Supported platform fact scope
The Go port SHALL treat Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, NetBSD, DragonFly BSD, and illumos as supported release targets after DragonFly and illumos complete candidate-target promotion.

#### Scenario: In-scope Ruby spec disposition
- **WHEN** the parity audit processes Ruby specs for Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, NetBSD, shared framework behavior, custom facts, external facts, and in-scope Linux distro families
- **THEN** each spec file MUST receive an explicit disposition of covered, newly covered, intentional deviation, or blocked with a concrete reason

#### Scenario: Candidate target disposition
- **WHEN** the audit processes DragonFly BSD or illumos behavior
- **THEN** each fact category MUST receive an explicit disposition of covered, newly covered, intentionally absent, blocked, or Facts-native extension with the native source and validation path recorded

#### Scenario: Out-of-scope platform exclusion
- **WHEN** the audit sees Ruby or Go behavior for Oracle Solaris, AIX, or unvalidated generic BSD-family paths
- **THEN** that behavior MUST NOT be treated as a release blocker unless it is needed by a shared parser or helper used by a supported release target

#### Scenario: OmniOS does not validate Oracle Solaris
- **WHEN** illumos behavior is validated through OmniOS
- **THEN** the result MUST apply to the illumos target only
- **AND** Oracle Solaris MUST remain outside the supported release target set until it has its own repeatable validation host

### Requirement: Core fact parity
The Go port SHALL expose Ruby-compatible structured facts for each supported platform where Ruby Facter has comparable behavior, except for the intentionally removed Ruby runtime and Puppet package-version built-ins. Legacy alias facts are not part of the surface. Facts MAY expose Facts-native extensions when the native source is stable, the canonical fact spelling is schema-documented, and platform validation covers the behavior.

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

## ADDED Requirements

### Requirement: DragonFly and illumos live validation
The Go port SHALL have repeatable native validation paths for DragonFly BSD and illumos before either platform is treated as release complete.

#### Scenario: Candidate native smoke gates
- **WHEN** maintainers run the DragonFly or illumos candidate release gate
- **THEN** the workflow MUST build the matching binary, run it on the matching native guest, and verify the target's release-gate fact set through structured fact names only

#### Scenario: DragonFly fixture-backed coverage
- **WHEN** DragonFly fact behavior depends on sysctl, ifconfig, route, mount, df, disklabel, swap, or other OS command output
- **THEN** deterministic Go tests MUST use fixtures and injectable seams so the behavior remains covered when the live guest is not running

#### Scenario: illumos fixture-backed coverage
- **WHEN** illumos fact behavior depends on uname, prtconf, kstat, swap, ifconfig, route, mount, df, zfs, zpool, zoneadm, or other OS command output
- **THEN** deterministic Go tests MUST use fixtures and injectable seams so the behavior remains covered when the live guest is not running

#### Scenario: Native source values are reported as-is
- **WHEN** a DragonFly or illumos native source reports a surprising but parseable value such as an unusual boot time from a lab VM
- **THEN** Facts MUST report the native source value without inventing a correction
- **AND** the release gate MUST validate shape and presence rather than imposing unrelated sanity ranges
