## 1. Audit And Fixtures

- [x] 1.1 Capture DragonFly fixture outputs for `uname`, `sysctl`, `ifconfig`, `route`, `mount`, `df -P`, swap tooling, disk tooling, SSH paths, timezone, and virtualization indicators.
- [x] 1.2 Capture illumos/OmniOS fixture outputs for `uname`, `prtconf`, `kstat`, `swap`, `ifconfig`, `route`, `mount`, `df`, `zfs`, `zpool`, zone tooling, SSH paths, timezone, and virtualization indicators.
- [x] 1.3 Create a parity/disposition note for DragonFly and illumos covering OS, networking, memory, processors, DMI, disks, partitions, mountpoints, uptime, load averages, virtualization, SSH, timezone, path, Augeas, ZFS, Zpool, FIPS, and SELinux.
- [x] 1.4 Mark each audited fact as covered, newly covered, intentionally absent, blocked, or Facts-native extension with the native source recorded.

## 2. Local Gate Tooling

- [x] 2.1 Add `tools/dragonfly-release-gate.sh` and `tools/illumos-release-gate.sh` that run on the guest and verify structured fact names only.
- [x] 2.2 Add Makefile wrapper variables for `LOCAL_FREEBSD_AMD64_SSH`, `LOCAL_OPENBSD_ARM64_SSH`, `LOCAL_OPENBSD_AMD64_SSH`, `LOCAL_NETBSD_ARM64_SSH`, `LOCAL_NETBSD_AMD64_SSH`, `LOCAL_DRAGONFLY_AMD64_SSH`, and `LOCAL_ILLUMOS_AMD64_SSH` without hardcoded lab details.
- [x] 2.3 Add local smoke targets for FreeBSD/OpenBSD/NetBSD amd64 lab wrappers and DragonFly/illumos candidate gates; keep existing Lima and arm64 local paths working.
- [x] 2.4 Update contributor docs with wrapper variable names and state that real `.local` SSH scripts remain untracked.

## 3. DragonFly Implementation

- [x] 3.1 Add failing fixture-backed DragonFly tests for OS/kernel identity and release parsing.
- [x] 3.2 Implement DragonFly OS facts with `DragonFly` for `os.name`, `os.family`, and `kernel.name`.
- [x] 3.3 Add DragonFly fixture-backed tests and resolvers for networking, memory/swap, processors, uptime/load averages, virtualization, SSH, timezone, and path where native sources are stable.
- [x] 3.4 Add DragonFly fixture-backed tests and resolvers for mountpoints, disks, and partitions where native sources are stable.
- [x] 3.5 Confirm DragonFly not-applicable facts remain absent, including FIPS, SELinux, and unsupported DMI/ZFS/Zpool paths.
- [x] 3.6 Run the DragonFly candidate gate through the untracked amd64 wrapper and record the tested OS release.

## 4. illumos Implementation

- [x] 4.1 Add failing fixture-backed illumos tests for OS/kernel identity and release parsing.
- [x] 4.2 Implement illumos OS facts with `illumos` as `os.family`, `SunOS` as `kernel.name`, and the validated distribution as `os.name`.
- [x] 4.3 Add illumos fixture-backed tests and resolvers for networking, memory/swap, processors, uptime/load averages, virtualization/zones where stable, SSH, timezone, and path.
- [x] 4.4 Add illumos fixture-backed tests and resolvers for mountpoints, disks, partitions, ZFS, and Zpool where native sources are stable.
- [x] 4.5 Confirm Oracle Solaris-specific behavior remains absent and `solaris/amd64` is not built or published.
- [x] 4.6 Run the illumos candidate gate through the untracked amd64 wrapper and record the tested OmniOS release.

## 5. Schema And Documentation

- [x] 5.1 Add `dragonfly` and `illumos` to schema platform vocabulary and schema conformance where required.
- [x] 5.2 Add DragonFly and illumos only to proven `docs/schema/facts.yaml` entries; mark host-dependent facts conditional.
- [x] 5.3 Regenerate `docs/supported-facts/` after schema changes.
- [x] 5.4 Update `CHANGELOG.md`, README/man page support text, and `CONTRIBUTING.md` for candidate/promoted target behavior.

## 6. Promotion And Release Matrix

- [x] 6.1 Add `dragonfly/amd64` and `illumos/amd64` to cross-compile and `make dist` matrices only after native gates and schema conformance pass.
- [x] 6.2 Add or assert DragonFly and illumos CI/native gate statuses after promotion.
- [x] 6.3 Keep `solaris/amd64` and unsupported DragonFly/illumos architectures out of CI, dist, and release artifacts.
- [x] 6.4 Update OpenSpec target lists from candidate to supported release targets after promotion criteria pass.

## 7. Validation

- [x] 7.1 Run `gofmt -w` on edited Go files.
- [x] 7.2 Run `go test ./...`.
- [x] 7.3 Run `go vet ./...`.
- [x] 7.4 Run `make build`.
- [x] 7.5 Run DragonFly and illumos native smoke gates.
- [x] 7.6 Run FreeBSD/OpenBSD/NetBSD amd64 lab smoke paths where wrappers are available.
- [x] 7.7 Run `make dist` and verify `dragonfly/amd64` and `illumos/amd64` artifacts only after promotion tasks are complete.
