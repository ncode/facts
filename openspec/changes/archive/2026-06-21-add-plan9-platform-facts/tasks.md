## 1. Baseline And Probe Fixtures

- [x] 1.1 Confirm the repo still cross-compiles with `CGO_ENABLED=0 GOOS=plan9 GOARCH=amd64 go build ./cmd/facts`
- [x] 1.2 Confirm compile-only tests still pass with `CGO_ENABLED=0 GOOS=plan9 GOARCH=amd64 go test -exec true ./...`
- [x] 1.3 Capture minimal Plan 9 lab samples for `/dev/sysname`, `/dev/swap`, `/dev/sysstat`, `/dev/cputype`, `/dev/archctl`, `/net/ipifc/*/status`, `/net/*/addr`, `/net/iproute`, `/net/ndb`, `uptime`, and timezone output
- [x] 1.4 Add parser test fixtures using the captured Plan 9 samples without requiring the tests to run on Plan 9

## 2. OS, Kernel, Hostname, And Architecture

- [x] 2.1 Add Plan 9 cases for canonical `os.name`, `os.family`, and `kernel.name` values
- [x] 2.2 Add Plan 9 hostname discovery from trimmed `/dev/sysname`
- [x] 2.3 Add Plan 9 architecture and hardware discovery using `$objtype` with `runtime.GOARCH` fallback
- [x] 2.4 Add Plan 9 processor ISA discovery using `$cputype`, `$objtype`, or `runtime.GOARCH`
- [x] 2.5 Add tests proving Plan 9 does not emit OS/kernel release facts from `/dev/osversion`

## 3. Memory And Processors

- [x] 3.1 Add a parser for Plan 9 `/dev/swap` memory total lines
- [x] 3.2 Wire `memory.system.total_bytes` and `memory.system.total` on Plan 9
- [x] 3.3 Add a parser for Plan 9 `/dev/sysstat` processor line counts
- [x] 3.4 Wire `processors.count` on Plan 9
- [x] 3.5 Add parsers for `/dev/cputype` and `/dev/archctl` processor model data
- [x] 3.6 Wire `processors.models` on Plan 9 when model data is present

## 4. Networking

- [x] 4.1 Add a parser for Plan 9 `/net/ipifc/*/status` device and IPv4 address rows
- [x] 4.2 Convert Plan 9 IPv4-mapped prefixes such as `/120` into IPv4 prefixes such as `/24`
- [x] 4.3 Derive Plan 9 IPv4 netmask and network values from parsed address rows
- [x] 4.4 Add a parser for Plan 9 interface MAC files such as `/net/ether0/addr`
- [x] 4.5 Add a parser for Plan 9 default routes from `/net/iproute`
- [x] 4.6 Wire Plan 9 `networking.primary` and top-level `networking.ip`, `networking.netmask`, `networking.network`, and `networking.mac`
- [x] 4.7 Keep DHCP server and MTU facts out of the required Plan 9 gate unless their semantics are separately tested

## 5. Uptime, Timezone, And Existing Generic Facts

- [x] 5.1 Add a parser for Plan 9 `uptime` output such as `cirno up 0 days, 01:35:26`
- [x] 5.2 Wire Plan 9 `system_uptime` and `system_uptime.seconds`
- [x] 5.3 Verify existing timezone discovery works on Plan 9; add a Plan 9 fallback from native timezone output only if needed
- [x] 5.4 Verify existing `path` fact behavior on Plan 9 and include it in the gate only if native output is stable

## 6. Schema And Documentation

- [x] 6.1 Add `plan9` to the schema platform vocabulary in schema validation tests
- [x] 6.2 Add `plan9` to `docs/schema/facts.yaml` only for facts emitted by the implemented Plan 9 discovery
- [x] 6.3 Mark Plan 9 schema entries conditional when host state or probe availability controls emission
- [x] 6.4 Generate `docs/supported-facts/plan9.md` from the schema
- [x] 6.5 Update README platform support text to describe Plan 9 as lab-validated fact support unless release artifacts are added
- [x] 6.6 Update CONTRIBUTING with the Plan 9 gate workflow and the rule that Plan 9 release-gate facts must stay schema-backed

## 7. Plan 9 Release Gate And Build Matrix

- [x] 7.1 Add `tools/plan9-release-gate.rc` using Plan 9 `rc` syntax
- [x] 7.2 Make the Plan 9 gate execute the real `facts` binary through structured fact queries
- [x] 7.3 Make the Plan 9 gate assert the first supported fact set and explicitly avoid unsupported release/filesystem/disk/load/DMI/cloud facts
- [x] 7.4 Add or document a local Plan 9 lab command path that copies the binary and gate to the guest without committing private lab details
- [x] 7.5 Add `plan9/amd64` compile/build verification after the native Plan 9 gate passes
- [x] 7.6 Leave `plan9/386` and `plan9/arm` out of build and release matrices until native validation exists

## 8. Validation

- [x] 8.1 Run `gofmt -w` on edited Go files
- [x] 8.2 Run `go test ./...`
- [x] 8.3 Run `go vet ./...`
- [x] 8.4 Run `make build`
- [x] 8.5 Run `CGO_ENABLED=0 GOOS=plan9 GOARCH=amd64 go build ./cmd/facts`
- [x] 8.6 Run `CGO_ENABLED=0 GOOS=plan9 GOARCH=amd64 go test -exec true ./...`
- [x] 8.7 Run the Plan 9 native gate through `facts-lab ssh plan9`
- [x] 8.8 Run schema supported-facts generation/checks and verify `docs/supported-facts/plan9.md` is current
- [x] 8.9 Run Open Code Review after implementation and address actionable findings
