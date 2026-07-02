## Why

A deletion-test review of `internal/engine` (seven deep dives, each adversarially red-teamed) found the engine reaching the same concern through two seams, or keeping entrances nothing uses:

- **Host I/O is reachable two ways in resolvers.** ADR-0010 deferred collapsing the `commandRunner`/`fileReader` parameter threading onto the Session; today `FromRoot`/`WithHost`/`WithReader`/`WithRunner`/`ForPlatform` variant families duplicate the Session host seam (networking alone carries 10 variants), `core.go`'s reader helpers silently default to a raw `osHost{}`, and raw `os.ReadDir`/`os.Stat` calls in networking, processors, virtual, and ec2 bypass every seam. The drift this causes is already real: `probeLinuxDistro` ignores `s.goos()`.
- **Platform and environment dispatch bypass the Session.** Category assemblies read `runtime.GOOS` directly (~30 sites across os, dmi, identity, ssh, selinux, fips, timezone, uptime, and `buildCoreFacts`) and `os.Getenv` at four sites, so fake-host tests cannot drive the windows or plan9 assembly paths and `core_gating_test.go` must skip ssh and fips off-host.
- **The host-virtualization gather runs three times per Linux discovery.** `virtual`, `hypervisors.*`, and the uptime container gate each re-run dmidecode/virt-what/vmware/lspci (twice on Windows for the wmic/reg gather). The Session memo pattern (`cachedDMI`) exists and is not applied to the most expensive repeated probe.
- **Dead and test-only entrances linger.** `detector.go` (six symbols, zero production callers), `query.go`'s Select delegates, the `LoadExternalFacts` facade the archived 2026-06-17 design already marked for deletion, dead `gceFacts`, the `EnvironmentDisabledFacts`/`DisabledFactsForFiltering` crutch exports, dead `filehelper.go`, and four bare `Format*` functions whose only production caller re-derives formatter precedence by hand.
- **Cloud metadata transport is copied four times.** ec2, gce, and az each hand-roll the same proxy-less client, 200-required check, and 1MB-capped read; the shared transport invariants are asserted nowhere because the code exists in copies.
- **The CLI and Discover mirror policy that already has an owner.** `internal/app` re-implements the engine's disabled-set union for the version fast path, and `Engine.Discover` duplicates two near-identical external-loader arms driven by the loader's own mode field.

All of this is locality and testability debt, paid down before more facts land on top of it.

## What Changes

- **Collapse the resolver double host-I/O seam onto the Session** (the ADR-0010 deferred follow-on, downscoped): delete the variant families (networking 10→4, processors triple→1, virtual/xen pairs, dmi's five `ForPlatform` pairs with `currentWindowsDMI` folded in or explicitly excluded), make `core.go`'s variadic-optional reader helpers take required readers, close the raw `os.*` leaks, merge mixed-seam `current*` functions and trivial `probe*`/`current*` repackaging pairs (keeping `probe*` as the Session memo-filler layer), extend `fakeHostOS` additively, migrate ~150–200 test closures and ~35 TempDir fixtures to the fake host, delete dead `filehelper.go`, and add a grep-gate test freezing "no raw host I/O in resolvers".
- **Route category assembly through the Session goos/env seam**: bind `goos := s.goos()` in every category assembly and impure probe call site; add a pure `envValue` helper (case-insensitive only on windows) fed by `hostOS.environ()` for the SystemRoot/programdata/PROCESSOR_ARCHITEW6432/PATH reads; remove the ssh and fips skips from `core_gating_test.go`. The whole `runtime.GOOS`→`s.goos()` class is one intentional latent-drift fix, production-identical on every real host.
- **Memoize the host-virtualization gather on the Session** (cachedDMI pattern, ADR-0005-compliant): linux and windows gather memos routed to the five gather call sites; classification ladders stay pure and untouched; Linux spawns the gather commands once instead of three times.
- **Delete the test-only entrances**: `detector.go` (+ tests) and `query.go`'s `Select`/`SelectWithDottedFacts` delegates, moving `factMatchesQuery` beside its sole caller in `projection.go` and retargeting the delegate tests at the production `NewProjection(...).Select` entrance.
- **Concentrate the cloud metadata fetch in one transport helper** (`metadatahttp.go`): shared client constructor and fetch (proxy-less, 200-required, 1MB cap, fail-closed) under provider adapters that keep their headers, token/flavor handling, parse, gates, and per-provider timeouts; delete dead `gceFacts` and retarget its six referencing tests; land the first-ever transport-invariant tests once.
- **Collapse Discover's duplicated external-loader arms** into one loader construction with a single mode-conditional error branch (CLI fail-fast byte-identical, library accumulate); add Discover-level pinning tests for both policies; delete the test-only `LoadExternalFacts`/`LoadExternalFactsWithBlocklist` facade.
- **Feed the version fast path from the engine's seams**: export one pure `DisabledUnion` replacing `internal/app`'s admitted mirror of the engine union; delete the `EnvironmentDisabledFacts`/`DisabledFactsForFiltering` crutch exports; route `writeVersionQuery` through `BuildFormatter`, demoting the four bare `Format*` functions to an in-package test helper. Fast-path decision and formatter selection stay CLI-owned per the deepen-discovery-input-surface design.
- **Preserve everything user-visible.** Public `facts` API, CLI flags, output contract, input contract, cache behavior, and diagnostics are unchanged. No breaking change is intended.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `facts-library-api`: the diagnostics requirement's enumerated source list drops OS-hierarchy detection (the detector module is deleted; it had no production callers).
- `go-port-supported-platform-facts`: resolver host probes, platform dispatch, and env lookup must flow through the Session host seam as the only resolver host-I/O path (structurally enforced), and the host-virtualization gather must probe once per discovery with all consumers reading the same memoized input.
- `facts-cli-option-contract`: the version fast path's disabled-set union and formatter selection must feed the engine's exported seams (`DisabledUnion`, `BuildFormatter`) instead of an `internal/app` mirror, while the fast-path decision itself stays CLI-owned.

## Impact

- **Code**: `internal/engine` (session, core, networking, os, dmi, disks remnants, processors, memory, uptime, virtual, xen, ssh, selinux, fips, identity, augeas, ec2, gce, az, external, engine, query→projection, detector deleted, filehelper deleted, new metadatahttp) and `internal/app` (fast-path wiring); the test diff dominates ~4:1 (closure/fixture migration to `fakeHostOS`).
- **Behavior**: Behavior-preserving refactor. Any observed public API, CLI output/status, input source precedence, diagnostics, or cache difference is a bug unless explicitly captured in a follow-up change. Production host behavior is bit-identical by construction: `osHost` methods are the same `os.*` calls the leaks made directly, and `s.goos()` equals `runtime.GOOS` on every real host.
- **Sequencing**: lands after `deepen-discovery-input-surface` and `fix-linux-dhcp-lease-interface-match` archive; the `core.go`/`core_gating_test.go`/fast-path work lands after `add-fact-disable-controls` archives or rebases against its deltas.
- **Docs/schema**: no fact schema change; one consolidated CHANGELOG internal-refactor entry; ADR-0010's deferred-follow-on note updated to record the collapse as done.
