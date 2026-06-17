# Delta: go-port-framework-parity

## MODIFIED Requirements

### Requirement: External fact parity
The Go port SHALL support Ruby-compatible external fact loading for supported-platform operation, and SHALL centralize external-fact filesystem, environment, platform, and command execution behavior behind an explicit loader seam so CLI and library modes preserve their distinct error and source semantics without sentinel-driven control flow or package-global test hooks.

#### Scenario: External fact loading
- **WHEN** external facts are loaded from environment variables, text files, JSON files, YAML files, executable scripts, PowerShell scripts, configured paths, default paths, or blocked paths
- **THEN** the Go port MUST match Ruby behavior for name normalization, structured value normalization, parser diagnostics, executable stderr warnings, recursive-resolution guards, timeout handling, null-byte rejection, blocklist handling, and precedence over core and registered facts

#### Scenario: CLI and library loader modes are explicit
- **WHEN** external facts are resolved through the `facts` CLI and through a library Engine configured with explicit external directories or system defaults
- **THEN** both paths MUST use the external-fact loader seam while preserving their existing semantics for environment fact inclusion, executable/context failures, hard source errors, partial results, and joined errors

#### Scenario: External fact host behavior is injectable
- **WHEN** deterministic Go tests exercise external-fact directory walking, platform-specific executable handling, PowerShell execution, recursive-resolution guards, environment facts, unreadable files, or command output/stderr
- **THEN** they MUST be able to substitute a fake loader host instead of mutating package-global platform, command, file, or environment state
