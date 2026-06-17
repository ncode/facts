## Context

Facts currently supports Linux, macOS/Darwin, Windows, and FreeBSD as release targets. The active specs explicitly exclude OpenBSD and NetBSD from release-blocking parity, the CI cross-compile matrix rejects them, the distribution matrix does not publish them, and the schema platform vocabulary only lists the four current targets.

The implementation already has useful pieces: OpenBSD has OS release parsing, DMI sysctl facts, DHCP/route networking, mountpoint parsing through `mount` plus `df -P`, SSH behavior, and virtualization from `hw.product`; NetBSD has kernel-version and load-average handling and OS family naming. The missing work is mostly category-local resolver coverage, schema updates, and real platform gates.

## Goals / Non-Goals

**Goals:**

- Treat OpenBSD and NetBSD as supported release targets with Puppet Facter parity at the structured fact boundary.
- Cover both platforms with deterministic parser tests and live VM smoke gates.
- Add release artifacts and cross-compilation for practical operator use.
- Keep platform logic in the existing category modules and injectable Session seams.

**Non-Goals:**

- No Ruby custom fact DSL support.
- No legacy alias facts or generic BSD-family alias surface.
- No Puppet runtime or Puppet package-version facts.
- No promise that every fact Ruby Facter ever exposed on unrelated BSDs becomes supported.

## Decisions

### Parity Is Fact-by-Fact, Not Generic BSD

OpenBSD and NetBSD will be audited against Puppet Facter as separate operating systems. Shared helper code is fine, but the contract is per OS: `os.name`, `os.family`, release parsing, command availability, nil behavior, and omitted facts must match the specific platform.

Alternative considered: add a broad "BSD" path and let both platforms share FreeBSD behavior. That would be faster but risks claiming FreeBSD-specific `geom`, `kenv`, or jail behavior on systems that do not have those interfaces.

### Keep Resolver Code Category-Owned

Implementation should extend the existing category files (`os.go`, `networking.go`, `memory.go`, `processors.go`, `dmi.go`, `disks.go`, `virtual.go`, `ssh.go`, `uptime.go`) using `goos` parameters and parse helpers. New helper files must avoid reserved GOOS suffixes unless they are genuinely build-tagged syscall code, as with `statfs_*`.

Alternative considered: create `openbsd.go` and `netbsd.go`. That fights ADR-0010 and hides platform parsing from normal cross-platform tests.

### Start From the Existing Partial BSD Surface

The implementation should reuse what already works:

- OS: keep OpenBSD `uname -r` parsing and add NetBSD release-map parsing instead of returning a raw string.
- Networking: share POSIX interface enumeration; route primary interface through `route -n get default`; keep OpenBSD `dhcpleasectl`; add NetBSD DHCP only if the parity audit identifies a stable Facter source.
- Memory/swap: add sysctl/swap command parsers for both platforms rather than returning zero-valued memory facts.
- Processors: add sysctl-backed logical count and model parsing; only emit speed when a real source exists.
- DMI/virtualization: keep OpenBSD `hw.*` sysctls; audit NetBSD `machdep.dmi.*` availability and omit `dmi` when unresolved.
- Mountpoints/disks/partitions: stop falling back to Linux sysfs defaults on BSDs; use per-OS `mount`, `df`, `sysctl`, `disklabel`, and NetBSD `dkctl` sources where audited, or omit facts when no stable source exists.
- ZFS/zpool: parse Puppet Facter-compatible `zfs upgrade -v` and `zpool upgrade -v` output only on supported BSDs where the commands return version data; omit facts for OpenBSD and for installed-but-unusable tools.

Alternative considered: implement only the release-gate fact list first. That would make CI green but leave schema and parity ambiguous, so the first implementation pass should include explicit audit dispositions for unsupported or intentionally absent facts.

### CI Uses Hosted BSD VMs With a Local Escape Hatch

GitHub Actions has no native OpenBSD or NetBSD runner. The primary CI path should mirror the current FreeBSD pattern: run a hosted BSD VM action from an Ubuntu runner, build the real CLI in the guest, run platform-sensitive Go tests, and execute a release-gate script. `vmactions/openbsd-vm@v1` and `vmactions/netbsd-vm@v1` are available and QEMU-backed; if they become unreliable, the workflow may assert an equivalent external BSD CI status.

Local validation should use the same release-gate scripts as CI. The existing `.local/bsd-vms` QEMU guests are useful for development but must stay ignored and must not be the only supported validation path.

Alternative considered: only cross-compile OpenBSD and NetBSD. Cross-compilation catches build errors but cannot validate sysctl, routing, mount, memory, DMI, or schema behavior, so it is not enough for release support.

## Risks / Trade-offs

- VM action flakiness -> Keep release-gate scripts small, reuse the same scripts locally, and allow an equivalent asserted external CI fallback.
- NetBSD command differences across architecture or release line -> Pin the initial live gate to one current release, cover parsers with fixtures, and keep unsupported values absent rather than guessed.
- Overclaiming schema support -> Run schema conformance in both live gates and mark host-dependent facts conditional.
- Large fact-surface change -> Require a parity audit table before implementation is marked complete, including intentional deviations and absent facts.

## Migration Plan

1. Add failing fixture-backed tests for the OpenBSD and NetBSD fact categories.
2. Implement missing resolvers and update schema entries.
3. Add release-gate scripts and wire them into GitHub Actions plus local make targets.
4. Extend cross-compile and release artifact matrices.
5. Run `go test ./...`, `go vet ./...`, the platform VM smoke gates, and `make dist`.

Rollback is straightforward: remove the new platforms from CI/distribution matrices and revert the spec/schema additions. Existing Linux, macOS, Windows, and FreeBSD behavior must not depend on the new platform paths.

## Open Questions

- Which NetBSD DMI source should be treated as stable for release support, if any, after checking Puppet Facter and live guests?
- Should NetBSD DHCP be supported in the first pass, or documented as absent if Puppet Facter cannot resolve it reliably on stock NetBSD?
- Should OpenBSD/NetBSD release artifacts start with `amd64` only, or also ship `arm64` once CI proves the live gates can validate an arm64 guest?
