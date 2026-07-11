## MODIFIED Requirements

### Requirement: Canonical tree queries and generic decode
A Snapshot SHALL expose the canonical tree — the same fact names, nesting, and value normalization the output contract pins — through pure query operations using Facter dot-notation, and a generic decode (`facts.As[T]`) SHALL convert any queried subtree into a caller-supplied type. Decode MUST read from the resolved canonical tree and MUST NOT resolve facts independently. Snapshot value lookup SHALL use the same internal projection semantics as CLI query projection where their contracts overlap, while preserving the library distinction between missing facts and resolved nil registered/external facts. Internal presentation consumers MUST receive a defensive Projection rather than raw Snapshot records, and that Projection MUST NOT expand the public Snapshot API or permit mutation of Snapshot state.

#### Scenario: Dotted query resolution
- **WHEN** a consumer queries `snapshot.Value("os.release.major")`
- **THEN** the returned value equals the corresponding node of the canonical tree, identical to the value the CLI would report for the same query

#### Scenario: Generic decode of any fact kind
- **WHEN** a consumer decodes a core, registered, or external fact subtree into a matching struct type via `facts.As[T]`
- **THEN** the populated value reflects the canonical tree, regardless of which source (core, registered, external) won precedence

#### Scenario: Decode shape mismatch fails loudly
- **WHEN** an operator-supplied fact has reshaped a name (e.g. an external fact redefines `os` as a string) and a consumer decodes it into an incompatible type
- **THEN** `facts.As[T]` returns a non-nil error describing the mismatch and never returns a partially or silently coerced value

#### Scenario: Presentation cannot mutate a Snapshot
- **WHEN** the internal CLI adapter formats a Snapshot through a presentation Projection and formatter or custom-value code mutates a returned map, slice, pointer, array, or exported struct field
- **THEN** subsequent `Snapshot.Value`, `Snapshot.Tree`, `Snapshot.All`, and `facts.As[T]` calls MUST observe the original immutable Snapshot values

#### Scenario: Internal presentation boundary hides resolved records
- **WHEN** `internal/app` obtains a Snapshot's defensive presentation Projection
- **THEN** no Snapshot method or app-visible Projection selection or iterator operation SHALL expose its backing `[]ResolvedFact` records
- **AND** normal app and formatter paths MUST consume only Projection shape, value, name, and missing-query views
- **AND** the version-query fast path MAY construct its separate synthetic resolved fact solely to build its independent presentation Projection

#### Scenario: Force-dot presentation does not replace the canonical tree
- **WHEN** the CLI requests force-dot presentation for dotted external or registered facts
- **THEN** the internal presentation Projection MAY merge those dotted facts for query/output behavior
- **AND** the Snapshot's canonical tree and public query/decode results MUST retain the existing non-force-dot semantics

#### Scenario: Public Snapshot surface remains unchanged
- **WHEN** the internal raw-fact formatter escape is replaced by a presentation Projection
- **THEN** the public Snapshot SHALL continue to expose only its existing canonical tree, value lookup, ordered iteration, and generic decode operations
- **AND** no public raw resolved-fact or Projection accessor SHALL be added
