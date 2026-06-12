# The binary and identity are "facts"; no facter alias

The project's user-visible identity becomes `facts` everywhere: the shipped binary (`cmd/facts`, `make build` → `./facts`, `make install` installs `facts`), release artifacts (`facts-<version>-<os>-<arch>`), help text, the man page (`man/man8/facts.8`), and the diagnostics program token (`WARN Facts -`, `ERROR Facts::OptionsValidator -`, `Facts failed to read config file …` — a token swap only; every stderr message keeps its Ruby-compatible structure and text otherwise). No `facter` alias or symlink is shipped. This supersedes ADR-0004, which kept the binary named `facter` for drop-in script compatibility: since then ADR-0006 (no Ruby DSL) and ADR-0007 (no legacy facts) deliberately narrowed Ruby compatibility, so a binary that still answers to the upstream product's name misrepresents what it ships. The tree carries no facter-named Go packages of our own either — `internal/facter` becomes `internal/engine`; references to Ruby Facter as the external system we interoperate with keep the Facter name.

The operator input surface goes native-first with the facter-named reads kept as the compat tier, native wins when both are present:

- **Config**: with no `--config`, `/etc/facts/facts.conf` (Windows `C:/ProgramData/facts/facts.conf`) is consulted first, then the existing facter default path; the first existing file wins, with identical parse semantics for both.
- **Environment facts**: `FACTS_<name>` loads alongside `FACTER_<name>`; on a name collision the `FACTS_` value wins.
- **Default external-fact directories**: facts-native locations (root: `/etc/facts/facts.d`; user: `~/.facts/facts.d`; Windows: `<ProgramData>/facts/facts.d`) are searched ahead of the existing facter/puppetlabs list; directory-precedence rules are otherwise unchanged.
- **Cache**: the default path's facter-named segment is renamed to facts; no compat read — caches regenerate.

Fact-name parity is untouched: `facterversion` and every other fact name are output contract. `--puppet` keeps its name (it is named after Puppet, the external system), `facter.conf` semantics and the parity wording about Ruby Facter ("accepted for Facter compatibility", the parity ledger, the migration guide) stay as they are — those name the system we interoperate with, not us.

## Considered Options

- **Keep the `facter` binary name (ADR-0004 status quo)** — rejected: the identity no longer matches the product. ADR-0004's rationale was drop-in compatibility for scripts that shell out to `facter`, but the compat promise has since been narrowed twice (ADR-0006, ADR-0007) and the project is unreleased with nobody to protect; leading with the upstream name advertises a byte-for-byte story we deliberately no longer tell.
- **Ship a `facter` symlink/alias next to `facts`** — rejected per the ADR-0006/0007 no-zombie-surface precedent: an alias is a surface that misrepresents the narrowed compat promise, doubles every packaging and PATH-collision question (including against a real Ruby Facter on the same host), and exists only for users we cannot demonstrate exist. Revisit only on real demand.
- **Hard rename (chosen)** — breaking only on paper; the project is unreleased, so the rename is free now and expensive after the first release. Hosts with existing facter configuration keep working unchanged through the compat input reads, and the diagnostics change is confined to the program name token.
