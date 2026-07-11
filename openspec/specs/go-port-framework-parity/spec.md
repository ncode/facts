# go-port-framework-parity Specification

## Purpose
TBD - created by archiving change complete-supported-platform-go-port. Update Purpose after archive.
## Requirements
### Requirement: External fact parity
The Go port SHALL support Ruby-compatible external fact loading for supported-platform operation, and SHALL centralize external-fact filesystem, environment, platform, and command execution behavior behind an explicit loader seam so CLI and library modes preserve their distinct error and source semantics without sentinel-driven control flow or package-global test hooks.

#### Scenario: External fact loading
- **WHEN** external facts are loaded from environment variables, text files, JSON files, YAML files, executable scripts, PowerShell scripts, configured paths, default paths, or blocked paths
- **THEN** the Go port MUST match Ruby behavior for name normalization, structured value normalization, parser diagnostics, executable stderr warnings, recursive-resolution guards, timeout handling, null-byte rejection, blocklist handling, and precedence over core and registered facts

#### Scenario: CLI and library loader modes are explicit
- **WHEN** external facts are resolved through the `facts` CLI and through a library Engine configured with explicit external directories or system defaults
- **THEN** both paths MUST use the external-fact loader seam while preserving their existing semantics for environment fact inclusion, executable/context failures, hard source errors, partial results, and joined errors

#### Scenario: External fact host behavior is injectable
- **WHEN** deterministic Go tests exercise external-fact directory walking, platform-specific executable handling, PowerShell execution, recursive-resolution guards, environment facts, unreadable files, or command output/stderr
- **THEN** they MUST be able to substitute a fake loader host instead of mutating package-global platform, command, file, or environment state

### Requirement: No legacy facts
Facts SHALL NOT expose legacy alias facts in any output mode. The canonical structured tree is the only fact surface; flat Ruby-era aliases (`operatingsystem`, `hostname`, `processorcount`, `sshfp_*`, `mtu_*`, …) do not resolve, whether unqueried, explicitly queried, or requested via removed flags.

#### Scenario: Legacy aliases are absent from all output

- **WHEN** the `facter` CLI runs with no query and any output format, or a library consumer discovers a Snapshot
- **THEN** the output MUST contain only structured facts; no legacy alias name appears at the top level

#### Scenario: Explicitly queried legacy aliases resolve nothing

- **WHEN** the `facter` CLI is invoked with a legacy alias query such as `facter operatingsystem`
- **THEN** the query MUST behave exactly like any other missing fact (empty output, exit 0; missing-fact error under `--strict`)

#### Scenario: Removed legacy flags fail as unknown options

- **WHEN** the `facter` CLI is invoked with `--show-legacy` or `--no-show-legacy`
- **THEN** the CLI MUST exit with a usage error identifying the unknown option, exactly as for any unrecognized flag

#### Scenario: Retired show-legacy config key is inert

- **WHEN** `facter.conf` contains a `show-legacy` key
- **THEN** the config MUST load without error and the key MUST have no effect, identical to any other unrecognized key

#### Scenario: Legacy blocklist group has no effect

- **WHEN** `facter.conf` contains `blocklist : [ "legacy" ]`
- **THEN** the config MUST load without error and discovery output MUST be identical to a run without the entry

### Requirement: Config, cache, query, formatter, and logging parity
The Go port SHALL preserve Ruby-compatible framework behavior around configuration, cache groups, fact filtering, query selection, formatting, and diagnostics. Selected-query projection, dotted fact mode, selected-query values, nil rendering, presentation names, and strict missing-fact detection SHALL be centralized behind one internal projection module. Normal CLI formatter, timing-name, and strict-mode paths MUST consume one presentation Projection rather than receiving raw resolved-fact records and rebuilding those rules independently.

#### Scenario: Configuration and cache behavior
- **WHEN** Facter reads config files, fact groups, TTLs, blocklists, default paths, invalid config, unreadable config, cache files, expired cache, corrupt cache, and external fact cache groups
- **THEN** the Go port MUST match Ruby behavior for accepted syntax, warnings/errors, cache reads, cache writes, cache invalidation, permission failures, unsupported cache cases, and blocklist expansion

#### Scenario: Query and formatter behavior
- **WHEN** Facter formats selected or unselected facts as legacy text, JSON, YAML, or HOCON
- **THEN** the Go port MUST match Ruby behavior for nested fact selection, arrays, dotted fact names, nil rendering, scalar formatting, map ordering where specified, string quoting, IPv6/path handling, and collision diagnostics
- **AND** selected-query projection, dotted fact mode, selected-query value maps, and strict missing-query detection MUST be provided by the internal projection module rather than duplicated across CLI and formatter paths
- **AND** formatter adapters MUST consume shape and selected values from a presentation Projection rather than reconstructing a Projection from raw Snapshot records
- **AND** each formatter MAY retain the format-specific scalar, map, ordering, quoting, and legacy transformation rules required for byte compatibility

#### Scenario: Normal CLI presentation reuses one projection
- **WHEN** the CLI completes normal discovery and then formats output, emits timing names, or classifies missing queries for strict mode
- **THEN** those paths MUST consume one defensive presentation Projection configured with the effective dotted-fact mode
- **AND** timing rendering, output routing, strict diagnostics, and strict exit status MUST remain owned by the CLI adapter

#### Scenario: Strict missing-query classification keeps CLI semantics
- **WHEN** the presentation Projection classifies selected records for strict mode
- **THEN** a selected registered or external fact resolved to nil MUST be reported missing even though public Snapshot lookup treats that resolved nil as found
- **AND** missing query names MUST retain selection order and duplicates
- **AND** full-output records with an empty `UserQuery` MUST NOT be reported missing

