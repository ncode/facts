# Delta: go-port-custom-fact-dsl-contract

## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Fact-author migration guide
The repository SHALL provide a migration guide stating that Facts reads no `.rb` fact files and mapping common Facter Ruby DSL patterns to their external-fact equivalents.

#### Scenario: Migration guide content
- **WHEN** an operator with existing Ruby custom facts evaluates Facts
- **THEN** `docs/CUSTOM_FACT_MIGRATION.md` MUST state that `.rb` fact files are not read, reference ADR-0006 for the rationale, and map at least: literal `setcode` values to structured-data external facts, command and `Facter::Core::Execution` `setcode` to executable external facts, `confine` to conditional logic inside the executable, and `weight` to single-source-of-truth external facts

#### Scenario: Puppet plugin deviation is documented
- **WHEN** an operator reads the migration guide or the man page
- **THEN** the documented deviation that `facter --puppet` does not load Puppet Ruby plugin custom facts MUST appear there (formerly documented in the deleted DSL contract document)

## REMOVED Requirements

### Requirement: Documented custom-fact DSL compatibility contract
**Reason**: The Ruby DSL layer is removed entirely (ADR-0006); there is no supported-construct surface left to document.
**Migration**: `docs/CUSTOM_FACT_COMPATIBILITY.md` is deleted; the migration guide documents the no-Ruby stance and pattern mapping.

### Requirement: Load-time detection of unsupported DSL constructs
**Reason**: The custom-fact loader no longer exists; no `.rb` file is parsed, so there are no constructs to detect.
**Migration**: The only surviving `.rb` touchpoint is the external-fact directory skip-with-warning, now specified under "No Ruby DSL is read anywhere".
