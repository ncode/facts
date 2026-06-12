## ADDED Requirements

### Requirement: Ruby implementation is fully removed
The repository SHALL contain no Ruby implementation, spec, acceptance, packaging, or lint files after this change.

#### Scenario: Ruby trees and files are deleted
- **WHEN** the change is applied
- **THEN** `lib/`, `spec/`, `spec_integration/`, `acceptance/`, and `ext/` MUST NOT exist, and `facter.gemspec`, `install.rb`, `Gemfile`, `Rakefile`, `.rspec`, `.rubocop.yml`, and `.rubocop_todo.yml` MUST NOT exist

#### Scenario: Go behavior is unchanged
- **WHEN** the full Go verification battery runs after removal
- **THEN** `go test ./...`, `go test -race ./...`, `go vet ./...`, and gofmt MUST pass, and all four platform CI gates MUST stay green on the removal commit

### Requirement: Go tests are self-contained after removal
No Go source or test SHALL reference a path under the deleted Ruby trees.

#### Scenario: Fixtures owned by Go testdata
- **WHEN** `internal/facter/core_test.go` exercises the zpool fixture parity tests
- **THEN** it MUST read the fixtures from `internal/facter/testdata/` with content byte-identical to the former `spec/fixtures/` files

#### Scenario: Version pinned without the gemspec
- **WHEN** the gemspec is removed
- **THEN** the version MUST remain pinned by Go-only tests exercising `facter.VersionString` and the CLI `--version` output

### Requirement: Historical records survive the removal
The porting verification record SHALL remain readable and truthful after its inputs are deleted.

#### Scenario: Parity ledger frozen
- **WHEN** `tools/parity-ledger` and the `parity-ledger`/`parity-ledger-check` make targets are removed alongside `spec/`
- **THEN** `docs/PARITY_LEDGER.md` MUST be retained with its final generated content and a header identifying it as a frozen historical record of the completed porting verification

#### Scenario: Migration log remains append-only
- **WHEN** ground rules in `docs/MIGRATION.md` that required keeping Ruby or regenerating the ledger become obsolete
- **THEN** the log MUST record their completion rather than silently deleting them, and `PORTING.md` MUST point to the frozen ledger and archived changes instead of deleted spec paths