#### Scenario: Presentation names retain record order
- **WHEN** timing output asks the presentation Projection for display names
- **THEN** it MUST return one name for every selected record in original order, including duplicate queries
- **AND** each name MUST use `UserQuery` when non-empty and otherwise fall back to the resolved fact `Name`

#### Scenario: Presentation shape retains formatter semantics
- **WHEN** a formatter receives an empty selection, full-output records, one distinct selected query, repeated copies of one query, or multiple distinct queries
- **THEN** Projection MUST classify them respectively as empty, full-tree, scalar, scalar, or query-map output
- **AND** every formatter MUST preserve its existing empty-output, scalar, nil, and container bytes

#### Scenario: Projection lifetimes remain distinct
- **WHEN** discovery selects queries, a Snapshot serves canonical lookup, the CLI applies force-dot presentation, or the version-query fast path renders its synthetic fact
- **THEN** each path MUST use a Projection with the lifetime and dotted-fact mode required by that path
- **AND** the implementation MUST NOT reuse a force-dot presentation tree as the canonical Snapshot tree

#### Scenario: Logging and diagnostics
- **WHEN** framework code emits debug, info, warning, error, once-only, exception, HTTP debug, timing, strict missing-fact, parser, resolver, cache, or external fact diagnostics
- **THEN** the Go port MUST match Ruby-compatible message text, severity, once-only semantics, and stderr routing for in-scope CLI behaviors, with the program name token rebranded from `Facter` to `Facts` (ADR-0008)

### Requirement: CLI parity
The `facts` CLI SHALL preserve Ruby-compatible output and status behavior for supported release use cases. Ruby compatibility is promised only at the CLI process boundary; the Go library API makes no Ruby-compatibility promises. The binary is named `facts` with no `facter` alias (ADR-0008, superseding ADR-0004). The `--custom-dir`, `--no-ruby`, `--no-custom-facts`, `--trace`, `--show-legacy`, and `--no-show-legacy` flags are deliberately not part of the surface (ADR-0006 for the custom-fact flags; ADR-0007 for the legacy flags) and fail as unknown options.

#### Scenario: CLI output and status behavior
- **WHEN** users run the `facts` CLI with no query, one query, multiple queries, `--json`, `--yaml`, `--hocon`, `--strict`, `--config`, `--external-dir`, logging flags, and compatibility short flags
- **THEN** stdout, stderr, exit status, option validation, missing-fact handling, query key preservation, and formatter output MUST match the corresponding Ruby behavior for in-scope features, except that legacy alias facts are absent from every mode and the diagnostic program token is `Facts`

### Requirement: Depth-colored keys in the default format
The `facts` CLI SHALL colorize keys in the default text format according to their nesting depth, cycling a fixed ANSI palette per level; values SHALL remain uncolored. Color SHALL be enabled by default when standard output is a terminal and disabled otherwise; `--color` forces it on and `--no-color` disables it. This is a Facts extension: Ruby Facter's `--color` affects diagnostics only. Machine formats are never colorized.

#### Scenario: Keys are colored by depth
- **WHEN** the default text format renders with color in effect (terminal output, or `--color` given)
- **THEN** every key MUST be wrapped in the ANSI color assigned to its nesting depth (top-level keys depth 0, their children depth 1, and so on, cycling the palette), and values MUST carry no color codes

#### Scenario: Piped output is clean by default
- **WHEN** `facts` runs without `--color` and standard output is not a terminal (piped or redirected)
- **THEN** the default text format MUST contain no ANSI escape sequences

#### Scenario: --no-color always disables
- **WHEN** `facts --no-color` runs, regardless of whether output is a terminal
- **THEN** the default text format MUST contain no ANSI escape sequences

#### Scenario: --color forces color for non-terminal output
- **WHEN** `facts --color` runs with output piped or redirected
- **THEN** keys in the default text format MUST carry their depth colors

#### Scenario: Machine formats are never colorized
- **WHEN** `facts` runs with `--json`, `--yaml`, or `--hocon`, with or without `--color`
- **THEN** the formatted output MUST be byte-identical regardless of color settings

### Requirement: Framework security boundaries
The Go port SHALL bound untrusted framework inputs and SHALL render machine formats with syntactically safe keys.

#### Scenario: Cache groups stay within the cache directory
- **WHEN** fact cache TTL configuration or custom fact groups contain absolute paths, path traversal, separators, drive prefixes, empty names, or dot-only names
- **THEN** cache reads, writes, freshness checks, and invalidation MUST ignore those groups and MUST NOT read, write, or delete files outside the configured cache directory

#### Scenario: External fact sources are bounded
- **WHEN** static external fact files, executable external fact stdout, or executable external fact stderr exceed the external fact byte limit
- **THEN** discovery MUST stop reading that source and report an oversized external fact failure through the same CLI/library error policy used for that source kind

#### Scenario: YAML and HOCON keys are escaped
- **WHEN** selected or unselected fact output contains a map key that is unsafe in YAML or HOCON plain-key syntax
- **THEN** the YAML and HOCON formatters MUST quote the key so the output remains parseable data rather than injected syntax

#### Scenario: Built-in probes ignore untrusted PATH entries
- **WHEN** core fact resolution runs built-in host probe commands
- **THEN** those commands MUST be resolved from a fixed platform system search path and MUST receive a sanitized `PATH` that excludes caller-provided path entries while preserving unrelated environment variables

#### Scenario: Filesystem byte math clamps overflow
- **WHEN** platform statfs data reports block counts or block sizes whose product exceeds the Go `int` range
- **THEN** mountpoint byte totals MUST clamp to the largest representable `int` before formatting facts

