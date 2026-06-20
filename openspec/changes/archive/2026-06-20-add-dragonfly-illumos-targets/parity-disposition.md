## Native Fixture Summary

Captured through native lab guests with tracked command names only; private
connection details stay outside git.

### DragonFly BSD

- Release: `DragonFly 6.4-RELEASE`.
- Stable sources used: `uname -s/-r/-m`, `sysctl -n hw.physmem hw.ncpu hw.model hw.clockrate vm.loadavg kern.disks`, `vmstat -s`, `swapinfo -k`, `diskinfo`, `disklabel`, `mount`, `df -P`, POSIX identity/timezone/path, and Go networking.
- Candidate gate result: passed with `sudo -n sh tools/dragonfly-release-gate.sh`.
- Supported facts in this change: OS/kernel identity and release, networking, memory/swap, processors, uptime/load averages, mountpoints, disks, partitions, virtualization fallback, identity, SSH when keys are visible, timezone, path.
- Blocked/absent: FIPS, SELinux, DMI, ZFS, and Zpool remain absent.

### illumos / OmniOS

- Release: OmniOS `r151058`; kernel `SunOS 5.11`.
- Stable sources used: `/etc/release`, `uname -s/-r/-m`, `kstat -p unix:0:system_pages:*`, `pagesize`, `swap -s`, `psrinfo -pv`, `uptime`, `mount`, `df -P`, `zfs upgrade -v`, `zpool upgrade -v`, `zonename`, `zoneadm list -cp`, POSIX identity/timezone/path, and Go networking.
- Candidate gate result: passed with `tools/illumos-release-gate.sh`.
- Supported facts in this change: OS/kernel identity and release, networking, memory/swap, processors, uptime/load averages, mountpoints, virtualization fallback, identity, SSH when keys are visible, timezone, path, and conditional ZFS/Zpool facts.
- Blocked/absent: Oracle Solaris behavior is not claimed; `solaris/amd64` is not built or published. Disks, partitions, and zones are not added because there is no stable schema contract for them in this change.

### Existing BSD Lab Smoke

- FreeBSD amd64 release gate passed through the lab.
- OpenBSD and NetBSD amd64 gates pass through the lab when invoked with `sudo -n`; their disk/partition facts require privileged disk label reads.
