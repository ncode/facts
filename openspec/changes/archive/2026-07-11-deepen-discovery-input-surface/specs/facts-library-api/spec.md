## ADDED Requirements

### Requirement: Discovery uses one input plan per run
The library SHALL derive source loading, blocklist, cache, and query-selection policy from one internal discovery plan for each `Discover` call. The plan MUST be recomputed per discovery so config files, external fact directories, environment facts, executable facts, and cache contents remain fresh across repeated discovery on the same immutable Engine.

#### Scenario: Config is read at discovery time
- **WHEN** an Engine configured with `WithConfigFile` discovers facts, the config file changes, and the same Engine discovers facts again
- **THEN** the second Snapshot reflects the updated config-derived external dirs, blocklists, and cache TTL/group policy

#### Scenario: Query selection happens in discovery
- **WHEN** a consumer calls `Discover(ctx, "os.family")`
- **THEN** the returned Snapshot is backed by facts selected with the same projection semantics used by CLI query projection where the contracts overlap
- **AND** the public `Discover(ctx, queries...)` method shape remains unchanged

#### Scenario: Cache policy stays discovery-scoped
- **WHEN** an Engine is configured with cache enabled and config-derived TTL/group policy
- **THEN** discovery applies cache resolution and cache refresh according to the per-discovery plan
- **AND** the Engine does not memoize resolved fact values between discoveries

#### Scenario: Force-dot resolution is not public library configuration
- **WHEN** a library consumer constructs an Engine through public `facts` options
- **THEN** no public option exists for force-dot resolution
- **AND** the canonical Snapshot tree preserves existing dotted external and registered fact behavior
