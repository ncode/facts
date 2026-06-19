# Delta: go-port-framework-parity

## ADDED Requirements

### Requirement: Framework security boundaries
The Go port SHALL bound untrusted framework inputs and SHALL render machine formats with syntactically safe keys.

#### Scenario: Cache groups stay within the cache directory
- **WHEN** fact cache TTL configuration or custom fact groups contain absolute paths, path traversal, separators, drive prefixes, empty names, or dot-only names
- **THEN** cache reads, writes, freshness checks, and invalidation MUST ignore those groups and MUST NOT read, write, or delete files outside the configured cache directory

#### Scenario: External fact sources are bounded
- **WHEN** static external fact files, executable external fact stdout, or executable external fact stderr exceed the external fact byte limit
- **THEN** discovery MUST stop reading that source and report an oversized external fact failure through the same CLI/library error policy used for that source kind

#### Scenario: YAML and HOCON keys are escaped
- **WHEN** selected or unselected fact output contains a map key that is unsafe in YAML or HOCON plain-key syntax
- **THEN** the YAML and HOCON formatters MUST quote the key so the output remains parseable data rather than injected syntax

#### Scenario: Built-in probes ignore untrusted PATH entries
- **WHEN** core fact resolution runs built-in host probe commands
- **THEN** those commands MUST be resolved from a fixed platform system search path and MUST receive a sanitized `PATH` that excludes caller-provided path entries while preserving unrelated environment variables

#### Scenario: Filesystem byte math clamps overflow
- **WHEN** platform statfs data reports block counts or block sizes whose product exceeds the Go `int` range
- **THEN** mountpoint byte totals MUST clamp to the largest representable `int` before formatting facts
