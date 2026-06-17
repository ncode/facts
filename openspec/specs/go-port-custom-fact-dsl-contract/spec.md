# go-port-custom-fact-dsl-contract Specification

## Purpose
TBD - created by archiving change close-go-port-release-readiness-gaps. Update Purpose after archive.
## Requirements
### Requirement: No Ruby DSL is read anywhere
Facts SHALL NOT read `.rb` fact files from any source. The Ruby custom-fact DSL is outside the input contract per ADR-0006; external facts (structured data files, executables, environment variables) and programmatic registration are the only operator- and embedder-supplied fact sources.

#### Scenario: Ruby files in external-fact directories are skipped with a warning
- **WHEN** an external-fact directory contains a `.rb` file
- **THEN** the loader MUST skip the file and emit a warning that Ruby fact files are not supported, naming the file

#### Scenario: Removed custom-fact flags fail as unknown options
- **WHEN** the `facter` CLI is invoked with `--custom-dir`, `--no-ruby`, or `--no-custom-facts`
- **THEN** the CLI MUST exit with a usage error identifying the unknown option, exactly as for any unrecognized flag

#### Scenario: FACTERLIB has no effect
- **WHEN** the `FACTERLIB` environment variable points at a directory containing `.rb` fact files
- **THEN** discovery MUST NOT read the directory and the facts defined there MUST be absent from the output

#### Scenario: Retired facter.conf keys are inert
- **WHEN** `facter.conf` contains `custom-dir`, `no-ruby`, or `no-custom-facts` keys
- **THEN** the config MUST load without error and the keys MUST have no effect, identical to any other unrecognized key

### Requirement: Fact-author migration guide
The repository SHALL provide a migration guide stating that Facts reads no `.rb` fact files and mapping common Facter Ruby DSL patterns to their external-fact equivalents.

#### Scenario: Migration guide content
- **WHEN** an operator with existing Ruby custom facts evaluates Facts
- **THEN** `docs/CUSTOM_FACT_MIGRATION.md` MUST state that `.rb` fact files are not read, reference ADR-0006 for the rationale, and map at least: literal `setcode` values to structured-data external facts, command and `Facter::Core::Execution` `setcode` to executable external facts, `confine` to conditional logic inside the executable, and `weight` to single-source-of-truth external facts

