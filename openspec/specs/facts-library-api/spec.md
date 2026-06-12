# facts-library-api Specification

## Purpose
TBD - created by archiving change introduce-facts-library-api. Update Purpose after archive.
## Requirements
### Requirement: Hermetic engine construction
`facts.New` SHALL return an Engine that resolves core facts only: it MUST NOT read configuration files, scan external fact directories, execute external-fact scripts, read external-fact environment variables, or touch the persistent cache unless the corresponding functional option is supplied. A `WithSystemDefaults` option SHALL configure an Engine with full CLI-equivalent system-following behavior. The library SHALL expose no option for loading Ruby DSL fact files; `WithCustomDirs` does not exist.

#### Scenario: Default engine performs no implicit host configuration
- **WHEN** a consumer calls `facts.New()` with no options on a host that has a configuration file, populated default fact directories, and executable external facts
- **THEN** discovery resolves core facts only, executes no scripts, reads no config file, and the resolved facts are unaffected by the host's configuration

#### Scenario: Explicit opt-in to fact sources
- **WHEN** an Engine is constructed with `WithExternalDirs`, `WithConfigFile`, or `WithFact` options
- **THEN** discovery loads exactly the opted-in sources with input-contract semantics (external-fact parsing, precedence, configuration interpretation) identical to the CLI's

#### Scenario: System-following engine matches CLI behavior
- **WHEN** an Engine is constructed with `WithSystemDefaults`
- **THEN** its resolved canonical tree matches what the `facts` CLI resolves on the same host

### Requirement: Immutable engine with explicit snapshot discovery
An Engine SHALL be immutable after construction, with all fact registrations and configuration fixed at `New`. Resolution SHALL happen only through `Discover(ctx)`, which returns an immutable Snapshot of the canonical tree. The library SHALL hold no package-global mutable state, and Engines and Snapshots SHALL be safe for concurrent use.

#### Scenario: Discovery honors context cancellation
- **WHEN** the context passed to `Discover` is cancelled or exceeds its deadline while resolvers (including command execution and cloud-metadata requests) are running
- **THEN** `Discover` returns promptly with an error satisfying `errors.Is(err, ctx.Err())`

#### Scenario: Freshness requires re-discovery
- **WHEN** system state changes after a Snapshot was discovered
- **THEN** the existing Snapshot is unchanged, and a new `Discover` call returns a new Snapshot reflecting the new state

#### Scenario: Engines are isolated
- **WHEN** two Engines with different configurations (e.g. different registered facts for the same fact name) discover concurrently in one process
- **THEN** each Snapshot reflects only its own Engine's configuration, with no cross-engine interference and no data races

### Requirement: Canonical tree queries and generic decode
A Snapshot SHALL expose the canonical tree — the same fact names, nesting, and value normalization the output contract pins — through pure query operations using Facter dot-notation, and a generic decode (`facts.As[T]`) SHALL convert any queried subtree into a caller-supplied type. Decode MUST read from the resolved canonical tree and MUST NOT resolve facts independently.

#### Scenario: Dotted query resolution
- **WHEN** a consumer queries `snapshot.Value("os.release.major")`
- **THEN** the returned value equals the corresponding node of the canonical tree, identical to the value the CLI would report for the same query

#### Scenario: Generic decode of any fact kind
- **WHEN** a consumer decodes a core, registered, or external fact subtree into a matching struct type via `facts.As[T]`
- **THEN** the populated value reflects the canonical tree, regardless of which source (core, registered, external) won precedence

#### Scenario: Decode shape mismatch fails loudly
- **WHEN** an operator-supplied fact has reshaped a name (e.g. an external fact redefines `os` as a string) and a consumer decodes it into an incompatible type
- **THEN** `facts.As[T]` returns a non-nil error describing the mismatch and never returns a partially or silently coerced value

### Requirement: Error semantics
The library SHALL distinguish missing facts from nil-valued facts via an `ErrFactNotFound` sentinel, SHALL return partial results with aggregated errors on partial discovery failure, and SHALL NOT treat not-applicable facts as failures.

#### Scenario: Missing fact versus nil-valued fact
- **WHEN** a consumer queries a name no fact resolved (`ErrFactNotFound` case) and separately queries a registered fact that legitimately resolved to nil
- **THEN** the former returns an error satisfying `errors.Is(err, facts.ErrFactNotFound)` and the latter returns `(nil, nil)`

#### Scenario: Partial discovery failure
- **WHEN** one configured source fails during discovery (e.g. an opted-in external-fact script exits non-zero) while other resolvers succeed
- **THEN** `Discover` returns a Snapshot containing every successfully resolved fact together with a non-nil joined error identifying each failure

#### Scenario: Not-applicable facts are not errors
- **WHEN** discovery runs on a host where a fact's preconditions do not hold (e.g. EC2 metadata facts off-cloud)
- **THEN** those facts are absent from the Snapshot and contribute nothing to the returned error

### Requirement: Diagnostics via structured logging
Engine diagnostics SHALL flow through `log/slog` with the contract-pinned message text and mapped severities, SHALL be discarded by default, and SHALL preserve once-only emission semantics per Engine.

#### Scenario: Silent by default
- **WHEN** an Engine constructed without `WithLogger` encounters warn-class conditions (e.g. an invalid external-fact file in an opted-in directory)
- **THEN** discovery proceeds, the fact is skipped per input-contract semantics, and nothing is written to any process-global logger or stderr

#### Scenario: Diagnostics routed to the consumer's logger
- **WHEN** an Engine is constructed with `WithLogger` and a once-only diagnostic condition occurs repeatedly within and across discoveries on that Engine
- **THEN** the diagnostic is emitted to the supplied logger with contract-equivalent message text and severity, exactly once per Engine
