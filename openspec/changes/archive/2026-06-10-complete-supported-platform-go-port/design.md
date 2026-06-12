## Context

The repository contains a Go port layered beside the Ruby implementation. The current Go surface includes the public package (`facter.go`), CLI (`cmd/facter`), app wiring (`internal/app`, `internal/cli`), and fact engine (`internal/facter`). Current local verification before this proposal: `go test ./... -count=1` passes across 5 Go packages, `make race` passes outside the sandbox, and statement coverage is 83.0%.

The port is not release-complete because supported-platform parity has not been exhaustively audited. With FreeBSD included, the in-scope Ruby compatibility set is:

- Linux, macOS/Darwin, Windows, and FreeBSD fact and resolver specs.
- Shared framework behavior, custom facts, external facts, config, cache, query, formatting, public API, and CLI behavior.
- Distro-specific Linux family behavior that feeds supported Linux facts.

The in-scope Ruby spec inventory now contains 614 spec files after excluding AIX, Solaris, OpenBSD, NetBSD, DragonFly, and generic BSD-family platform scopes that do not have a repeatable release validation target. The current migration log explicitly references 181 of those spec files. The remaining 433 specs are not automatically missing implementation; they are missing an explicit audit disposition.

Largest unaudited or unlogged groups from the current scan:

- Linux facts: 113 files.
- macOS facts: 104 files.
- FreeBSD facts/resolvers: 78 fact files and 2 resolver files not explicitly logged.
- Windows facts: 72 files.
- Distro/release facts: Amazon, Debian, Devuan, OpenWrt, RHEL, SLES, Ubuntu.
- Shared resolver/framework areas: EC2/Azure/FIPS/timezone/path/SSH/load average/uptime helpers, base resolver behavior, and selected utility specs.

Current Go implementation already covers many of these domains directly or indirectly: public API, CLI, formatting, config, cache, custom facts, external facts, Linux distro/release, Linux networking/virtualization/memory/processor paths, macOS system profiler parsing, Windows WMI-style DMI/memory/processor/kernel/uptime/product release paths, FreeBSD DMI/memory/mountpoint/processor/partition/disk/virtualization parsing, and cloud metadata clients. The completion work is therefore an audit-and-close effort, not a greenfield rewrite.

## Goals / Non-Goals

**Goals:**

- Treat Linux, macOS/Darwin, Windows, and FreeBSD as release targets for the Go port.
- Produce and maintain a parity ledger mapping each in-scope Ruby spec file to one of: covered by existing Go test, newly covered by Go test, intentional documented deviation, or blocked by missing validation fixture.
- Convert every real missing supported-platform behavior into a focused failing Go test before implementation.
- Keep Go implementation changes narrow and behavior-driven.
- Use Lima as the repeatable FreeBSD validation path, starting from the existing `lima-freebsd-smoke` target and extending it for live FreeBSD facts as needed.
- Preserve local baseline gates: `go test ./...`, `go test -race ./...`, gofmt, `git diff --check`, and focused benchmarks when hot paths change.

**Non-Goals:**

- Do not delete the Ruby implementation as part of this change.
- Do not add Solaris, AIX, OpenBSD, NetBSD, or DragonFly to the release target set.
- Do not implement unsupported platform behavior solely because historical Go code or Ruby specs exist.
- Do not broaden compatibility beyond Ruby behavior unless a deviation is explicitly documented and approved.
- Do not require every Ruby spec to have a one-to-one Go test when a smaller integration-style Go test proves the same public behavior.

## Decisions

1. FreeBSD is in scope because the repo already has a repeatable Lima path.

   The Makefile includes `lima-build-freebsd-binary`, `lima-freebsd-start`, and `lima-freebsd-smoke`. That gives the port a practical way to validate a FreeBSD binary on an actual FreeBSD guest. OpenBSD and other BSDs remain out of scope until they have equivalent validation.

2. The parity ledger is the central completion artifact.

   A raw count of Go tests or Ruby spec references is not enough. One Go integration test can cover many Ruby leaf specs, while some Ruby specs encode platform details that are deliberately out of scope. The ledger must record the decision per Ruby spec so the release decision can be audited later.

3. Public behavior is the preferred test boundary.

   New Go tests should use `facter` package APIs, `internal/app.Run`, `cmd/facter`, or `internal/facter` fact constructors depending on the behavior. Private parser tests are acceptable when they are the narrowest way to pin resolver behavior, but the audit should prefer public fact output where practical.

4. Platform-specific live checks must stay separate from deterministic unit tests.

   Unit tests should use fixtures and injectable command/file seams. Live validation should run through macOS host tests, Linux Lima/Docker workloads, Windows CI or documented manual runner, and FreeBSD Lima smoke/extended smoke targets. This keeps `go test ./...` stable on any developer machine.

5. Benchmarks are evidence, not ceremony.

   Benchmarks are required when changing hot collection, formatting, cache, custom fact, external fact, or parser paths. Cold compatibility diagnostics do not need benchmarks unless they introduce repeated work in normal fact collection.

## Risks / Trade-offs

- FreeBSD Lima startup and cross-compile workflows can be slower than local unit tests -> Keep FreeBSD live checks in explicit `make lima-freebsd-*` targets and avoid making them mandatory for every local edit.
- The parity ledger can become stale -> Require every task slice to update the ledger and migration log in the same change as the Go test/implementation.
- Ruby specs can test internals that do not map cleanly to Go -> Record the public Go behavior that covers the same contract, or document an intentional deviation.
- Supported-platform scope can creep through shared BSD/AIX/Solaris helpers -> Scope decisions must be made at the Ruby spec path level and recorded before implementation work starts.
- Some Windows behavior cannot be validated from this macOS workspace -> Keep deterministic WMI/registry parser tests in Go, and require a Windows CI/manual smoke gate before release completion.

## Migration Plan

1. Update the porting docs to list FreeBSD as a supported release target and to distinguish OpenBSD/other BSD code as historical until validated.
2. Generate the parity ledger from the Ruby spec tree and seed it with the current explicit migration-log references.
3. Audit framework/API/CLI/custom/external/cache/config/formatter behavior first because these affect all platforms.
4. Audit platform facts by domain: OS/release, identity, networking, memory, processors, DMI/hardware, disks/partitions/filesystems/mountpoints, uptime/load averages, virtualization/hypervisors, cloud, SSH, timezone, and legacy aliases.
5. For each unsupported behavior, add a failing Go test, implement minimal code, run focused tests, then run package/full gates as risk requires.
6. Extend Lima FreeBSD smoke coverage from `os.name kernel virtual` to the finalized FreeBSD release gate fact set.
7. Declare completion only when every in-scope Ruby spec has ledger disposition and all verification gates pass.

## Open Questions

- Should OpenBSD be promoted later if a repeatable Lima or CI validation target is added?
- What is the required Windows validation environment for final release: GitHub Actions Windows only, local manual runner, or both?
- Should FreeBSD release completion require only smoke facts, or a broader fixture-backed acceptance set for DMI, memory, mountpoints, disks, partitions, processors, and virtualization?
