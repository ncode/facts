# go-port-supported-platform-facts Specification

## Purpose
TBD - created by archiving change complete-supported-platform-go-port. Update Purpose after archive.
## Requirements
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
The Go port SHALL match Ruby-compatible virtualization, hypervisor, and cloud metadata behavior on supported platforms, and MAY expose accurate Facts-native virtualization detection when a supported platform has stable native indicators not covered by Ruby Facter.

#### Scenario: Virtualization and hypervisor facts
- **WHEN** supported-platform virtualization facts are resolved from Linux `virt-what`, cgroups, DMI, VMware, Xen, OpenVZ, Windows OEM/netkvm/WMI indicators, macOS indicators, FreeBSD virtualization indicators, OpenBSD DMI product/vendor indicators, NetBSD DMI indicators, DragonFly DMI/PCI indicators, illumos SMBIOS/PCI indicators, or other indicators identified by the parity audit
- **THEN** the Go port MUST match Ruby `virtual`, `is_virtual`, `hypervisors.*`, Xen, container, and nil/unknown behavior for supported detection paths
- **AND** it MUST report QEMU/KVM guests as virtual when those supported native indicators expose QEMU, SeaBIOS, or Virtio metadata

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

### Requirement: Not-applicable facts are omitted
A fact that cannot resolve a value or does not apply to the host platform SHALL be absent from the canonical tree. Facts MUST NOT be emitted with empty-string values, empty-map values, or platform-inapplicable defaults. Additional accurate structured data beyond Ruby Facter's set MAY be exposed only as a documented deviation.

#### Scenario: Unresolvable facts are absent
- **WHEN** a fact's source cannot produce a value (no augparse binary for `augeas.version`, no enumerable devices for `disks`/`partitions`, unknown `processors.speed`)
- **THEN** the fact (or key) MUST be absent from every output mode, not rendered as an empty string or empty map

#### Scenario: Optional primary networking values are absent when unresolved
- **WHEN** supported-platform networking facts are rendered for full structured output
- **THEN** optional top-level primary fields such as `networking.ip`, `networking.ip6`, `networking.mac`, `networking.netmask`, `networking.netmask6`, `networking.network`, `networking.network6`, `networking.primary`, and `networking.scope6` MUST be absent when unresolved
- **AND** populated primary networking values MUST still be emitted

#### Scenario: Empty mountpoint entries are absent
- **WHEN** supported-platform mountpoint facts are rendered for full structured output
- **THEN** entries with no mountpoint fields MUST be absent instead of rendering an empty map
- **AND** populated mountpoint entries MUST still be emitted

#### Scenario: Platform-inapplicable facts are absent
- **WHEN** discovery runs on a platform where Ruby Facter does not resolve a fact (`fips_enabled` outside Linux and Windows, `os.selinux` outside Linux)
- **THEN** that fact MUST be absent from the canonical tree on that platform

#### Scenario: Additional data is a documented deviation
- **WHEN** the Go port exposes accurate structured data Ruby Facter lacks on that platform (e.g. `processors.extensions` on ARM macOS)
- **THEN** the deviation MUST be documented in the man page COMPATIBILITY section

### Requirement: Primary IPv6 selection prefers routable addresses
When selecting the primary IPv6 address for `networking.ip6`, `networking.network6`, and `networking.scope6`, the Go port SHALL prefer routable addresses (global scope, then unique-local) over link-local addresses on the primary interface. This is a deliberate, documented deviation from Ruby Facter's first-bound-address rule, which can surface `fe80::` link-locals.

#### Scenario: Routable address wins over link-local
- **WHEN** the primary interface carries both a link-local (`fe80::/10`) and a routable (global or unique-local) IPv6 address
- **THEN** `networking.ip6` MUST report the routable address and `networking.scope6` its scope, regardless of binding order

#### Scenario: Link-local only
- **WHEN** the primary interface carries only link-local IPv6 addresses
- **THEN** `networking.ip6` MUST report the link-local address with `networking.scope6` of `link`

#### Scenario: Deviation is documented
- **WHEN** an operator reads the man page COMPATIBILITY section
- **THEN** the IPv6 selection deviation from Ruby Facter MUST be stated there

### Requirement: Runtime and package-version facts are Go-native
Facts SHALL NOT expose Ruby runtime or Puppet package-version built-ins. The canonical built-in fact surface is Go-native; operator-supplied external facts remain the compatibility input surface.

#### Scenario: Ruby and puppet-agent package facts are absent
- **WHEN** core facts are resolved on any supported platform, even when Ruby, Puppet, or puppet-agent files are installed
- **THEN** the canonical tree MUST NOT contain `ruby`, `aio_agent_version`, or any Puppet package-version fact emitted by core discovery

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

### Requirement: OpenBSD and NetBSD live validation
The Go port SHALL have repeatable OpenBSD and NetBSD validation paths before either platform is treated as release complete.

