## Context

Facts currently supports Linux, Darwin/macOS, Windows, FreeBSD, OpenBSD, NetBSD, DragonFly BSD, and illumos through a mix of cross-platform Go runtime data, OS-specific native probes, schema metadata, generated supported-facts pages, and release-gate scripts. Plan 9 is not in the schema platform vocabulary, README platform table, release artifact matrix, or platform gate set.

The facts-lab Plan 9 guest is now reachable through the lab SSH path. Local compile-only checks have already shown that the current codebase can build for `GOOS=plan9 GOARCH=amd64`:

- `CGO_ENABLED=0 GOOS=plan9 GOARCH=amd64 go build ./cmd/facts`
- `CGO_ENABLED=0 GOOS=plan9 GOARCH=amd64 go test -exec true ./...`

Plan 9 should not be treated as a Unix variant. Lab probes and Plan 9 manpages show these constraints:

- `uname` is absent.
- `sysctl` is absent.
- `/dev/sysname` provides the host name.
- `$cputype`, `$objtype`, `/dev/cputype`, and `/dev/archctl` provide CPU/architecture data.
- `/dev/swap` exposes total memory, page size, user pages, and swap pages.
- `/dev/sysstat` has one line per processor.
- `/net/ipifc/*/status`, `/net/*/addr`, `/net/iproute`, and `/net/ndb` expose network state.
- Plan 9 namespaces are per-process and do not map cleanly to Unix mountpoint/filesystem facts.
- `/dev/osversion` is the 9P protocol version, not an operating system release.

The implementation should follow the existing Facts pattern: parse native output through small deterministic functions, test those parsers with fixtures, wire them into discovery, then validate the real binary on the native lab guest.

## Goals / Non-Goals

**Goals:**

- Promote Plan 9 from "compiles" to an explicitly supported, lab-validated platform for a narrow initial fact set.
- Keep the Plan 9 fact contract honest: schema entries list `plan9` only when the fact is backed by deterministic tests and native lab validation.
- Add a tracked Plan 9 release gate using `rc`, because the guest is Plan 9 and should not depend on a POSIX shell.
- Validate the real `cmd/facts` binary on the facts-lab Plan 9 guest.
- Add `plan9/amd64` compile/build coverage once the native release gate proves the first supported fact set.
- Document unsupported or intentionally omitted Plan 9 facts so future work does not infer Unix behavior.

**Non-Goals:**

- Do not claim Plan 9 support for `os.release.*`, `kernel.release.*`, or `kernel.version.full` from `/dev/osversion`; that file is not an OS/kernel release.
- Do not implement mountpoint capacity, filesystem inventory, disk inventory, or partition facts in the first slice.
- Do not map Plan 9 `/dev/sysstat` load data onto the existing `load_averages` fact without a separate spec.
- Do not classify virtualization beyond safe, tested signals.
- Do not add Plan 9 cloud, DMI, FIPS, package manager, or container facts.
- Do not add unvalidated Plan 9 architectures to release artifacts just because the Go toolchain lists them.

## Decisions

### 1. Treat Plan 9 as an explicit platform, not a fallback `runtime.GOOS` string

Facts should return canonical names for Plan 9 identity facts instead of letting the default branches emit raw `plan9` values inconsistently.

Initial identity mapping:

| Fact | Value |
| --- | --- |
| `os.name` | `Plan 9` |
| `os.family` | `Plan 9` |
| `kernel.name` | `Plan 9` |
| `networking.hostname` | trimmed contents of `/dev/sysname` |

Alternative considered: leave the default `goos` fallback for `os.family`, `os.name`, and `kernel.name`. That is less code, but it produces lower-quality public output and creates schema/docs churn later if spelling changes.

### 2. Do not invent release/version facts

Plan 9 has `/dev/osversion`, but the researched source says it is the 9P protocol version. The lab value `2000` is not suitable for `os.release` or `kernel.release`.

The first implementation must omit these Plan 9 facts:

- `os.release`
- `os.release.full`
- `os.release.major`
- `kernel.release.full`
- `kernel.release.major`
- `kernel.version.full`

Alternative considered: map `/dev/osversion` to `os.release.full`. That would be easy but misleading.

### 3. Keep Plan 9 probe parsers small and fixture-backed

Each native Plan 9 text format should have a focused parser with unit tests:

| Surface | Native source | Parser responsibility |
| --- | --- | --- |
| Hostname | `/dev/sysname` | trim trailing newline |
| Memory | `/dev/swap` | parse total memory bytes and page size; first gate asserts total only |
| Processors | `/dev/sysstat`, `/dev/cputype`, `/dev/archctl` | count processor lines; extract model/ISA when present |
| Networking | `/net/ipifc/*/status`, `/net/*/addr`, `/net/iproute`, `/net/ndb` | parse IPv4 address, prefix/netmask/network, MAC, primary route |
| Uptime | `uptime` | parse `host up D days, HH:MM:SS` |
| Timezone | Go local time or `date`/`/env/timezone` | return the same `timezone` shape used elsewhere |

