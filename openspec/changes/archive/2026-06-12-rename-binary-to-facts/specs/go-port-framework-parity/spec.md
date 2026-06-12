# Delta: go-port-framework-parity

## MODIFIED Requirements

### Requirement: CLI parity
The `facts` CLI SHALL preserve Ruby-compatible output and status behavior for supported release use cases. Ruby compatibility is promised only at the CLI process boundary; the Go library API makes no Ruby-compatibility promises. The binary is named `facts` with no `facter` alias (ADR-0008, superseding ADR-0004). The `--custom-dir`, `--no-ruby`, `--no-custom-facts`, `--trace`, `--show-legacy`, and `--no-show-legacy` flags are deliberately not part of the surface (ADR-0006 for the custom-fact flags; ADR-0007 for the legacy flags) and fail as unknown options.

#### Scenario: CLI output and status behavior
- **WHEN** users run the `facts` CLI with no query, one query, multiple queries, `--json`, `--yaml`, `--hocon`, `--strict`, `--config`, `--external-dir`, logging flags, and compatibility short flags
- **THEN** stdout, stderr, exit status, option validation, missing-fact handling, query key preservation, and formatter output MUST match the corresponding Ruby behavior for in-scope features, except that legacy alias facts are absent from every mode and the diagnostic program token is `Facts`

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
- **THEN** the Go port MUST match Ruby-compatible message text, severity, once-only semantics, and stderr routing for in-scope CLI behaviors, with the program name token rebranded from `Facter` to `Facts` (ADR-0008)
