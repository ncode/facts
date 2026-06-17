# Design: Remove the `--puppet` CLI flag

## Context

`--puppet` (`-p`) does two things at the CLI layer (`internal/app/app.go:345-348`):

1. Appends `engine.PuppetPluginFactDirs()` — Puppet's agent plugin-fact destination `vardir/facts.d` (`/opt/puppetlabs/puppet/cache/facts.d` for root, `~/.puppetlabs/opt/puppet/cache/facts.d` for a user, `ProgramData\PuppetLabs\puppet\cache\facts.d` on Windows) — to the external-fact directories.
2. Calls `engine.WarnPuppetRubyPluginFacts()`, which warns when Puppet has pluginsync'd Ruby custom facts into `vardir/lib/facter/*.rb` (a directory facts never loads, hence the dedicated warning).

The engine carries an `EngineConfig.Puppet bool` field that no engine code reads — it is set by the CLI and inert.

## The decision

Facts implements **Facter's input/output contract, not Puppet's runtime behavior**. `--puppet` is the sole feature that crosses that line: it re-enacts what `puppet agent` arranges on disk (pluginfactdest discovery, pluginsync'd `.rb` plugin loading) rather than accepting a Facter-shaped input. It is removed. ADR-0009 records the principle so the next puppet-runtime feature does not get re-added.

The load-bearing boundary:

- **In the contract (kept):** Facter's own external-fact directories, including the ones that merely live under a `puppetlabs/` path — `/opt/puppetlabs/facter/facts.d`, `~/.puppetlabs/opt/facter/facts.d`, etc. (`internal/engine/config.go:50-76`). These are Facter's dirs, not Puppet's runtime, and stay in the always-on default set.
- **Out of the contract (removed):** Puppet's agent-runtime cache `vardir/facts.d` (pluginfactdest), pluginsync, and `.rb` plugin loading.

Reading "no Puppet" and deleting the `config.go` Facter defaults would be the predictable mistake; the boundary above is stated explicitly in ADR-0009 and the `CONTEXT.md` Input-contract entry to prevent it.

## Alternatives considered

- **CLI-only removal (hide the flag, keep `puppet.go` dormant):** rejected. The `Puppet` field is already a no-op and the helper is CLI-only; a "library not bound to Puppet" that still ships `PuppetPluginFactDirs` contradicts the identity. Full delete to the engine.
- **Bespoke tombstone error (`--puppet is removed; use --external-dir …`):** rejected. The pre-flight validator already fails unknown flags with `unrecognised option '--puppet'` (`internal/cli/validation.go:53-54`); a special-cased hint for one flag is inconsistent with every other removed/unknown flag and keeps a puppet-shaped stub. Human guidance lives in the CHANGELOG migration row and ADR.
- **Generalize the Ruby-plugin warning to every external dir:** unnecessary. The general external-dir loader *already* warns and skips any `.rb` file (`internal/app/contract_test.go:301`, `TestRun_warnsAndSkipsRubyExternalFact`). Only the Puppet-specific `vardir/lib/facter` warning is lost — and that directory is never loaded as an external dir, which was the only reason it needed a bespoke warning.

## Out of scope

- The `puppetversion` core fact and the "puppet-agent package facts" platform scenarios (`go-port-supported-platform-facts`) are untouched — they describe core facts, not the `--puppet` flag.
- No changes to caching, formatting, output, or any other input source.
