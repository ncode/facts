## ADDED Requirements

### Requirement: Version fast path reuses engine-owned seams

The CLI's version-query fast path SHALL derive its disabled-fact set from the engine's exported pure disabled-union function — the same union semantics the engine's discovery planning applies — instead of re-implementing the union in `internal/app`, and SHALL render its output through the engine's formatter-selection seam (`BuildFormatter`) instead of a CLI-local re-derivation of format precedence. The fast-path decision itself and formatter selection remain owned by `internal/app` per the discovery-input-surface design. The engine SHALL NOT export helpers whose only purpose is to feed a CLI-side re-implementation of engine policy.

#### Scenario: Fast-path disabled set matches discovery semantics

- **WHEN** `facts facterversion` runs with any combination of `--disable`, the `FACTS_DISABLE` environment variable, and a config-file disable list
- **THEN** the fast path takes effect exactly when a full discovery would omit `facterversion` for the same inputs, because both derive the disabled set from the same engine union

#### Scenario: Disabled facterversion falls through identically

- **WHEN** `facterversion` is disabled by any disable source and queried in the default format
- **THEN** stdout, stderr diagnostics, and exit status are byte-identical to the behavior before the fast path consumed the engine union

#### Scenario: Version output is format-stable

- **WHEN** `facts facterversion` is rendered with `--json`, `--yaml`, `--hocon`, or the default format
- **THEN** the bytes written to stdout are identical to the previous hand-selected formatter output for each format
