## 1. Scope And Ledger Setup

- [x] 1.1 Update `PORTING.md` and `docs/MIGRATION.md` to list Linux, macOS/Darwin, Windows, and FreeBSD as release targets, with Solaris/AIX/OpenBSD/other BSD-family paths explicitly out of scope until validated.
- [x] 1.2 Add a generated parity ledger artifact for every in-scope Ruby spec file, including Linux, macOS/Darwin, Windows, FreeBSD, shared framework, custom fact, external fact, and in-scope Linux distro-family specs.
- [x] 1.3 Seed the ledger with current migration-log references and mark them as covered only after confirming the named Go tests still exist and pass.
- [x] 1.4 Add a script or documented command that regenerates the Ruby spec inventory and highlights unassigned ledger entries.
- [x] 1.5 Add a rule to every future parity slice: update the ledger and migration log in the same change as the Go test/implementation.

## 2. Shared Framework Audit

- [x] 2.1 Audit `spec_integration/facter_spec.rb`, `facter_to_hash_spec.rb`, `facter_resolve_spec.rb`, and `facter_block_legacy_spec.rb` against `facter.go`, `internal/app`, and `cmd/facter` coverage.
- [x] 2.2 Add or confirm Go tests for public API query behavior, nested lookup, Ruby-symbol-style fact names, custom/core/external precedence, nil handling, `Flush`, `Reset`, `Search`, `SearchExternal`, and compatibility option state.
- [x] 2.3 Add or confirm CLI tests for output formats, strict missing facts, concatenated flags, config flags, legacy flags, no-ruby/no-custom/no-external flags, logging flags, and exit status.
- [x] 2.4 Audit formatter specs and close any JSON/YAML/HOCON/legacy text gaps around arrays, maps, nils, dotted fact names, Windows paths, IPv6 strings, and scalar quoting.
- [x] 2.5 Audit query and collection collision behavior, including force-dot-resolution and legacy wildcard matching.

## 3. Custom And External Fact Audit

- [x] 3.1 Audit custom fact parser, loader, confine, suitable, resolvable, aggregate, directed graph, execution, and normalization specs against `internal/facter/custom.go`.
- [x] 3.2 Add or confirm Go tests for custom fact setcode forms, aggregate options/chunks, weights, execution environment, command lookup, timeouts, logger behavior, raised resolvers, missing confine facts, and continued loading after failures.
- [x] 3.3 Audit external fact behavior for environment variables, text/JSON/YAML files, executable facts, PowerShell facts, null bytes, invalid UTF-8, parser warnings, recursion guards, timeouts, and precedence.
- [x] 3.4 Add or confirm Go tests for default custom/external search paths on Linux, macOS/Darwin, Windows, and FreeBSD.
- [x] 3.5 Run focused benchmarks for custom and external fact loading after any parser or loader change.

## 4. Config, Cache, And Fact Group Audit

- [x] 4.1 Audit Ruby config reader, option store, blocklist, and fact group behavior against `internal/facter/config.go`, `groups.go`, and `internal/app`.
- [x] 4.2 Add or confirm Go tests for readable/unreadable/invalid config, repeated entries, bare entries, quoted values, default paths, sequential/log option handling, and CLI/config precedence.
- [x] 4.3 Audit cache manager behavior against `internal/facter/cache.go`, including TTL parsing, configured groups, invalid JSON, non-object JSON, permission errors, stale cache, external fact custom-group rejection, and missing searched keys.
- [x] 4.4 Add or confirm Go tests for cache path defaults on Linux, macOS/Darwin, Windows, and FreeBSD.
- [x] 4.5 Run focused cache/config benchmarks after hot-path cache or config parser changes.

## 5. Linux Fact Audit

- [x] 5.1 Audit Linux OS/release/distro behavior across generic Linux and distro-family specs: Amazon, Debian, Devuan, OpenWrt, RHEL, SLES, Ubuntu, Arch, Alpine, Azure Linux, Gentoo, Linux Mint, Mageia, Mariner, Meego, OEL/OL, OVS, Photon, and Slackware.
- [x] 5.2 Add or confirm Go tests for Linux networking and interface aliases: primary interface, IPv4/IPv6, netmask/network/scope/MTU/MAC, DHCP, bonding, NetworkManager/systemd/dhclient/dhcpcd leases, and invalid interface names.
- [x] 5.3 Add or confirm Go tests for Linux memory, swap, load averages, uptime including Docker PID 1 fallback, mountpoints, filesystems, disks, partitions, and root-device resolution.
- [x] 5.4 Add or confirm Go tests for Linux processors, DMI, SSH, SELinux, FIPS, path, timezone, Ruby, Augeas, ZFS, Zpool, and all legacy aliases.
- [x] 5.5 Add or confirm Go tests for Linux virtualization, containers, Xen, OpenVZ, VMware, KVM, VirtualBox, virt-what, cloud provider facts, EC2, GCE, and Azure metadata.
- [x] 5.6 Run Linux Docker/Lima distro smoke coverage and focused benchmarks for changed hot Linux paths.

