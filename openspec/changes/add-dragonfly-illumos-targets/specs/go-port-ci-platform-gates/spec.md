## MODIFIED Requirements

### Requirement: CI build matrix is limited to in-scope platforms
The Go port's CI SHALL build only supported release targets and active candidate release targets that have repeatable validation.

#### Scenario: In-scope cross-compiles
- **WHEN** the cross-compile CI job runs after DragonFly and illumos promotion
- **THEN** it MUST build linux, darwin, windows, freebsd, openbsd, netbsd, dragonfly, and illumos targets only
- **AND** it MUST NOT build solaris or aix targets

#### Scenario: Oracle Solaris is not built by illumos validation
- **WHEN** the illumos candidate or supported gate runs
- **THEN** the pipeline MUST build `illumos/amd64`
- **AND** it MUST NOT build or publish `solaris/amd64`

## ADDED Requirements

### Requirement: DragonFly and illumos validation is automated after promotion
The Go port SHALL validate DragonFly and illumos release targets through automated or asserted native gates after promotion, not only manual cross-compilation.

#### Scenario: DragonFly native gate
- **WHEN** the pipeline validates DragonFly after promotion
- **THEN** an automated job or asserted external lab status MUST execute platform-sensitive Go tests and the DragonFly release-gate fact-set smoke on a DragonFly environment
- **AND** its failure MUST fail the pipeline

#### Scenario: illumos native gate
- **WHEN** the pipeline validates illumos after promotion
- **THEN** an automated job or asserted external lab status MUST execute platform-sensitive Go tests and the illumos release-gate fact-set smoke on an illumos environment
- **AND** its failure MUST fail the pipeline

#### Scenario: Local and CI smokes stay aligned
- **WHEN** the DragonFly or illumos release-gate fact set changes
- **THEN** the CI smoke and local smoke target MUST verify the same fact set by running the same tracked release-gate script

#### Scenario: Lab details stay out of git
- **WHEN** local smoke targets invoke DragonFly, illumos, or amd64 BSD lab guests
- **THEN** tracked files MUST reference configurable wrapper variables only
- **AND** lab hostnames, addresses, keys, and private helper commands MUST remain outside tracked files