Alternative considered: call commands inline and parse directly in fact-building functions. That is shorter initially but harder to test and more brittle under Plan 9's different command surface.

### 4. Use Plan 9 `rc` for the release gate

The release gate should be `tools/plan9-release-gate.rc`. It should run the real `facts` binary through structured fact queries and assert only the first supported fact set.

The gate should avoid POSIX shell syntax and utilities that are not guaranteed on Plan 9. The lab invocation can copy the binary and gate through `facts-lab ssh plan9`, but tracked repository files should not include private lab addresses, SSH keys, or service internals.

Alternative considered: write a POSIX `sh` gate for consistency with BSD gates. That fails the portability goal because Plan 9 is not expected to provide `sh`.

### 5. Gate mandatory facts conservatively

The first Plan 9 gate should require facts that are stable across normal Plan 9 installations and the lab guest:

- `os.name`
- `os.family`
- `os.architecture`
- `os.hardware`
- `kernel.name`
- `networking.hostname`
- `networking.primary`
- `networking.ip`
- `networking.netmask`
- `networking.network`
- `networking.mac`
- `memory.system.total`
- `memory.system.total_bytes`
- `processors.count`
- `processors.isa`
- `processors.models`
- `system_uptime`
- `system_uptime.seconds`
- `timezone`
- `path`

Facts with unresolved semantics, such as network MTU normalization, processor speed, swap usage, virtualization, and load averages, can be emitted later only after the spec and tests define their meaning.

Alternative considered: mirror the BSD release-gate fact set. That overclaims Plan 9 support for Unix-specific surfaces and would create false confidence.

### 6. Add release artifacts only after native validation is repeatable

Plan 9 can be compiled by the Go toolchain for `386`, `amd64`, and `arm`, but the lab validation target is currently `plan9/amd64`. This change should allow `plan9/amd64` promotion after the release gate passes. Other Plan 9 tuples remain out of scope until there is native validation for them.

Alternative considered: add every `go tool dist list` Plan 9 tuple to `DIST_TARGETS`. That is easy but repeats the mistake this project already avoids for unsupported Solaris/AIX-style targets.

### 7. Use nlab as the trusted native validation surface

Human decision on 2026-06-21: nlab is the intended Facts validation host for this work. Copying the built Plan 9 `facts` binary and the tracked Plan 9 gate script to nlab for native validation is allowed. Tracked repository files should still avoid private nlab hostnames, keys, guest addresses, generated credentials, and service internals.

## Risks / Trade-offs

- [Plan 9 output formats vary by installation] -> Keep parser tests based on documented formats plus lab samples, and mark host-state-dependent facts conditional in the schema.
- [Networking MTU semantics are ambiguous] -> Do not require `networking.mtu` in the first gate unless the implementation documents and tests a Plan 9 conversion rule.
- [Plan 9 namespaces do not match Unix mountpoint facts] -> Exclude mountpoints/filesystems/disks from this change.
- [Release gate may depend on lab transport] -> Track only the `rc` gate and configurable/local invocation points; keep nlab details outside git.
- [Schema can overclaim Plan 9 facts] -> Run schema conformance against the native Plan 9 discovery and fail missing non-conditional Plan 9 entries.
- [Plan 9 support increases CI time or flakiness] -> Start with compile coverage plus lab validation; require a passing native gate before release/dist promotion.
- [Future contributors may try to map `/dev/osversion` to OS release] -> Document this as explicitly unsupported in the spec and supported-facts page.

## Migration Plan

1. Add parser tests and Plan 9-specific parser helpers for the first stable fact set.
2. Wire Plan 9 branches into existing engine discovery without changing behavior on other platforms.
3. Add schema platform vocabulary and Plan 9 entries only for emitted, tested facts.
4. Generate `docs/supported-facts/plan9.md`.
5. Add `tools/plan9-release-gate.rc`.
6. Cross-build `cmd/facts` for `plan9/amd64`.
7. Copy the built binary and gate to the facts-lab Plan 9 guest and run the gate through `facts-lab ssh plan9`.
8. Add `plan9/amd64` to compile/build verification and documentation only after the native gate passes.

Rollback is simple: remove `plan9` from schema/docs/build matrices and disable the Plan 9 gate. No persisted user data or migration state is involved.

## Open Questions

- Should the public display spelling be `Plan 9` or `Plan9`? This design recommends `Plan 9`.
- Should Plan 9 `networking.mtu` report raw `maxtu` or normalized Ethernet payload MTU? This change should skip the mandatory gate until decided.
- Should `memory.system.available` and `memory.swap.*` be emitted from `/dev/swap`, or should the first slice expose only total memory?
- Should virtualization report a generic value when virtio PCI devices are present, or remain absent until classification is broader than the lab guest?
