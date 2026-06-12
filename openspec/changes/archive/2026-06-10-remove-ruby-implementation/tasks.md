## 1. Decouple Go from the Ruby tree

- [x] 1.1 `git mv spec/fixtures/zpool spec/fixtures/zpool-with-featureflags internal/facter/testdata/` and repoint the two reads in `internal/facter/core_test.go`; verify the fixture tests pass.
- [x] 1.2 Remove `TestVersion_matchesGemspecVersion` from `facter_test.go`; confirm `TestVersionString_returnsPublicFacterVersion` and the CLI version tests still pin the version.

## 2. Remove the Ruby implementation

- [x] 2.1 Delete `lib/`, `spec/`, `spec_integration/`, `acceptance/`, `ext/`, `facter.gemspec`, `install.rb`, `Gemfile`, `Rakefile`, `.rspec`, `.rubocop.yml`, `.rubocop_todo.yml`.
- [x] 2.2 Sweep for any remaining references to the deleted paths in Go sources, Makefile, workflows, and scripts; fix or remove each.

## 3. Retire the parity ledger tooling

- [x] 3.1 Remove `tools/parity-ledger/` and the `parity-ledger`/`parity-ledger-check` Makefile targets.
- [x] 3.2 Replace the `docs/PARITY_LEDGER.md` header with a frozen-historical-record note (final generation date and source commit) while keeping the final table content.

## 4. Update documentation

- [x] 4.1 Add a closing note to the obsolete `docs/MIGRATION.md` ground rules (Ruby retention, ledger regeneration) and record this change as a checkpoint.
- [x] 4.2 Update `PORTING.md`: drop the Ruby compatibility-sources and ledger-regeneration sections in favor of pointers to the frozen ledger and archived OpenSpec changes.

## 5. Verification

- [x] 5.1 Run `go test ./... -count=1`, `go test -race ./...`, `go vet ./...`, gofmt, and `git diff --check`; record results in the migration log checkpoint.
- [x] 5.2 Commit, push, and confirm all four platform gates and the auxiliary workflows are green on the removal commit.
