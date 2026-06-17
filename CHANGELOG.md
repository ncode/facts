# Facts (Go port)

## Unreleased

### Added

- OpenBSD and NetBSD are being promoted to supported targets with core fact
  resolvers, schema metadata, local smoke targets, release-gate scripts, and
  GitHub VM/cross-compile coverage.
- Conditional ZFS and zpool facts are now emitted on FreeBSD and NetBSD when
  usable `zfs upgrade -v`/`zpool upgrade -v` output is available.
- Added generated per-platform supported fact pages under
  `docs/supported-facts/`, derived from `docs/schema/facts.yaml` and checked
  by `go test ./...`.

### Changed

- **BREAKING**: Linux disk serial numbers now emit as
  `disks.*.serial_number` instead of `disks.*.serial`, matching the
  schema-owned canonical spelling used for disk serials across supported
  targets (ADR-0011).
- Engine diagnostics now flow through a single seam — the per-Engine
  `log/slog` logger. The package-global diagnostic handlers
  (`SetWarningHandler`/`SetDebugHandler`/`SetErrorHandler`) and the
  process-global `warn`/`debug`/`reportError` are removed. A library Engine
  built with `WithLogger` now also receives the diagnostics previously routed
  to the global sink — config-file parsing, the persistent cache, fact-group
  TTL parsing, OS-hierarchy detection, and canonical-tree collection
  collisions (the last reported once at discovery, at error severity). The
  `facts` CLI stderr output is unchanged: the same `WARN Facts -`/`DEBUG
  Facts -` lines, and error-class engine diagnostics remain dropped. This
  last point is Facter-aligned, not a gap: when `--force-dot-resolution`
  expands dotted typed facts and two collide, the colliding fact is silently
  dropped with no diagnostic — matching Facter 4.10.0, validated in-VM
  (silent drop, empty stderr at every log level, exit 0).

## v0.0.2 - 2026-06-16

### Changed

- The frozen porting records (the port tracker, the append-only migration
  log, the parity ledger, and the Ruby internals reference) are consolidated
  into `docs/HISTORY.md` and removed; the repository history was
  subsequently flattened to a single initial commit (see `docs/HISTORY.md` —
  the Ruby history remains in upstream puppetlabs/facter).
  `CONTRIBUTING.md` is rewritten for Facts
  (build/test/bench commands, TDD rules, platform scope, release gates, the
  OpenSpec workflow, and the Ruby Facter comparison technique), and the Ruby
  Facter 4.0.x release notes this file inherited from upstream are removed.
- **BREAKING**: Renamed the shipped binary and user-visible identity to
  `facts` (ADR-0008, superseding ADR-0004). `make build` produces `./facts`,
  `make install` installs `facts`, `make dist` produces
  `facts-<version>-<os>-<arch>` archives, and the man page moves to
  `man/man8/facts.8`; no `facter` alias or symlink is shipped (the
  `bin/facter` cutover shim is removed), so invoking `facter` no longer
  works. Stderr diagnostics rebrand the program name token only — `WARN
  Facts -`, `ERROR Facts::OptionsValidator -`, `Facts failed to read config
  file …` — message structure and text are otherwise unchanged. Facts-native
  input names become the primary operator surface, with the facter-named
  reads kept as the compatibility tier (native wins when both are present):
  `/etc/facts/facts.conf` (Windows `C:/ProgramData/facts/facts.conf`) is
  consulted before the facter.conf default paths with the first existing
  file winning; `FACTS_<name>` environment facts load alongside
  `FACTER_<name>` and win name collisions; and the facts-native
  external-fact directories (`/etc/facts/facts.d`, `~/.facts/facts.d`,
  `<ProgramData>/facts/facts.d`) are searched ahead of the existing
  facter/puppetlabs ones. The default cache path's facter-named segment is
  renamed (`/opt/puppetlabs/facts/cache/cached_facts`); caches regenerate
  with no compat read. Fact names (`facterversion`, `os.*`, …), output
  formats, and `facter.conf` semantics are unchanged; hosts with
  existing facter configuration keep working through the compat reads. See
  `docs/FACTER_CONF_COMPATIBILITY.md`.
