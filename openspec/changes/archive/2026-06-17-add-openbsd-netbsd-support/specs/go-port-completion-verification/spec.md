## MODIFIED Requirements

### Requirement: Supported-platform verification matrix
The Go port SHALL pass a defined verification matrix before release completion.

#### Scenario: Local deterministic gates
- **WHEN** maintainers validate deterministic Go behavior
- **THEN** `go test ./...`, `go test -race ./...`, gofmt checks, `go vet ./...`, and `git diff --check` MUST pass for the tracked Go port files

#### Scenario: Linux validation gates
- **WHEN** maintainers validate Linux behavior
- **THEN** Linux Go tests, Linux distro/container smoke coverage, and Linux CLI fact smoke checks MUST pass for representative supported Linux families

#### Scenario: macOS validation gates
- **WHEN** maintainers validate macOS/Darwin behavior
- **THEN** macOS host tests and CLI fact smoke checks MUST pass for representative macOS facts, including OS, networking, memory, processors, DMI, system profiler, mountpoints, uptime, and virtualization

#### Scenario: Windows validation gates
- **WHEN** maintainers validate Windows behavior
- **THEN** Windows CI or an approved Windows runner MUST pass Go tests and CLI fact smoke checks for OS, system32, networking, memory, processors, DMI, uptime, virtualization, FIPS, and timezone, using structured fact names only

#### Scenario: FreeBSD validation gates
- **WHEN** maintainers validate FreeBSD behavior
- **THEN** Lima FreeBSD build/smoke coverage MUST pass and MUST be broad enough to exercise the FreeBSD release-gate fact set defined by the supported-platform facts capability

#### Scenario: OpenBSD validation gates
- **WHEN** maintainers validate OpenBSD behavior
- **THEN** OpenBSD CI or an approved OpenBSD VM runner MUST pass platform-sensitive Go tests and CLI fact smoke checks for OS, networking, memory, processors, DMI when supported, disks, partitions, mountpoints, uptime, virtualization, SSH, and timezone, using structured fact names only

#### Scenario: NetBSD validation gates
- **WHEN** maintainers validate NetBSD behavior
- **THEN** NetBSD CI or an approved NetBSD VM runner MUST pass platform-sensitive Go tests and CLI fact smoke checks for OS, networking, memory, processors, DMI when supported, disks, partitions, mountpoints, uptime, virtualization, SSH, and timezone, using structured fact names only
