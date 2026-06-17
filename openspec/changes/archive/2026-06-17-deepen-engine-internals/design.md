# Design: Deepen engine internals

## Context

Five findings from a deletion-test pass over `internal/engine`, all behavior-preserving:

1. **Host I/O.** `Session` is the per-run host-probe carrier (`session.go:9-13`) and already exposes `Session.commandOutput(name, args...)` backed by `exec.CommandContext(s.ctx, …)` (`core.go:2864`). But it is used inconsistently: 29 `probe*` functions take `*Session`, ~53 helper functions instead take injected `commandRunner`/`fileReader` parameters (37 + 16), and 47 call sites in `core.go`/`virtual.go` reach `exec.Command`/`os.ReadFile`/`os.Stat` *directly*, bypassing every seam. The direct calls are the only ones that cannot be faked. Concrete smell: `currentMountEntries(s *Session)` holds a Session yet calls `exec.Command` (2437, 2443) and `os.ReadFile` (2449) directly.
2. **External facts.** External facts are the input contract's most sensitive loader, but the current implementation encodes its two semantics through `failures == nil` (`external.go:137-190`): nil means CLI mode (skip executable/context failures, first hard error aborts), non-nil means library mode (collect partial failures and continue). Environment facts are also split: CLI mode loads them inside `LoadExternalFactsWithBlocklist`, while library `SystemDefaults` appends them from `Engine.Discover` (`engine.go:197-205`). Tests mutate globals (`externalFactGOOS`, `externalFactRunCommand`, `externalFactOpen`) to fake platform/filesystem/command behavior.
3. **Config.** `readConfigOptions` (`config.go:138-150`) is already the fully-parsed value; the four `Config*` accessors each re-run `readConfigOptionsFile` → `os.ReadFile` + parse. Two independent paths re-parse redundantly: the CLI reads up to 5× (`app.go:296,320,324,386,390`, fact-groups twice), and — separately, for library consumers that opt into a config file — `Engine.Discover` reads 4× (`engine.go:139,148,152,162`). They are not additive in one run: the CLI builds its `EngineConfig` without `ConfigFile`/`SystemDefaults`, so `Discover`'s re-parse (gated at `engine.go:137`) fires only on the library path. Both paths benefit from parse-once.
4. **Formatter.** `Formatter`/`BuildFormatter` (`formatter.go:20-48`) are dead because `FormatOptions{JSON,YAML,HOCON}` has no dotted-merge or color field; the CLI hand-rolls a switch over `Format*WithDottedFacts`/`FormatLegacyColored` (`app.go:402-411`).
5. **Dead module.** `directed_graph.go` (131 lines) + test (89 lines): zero production references.
6. **Ruby/Puppet package-version facts.** The current core surface still includes package/runtime inventory from the old stack: `ruby`, `aio_agent_version`, and `--puppet`-driven `puppetversion`. These require probing Ruby/Puppet installations and are no longer appropriate for a Go-native Facts binary.

## Goals / Non-Goals

**Goals:**
- One swappable host seam reachable from any resolver that holds a `Session`; the 47 direct calls route through it and become fakeable.
- One explicit external-fact loader seam with named CLI/library semantics, not sentinel-driven behavior or package-global test hooks.
- Config parsed once per run, on both the CLI and `Engine.Discover` paths.
- The `Formatter` seam carries everything the CLI needs, so the CLI uses it instead of a switch.
- Dead directed-graph module removed.
- Ruby/Puppet package-version built-ins removed from the output contract.
- Input contract byte-for-byte unchanged, and the remaining output contract proven by the existing contract/acceptance suites staying green.

**Non-Goals:**
- **Collapsing the `commandRunner`/`fileReader` parameters into `s.*` (the ~53 helper signatures + ~29 test call sites).** Those helpers are already fakeable via closures; rewriting them is high churn for marginal gain. Deferred to a follow-on change once the bypasses are gone.
- **The niche injected seams** — `now func() time.Time` (a clock, not host I/O; 5 functions), `lookPath`, `stat`-as-param, `lookup`, `snapshotProvider`, `isWOW64`. Low volume, already testable; folding them into the host interface is over-abstraction.
- **Ruby DSL or custom-fact revival.** The loader module keeps `.rb` skipping exactly as-is (ADR-0006). It does not add Ruby parsing, restore custom-fact flags, or read `FACTERLIB`.
- **Removing Puppet external-fact compatibility paths.** The `--puppet` plugin `facts.d` lookup remains because it loads operator-supplied external facts, not Puppet package-version inventory.
- **Public external-fact loader API.** This is an internal seam. The public `facts` package keeps `WithExternalDirs`, `WithSystemDefaults`, and existing discovery behavior.
- **Broad diagnostics rewrite.** External fact diagnostics may move behind the loader's host/diagnostic adapter where needed, but process-wide diagnostic cleanup outside external facts stays out of scope.
- **Dogfooding the public `facts` facade from the CLI.** Rejected: the CLI-only `EngineConfig` knobs are a deliberate decision (`engine.go:49-63`, ADR-0001) to keep the public API small. Out of scope, and not a defect.
- No new facts, no formatter output changes, no config-semantics changes, no `docs/schema/facts.yaml` change.

