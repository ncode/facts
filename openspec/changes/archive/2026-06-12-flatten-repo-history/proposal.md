# Flatten the repository history

## Why

The repository carries the full upstream puppetlabs/facter history: 2,340 commits and 84 contributors dating to July 2019, ~98% of it another product's work. As a public signal of this project's activity and authorship it is misleading — the Go port is a few dozen commits by one author. The maintainer decided on 2026-06-12 to flatten the history to a single initial commit, without preserving the old history in an archive (upstream puppetlabs/facter remains the reference for the Ruby past).

## What Changes

- **BREAKING (repo meta)**: `main` is rewritten to a single initial commit containing the current tree; the upstream and porting-era commit history is discarded from this repository. Any existing clone or fork diverges permanently.
- The porting-era detail records (the 9,471-line migration log, the frozen parity ledger, the port tracker, the Ruby internals reference, the inherited release notes) become unrecoverable from this repository: `docs/HISTORY.md` stops printing `git show` recovery commands and states the flattening honestly. The summary content of `docs/HISTORY.md` is now the only surviving record, which is the accepted cost.
- The `go-port-ruby-removal` requirement "Historical records survive the removal" is removed: its recoverability guarantee cannot hold without the history, by deliberate owner decision.
- `NOTICE` is unchanged — Apache 2.0 attribution never depended on git history. The new initial commit message records the provenance (ported from puppetlabs/facter).
- Commit hashes cited inside archived OpenSpec artifacts become inert historical text; archives are point-in-time records and are not edited.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-ruby-removal`: the "Historical records survive the removal" requirement is REMOVED.

## Impact

- **Repo**: history 2,340 commits → 1; contributor graph and age reset to the truth.
- **Docs**: `docs/HISTORY.md` recovery section replaced with the flattening statement.
- **Specs**: one requirement removed.
- **Irreversibility**: once garbage-collected on the remote, the old objects are gone from this repository; upstream puppetlabs/facter retains the Ruby history independently.
