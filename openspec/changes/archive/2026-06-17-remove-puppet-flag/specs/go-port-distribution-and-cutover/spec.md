# Delta: go-port-distribution-and-cutover

## MODIFIED Requirements

### Requirement: End-user documentation matches the Go CLI
The Go port SHALL ship user documentation that reflects the Go binary's actual behavior.

#### Scenario: Man page parity
- **WHEN** the man page is regenerated or audited against the Go CLI
- **THEN** every documented flag, default, and exit code MUST match the Go implementation, and Go-port deviations (the no-Ruby-DSL input contract) MUST be noted
