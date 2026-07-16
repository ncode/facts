## 1. Descriptor-Driven Resolution Gating

- [x] 1.1 Add failing probe-count tests for every standalone, multi-output, shared-output, cloud-provider, identity/SSH, and DMI/GCE all-disabled versus one-kept case.
- [x] 1.2 Add public `facts` API or `internal/app` contract coverage proving corrected gating leaves resolved output, pruning, cache, queries, and ambient-disable diagnostics unchanged.
- [x] 1.3 Replace the four inert gating classes with the minimum always-eager/gateable policy and make exact `emittedRoots` drive one resolver-run predicate.
- [x] 1.4 Remove `probeConsumers` and `emitsUnder`, move DMI acquisition from eager core-build construction to the kept DMI/GCE assemblies, and preserve eager virtualization plus `selinux` behavior.
- [x] 1.5 Add row-exact descriptor metadata/output agreement tests covering shared `cloud` roots and every multi-output category.
- [x] 1.6 Update `CHANGELOG.md` with the corrected fully-disabled resolver behavior.

## 2. Invocation-Local Discovery Inputs

- [x] 2.1 Add failing concurrent-isolation and repeated-discovery tests for distinct config paths, cache paths, and ordered default external directories without package-variable mutation.
- [x] 2.2 Introduce one clone-safe internal defaults value and derive it once per library discovery or CLI invocation through current platform/environment adapters.
- [x] 2.3 Thread invocation defaults through config parsing, discovery planning, fact-group listing, Engine construction, and cache construction while preserving all explicit/default precedence.
- [x] 2.4 Delete `DefaultCachePath`, `DefaultConfigPath`, `NativeDefaultConfigPath`, and `defaultExternalFactDirs`; retarget affected root, engine, and app tests to local inputs and enable safe parallelism.
- [x] 2.5 Delete `cacheWriteFile` and `cacheRemove`, call production I/O directly, and retarget permission-failure assertions to production-owned warning-policy helpers.
- [x] 2.6 Make the external-fact timeout and byte ceiling constants; retarget timeout coverage to `testing/synctest` and size coverage to real limit-plus-one inputs.
- [x] 2.7 Extend the architecture seam gate to reject the removed mutable hook names and equivalent package-level function/limit replacements.

## 3. False Test Surface Cleanup

- [x] 3.1 Retarget partition and Windows assertions to live production seams, then delete `partitionsFact`, `windowsHardwareArchitecture`, `windowsOSDescription`, and `currentWindowsOSDescription`.
- [x] 3.2 Retarget Plan 9 networking assembly tests to `Session` plus `fakeHostOS.globs`, then delete the unreachable `networkingInterfacesForPlatform` Plan 9 branch and `plan9NetworkingCoreFactsWithGlob` wrapper.
- [x] 3.3 Run cross-target dead-code analysis and confirm every removed helper/shadow path is absent without introducing replacement dead code.

## 4. Verification and Review

- [x] 4.1 Run `gofmt -w` on edited Go files and the focused engine/root/app tests while implementing each workstream test-first.
- [x] 4.2 Run `go test ./...`, `go test -race . ./internal/engine ./internal/app`, repeated shuffled tests, `go vet ./...`, and `make build` locally.
- [x] 4.3 Run `@open-code-review`, classify every finding, fix accepted High/Medium issues, and rerun affected local gates.
- [x] 4.4 Sync the exact checkout to nlab; run Debian full/race/build gates and focused native Windows, FreeBSD, and Plan 9 coverage for defaults, external facts, and networking parity.
- [x] 4.5 Run the remaining supported-guest release gates required by any platform-sensitive diff and record evidence before pull-request publication.
