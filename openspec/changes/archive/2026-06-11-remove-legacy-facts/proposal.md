# Remove legacy fact support

## Why

Facts is a new, unreleased service and should start with the future: the canonical structured tree (`os.name`, `networking.hostname`, `processors.count`), not the ~150 flat Ruby-era aliases (`operatingsystem`, `hostname`, `processorcount`, `sshfp_*`, `mtu_*`) that Ruby Facter itself has deprecated and hidden behind `--show-legacy` since Facter 3. A 2026-06-11 host comparison against Ruby Facter 4.10.0 also showed the Go port leaking 25 untagged legacy facts into default output (51 top-level facts vs Ruby's 22); deleting the layer resolves that bug by removal instead of by re-tagging dead weight.

## What Changes

- **BREAKING** Remove all legacy facts from every output mode: `LegacyFacts`, the per-platform legacy alias builders, and the untagged legacy-named core facts (`architecture`, `hostname`, `fqdn`, `domain`, `ipaddress*`, `macaddress`, `memorysize*`, `netmask`, `network*`, `operatingsystem*`, `osfamily`, `hardwareisa`, `interfaces`, `processorcount`, `physicalprocessorcount`, `ssh*key`, `sshfp_*`, `mtu_*`, …). Structured facts are the only surface; `facter operatingsystem` resolves nothing.
- **BREAKING** Drop the `--show-legacy` and `--no-show-legacy` flags (fail as unknown options, per the ADR-0006 no-zombie-flags precedent), the `show-legacy` `facter.conf` key (inert like any unrecognized key), and the CLI behavior that resolved explicitly queried legacy facts without `--show-legacy`.
- The `legacy` blocklist group becomes meaningless: a blocklist containing `legacy` loads without error and has no effect (there is nothing to block).
- Engine internals go with the surface: `EngineConfig.IncludeLegacy`, `queriesLegacyFacts`, `ConfigBlocksLegacy`, and the `Type: "legacy"` fact classification.
- Release gates, acceptance tests, and smoke targets swap legacy aliases for structured equivalents (`operatingsystem` → `os.name`, `osfamily` → `os.family`, `architecture` → `os.architecture`, `hardwaremodel` → `os.hardware`, `processorcount` → `processors.count`, `memorysize` → `memory.system.total`).
- The *legacy output format* (the default `key => value` text formatter, `FormatLegacy*`) is an unrelated concept and is untouched.
- A new ADR records the decision and the rejected alternative (Ruby-parity hiding via `--show-legacy`).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-framework-parity`: the CLI parity scenario drops `--show-legacy`/`--no-show-legacy` and "legacy fact inclusion"; a new requirement states legacy facts are not part of the surface and the flags fail as unknown options.
- `go-port-supported-platform-facts`: "Core fact and legacy alias parity" becomes "Core fact parity" — legacy aliases and "legacy alias visibility" leave all four platform scenarios.
- `go-port-ci-platform-gates`: the Windows release-gate fact set and the Lima FreeBSD smoke fact set drop "legacy aliases" in favor of structured equivalents.
- `go-port-completion-verification`: the Windows validation gate fact list drops legacy aliases.
- `go-port-distribution-and-cutover`: the binary acceptance suite's flag combinations drop `--show-legacy`.

## Impact

- **Code removed**: `LegacyFacts` and per-platform legacy builders in `internal/facter/core.go` (including `Type: "legacy"` sites); the untagged legacy-named facts emitted alongside structured core facts; `--show-legacy`/`--no-show-legacy` parsing, help/man text in `internal/app/app.go` and `internal/cli/`; `showLegacyPattern`/`ShowLegacy` in `internal/facter/config.go`; `IncludeLegacy` and `queriesLegacyFacts` in `internal/facter/engine.go`; `ConfigBlocksLegacy`.
- **Gates and tests**: `tests/acceptance/acceptance_test.go` release-gate fact set, `tools/windows-release-gate.ps1`, `tools/freebsd-release-gate.sh`, Makefile smoke targets, and the integration-tests workflow swap legacy aliases for structured queries; engine/app tests covering `--show-legacy` and legacy filtering are deleted or retargeted.
- **Docs**: help text, man page, README (`--show-legacy` mentions), `FACTER_CONF_COMPATIBILITY.md` (`show-legacy` key retired), CHANGELOG breaking entry, new ADR. `docs/PARITY_LEDGER.md` stays frozen.
- **Behavioral**: default output shrinks from 51 to ~26 top-level facts on macOS (the structured set plus whatever `omit-not-applicable-facts` later removes); scripts querying legacy aliases must move to structured names.
- **Dependencies**: none. Builds on `remove-ruby-custom-fact-dsl` (apply that change first; both edit the same CLI surface).
