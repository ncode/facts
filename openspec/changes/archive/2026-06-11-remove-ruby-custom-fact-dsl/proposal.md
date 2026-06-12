# Remove the Ruby custom-fact DSL layer

## Why

The Ruby custom-fact DSL support is a ~4,900-line regex-based static parser (~17% of `internal/facter/`) that pattern-matches a documented subset of a foreign language — the most fragile surface in the engine. It serves only hypothetical standalone `--custom-dir`/`FACTERLIB` users: Puppet plugin-synced facts (where real-world `.rb` facts live) were already out of scope, and the project is unreleased with no known deployments. ADR-0006 records the decision: remove the layer now, while the contract amendment is free, and let external facts (data files, executables, env vars) plus `facter.conf` be the whole input contract.

## What Changes

- **BREAKING** Delete the Ruby DSL parser and diagnostics (`internal/facter/custom.go`, `internal/facter/dsl_diagnostics.go`, and their tests). No `.rb` file is read anywhere; facts defined in `.rb` files no longer resolve.
- **BREAKING** Drop the `--custom-dir`, `--no-ruby`, and `--no-custom-facts` CLI flags, their conflict checks, `FACTERLIB` handling, and the matching help/man text. Invocations passing these flags now fail with a usage error.
- **BREAKING** Drop the `--trace` flag and the `cli.trace` config key with the layer: its only function was Ruby exception backtraces from custom-fact code, so with no Ruby evaluation it could never do anything, and accepted no-op flags misrepresent what the binary does (same rationale as ADR-0006's rejection of no-op `--no-ruby`).
- **BREAKING** Remove `WithCustomDirs` from the public Go API. `WithFact` (programmatic registration) is unaffected.
- The `facter.conf` keys `custom-dir`, `no-ruby`, and `no-custom-facts` become inert, like any other unrecognized key (the config parser already silently ignores unknown keys).
- Retire the term "custom fact": programmatic registrations are "registered facts" (CONTEXT.md already updated). Fact categories are core / external / registered.
- Delete `docs/CUSTOM_FACT_COMPATIBILITY.md`; rewrite `docs/CUSTOM_FACT_MIGRATION.md` as the single page stating that Facts reads no `.rb` fact files and mapping common `Facter.add` patterns to external-fact equivalents.
- `facter --puppet` keeps warning that Puppet Ruby plugin facts are not loaded, but the deviation is documented in the migration guide and man page instead of the deleted DSL contract.
- Output contract and the remaining input contract (external facts, `facter.conf`) are byte-for-byte unchanged.

## Capabilities

### New Capabilities

(none — the new no-Ruby requirements land as the replacement requirement set of the existing `go-port-custom-fact-dsl-contract` capability)

### Modified Capabilities

- `go-port-custom-fact-dsl-contract`: All three existing requirements (documented DSL compatibility contract, load-time detection of unsupported constructs, migration guide for the supported subset) are removed. Replaced by: Facts reads no `.rb` fact files anywhere, and a migration guide maps `Facter.add` patterns to external-fact equivalents.
- `facts-library-api`: The explicit opt-in scenario loses `WithCustomDirs` and DSL parsing/weight/confine semantics; "custom fact" wording in decode, error-semantics, and diagnostics scenarios becomes "registered fact" with external-fact examples where directory loading is implied.
- `go-port-framework-parity`: The custom-fact half of "Custom and external fact parity" is removed (external-fact parity stays); the CLI parity flag list drops `--no-ruby` and `--custom-dir`; custom-fact diagnostics leave the logging parity scenario.
- `go-port-framework-fidelity`: The `--puppet` warning requirement now points its documentation obligation at the migration guide and man page instead of the DSL contract document.

## Impact

- **Code removed**: `internal/facter/custom.go` (2,102 lines), `internal/facter/dsl_diagnostics.go`, `custom_test.go`, `dsl_diagnostics_test.go`; `WithCustomDirs` in `engine.go` and its tests; flag parsing, conflict checks, and help text in `internal/app/app.go` and `internal/cli/`; `custom-dir`/`no-ruby`/`no-custom-facts` patterns in `internal/facter/config.go`; custom-fact weight/confine plumbing in `internal/facter/engine.go`.
- **Docs**: `docs/CUSTOM_FACT_COMPATIBILITY.md` deleted; `docs/CUSTOM_FACT_MIGRATION.md` rewritten; README `--custom-dir` example and Ruby-compat positioning sentence updated; man page regenerated without the removed flags; CHANGELOG breaking-change entry. `docs/PARITY_LEDGER.md` is a frozen historical record (per `go-port-ruby-removal`) and is NOT rewritten. ADR-0006 and the CONTEXT.md glossary updates are already committed to the working tree.
- **Behavioral**: scripts passing removed flags get usage errors; `.rb` files anywhere are simply not read (external-dir `.rb` skip behavior unchanged — those files were never read); everything else (external facts, config, caching, formatting, output) is unchanged.
- **Dependencies**: none added or removed.
