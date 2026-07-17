## Context

The core-fact descriptor table records emitted roots and four gating classes, but production only gates the `standalone` class. Multi-output resolvers therefore still run when every output is disabled, and `newCoreFactBuild` acquires DMI eagerly even when neither DMI nor GCE output is kept. This conflicts with ADR-0015 and the existing `fact-disable-controls` requirement.

Engine and CLI tests also replace eight package variables for default paths/directories, cache I/O failures, and external-fact limits. Production does not mutate them and the current race suite is clean, but the variables still contradict the immutable-Engine/no-global-state contract and prevent isolated parallel tests. Three resolver helpers and two Plan 9 networking seams are reachable only from tests.

The audit rejected a broad networking boundary refactor. `networkingCoreFacts(*Session)` already returns the final category facts, while the injected `goos` and probe parameters are the fixture surface required by ADR-0010. This design only removes the two proven Plan 9 shadow paths.

## Goals / Non-Goals

**Goals:**

- Make descriptor-declared emitted roots control resolver gating without changing enabled output.
- Preserve shared probe reuse through existing `Session` memoization and acquire DMI only when a kept consumer needs it.
- Remove mutable production globals while preserving discovery-time freshness and CLI/library input behavior.
- Preserve the fixed 30-second executable timeout, 1 MiB external-source limit, and cache warning semantics.
- Delete test-only/shadow code after moving useful assertions onto production paths.

**Non-Goals:**

- No fact names, values, ordering, format bytes, schema entries, public Go APIs, CLI flags, or dependencies change.
- No general resolver dependency graph or scheduler is introduced.
- No broad networking interface, `hostOS`, or fixture refactor is performed.
- `virtual`, `is_virtual`, other inline facts, and the `selinux` compatibility exception do not become resolution-gated.
- Platform-inapplicable roots are not treated as disabled roots.

## Decisions

### 1. Use one emitted-root predicate instead of a dependency graph

Each descriptor will have the minimum live scheduling distinction: always-eager or gateable. A gateable descriptor runs when at least one declared top-level emitted root is enabled and is skipped only when every declared root is explicitly disabled. A standalone resolver is the one-root case of the same rule.

The descriptor order and assembly functions remain the orchestrator. `emittedRoots` stays authoritative and receives a row-exact agreement test. The unused, ambiguous `probeConsumers` and `emitsUnder` fields are removed; shared-probe behavior comes from the kept assembly functions calling existing `Session` memoizers.

Alternatives considered:

- Retaining four class names would keep values with no distinct behavior and invite another metadata/implementation split.
- Building a probe-consumer graph would require resolving ambiguous non-root names (`azure` versus `az_metadata`, for example) and adds scheduling machinery that the existing memoizers already avoid.
- Deleting all metadata would leave multi-output gating dependent on another handwritten list.

Provider descriptors must include every root they can emit, including shared `cloud`: Azure declares `az_metadata` and `cloud`; EC2 declares `ec2_metadata`, `ec2_userdata`, and `cloud`; GCE declares `gce` and `cloud`. OS, disk, and uptime descriptors likewise declare their complete root sets.

### 2. Keep eager virtualization, make DMI demand-driven

`detectVirtualization` remains in core-build construction because `virtual` and `is_virtual` are intentionally always eager under ADR-0015. DMI is removed from eager build construction. The DMI assembly and GCE assembly call `Session.cachedDMI()` independently when kept; the existing memoizer guarantees one probe when both need it and no probe when neither does.

Identity/SSH sharing follows the same rule without new metadata: kept SSH resolution may call `cachedIdentity`, while an independently kept identity assembly calls the same memoizer.

### 3. Capture ambient defaults in one invocation-local value

An internal concrete defaults value will contain the facts-native and compatible config paths, cache path, and ordered default external-fact directories. The current platform/environment adapter produces this value once per `Engine.Discover` and once per CLI `Run`; tests may supply a literal value through internal construction seams. Slices are cloned when frozen.

The zero-value public Engine remains hermetic. A system-following Engine derives a fresh defaults value for every discovery, preserving current environment/config freshness. The CLI derives one value per invocation and passes it through config parsing, fact-group listing, and Engine construction. This removes mutable path/directory functions without adding a new interface or public option.

Native config still precedes the compatible config; explicit config and external directories still override defaults; `--no-external-facts` still suppresses every external source; cache selection and TTL policy remain discovery-scoped.

Alternatives considered:

- Storing defaults in a package singleton would retain the contract violation.
- Memoizing defaults on Engine construction would make repeated system-following discoveries stale.
- Adding one interface per filesystem/network operation would broaden the architecture for values that are already naturally represented as data.

### 4. Treat cache I/O and external limits as fixed policy

Cache production code calls `writeCacheFile` and `os.Remove` directly. Tests of permission-error diagnostics target small warning-policy helpers with explicit errors rather than replacing process-global functions.

The external executable deadline and byte ceiling become constants. The timeout test uses the existing fake loader host to block on `ctx.Done()` inside `testing/synctest`, allowing the real 30-second deadline to advance virtually; it does not drive real `os/exec` I/O, which is not durably blocking to `synctest`. Size tests use a real limit-plus-one fixture. The loader remains configurable only through its existing host/session seam, not through policy fields.

### 5. Remove false test surfaces

Tests using `partitionsFact` move to the production `partitionsFactWithMountEntries` path. Windows mapping assertions move to `windowsHardwareFromGoArch`, `windowsArchitectureFromHardware`, and `parseWindowsOSVersionInfo`; the unreachable wrapper/type helpers are deleted.

The unreachable Plan 9 branch in `networkingInterfacesForPlatform` is removed because production dispatches to `plan9NetworkingCoreFacts` first. Tests of `plan9NetworkingCoreFactsWithGlob` move to a `Session` backed by `fakeHostOS.globs`, after which the wrapper is deleted. All pure platform parser fixtures remain.

## Risks / Trade-offs

- **Incomplete emitted-root metadata could over-skip a resolver** → Add a row-exact descriptor test, output-subset agreement tests, and all-disabled/one-kept probe-count cases for every multi-output/provider descriptor.
- **Skipping work can remove debug messages or benign side effects** → Treat that as the intended ADR-0015 behavior, update the changelog, and pin public output plus ambient-disable diagnostic bytes.
- **Default capture could change precedence or freshness** → Exercise native/compatible config precedence, explicit/default external dirs, cache path selection, repeated discovery, concurrent isolation, and CLI contract tests.
- **Real timeout/size tests could become slow or memory-heavy** → Use `testing/synctest` and bounded 1 MiB-plus-one fixtures.
- **Plan 9 cleanup could accidentally change assembly** → Retarget tests to the production Session seam and run the native Plan 9 release gate with networking parity.

## Migration Plan

1. Add failing resolver-gating/probe-count and invocation-isolation tests.
2. Implement descriptor scheduling and demand-driven DMI, then update the changelog.
3. Introduce invocation-local defaults and remove mutable path/directory globals.
4. Constify external policy, remove cache I/O globals, and retarget tests.
5. Delete unreachable helper/shadow paths and retain assertions on production seams.
6. Run local formatting, unit, race, shuffle, vet, build, and dead-code gates; then validate the exact checkout on nlab and its supported guests.

Rollback is a normal source revert; there is no data or configuration migration.

## Open Questions

None. The deep audit resolved the catalog, default-input, networking, and cleanup boundaries before implementation.
