# facts-cli-option-contract Specification

## Purpose
Define the internal contract that keeps accepted `facts` CLI options, parser metadata, help output, man output, and installed documentation in sync.
## Requirements
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

### Requirement: Version fast path reuses engine-owned seams

The CLI's version-query fast path SHALL derive its disabled-fact set from the engine's exported pure disabled-union function — the same union semantics the engine's discovery planning applies — instead of re-implementing the union in `internal/app`, and SHALL render its output through the engine's formatter-selection seam (`BuildFormatter`) instead of a CLI-local re-derivation of format precedence. The fast-path decision itself and formatter selection remain owned by `internal/app` per the discovery-input-surface design. The engine SHALL NOT export helpers whose only purpose is to feed a CLI-side re-implementation of engine policy.

#### Scenario: Fast-path disabled set matches discovery semantics

- **WHEN** `facts facterversion` runs with any combination of `--disable`, the `FACTS_DISABLE` environment variable, and a config-file disable list
- **THEN** the fast path takes effect exactly when a full discovery would omit `facterversion` for the same inputs, because both derive the disabled set from the same engine union

#### Scenario: Disabled facterversion falls through identically

- **WHEN** `facterversion` is disabled by any disable source and queried in the default format
- **THEN** stdout, stderr diagnostics, and exit status are byte-identical to the behavior before the fast path consumed the engine union

#### Scenario: Version output is format-stable

- **WHEN** `facts facterversion` is rendered with `--json`, `--yaml`, `--hocon`, or the default format
- **THEN** the bytes written to stdout are identical to the previous hand-selected formatter output for each format

