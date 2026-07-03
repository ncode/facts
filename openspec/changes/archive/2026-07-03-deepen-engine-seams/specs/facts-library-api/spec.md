## MODIFIED Requirements

### Requirement: Diagnostics via structured logging

Engine diagnostics SHALL flow through `log/slog` with the contract-pinned message text and mapped severities, SHALL be discarded by default, and SHALL preserve once-only emission semantics per Engine. There SHALL be no package-global diagnostic sink: every diagnostic raised during discovery — including those from config-file parsing, the persistent cache, fact-group TTL parsing, and canonical-tree collection collisions (the default collection the Snapshot exposes) — SHALL be routed to the Engine's logger, not to a process-global handler. Collisions that arise only from a format-time transform (the CLI's `--force-dot-resolution`), not from the canonical tree, are out of scope of this requirement.

#### Scenario: Silent by default

- **WHEN** an Engine constructed without `WithLogger` encounters warn-class conditions (e.g. an invalid external-fact file in an opted-in directory)
- **THEN** discovery proceeds, the fact is skipped per input-contract semantics, and nothing is written to any process-global logger or stderr

#### Scenario: Diagnostics routed to the consumer's logger

- **WHEN** an Engine is constructed with `WithLogger` and a once-only diagnostic condition occurs repeatedly within and across discoveries on that Engine
- **THEN** the diagnostic is emitted to the supplied logger with contract-equivalent message text and severity, exactly once per Engine

#### Scenario: Config, cache, and group diagnostics reach the logger

- **WHEN** an Engine constructed with `WithLogger`, `WithConfigFile`, and `WithCache` encounters a config read failure, a cache write failure, or an unparseable group TTL during discovery
- **THEN** each diagnostic is emitted to the supplied logger with its mapped severity, rather than discarded or sent to a process-global handler

#### Scenario: Canonical-tree collision is reported once at discovery

- **WHEN** a resolved fact value collides with a dotted child in the canonical tree (e.g. `os` resolves to a scalar while `os.name` also resolves)
- **THEN** the collision is emitted once to the Engine's logger at error severity during discovery, and is not re-emitted when the resulting Snapshot is formatted

#### Scenario: Format-time-only collisions under force-dot resolution are out of scope

- **WHEN** two typed (custom or external) facts collide only when `--force-dot-resolution` expands their dotted names (e.g. `myapp.version` and `myapp.version.major` with no plain `myapp`), so they do not collide in the canonical tree and both appear in the Snapshot as flat keys
- **THEN** no collision diagnostic is emitted at discovery, matching Facter 4.10.0, which under `global.force-dot-resolution` also silently drops the colliding fact with no diagnostic on stderr at any log level (the facts CLI drops error-class diagnostics regardless)

#### Scenario: Error-class diagnostics reach the library logger

- **WHEN** an Engine constructed with `WithLogger` raises an error-class diagnostic (a collection collision, an unsupported cache group for an external fact, or an unparseable TTL unit)
- **THEN** the diagnostic is emitted to the supplied logger at error severity, even though the facts CLI's stderr handler drops error-class lines
