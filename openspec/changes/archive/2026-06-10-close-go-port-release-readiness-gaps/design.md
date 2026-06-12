## Context

The 2026-06-10 deep review validated that built-in fact parity for the four release targets is essentially done: the full Go test suite passes, the fact inventory matches Ruby per platform, and earlier audit concerns about missing resolvers (virt-what, lspci, NetKVM, Docker-aware uptime) were verified as already implemented in `internal/facter/virtual.go` and `internal/facter/core.go`.

What remains is release engineering and honesty about compatibility boundaries:

- `internal/facter/custom.go` is a static simulator of the Ruby DSL (46 `regexp.MustCompile` patterns). It never evaluates Ruby. A fact whose `setcode` block contains anything beyond a recognized literal/command pattern is silently dropped or resolves to nil. There is no detection path that tells the user *why* a fact is missing.
- `tools/parity-ledger/main.go` extracts `(spec path, nearby go test command)` pairs from `docs/MIGRATION.md` and trusts them. It does not check that named test functions exist, that the command is more specific than `./...`, or that every `*_spec.rb` file in the repo received an explicit in-scope/out-of-scope classification.
- CI (`.github/workflows/`) tests Linux and macOS thoroughly and runs Windows unit tests, but the Windows release gate result is not a blocking criterion and FreeBSD has no automated coverage at all. The cross-compile job also builds out-of-scope platforms.
- Nothing exists for shipping: no `dist`/`install`/packaging targets, `bin/facter` is a development shim, the gemspec and installer still describe Ruby facter, Beaker acceptance tests are Ruby-only, and the man page is unverified against the Go CLI.
- `--puppet` runs `puppet --version` only (`internal/facter/puppet.go`), and `facter.conf` is parsed by regex (`internal/facter/config.go`).

## Goals / Non-Goals

**Goals:**

- Make missing custom facts diagnosable: every unsupported DSL construct encountered at load time produces a warning naming the file, the construct, and the documented alternative.
- Publish the DSL compatibility contract and a fact-author migration guide so operators can audit their fact repos before switching.
- Make the parity ledger's "covered" disposition machine-verified, and make every spec file's scoping decision explicit.
- Give all four release targets blocking automated validation.
- Produce installable, versioned release artifacts and a documented cutover path from the Ruby entry points.
- Close or explicitly bound the `--puppet` and HOCON gaps.

**Non-Goals:**

- Do not delete or modify Ruby sources under `lib/`, `spec/`, or `acceptance/` beyond marking dispositions; Ruby cleanup remains a separate approved change.
- Do not embed a Ruby interpreter by default (see Decision 1 — this is the fallback path, not the plan of record).
- Do not add release targets beyond Linux, macOS/Darwin, Windows, FreeBSD.
- Do not chase one-to-one Ruby spec/test mapping; the ledger fix is about verifiability of references, not granularity quotas.

## Decisions

1. **Custom-fact DSL: diagnose-and-document, not a Ruby runtime.**

   Embedding or shelling out to Ruby would reintroduce the dependency the port exists to remove, and conditional "use Ruby if present" execution would make fact results environment-dependent. Instead the regex engine gains a detection layer: when a `Facter.add`/`define_fact` block is found whose `setcode`/`confine` body matches no supported pattern, the loader emits a structured warning (`unsupported custom fact construct in <file>: <reason>`) and records the fact name as unresolvable-by-design. The contract document enumerates supported constructs (literals, `%w`, hashes/arrays, `Execution.exec/execute` with literal commands, `Facter.value`, `ENV[]`, basic confines, simple block confines, weights, aggregates) and maps each unsupported pattern to its migration path — usually an executable external fact. If field feedback later shows this is insufficient, a separate change can propose an embedded interpreter; this change deliberately keeps that out.

2. **Unsupported DSL areas get explicit dispositions, not silent absence.**

   - `on_flush`: detected and warned; not implemented (session-scoped cache makes flush hooks a no-op in the Go engine). Documented deviation.
   - Ruby `$LOAD_PATH`/gem fact discovery: not implemented; documented deviation with `FACTERLIB`/`--custom-dir` as the supported mechanisms.
   - `.rb` files in external-fact directories: detected, warned, skipped (matches the "external facts are data or executables" model).
   - Complex confine blocks: detected and warned; the resolution is treated as unsuitable (conservative: never matching is safer than wrongly matching).

3. **Ledger verification is structural, not semantic.**

   The ledger tool will: (a) parse every `-run '^(...)$'` pattern in a coverage reference and require each named test prefix to match at least one `func Test...` in the repo, (b) classify references whose command is package-blanket (`./...` with no `-run`) as `blanket-coverage` — a disposition that fails `parity-ledger-check` until replaced with a focused reference or an explicit waiver note in `docs/MIGRATION.md`, and (c) emit an `unclassified` row for any `*_spec.rb` file that matches neither an in-scope domain nor an explicit out-of-scope rule, so the 614-vs-895 delta is fully accounted for. Semantic adequacy (does the test really pin the Ruby behavior?) stays a human audit concern, as in the prior change.

