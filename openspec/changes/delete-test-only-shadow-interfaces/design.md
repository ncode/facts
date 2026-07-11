## Context

The core-fact engine has ten unexported functions in production source that have no production callers. Six are test-only adapters beside production-owned seams—five category wrappers plus the `releaseHashFromMatchData` compatibility adapter—and four are standalone utilities whose behavior is asserted only by tests or benchmarks. They survived earlier category and host-probe refactors, so some tests now exercise a contract that production discovery never crosses.

This is especially misleading in DHCP discovery: the test-only wrapper discards the `matched` state that controls production fallback behavior. Similar wrappers omit uptime provenance, choose a synthetic SSH platform, or duplicate DragonFly DMI coordination in a form that production cannot safely use.

The cleanup overlaps the completed but unarchived `fix-linux-dhcp-lease-interface-match` change. Implementation begins after that change is archived so its final state-returning semantics and tests are the recorded baseline; the proposals may coexist.

## Goals / Non-Goals

**Goals:**

- Remove the exact ten verified test-only or dead functions.
- Retarget valuable tests to category assembly or pure internal seams that production also invokes.
- Preserve coordination state that the narrower wrappers currently hide.
- Leave the core-fact test surface smaller and more truthful without changing fact behavior.

**Non-Goals:**

- Establish a repository-wide dead-code policy or automated dead-code gate.
- Remove pure internal seams that have both production and test callers.
- Refactor category ownership, platform dispatch, or probe implementations.
- Change facts, values, platform gating, diagnostics, public APIs, or CLI behavior.
- Add exported test hooks or new test-only production interfaces.

## Decisions

### Delete a fixed, reviewed set instead of every apparent dead function

The implementation will remove only these functions:

- `linuxDHClientDHCPServerForInterface`
- `currentUptime`
- `augeasFacts`
- `dragonFlyDMIFacts`
- `sshFacts`
- `rootedPath`
- `bytesToMB`
- `releaseHashFromMatchData`
- `tryToBool`
- `tryToInt`

Repository-wide dead-code output can include public library entry points, build-target-specific code, and intentional internal seams. A fixed list keeps this change reviewable and avoids turning a local cleanup into a new architectural policy.

### Retarget wrapper tests to production-owned seams

Tests that still protect behavior will cross the corresponding production seam:

- DHCP tests will call `linuxDHClientDHCPServerForInterfaceState` and assert the server, `matched`, and `explicit` results.
- Uptime tests will call `currentUptimeInfo` and preserve duration, `Known`, and fake-host probe/source-selection call assertions.
- Augeas tests will cover `parseAugeasVersion` and `augeasVersionFacts` directly.
- DragonFly DMI coordination will be tested through `currentDragonFlyDMIFacts` with `fakeHostOS`; decoding remains covered through `dragonFlyDMIDecodeFacts`.
- SSH tests will call `sshFactsForPlatform` with an explicit supported platform.
- Release parsing will remain covered through `releaseHashFromString`; match-array assertions unique to the unreachable `releaseHashFromMatchData` adapter will be removed.

These seams already participate in production discovery. Retargeting therefore improves coverage of the actual control flow instead of introducing replacements for the deleted wrappers.

### Delete ghost utility contracts without replacements

`rootedPath`, `bytesToMB`, `tryToBool`, and `tryToInt` have no production caller or production-owned replacement contract. Their dead-contract assertions and the benchmark line that calls `bytesToMB` will be removed rather than moved behind another abstraction. Mixed tests and benchmarks will retain coverage for production-used siblings such as `isSymlink`, `bytesToHumanReadable`, `hertzToHumanReadable`, and `numericValue`.

### Preserve lazy platform coordination in tests

DragonFly DMI tests must retain assertions that kenv data short-circuits dmidecode and that dmidecode is used only as fallback. The tests will inject `fakeHostOS` and observe calls rather than eagerly computing a value for a convenience wrapper. This keeps the test faithful to the production side-effect boundary.

### Sequence after the DHCP change is archived

The active DHCP change owns the final meaning of the state-returning seam in `networking.go` and its tests. This cleanup will begin after that result is archived and treat it as the recorded baseline rather than restating its behavior.

## Risks / Trade-offs

- **Coverage can be lost while deleting contract-only tests.** Each surviving behavioral assertion will first be mapped to a production-owned seam; only assertions for unreachable behavior will disappear.
- **DMI tests could accidentally make dmidecode eager.** Fake-host call counts and the kenv short-circuit scenario will pin the lazy production order.
- **DHCP cleanup could start from an unsettled contract.** Beginning after `fix-linux-dhcp-lease-interface-match` is archived makes its final state-returning semantics the implementation baseline.
- **Static dead-code analysis can produce false positives across targets.** The implementation is constrained to the reviewed list and will verify each name is absent after deletion.

## Migration Plan

1. Archive `fix-linux-dhcp-lease-interface-match`.
2. Retarget the useful tests to production-owned seams while preserving their assertions.
3. Delete the ten functions and tests or benchmark cases that exist only for unreachable contracts.
4. Verify the deleted names are absent and run the engine, full, race-sensitive, vet, and build checks.

Rollback is a normal revert because the change has no persisted data, public API, or user-visible migration.

## Open Questions

None.
