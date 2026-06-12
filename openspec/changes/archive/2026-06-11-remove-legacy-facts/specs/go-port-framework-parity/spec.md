# Delta: go-port-framework-parity

## ADDED Requirements

### Requirement: No legacy facts
Facts SHALL NOT expose legacy alias facts in any output mode. The canonical structured tree is the only fact surface; flat Ruby-era aliases (`operatingsystem`, `hostname`, `processorcount`, `sshfp_*`, `mtu_*`, …) do not resolve, whether unqueried, explicitly queried, or requested via removed flags.

#### Scenario: Legacy aliases are absent from all output

- **WHEN** the `facter` CLI runs with no query and any output format, or a library consumer discovers a Snapshot
- **THEN** the output MUST contain only structured facts; no legacy alias name appears at the top level

#### Scenario: Explicitly queried legacy aliases resolve nothing

- **WHEN** the `facter` CLI is invoked with a legacy alias query such as `facter operatingsystem`
- **THEN** the query MUST behave exactly like any other missing fact (empty output, exit 0; missing-fact error under `--strict`)

#### Scenario: Removed legacy flags fail as unknown options

- **WHEN** the `facter` CLI is invoked with `--show-legacy` or `--no-show-legacy`
- **THEN** the CLI MUST exit with a usage error identifying the unknown option, exactly as for any unrecognized flag

#### Scenario: Retired show-legacy config key is inert

- **WHEN** `facter.conf` contains a `show-legacy` key
- **THEN** the config MUST load without error and the key MUST have no effect, identical to any other unrecognized key

#### Scenario: Legacy blocklist group has no effect

- **WHEN** `facter.conf` contains `blocklist : [ "legacy" ]`
- **THEN** the config MUST load without error and discovery output MUST be identical to a run without the entry

## MODIFIED Requirements

### Requirement: CLI parity
The `facter` CLI SHALL preserve Ruby-compatible output and status behavior for supported release use cases. Ruby compatibility is promised only at the CLI process boundary; the Go library API makes no Ruby-compatibility promises. The `--custom-dir`, `--no-ruby`, `--no-custom-facts`, `--trace`, `--show-legacy`, and `--no-show-legacy` flags are deliberately not part of the surface (ADR-0006 for the custom-fact flags; the legacy-facts removal ADR for the legacy flags) and fail as unknown options.

#### Scenario: CLI output and status behavior
- **WHEN** users run the Go `facter` CLI with no query, one query, multiple queries, `--json`, `--yaml`, `--hocon`, `--strict`, `--config`, `--external-dir`, logging flags, and compatibility short flags
- **THEN** stdout, stderr, exit status, option validation, missing-fact handling, query key preservation, and formatter output MUST match the corresponding Ruby behavior for in-scope features, except that legacy alias facts are absent from every mode
