## 1. Tests First

- [ ] 1.1 Test all facts resolve by default with an empty disabled set, `packages` included.
- [ ] 1.2 Test the disabled set is the union of `--disable`, `FACTS_DISABLE`, and config `disable`/`blocklist`, accepting fact names and group names, with `disable` winning over the `blocklist` alias.
- [ ] 1.3 Test resolution-gating: a standalone-resolver fact (`packages`) skips its resolver; a multi-output category runs and prunes when only some of its outputs are disabled; a disabled sub-fact is pruned.
- [ ] 1.4 Test `--no-block` clears the set; disable-beats-query returns empty; an env/config-sourced disable on an explicit query emits the stderr diagnostic with empty stdout.
- [ ] 1.5 Test `FACTS_DISABLE`, `FACTSDISABLE`, `FACTER_DISABLE`, and `FACTERDISABLE` are all reserved (no `disable` fact created) and feed the disabled set.
- [ ] 1.6 Test a disabled fact is never served from cache and a pruned sub-fact is not persisted into a cached group.

## 2. Implementation

- [ ] 2.1 Parse the `disable` config key (native) alongside `blocklist` (compat) into one disabled set; native wins on collision (`config.go`).
- [ ] 2.2 Add the `--disable` option (valued, comma-split, repeatable) to the shared option vocabulary, feed the disabled set, and make `--no-block` clear it (`cli/options.go`, `app.go`).
- [ ] 2.3 Reserve the resolved env name `disable` in the env-fact loader (covering `facts_`/`facts`/`facter_`/`facter`), routing the variable into the disabled set instead of creating a fact (`external.go`).
- [ ] 2.4 Union the three sources into the disabled set (`discovery_plan.go`).
- [ ] 2.5 Add a fact/group → resolver mapping and pass the disabled set into `buildCoreFacts`; skip a resolver only when all its top-level outputs are disabled; gate per shared probe (`core.go`).
- [ ] 2.6 Emit the stderr diagnostic for an explicitly-queried fact disabled by env/config; keep disabled-set subtraction before cache resolution and `Select` (`engine.go`).
- [ ] 2.7 Re-audit `BuiltinFactGroups` to drop legacy flat names removed by ADR-0007 from group membership.

## 3. Docs

- [ ] 3.1 Document `--disable`, `FACTS_DISABLE`, and the `disable` config key (with `blocklist` compat) in help, man, and the configuration-compatibility document.
- [ ] 3.2 Update README and CHANGELOG.

## 4. Verification

- [ ] 4.1 Run `go test ./...` and `go vet ./...`.
- [ ] 4.2 Run the CLI option-contract tests (help/man drift on `--disable`).
- [ ] 4.3 Run `openspec validate add-fact-disable-controls --strict`.