## 6. macOS/Darwin Fact Audit

- [x] 6.1 Audit macOS OS/release, product/build/version, architecture, family, hardware, kernel, and legacy alias specs.
- [x] 6.2 Add or confirm Go tests for macOS networking and interface aliases: primary interface, IPv4/IPv6, netmask/network/scope/MTU/MAC, DHCP, hostname/FQDN/domain fallbacks, and route-derived primary interface behavior.
- [x] 6.3 Add or confirm Go tests for macOS memory, swap, load averages, uptime, mountpoints, filesystems, processors, DMI, SSH, path, timezone, Ruby, and Augeas.
- [x] 6.4 Audit all macOS system profiler specs and add or confirm Go tests for hardware, software, and ethernet fields, including nil behavior and legacy aliases.
- [x] 6.5 Add or confirm Go tests for macOS virtualization behavior and run macOS host CLI smoke checks for the release-gate fact set.
- [x] 6.6 Run focused benchmarks for changed macOS parser or collection paths.

## 7. Windows Fact Audit

- [x] 7.1 Audit Windows OS/release/product/system32, kernel, architecture, hardware, FIPS, timezone, path, Ruby, and legacy alias specs.
- [x] 7.2 Add or confirm Go tests for Windows networking and interface aliases: primary interface, IPv4/IPv6, netmask/network/scope/MTU/MAC, DHCP, DNS domain, loopback handling, addressless interfaces, and invalid friendly names.
- [x] 7.3 Add or confirm Go tests for Windows memory, processors, DMI, identity, uptime, SSH, and diagnostic messages for empty or malformed WMI/registry data.
- [x] 7.4 Add or confirm Go tests for Windows virtualization, hypervisors, NetKVM, VirtualBox, VMware, Hyper-V, Xen, EC2, GCE, Azure, and cloud provider facts.
- [x] 7.5 Define and run the Windows CI/manual validation gate for the release-gate fact set.
- [x] 7.6 Run focused benchmarks only for changed hot Windows parser or collection paths.

## 8. FreeBSD Fact Audit

- [x] 8.1 Audit all FreeBSD Ruby fact and resolver specs and add them to the parity ledger with explicit disposition.
- [x] 8.2 Add or confirm Go tests for FreeBSD OS/release, kernel, identity, path, timezone, Ruby, Augeas, and legacy aliases.
- [x] 8.3 Add or confirm Go tests for FreeBSD networking and interface aliases, including IPv4/IPv6, netmask/network/scope/MTU/MAC, DHCP, primary interface, and route-derived behavior.
- [x] 8.4 Add or confirm Go tests for FreeBSD memory, swap, processors, DMI, disks, partitions, filesystems, mountpoints, uptime, load averages, SSH, and virtualization.
- [x] 8.5 Extend `make lima-freebsd-smoke` or add a companion target so the FreeBSD Lima guest verifies the release-gate fact set beyond `os.name kernel virtual`.
- [x] 8.6 Run the FreeBSD Lima validation target and record the result in the migration log.

## 9. Verification Gates

- [x] 9.1 Run `go test ./... -count=1` and record the result.
- [x] 9.2 Run `go test -race ./...` outside restrictive sandboxes when local listener tests require it, and record the result.
- [x] 9.3 Run gofmt checks, `go vet ./...`, and `git diff --check`.
- [x] 9.4 Run Linux distro/container smoke coverage through existing Lima/Docker targets.
- [x] 9.5 Run macOS host smoke coverage for the release-gate fact set.
- [x] 9.6 Run Windows validation through CI or an approved Windows runner.
- [x] 9.7 Run FreeBSD Lima validation through the extended FreeBSD target.
- [x] 9.8 Run repeated focused benchmarks for every changed hot path and record representative results.

## 10. Completion And Handoff

- [x] 10.1 Confirm every in-scope Ruby spec has a final ledger disposition with a covering Go test, intentional deviation, or non-blocking rationale.
- [x] 10.2 Confirm all OpenSpec requirements in this change are satisfied.
- [x] 10.3 Update `docs/MIGRATION.md` with the final completion summary, verification matrix, and remaining non-release blockers.
- [x] 10.4 Leave Ruby implementation cleanup as a separate explicitly approved change.
- [x] 10.5 Archive or mark this OpenSpec change complete only after all release gates pass.
