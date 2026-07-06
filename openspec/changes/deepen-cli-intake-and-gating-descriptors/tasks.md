## 1. Pre-refactor pins (tests first — all must pass against today's code)

- [x] 1.1 Pin list-task option inertness: `--list-cache-groups`/`--list-block-groups` with `--no-external-facts`, `--no-cache`, `--disable`, `--no-block`, `--debug` produce byte-identical output to the bare invocation (`internal/app/app_test.go`).
- [x] 1.2 Pin walker arity-skip behavior: list tasks with interleaved valued options (e.g. `--list-cache-groups -l debug -c PATH --external-dir DIR`, `=`-attached and short-alias spellings) honor exactly the parsed config path and external dirs (`internal/app/app_test.go`).
- [x] 1.3 Pin `--no-json`/`--no-yaml`/`--no-hocon` standalone acceptance and inertness on a query (`internal/app/app_test.go`).
- [x] 1.4 Pin the two-tier option-error rendering at the binary layer: validation error → `ERROR Facts::OptionsValidator - ` + exit 1; runtime interplay error (`--no-external-facts --external-dir DIR`, config log-level conflict) → plain error line without the prefix + exit 1 (`cmd/facts/main_test.go`).
- [x] 1.5 Pin `facts --color facterversion` and `facts --force-dot-resolution facterversion` stdout bytes (fast path renders uncolored, un-merged) (`internal/app/app_test.go`).
- [x] 1.6 Add the deferred `add-fact-disable-controls` task 1.6 regression test: a disabled fact with a live cached value is not served from cache, and a pruned sub-fact is not persisted into a cached group (`internal/engine/cache_gating_test.go` or alongside `core_gating_test.go`).

## 2. Gating descriptor table (internal/engine)

- [x] 2.1 Add the descriptor table: one row per core fact category — root name, Facter-compat group name, gating class (`standalone`/`multiOutput`/`sharedProbe`/`inlineEager`), assembly func, emitted roots, probe consumers, emits-under (`internal/engine/descriptors.go`).
- [x] 2.2 Add the descriptor-agreement test: gate names, group membership, and emitted roots asserted against the table; a drifted literal fails (`internal/engine/descriptors_test.go`).
- [x] 2.3 Rewrite `buildCoreFacts` to iterate the table (gated standalone categories; eager multi-output, shared-probe, inline, emits-under classes exactly as today), removing the nine inline `gate()` calls (`core.go`).
- [x] 2.4 Re-derive `BuiltinFactGroups()` as a projection of the table; `cache.go` TTL bucketing and `--list-*-groups` output unchanged (`groups.go`).
- [x] 2.5 Add the shared hierarchy helper and collapse the four walk implementations onto it: `FilterDisabledFacts` root check, `pruneDisabledDescendants`, the ambient descendant subsumption (`discovery_plan.go:111-118`), and `ambientDisableSource` ancestor walk (`engine.go:135-151`).
- [x] 2.6 Migrate `core_gating_test.go`'s hard-coded gated-category list to read from the table; all existing gating, group, pruning, and diagnostic tests pass unmodified.

## 3. Registry-driven option intake (internal/app + internal/cli)

- [x] 3.1 Add the FlagSet builder: iterate `cli` option metadata, bind every non-task option (canonical + aliases, arity/repeatability from metadata) into one `parsedOptions` struct (`internal/app`).
- [x] 3.2 Add the registry↔parser parity test: every non-task registry entry has a binding; the FlagSet accepts nothing outside the registry (`internal/app`).
- [x] 3.3 Replace `runQuery`'s 28 hand declarations with the builder; all existing `app_test.go`, `disable_test.go`, `contract_test.go`, and `main_test.go` cases pass unmodified (`app.go`).
- [x] 3.4 Route `factGroups` through the same parsed intake (reading only config path and external dirs) and delete `configPathFromArgs`/`externalDirsFromArgs`; tasks 1.1–1.2 pins stay green (`app.go`).

## 4. Engine-owned planning for the fast-path gate (internal/app + internal/engine)

- [x] 4.1 Export the pure external-dir planning seam from the engine (the same resolution `planDiscovery` applies: CLI dirs → config dirs → process defaults, honoring no-external-facts), following the `DisabledUnion` precedent (`discovery_plan.go`).
- [x] 4.2 Rewire `canUseVersionQueryFastPath` inputs to the seam's answer and delete the app-side mirror (`app.go:272-291`); config-parse-error ordering, disabled-facterversion fall-through, external-dir shadowing, and `--timing` bypass all covered by existing tests plus task 1.5 pins.
- [x] 4.3 Use the same seam for `factGroups`' external-dir resolution, deleting its private `effectiveExternalDirs` merge duplication (`app.go:81-86`).

## 5. Docs

- [x] 5.1 Update CHANGELOG (internal consolidation; no user-facing behavior change).
- [x] 5.2 Record the frozen list-task option semantics decision where contributor docs describe the CLI tasks (CONTRIBUTING or package docs), so a future change to unify semantics starts from a recorded decision.

## 6. Verification

- [x] 6.1 Run `go test ./...` and `go vet ./...`; confirm the pre-existing test files pass unmodified (only additions permitted).
- [x] 6.2 Run the CLI option-contract tests (help/man drift) and the acceptance suite.
- [ ] 6.3 Confirm `add-fact-disable-controls` is archived before this change archives; run `openspec validate deepen-cli-intake-and-gating-descriptors --strict`.
