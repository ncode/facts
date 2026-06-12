# Design: Rename the binary and identity to facts

## Context

ADR-0004 named the project Facts but kept the binary `facter` so existing scripts could shell out unchanged. Since then ADR-0006 (no Ruby DSL) and ADR-0007 (no legacy facts) deliberately narrowed Ruby compatibility; the byte-for-byte drop-in story is already amended, and the maintainer's standing direction is to lead with the project's own identity. The binary, help/man text, diagnostics prefixes (`WARN Facter -`, `ERROR Facter::OptionsValidator -`), artifact names (`facter-<version>-<os>-<arch>`), and `cmd/facter`/`internal/facter` paths still carry the old name. The operator input surface (`facter.conf`, `FACTER_*`, puppetlabs facts.d paths) is where real fleets' data lives.

## Goals / Non-Goals

**Goals:**
- The user-visible identity is `facts` everywhere: binary, build/dist/install, help, man page, diagnostics program token, README, gates.
- Facts-native input names (`facts.conf`, `FACTS_*`, `/etc/facts/facts.d` and friends) work as the primary surface; the facter-named surfaces keep working as documented compat reads with native-wins precedence.
- No facter-named Go packages of our own remain (`cmd/facter` → `cmd/facts`, `internal/facter` → `internal/engine`).
- ADR-0008 records the decision and supersedes ADR-0004.

**Non-Goals:**
- No `facter` alias or symlink (rejected per the no-zombie-surface pattern; revisit only on real demand).
- No fact renames: `facterversion` and every other fact name are output contract, untouched.
- No change to `--puppet` (it is named after Puppet, the external system) or to wording that refers to Ruby Facter as the system we interoperate with ("accepted for Facter compatibility", parity docs).
- No migration of existing cache contents (caches regenerate).

## Decisions

**1. Hard rename, no alias (supersedes ADR-0004).**
Same rationale as ADR-0006/0007: unreleased project, nobody to protect, and a `facter` symlink would misrepresent the narrowed compat promise. The README repositions from "the facter CLI your scripts shell out to" to "reads your existing facter configuration and fact directories".

**2. Native-first input discovery, compat read second.**
- Config: with no `--config`, try `/etc/facts/facts.conf` (Windows `C:/ProgramData/facts/facts.conf`), then the existing facter default path; first existing file wins. Semantics identical for both.
- Env facts: `FACTS_<name>` and `FACTER_<name>` both load as external environment facts; on collision the `FACTS_` value wins.
- Default external-fact dirs: prepend native paths (root: `/etc/facts/facts.d`; user: `~/.facts/facts.d`; Windows: `<ProgramData>/facts/facts.d`) to the existing facter/puppetlabs list; existing directory-precedence rules then apply unchanged.
- Cache: default path's facter-named segment renamed to facts; no compat read.

**3. Diagnostics rebrand is a token swap only.**
Every stderr message keeps its Ruby-compatible structure and text with the program token swapped: `WARN Facts -`, `ERROR Facts::OptionsValidator -`, `Facts failed to read config file …`. The parity spec is amended to "Ruby-compatible apart from the program name token", so existing message-text tests update mechanically.

**4. `internal/facter` becomes `internal/engine`.**
The root public package is already `facts`; naming the internal package `facts` too would force aliasing everywhere it meets the root package. `engine` says what it is and matches how the root package already imports it (`engine "github.com/ncode/facts/internal/facter"`). Mechanical rename: directory, package clauses, imports; no API changes.

**5. The man page moves to `man/man8/facts.8`.**
Renamed and re-titled; content updated for the binary name and the native+compat input surface; the FILES section lists both config paths with precedence.

## Risks / Trade-offs

- [Hidden facter-name assumptions in tests/gates/workflows] → the rename is grep-driven (`facter` as a word, case-insensitive, across Makefile, workflows, tools/, tests/) with the explicit keep-list from Non-Goals; CI gates on all four platforms are the backstop.
- [Native config dir conflicts with an existing `/etc/facts` on some host] → name is project-owned; first-existing-wins keeps facter-configured hosts unchanged.
- [Env collision precedence surprises] → pinned by spec scenario and tests; documented in FACTER_CONF_COMPATIBILITY.md (which gains the native-name section).
- [Package rename churn obscures real changes in review] → isolate the mechanical `internal/facter` → `internal/engine` rename in its own commit within the PR.

## Migration Plan

Single PR, ordered: (1) ADR-0008 + deltas; (2) mechanical package/dir renames (`cmd/facts`, `internal/engine`); (3) identity rename (binary, Makefile, dist, gates, workflows, help/man, diagnostics token) with test re-pins; (4) native input surface + precedence tests; (5) docs sweep and full verification on all platform gates. Rollback: revert.

## Open Questions

None — alias, input-surface, and diagnostics scope were decided by the maintainer on 2026-06-12.