#### Scenario: Local BSD smoke gates
- **WHEN** maintainers run the OpenBSD or NetBSD validation target from macOS or Linux
- **THEN** the workflow MUST build the matching BSD binary, run it in the matching BSD guest, and verify a release-gate fact set that includes at least `os.name`, `os.family`, `os.release`, `kernel`, `virtual`, `is_virtual`, `networking`, `memory`, `processors`, `disks`, `partitions`, `mountpoints`, `system_uptime`, `load_averages`, `ssh`, `timezone`, and any OS-specific DMI fact the parity audit marks supported

#### Scenario: BSD fixture-backed parity
- **WHEN** OpenBSD or NetBSD fact behavior depends on sysctl, mount, df, route, DHCP, disk, DMI, or other OS command output
- **THEN** deterministic Go tests MUST use fixtures and injectable seams so the behavior remains covered even when live BSD smoke gates are not running

### Requirement: Linux disk serial uses canonical spelling

Linux disk serial numbers SHALL be emitted as `disks.*.serial_number`, matching the schema-owned canonical spelling for the same concept on other supported release targets.

#### Scenario: Linux disk serial key

- **WHEN** Linux disk discovery finds a disk serial number
- **THEN** the disk entry MUST contain `serial_number`
- **AND** the disk entry MUST NOT contain `serial`

### Requirement: Validated BSD fact extensions
Facts SHALL extend FreeBSD, OpenBSD, and NetBSD structured fact coverage only when a native platform source is stable, fixture-backed, and represented in the schema. Unsupported or unstable BSD fields MUST remain absent from the canonical tree.

#### Scenario: NetBSD mountpoint byte facts
- **WHEN** NetBSD mountpoints are resolved from `mount` and `df -P` output
- **THEN** each mountpoint with matching `df` data MUST include `available`, `available_bytes`, `capacity`, `size`, `size_bytes`, `used`, and `used_bytes`
- **AND** mountpoints without matching size data MUST omit those byte and capacity fields

#### Scenario: BSD interface operational state
- **WHEN** FreeBSD, OpenBSD, or NetBSD `ifconfig` output reports an interface status
- **THEN** `networking.interfaces.<name>.operational_state` MUST be emitted with the normalized status value for that interface

#### Scenario: FreeBSD interface speed and duplex
- **WHEN** FreeBSD `ifconfig -m` output reports negotiated media speed and duplex for an interface
- **THEN** `networking.interfaces.<name>.speed` MUST be emitted in Mbit/s
- **AND** `networking.interfaces.<name>.duplex` MUST be emitted as `full` or `half`

#### Scenario: FreeBSD interface DHCP
- **WHEN** a FreeBSD DHCP lease file for an interface contains a `dhcp-server-identifier`
- **THEN** `networking.interfaces.<name>.dhcp` MUST report that server address
- **AND** `networking.dhcp` MUST report the primary interface DHCP server when known

#### Scenario: FreeBSD partition type metadata
- **WHEN** FreeBSD GEOM XML reports partition type metadata for a partition provider
- **THEN** `partitions.<name>.parttype` MUST report the platform partition type identifier

#### Scenario: Conditional FreeBSD disk type
- **WHEN** FreeBSD disk metadata reports a known rotation rate
- **THEN** `disks.<name>.type` MUST be emitted as `ssd` or `hdd`
- **AND** the field MUST be absent when the platform reports an unknown rotation rate

#### Scenario: BSD DMI product version disposition
- **WHEN** OpenBSD or NetBSD exposes a stable system product version source
- **THEN** Facts MUST either expose it as `dmi.product.version` with schema coverage or document and test why it remains absent
- **AND** Facts MUST NOT silently move an existing `dmi.bios.version` value without a compatibility test covering the changed shape

#### Scenario: Unstable NetBSD DHCP source remains absent
- **WHEN** NetBSD DHCP data is available only from an unstable or binary lease source
- **THEN** `networking.interfaces.<name>.dhcp` and `networking.dhcp` MUST remain absent unless a stable text source is selected and tested

### Requirement: Kernel facts are structured

Kernel facts SHALL be emitted as a structured `kernel` map rather than as flat top-level facts.

#### Scenario: Kernel output shape

- **WHEN** core facts are resolved on any supported release target
- **THEN** `kernel.name` MUST contain the kernel name
- **AND** `kernel.release.full` MUST contain the full kernel release
- **AND** `kernel.release.major` and `kernel.release.minor` MUST contain the parsed release components when available
- **AND** `kernel.release.patch` MUST be present only when a patch component is available
- **AND** `kernel.version.full` MUST contain the kernel version
- **AND** `kernelmajversion`, `kernelrelease`, and `kernelversion` MUST be absent

### Requirement: Collection facts are arrays

Collection facts SHALL be emitted as arrays rather than delimiter-separated strings.

#### Scenario: Filesystems output shape

- **WHEN** filesystem types are resolved on Linux or macOS/Darwin
- **THEN** `filesystems` MUST be an array of filesystem type strings
- **AND** it MUST NOT be a comma-separated string

#### Scenario: PATH output shape

- **WHEN** core facts are resolved on any supported release target
- **THEN** `path` MUST be an array of PATH entries in lookup order
- **AND** platform path-list separators MUST NOT appear inside entries unless they are part of the entry text itself
- **AND** empty path entries MUST be omitted

