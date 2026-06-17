## 1. Parity Audit And Fixtures

- [x] 1.1 Create an OpenBSD and NetBSD parity audit table covering OS, networking, memory, processors, DMI, disks, partitions, mountpoints, uptime, virtualization, SSH, timezone, path, FIPS, SELinux, Augeas, ZFS, and Zpool.
- [x] 1.2 Capture fixture outputs from stock OpenBSD and NetBSD guests for `uname`, `sysctl`, `route`, `ifconfig`, DHCP tooling where present, `mount`, `df -P`, swap tooling, disk tooling, and DMI-related probes.
- [x] 1.3 Mark each audited fact as covered, newly covered, intentionally absent, or blocked, with Puppet Facter behavior and Facts behavior recorded side by side.
- [x] 1.4 Add failing fixture-backed tests for every newly covered OpenBSD and NetBSD parser before implementation.

## 2. Core Fact Implementation

- [x] 2.1 Update OS facts so OpenBSD and NetBSD emit the correct `kernel`, `kernelrelease`, `kernelversion`, `kernelmajversion`, `os.name`, `os.family`, `os.release`, `os.architecture`, and `os.hardware` shapes.
- [x] 2.2 Extend networking so NetBSD uses BSD primary-route parsing and both platforms match Facter-compatible interface, binding, DHCP, hostname, FQDN, and domain behavior.
- [x] 2.3 Add OpenBSD and NetBSD memory and swap resolvers with non-zero system totals and absent values for unsupported fields.
- [x] 2.4 Add OpenBSD and NetBSD processor resolvers for count, model, ISA, topology, and speed when a supported source exists.
- [x] 2.5 Complete OpenBSD DMI and virtualization coverage and add NetBSD DMI/virtualization coverage only where stable audited sources exist.
- [x] 2.6 Fix BSD disk, partition, and mountpoint handling so OpenBSD and NetBSD never fall back to Linux sysfs and only emit facts backed by audited platform sources.
- [x] 2.7 Confirm SSH, timezone, Augeas, uptime, load averages, FIPS absence, SELinux absence, ZFS, and Zpool behavior on both platforms with targeted tests.
- [x] 2.8 Add OpenBSD and NetBSD disk and partition facts from audited `disklabel`/`dkctl` sources.
- [x] 2.9 Add conditional ZFS and zpool root facts from Puppet Facter-compatible upgrade output, omitting OpenBSD and unusable command output.

## 3. Schema And Documentation

- [x] 3.1 Update `docs/schema/facts.yaml` platform vocabulary and per-fact `platforms` entries for OpenBSD and NetBSD.
- [x] 3.2 Update schema conformance expectations so OpenBSD and NetBSD gates fail on undocumented emitted facts and missing non-conditional facts.
- [x] 3.3 Update `CHANGELOG.md` for the new user-visible platform support.
- [x] 3.4 Update contributor or release documentation for the OpenBSD and NetBSD gates and local smoke workflow.
- [x] 3.5 Update schema, release gates, and parity disposition for BSD disks, partitions, and conditional ZFS/zpool support.
- [x] 3.6 Update README, man page, and generated supported-fact pages for the expanded supported release-target matrix.

## 4. CI And Release Tooling

- [x] 4.1 Add OpenBSD and NetBSD release-gate scripts that query structured fact names through the built `facts` CLI and fail on missing or null required facts.
- [x] 4.2 Add local smoke targets that build the matching BSD binary and run the same release-gate scripts in local OpenBSD and NetBSD guests.
- [x] 4.3 Add blocking GitHub Actions jobs for OpenBSD and NetBSD VM tests and release-gate smokes.
- [x] 4.4 Add OpenBSD and NetBSD entries to the cross-compile matrix.
- [x] 4.5 Add OpenBSD and NetBSD release artifact targets to `make dist` and the release workflow.

## 5. Validation

- [x] 5.1 Run `gofmt -w` on edited Go files.
- [x] 5.2 Run `go test ./...`.
- [x] 5.3 Run `go vet ./...`.
- [x] 5.4 Run `go test -race . ./internal/engine ./internal/app` if shared resolver or Session behavior changed.
- [x] 5.5 Run OpenBSD and NetBSD live smoke gates and record the tested OS releases.
- [x] 5.6 Run `make dist` and verify the new BSD artifacts are produced with embedded versions.
- [x] 5.7 Run Ruby Facter 4.10.0 inside the local OpenBSD and NetBSD VMs and record live comparison results.
- [x] 5.8 Re-run Go tests, OpenSpec validation, and local BSD smoke gates after adding disks, partitions, and conditional ZFS/zpool.
