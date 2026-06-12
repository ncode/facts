## ADDED Requirements

### Requirement: CLI parity
The `facter` CLI SHALL preserve Ruby-compatible output and status behavior for supported release use cases. Ruby compatibility is promised only at the CLI process boundary; the Go library API makes no Ruby-compatibility promises.

#### Scenario: CLI output and status behavior
- **WHEN** users run the Go `facter` CLI with no query, one query, multiple queries, `--json`, `--yaml`, `--hocon`, `--show-legacy`, `--no-show-legacy`, `--no-ruby`, `--strict`, `--config`, `--external-dir`, `--custom-dir`, logging flags, and compatibility short flags
- **THEN** stdout, stderr, exit status, option validation, missing-fact handling, legacy fact inclusion, query key preservation, and formatter output MUST match the corresponding Ruby behavior for in-scope features

## REMOVED Requirements

### Requirement: Public API and CLI parity
**Reason**: The Ruby-compatible Go API (`ToHash`, `List`, `Each`, `Fact`, `Value`, `Lookup`, `Resolve`, `CoreValue`, `LoadFacts`, `Flush`, `Reset`, `Search`, `SearchExternal`, and package-global custom fact registration) is removed per ADR 0001 — Ruby compatibility is promised only at the `facter` CLI process boundary, and no external Go consumers of the old API exist.
**Migration**: CLI behavior parity continues under the new `CLI parity` requirement. Go consumers use the `facts` library API (see the `facts-library-api` capability): construct an Engine with `facts.New`, discover a Snapshot, and query it; programmatic custom facts register via the `WithFact` option.

## MODIFIED Requirements

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
- **THEN** the Go port MUST match Ruby-compatible message text, severity, once-only semantics, and stderr routing for in-scope CLI behaviors
