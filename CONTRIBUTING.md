# Contributing to Facts

Facts discovers and reports facts about the system it runs on — an embeddable
Go library and the `facts` CLI. This page is how to work on it.

## Development setup

Go 1.26+ is the only requirement. The make targets wrap the standard tools:

```sh
make build         # go build -o facts ./cmd/facts
make test          # go test ./...
make race          # go test -race ./...
make bench-stable  # repeated benchmarks: -benchtime 5s -count 5 -benchmem
```

`gofmt` and `go vet ./...` must come back clean; CI checks both. Full
`make bench-stable` across the module is slow — scope it while developing:

```sh
make bench-stable BENCH=BenchmarkFormatJSON PACKAGES=./internal/engine
```

## Test-driven development

Every behavior starts with a failing Go test, then the minimal code to pass:

- Exercise the public surface: the `facts` package API or `app.Run`. Library
  behaviors belong in the root `facts` package tests, CLI-contract behaviors
  (stdout/stderr/exit codes) in `internal/app` tests.
- Prefer integration-style tests over unit tests of internals.
- Platform-dependent behavior is tested through fixtures and seam injection
  (command output fixtures, fake filesystems, injected probes), so every
  platform's logic runs in a plain `go test` on any host.

## Adding facts

Every supported fact is documented in
[docs/schema/facts.yaml](docs/schema/facts.yaml) — dotted path (with `*`
patterns for dynamic key segments), value type, description, platforms, and a
`conditional` marker for facts whose presence depends on host state. New or
renamed facts MUST get a schema entry: `TestFactsSchemaConformance` runs on
all platform gates and fails on any emitted fact the schema does not
describe, and on any non-conditional entry the platform does not emit.

To see exactly what still needs documenting, run the report mode:

```sh
go test -run TestFactsSchemaConformance . -args -schema-report
```

It prints the undocumented paths grouped by top-level fact instead of
failing. Mark an entry `conditional: true` when the fact can legitimately be
absent on a host of that platform (cloud metadata, swap, DMI, installed
tools).

## Platform scope

Release targets: **Linux, macOS/Darwin, Windows, FreeBSD, OpenBSD, and
NetBSD**. Solaris, AIX, DragonFly, and generic BSD-family paths are out of
scope — do not start work for them until a repeatable validation target
exists, and do not treat their gaps as blockers.

## Benchmark discipline

Hot paths — fact resolution, query/formatting, cache, config parsing,
external-fact loading, and the networking/memory/processor/disk parsers —
carry benchmarks. When you touch one:

- Run repeated focused benchmarks before and after
  (`go test -run '^$' -bench <name> -count 5 -benchmem`, or the
  `make bench-stable` form above).
- Record representative results in the change record (PR description or
  CHANGELOG entry); unjustified regressions don't merge.
- Cold diagnostic paths may skip this if the change record says why.

## Release gates

Every release target has a blocking, automated CI gate; a gate failure fails
the pipeline (`.github/workflows/unit_tests.yaml`):

- **Linux** (x64, arm64): tests, build, and CLI smoke on native runners, plus
  the container distro matrix and in-scope cross-compiles in
  `integration_tests.yaml`.
- **macOS/Darwin** (arm64, x64): tests, build, and smoke on native runners.
- **Windows** (Server 2022, 2025): tests plus
  `tools/windows-release-gate.ps1`, which verifies the Windows release-gate
  fact set through the built binary; non-zero exit fails the job.
- **FreeBSD**: a hosted VM job runs the platform-sensitive packages and
  `tools/freebsd-release-gate.sh` — the same fact-set definition as the local
  `make lima-freebsd-smoke` target.
- **OpenBSD** and **NetBSD**: hosted VM jobs run the platform-sensitive
  packages and `tools/openbsd-release-gate.sh` /
  `tools/netbsd-release-gate.sh`. Local QEMU guests under `.local/bsd-vms`
  can run the same checks through `make local-bsd-smoke`.
- **DragonFly BSD** and **illumos**: native gates use
  `tools/dragonfly-release-gate.sh` and `tools/illumos-release-gate.sh`
  through local, untracked SSH wrappers. These validate `dragonfly/amd64` and
  `illumos/amd64`; Oracle Solaris is not covered by the illumos gate.

Local equivalents: `make lima-freebsd-smoke`, `make lima-linux-flavor-smoke`,
`make local-bsd-smoke`, `make local-amd64-bsd-smoke`,
`make local-candidate-smoke`, and running `tools/windows-release-gate.ps1` on
a Windows host.

The amd64 lab smoke targets are configured only through wrapper variables:
`LOCAL_FREEBSD_AMD64_SSH`, `LOCAL_OPENBSD_ARM64_SSH`,
`LOCAL_OPENBSD_AMD64_SSH`, `LOCAL_NETBSD_ARM64_SSH`,
`LOCAL_NETBSD_AMD64_SSH`, `LOCAL_DRAGONFLY_AMD64_SSH`, and
`LOCAL_ILLUMOS_AMD64_SSH`. OpenBSD, NetBSD, and DragonFly wrappers must allow
`sudo -n` because their release gates read privileged disk labels. Real
`.local` wrapper scripts stay untracked.

## The change workflow

Behavior changes go through OpenSpec. A change lives under
`openspec/changes/<change-id>/` with a proposal, design, tasks, and spec
deltas against the capabilities in `openspec/specs/`:

- `/opsx:propose` — create a change and generate its artifacts.
- `/opsx:apply` — implement the tasks.
- `/opsx:archive` — after implementation, archive the change and sync its
  deltas into `openspec/specs/`.

Record user-visible behavior changes in `CHANGELOG.md` under Unreleased.

## Parity questions

Ruby Facter compatibility is promised at the CLI process boundary (output
contract) and for operator-supplied fact sources (input contract). When a
behavior question comes down to "what does Ruby Facter do?":

- Install the gem (`gem install facter`) and compare `facter --json` with
  `./facts --json` side by side on the same host.
- Match Ruby unless there is a recorded reason not to. A deliberate deviation
  must be documented — an ADR under `docs/adr/` for contract-level decisions,
  or a COMPATIBILITY entry in `man/man8/facts.8` for fact-level ones — and
  pinned by a test.

The porting-era verification record (parity ledger, migration log) is
summarized in [docs/HISTORY.md](docs/HISTORY.md), which also shows how to
recover the full frozen records from git history.

## Where decisions live

- [CONTEXT.md](CONTEXT.md) — the project's language: use these terms in code,
  tests, and docs.
- [docs/adr/](docs/adr/) — architectural decisions and their rationale; don't
  re-litigate them in a PR, supersede them with a new ADR.
- [docs/HISTORY.md](docs/HISTORY.md) — how the port happened and where its
  records are.
