# go-port-ruby-removal Specification

## Purpose
TBD - created by archiving change remove-ruby-implementation. Update Purpose after archive.
## Requirements
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


