## Why

Ten unexported helpers in `internal/engine` have no production callers on any supported or candidate target. Six are test-only adapters beside production-owned seams—five category wrappers plus the `releaseHashFromMatchData` compatibility adapter—while four are standalone utilities for behavior that no production path consumes. Both patterns make coverage less truthful and leave obsolete interfaces in production source.

## What Changes

- Delete the test-only `linuxDHClientDHCPServerForInterface`, `currentUptime`, `augeasFacts`, `dragonFlyDMIFacts`, `sshFacts`, and `releaseHashFromMatchData` adapters.
- Retarget useful coverage to production-owned category assembly or pure seams, including every valid DHCP match-state tuple, uptime `Known` state and probe-selection order, DragonFly lazy fallback behavior, platform-specific SSH behavior, and release parsing through `releaseHashFromString`.
- Delete the unused `rootedPath`, `bytesToMB`, `tryToBool`, and `tryToInt` utilities together with only the assertions and benchmark cases that specify those dead contracts.
- Strengthen the affected platform-fact regression contract so the production-owned coordination states remain covered during the cleanup.
- Preserve every resolved fact, public Facts interface, CLI byte/status contract, input contract, diagnostic, schema entry, and platform behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `go-port-supported-platform-facts`: Removing obsolete internal test entrances preserves production match state, probe order, fallback behavior, and all supported and candidate platform facts.

## Impact

- **Code**: focused deletions and test retargeting in `internal/engine` category files, `core.go`, `factsutil.go`, and their tests.
- **Behavior**: none; this is an in-process test-surface cleanup.
- **Sequencing**: begin implementation after the completed `fix-linux-dhcp-lease-interface-match` change is archived so its final state-returning DHCP semantics and tests are the recorded baseline; the proposals may coexist.
- **Dependencies/docs**: no new dependency, public interface, schema update, or `CHANGELOG.md` entry.
