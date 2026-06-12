# Delta: go-port-distribution-and-cutover

## MODIFIED Requirements

### Requirement: Go-era acceptance verification
The Go port SHALL have end-to-end acceptance verification that exercises the real binary on each supported platform.

#### Scenario: Binary-level acceptance suite
- **WHEN** the acceptance suite runs on a supported platform
- **THEN** it MUST build the real `cmd/facter` binary, execute it with representative flag combinations (default, single query, dotted query, `--json`, `--yaml`, `--strict`), and assert the release-gate fact set and exit codes against the live host using structured fact names only

#### Scenario: Beaker suite marked historical
- **WHEN** the Go acceptance suite is in place for the four supported platforms
- **THEN** the Ruby `acceptance/` Beaker suite MUST be documented as historical for the Go port and excluded from Go release gates
