## Context

Facts currently treats Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, and NetBSD as supported release targets. The active specs still exclude DragonFly, Solaris, and AIX from the release-blocking matrix, while Go supports `dragonfly/amd64`, `illumos/amd64`, and `solaris/amd64` as distinct targets.

The lab has native DragonFly BSD and OmniOS guests. OmniOS validates illumos behavior, not Oracle Solaris behavior. `/Users/ncode/Devel/facter` remains useful for shared SunOS-compatible behavior, but DragonFly has no useful upstream Ruby Facter target.

## Goals / Non-Goals

**Goals:**

- Promote DragonFly and illumos through a candidate-target lifecycle before they become supported release targets.
- Add DragonFly first, then illumos, with strict schema conformance and native validation.
- Ship only Go-supported release tuples: `dragonfly/amd64` and `illumos/amd64`.
- Keep lab connection details out of git by using configurable `.local` SSH wrappers.
- Add amd64 lab-wrapper paths for FreeBSD, OpenBSD, and NetBSD without removing existing Lima or arm64 local paths.

**Non-Goals:**

- No Oracle Solaris support in this change.
- No `solaris/amd64` cross-compile or release artifact until Oracle Solaris has its own validation host.
- No hardcoded lab hostnames, addresses, keys, or commands in tracked files.
- No Lima removal; that is a later cleanup after lab paths are proven.
- No Plan 9, AIX, Android, iOS, js/wasm, wasip1, or broad "all Go OSes" support.

## Decisions

### Candidate lifecycle before support

DragonFly and illumos start as candidate release targets. A target becomes supported only after its release-gate script, schema conformance, native smoke, release artifact, and documented platform matrix are all in place.

Alternative considered: promote both immediately. Rejected because the schema would overclaim before the native fact set is proven.

### Platform identity is explicit

DragonFly reports `DragonFly` for `os.name`, `os.family`, and `kernel.name`. illumos uses `illumos` as the schema/platform key, reports the validated distribution as `os.name` (`OmniOS` in the lab), uses `illumos` for `os.family`, and uses `SunOS` for `kernel.name`.

Alternative considered: call OmniOS "Solaris". Rejected because Go separates `illumos` and `solaris`, and Oracle Solaris remains a separate future candidate target.

### Facts-native extensions are allowed

Ruby Facter parity guides shape when a comparable fact exists, but it is not the ceiling. DragonFly and illumos may expose accurate Facts-native extensions when the source is stable, the canonical spelling is schema-documented, and fixture plus native validation covers it.

Alternative considered: require byte parity only. Rejected because DragonFly lacks useful Facter coverage and Facts already uses schema-owned canonical behavior where Ruby Facter is incomplete or inconsistent.

### Resolver code stays category-owned

Implementation extends existing category modules (`os.go`, `networking.go`, `memory.go`, `processors.go`, `disks.go`, `dmi.go`, `uptime.go`, `virtual.go`, `ssh.go`) through `goos` parameters and injectable seams. Parser/helper files must not use reserved GOOS suffixes unless they are genuinely build-tagged syscall code.

Alternative considered: add `dragonfly.go` and `illumos.go` catch-all files. Rejected because ADR-0010 keeps resolver logic category-owned and cross-platform testable.

### Native validation uses tracked gates and untracked wrappers

Tracked files define release-gate scripts and configurable wrapper variables only. Real SSH details stay under ignored `.local/` scripts. The wrapper variable set is:

- `LOCAL_FREEBSD_AMD64_SSH`
- `LOCAL_OPENBSD_ARM64_SSH`
- `LOCAL_OPENBSD_AMD64_SSH`
- `LOCAL_NETBSD_ARM64_SSH`
- `LOCAL_NETBSD_AMD64_SSH`
- `LOCAL_DRAGONFLY_AMD64_SSH`
- `LOCAL_ILLUMOS_AMD64_SSH`

Alternative considered: call the lab helper directly from `Makefile`. Rejected because it would leak private lab topology and make the repo less portable.

### Release-gate facts are proven, not aspirational

DragonFly and illumos gates should require the common fact set only after the target emits it correctly: OS/kernel identity, networking, memory, processors, uptime/load averages, virtualization, mountpoints, and stable disk/partition facts where native sources are proven. The gate checks presence and shape, not whether the lab clock or VM metadata looks sane.

Alternative considered: copy OpenBSD/NetBSD required facts wholesale. Rejected because illumos and DragonFly have different native sources and failure modes.

## Risks / Trade-offs

- **Schema overclaims support** -> Add platform keys only to facts proven by tests and native gates; mark host-dependent facts conditional.
- **Lab details leak into git** -> Track only wrapper variable names and scripts that run on the guest.
- **Guest clocks or VM metadata are odd** -> Report native sources as-is; gates verify shape/non-null, not "reasonable" dates.
- **illumos diverges from Oracle Solaris** -> Keep Oracle Solaris out until it has a repeatable host.
- **Lima and lab paths overlap** -> Keep both until a separate cleanup removes Lima.

## Migration Plan

1. Add candidate-target specs, docs, and wrapper-variable guidance.
2. Add DragonFly release-gate script, tests, schema entries, and implementation.
3. Add illumos release-gate script, tests, schema entries, and implementation.
4. Add amd64 lab smoke targets for FreeBSD, OpenBSD, NetBSD, DragonFly, and illumos using `.local` wrappers.
5. Promote DragonFly/illumos to supported release targets and add `make dist` artifacts only after native gates and schema conformance pass.

Rollback: remove DragonFly/illumos from schema/platform matrices and artifact targets, leaving existing Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, and NetBSD behavior unchanged.
