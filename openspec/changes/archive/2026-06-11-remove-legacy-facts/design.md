# Design: Remove legacy fact support

## Context

Ruby Facter carries ~150 flat "legacy" aliases of structured facts (`operatingsystem` for `os.name`, `hostname`/`fqdn`/`domain` for `networking.*`, `processorcount` for `processors.count`, per-interface `mtu_*`/`ipaddress_*`, `ssh*key`/`sshfp_*`), deprecated since Facter 3 and hidden behind `--show-legacy`. The Go port reproduces them in `internal/facter/core.go` two ways: a `LegacyFacts` set appended under `--show-legacy` (tagged `Type: "legacy"`), and — by accident — 25 untagged legacy-named facts emitted alongside the structured core set, which leak into default output (51 top-level facts on macOS vs Ruby's 22, found in the 2026-06-11 host comparison). The CLI also resolves explicitly queried legacy facts without `--show-legacy` (`queriesLegacyFacts` under `CLICompat`). Facts is unreleased; the decision is to start structured-only rather than reproduce a deprecated compatibility layer.

## Goals / Non-Goals

**Goals:**
- No legacy alias resolves anywhere: not in default output, not under any flag, not as an explicit query, not in the library Snapshot.
- The CLI surface matches: `--show-legacy`/`--no-show-legacy` fail as unknown options; the `show-legacy` config key and the `legacy` blocklist group are inert.
- Release gates, smokes, and acceptance tests assert structured names only and stay green on all four platforms.
- The decision is recorded in an ADR with the rejected alternatives.

**Non-Goals:**
- No change to the *legacy output format* (the default `key => value` text formatter, `FormatLegacy*` functions) — "legacy" there names Ruby's default text format, not legacy facts. Renaming those identifiers is out of scope.
- No change to structured fact values (separate changes cover not-applicable omission and Darwin networking fixes).
- No removal of dotted-fact querying or `--force-dot-resolution` (external facts can still define dotted names).
- No rewrite of `docs/PARITY_LEDGER.md` (frozen historical record).

## Decisions

**1. Hard removal, not Ruby-parity hiding.**
The alternative — tag the 25 leaked facts `Type: "legacy"` so default output matches Ruby's 22 and keep `--show-legacy` — was rejected. Facts is a new service with no users; shipping a deprecated alias layer permanently just to match a compatibility flag optimizes for a past nobody here has. This follows the ADR-0006 pattern: remove while the contract amendment is free. Recorded as a new ADR (next number free at implementation time).

**2. Flags are deleted, not no-ops.**
`--show-legacy`/`--no-show-legacy` fail as unrecognized options, the `show-legacy` config key parses as an unknown key, both per the ADR-0006 no-zombie-flags rationale. The `legacy` blocklist group needs no special casing: with no legacy facts produced, `BlocklistedFactsForFiltering` expanding `legacy` matches nothing — verify it degrades silently rather than warning.

**3. Explicit legacy queries become ordinary missing facts.**
`queriesLegacyFacts` (CLI resolves queried legacy facts without `--show-legacy`) is deleted with `LegacyFacts`. `facter operatingsystem` prints nothing and exits 0; under `--strict` it is a missing fact. No alias-to-structured redirect: aliasing is the layer being removed.

**4. Gates migrate to structured equivalents in the same commit.**
Every gate fact-set keeps its *coverage* (OS, networking, memory, processors, DMI, …) but swaps names: `operatingsystem` → `os.name`, `osfamily` → `os.family`, `architecture` → `os.architecture`, `hardwaremodel` → `os.hardware`, `processorcount` → `processors.count`, `memorysize` → `memory.system.total`, `kernelmajversion` stays (core, not legacy). Affected: `tests/acceptance/acceptance_test.go` `releaseGateFactSet`, `tools/windows-release-gate.ps1`, `tools/freebsd-release-gate.sh`, Makefile Lima smokes, `.github/workflows/integration_tests.yaml`.

**5. The legacy/core boundary is decided by Ruby's classification, not the Go port's current tagging.**
The authoritative list of what to delete is Ruby Facter 4.10.0's `--show-legacy` minus default output (captured 2026-06-11 in `/tmp/facter-parity/`; regenerate if stale). Facts in Go's default output that Ruby treats as legacy are deleted even though the Go port never tagged them (`architecture`, `hostname`, `fqdn`, `interfaces`, `ipaddress*`, …). Top-level facts Ruby shows by default (`kernel*`, `timezone`, `path`, `facterversion`, `is_virtual`, `virtual`, `identity`, `dmi`, …) stay.

## Risks / Trade-offs

- [Scripts ported from Ruby facter query legacy aliases and silently get nothing] → Deliberate and documented: CHANGELOG breaking entry, README positioning, migration table in the ADR and man page; `--strict` turns silence into a hard error for callers who want it.
- [A structured equivalent is missing for some legacy alias someone needs] → The alias table in the migration notes maps every removed name to its structured source; anything truly unmapped (none known) would be added as a structured fact, not as an alias.
- [Gate fact-set swap breaks a platform gate (name typo, value shape change)] → Swap and verify per platform in one PR; the four CI gates are blocking and catch it before merge.
- [Deleting untagged legacy facts accidentally removes a structured fact] → The deletion list is derived from the Ruby legacy classification (decision 5) and reviewed against `ruby-keys.txt`/`go-keys.txt`; structured-tree tests (`os`, `networking`, `processors`, …) pass unmodified as the guard.
- [`blocked["legacy"]` code paths left half-removed] → Sweep `"legacy"` string literals in `internal/` after removal; only the text-format function names remain.

## Migration Plan

Single PR, ordered so each commit builds green; applied after `remove-ruby-custom-fact-dsl` (same CLI files):
1. ADR + delta specs (this change).
2. Engine: delete `LegacyFacts`, legacy builders, `Type: "legacy"` sites, untagged legacy-named core facts, `IncludeLegacy`, `queriesLegacyFacts`; retarget/delete engine tests.
3. CLI: drop `--show-legacy`/`--no-show-legacy`, `showLegacyPattern`/`ShowLegacy`, `ConfigBlocksLegacy`; negative tests for flags/config key/blocklist group.
4. Gates and acceptance: swap fact sets to structured names (decision 4).
5. Docs sweep (help, man, README, FACTER_CONF_COMPATIBILITY, CHANGELOG) and full verification on all platform gates.

Rollback: revert the PR; no data or on-disk format involved.

## Open Questions

None — direction set by the maintainer on 2026-06-11 ("this is a new service, we should start with the future").
