## ADDED Requirements

### Requirement: Documented custom-fact DSL compatibility contract
The Go port SHALL publish a compatibility contract that enumerates every supported Ruby custom-fact DSL construct and every known-unsupported construct with its migration path.

#### Scenario: Contract enumerates supported constructs
- **WHEN** a fact author reads the DSL contract document
- **THEN** it MUST list the supported constructs (literal setcode values including strings, numbers, booleans, nil, symbols, `%w` arrays, arrays, hashes, and `Time.utc`/`Date.new` literals; `Facter::Core::Execution.exec`/`execute` with literal command strings and `on_fail`/`timeout`/`expand`/`logger` options; direct command-string setcode; `Facter.value` and `ENV[]` references; non-block confines over strings, arrays, `%w`, symbols, regexes, ranges, integers, and booleans; trivial and simple-comparison block confines; `has_weight`/`weight:`; resolution options; `value:` options; aggregate facts with chunks, chunk dependencies, and aggregate blocks)

#### Scenario: Contract enumerates unsupported constructs with alternatives
- **WHEN** a fact author reads the DSL contract document
- **THEN** it MUST identify arbitrary Ruby code in `setcode` blocks, confine blocks beyond simple literal comparison, `on_flush` callbacks, `require` statements, Ruby `$LOAD_PATH`/gem-based fact discovery, and `.rb` files in external-fact directories as unsupported, and MUST state the recommended alternative for each (typically an executable external fact, `FACTERLIB`, or `--custom-dir`)

### Requirement: Load-time detection of unsupported DSL constructs
The custom-fact loader SHALL detect unsupported DSL constructs at load time and emit an actionable warning instead of failing silently.

#### Scenario: Unrecognized setcode body warns
- **WHEN** a custom fact file defines a fact whose `setcode` body matches no supported extraction pattern
- **THEN** the loader MUST emit a warning that names the source file, the fact name, and that the setcode body uses unsupported Ruby code, and MUST continue loading remaining facts from the file and directory

#### Scenario: Unsupported confine block warns and confines conservatively
- **WHEN** a custom fact resolution uses a confine block more complex than literal `true`/`false` or a single `==`/`!=` comparison against a literal
- **THEN** the loader MUST emit a warning naming the source file and fact, and MUST treat the resolution as not suitable rather than assuming the confine matches

#### Scenario: on_flush is detected and documented as inert
- **WHEN** a custom fact registers an `on_flush` callback
- **THEN** the loader MUST emit a warning that `on_flush` is not supported by the Go engine and MUST otherwise load the fact normally

#### Scenario: Ruby files in external fact directories are skipped with a warning
- **WHEN** an external-fact directory contains a `.rb` file
- **THEN** the loader MUST skip the file and emit a warning that Ruby external facts are not supported, naming the file

#### Scenario: Supported facts produce no false-positive diagnostics
- **WHEN** a custom fact uses only constructs listed as supported in the DSL contract
- **THEN** the loader MUST NOT emit an unsupported-construct warning for that fact

### Requirement: Fact-author migration guide
The Go port SHALL provide a migration guide that lets operators audit an existing custom-fact repository for Go-port compatibility before cutover.

#### Scenario: Migration guide content
- **WHEN** an operator prepares to migrate a fleet from Ruby facter to the Go port
- **THEN** the repository MUST contain a guide describing how to identify incompatible facts (including via the load-time warnings), how to rewrite common patterns as executable external facts, and how `FACTERLIB`/`--custom-dir` replace gem-based fact distribution
