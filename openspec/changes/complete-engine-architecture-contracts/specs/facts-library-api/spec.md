## MODIFIED Requirements

### Requirement: Immutable engine with explicit snapshot discovery
An Engine SHALL be immutable after construction, with all fact registrations and configuration fixed at `New`. Resolution SHALL happen only through `Discover(ctx)`, which returns an immutable Snapshot of the canonical tree. The library SHALL hold no package-global mutable state, SHALL capture ambient system-following defaults in an invocation-local value, and SHALL keep Engines and Snapshots safe for concurrent use.

#### Scenario: Discovery honors context cancellation
- **WHEN** the context passed to `Discover` is cancelled or exceeds its deadline while resolvers (including command execution and cloud-metadata requests) are running
- **THEN** `Discover` returns promptly with an error satisfying `errors.Is(err, ctx.Err())`

#### Scenario: Freshness requires re-discovery
- **WHEN** system state changes after a Snapshot was discovered
- **THEN** the existing Snapshot is unchanged, and a new `Discover` call returns a new Snapshot reflecting the new state

#### Scenario: Engines are isolated
- **WHEN** two Engines with different configurations or invocation-local defaults discover concurrently in one process
- **THEN** each Snapshot reflects only its own Engine's configuration and discovery inputs, with no cross-engine interference and no data races

#### Scenario: Test seams do not mutate process policy
- **WHEN** tests exercise alternate config paths, cache paths, external directories, cache failures, or the fixed executable deadline and byte-limit boundaries
- **THEN** they MUST use invocation-local inputs, fixed boundary values, production-owned helpers, or injected host probes
- **AND** they MUST NOT replace mutable package variables

### Requirement: Discovery uses one input plan per run
The library SHALL derive source loading, disabled-set, cache, query-selection, and ambient default-path policy from one internal discovery plan for each `Discover` call. The plan MUST capture facts-native and compatible config paths, the cache path, and ordered default external-fact directories once per discovery. It MUST be recomputed per discovery so config files, external fact directories, environment facts, executable facts, cache contents, and ambient defaults remain fresh across repeated discovery on the same immutable Engine.

#### Scenario: Config is read at discovery time
- **WHEN** an Engine configured with `WithConfigFile` discovers facts, the config file changes, and the same Engine discovers facts again
- **THEN** the second Snapshot reflects the updated config-derived external dirs, blocklists, and cache TTL/group policy

#### Scenario: System defaults are invocation-local
- **WHEN** two concurrent system-following discoveries receive different ambient config, cache, or external-directory defaults through the internal host seam
- **THEN** each discovery MUST use only the defaults captured in its own input plan
- **AND** native config and external-directory locations MUST retain precedence over compatible locations

#### Scenario: Query selection happens in discovery
- **WHEN** a consumer calls `Discover(ctx, "os.family")`
- **THEN** the returned Snapshot is backed by facts selected with the same projection semantics used by CLI query projection where the contracts overlap
- **AND** the public `Discover(ctx, queries...)` method shape remains unchanged

#### Scenario: Cache policy stays discovery-scoped
- **WHEN** an Engine is configured with cache enabled and config-derived TTL/group policy
- **THEN** discovery applies cache resolution and cache refresh according to the per-discovery plan and its captured cache path
- **AND** the Engine does not memoize resolved fact values between discoveries

#### Scenario: Force-dot resolution is not public library configuration
- **WHEN** a library consumer constructs an Engine through public `facts` options
- **THEN** no public option exists for force-dot resolution
- **AND** the canonical Snapshot tree preserves existing dotted external and registered fact behavior
