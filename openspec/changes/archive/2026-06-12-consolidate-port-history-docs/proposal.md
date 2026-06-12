# Consolidate the porting history into one doc; retire stale upstream content

## Why

The repository carries 11,600+ lines of documentation, ~95% of it frozen porting history: `docs/MIGRATION.md` (9,471 lines of porting log), `docs/PARITY_LEDGER.md` (938 lines, declared frozen), the 507-line tail of `CHANGELOG.md` (inherited Ruby Facter 4.0.x release notes pointing at puppetlabs PRs), and `docs/data-flow.md` (39 lines describing *Ruby* Facter's internal classes — `LoadedFact`, `QueryParser`, `Facter.add` — none of which exist here). `CONTRIBUTING.md` is nine lines of upstream Puppet boilerplate whose schema link (`lib/schema/facter.yaml`) points at a file that does not exist and whose contribution guide links to puppetlabs. A reader landing on this repo today gets Puppet's history instead of Facts' present.

## What Changes

- Create **`docs/HISTORY.md`** — a single condensed summary (~200 lines) of the port: what was ported, the approach (TDD slices, parity ledger, platform gates), the release-readiness milestones, the deliberate departures (ADR-0006/0007/0008), and a pointer to commit `daa2f7b9` (the last commit containing the full records) for the complete `MIGRATION.md`, `PARITY_LEDGER.md`, and `PORTING.md` texts in git history.
- Delete from HEAD: `PORTING.md`, `docs/MIGRATION.md`, `docs/PARITY_LEDGER.md`, `docs/data-flow.md`. Git history is the archive; `HISTORY.md` is the index into it.
- Trim `CHANGELOG.md` to Facts-only content: the inherited Ruby Facter "Previous versions" section (~507 lines) is removed, with a one-line pointer in `HISTORY.md`.
- Rewrite **`CONTRIBUTING.md`** for Facts: build/test/bench commands, the TDD rules and platform scope that currently live in `PORTING.md` (the only still-live content in it), the four release gates, the OpenSpec change workflow, and the Ruby Facter comparison technique for parity questions. No Puppet boilerplate, no broken links.
- Retire the porting-era spec scaffolding that pinned those files: the `go-port-parity-ledger-integrity` capability (its generator was deleted with the Ruby removal; both requirements are about regenerating a now-frozen record), the completed-milestone requirements in `go-port-completion-verification` and `go-port-distribution-and-cutover`, and the doc pointers in `go-port-ci-platform-gates` and `go-port-ruby-removal`.
- Final Puppet-mention sweep with an explicit keep-list: compat input paths (`/etc/puppetlabs/...`), the `--puppet` feature, `NOTICE` attribution, ADRs, and the single README positioning sentence stay; everything else stale goes.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-parity-ledger-integrity`: both requirements REMOVED — the ledger generator no longer exists and the ledger is a frozen record summarized in `docs/HISTORY.md`.
- `go-port-ruby-removal`: "Historical records survive the removal" now points at `docs/HISTORY.md` + git history instead of requiring the frozen files in HEAD.
- `go-port-completion-verification`: the completed one-time milestones (ledger completion gate, release completion declaration) are REMOVED; the living requirements (verification matrix, benchmark evidence) stay, with the benchmark-record pointer moving from the migration log to the CHANGELOG/PR record.
- `go-port-ci-platform-gates`: the Windows gate is documented in `CONTRIBUTING.md` instead of `PORTING.md`.
- `go-port-distribution-and-cutover`: the completed "Ruby entry-point cutover" requirement is REMOVED (the `bin/facter` shim was already deleted by ADR-0008; gemspec dispositions are recorded in `docs/HISTORY.md`).

## Impact

- **Removed from HEAD**: ~11,100 lines of frozen documentation (recoverable at the cited commit).
- **Docs**: new `docs/HISTORY.md` and `CONTRIBUTING.md`; `CHANGELOG.md` trimmed; cross-references updated in `README.md`, `CONTEXT.md`, `docs/FACTER_CONF_COMPATIBILITY.md`, `docs/CUSTOM_FACT_MIGRATION.md` (any pointers to deleted files).
- **Specs**: five capability deltas as above; no code or behavior changes.
- **Dependencies**: pairs with the `facts-schema` change, which adds the schema `CONTRIBUTING.md` will reference; apply this change first.
