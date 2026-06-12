# Delta: go-port-supported-platform-facts

## ADDED Requirements

### Requirement: Core fact parity
The Go port SHALL expose Ruby-compatible structured facts for each supported platform. Legacy alias facts are not part of the surface.

#### Scenario: Linux fact parity
- **WHEN** Linux facts are resolved for OS/release/distro, SELinux, identity, networking, DHCP, memory, swap, processors, DMI, disks, partitions, filesystems, mountpoints, uptime, load averages, virtualization, hypervisors, cloud metadata, SSH, timezone, path, FIPS, Ruby, Augeas, ZFS, and Zpool
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, fallback precedence, diagnostics, and formatted output for supported Linux behavior

#### Scenario: macOS fact parity
- **WHEN** macOS/Darwin facts are resolved for OS/release, macOS product/build/version, identity, networking, DHCP, memory, swap, processors, DMI, system profiler hardware/software/ethernet, filesystems, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, Ruby, and Augeas
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, command parsing, fallback precedence, diagnostics, and formatted output for supported macOS behavior

#### Scenario: Windows fact parity
- **WHEN** Windows facts are resolved for OS/release/product metadata, system32 path, identity, networking, DHCP, memory, processors, DMI, kernel, FIPS, virtualization, hypervisors, cloud metadata, SSH, uptime, timezone, and path
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, WMI/registry parsing, fallback precedence, and diagnostic messages for supported Windows behavior

#### Scenario: FreeBSD fact parity
- **WHEN** FreeBSD facts are resolved for OS/release, identity, networking, memory, swap, processors, DMI, disks, partitions, filesystems, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, Ruby, and Augeas
- **THEN** the Go port MUST match Ruby structured fact names, values, nil behavior, sysctl/geom/mount/df/parser behavior, fallback precedence, diagnostics, and formatted output for supported FreeBSD behavior

## MODIFIED Requirements

### Requirement: FreeBSD live validation
The Go port SHALL have a repeatable FreeBSD validation path before the port is declared complete.

#### Scenario: Lima FreeBSD smoke gate
- **WHEN** maintainers run the FreeBSD validation target from macOS using Lima
- **THEN** the workflow MUST build a FreeBSD binary, run it in a FreeBSD guest, and verify a release-gate fact set that includes at least `os.name`, `os.release`, `kernel`, `virtual`, `is_virtual`, `memory`, `processors`, `mountpoints`, `filesystems`, and `dmi`

#### Scenario: FreeBSD fixture-backed parity
- **WHEN** FreeBSD fact behavior depends on sysctl, geom, mount, df, ifconfig, route, kenv, or other OS command output
- **THEN** deterministic Go tests MUST use fixtures and injectable seams so the behavior remains covered even when the live Lima smoke gate is not running

## REMOVED Requirements

### Requirement: Core fact and legacy alias parity
**Reason**: Legacy alias facts are removed entirely; Facts exposes only the canonical structured tree. Structured-fact parity continues under the new "Core fact parity" requirement.
**Migration**: Consumers of legacy aliases move to structured equivalents (`operatingsystem` → `os.name`, `osfamily` → `os.family`, `architecture` → `os.architecture`, `hardwaremodel` → `os.hardware`, `processorcount` → `processors.count`, `memorysize` → `memory.system.total`, `hostname`/`fqdn`/`domain` → `networking.*`, `ssh*key`/`sshfp_*` → `ssh.*`).
