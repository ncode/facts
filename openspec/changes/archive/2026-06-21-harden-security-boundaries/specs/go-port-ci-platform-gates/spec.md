# Delta: go-port-ci-platform-gates

## ADDED Requirements

### Requirement: Vulnerability scanning is automated in CI
The Go port SHALL run Go vulnerability analysis as a blocking CI check.

#### Scenario: Vulnerability scan failure fails the workflow
- **WHEN** the Go checks workflow runs
- **THEN** it MUST run the repository-pinned `govulncheck` tool against `./...`, and any reported vulnerability or scanner failure MUST fail the workflow
