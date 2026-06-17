# History: the Ruby-to-Go port

Facts began as a Go port of
[Puppet Facter](https://github.com/puppetlabs/facter). The port — proposed,
implemented, verified, and then deliberately departed from — took place
between 2026-06-07 and 2026-06-12. This file is the surviving summary of
that project (see
[About this repository's history](#about-this-repositorys-history)).

## Method

The port was test-driven against the Ruby implementation, which stayed in the
tree until completion:

- **TDD slices.** One Ruby behavior at a time: read the Ruby spec, write one
  failing Go test through the public API or CLI, write the minimal Go code to
  pass, verify, and record the slice in an append-only migration log
  (`docs/MIGRATION.md`, ~9,500 lines by the end — some 685 dated slice
  entries between 2026-06-07 and 2026-06-09).
- **Parity-ledger discipline.** A generator (`tools/parity-ledger`) bucketed
  every `*_spec.rb` file under the Ruby `spec/` and `spec_integration/` trees
  as in-scope, out-of-scope (with an explicit exclusion rule), or
  unclassified, and machine-verified that every in-scope entry's coverage
  reference named real `func Test...` declarations. Unclassified files and
  unwaived blanket coverage failed the check.
- **Platform scope.** Four release targets: Linux, macOS/Darwin, Windows, and
  FreeBSD. Solaris, AIX, OpenBSD, NetBSD, DragonFly, and generic BSD paths
  were declared out of scope because no repeatable validation target existed
  for them.
- **Four blocking platform gates.** Every release target got a blocking CI
  gate (Linux and macOS native runners, Windows Server 2022/2025 runners with
  `tools/windows-release-gate.ps1`, and a FreeBSD VM job running
  `tools/freebsd-release-gate.sh`); a gate failure fails the pipeline. The
  current release-target matrix has since expanded — see
  [CONTRIBUTING.md](../CONTRIBUTING.md).

## Final parity-ledger accounting

The frozen ledger (`docs/PARITY_LEDGER.md`, generated 2026-06-10, never
regenerated) closed with:

| Bucket | Count |
| --- | --- |
| Total Ruby spec files | 895 |
| In-scope | 614 |
| Out-of-scope (explicit exclusion rule) | 281 |
| Unclassified | 0 |
| Covered by verified Go test reference | 612 |
| Blanket coverage waived with documented reason | 2 |
| Failed references | 0 |

The two waivers were Ruby implementation machinery with no Go analog
(`win32ole_spec.rb`, WIN32OLE COM internals; `class_discoverer_spec.rb`,
constant-reflection class discovery), each pinned instead by the Go tests
covering the equivalent behavior.

## Release-readiness milestones

- **2026-06-07 — port begins.** TDD slices against the Ruby specs, logged
  daily in the migration log through 2026-06-09.
- **2026-06-10 — release readiness.** The `close-go-port-release-readiness-gaps`
  change closed the gaps: hardened parity ledger (the accounting above),
  `make dist`/`make install` and the release workflow, `--puppet` bounds, and
  the facter.conf parser decision — a HOCON-library bake-off found that
  neither maintained Go HOCON library reproduced Ruby `Hocon.load` on the
  documented facter.conf surface, so Facts keeps its pinned-subset parser
  (see [FACTER_CONF_COMPATIBILITY.md](FACTER_CONF_COMPATIBILITY.md)).
- **2026-06-10 — all four platform gates green**: the
  first green Windows and FreeBSD gate runs, after CI iterations that caught
  real product bugs (missing `ol`/`amzn` family mapping, Server 2025 WMI
  fallback, Windows command quoting, CRLF fixture corruption, and more).
- **2026-06-10 — port declared complete; Ruby removed.** The
  `remove-ruby-implementation` change deleted `lib/`, `spec/`,
  `spec_integration/`, `acceptance/`, `ext/`, `facter.gemspec`, `install.rb`,
  `Gemfile`, `Rakefile`, and the Ruby lint configs, with all gates green on
  the removal commit. The parity ledger was frozen as the final verification
  record. Packaging dispositions: the gemspec, `install.rb`, and the `ext/`
  vanagon/ezbake metadata were frozen as historical for the Ruby gem line and
  deleted with the removal — Go distribution is `make dist` archives and
  `make install`; the Beaker `acceptance/` suite was succeeded by the Go
  binary-level `tests/acceptance` package. `bin/facter` briefly lived on as a
  cutover shim preferring an installed Go binary; ADR-0008 later deleted it
  outright.
- **2026-06-11/12 — post-port departures** (below), concluding with the
  rename to `facts` on 2026-06-12.

## Post-port departures

With the port complete and the project still unreleased, Facts deliberately
narrowed and reshaped the Ruby-compatibility story. Each step is an archived
OpenSpec change with an ADR where it changed a contract:

- **Idiomatic Go API** (`introduce-facts-library-api`, 2026-06-11, ADR-0001):
  the ~58 Ruby-shaped package-level exports and all package-global state were
  replaced by the immutable `facts.Engine` / `Snapshot` surface. Ruby
  compatibility is promised only at the CLI process boundary.
- **No Ruby DSL** (`remove-ruby-custom-fact-dsl`, 2026-06-11, ADR-0006): the
  ~4,900-line regex DSL parser was removed; no `.rb` fact file is read
  anywhere. External facts are the whole input contract (see
  [CUSTOM_FACT_MIGRATION.md](CUSTOM_FACT_MIGRATION.md)).
- **No legacy facts** (`remove-legacy-facts`, 2026-06-11, ADR-0007): the ~150
  flat Ruby-era aliases (`operatingsystem`, `hostname`, `processorcount`, …)
  no longer resolve; the structured tree is the only fact surface.
- **Not-applicable facts omitted** (`omit-not-applicable-facts`, 2026-06-11):
  facts whose preconditions don't hold are absent instead of empty
  placeholders, converging default output on Ruby Facter's fact set.
- **Default-format parity and depth color**
  (`default-format-parity-and-depth-color`, 2026-06-12): the default
  `key => value` text format became byte-compatible with Ruby Facter 4.10.0's
  `LegacyFactFormatter`, plus a Facts-only depth-colorized terminal mode.
- **Rename to facts** (`rename-binary-to-facts`, 2026-06-12, ADR-0008): the
  binary, man page, dist artifacts, and diagnostics token all became `facts`;
  no `facter` alias ships. Facter-named inputs remain as the compat tier
  under the facts-native ones.
- **OpenBSD and NetBSD support** (`add-openbsd-netbsd-support`, 2026-06-17):
  the supported release-target matrix expanded from the original four
  port-completion targets to include OpenBSD and NetBSD, with VM release
  gates and schema-backed supported-fact documentation.

The migration log's append-only discipline ended with the log itself: future
behavior changes are recorded in [CHANGELOG.md](../CHANGELOG.md) entries and
OpenSpec change records under `openspec/`.

## Architecture decisions

The full decisions live in [docs/adr/](adr/); one line each:

- **ADR-0001** — One idiomatic Go API; the CLI process boundary is the only
  Ruby-compatibility boundary (partially superseded by ADR-0006).
- **ADR-0002** — The canonical dynamic tree is the only fact model; typing is
  a view (`facts.As[T]`), never an independent resolver.
- **ADR-0003** — Library engines are hermetic by default; reading host
  config, fact directories, and caches is explicit opt-in.
- **ADR-0004** — The project is "facts", the binary stays "facter"
  (superseded by ADR-0008).
- **ADR-0005** — Immutable `Engine`, explicit `Discover` → immutable
  `Snapshot`; no memoizing fact state in the engine.
- **ADR-0006** — No Ruby custom-fact DSL; external facts are the whole input
  contract.
- **ADR-0007** — No legacy facts; the structured tree is the only fact
  surface.
- **ADR-0008** — The binary and identity are `facts`; no facter alias;
  facts-native input names first, facter-named reads as the compat tier.

## About this repository's history

This repository started as a fork of `puppetlabs/facter` carrying its full
upstream git history (2,340 commits by 84 contributors, dating to 2019). On
2026-06-12 the history was deliberately flattened to a single initial
commit: the inherited record overwhelmingly described another product and
misrepresented this project's activity and authorship. The detailed
porting-era records (the port tracker, the 9,471-line migration log, the
frozen parity ledger, the Ruby internals reference, and the inherited Ruby
Facter 4.0.x release notes) were summarized into this file before their
removal and are not recoverable from this repository. The Ruby
implementation and its complete history remain available in upstream
[puppetlabs/facter](https://github.com/puppetlabs/facter); attribution is
carried by [NOTICE](../NOTICE), which does not depend on git history.

What lives in HEAD today: [CONTEXT.md](../CONTEXT.md) (the project language),
[docs/adr/](adr/) (the decisions), [CONTRIBUTING.md](../CONTRIBUTING.md) (how
to work on Facts, including the still-live TDD rules and release gates that
originated in the port tracker),
[FACTER_CONF_COMPATIBILITY.md](FACTER_CONF_COMPATIBILITY.md) and
[CUSTOM_FACT_MIGRATION.md](CUSTOM_FACT_MIGRATION.md) (the operator-facing
compatibility contracts), and [man/man8/facts.8](../man/man8/facts.8) (the
CLI manual).
