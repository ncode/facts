# facts-cli-option-contract Specification

## Purpose
Define the internal contract that keeps accepted `facts` CLI options, parser metadata, help output, man output, and installed documentation in sync.
## Requirements
### Requirement: CLI option vocabulary is shared

The `facts` CLI SHALL use one supported option vocabulary for validation, option metadata, help output, man output, installed man page content, and runtime parsing. The runtime parser's flag set SHALL be derived from the shared option metadata rather than declared independently, so an option cannot be accepted by validation and unknown to the parser (or vice versa).

#### Scenario: Accepted options are documented

- **WHEN** an option is accepted by `facts` validation and runtime handling
- **THEN** the option MUST be listed in generated help/man documentation unless it is explicitly marked hidden
- **AND** `--force-dot-resolution` MUST be documented while it remains accepted

#### Scenario: Unsupported options remain rejected

- **WHEN** validation reads an option outside the shared supported option vocabulary
- **THEN** validation MUST reject it before runtime execution

#### Scenario: Parser flag set derives from the shared vocabulary

- **WHEN** a non-task option exists in the shared option metadata
- **THEN** the runtime parser MUST accept it (with its aliases, arity, and repeatability) without a second hand-written declaration
- **AND** a mismatch between the shared metadata and the parser's accepted flag set MUST fail tests

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

The CLI's version-query fast path SHALL derive its disabled-fact set from the engine's exported pure disabled-union function — the same union semantics the engine's discovery planning applies — and SHALL consume the engine's resolved external-fact-directory planning (CLI dirs, then config dirs, then process defaults) instead of re-deriving that resolution in `internal/app`. It SHALL render its output through the engine's formatter-selection seam (`BuildFormatter`) instead of a CLI-local re-derivation of format precedence. The fast-path decision itself and formatter selection remain owned by `internal/app` per the discovery-input-surface design. The engine SHALL NOT export helpers whose only purpose is to feed a CLI-side re-implementation of engine policy.

#### Scenario: Fast-path disabled set matches discovery semantics

- **WHEN** `facts facterversion` runs with any combination of `--disable`, the `FACTS_DISABLE` environment variable, and a config-file disable list
- **THEN** the fast path takes effect exactly when a full discovery would omit `facterversion` for the same inputs, because both derive the disabled set from the same engine union

#### Scenario: Fast-path external-dir gate matches discovery planning

- **WHEN** external-fact directories are supplied by `--external-dir`, by config, or by process defaults, with or without `--no-external-facts`
- **THEN** the fast path declines exactly when a full discovery would load external facts for the same inputs, because the gate consumes the engine's resolved planning rather than a CLI-side copy of the resolution order

#### Scenario: Disabled facterversion falls through identically

- **WHEN** `facterversion` is disabled by any disable source and queried in the default format
- **THEN** stdout, stderr diagnostics, and exit status are byte-identical to the behavior before the fast path consumed the engine union

#### Scenario: Version output is format-stable

- **WHEN** `facts facterversion` is rendered with `--json`, `--yaml`, `--hocon`, or the default format
- **THEN** the bytes written to stdout are identical to the previous hand-selected formatter output for each format

#### Scenario: Color does not change fast-path bytes

- **WHEN** `facts --color facterversion` runs
- **THEN** stdout bytes are identical to `facts facterversion` (the fast path renders the bare version scalar uncolored, as today)

### Requirement: The --disable option is part of the shared option vocabulary

The `facts` CLI SHALL accept `--disable` as a valued, comma-separated, repeatable option in the shared option vocabulary, documented like any other non-hidden option, contributing fact and group names to the disabled set.

#### Scenario: --disable is accepted and documented

- **WHEN** `--disable packages,os` is parsed
- **THEN** validation MUST accept it as a valued option contributing `packages` and `os` to the disabled set
- **AND** `--disable` MUST appear in generated help and man output

#### Scenario: --disable composes with --no-block

- **WHEN** both `--disable packages` and `--no-block` are given
- **THEN** `--no-block` MUST clear the disabled set so nothing is disabled

### Requirement: Group-listing tasks parse options through the shared intake

The `--list-block-groups` and `--list-cache-groups` tasks SHALL obtain option values through the shared registry-driven intake — the option vocabulary, aliases, and value arity come from the same option metadata the query task's parser derives from, with no independently-maintained option knowledge. The list-task intake preserves the historical permissive scan: it walks the entire argument tail, skipping positional tokens, `--`, and additional task flags, and honors `--config`/`--external-dir` wherever they appear. Option semantics are unchanged: only `--config` and `--external-dir` affect group listing; every other accepted option remains inert on these tasks.

#### Scenario: Config and external dirs reach group listing via the shared intake

- **WHEN** `facts --list-cache-groups -c PATH --external-dir DIR` runs (including `=`-attached and short-alias spellings, and with other valued options interleaved)
- **THEN** group listing MUST honor exactly the config file and external directories the shared intake parsed
- **AND** an interleaved valued option (such as `-l debug`) MUST NOT have its value misread as a different option or as a query

#### Scenario: List tasks tolerate positionals, delimiters, and extra task flags

- **WHEN** `--config` or `--external-dir` appears after a positional token, after `--`, or alongside an additional task flag (e.g. `facts --list-cache-groups bogus --config PATH`, `facts --list-cache-groups --version`)
- **THEN** group listing MUST still honor the option, ignore the stray tokens and extra task flags, and exit 0 — matching the historical permissive scan

#### Scenario: Other options stay inert on list tasks

- **WHEN** `facts --list-cache-groups --no-external-facts` (or any other accepted non-config, non-external-dir option) runs
- **THEN** the group listing output MUST be byte-identical to the same invocation without that option

### Requirement: Option error rendering is stable at the process edge

Validation-time option errors SHALL render on stderr with the `ERROR Facts::OptionsValidator - ` prefix and exit status 1. Runtime option-interplay errors (conflicts detectable only after config parsing, such as `--no-external-facts` combined with `--external-dir`, or a config-file log-level conflict) SHALL render as plain error lines without the OptionsValidator prefix, byte-identical to current behavior.

#### Scenario: Validation errors carry the OptionsValidator prefix

- **WHEN** the binary runs with an unknown or conflicting option rejected by validation (such as `-z`)
- **THEN** stderr MUST begin `ERROR Facts::OptionsValidator - ` and the process MUST exit 1

#### Scenario: Runtime option-interplay errors render plainly

- **WHEN** the binary runs with an option combination rejected after config parsing (such as `--no-external-facts --external-dir DIR`)
- **THEN** the error line MUST render without the OptionsValidator prefix, byte-identical to today's output, and the process MUST exit 1
