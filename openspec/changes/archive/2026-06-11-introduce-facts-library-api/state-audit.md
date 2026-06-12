# internal/facter package-level state audit (task 2.1)

Every package-level `var` in `internal/facter`, classified. Categories:

- **session** — mutable resolution state; moves onto the per-run `Session` struct threaded through resolution. A fresh Session per discovery run gives engine isolation and the "freshness = discover again" semantics.
- **seam** — test seam (function/value var stubbed by tests, never mutated in production). Stays package-level; not user-visible state.
- **immutable** — constants, error sentinels, compiled regexps, lookup tables. Stays.

## session (move to Session struct)

| Var | File | Notes |
|---|---|---|
| `coreFactsCache` | core.go:33 | memoized core fact list keyed by includeRuby; `ClearCoreFacts` becomes a Session method (root API holds a current Session) |
| 32 × `cached*` (`sync.OnceValue`/`OnceValues`) | core.go | host probes (architecture, kernel, os-release, macOS profiler, distro, memory, swap, processors, uptime, loadavg, filesystems, augeas/zfs/zpool, windows version). Probe closures become `probeX(s *Session)` funcs; probes call each other (memory→meminfo, processors→platform info, …) so they need the session |
| `customFactSourceCache` | custom.go:1370 | custom-fact file source memo; per-session so engines see fresh files per run |
| `sessionCache` (subscribers) | cache.go:31 | orphaned invalidator registry (only referenced by its own tests); folded into Session/removed |
| `diagnosticState` (handlers) | external.go:36 | process-wide debug/warn/error callbacks; kept process-wide through group 2, replaced by per-engine `log/slog` in task 3.5 |
| custom DSL `coreCollection` | custom.go:429/1086 | `Facter.value` recursion resolves against `CoreFacts()`; scoped to Session (task 2.4) |

## seam (stays package-level)

`cacheRemove`, `cacheWriteFile` (cache.go); `DefaultCachePath` (cache.go); `DefaultConfigPath` (config.go); `externalFactCommandTimeout`, `externalFactGOOS`, `externalFactRunCommand`, `externalFactFileReadable`, `externalFactOpen` (external.go); `customCommandRunner`, `customCommandOS`, `customFactReadFile` (custom.go); `puppetGOOS`, `puppetGeteuid`, `puppetCacheDirFn` (puppet.go). Plus `internal/app.defaultExternalFactDirs`.

These are stubbed in tests only. They stay package-level vars; two-engine isolation tests must not stub them concurrently with different values.

## immutable (stays)

Error sentinels (`ErrUnknownOS`, `ErrNullByte`, `errCustomFactRead`, directed-graph errors), all compiled regexps (dsl_diagnostics.go, custom.go, virtual.go), lookup tables (`legacyAliasesByCoreFact`, `dmiDecodeVMwareVersions`, `wmicAliasClasses`, `freeBSDDMIKeys`, `openBSDDMIKeys`, `supportedSetcodeFactREs`), `Version` const.

## Threading plan

`Session` (new `internal/facter/session.go`): generic `memo[T]` fields for the probes plus the caches above. Entry points gain a `*Session` parameter: `CoreFacts`, `CoreFactsWithRuby`, `LegacyFacts`, `LoadCustomFacts`, custom DSL evaluation, and the transitive helpers that consume probes (`buildCoreFacts`, `detectVirtualization`, `currentOSRelease`, `currentMacOSModel`, `currentProcessorISA`, `linuxDistroFacts`, …; the compiler enforces completeness). The root package keeps one process-wide current Session (replaced on `Flush`/`Reset`/`Clear`, preserving Ruby-API lifecycle semantics) until task 5 deletes it; `internal/app.Run` creates a Session per invocation.

Found during task 1.3, relevant to 3.5/4.1: `SetErrorHandler` has no callers — `reportError` diagnostics (collisions, custom raise/timeouts, invalid UTF-8, cache-group errors, TTL parse) are currently silent at the CLI; the slog rewire must keep them off stderr to preserve the output contract.
