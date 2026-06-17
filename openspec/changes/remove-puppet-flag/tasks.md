# Tasks: Remove the `--puppet` CLI flag

## 1. Pre-removal survey

- [ ] 1.1 `rtk grep -rn "puppet\|Puppet" --include="*.go"` and confirm every hit outside `internal/engine/config.go` (Facter default-dir path strings, kept) and the `puppetversion` core-fact resolver belongs to the `--puppet` flag path and is safe to delete
- [ ] 1.2 Confirm `EngineConfig.Puppet` is read by no engine code (only set in `internal/app/app.go:359`) before deleting the field

## 2. CLI surface removal

- [ ] 2.1 Remove the `--puppet`, `-p` (`puppetShort`), and `--no-puppet` flag definitions and the duplicate-`--puppet` check from `internal/app/app.go` (273-274, 281, 288-290)
- [ ] 2.2 Remove the pluginfactdest append + `WarnPuppetRubyPluginFacts` call (`internal/app/app.go:345-348`) and the `puppet` parameter from `canUseVersionQueryFastPath` (340, 522-530)
- [ ] 2.3 Remove `--puppet`/`-p`/`--no-puppet` from `internal/cli/validation.go`: the `knownOption` entries (172), the `-p` alias in `shortOptionAlias` (141-142), and the now-unreachable `--puppet`/`-p` ↔ `--no-puppet` conflict rows (87-88)
- [ ] 2.4 Remove `--puppet`/`-p` from the usage block and `--help`/option docs in `internal/app/app.go` (150, 163, 194, 207)
- [ ] 2.5 Add negative test: `facts --puppet` (and `-p`, `--no-puppet`) exits with `unrecognised option '...'`, matching any unknown flag

## 3. Library surface removal

- [ ] 3.1 Delete `internal/engine/puppet.go` in full and `internal/engine/puppet_test.go` if it exists
- [ ] 3.2 Remove the `Puppet bool` field and its doc comment from `EngineConfig` (`internal/engine/engine.go:58-59`) and the `Puppet:` assignment in `internal/app/app.go:359`
- [ ] 3.3 Sweep for dangling references: `rtk grep -rn "PuppetPluginFactDirs\|WarnPuppetRubyPluginFacts\|defaultPuppetCacheDir\|puppetCacheDirFn\|\.Puppet\b\|Puppet:" --include="*.go"` returns nothing outside the kept `config.go` path strings
- [ ] 3.4 Confirm the general external-dir `.rb` skip-with-warning still passes unchanged (`internal/app/contract_test.go:301`, `TestRun_warnsAndSkipsRubyExternalFact`) — it is now the only `.rb` warning touchpoint

## 4. Tests for removed flags

- [ ] 4.1 Delete or retarget the app/CLI tests exercising `--puppet`: `internal/app/app_test.go` (puppet plugin-fact and `--puppet`/`--no-puppet`/`-p` conflict tests ~617-667, 1554-1585), `internal/cli/arguments_test.go:76-81`, `internal/cli/validation_test.go:58-62`
- [ ] 4.2 Keep `internal/app/contract_test.go:301` (general `.rb` warning — not puppet-coupled) and the `go-port-supported-platform-facts` puppet-package scenarios

## 5. Documentation

- [ ] 5.1 Delete the "Deviation: `facts --puppet` and Ruby plugin facts" section from `docs/CUSTOM_FACT_MIGRATION.md` (75-86); optionally add half a line to the top section noting Puppet-synced `.rb` (`vardir/lib/facter`) is covered by "no `.rb` anywhere"
- [ ] 5.2 Regenerate the man page / `--help` man source without `--puppet`/`-p`/`--no-puppet`
- [ ] 5.3 Update README and `doc.go` if they name `--puppet`
- [ ] 5.4 Write `docs/adr/0009-facter-contract-not-puppet-runtime.md`: Facts implements Facter's input/output contract, not Puppet's runtime; Facter dirs in (incl. `puppetlabs/facter/facts.d` defaults), Puppet agent-runtime surface (`puppet/cache/facts.d` pluginfactdest, pluginsync, `.rb` plugin loading) out; builds on ADR-0006 and ADR-0001
- [ ] 5.5 Add CHANGELOG breaking-change entry: `--puppet`/`-p`/`--no-puppet` removed; migration line `--external-dir /opt/puppetlabs/puppet/cache/facts.d` (platform/user equivalent); `EngineConfig.Puppet` removed
- [ ] 5.6 Confirm the `CONTEXT.md` Input-contract glossary update is in the working tree (already done during design)

## 6. Verification

- [ ] 6.1 `rtk go test ./...` and `rtk go test -race ./...` pass; `go vet ./...` and gofmt clean
- [ ] 6.2 External-fact and formatter/output-contract tests pass unmodified (proof the surviving contracts are untouched)
- [ ] 6.3 `openspec validate remove-puppet-flag --strict` passes; archive after merge and sync the three spec deltas to `openspec/specs/`
- [ ] 6.4 All platform CI gates green on the final commit
