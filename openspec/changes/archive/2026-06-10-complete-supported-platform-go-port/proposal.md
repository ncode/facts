## Why

The Go port is green and broad, but it is not yet release-complete: the remaining work is a systematic parity audit and closure pass against the Ruby behavior for supported platforms. This change defines the missing completion scope so future TDD slices converge on a clear release gate instead of continuing as open-ended compatibility fixes.

## What Changes

- Complete the Ruby-to-Go port for the supported platform scope: Linux, macOS/Darwin, Windows, and FreeBSD.
- Audit every in-scope Ruby fact, resolver, framework, custom fact, external fact, cache, config, formatter, CLI, and public API spec against the current Go implementation.
- Add Go parity tests for every supported-platform Ruby behavior that is missing, only unverified, or covered only indirectly.
- Implement the minimal Go changes needed to pass those parity tests while keeping Ruby sources in place until final cleanup is explicitly approved.
- Add a maintained parity ledger that records audited Ruby specs, Go coverage, known intentional deviations, and out-of-scope Solaris/AIX/BSD-family exclusions.
- Add platform validation gates for Linux, macOS/Darwin, Windows, and FreeBSD, including focused benchmarks for hot fact collection, parsing, custom fact, cache, and formatter paths when changed.
- Use Lima for repeatable FreeBSD validation via the existing FreeBSD smoke/build workflow, and extend that workflow when additional FreeBSD fact parity requires live OS checks.
- Do not expand the release scope to Solaris, AIX, OpenBSD, NetBSD, or DragonFly; existing code for those platforms remains historical or shared-parser reference unless separately approved.

## Capabilities

### New Capabilities

- `go-port-framework-parity`: Public API, CLI, formatting, query, config, cache, custom fact, external fact, logging, and blocklist behavior required for Ruby-compatible Facter operation.
- `go-port-supported-platform-facts`: Linux, macOS/Darwin, Windows, and FreeBSD core fact and resolver parity, including structured facts, legacy aliases, diagnostics, platform-specific fallbacks, virtualization, cloud metadata, networking, memory, processors, DMI, OS release, uptime, SSH, mountpoints, filesystems, disks, partitions, and distro/release-specific behavior.
- `go-port-completion-verification`: The parity ledger, supported-platform validation matrix, benchmark policy, and release-completion gates for declaring the Go port complete.

### Modified Capabilities

- None. This repository does not yet have checked-in OpenSpec capability specs; this change introduces the port-completion contract.

## Impact

- Affected Go code: `facter.go`, `cmd/facter`, `internal/app`, `internal/cli`, and `internal/facter`.
- Affected compatibility sources: `spec/`, `spec_integration/` when present, `lib/facter/`, `lib/facter/custom_facts/`, `PORTING.md`, and `docs/MIGRATION.md`.
- Affected verification: `go test ./...`, `go test -race ./...`, focused platform smoke runs, and repeated benchmarks for changed hot paths.
- Current analysis baseline: 614 in-scope Ruby spec files after including FreeBSD and excluding AIX/Solaris/OpenBSD/other BSD-family platform trees; 181 are explicitly referenced by the current migration log, leaving 433 in-scope specs that still need explicit audit disposition even when Go already covers them indirectly.
