## ADDED Requirements

### Requirement: Public API and CLI parity
The Go port SHALL preserve Ruby-compatible public API and CLI behavior for supported release use cases.

#### Scenario: Public API fact access
- **WHEN** callers use `ToHash`, `List`, `Each`, `Fact`, `Value`, `Lookup`, `Resolve`, `CoreValue`, `LoadFacts`, `Flush`, `Reset`, `Search`, `SearchExternal`, or programmatic custom fact registration
- **THEN** the Go port MUST match Ruby Facter behavior for fact naming, nested query resolution, nil values, custom/core/external precedence, caching, force-dot-resolution, and compatibility option handling

#### Scenario: CLI output and status behavior
- **WHEN** users run the Go `facter` CLI with no query, one query, multiple queries, `--json`, `--yaml`, `--hocon`, `--show-legacy`, `--no-show-legacy`, `--no-ruby`, `--strict`, `--config`, `--external-dir`, `--custom-dir`, logging flags, and compatibility short flags
- **THEN** stdout, stderr, exit status, option validation, missing-fact handling, legacy fact inclusion, query key preservation, and formatter output MUST match the corresponding Ruby behavior for in-scope features

### Requirement: Custom and external fact parity
The Go port SHALL support Ruby-compatible custom and external fact loading for supported-platform operation.

#### Scenario: Custom fact loading
- **WHEN** custom facts are loaded from registered directories or `facterlib` with literal `setcode`, block `setcode`, execution `setcode`, aggregate chunks, weights, confines, options, nil values, arrays, hashes, symbols, temporal values, raised resolvers, and invalid definitions
- **THEN** the Go port MUST match Ruby behavior for suitability, precedence, parsed values, diagnostics, skipped facts, aggregate resolution, timeouts, execution environment, and continued loading after errors

#### Scenario: External fact loading
- **WHEN** external facts are loaded from environment variables, text files, JSON files, YAML files, executable scripts, PowerShell scripts, configured paths, default paths, or blocked paths
- **THEN** the Go port MUST match Ruby behavior for name normalization, structured value normalization, parser diagnostics, executable stderr warnings, recursive-resolution guards, timeout handling, null-byte rejection, blocklist handling, and precedence over core/custom facts

### Requirement: Config, cache, query, formatter, and logging parity
The Go port SHALL preserve Ruby-compatible framework behavior around configuration, cache groups, fact filtering, query selection, formatting, and diagnostics.

#### Scenario: Configuration and cache behavior
- **WHEN** Facter reads config files, fact groups, TTLs, blocklists, default paths, invalid config, unreadable config, cache files, expired cache, corrupt cache, custom cache groups, and external fact cache groups
- **THEN** the Go port MUST match Ruby behavior for accepted syntax, warnings/errors, cache reads, cache writes, cache invalidation, permission failures, unsupported cache cases, and blocklist expansion

#### Scenario: Query and formatter behavior
- **WHEN** Facter formats selected or unselected facts as legacy text, JSON, YAML, or HOCON
- **THEN** the Go port MUST match Ruby behavior for nested fact selection, arrays, dotted fact names, nil rendering, scalar formatting, map ordering where specified, string quoting, IPv6/path handling, and collision diagnostics

#### Scenario: Logging and diagnostics
- **WHEN** framework code emits debug, info, warning, error, once-only, exception, HTTP debug, timing, strict missing-fact, parser, resolver, cache, custom fact, or external fact diagnostics
- **THEN** the Go port MUST match Ruby-compatible message text, severity, once-only semantics, and stderr/API handler routing for in-scope behaviors
