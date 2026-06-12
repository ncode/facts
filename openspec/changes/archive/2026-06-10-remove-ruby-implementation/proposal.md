## Why

The Go port is complete: all four platform gates (Linux, macOS, Windows Server 2022/2025, FreeBSD) are green and blocking, release artifacts build from CI, and both porting changes are archived with their specs synced into `openspec/specs/`. The Ruby implementation was retained only until that point (the prior changes' explicit ground rule); it now adds ~9.3MB of dead code, misleading entry points, and obsolete packaging metadata to what is a Go product.

## What Changes

- **BREAKING (repo-level, not product-level):** delete the Ruby implementation and its support files: `lib/`, `spec/`, `spec_integration/`, `acceptance/`, `ext/`, `facter.gemspec`, `install.rb`, `Gemfile`, `Rakefile`, `.rspec`, `.rubocop.yml`, `.rubocop_todo.yml`. The Go binary's behavior does not change.
- Relocate the two byte-exact zpool fixtures that Go tests read from `spec/fixtures/` into `internal/facter/testdata/` and repoint `internal/facter/core_test.go`.
- Remove `TestVersion_matchesGemspecVersion` (it pins the version to the deleted gemspec; the version stays pinned by `TestVersionString_returnsPublicFacterVersion` and `TestFacterCommand_version`).
- Retire `tools/parity-ledger` and the `parity-ledger`/`parity-ledger-check` make targets: the ledger's input (the Ruby spec tree) is being deleted and its mission — proving port coverage — is complete and archived. `docs/PARITY_LEDGER.md` is frozen as a final historical record with a header note.
- Update `docs/MIGRATION.md` ground rules (Ruby-retention and ledger-regeneration rules are obsolete) and `PORTING.md` (compatibility-sources section references deleted spec paths).
- Keep: `bin/facter` (now a Go-only shim), the `--no-ruby` CLI compatibility flag and all other CLI compatibility behavior, the man page, and all `docs/` contracts and guides.

## Capabilities

### New Capabilities

- `go-port-ruby-removal`: The repository state after Ruby removal — what is deleted, what is preserved (historical records, fixtures the Go tests depend on, CLI compatibility surface), and the verification required.

### Modified Capabilities

- None. Existing capability requirements (fact parity, gates, distribution, DSL contract) are expressed against the Go implementation and are unaffected by deleting the Ruby tree; the parity ledger requirements in `go-port-parity-ledger-integrity` were verification rules for the now-completed porting effort and remain satisfied by the frozen final ledger.

## Impact

- Deleted trees: `lib/` (4.0MB), `spec/` (4.4MB), `spec_integration/` (44KB), `acceptance/` (820KB), `ext/` (8KB), plus root Ruby packaging/lint files.
- Affected Go tests: `internal/facter/core_test.go` (fixture paths), `facter_test.go` (gemspec test removed).
- Affected tooling: `tools/parity-ledger` removed, `Makefile` targets removed.
- Affected docs: `docs/MIGRATION.md`, `PORTING.md`, `docs/PARITY_LEDGER.md` (frozen header).
- Not affected: CLI behavior and flags, public Go API, CI workflows (none reference Ruby), `bin/facter`, release artifacts.