#### Scenario: ZFS output shape

- **WHEN** usable ZFS command output is available on a supported platform
- **THEN** `zfs.feature_numbers` MUST be an array of supported filesystem version strings
- **AND** `zfs.version` MUST be the latest supported filesystem version string
- **AND** `zfs_featurenumbers` and `zfs_version` MUST be absent

#### Scenario: Zpool output shape

- **WHEN** usable Zpool command output is available on a supported platform
- **THEN** `zpool.feature_numbers` MUST be an array of supported pool version strings
- **AND** `zpool.feature_flags` MUST be an array of supported pool feature flag strings when feature flags are available
- **AND** `zpool.version` MUST be the latest supported pool version string, or `5000` when feature flags are present
- **AND** `zpool_featurenumbers`, `zpool_featureflags`, and `zpool_version` MUST be absent

### Requirement: Host probes remain Session-injectable

Facts SHALL keep host I/O used for platform fact discovery reachable through the run-scoped Session seam so category behavior can be tested with injected native source data. The Session host seam SHALL be the only resolver host-I/O path: core fact resolvers MUST NOT read the host through raw `os`/`filepath`/`exec` calls or through parameter-injected reader/runner alternatives that duplicate the Session seam, and this MUST be structurally enforced by an automated check with a fixed, documented exclusion list (the seam implementation itself, the external-fact loader, the persistent cache, config parsing, syscall-tagged files, and test files). Category assemblies SHALL obtain platform identity through the Session (`s.goos()`) and environment values through the Session's host environment with Windows-only case-insensitive lookup, so platform-conditional assembly paths are exercisable with a fake host on any development platform. Pure parse functions keep their string and goos parameters per the category-split contract; recorded exceptions (process clock, `exec.LookPath` in the Linux distro probe, identity's uid/gid syscalls, `net.Interfaces`) remain injectable parameters where tests need them.

#### Scenario: Disk probes are injectable

- **WHEN** disk, partition, or mountpoint facts need command output, file reads, stat data, directory reads, glob matches, or platform identity
- **THEN** tests MUST be able to provide those inputs without reading the developer host directly

#### Scenario: Session command behavior is preserved

- **WHEN** a fact resolver executes a platform command through the Session host seam
- **THEN** command timeout, context cancellation, logging, and sanitized environment behavior MUST remain consistent with current Session command execution

#### Scenario: Resolver host I/O cannot bypass the seam

- **WHEN** a core fact resolver outside the documented exclusion list reads a file, lists a directory, stats a path, expands a glob, or executes a command through raw `os`/`filepath`/`exec` calls instead of the Session host seam
- **THEN** the automated seam check fails, identifying the offending file and call

#### Scenario: Category assembly is drivable onto another platform

- **WHEN** a test constructs a Session whose fake host reports a platform identity different from the test host (e.g. windows assembly driven from a Linux CI host)
- **THEN** the category assembly functions resolve using the fake host's platform identity, environment values, file contents, and command outputs, without reaching the real host

### Requirement: Platform capability policy is explicit

Facts SHALL keep coarse platform capability policy explicit while preserving category-oriented resolver modules.

#### Scenario: Not-applicable fact groups are omitted by policy

- **WHEN** a target profile marks a fact group as inapplicable for the current platform
- **THEN** the relevant category module MUST omit that fact group rather than emitting empty placeholder values

#### Scenario: Category modules own resolver implementation

- **WHEN** platform capability policy is added or changed
- **THEN** parser and resolver bodies MUST remain in the relevant category modules rather than moving into a platform registry

### Requirement: Host virtualization is gathered once per discovery

The host-virtualization signal gather (on Linux: DMI reads plus the dmidecode/virt-what/vmware/lspci command set; on Windows: the wmic/CIM and registry gather) SHALL run at most once per discovery, memoized on the run-scoped Session like other shared host probes, with the `virtual`/`is_virtual` facts, the `hypervisors` fact tree, and the uptime container gate all reading the same memoized gather input. Classification of the gathered input SHALL remain a pure derivation so memoizing the gather does not change any resolved fact value.

#### Scenario: Linux gather commands run once

- **WHEN** a discovery resolves `virtual`, `hypervisors`, and `system_uptime` on a Linux host
- **THEN** each virtualization gather command (dmidecode, virt-what, vmware, lspci) is executed at most once for that discovery, and all three consumers observe facts derived from the same gather

#### Scenario: Windows gather runs once

- **WHEN** a discovery resolves `virtual` and `hypervisors` on a Windows host
- **THEN** the wmic/CIM and registry virtualization gather executes at most once for that discovery

#### Scenario: Memoization is discovery-scoped

- **WHEN** the same Engine runs two discoveries
- **THEN** the second discovery re-gathers virtualization signals fresh (the memo lives on the per-discovery Session, not the Engine)

#### Scenario: Resolved values are unchanged by memoization

- **WHEN** the memoized gather input is classified for the `virtual` fact and for the `hypervisors` tree
- **THEN** each consumer's classification produces the same fact names and values as before memoization, including their documented divergences