4. **FreeBSD CI uses a VM action; Windows gate becomes pass/fail.**

   GitHub Actions has no native FreeBSD runner. The workflow will use a hosted-VM action (e.g. `vmactions/freebsd-vm`) on the Linux runner to boot FreeBSD, build, run `go test ./internal/facter ./internal/app`, and execute the release-gate fact-set smoke (the same set as `make lima-freebsd-smoke`). If the VM action proves flaky, fallback is a scheduled Cirrus CI job with the GitHub workflow asserting its latest status. Windows: `tools/windows-release-gate.ps1` already runs in `unit_tests.yaml`; the change makes its exit code fail the job and documents it as a release gate in `PORTING.md`.

5. **Distribution starts with `make dist` artifacts, not OS packages.**

   The first deliverable is reproducible versioned archives (`facter-<version>-<os>-<arch>.tar.gz`/`.zip`) for linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/{amd64,arm64}, freebsd/amd64, built by `make dist` and attached by a release workflow, plus `make install` (PREFIX-aware). Apt/yum/Homebrew/MSI packaging and code signing are follow-on work tracked as explicit open items, not blockers for this change. `bin/facter` cutover: the shim learns to prefer an installed Go binary and warn when falling back to `go run`; full replacement happens with Ruby cleanup.

6. **Acceptance testing is a Go smoke suite, not a Beaker port.**

   Beaker's host-orchestration model serves the Ruby agent stack. The Go port needs end-to-end confidence per platform, which the existing CI matrix mostly provides. The change adds a small `cmd/facter` acceptance test package (build the real binary, run it with real flags against the real host, assert on the release-gate fact set and output formats) reused across the four platform CI jobs. The Ruby `acceptance/` tree is marked historical in docs.

7. **HOCON: adopt a parser library behind the existing seam.**

   `internal/facter/config.go` keeps its public functions; the implementation switches to a maintained Go HOCON library, with the existing regex behavior retained as documented fallback only if the library's behavior diverges from Ruby's `Hocon.load` on the repo's fixture corpus. New tests pin include-free real-world configs plus the edge cases the regex parser mishandles today.

8. **`--puppet` is bounded, not deepened.**

   Loading Puppet's Ruby plugin facts requires a Ruby runtime (Decision 1 forecloses that). The Go `--puppet` will: resolve `puppetversion` via the puppet binary (as today), additionally search Puppet's default `pluginfactdest` paths for *external* facts, and emit a clear warning that Ruby plugin custom facts are not loaded. Documented deviation in the DSL contract and man page.

## Risks / Trade-offs

- **Diagnostic regexes can misclassify.** The unsupported-construct detector may warn on facts the engine actually resolves, or miss exotic syntax. Mitigation: detector runs only when no supported pattern matched, and the warning text is advisory; tests cover the documented contract table.
- **VM-action FreeBSD CI can be slow/flaky.** Mitigation: cache Go modules, keep the in-VM test scope to the two platform-sensitive packages plus the smoke run; Cirrus fallback documented.
- **`blanket-coverage` disposition will initially fail `parity-ledger-check`.** That is intended — it converts hidden weakness into visible work. The tasks sequence ledger hardening before gate enforcement so the failure window is within this change.
- **HOCON library swap may change parse results for odd configs.** Mitigation: fixture corpus comparison before swap; regex fallback retained until corpus parity is shown.

## Migration Plan

1. Land ledger hardening and reclassify (makes remaining work visible and honest).
2. Land DSL diagnostics + contract docs (no behavior change to supported facts).
3. Land CI gates (Windows blocking, FreeBSD VM job, cross-compile scope cleanup).
4. Land distribution targets and release workflow; verify artifacts on each platform gate.
5. Land framework fidelity items (HOCON, `--puppet` bounds, man page).
6. Update `docs/MIGRATION.md`, regenerate the ledger, and close the prior change's open tasks 9.6/10.2/10.3/10.5 where this change satisfies them.

## Open Questions

- Which Go HOCON library best matches Ruby `Hocon.load` semantics (`gurkankaymak/hocon` is the leading candidate; needs a fixture-corpus bake-off).
- Whether release artifacts must be code-signed/notarized (macOS Gatekeeper, Windows SmartScreen) before first binary release or whether checksummed archives suffice initially.
- Where the fact-author migration guide lives long-term (`docs/` here vs. puppet.com docs pipeline).
