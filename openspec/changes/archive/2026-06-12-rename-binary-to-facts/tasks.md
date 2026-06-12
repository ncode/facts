# Tasks: Rename the binary and identity to facts

## 1. Decision record and mechanical renames

- [x] 1.1 Write ADR-0008 (binary and identity are `facts`, no alias; supersedes ADR-0004; records the native+compat input surface and the diagnostics token swap)
- [x] 1.2 Rename `cmd/facter` → `cmd/facts` (main package, tests, references)
- [x] 1.3 Rename `internal/facter` → `internal/engine` (directory, package clauses, all imports; no API changes)

## 2. Identity rename

- [x] 2.1 Makefile: `make build` → `./facts`, dist archives `facts-<version>-<os>-<arch>`, install target, Lima smoke targets and instance prefixes
- [x] 2.2 Release/CI: release workflow artifact names, `tools/windows-release-gate.ps1`, `tools/freebsd-release-gate.sh`, integration workflow, acceptance suite build path and binary name
- [x] 2.3 Diagnostics token swap: `WARN/ERROR/INFO/DEBUG Facter -` → `Facts -`, `ERROR Facter::OptionsValidator -` → `ERROR Facts::OptionsValidator -`, config-reader warnings; re-pin affected tests
- [x] 2.4 Help text and man page: usage says `facts`, `man/man8/facter.8` → `man/man8/facts.8` with updated name/synopsis/FILES; keep Ruby-Facter-as-external-system wording intact

## 3. Facts-native input surface

- [x] 3.1 Config discovery: native `facts.conf` paths first, facter compat path second, first existing wins; unchanged parse semantics; tests for native-only, compat-only, both-present
- [x] 3.2 Environment facts: `FACTS_*` loaded alongside `FACTER_*`, native wins collisions; tests
- [x] 3.3 Default external-fact directories: native paths prepended (root, user, Windows); tests
- [x] 3.4 Cache default path renamed to the facts-native location; tests updated
- [x] 3.5 Document the native names, compat reads, and precedence in `docs/FACTER_CONF_COMPATIBILITY.md` (and retitle/reframe that doc as the input-compatibility reference)

## 4. Docs and verification

- [x] 4.1 README identity sweep (binary name, positioning, examples); CONTEXT.md; PORTING.md; CHANGELOG breaking entry
- [x] 4.2 `go test ./...`, `go test -race ./...`, vet/gofmt clean; grep sweep shows no facter-named surfaces of our own (keep-list: Ruby-Facter references, fact names, compat input names)
- [x] 4.3 Smoke: `./facts os.name` works; `FACTS_x`/`FACTER_x` precedence; native config wins; stderr says `Facts`
- [x] 4.4 Platform CI gates green on the final commit
