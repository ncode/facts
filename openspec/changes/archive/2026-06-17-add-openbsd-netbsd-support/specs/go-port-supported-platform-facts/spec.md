## MODIFIED Requirements

### Requirement: Supported platform fact scope
The Go port SHALL treat Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, and NetBSD as supported release targets.

#### Scenario: In-scope Ruby spec disposition
- **WHEN** the parity audit processes Ruby specs for Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, NetBSD, shared framework behavior, custom facts, external facts, and in-scope Linux distro families
- **THEN** each spec file MUST receive an explicit disposition of covered, newly covered, intentional deviation, or blocked with a concrete reason

#### Scenario: Out-of-scope platform exclusion
- **WHEN** the audit sees Ruby or Go behavior for Solaris, AIX, DragonFly, or unvalidated generic BSD-family paths
- **THEN** that behavior MUST NOT be treated as a release blocker unless it is needed by a shared parser or helper used by Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, or NetBSD

### Requirement: Core fact parity
The Go port SHALL expose Ruby-compatible structured facts for each supported platform, except for the intentionally removed Ruby runtime and Puppet package-version built-ins. Legacy alias facts are not part of the surface.

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

### Requirement: Virtualization and cloud parity
The Go port SHALL match Ruby-compatible virtualization, hypervisor, and cloud metadata behavior on supported platforms.

#### Scenario: Virtualization and hypervisor facts
- **WHEN** supported-platform virtualization facts are resolved from Linux `virt-what`, cgroups, DMI, VMware, Xen, OpenVZ, Windows OEM/netkvm/WMI indicators, macOS indicators, FreeBSD virtualization indicators, OpenBSD DMI product indicators, or NetBSD indicators identified by the parity audit
- **THEN** the Go port MUST match Ruby `virtual`, `is_virtual`, `hypervisors.*`, Xen, container, and nil/unknown behavior for supported detection paths

#### Scenario: Cloud metadata facts
- **WHEN** EC2, GCE, or Azure metadata is available, invalid, blocked by virtualization/provider checks, empty, or unavailable
- **THEN** the Go port MUST match Ruby cloud provider gating, request headers, metadata normalization, user-data behavior, nil values, and diagnostic behavior on supported platforms

### Requirement: Core facts are assembled by category for independent testing

Core-fact resolution SHALL be organized into per-fact-category resolver modules, with a primary file per category (e.g. networking, processors, memory, os, dmi, disks, ssh) and optional non-GOOS auxiliary files only when a category becomes unwieldy. Each category module SHALL expose a package-internal assembly function that returns that category's resolved facts from a resolution Session. The core-fact orchestrator SHALL be the composition of those category functions, so a test MAY resolve and assert a single category in isolation without running full core-fact discovery. This is a structural constraint only: the resolved fact set, names, values, ordering after collection, and per-platform behavior MUST remain identical, and a per-platform split within a category MUST NOT use Go's reserved GOOS filename suffixes (`_linux`, `_windows`, `_darwin`, `_freebsd`, `_openbsd`, `_netbsd`), which would impose an implicit build constraint and exclude cross-platform resolver logic from other platforms' builds and tests.

#### Scenario: A category resolves independently of the full core set

- **WHEN** a test invokes a single core-fact category's assembly function (for example the networking category) with a resolution Session backed by category-specific fake host inputs
- **THEN** it MUST receive only that category's resolved facts, without invoking `CoreFacts`, `buildCoreFacts`, or another category's assembly function

#### Scenario: Category composition preserves the core fact set

- **WHEN** core facts are discovered through the per-category orchestrator and compared with the pre-split baseline under the same host and fixture inputs
- **THEN** the resolved fact names and values MUST be identical to the pre-split core fact set on every supported platform, with no fact added, removed, or reshaped

#### Scenario: Cross-platform resolver logic stays buildable on every platform

- **WHEN** a category's resolver or parsing logic for one platform (for example parsing Windows networking command output) is exercised by a deterministic Go test
- **THEN** that logic MUST compile and run on the other supported platforms' builds and CI, reached through the `goos` parameter seam rather than gated behind a GOOS-suffixed file

## ADDED Requirements

### Requirement: OpenBSD and NetBSD live validation
The Go port SHALL have repeatable OpenBSD and NetBSD validation paths before either platform is treated as release complete.

#### Scenario: Local BSD smoke gates
- **WHEN** maintainers run the OpenBSD or NetBSD validation target from macOS or Linux
- **THEN** the workflow MUST build the matching BSD binary, run it in the matching BSD guest, and verify a release-gate fact set that includes at least `os.name`, `os.family`, `os.release`, `kernel`, `virtual`, `is_virtual`, `networking`, `memory`, `processors`, `disks`, `partitions`, `mountpoints`, `system_uptime`, `load_averages`, `ssh`, `timezone`, and any OS-specific DMI fact the parity audit marks supported

#### Scenario: BSD fixture-backed parity
- **WHEN** OpenBSD or NetBSD fact behavior depends on sysctl, mount, df, route, DHCP, disk, DMI, or other OS command output
- **THEN** deterministic Go tests MUST use fixtures and injectable seams so the behavior remains covered even when live BSD smoke gates are not running