## Decisions

**1. Host seam = a small unexported interface on the Session, defaulting to the real OS.**
```go
type hostOS interface {
    run(ctx context.Context, name string, args ...string) string
    readFile(path string) ([]byte, error)
    stat(path string) (os.FileInfo, error)
    lstat(path string) (os.FileInfo, error)
}
```
`Session` gains a `host hostOS` field; `NewSessionContext` defaults it to `osHost{}` whose methods are exactly today's call bodies (`exec.CommandContext(ctx, …)`, `os.ReadFile`, `os.Stat`, `os.Lstat`). `Session.commandOutput` delegates to `s.host.run(s.ctx, …)`; new `s.readFile`/`s.stat`/`s.lstat` delegate likewise. An interface (not func fields) because two adapters justify it — the real OS in production, one fake host in tests — and the method set is small and fixed. The exact method set is finalized during apply against the real 47 call sites; `open`/`exists` fold into `readFile`/`stat` unless a call genuinely needs a streaming handle.

**2. Route the bypasses; leave the already-injectable params alone (this change).**
The 47 direct calls in functions that hold a `Session` move to `s.run`/`s.readFile`/`s.stat`. For the direct calls in non-Session helpers that remain after removing Ruby package/runtime facts (`detectXenDomains` 1124, `addLinuxRouteSourceBindings` 1996/1999), thread the `Session` in (cheap — their callers have one). The `current*(… commandRunner, fileReader)` helpers keep their parameters; their `probe*` callers pass `s.run`/`s.readFile` instead of `s.commandOutput`/`os.ReadFile`. Net: the seam is unified at the *call* level (everything goes through the host) while the helper *signatures* stay put for now.

**3. External facts: one loader module, explicit mode, fakeable host.**
Introduce an unexported external-fact loader module that owns directory facts, environment facts, executable facts, PowerShell facts, blocklist filtering, recursive-resolution guards, null-byte handling, and diagnostic emission for external facts.

The loader has an explicit mode enum/value for the two existing semantics:
- **CLI mode**: matches `LoadExternalFactsWithBlocklist` today — includes environment facts, silently skips executable and cancelled-context failures, and returns the first hard source error.
- **Library mode**: matches the former `LoadExternalFactsFromDirs` semantics plus `SystemDefaults` env behavior — returns partial facts and joined per-source failures, and includes environment facts only when system defaults opt in.

The loader has a small internal host adapter for the operations external facts actually need: directory reads, file open/readability, environment, platform name, command execution, and recursion marker access. The exact method set is finalized during apply from `external.go`; do not add methods for hypothetical future fact sources. Tests use one fake loader host instead of mutating `externalFactGOOS`, `externalFactRunCommand`, or `externalFactOpen`.

Keep the old exported internal helpers only if callers still need compatibility inside `internal/engine`; otherwise collapse them into the loader. No public `facts` package surface changes.

**4. Config: `ParseConfig(path) (Config, error)` once; `Config` is the parsed value.**
Export `readConfigOptions` as `Config` (it already has every field). `ParseConfig` runs `readConfigOptionsFile` once. The CLI and `Engine.Discover` call `ParseConfig` once and read fields directly (`cfg.Blocklist`, `cfg.FactGroups`, `cfg.TTLs`, `cfg.ExternalDirs`, …). The four single-aspect package functions (`ConfigBlocklist`, `ConfigFileOptions`, `ConfigTTLs`, `ConfigFactGroups`) are removed and their call sites migrated; `config_test.go` migrates to asserting fields on a parsed `Config`. `ConfigOptions` (the subset type) is dropped or kept as a view — implementation detail.

