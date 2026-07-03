## Context

`internal/engine` already has one real host-I/O seam: `Session.host` (the unexported `hostOS` interface) with two adapters — `osHost` in production, `fakeHostOS` in tests. But most resolvers predate it, so a second seam made of function-typed `commandRunner`/`fileReader` parameters and `FromRoot`/`WithHost`/`WithReader`/`WithRunner`/`ForPlatform` variant families runs in parallel, `core.go`'s reader helpers silently fall back to a raw `osHost{}` when the optional reader is omitted, and a handful of call sites bypass both seams with raw `os.ReadDir`/`os.Stat`. The same pattern repeats one level up: category assemblies read `runtime.GOOS` and `os.Getenv` directly instead of `s.goos()`/the host environ, the expensive virtualization gather is the only repeated probe without a Session memo, and several modules keep test-only or dead entrances (`detector.go`, `query.go` delegates, `LoadExternalFacts`, `gceFacts`, `filehelper.go`, the fast-path crutch exports, the bare `Format*` functions).

This change collapses each of those onto the seam that already owns the concern. It is the deferred follow-on recorded in ADR-0010 and the archived 2026-06-17 deepen-engine-internals design, extended by six adjacent deepenings found by the same deletion-test review. Every candidate here was deep-dived at implementation depth and adversarially red-teamed; the red-team corrections below are binding, not advisory.

Hard constraints restated (decided, not re-litigated):

- ADR-0002: canonical dynamic tree only.
- ADR-0003: hermetic-vs-system-following split — `config.go`/`cache.go` default-path GOOS reads stay outside the Session seam (they are the system-following CLI layer and have no Session in scope).
- ADR-0005: memos are Session-resident per discovery, never Engine-resident; `envValue` is a per-call scan, not a memo.
- ADR-0010: pure `parse*`/`detect*(input)`/goos-string-parameter signatures unchanged; no GOOS-suffixed filenames for cross-platform logic.
- Output contract and input contract are binding: zero user-visible change. Contract test files may not be modified.

## Goals / Non-Goals

**Goals:**

- Make the Session host seam the only resolver host-I/O path, structurally enforced by a gate test.
- Make category platform dispatch and env lookup flow through Session accessors so windows/plan9 assembly paths are exercisable with a fake host, and the ssh/fips gating-test skips disappear.
- Run the host-virtualization gather once per discovery, with `virtual`/`is_virtual`, `hypervisors.*`, and the uptime container gate reading one memoized input.
- Delete every dead or test-only entrance found by the review: `detector.go`, the `query.go` delegates, `LoadExternalFacts`/`WithBlocklist`, `gceFacts`, `filehelper.go`, `EnvironmentDisabledFacts`, `DisabledFactsForFiltering`, and the production exports of the four bare `Format*` functions.
- Concentrate the cloud-metadata transport invariants (proxy-less client, 200-required, 1MB cap, fail-closed) in one helper with the first tests to ever assert them.
- Collapse Discover's duplicated external-loader arms and `internal/app`'s disabled-union mirror onto their owners.
- Converge resolver tests on `fakeHostOS` under a same-assertions-relocated-fixtures rule.

**Non-Goals:**

- No deletion of the `probe*` memo-filler layer — those functions are wired to `Session.cached*` and most contain real platform dispatch; only the parameter-taking `current*`/`ForPlatform`/`With*` siblings collapse.
- No conversion of single-seam parameterized functions whose readers/runners are already bound once from a Session-holding assembly (the packages per-source listers — freshly reviewed code, `windowsWMIOutput`, plan9 helpers, `snapshotProvider`).
- No new `hostOS` methods for `getenv`/`now`/`lookPath`/`net.Interfaces` — those parameters survive. Accepted, recorded leaks: `exec.LookPath` in the linux distro probe (fires only when goos=="linux", degrades gracefully; it also bypasses the hardened-PATH policy — a separate follow-up) and identity's uid/gid/`osuser.Current` syscalls.
- No loader-owns-policy redesign and no `(facts, fatal, soft)` dual-error return — the error truth table already lives in the loader; Discover keeps one documented mode-conditional branch.
- No exposing of engine plan external-dirs and no Engine plan-inputs method — the `internal/app` dir merge feeds CLI-owned `--no-external-facts`/`--external-dir` conflict validation and the list-groups path, so engine exposure removes nothing (and a session-less plan method would invite ADR-0005 scrutiny for zero gain).
- No change to the two classification ladders in `virtual.go` — the kubepods divergence is test-pinned, and reconciling them is behavior-affecting work for a separate change.
- No formatter rewrite, no `Projection`/`FactCache` changes, no fact schema or release-target change.

## Decisions

**1. Collapse onto the existing seam; delete the duplicating one.**

