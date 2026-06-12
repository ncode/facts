# Delta: go-port-framework-parity

## ADDED Requirements

### Requirement: External fact parity
The Go port SHALL support Ruby-compatible external fact loading for supported-platform operation.

#### Scenario: External fact loading
- **WHEN** external facts are loaded from environment variables, text files, JSON files, YAML files, executable scripts, PowerShell scripts, configured paths, default paths, or blocked paths
- **THEN** the Go port MUST match Ruby behavior for name normalization, structured value normalization, parser diagnostics, executable stderr warnings, recursive-resolution guards, timeout handling, null-byte rejection, blocklist handling, and precedence over core and registered facts

## MODIFIED Requirements

### Requirement: Config, cache, query, formatter, and logging parity
The Go port SHALL preserve Ruby-compatible framework behavior around configuration, cache groups, fact filtering, query selection, formatting, and diagnostics.

#### Scenario: Configuration and cache behavior
- **WHEN** Facter reads config files, fact groups, TTLs, blocklists, default paths, invalid config, unreadable config, cache files, expired cache, corrupt cache, and external fact cache groups
- **THEN** the Go port MUST match Ruby behavior for accepted syntax, warnings/errors, cache reads, cache writes, cache invalidation, permission failures, unsupported cache cases, and blocklist expansion

#### Scenario: Query and formatter behavior
- **WHEN** Facter formats selected or unselected facts as legacy text, JSON, YAML, or HOCON
- **THEN** the Go port MUST match Ruby behavior for nested fact selection, arrays, dotted fact names, nil rendering, scalar formatting, map ordering where specified, string quoting, IPv6/path handling, and collision diagnostics

#### Scenario: Logging and diagnostics
- **WHEN** framework code emits debug, info, warning, error, once-only, exception, HTTP debug, timing, strict missing-fact, parser, resolver, cache, or external fact diagnostics
- **THEN** the Go port MUST match Ruby-compatible message text, severity, once-only semantics, and stderr routing for in-scope CLI behaviors

### Requirement: CLI parity
The `facter` CLI SHALL preserve Ruby-compatible output and status behavior for supported release use cases. Ruby compatibility is promised only at the CLI process boundary; the Go library API makes no Ruby-compatibility promises. The `--custom-dir`, `--no-ruby`, `--no-custom-facts`, and `--trace` flags are deliberately not part of the surface (ADR-0006; `--trace` only ever controlled Ruby custom-fact backtraces) and fail as unknown options.

#### Scenario: CLI output and status behavior
- **WHEN** users run the Go `facter` CLI with no query, one query, multiple queries, `--json`, `--yaml`, `--hocon`, `--show-legacy`, `--no-show-legacy`, `--strict`, `--config`, `--external-dir`, logging flags, and compatibility short flags
- **THEN** stdout, stderr, exit status, option validation, missing-fact handling, legacy fact inclusion, query key preservation, and formatter output MUST match the corresponding Ruby behavior for in-scope features

## REMOVED Requirements

### Requirement: Custom and external fact parity
**Reason**: Ruby DSL custom-fact loading is removed entirely (ADR-0006); custom-fact parity with Ruby facter is no longer a goal.
**Migration**: External-fact parity continues unchanged under the new "External fact parity" requirement; operators migrate `.rb` facts per `docs/CUSTOM_FACT_MIGRATION.md`.
