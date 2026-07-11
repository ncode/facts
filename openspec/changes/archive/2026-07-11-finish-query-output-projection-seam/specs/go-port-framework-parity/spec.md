## MODIFIED Requirements

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