**5. Formatter: widen `FormatOptions`, wire `BuildFormatter`, call it from the CLI.**
Add `IncludeTypedDotted bool` and `Colorize bool` to `FormatOptions`. `BuildFormatter` wires them: json → `FormatJSONWithDottedFacts(facts, o.IncludeTypedDotted)`, yaml/hocon likewise, legacy → `FormatLegacyColored(facts, o.IncludeTypedDotted, o.Colorize)`. Machine formats ignore `Colorize` (unchanged behavior; matches the format-parity ADR — only the default text format colorizes). The CLI builds one `FormatOptions` from its flags + `mergeDottedFacts` + resolved `colorOutput`, then `out, err := BuildFormatter(opts).Format(facts)`, replacing the four-branch switch.

**6. Delete `directed_graph.go` + `directed_graph_test.go`.**
Verified zero production references. Pure subtraction.

**7. Remove Ruby/Puppet package-version facts; keep external facts.**
Delete `rubyInfo`, `resolveRubyInfo`, `parseRubyInfo`, `rubyFacts`, `aioAgentVersion`, and `currentAioAgentVersion`. `CoreFacts` no longer accepts or caches an `includeRuby` dimension; there is no `CoreFactsWithRuby` because Ruby is no longer a built-in fact source. `Engine.Discover` stops appending `PuppetFacts`, and the Puppet executable version helper is removed. Keep `PuppetPluginFactDirs` and the Ruby-plugin warning under `--puppet`, because they are about external-fact input compatibility and operator migration, not package-version facts.

## Risks / Trade-offs

- **[Host realHost must replicate current semantics exactly]** → `osHost` methods are the verbatim current call bodies (same ctx, same output trimming in `commandOutput`); covered by the unchanged engine/contract tests + a `-race` run (AGENTS.md flags engine changes as concurrency-sensitive).
- **[External loader mode must not blur CLI and library semantics]** → mode is explicit and covered by tests that pin executable failure handling, context cancellation, environment fact inclusion, blocklist filtering, and partial-error behavior in both paths.
- **[External loader host adapter could grow too wide]** → method set is finalized from today's `external.go` operations only. No generic filesystem abstraction.
- **[Parse-once changes duplicate diagnostics]** → today a malformed/unreadable config file `warn()`s once per `Config*` call (up to 9×); parse-once emits it once. This is observable on stderr. Audit `app_test.go`/`config_test.go` for any test that counts the duplicate warnings; if one pins the count, update it and note the convergence (one warning is the correct behavior). **Open question below.**
- **[`config_test.go` migration, 922 lines]** → accessors return identical values, so assertions change call shape, not expected data; mechanical.
- **[Two ways to do host I/O remain after this change]** (`s.*` calls and the surviving `commandRunner`/`fileReader` params) → accepted, by design: the untestable bypasses are gone, and the param collapse is a clean follow-on. Recorded so it isn't read as an oversight.
- **[Removing `puppetversion` could be confused with removing `--puppet`]** → keep `--puppet` for plugin external facts and its migration warning; only the Puppet executable/package-version fact is removed.

## Migration Plan

Land in five independently-revertable steps, each `go build` + `go test ./...` green before the next (run `go vet` and `gofmt -w` per AGENTS.md):
1. Delete the dead directed-graph module (trivial, isolated).
2. External-fact loader module; migrate `Engine.Discover` and external-fact tests.
3. `ParseConfig`/`Config`; migrate CLI + `Discover` + `config_test.go`.
4. `FormatOptions` fields + `BuildFormatter` wiring; migrate the CLI; formatter tests.
5. Host seam on `Session` + route the 47 direct calls; `-race` on `. ./internal/engine ./internal/app`.
6. Remove Ruby/AIO/Puppet package-version facts; update schema, changelog, help/man text, and contract tests.

Final gate: full suite + `-race`, then a side-by-side CLI output check against the pre-change binary (must be byte-identical modulo volatile values and the intentional `ruby`/`aio_agent_version`/`puppetversion` removals). Rollback: revert the offending step's commit.

## Open Questions

- **Duplicate config-read warnings.** Parse-once converges up-to-9 identical "failed to read config" warnings to one. Treat the single warning as correct and update any test that pinned the old count? (Recommended: yes — the duplicate emission was an artifact of re-parsing, not intended behavior.)
- **External loader compatibility helpers.** Keep thin `LoadExternalFacts*` functions as internal compatibility wrappers during migration, or delete them immediately once all callers use the loader? (Recommended: delete if all callers migrate in the same change; keep no pass-through modules.)
- **Host method set.** Finalize `hostOS` to exactly the operations the 47 calls need (`run`/`readFile`/`stat`/`lstat`, possibly `open`) during apply — don't speculatively add methods.