The Session/`hostOS` seam wins because it already has two adapters (`osHost`, `fakeHostOS`) — a real seam by definition — while the parameter threading is a per-function hypothetical seam re-invented ~126 times. I/O-doing `current*`/`detect*`/`discover*` functions take the Session (or `hostOS`) and production paths are hardcoded (`rootedPath(root="/")` is identity, so path strings are unchanged). `core.go`'s `readFileString`/`isSymlink`/`readSysfsString`/`readDMIString` take a required reader — a signature-only edit; all 17 production call sites already pass one explicitly.

Red-team corrections carried: networking is 10 variants→4 functions (metadata, bonding, `linuxDHCPServer`, `linuxDHCPServerFromLeaseDir`); dmi has FIVE `current*DMIFacts`/`ForPlatform` pairs (FreeBSD, DragonFly, OpenBSD, NetBSD, Illumos) plus `currentWindowsDMI` (dmi.go:415) which must be folded in or explicitly excluded with a recorded reason; xen's remaining closure seam is the `detectXenDomains` pair only (`detectXenVM` is already Session-based); uptime keeps its `now func() time.Time` parameter.

Rejected alternative: also deleting the 30 `probe*` functions (the reviewed candidate's original shape). They are the memo-filler layer wired at session.go:326-429 and most contain real dispatch logic; deleting them is relocation churn with no seam removed.

**2. A grep-gate test freezes the collapsed state.**

`TestNoRawHostIOInResolvers` asserts no `os.ReadDir`/`os.ReadFile`/`os.Stat`/`os.Lstat`/`filepath.Glob`/`exec.Command` outside the canonical exclusion list: `session*.go`, `external.go`, `cache.go`, `config.go`, `statfs*`, and `*_test.go`. `filehelper.go` is deleted (dead code — zero callers outside its own test) rather than excluded. Without the gate, the leak class regrows; with it, the collapse is structural, not conventional.

**3. Extend `fakeHostOS` additively only, before any migration.**

Confirmed gaps: errors are hardwired to `os.ErrNotExist` (tests injecting `os.ErrPermission` lose coverage), unmatched `run()` falls through to the sentinel `"host-output\n"` ("every command returns empty" is inexpressible), and `fakeDirEntries` hardcodes every entry as a directory — while the bonding and DHCP lease-dir loops skip `IsDir` entries, so migrating those tests without a file-entry helper makes want-empty tests pass vacuously. Additions: per-path error maps, an explicit empty-run-default knob, and a file-entry helper. Defaults never change — existing fake-based tests keep their meaning. No per-call sequencing feature is needed (closure counters only assert was-called, which the existing `runCalls`/`readDirCalls` recorders cover). Test migration follows a same-assertions-relocated-fixtures rule: `t.Fatal`-on-call closures become `len(h.readDirCalls)==0` assertions, injected error types become error-map entries; assertions are never dropped.

**4. One pure env helper, goos-conditional, fed by the host.**

`envValue(env []string, goos, name string)` is case-insensitive ONLY when `goos=="windows"` (first match wins), exact elsewhere — plan9's lowercase `path` and unix case-sensitivity are preserved. (The existing `systemRootFromEnv`'s unconditional `EqualFold` is the wrong template.) `currentPathEntries` switches its separator from compile-time `os.PathListSeparator` to the existing `corePathListSeparator(goos)`. The orphaned `discoverSSHHostKeys` and `currentWindowsProcessWOW64` thin wrappers are deleted. The whole `runtime.GOOS`→`s.goos()` conversion class (~30 sites) is one intentional latent-drift fix: production-identical because `osHost.goos()` returns `runtime.GOOS` and the only production Session constructor uses `osHost{}`.

**5. Memoize the virtualization gather's raw input, not its classification.**

Two Session memos — `linuxVirtualization memo[linuxVirtualizationInput]`, `windowsVirtualization memo[windowsVirtualizationInput]` with `cached*Input()` accessors (cachedDMI pattern) — routed to the five gather call sites (detectVirtualization linux+windows branches, both hypervisor fact builders, the uptime container gate). Classification stays pure and derives on demand, so the untouched ladders keep their pinned outputs. Red-team correction: the `currentLinuxVirtualizationInputWithCommands` wrapper KEEPS existing until the session-io group folds it (it has a live test caller at virtual_test.go:1488, which migrates to `fakeHostOS` in that same fold).

Rejected alternative: memoizing the classified result — it would fuse the two ladders' inputs and outputs and invite exactly the reconciliation this change excludes.

**6. Cloud transport helper is stateless functions, not an interface.**

