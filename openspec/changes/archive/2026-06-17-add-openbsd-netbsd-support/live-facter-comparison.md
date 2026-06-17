# Live Ruby Facter Comparison

Date: 2026-06-17

Guests:

- OpenBSD 7.9 arm64 under local QEMU
- NetBSD 10.1 arm64 under local QEMU

Tooling:

- Ruby Facter 4.10.0 installed in both guests.
- OpenBSD used Ruby 3.4 from packages plus `gem34 install facter -v 4.10.0` and `ruby34-ffi`.
- NetBSD used pkgsrc `ruby34-facter-4.10.0`; packages were fetched over IPv4 and installed locally because the default package fetch path hung on this VM.
- Facts used freshly built `GOOS=openbsd GOARCH=arm64` and `GOOS=netbsd GOARCH=arm64` binaries from this worktree.

OpenBSD result:

- Stable OS/kernel, DMI, virtualization, processors core fields, networking identity, SSH fingerprints, timezone, path, and interface binding values matched Ruby Facter after fixing two live-detected networking issues.
- `disks` and `partitions` are now Facts extensions backed by live `sysctl hw.disknames` and `disklabel` output; Ruby Facter 4.10.0 has no OpenBSD disk/partition fact files.
- `zfs` and `zpool` commands are not present on the audited OpenBSD 7.9 guest, so Facts omits the ZFS/zpool facts there.
- The fixes were:
  - omit `mtu: 0` for addressless POSIX interfaces such as `enc0`;
  - choose the first non-link IPv6 binding for per-interface IPv6 summaries while preserving host loopback summaries such as `lo0.ip6 = ::1`.
- Remaining live diffs are accepted:
  - mountpoint available/used/capacity values drift between command runs and use slightly different percentage math;
  - Facts documents and emits `networking.interfaces.<name>.dhcp`, while Ruby Facter 4.10.0 on OpenBSD emits only top-level `networking.dhcp`.

NetBSD result:

- Ruby Facter 4.10.0 on NetBSD arm64 emitted generic BSD facts only and logged FFI resolver errors (`unable to resolve type 'size_t'`) for richer resolvers.
- The usable Ruby comparison covered `os.family`, `kernelversion`, `kernelmajversion`, and `load_averages`; Facts matched those stable values, with load averages treated as volatile.
- Facts intentionally exceeds the usable Ruby Facter surface on this guest for structured OS release, networking, memory, DMI, processors, disks, partitions, mountpoints, SSH, timezone, path, uptime, and virtualization, backed by audited NetBSD probes and the local release gate.
- NetBSD has `/sbin/zfs` and `/sbin/zpool`; they initially returned `internal error: failed to initialize ZFS library` because `kern.securelevel=1` prevented manual `modload`. Adding `solaris` and `zfs` to `/etc/modules.conf` and rebooting loaded the modules before securelevel was raised. After that, Facts emitted `zfs_version=5`, `zfs_featurenumbers=1,2,3,4,5`, `zpool_version=5000`, legacy pool versions `1..28`, and supported zpool feature flags.

Validation:

- `go test ./internal/engine`
- `go test ./...`
- `go vet ./...`
- `make local-bsd-smoke`
