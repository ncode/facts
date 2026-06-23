## ADDED Requirements

### Requirement: CLI discovery options feed the shared plan
The `facts` CLI SHALL translate runtime flags and config values that affect discovery inputs, cache policy, and dotted query projection into engine discovery configuration instead of independently applying those discovery rules after `Discover`.

#### Scenario: External source options are planned once
- **WHEN** the CLI runs with `--external-dir`, `--no-external-facts`, `--config`, or config-derived external dirs/blocklists
- **THEN** `internal/app` passes the resolved discovery policy into the engine path
- **AND** it does not duplicate source precedence, default-dir, or blocklist application outside the shared plan

#### Scenario: Cache options are planned once
- **WHEN** the CLI runs with cache enabled, `--no-cache`, or config-derived cache TTL/groups
- **THEN** cache enablement and TTL/group policy are applied through the shared discovery plan
- **AND** persistent cache storage remains handled by the engine cache implementation

#### Scenario: Force-dot resolution is projection-only
- **WHEN** the CLI runs with `--force-dot-resolution` or config `force-dot-resolution: true`
- **THEN** the value affects selected-query and formatter projection for dotted external or registered facts
- **AND** it does not affect source loading, source precedence, or the canonical Snapshot tree

#### Scenario: CLI process-edge behavior stays in app
- **WHEN** discovery planning moves into the engine path
- **THEN** `internal/app` still owns stdout/stderr, help/man/version tasks, formatter selection, timing output, diagnostic rendering, strict exit behavior, and supported option validation