`metadatahttp.go`: `newMetadataHTTPClient(timeout)` and `fetchMetadata(ctx, client, method, url, headers) (body, respHeader, ok)` plus one shared 1MB cap const. Four plumbing copies (ec2 `getRaw`, ec2 `v2Token` request leg, gce `get`, az `metadata`) and three nil-default ctors convert. Provider files keep everything provider-specific: EC2 token cache/TTL and conditional token injection, untrimmed userdata, GCE `Metadata-Flavor` response validation (via the returned header) and TrimSpace, Azure JSON/empty-map error shape, hypervisor/BIOS gates, per-provider timeouts (az 5s, ec2/gce 100ms). Ctor signatures `(baseURL, *http.Client)` — the httptest injection seam — are unchanged. Dead `gceFacts` is deleted first; six tests reference it (five in gce_test.go, one in ec2_test.go): retarget to production entry points with production empty-case expectations (`{gce: nil}`), deleting `TestGCEFactsSkipNilClient` as an exact duplicate of the existing nil-client test.

Rejected alternative: a provider interface with method/header/flavor hooks — permanent interface surface for three call sites; functions suffice.

**7. Discover keeps the abort decision; the loader keeps the policy it already owns.**

The two near-identical loader arms in `Discover` (engine.go:199-221) collapse to one construction and one `load()` call with a single commented abort-vs-accumulate branch on `plan.loaderMode`. The CLI error path stays byte-identical: `return newSnapshot(nil, s.logger), err` — bare loader error, earlier planFailures discarded, `finish()`'s ctx.Err() join skipped; the library path appends to failures and keeps partial facts (`externalFacts` assigned unconditionally). `load()`/`loadDirFacts` internals and the verified error truth table (env null-byte: CLI hard/library soft; dir-read ErrNotExist: both silent; dir-read other and stat: CLI hard/library soft; exec-class incl. timeout and oversized output: CLI silent-skip) are untouched. The test-only `LoadExternalFacts`/`LoadExternalFactsWithBlocklist` facade is deleted; its ~40 call sites retarget to a field-for-field test-local helper (mode CLI, includeEnv true, default host). This resolves the archived 2026-06-17 open question that marked the facade for deletion.

Rejected alternatives: the full loader-owns-policy redesign and the `(facts, fatal, soft)` return — the mode field already drives all five branch decisions inside the loader; only `load()`'s final line flattens the outcome, and its sole production caller sits three lines from where the mode is set. Both alternatives churn ~20 loader-construction test sites for zero behavior delta.

**8. The fast path asks the engine's owners; the decision stays in the CLI.**

Per the deepen-discovery-input-surface design (fast-path handling and formatter selection stay in `internal/app`): (a) refactor the engine's `unionDisabledFacts` into a pure core taking `environ []string` and export one pure `DisabledUnion(config, extraDisabled, environ)`; `internal/app` replaces its admitted 8-line mirror with one call and the two crutch exports (`EnvironmentDisabledFacts`, `DisabledFactsForFiltering`) are deleted. (b) `writeVersionQuery` routes through `engine.BuildFormatter` with FormatOptions carrying ONLY the three format booleans — Colorize/IncludeTypedDotted stay false because the fast path deliberately ignores `--color`/`--force-dot-resolution` today. The four bare `Format*` functions move verbatim into an in-package `_test.go` helper file: 102 formatter_test.go call sites plus 3 benchmarks stay untouched while the production build loses the exports.

Rejected alternative (killed at deep-dive): exposing engine plan external-dirs — the app-side dir merge feeds CLI-owned conflict validation and the list-groups path, so engine exposure strictly adds surface.

**9. One change, ordered task groups, each landing green.**

