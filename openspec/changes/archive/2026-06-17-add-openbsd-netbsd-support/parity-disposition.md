# OpenBSD and NetBSD Parity Disposition

This file closes task 1.3. It records the implementation disposition after the
local Puppet Facter source audit, VM fixture capture, and live smoke gates.

Status meanings:

- Covered: matches an existing Puppet Facter behavior for that OS.
- Newly covered: Facts now supports the fact with audited platform sources;
  for NetBSD this may intentionally exceed local Puppet Facter, which only has
  generic BSD coverage.
- Intentionally absent: omitted because Puppet Facter omits it for that OS, or
  because no audited stable platform source exists.
- Blocked: a useful fact exists in principle, but no stable source has been
  selected for this change.

| Area | OpenBSD disposition | NetBSD disposition |
| --- | --- | --- |
| OS and kernel | Covered. `kernel`, `kernelrelease`, `kernelversion`, `kernelmajversion`, `os.name`, `os.family`, `os.release`, `os.architecture`, and `os.hardware` use the existing BSD `uname` paths plus structured release parsing. | Newly covered. Local Puppet Facter has only generic BSD coverage, but Facts intentionally emits the supported structured OS shape using audited `uname` output. |
| Networking | Covered. POSIX interfaces, route primary selection, hostname/FQDN/domain, bindings, and `dhcpleasectl` DHCP remain covered. | Newly covered for POSIX interfaces, bindings, and route primary selection. DHCP server resolution is intentionally absent until a stable text lease source is selected. |
| Memory and swap | Newly covered. `hw.physmem`, `vmstat -s` page counters, and `swapctl -sk` now replace zero-valued placeholder memory without using near-constant `hw.usermem` as free memory. | Newly covered. `hw.physmem64`/`hw.physmem` provide totals, `vmstat -s` page counters provide used/available memory, and no-swap output leaves swap values absent/null. |
| Processors | Newly covered. Generic BSD sysctl count/model/speed support is used, with speed absent when `hw.cpuspeed` is unavailable. | Newly covered. Generic BSD sysctl count/model/speed support is used, with speed absent when `hw.cpuspeed` is unavailable. |
| DMI | Covered. Existing `hw.vendor`, `hw.product`, `hw.version`, `hw.serialno`, and `hw.uuid` mapping remains the source. | Newly covered from audited `machdep.dmi.system-*` sysctls. Missing serial stays absent. |
| Virtualization | Covered. OpenBSD `hw.product` uses Facter's explicit product-name map; unknown QEMU product names remain physical. | Newly covered only as the generic `physical`/`is_virtual=false` fallback. No NetBSD-specific hypervisor map is claimed. |
| Disks | Newly covered from audited `sysctl hw.disknames` and `disklabel` output. This intentionally exceeds local Puppet Facter, which has no OpenBSD disk fact file. | Newly covered from audited `sysctl hw.disknames` and `disklabel` output, skipping NetBSD wedge devices in the disk list. This intentionally exceeds local Puppet Facter's generic BSD surface. |
| Partitions | Newly covered from `disklabel` non-`unused` partitions keyed as `/dev/<disk><letter>`, with mountpoints joined where device names match. | Newly covered from `dkctl <disk> listwedges`, keyed as `/dev/dkN`, with disklabel fallback when wedges are unavailable. |
| Mountpoints | Covered. OpenBSD `mount` plus `df -P` parsing remains fixture-backed. | Newly covered for mount metadata from NetBSD `mount`; size/stat fields remain absent because this change did not add NetBSD `statfs` support. |
| Uptime and load averages | Covered. Load averages use `vm.loadavg`; uptime falls back through the POSIX uptime parser when `kern.boottime` is not parseable. | Newly covered. Load averages use `vm.loadavg`; system uptime is an intentional structured Facts extension over local Puppet Facter's generic BSD coverage. |
| SSH | Covered. OpenBSD's empty resolver result emits an empty structured `ssh` map; populated host keys use the shared POSIX parser. | Newly covered as conditional shared POSIX SSH host-key discovery. |
| Timezone and path | Covered through shared POSIX/platform process sources. | Newly covered as shared POSIX/process facts. |
| Augeas | Covered when `augparse` exists. | Newly covered when `augparse` exists; absent otherwise. |
| FIPS | Intentionally absent outside Linux and Windows. | Intentionally absent outside Linux and Windows. |
| SELinux | Intentionally absent outside Linux. | Intentionally absent outside Linux. |
| ZFS and Zpool | Intentionally absent. OpenBSD has no stock `zfs`/`zpool` commands in the audited 7.9 guest and OpenZFS platform support does not include OpenBSD. | Newly covered conditionally. Facts parses Puppet Facter-compatible `zfs upgrade -v` and `zpool upgrade -v` output when the commands return version data. The audited NetBSD 10.1 guest requires boot-time `solaris` and `zfs` module loading via `/etc/modules.conf`; once loaded, it emits `zfs_version=5` and `zpool_version=5000`. |
| Schema, CI, and distribution | Covered by schema platform metadata, OpenBSD release gate, GitHub VM job, cross-compile row, local smoke target, and `make dist` target. | Covered by schema platform metadata, NetBSD release gate, GitHub VM job, cross-compile row, local smoke target, and `make dist` target. |
