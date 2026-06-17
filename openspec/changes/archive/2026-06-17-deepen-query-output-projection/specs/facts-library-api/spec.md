## MODIFIED Requirements

### Requirement: Canonical tree queries and generic decode
A Snapshot SHALL expose the canonical tree — the same fact names, nesting, and value normalization the output contract pins — through pure query operations using Facter dot-notation, and a generic decode (`facts.As[T]`) SHALL convert any queried subtree into a caller-supplied type. Decode MUST read from the resolved canonical tree and MUST NOT resolve facts independently. Snapshot value lookup SHALL use the same internal projection semantics as CLI query projection where their contracts overlap, while preserving the library distinction between missing facts and resolved nil registered/external facts.

#### Scenario: Dotted query resolution
- **WHEN** a consumer queries `snapshot.Value("os.release.major")`
- **THEN** the returned value equals the corresponding node of the canonical tree, identical to the value the CLI would report for the same query

#### Scenario: Generic decode of any fact kind
- **WHEN** a consumer decodes a core, registered, or external fact subtree into a matching struct type via `facts.As[T]`
- **THEN** the populated value reflects the canonical tree, regardless of which source (core, registered, external) won precedence

#### Scenario: Decode shape mismatch fails loudly
- **WHEN** an operator-supplied fact has reshaped a name (e.g. an external fact redefines `os` as a string) and a consumer decodes it into an incompatible type
- **THEN** `facts.As[T]` returns a non-nil error describing the mismatch and never returns a partially or silently coerced value
