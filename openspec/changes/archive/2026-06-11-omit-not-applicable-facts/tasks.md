# Tasks: Omit not-applicable and empty facts

## 1. Resolver fixes

- [x] 1.1 `augeas`: omit the fact entirely when augparse is unavailable (no `{version: ""}`)
- [x] 1.2 `disks`/`partitions`: omit when device enumeration yields nothing (macOS today)
- [x] 1.3 `processors.speed`: omit the key when the speed probe yields no value (Apple Silicon)
- [x] 1.4 `fips_enabled`: emit only on Linux and Windows
- [x] 1.5 `os.selinux` (and any related SELinux data): emit only on Linux

## 2. Tests

- [x] 2.1 Per-fact tests: absence on the not-applicable platform/state, continued presence where Ruby resolves the fact (fixture-backed for non-host platforms)
- [x] 2.2 Sweep test: a default discovery on the host produces no top-level fact whose value is an empty string or empty map
- [x] 2.3 Existing structured-tree and formatter tests pass unmodified

## 3. Documentation and verification

- [x] 3.1 Document the `processors.extensions` deviation in the man page GO PORT NOTES
- [x] 3.2 CHANGELOG entry: placeholder facts omitted; per-platform set converges on Ruby's
- [x] 3.3 `go test ./...`, `go test -race ./...`, vet/gofmt clean; platform CI gates green
- [x] 3.4 Rerun the macOS Ruby-vs-Go comparison: `augeas`, `disks`, `partitions`, `fips_enabled`, `os.selinux`, and empty `processors.speed` no longer appear
