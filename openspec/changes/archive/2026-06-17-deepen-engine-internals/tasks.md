# Tasks: Deepen engine internals

Land in order; `gofmt -w`, `go vet ./...`, and `go test ./...` green before moving to the next section.

## 1. Delete the dead directed-graph module

- [x] 1.1 Remove `internal/engine/directed_graph.go` and `internal/engine/directed_graph_test.go`
- [x] 1.2 `go build ./...` and `go test ./...` confirm zero references (no other file used it)

## 2. Make external facts a real loader module

- [x] 2.1 Introduce an unexported external-fact loader module with explicit CLI/library modes, replacing the `failures == nil` sentinel in `loadExternalDirFacts`
- [x] 2.2 Move directory facts, environment facts, executable facts, PowerShell facts, recursion guards, null-byte handling, blocklist filtering, and external-fact diagnostics behind that loader module
- [x] 2.3 Add a small loader host adapter for today's external-fact operations only (directory reads, file open/readability, environment, platform, command execution, recursion marker); remove or stop using `externalFactGOOS`/`externalFactRunCommand`/`externalFactOpen`
- [x] 2.4 Migrate `Engine.Discover` to delegate external fact loading through the loader, preserving current CLI mode and library/SystemDefaults mode behavior
- [x] 2.5 Update external-fact tests to use a fake loader host and pin both modes: executable failures, context cancellation, environment fact inclusion, blocklists, recursive-resolution guards, null-byte rejection, and warnings

## 3. Parse the config once

- [x] 3.1 Export the parsed value as `Config` and add `ParseConfig(path) (Config, error)` calling `readConfigOptionsFile` once; expose fields/accessors for options, external dirs, blocklist, fact-groups, TTLs
- [x] 3.2 Migrate `internal/app/app.go` to call `ParseConfig` once (replacing the 5 calls at 296/320/324/386/390) and read fields
- [x] 3.3 Migrate `Engine.Discover` (`engine.go:137-169`) to call `ParseConfig` once (replacing the 4 calls at 139/148/152/162)
- [x] 3.4 Remove the now-unused `ConfigBlocklist`/`ConfigFileOptions`/`ConfigTTLs`/`ConfigFactGroups` (and `ConfigOptions` if no longer referenced); migrate `config_test.go` to assert fields on a parsed `Config`
- [x] 3.5 Audit `app_test.go`/`config_test.go` for any test pinning duplicate "failed to read config" warnings; update to the single-warning result (parse-once converges them)

## 4. Activate the Formatter seam

- [x] 4.1 Add `IncludeTypedDotted` and `Colorize` to `FormatOptions`; wire them through `BuildFormatter` (legacy → `FormatLegacyColored`; json/yaml/hocon → `*WithDottedFacts`; machine formats ignore `Colorize`)
- [x] 4.2 Replace the CLI's four-branch switch (`app.go:402-411`) with one `BuildFormatter(opts).Format(facts)`, building `FormatOptions` from the format flags + `mergeDottedFacts` + resolved `colorOutput`
- [x] 4.3 Formatter tests cover `BuildFormatter` for each format with `IncludeTypedDotted`/`Colorize` on and off; confirm machine formats are byte-identical regardless of `Colorize`

## 5. Carry host I/O on the Session

- [x] 5.1 Add the `hostOS` interface + default `osHost{}` (methods = today's `exec.CommandContext`/`os.ReadFile`/`os.Stat`/`os.Lstat` bodies); add the `host` field to `Session`, defaulted in `NewSessionContext`; add a test helper to construct a Session with a fake host
- [x] 5.2 Route `Session.commandOutput` through `s.host.run`; add `s.readFile`/`s.stat`/`s.lstat` delegating to `s.host`
- [x] 5.3 Replace the direct `exec`/`os` calls in `core.go`/`virtual.go` with `s.*`; thread `*Session` into the non-Session helpers that still need host I/O (`detectXenDomains`, `addLinuxRouteSourceBindings`); fix `currentMountEntries` to use `s.*`
- [x] 5.4 Add fake-host coverage for the previously-unfakeable routed calls (the probes that hit `exec`/`os` directly)
- [x] 5.5 `go test -race . ./internal/engine ./internal/app` (engine changes are concurrency-sensitive)

## 6. Remove Ruby/Puppet package-version facts

- [x] 6.1 Add failing tests proving fake Ruby/AIO sources do not emit `ruby`/`aio_agent_version`, and `--puppet puppetversion` resolves as missing
- [x] 6.2 Remove the core Ruby resolver (`CoreFactsWithRuby`, `resolveRubyInfo`, `rubyFacts`) and puppet-agent AIO resolver (`aio_agent_version`)
- [x] 6.3 Stop `Engine.Discover`/`--puppet` from appending `puppetversion`, while keeping Puppet plugin external-fact directories
- [x] 6.4 Update schema, changelog, help/man text, and specs for the intentional output-contract removal
- [x] 6.5 Targeted tests for core and app behavior pass

## 7. Verification

- [x] 7.1 `gofmt -w`, `go vet ./...`, `go test ./...`, `go test -race . ./internal/engine ./internal/app` all clean
- [x] 7.2 Existing contract/acceptance suites (`engine_contract_test.go`, `internal/app/contract_test.go`, `tests/acceptance`) green with only the intentional Ruby/Puppet package-version removals
- [x] 7.3 Side-by-side: CLI output (default, `--json`, `--yaml`, `--hocon`, a query, `--color`) byte-identical to the pre-change binary modulo volatile values and the intentional `ruby`/`aio_agent_version`/`puppetversion` removals
- [x] 7.4 Platform CI gates green on the final commit
