# OpenBSD and NetBSD Parity Audit

Scope: source audit for task 1.1 only. Fixture capture is task 1.2; final
covered/new/absent/blocked dispositions are task 1.3.

Primary parity reference: local Puppet Facter source at
`/Users/ncode/Devel/facter`.

## Source Findings

- Puppet Facter detects `openbsd` explicitly, but detects `netbsd` through the
  generic `/bsd/` branch in
  `/Users/ncode/Devel/facter/lib/facter/framework/detector/os_detector.rb`.
- Puppet Facter's configured BSD hierarchy contains `Freebsd` and `Openbsd`,
  but not `Netbsd`, in
  `/Users/ncode/Devel/facter/lib/facter/config.rb`.
- Therefore OpenBSD loads the generic `bsd` facts plus the OpenBSD-specific
  facts. NetBSD loads only generic `bsd` facts from the local Facter source.
- Generic `bsd` facts are limited to `kernelversion`, `kernelmajversion`,
  `load_averages`, `os.family`, `processors.count`, `processors.models`, and
  `processors.speed`.
- OpenBSD-specific Facter adds OS, identity, networking, DMI, mountpoints,
  virtualization, SSH, timezone, path, Augeas, EC2, Ruby runtime, and legacy
  aliases. Facts intentionally does not support Ruby runtime facts or legacy
  aliases.

## Category Table

