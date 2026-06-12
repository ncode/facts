## ADDED Requirements

### Requirement: Coverage references are machine-verified
The parity ledger generator SHALL verify that every coverage reference resolves to real Go tests before marking a spec covered.

#### Scenario: Named test functions must exist
- **WHEN** the ledger generator processes a migration-log coverage reference containing a `go test -run` pattern
- **THEN** it MUST verify that each test name prefix in the pattern matches at least one `func Test...` declaration in the repository's `*_test.go` files, and MUST mark the entry as a failed reference rather than covered when a name matches nothing

#### Scenario: Blanket coverage is a distinct disposition
- **WHEN** a migration-log coverage reference runs an entire package tree without a `-run` filter (for example `go test ./... -count=1`)
- **THEN** the ledger MUST record the entry with a `blanket-coverage` disposition distinct from `covered-by-existing-go-test`, and `parity-ledger-check` MUST fail while any in-scope entry remains `blanket-coverage` without an explicit documented waiver in the migration log

#### Scenario: Known mismapped entries are corrected
- **WHEN** the ledger is regenerated after this change
- **THEN** previously mismapped entries (including `spec/facter/resolvers/freebsd/freebsd_version_spec.rb`, whose reference pointed at networking tests) MUST reference tests that exercise the spec's actual domain

### Requirement: Every spec file has an explicit scoping decision
The parity ledger SHALL account for every `*_spec.rb` file in the repository so that exclusions are explicit rather than silent.

#### Scenario: Full inventory accounting
- **WHEN** the ledger is generated
- **THEN** every `*_spec.rb` file under `spec/` and `spec_integration/` MUST appear in exactly one bucket — in-scope (with a disposition), out-of-scope (with the matching exclusion rule, such as an unsupported platform), or `unclassified` — and the summary MUST report the count of each bucket

#### Scenario: Unclassified specs block the check
- **WHEN** `parity-ledger-check` runs and any spec file is `unclassified`
- **THEN** the check MUST fail and name the unclassified files so the classifier or the scoping rules are updated explicitly