Order: delete-dead → fakeHostOS upgrade → goos-seam → virt-memo → session-io bulk → cloud-fetch → loader-policy → fastpath → cross-cutting verification. Rationale: the resolver trio rewrites the same files, so goos-seam establishes the platform/env seam first, virt-memo defines the final gather call shape, and the session-io bulk collapses onto already-final seams — each line touched once. Intra-file sequencing (binding): session.go/session_test.go additive in order 2→3→4; os.go, dmi.go, fips.go, identity.go: goos-seam call-site edits first and kept minimal (session-io's merges subsume them); uptime.go strict order goos-seam→virt-memo→session-io; virtual.go: virt-memo routes memos, session-io folds `WithCommands`; networking tasks (bonding vs DHCP) are sequential, never parallel; ec2.go: session-io's `fileExecutable` hunk before cloud-fetch's client hunks; external.go: loader-policy's facade deletion before fastpath's crutch-export deletion. The probeLinuxDistro `runtime.GOOS` fix belongs to goos-seam alone (struck from session-io).

Rejected alternative: seven separate openspec changes — cross-change sequencing bookkeeping for files each change would re-touch; the archived deepen-engine-internals precedent bundled five seams the same way.

## Risks / Trade-offs

- **[Test migration silently weakens coverage]** → Same-assertions-relocated-fixtures rule checked per task group in review; fake upgrades are additive knobs only (defaults never change meaning); the two want-empty DHCP lease tests get the file-entry fake helper so they cannot pass vacuously.
- **[DHCP lease iteration order]** → Production is unchanged (`osHost.readDir` == `os.ReadDir`, sorted); `fakeHostOS` documents and enforces the sorted-order convention in `fakeDirEntries`; the existing lease-order tests keep asserting the same outcomes.
- **[Fake-host-only output delta at the GCE site]** → After the goos-seam conversion, platform-set fake sessions emit `{gce: nil}` where they previously emitted nothing (no network call involved). No current test asserts against it; documented here so it is not mistaken for a production change.
- **[Windows env-var case-insensitivity]** → `envValue` is goos-conditional; nlab Windows guest smoke (env-casing, windows gather memo) gates archive.
- **[plan9 env divergence (lowercase `path`, NUL separators, case-sensitive)]** → plan9 guest smoke asserting the `path` fact is non-empty and NUL-split correctly — or a recorded accepted risk if the guest is unavailable at archive time.
- **[Virtualization memo count-tests are brittle]** → Count by exact command name (fakeRunKey-style): ec2 runs a path-qualified `/opt/puppetlabs/puppet/bin/virt-what` and the BSD DMI probe runs `/usr/local/sbin/dmidecode`, so substring counting miscounts; pin `uname -r` (the gather calls `cachedKernelRelease`); pin wmic outputs containing `=` or count the powershell CIM fallback keys — otherwise the windows gather is 5 commands, not 3.
- **[Reviewer fatigue on a test-dominated ~4:1 diff]** → Per-group commits each landing `go test ./...` green; three untouchable gates (contract tests, full test suite, Lima Facter 4.10.0 parity diffs bracketing the resolver trio and after fastpath).
- **[Loader pinning fixtures are platform-fragile]** → Library-mode soft-failure fixture is a second null-byte external FILE (permission-denied dirs break on Windows/root; NUL env vars are unconstructible via t.Setenv); CLI-mode fixture uses a malformed facter.conf for planFailures, or drops that vacuous assertion.
- **[In-flight change collisions]** → Lands after deepen-discovery-input-surface and fix-linux-dhcp-lease-interface-match archive; core.go/core_gating_test.go/fastpath work lands after add-fact-disable-controls archives or rebases against its deltas; the facts-library-api spec delta targets the diagnostics requirement by name, not line number, because that spec file gains a requirement when the in-flight change archives.

## Migration Plan

No deployment migration — internal refactor, binary behavior identical. Rollback is per task group: each lands independently green, so any group can be reverted without unwinding the others (within a file, later groups depend on earlier ones per the sequencing above). Verification brackets: Lima VM Facter 4.10.0 parity diff before the resolver trio, after it, and after fastpath; nlab Windows + plan9 guest smokes gate archive, not individual tasks.

## Open Questions

- ~~`currentWindowsDMI` (dmi.go:415): fold into the dmi pair-merge or exclude with a recorded reason.~~ **Resolved:** folded into the Session-taking dmi shape (it takes `s.logr()`, same pattern as the five `ForPlatform` merges).
- ~~plan9 guest availability at archive time determines smoke-vs-accepted-risk for the `path` fact check.~~ **Resolved (validated on real guests):** the nlab Windows Server 2025 and Plan 9 (9front) guests were reachable and both smokes ran — see the Verification Record.

## Verification Record

- **Lima real-Linux behavior-preservation (9.1):** on the `facts-dev` VM (Facter 4.10.0 host), `facts --json` from this branch was diffed against a `main`-built binary. The stable fact subset is byte-identical; the only differences are inherently-volatile live measurements (memory `used_bytes`, disk `available_bytes`/`capacity`) that drift between any two sequential runs. Behavior-preserving on real Linux confirmed.
- **nlab real-guest smokes (9.2):** main and branch binaries were cross-built on the nlab host and run on the Windows Server 2025 and Plan 9 (9front) guests. Both diffed stable-subset byte-identical to `main` (only volatile uptime/memory/disk facts differed). The env-casing paths validate on real Windows — `os.windows.system32` = `C:\WINDOWS\system32` and the `path` fact leads with it, proving `s.getenv("SystemRoot")` matches the old `os.Getenv`. On Plan 9 the `path` fact is `["/bin", "."]` — non-empty and NUL-split correctly.
- **Full sweep (9.4):** `go build ./...`, `go vet ./...`, `go test ./...` (2326 tests), and `go test -race ./internal/engine/ ./internal/app/` all green; contract test files unchanged vs `main`; `openspec validate deepen-engine-seams --strict` passes.