| Category | Puppet Facter OpenBSD | Current Facts OpenBSD | Puppet Facter NetBSD | Current Facts NetBSD | Highest-risk gap |
| --- | --- | --- | --- | --- | --- |
| OS | Generic BSD `kernelversion`, `kernelmajversion`, `os.family`; OpenBSD adds `kernel`, `kernelrelease`, `os.name`, `os.release`, `os.architecture`, `os.hardware`. `os.release` is `{full, major, minor}` from `uname -r`. | Mostly present in `internal/engine/os.go`: OpenBSD name/family/kernel/release are explicit and release is structured from `uname -r`. Kernel version uses a shorter BSD regex than Facter's all-dot numeric regex. | Generic BSD only: `os.family`, `kernelversion`, `kernelmajversion`. No NetBSD-specific `kernel`, `kernelrelease`, `os.name`, `os.release`, architecture, or hardware files in local Facter. | Emits universal OS facts. `os.family` is `NetBSD`, but `os.name` and `kernel` fall through to lowercase `netbsd`; `os.release` falls back to raw kernel release string. | NetBSD proposal scope exceeds local Facter. If we still support rich NetBSD OS facts, implementation needs an explicit intentional-extension note. |
| Networking | OpenBSD-specific structured networking facts plus legacy interface facts. Uses shared networking resolver and `dhcpleasectl -l <iface>` for DHCP. | Present in `networking.go`: POSIX interface enumeration, OpenBSD DHCP via `dhcpleasectl`, `route -n get default`, expanded bindings. Needs fixture comparison for exact nil and primary behavior. | No generic BSD networking facts in local Facter. | Emits full networking facts through universal core, but no NetBSD-specific route or DHCP path; default branch returns no configured primary. | High. NetBSD current behavior is neither local-Facter parity nor proposed support. |
| Memory | No OpenBSD memory fact files loaded by local Facter. | Emits full `memory.*` fact set with zero system totals and nil swap values because OpenBSD has no resolver in `memory.go`. | No generic BSD memory facts in local Facter. | Same zero/nil universal memory output as OpenBSD. | High. Current zero-valued memory facts look resolved but are not source-backed. |
| Processors | Generic BSD count/models/speed from sysctl OIDs plus OpenBSD `processors.isa` from `uname -p`. | Emits count/topology from Go runtime, ISA from `uname -p`, models mostly from architecture fallback, speed absent. | Generic BSD count/models/speed from sysctl OIDs. No `processors.isa` in generic BSD. | Emits count/topology from Go runtime, models from architecture fallback, speed absent, ISA emitted by universal core. | High. Sysctl-backed model and speed parity are missing for both; NetBSD also over-emits ISA relative to local Facter. |
| DMI | OpenBSD-specific DMI from `hw.vendor`, `hw.version`, `hw.product`, `hw.serialno`, `hw.uuid`. | Present in `dmi.go` using `/sbin/sysctl -n` for the same keys and the same structured DMI shape. | No generic BSD DMI facts in local Facter. | No NetBSD DMI resolver, so DMI is absent unless Linux-style `/sys/class/dmi/id` somehow exists. | OpenBSD is low risk; NetBSD should stay absent unless task 1.2 finds a stable source and proposal intentionally extends local Facter. |
| Disks | No OpenBSD disks fact file in local Facter. | Initially fell through to Linux `/sys/block`; the implementation decision is to exceed local Facter with audited `sysctl hw.disknames` plus `disklabel` size facts. | No generic BSD disks facts in local Facter. | Initially fell through to Linux `/sys/block`; the implementation decision is to exceed local Facter with audited NetBSD `sysctl hw.disknames` plus `disklabel`, skipping wedges. | Medium. Storage support is an intentional Facts extension over local Facter, so schema and gates must make the extension explicit. |
| Partitions | No OpenBSD partitions fact file in local Facter. | Implementation decision is to emit audited non-`unused` `disklabel` partitions keyed by `/dev/<disk><letter>`. | No generic BSD partitions facts in local Facter. | Implementation decision is to emit audited `dkctl listwedges` partitions keyed by `/dev/dkN`, with disklabel fallback. | Medium. Partition facts intentionally exceed local Facter and must be fixture-backed. |
| Mountpoints | OpenBSD-specific mountpoints resolver parses `mount` and merges size data from `df -P`. | Present in `disks.go`: OpenBSD special path parses `mount` plus `df -P`. Needs fixture comparison against local Facter's parser. | No generic BSD mountpoints facts in local Facter. | Emits `mountpoints` with a default `/` entry, but `statfs` is unsupported on NetBSD, so size data is absent. | High. NetBSD over-emits a weak placeholder relative to local Facter and still fails proposed release-gate quality. |
| Uptime | OpenBSD-specific `system_uptime.*`; generic BSD `load_averages` from `vm.loadavg`. | Present: POSIX uptime probes plus `vm.loadavg` for OpenBSD. | Generic BSD has only `load_averages`; no generic `system_uptime.*` files. | Emits both `system_uptime.*` and `load_averages`. | Medium. NetBSD `load_averages` is parity-backed, but `system_uptime.*` is an intentional extension if kept. |
| Virtualization | OpenBSD-specific `virtual` and `is_virtual`; reads `hw.product` and maps known DMI products, including `OpenBSD` to `vmm`. | Present in `virtual.go`: reads `hw.product` and uses the shared DMI product hypervisor map. | No generic BSD virtualization facts in local Facter. | Defaults to `virtual = physical`, `is_virtual = false`. | Medium. OpenBSD is likely covered; NetBSD current output is an over-claim relative to local Facter unless proposal requires the extension. |
| SSH | OpenBSD-specific structured `ssh` plus legacy SSH alias facts. | Structured `ssh` present; OpenBSD no-key result returns `{}` to match OpenBSD behavior. Legacy aliases intentionally omitted. | No generic BSD SSH facts in local Facter. | Emits structured `ssh` nil or keys through universal core. | Medium. OpenBSD likely covered; NetBSD over-emits relative to local Facter. |
| Timezone | OpenBSD-specific `timezone` from Facter timezone resolver. | Present through universal timezone core. | No generic BSD timezone fact in local Facter. | Emits timezone through universal core. | Low for OpenBSD, medium for NetBSD over-emission. |
| Path | OpenBSD-specific `path` from Facter path resolver. | Present through universal `PATH` fact in `core.go`. | No generic BSD path fact in local Facter. | Emits path through universal core. | Low for OpenBSD, medium for NetBSD over-emission. |
| FIPS | No OpenBSD FIPS fact in local Facter. | Absent because `fips_enabled` is Linux/Windows only. | No generic BSD FIPS fact in local Facter. | Absent. | Covered absent. |
| SELinux | No OpenBSD SELinux fact in local Facter. | Absent because `os.selinux.*` is Linux only. | No generic BSD SELinux fact in local Facter. | Absent. | Covered absent. |
| Augeas | OpenBSD-specific `augeas.version` when `augparse` resolves. | Present through universal Augeas resolver when `augparse` exists. | No generic BSD Augeas fact in local Facter. | Would emit `augeas.version` if `augparse` exists because resolver is universal. | Medium. NetBSD should be gated or documented as an intentional extension. |
| ZFS | No OpenBSD ZFS facts in local Facter. | Intentionally absent: stock OpenBSD has no audited `zfs` command source. | No generic BSD ZFS facts in local Facter. | Conditional extension: parse Puppet Facter-compatible `zfs upgrade -v` only when version output is usable. | Low. OpenBSD stays absent; NetBSD output is omitted on the stock guest because ZFS library initialization fails. |
| Zpool | No OpenBSD zpool facts in local Facter. | Intentionally absent: stock OpenBSD has no audited `zpool` command source. | No generic BSD zpool facts in local Facter. | Conditional extension: parse Puppet Facter-compatible `zpool upgrade -v` only when version output is usable. | Low. OpenBSD stays absent; NetBSD output is omitted on the stock guest because ZFS library initialization fails. |

## Implementation Notes For Later Tasks

- Do not treat FreeBSD Facter behavior as NetBSD behavior. The local Facter
  source gives NetBSD only generic BSD coverage.
- The riskiest current Facts outputs are zero-valued memory facts and NetBSD
  placeholder mountpoints, because they look resolved but are not backed by
  Puppet Facter or audited NetBSD probes.
- If the change intentionally supports richer NetBSD facts than local Puppet
  Facter, task 1.3 should mark those as intentional extensions before Go
  implementation starts.
