# Rename the binary and identity to facts

## Why

The project is named Facts; the shipped binary, help text, man page, diagnostics, and artifact names still say `facter`. ADR-0004 kept the `facter` name for drop-in CLI compatibility, but the project has since deliberately narrowed Ruby compatibility (ADR-0006 removed the DSL, ADR-0007 removed legacy facts) — the identity should match the product. This change supersedes ADR-0004: the binary is `facts`, with no `facter` alias.

## What Changes

- **BREAKING** The binary, build targets, and release artifacts are renamed: `cmd/facter` → `cmd/facts`, `make build` produces `./facts`, `make dist` produces `facts-<version>-<os>-<arch>`, `make install` installs `facts`. No `facter` compatibility symlink is shipped.
- **BREAKING** Diagnostics rebrand the program token: `WARN Facter -` → `WARN Facts -`, `ERROR Facter::OptionsValidator -` → `ERROR Facts::OptionsValidator -`, config-reader warnings, and the stderr log handler. Message text stays Ruby-compatible apart from the program name token.
- Facts-native input names become primary, with the facter-named surfaces still read for compatibility:
  - Config: `/etc/facts/facts.conf` (Windows `C:/ProgramData/facts/facts.conf`) is consulted first; the existing `facter.conf` default paths remain as compat fallback. First existing file wins.
  - Environment facts: `FACTS_<name>` joins `FACTER_<name>`; on a name collision the facts-native variable wins.
  - Default external-fact directories: facts-native locations (`/etc/facts/facts.d`, `~/.facts/facts.d`, Windows `ProgramData/facts/facts.d`) searched ahead of the existing facter/puppetlabs paths.
- The persistent cache moves to the facts-native path (the facter-named segment renamed); no compat read — caches regenerate.
- Help text, man page (`man/man8/facts.8`), README, release gates, CI workflows, and acceptance tests follow the new names. Internal `internal/facter` package is renamed (`internal/facter` → `internal/engine`) so the tree carries no facter-named code of our own; references to Ruby Facter as the external system we interoperate with keep the Facter name.
- Fact-name parity is untouched: fact names (`facterversion`, `os.*`, …), output formats, `facter.conf` semantics, `FACTER_*` env reading, and `--puppet` behavior all keep working.
- ADR-0008 records the decision and supersedes ADR-0004.

## Capabilities

### New Capabilities

- `facts-native-input-surface`: facts-native config/env/directory names as the primary operator input surface, with facter-named compatibility reads and defined precedence.

### Modified Capabilities

- `go-port-framework-parity`: the CLI is named `facts`; diagnostics message-text parity is amended to "Ruby-compatible apart from the program name token".
- `go-port-distribution-and-cutover`: artifact names, install target, acceptance-suite binary path, and man page move to `facts`.
- `go-port-ci-platform-gates`: gates build `cmd/facts`.
- `facts-library-api`: scenario references to "the `facter` CLI" become "the `facts` CLI".

## Impact

- **Code**: `cmd/facter` → `cmd/facts`; `internal/facter` → `internal/engine` (package clause + imports, mechanical); diagnostics prefixes in `internal/app` and `cmd`; config/env/dir discovery in the engine (native names + precedence); cache default path.
- **Build/CI**: Makefile (build, dist, Lima smokes), release workflow, `tools/windows-release-gate.ps1`, `tools/freebsd-release-gate.sh`, integration workflow, acceptance build path.
- **Docs**: README, man page rename and content, help text, `FACTER_CONF_COMPATIBILITY.md` (native names + compat), CONTEXT.md identity, PORTING.md, CHANGELOG breaking entry, ADR-0008.
- **Behavioral**: invoking `facter` no longer works (no alias); stderr prefixes change; hosts with existing facter config/facts keep working unchanged via the compat reads.
