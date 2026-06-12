# Tasks: Remove the Ruby custom-fact DSL layer

## 1. Pre-removal survey

- [x] 1.1 Inventory every symbol defined in `internal/facter/custom.go` and `internal/facter/dsl_diagnostics.go`; grep each for uses outside those files and list the shared ones (do not delete anything shared with the external-fact path)
- [x] 1.2 Inventory engine/app/CLI tests that use `.rb` fixtures for non-DSL behavior (precedence ordering, logger wiring, directory normalization, engine isolation) and mark each for retargeting onto external-fact fixtures

## 2. CLI surface removal

- [x] 2.1 Remove `--custom-dir`, `--no-ruby`, and `--no-custom-facts` flag definitions, their mutual-conflict checks, and `FACTERLIB` handling from `internal/app/app.go` and `internal/cli/`
- [x] 2.2 Remove the removed flags from help text and option docs in `internal/app/app.go` (usage block and `--help` listing)
- [x] 2.3 Delete the `customDirPattern`, `noRubyPattern`, and `noCustomFactsPattern` extractions and the `CustomDirs`/`NoRuby`/`NoCustomFacts` fields from `internal/facter/config.go`
- [x] 2.4 Add negative test: `facter --custom-dir <dir>` (and `--no-ruby`, `--no-custom-facts`) exits with a usage error identifying the unknown option
- [x] 2.5 Add negative test: a `facter.conf` containing `custom-dir`, `no-ruby`, and `no-custom-facts` keys loads without error and the keys have no effect
- [x] 2.6 Add negative test: `FACTERLIB` pointing at a directory of `.rb` files has no effect on discovery
- [x] 2.7 Delete or retarget the app/CLI tests that exercised the removed flags (`TestRun_reportsConfiguredNoRubyCustomDirConflict`, `TestRun_rejectsNoCustomFactsWithCustomDir`, `TestRun_cliCustomDirOverridesConfiguredCustomDir`, `TestRun_queryCustomDirRubyFact`, etc. in `internal/app/app_test.go`)
- [x] 2.8 Remove the `--trace` flag and `cli.trace` config key (scope extension, 2026-06-11): its only function was Ruby custom-fact exception backtraces, so it goes with the layer — drop the flag definition, help/man text, `tracePattern`/`Trace` config parsing, and add it to the removed-flag negative tests

## 3. Library surface removal

- [x] 3.1 Remove `WithCustomDirs` from `engine.go`; remove custom-dir wiring (including `FACTERLIB` and default custom directories) from `WithSystemDefaults` and `WithConfigFile` handling
- [x] 3.2 Remove custom-fact loading and the weight-map plumbing from `internal/facter/engine.go` resolution order (external > registered > core remains)
- [x] 3.3 Retarget `engine_test.go` tests that used `.rb` fixtures for non-DSL behavior onto external-fact or `WithFact` fixtures (per the 1.2 inventory); delete tests that only exercised DSL parsing
- [x] 3.4 Verify a `.rb` file in an external-fact directory is still skipped with a warning naming the file (existing behavior, now the only `.rb` touchpoint — add a test if not already covered in `external_test.go`)

## 4. Parser deletion

- [x] 4.1 Delete `internal/facter/custom.go`, `internal/facter/dsl_diagnostics.go`, `internal/facter/custom_test.go`, and `internal/facter/dsl_diagnostics_test.go`
- [x] 4.2 Move any symbols identified as shared in 1.1 into the external-fact path (or a neutral file) before deletion; confirm the 44 external-fact tests pass unmodified
- [x] 4.3 Sweep for dangling references: `rtk grep -rn "LoadCustomFactsFromDirs\|WithCustomDirs\|customDir\|CustomDirs\|FACTERLIB\|no-ruby\|no-custom-facts" --include="*.go"` returns nothing outside external-dir `.rb`-skip handling

## 5. Documentation

- [x] 5.1 Delete `docs/CUSTOM_FACT_COMPATIBILITY.md`
- [x] 5.2 Rewrite `docs/CUSTOM_FACT_MIGRATION.md`: Facts reads no `.rb` fact files; link ADR-0006; pattern-mapping table (literal `setcode` → YAML/JSON external fact, command/`Facter::Core::Execution` `setcode` → executable external fact, `confine` → logic inside the executable, `weight` → single source of truth); include the `--puppet` plugin-facts deviation note
- [x] 5.3 Update the man page / `--help` man source: remove the three flags and `FACTERLIB`, add the `--puppet` deviation note
- [x] 5.4 Update README: remove the `--custom-dir` example (README.md:106), rewrite the Ruby-compat positioning sentence (README.md:23) and the docs pointer (README.md:142) to reflect the no-Ruby-DSL stance and the migration page
- [x] 5.5 Sweep remaining docs (`docs/MIGRATION.md`, `docs/FACTER_CONF_COMPATIBILITY.md`, `PORTING.md`) for references to the DSL contract, `--custom-dir`, `custom-dir` conf key, or "custom fact"; update wording to "registered fact"/external facts (do NOT touch `docs/PARITY_LEDGER.md` — frozen historical record)
- [x] 5.6 Add CHANGELOG breaking-change entry: Ruby custom-fact DSL removed; flags `--custom-dir`/`--no-ruby`/`--no-custom-facts` removed; `WithCustomDirs` removed; conf keys inert; migration guide pointer

## 6. Verification

- [x] 6.1 `rtk go test ./...` and `rtk go test -race ./...` pass; `go vet ./...` and gofmt clean
- [x] 6.2 External-fact and formatter/output-contract tests pass unmodified (proof the surviving contracts are untouched)
- [ ] 6.3 All platform CI gates green on the final commit
- [x] 6.4 `openspec status --change remove-ruby-custom-fact-dsl` artifacts complete; run validation if available before marking the change ready to archive
