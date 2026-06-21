## Why

The Plan 9 lab guest is now reachable through normal SSH, and the Go toolchain already cross-builds Facts for `plan9/amd64`. This lets Facts start adding real Plan 9 support with native validation instead of leaving Plan 9 as an unreachable or purely aspirational target.

Plan 9 is not Unix-shaped: it has no `uname`, no `sysctl`, no conventional mount table, and different networking and memory surfaces. The first support slice needs an explicit contract so Facts only claims facts that are backed by native Plan 9 probes and a repeatable lab gate.

## What Changes

- Add Plan 9 as a supported, lab-validated platform for a narrow initial fact set.
- Implement Plan 9 discovery for stable native surfaces:
  - OS/kernel identity from explicit Plan 9 constants.
  - Hostname from `/dev/sysname`.
  - Architecture and hardware from Go runtime values, `$cputype`/`$objtype`, `/dev/cputype`, and `/dev/archctl`.
  - Memory total from `/dev/swap`.
  - Processor count/model/speed from `/dev/sysstat`, `/dev/cputype`, and `/dev/archctl`.
  - Basic IPv4 networking from `/net/ipifc/*/status`, `/net/*/addr`, `/net/iproute`, and `/net/ndb`.
  - Uptime from Plan 9 `uptime`.
  - Timezone from Go's local time support or Plan 9 `date`/`/env/timezone`.
  - Existing path/environment facts where they already work under Go on Plan 9.
- Add deterministic parser tests for each Plan 9-specific format before wiring live probes into discovery.
- Add a tracked Plan 9 release-gate script written for `rc`, not `sh`, and validate it on the facts-lab Plan 9 guest.
- Add `plan9/amd64` to compile/build verification only after the native gate can prove the supported fact set.
- Update `docs/schema/facts.yaml`, generated `docs/supported-facts/plan9.md`, README/CONTRIBUTING support tables, and schema platform validation for only the facts Plan 9 can emit.
- Explicitly avoid first-pass support for facts that do not have a stable Plan 9 contract:
  - `os.release.*`
  - `kernel.release.*`
  - `kernel.version.full`
  - filesystems and mountpoint capacity
  - disk/partition inventory
  - `load_averages`
  - DMI/cloud facts
  - exact virtualization classification
  - DHCP server facts
- No breaking changes are intended.

## Capabilities

### New Capabilities
- `plan9-platform-facts`: Plan 9 fact discovery, supported fact contract, and native release-gate validation for the initial stable fact set.

### Modified Capabilities
- `facts-schema`: Add Plan 9 to the supported platform vocabulary and list `plan9` only on schema entries proven by parser tests and native Plan 9 validation.
- `go-port-ci-platform-gates`: Add Plan 9 lab validation and `plan9/amd64` compile coverage as an in-scope candidate gate with explicit limits.
- `go-port-distribution-and-cutover`: Define when Plan 9 enters build/dist artifacts and prevent publishing unsupported or unvalidated Plan 9 tuples.

## Impact

- Code:
  - `internal/engine/os.go`
  - `internal/engine/networking.go`
  - `internal/engine/memory.go`
  - `internal/engine/processors.go`
  - `internal/engine/uptime.go`
  - `internal/engine/timezone.go` if Go's local timezone behavior is insufficient on Plan 9
  - Small Plan 9-only helpers may be added when existing cross-platform code cannot safely read native files.
- Tests:
  - Parser/unit tests beside the touched engine files.
  - Compile-only `GOOS=plan9 GOARCH=amd64` verification.
  - Native facts-lab Plan 9 release gate.
  - Schema conformance for the Plan 9 supported fact set.
- Docs/schema:
  - `docs/schema/facts.yaml`
  - `docs/supported-facts/plan9.md`
  - README platform table
  - CONTRIBUTING platform gate notes
- Tooling:
  - New `tools/plan9-release-gate.rc`.
  - Optional Makefile target or documented lab command that runs the same tracked Plan 9 gate.
- Lab:
  - Uses `facts-lab ssh plan9` through the nlab host.
  - Does not store lab hostnames, keys, guest addresses, or private helper internals in tracked repository files.
