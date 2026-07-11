## 1. Retarget Behavioral Coverage

- [x] 1.1 Confirm `fix-linux-dhcp-lease-interface-match` is archived, then retarget DHCP helper tests to `linuxDHClientDHCPServerForInterfaceState`; assert the server for each lease form and all valid `(matched, explicit)` tuples: `(false, false)`, `(false, true)`, and `(true, true)`, including a matched lease with an empty server value.
- [x] 1.2 Retarget uptime tests to `currentUptimeInfo`, preserving duration, `Known`, and fake-host probe/source-selection call assertions.
- [x] 1.3 Preserve Augeas parsing and fact-shaping coverage through `parseAugeasVersion` and `augeasVersionFacts`, eliminating assertions that exist only for the wrapper contract.
- [x] 1.4 Retarget DragonFly DMI coordination tests to `currentDragonFlyDMIFacts` with `fakeHostOS`, preserving kenv short-circuit, dmidecode fallback, and probe call-count assertions.
- [x] 1.5 Retarget SSH fact tests to `sshFactsForPlatform` with explicit supported platforms.
- [x] 1.6 Preserve production release-parsing coverage through `releaseHashFromString` while removing assertions unique to the unreachable match-array adapter.

## 2. Delete Shadow Interfaces

- [x] 2.1 Delete `linuxDHClientDHCPServerForInterface`, `currentUptime`, `augeasFacts`, `dragonFlyDMIFacts`, `sshFacts`, and `releaseHashFromMatchData` after their useful coverage crosses production-owned seams or their adapter-only contract is discarded.
- [x] 2.2 Delete `rootedPath`, `bytesToMB`, `tryToBool`, and `tryToInt`; remove only the `rootedPath` assertions from the combined helper test and the `bytesToMB` line from `BenchmarkUnitFormatting`, retaining the `isSymlink`, `bytesToHumanReadable`, and `hertzToHumanReadable` coverage and deleting only dedicated dead-contract tests otherwise.
- [x] 2.3 Search tracked Go source, tests, benchmarks, build-tag variants, and scripts—excluding OpenSpec artifacts and history—to verify the ten names have no remaining executable-source references while the production-owned replacement seams remain covered.
- [x] 2.4 Run `gofmt -w` on every edited Go file.

## 3. Verify Behavior Preservation

- [x] 3.1 Run `rtk go test ./internal/engine` and confirm the retargeted tests exercise the production paths.
- [x] 3.2 Run `rtk go test ./...` and `rtk go test -race . ./internal/engine ./internal/app`.
- [x] 3.3 Run `rtk go vet ./...` and `rtk make build`.
- [x] 3.4 Run strict OpenSpec validation for `delete-test-only-shadow-interfaces` and confirm no public API, CLI, schema, changelog, or dependency update is required.
