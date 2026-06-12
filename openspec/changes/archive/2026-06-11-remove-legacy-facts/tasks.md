# Tasks: Remove legacy fact support

## 1. Decision record and inventory

- [x] 1.1 Write the ADR (next free number): legacy facts removed, structured tree is the only surface; record the rejected Ruby-parity-hiding alternative and the alias→structured migration table
- [x] 1.2 Derive the authoritative deletion list: diff Ruby Facter 4.10.0 default vs `--show-legacy` output (regenerate `/tmp/facter-parity/` captures if stale) and enumerate every legacy-classified fact the Go port emits, tagged or untagged
- [x] 1.3 Inventory tests, gates, and docs that reference legacy aliases or `--show-legacy` (engine/app/CLI tests, acceptance `releaseGateFactSet`, `tools/*-release-gate.*`, Makefile smokes, integration workflow, README, man page)

## 2. Engine removal

- [x] 2.1 Delete `LegacyFacts`, the per-platform legacy alias builders, and every `Type: "legacy"` emission in `internal/facter/core.go`
- [x] 2.2 Delete the untagged legacy-named core facts from the 1.2 list (`architecture`, `hostname`, `fqdn`, `domain`, `ipaddress*`, `macaddress`, `memorysize*`, `netmask`, `network*`, `operatingsystem*`, `osfamily`, `hardwareisa`, `interfaces`, `processorcount`, `physicalprocessorcount`, `ssh*key`, `sshfp_*`, …) while leaving their structured sources untouched
- [x] 2.3 Remove `EngineConfig.IncludeLegacy` and `queriesLegacyFacts` from `internal/facter/engine.go`
- [x] 2.4 Verify `blocklist : [ "legacy" ]` loads without error or warning and has no effect; delete `ConfigBlocksLegacy`
- [x] 2.5 Retarget engine/snapshot tests that used legacy facts for non-legacy behavior; delete tests that only pinned legacy aliases; add a test that no legacy alias from the 1.2 list appears in a default Snapshot

## 3. CLI surface removal

- [x] 3.1 Remove `--show-legacy`/`--no-show-legacy` flag definitions and wiring from `internal/app/app.go`; remove them from `internal/cli` option validation; remove `showLegacyPattern`/`ShowLegacy` from `internal/facter/config.go`
- [x] 3.2 Remove the flags from help text and the man source; regenerate/edit `man/man8/facter.8`
- [x] 3.3 Add negative tests: both flags exit with a usage error naming the unknown option; a `facter.conf` with `show-legacy : true` loads with no error and no effect
- [x] 3.4 Add negative test: `facter operatingsystem` prints nothing and exits 0; with `--strict` it reports the missing fact and exits 1
- [x] 3.5 Delete or retarget app/CLI tests that exercised `--show-legacy`, `--no-show-legacy`, legacy queries, or legacy blocklisting

## 4. Gates, acceptance, and smokes

- [x] 4.1 Swap `releaseGateFactSet` in `tests/acceptance/acceptance_test.go` to structured equivalents (`os.name`, `os.family`, `os.architecture`, `os.hardware`, `processors.count`, `memory.system.total`, …) and drop the `--show-legacy` acceptance case
- [x] 4.2 Update `tools/windows-release-gate.ps1` and `tools/freebsd-release-gate.sh` fact sets to structured names
- [x] 4.3 Update Makefile Lima smokes and `.github/workflows/integration_tests.yaml` fact queries to structured names
- [x] 4.4 Sweep for dangling references: `grep -rn '"legacy"\|show-legacy\|ShowLegacy\|IncludeLegacy\|LegacyFacts' --include='*.go'` returns only the text-format (`FormatLegacy*`) identifiers

## 5. Documentation

- [x] 5.1 Update README and man page: no legacy facts, no `--show-legacy`; point at the alias→structured table in the ADR
- [x] 5.2 Update `docs/FACTER_CONF_COMPATIBILITY.md`: `show-legacy` joins the retired-keys list; `legacy` blocklist group documented as inert
- [x] 5.3 Add CHANGELOG breaking-change entry: legacy facts removed; flags removed; config key inert; alias migration table pointer
- [x] 5.4 Sweep `docs/MIGRATION.md` forward-pointing references and add a completed-slice checkpoint (do NOT touch `docs/PARITY_LEDGER.md`)

## 6. Verification

- [x] 6.1 `go test ./...`, `go test -race ./...`, `go vet ./...`, gofmt clean
- [x] 6.2 Host comparison rerun: Go default output top-level keys are a subset of Ruby's default set plus documented deviations (no legacy names present)
- [ ] 6.3 All platform CI gates green on the final commit with the structured fact sets
- [x] 6.4 `openspec validate remove-legacy-facts` passes; artifacts complete before archive
