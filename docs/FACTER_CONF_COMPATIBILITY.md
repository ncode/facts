# Operator Input Compatibility (facts.conf / facter.conf)

This file is the reference for the operator input surface: the facts-native
input names, the facter-named compatibility reads, the precedence between
them, and the pinned configuration-file subset both names share.

## Facts-native names and facter-compatible reads (ADR-0008)

Facts accepts its operator input surface under facts-native names while
continuing to read the facter-named equivalents for compatibility. Native
names win when both are present; semantics are identical for both tiers.

- **Config file**: with no `--config`, Facts consults the facts-native path
  first and the facter-compatible path second; the first *existing* file
  wins and the other is ignored. `--config` overrides both.

  | Tier | Unix-like targets | Windows |
  | --- | --- | --- |
  | native | `/etc/facts/facts.conf` | `C:/ProgramData/facts/facts.conf` |
  | compat | `/etc/puppetlabs/facter/facter.conf` | `C:/ProgramData/PuppetLabs/facter/etc/facter.conf` |

- **Environment facts**: `FACTS_<name>` variables load as external facts
  alongside `FACTER_<name>` variables, with identical name rules. On a name
  collision the `FACTS_` value wins regardless of environment order; a name
  set through only one prefix resolves from that prefix.

- **Default external-fact directories**: the facts-native locations are
  searched ahead of the facter-compatible locations; facts found in both
  follow the normal directory-precedence rules.

  | Scope | Native (searched first) | Compat (searched after) |
  | --- | --- | --- |
  | root | `/etc/facts/facts.d` | `/etc/puppetlabs/facter/facts.d`, `/etc/facter/facts.d/`, `/opt/puppetlabs/facter/facts.d` |
  | non-root | `~/.facts/facts.d` | `~/.facter/facts.d`, `~/.puppetlabs/opt/facter/facts.d` |
  | Windows | `<ProgramData>/facts/facts.d` | `<ProgramData>/PuppetLabs/facter/facts.d` |

- **Cache**: the default persistent-cache path is the facts-named location
  (`/opt/puppetlabs/facts/cache/cached_facts`; on Windows
  `<ProgramData>/PuppetLabs/facts/cache/cached_facts`). There is no compat
  read of the old facter-named path — caches regenerate.

## The pinned facter.conf parsing subset

Ruby Facter parses `facter.conf` with the Ruby `hocon` gem (`Hocon.load`).
The Go port parses the documented facter.conf surface with a pinned-subset
parser instead of a general HOCON library — and parses the facts-native
`facts.conf` with exactly the same parser and semantics. This file defines
that subset; the boundary is pinned by tests in
`internal/engine/config_test.go`.

## Why not a HOCON library

A 2026-06 bake-off (summarized in `docs/HISTORY.md`) ran the two maintained
Go HOCON candidates against a fixture corpus drawn from the documented
facter.conf surface:

- `gurkankaymak/hocon` v1.2.23 mangles unquoted strings containing `/`
  (`/first/external` parses to `/"""first"""/"""external`), mangles bare
  dotted keys (`os.name`), and converts durations like `30 days` into Go
  duration syntax, losing the Ruby TTL unit strings.
- `go-akka/configuration` (unmaintained since 2020) parses bare paths but
  returns unordered objects (Ruby hashes preserve document order, which
  `fact-groups` relies on), cannot expose `ttls`' array-of-objects through
  its typed API, and reports parse errors by panic.

Neither reproduces Ruby `Hocon.load` on the corpus, so the Go port keeps its
own parser for the supported subset below.

## Supported configuration surface

Everything Ruby Facter documents for facter.conf:

- Sections: `global`, `cli`, `facts`, `fact-groups` (top level, brace
  blocks).
- `global`: `external-dir` (string or array; quoted or bare paths),
  `no-external-facts`, `force-dot-resolution`, `sequential`.
  The retired keys (`custom-dir`, `no-custom-facts`, `no-ruby`, `cli.trace`,
  and `show-legacy`) are ignored like any other unrecognized key (ADR-0006
  for the custom-fact keys; ADR-0007 for `show-legacy`).
- `cli`: `debug`, `verbose`, `log-level`.
- `facts`: `blocklist` (fact and group names, quoted or bare, dotted names
  allowed), `ttls` (array of single-entry objects; values are Ruby TTL
  strings such as `30 days`, `1 hour`, `30 minutes`, or bare numbers).
  A `legacy` blocklist entry is inert: with legacy facts removed (ADR-0007)
  there is nothing for it to block — the config loads without error or
  warning and discovery output is unchanged.
- `fact-groups`: custom group name to fact array (or single scalar fact);
  quoted group names may contain spaces.
- Key/value separators `:` and `=`; trailing commas; `#` and `//` comments
  (including inline); repeated keys across sections are collected in
  document order (e.g. `external-dir` in both `global` and `cli`).
- CLI flags always override config-file options.

Leniencies beyond Ruby (accepted by the Go parser, kept for
backward-compatibility with the port's earlier releases):

- Single-quoted strings are treated like double-quoted strings (Ruby's HOCON
  would keep the quotes as literal characters).
- A config whose sections are malformed degrades per-key instead of failing
  the whole file.

## Unsupported HOCON features

These general-HOCON features are not part of the documented facter.conf
surface and are not supported; the boundary tests pin the behavior:

- `include` directives: not processed (no file inclusion happens; an
  include-only file is reported as invalid config).
- Substitutions (`${path}`): never expanded.
- Value concatenation outside TTL strings, `+=` appends, and unicode escapes.

Invalid or unreadable config files produce a warning and are ignored (Facter
runs with defaults), matching Ruby Facter's diagnostic-not-fatal handling.
