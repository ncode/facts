## ADDED Requirements

### Requirement: CLI option vocabulary is shared

The `facts` CLI SHALL use one supported option vocabulary for validation, option metadata, help output, man output, and installed man page content.

#### Scenario: Accepted options are documented

- **WHEN** an option is accepted by `facts` validation and runtime handling
- **THEN** the option MUST be listed in generated help/man documentation unless it is explicitly marked hidden
- **AND** `--force-dot-resolution` MUST be documented while it remains accepted

#### Scenario: Unsupported options remain rejected

- **WHEN** validation reads an option outside the shared supported option vocabulary
- **THEN** validation MUST reject it before runtime execution

### Requirement: CLI option metadata preserves parser behavior

The shared CLI option metadata SHALL describe canonical names, aliases, value arity, repeatability, task flags, and conflicts without replacing the existing parser.

#### Scenario: Short aliases canonicalize consistently

- **WHEN** validation processes grouped short options such as `-jdtz`
- **THEN** each short alias MUST map to the same canonical option used by runtime handling

#### Scenario: Repeated and valued options are recognized consistently

- **WHEN** validation, group-listing logic, config path discovery, or external-dir discovery needs to know whether an option takes a value or can repeat
- **THEN** each caller MUST receive the same answer from the shared option metadata

#### Scenario: Documentation drift fails tests

- **WHEN** help text, man text, or the installed man page omits a non-hidden supported option
- **THEN** the CLI option contract tests MUST fail
