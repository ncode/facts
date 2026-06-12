# Delta: go-port-ci-platform-gates

## MODIFIED Requirements

### Requirement: Windows validation is a blocking CI gate
The Go port SHALL treat the Windows release gate as a blocking, automated pass/fail criterion.

#### Scenario: Release gate failure fails the workflow
- **WHEN** `tools/windows-release-gate.ps1` exits non-zero on a Windows CI runner
- **THEN** the unit-test workflow job MUST fail, and the gate MUST be documented in `CONTRIBUTING.md` as a release-blocking check

#### Scenario: Gate covers the Windows release fact set
- **WHEN** the Windows release gate runs
- **THEN** it MUST build the real `cmd/facts` binary and verify the Windows release-gate fact set (including OS, system32, networking, memory, processors, DMI, uptime, virtualization, FIPS, and timezone) through the CLI using structured fact names only
