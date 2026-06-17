# Remove the `--puppet` CLI flag

## Why

`--puppet` (`-p`) is the one feature that reaches into Puppet's *runtime* rather than Facter's input/output contract. It appends Puppet's agent plugin-fact destination (`vardir/facts.d`, e.g. `/opt/puppetlabs/puppet/cache/facts.d`) to the external-fact directories and warns about pluginsync'd Ruby plugin custom facts in `vardir/lib/facter`. Facts is a standalone fact-discovery library and CLI that speaks Facter's contract; it is not bound to Puppet or to what Puppet does at runtime. ADR-0009 records the principle: Facter's own input shapes and external-fact directories are inside the contract, Puppet's agent-runtime surface is not. With facts decoupled from Puppet, remove the flag now while the contract amendment is still cheap.

## What Changes

- **BREAKING** Drop the `--puppet`, `-p`, and `--no-puppet` CLI flags, their conflict/duplicate checks, and the matching help/usage/man text. Invocations passing them now fail with `unrecognised option '--puppet'`, exactly as for any unknown flag — no bespoke tombstone error.
- **BREAKING** Stop auto-discovering Puppet's agent plugin-fact destination (`vardir/facts.d`). That directory is still reachable explicitly via `--external-dir /opt/puppetlabs/puppet/cache/facts.d` (or the platform/user equivalent). Facter's own external-fact directories — the `puppetlabs/facter/facts.d` defaults in the always-on default set — are unchanged.
- **BREAKING** Drop the Puppet Ruby-plugin warning (`WarnPuppetRubyPluginFacts`, the `vardir/lib/facter/*.rb` check). The general external-dir `.rb` skip-with-warning is unaffected: any `.rb` file in an external dir still warns and is skipped.
- Delete `internal/engine/puppet.go` in full (`PuppetPluginFactDirs`, `WarnPuppetRubyPluginFacts`, `defaultPuppetCacheDir` + its root/user/Windows path logic) and the inert `EngineConfig.Puppet` field. The engine loses every notion of Puppet; "puppet" survives only as path strings in the Facter default-dir list.
- Delete the stale "Deviation: `facts --puppet`" section from `docs/CUSTOM_FACT_MIGRATION.md`; the general no-`.rb` statement at the top of that guide already covers Puppet-synced `.rb`.
- Output contract and the rest of the input contract (Facter external facts, `facter.conf`/`facts.conf`, `FACTER_*`/`FACTS_*` env) are byte-for-byte unchanged.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-framework-fidelity`: The "Bounded and documented --puppet behavior" requirement is removed entirely (ADR-0009) — `--puppet` no longer exists, so there is no bounded behavior to specify.
- `go-port-custom-fact-dsl-contract`: The "Puppet plugin deviation is documented" scenario leaves the migration-guide requirement (no `--puppet`, no deviation to document); the no-`.rb`-anywhere requirement is unchanged.
- `go-port-distribution-and-cutover`: The man-page-parity scenario drops `--puppet` from the list of Go-port deviations to note.

## Impact

- **Code removed**: `internal/engine/puppet.go` (whole file) and `internal/engine/puppet_test.go` if present; `EngineConfig.Puppet` in `internal/engine/engine.go`; the `--puppet`/`-p`/`--no-puppet` flag defs, duplicate check, version fast-path puppet param, and the pluginfactdest append + warn in `internal/app/app.go`; the `--puppet`/`-p`/`--no-puppet` entries (`knownOption`, `-p` alias, conflict rows) in `internal/cli/validation.go`; usage/help text.
- **Docs**: `docs/CUSTOM_FACT_MIGRATION.md` "Deviation: --puppet" section deleted; man page regenerated without `--puppet`/`-p`/`--no-puppet`; README/`doc.go` if they name `--puppet`; `docs/adr/0009-facter-contract-not-puppet-runtime.md` added; CHANGELOG breaking-change entry with the `--external-dir` migration line; the `CONTEXT.md` Input-contract glossary update is already in the working tree.
- **Behavioral**: scripts passing `--puppet`/`-p`/`--no-puppet` get a usage error; Puppet agent-synced module external facts are no longer auto-loaded (pass `--external-dir`); `.rb` skip-with-warning in external dirs unchanged; everything else (Facter external facts, config, caching, formatting, output) unchanged.
- **Dependencies**: none added or removed.