- **BREAKING**: Removed legacy alias facts entirely (ADR-0007). The canonical
  structured tree (`os.name`, `networking.hostname`, `processors.count`, …) is
  the only fact surface: the ~150 Ruby-era flat aliases (`operatingsystem`,
  `hostname`, `fqdn`, `processorcount`, `memorysize`, `ssh*key`, `sshfp_*`,
  `mtu_*`, `sp_*`, `lsb*`, …) no longer resolve in any output mode, explicit
  query, or library Snapshot — `facter operatingsystem` prints nothing and
  exits 0 (a missing-fact error under `--strict`). The `--show-legacy` and
  `--no-show-legacy` flags are gone and fail as unknown options, the
  `show-legacy` `facter.conf` key is inert like any other unrecognized key,
  and a `legacy` blocklist entry loads without error and blocks nothing. The
  default `key => value` text format (Ruby's "legacy format") is unrelated
  and unchanged. The alias-to-structured migration table is in
  `docs/adr/0007-no-legacy-facts-structured-tree-only.md`.
- **BREAKING**: Removed the Ruby custom-fact DSL layer (ADR-0006). No `.rb`
  fact file is read from any source: the `--custom-dir`, `--no-ruby`,
  `--no-custom-facts`, and `--trace` CLI flags are gone and now fail as
  unknown options (`--trace` only ever controlled Ruby custom-fact exception
  backtraces); `FACTERLIB` is ignored; the `custom-dir`, `no-ruby`,
  `no-custom-facts`, and `cli.trace` `facter.conf` keys are inert like any
  other unrecognized key; and `facts.WithCustomDirs` is removed from the Go
  API. A `.rb` file in an
  external-fact directory is still skipped with a warning. External facts
  (data files, executables, `FACTER_*` environment variables) and
  programmatic `facts.WithFact` registration are the input contract; see
  `docs/CUSTOM_FACT_MIGRATION.md` for the pattern mapping.
- **BREAKING**: Removed Ruby runtime and Puppet package-version built-in facts.
  Core discovery no longer emits `ruby` or `aio_agent_version`, and
  `puppetversion` is no longer resolved by executing Puppet.
- **BREAKING**: Removed the `--puppet`/`-p`/`--no-puppet` CLI flags (ADR-0009).
  Facts implements Facter's input/output contract, not Puppet's runtime
  behavior: it no longer auto-discovers Puppet's agent plugin-fact destination
  (`vardir/facts.d`) or warns about pluginsync'd Ruby plugin facts. The flags
  now fail as unknown options. Facter's own external-fact directories (the
  `…/puppetlabs/facter/facts.d` defaults and the rest of the default set) are
  unchanged; to load Puppet's agent-synced module facts, pass `--external-dir
  /opt/puppetlabs/puppet/cache/facts.d` (platform/user equivalent). The inert
  `EngineConfig.Puppet` field is also removed.
- **BREAKING (Go API only)**: Removed the Ruby-compatible Go API — the ~58
  package-level exports (`Value`, `ToHash`, `Resolve`, `Add`, `Flush`,
  `Search`, message and option toggles, …) and all package-global mutable
  state. Ruby compatibility is promised only at the CLI process
  boundary; CLI stdout/stderr behavior and operator fact sources (external
  facts, `facter.conf`) are unchanged.
- **BREAKING (Go API only)**: Renamed the root package `facter` → `facts` to
  match the module path `github.com/ncode/facts`. The shipped binary kept
  the name `facter` at the time (ADR-0004); it is now `facts` per the
  ADR-0008 entry above.
- Facts that cannot resolve a value or do not apply to the platform are now
  omitted instead of emitted as empty placeholders, converging each
  platform's default output on Ruby Facter's fact set: `augeas` is absent
  when no augparse binary exists (no more `{version: ""}`), `disks` and
  `partitions` are absent when no devices enumerate (e.g. macOS),
  `processors.speed` is absent when the speed is unknown (e.g. Apple
  Silicon), `fips_enabled` resolves only on Linux and Windows, `os.selinux`
  resolves only on Linux, and `dmi` and `filesystems` are absent when their
  probes resolve nothing. `processors.extensions` is kept as
  accurate additional data and documented as a deliberate deviation in the man
  page COMPATIBILITY section. Operator-supplied external facts with empty
  values are unaffected — the change is in the core resolvers, not the
  formatter.

### Added

- A published, gate-enforced fact schema: `docs/schema/facts.yaml` documents
  every supported fact as a dotted path (with `*` patterns for dynamic key
  segments) carrying its type, description, platforms, and a `conditional`
  marker for facts whose presence depends on host state. A conformance test
  (`TestFactsSchemaConformance`) rides all platform CI gates and fails
  on undocumented facts and on non-conditional entries the platform does not
  emit; its `-schema-report` mode lists exactly what a new fact still needs
  documented. `CONTRIBUTING.md` gains the matching "Adding facts" rule and
  the FreeBSD gate now runs the root-package tests so the schema is verified
  there too.
- The embeddable library API: immutable, hermetic-by-default `facts.Engine`
  built via `facts.New` with functional options (`WithConfigFile`,
  `WithExternalDirs`, `WithCache`, `WithFact`, `WithLogger`,
  `WithSystemDefaults`); explicit `Discover(ctx, queries...)` returning an
  immutable `Snapshot` with dotted `Value` queries (`ErrFactNotFound` vs
  resolved-nil), `Tree`, `All` iteration, and generic `facts.As[T]` decode;
  partial-results-with-joined-errors discovery; `log/slog` diagnostics with
  per-engine once-only dedup (discarded by default).
- `context.Context` now bounds all fact resolution: command execution,
  external-fact scripts, and EC2/GCE/Azure metadata HTTP requests honor
  cancellation and deadlines.
- Upgraded the module to Go 1.26 (`go 1.26.0` directive; CI follows via
  `go-version-file`). Applied the `go fix` modernizers and the new stdlib
  idioms: `sync.WaitGroup.Go` replaces the `Add`/`Done` dance and
  `errors.AsType` replaces `errors.As` target-variable extraction. The
  Green Tea garbage collector is now on by default.
- Fact keys in the default text format are colorized by nesting depth,
  cycling a fixed ANSI palette (cyan, yellow, green, magenta, blue) per
  level; values, braces, and punctuation stay uncolored. This is a Facts
  extension — Ruby Facter's `--color` only affects diagnostics. Color is on
  by default when writing to a terminal and off otherwise, so piped and
  redirected output carries no escape sequences; `--color` forces it on,
  `--no-color` turns it off, and `--json`/`--yaml`/`--hocon` output is
  always byte-identical with and without color.

### Fixed

- The default `key => value` text format is now byte-compatible with Ruby
  Facter 4.10.0's `LegacyFactFormatter` in all three modes (full output,
  single query, multiple queries): nested strings are double-quoted, map
  entries and array elements carry separating commas, arrays render one
  element per line, multi-query nil values render empty with top-level
  strings unquoted, and single-query structures keep their enclosing braces
  with nested quoting intact. Full output expands embedded `\n` sequences to
  real newlines exactly like Ruby; single-query output does not. The
  previous hand-rolled tree walk (no commas, unquoted nested strings,
  single-line arrays, and a divergent query-mode path) is replaced by one
  pipeline that replicates Ruby's pretty-printed-JSON transforms, quirks
  included.
- `memory.swap` is omitted entirely on hosts without swap (zero total) on
  every platform, matching Ruby Facter; previously macOS and FreeBSD hosts
  without swap reported a zeroed swap subtree (and `memory.swap.encrypted`
  could appear alone on Linux).

- `networking.hostname` now reports the short host name (the node name up to
  the first dot) instead of the full node name, matching Ruby Facter:
  `networking.domain` is the remainder (falling back to the resolver
  search/domain configuration when the node name is undotted) and
  `networking.fqdn` stays `hostname.domain`. On a macOS host named
  `dream-factory.lan` this fixes `networking.hostname` from
  `dream-factory.lan` to `dream-factory`.
- `networking.interfaces` now includes interfaces that carry no addresses,
  reporting their MTU like Ruby Facter's getifaddrs-driven map. On macOS this
  surfaces the default `gif0` and `stf0` tunnel interfaces that were
  previously missing.
- Primary IPv6 selection for `networking.ip6`, `networking.network6`, and
  `networking.scope6` is now pinned as a deliberate, documented deviation
  from Ruby Facter: routable addresses on the primary interface win over
  link-locals — global scope first, then unique-local, then link-local —
  where Ruby reports the first-bound address (on macOS often the `fe80::`
  link-local). A primary interface carrying only link-local IPv6 still
  reports it with `scope6` `link`, matching Ruby. Documented in the man page
  COMPATIBILITY section.
