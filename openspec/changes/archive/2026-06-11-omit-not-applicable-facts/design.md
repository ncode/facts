# Design: Omit not-applicable and empty facts

## Context

Five resolvers in `internal/facter/` emit placeholder values where Ruby Facter omits the fact: `augeas` resolves `{version: ""}` when augparse is missing; `disks` and `partitions` resolve empty maps on platforms with no enumerable devices (macOS); `processors.speed` resolves an empty string when the speed probe fails (Apple Silicon); `fips_enabled` and `os.selinux` resolve `false` on platforms whose kernels have no such concept. Ruby's resolvable-vs-not model treats all of these as "did not resolve" → absent. The Go engine already supports absence (not-applicable facts are absent per facts-library-api); the resolvers just never exercise it for these facts.

## Goals / Non-Goals

**Goals:**
- The six findings from the 2026-06-11 comparison disappear: no empty `augeas`/`disks`/`partitions`, no empty `processors.speed`, no `fips_enabled`/`os.selinux` on macOS/FreeBSD.
- A general spec-level rule exists so new resolvers follow omit-when-unresolvable instead of emitting placeholders.

**Non-Goals:**
- No formatter changes: a legitimately resolved fact whose *content* contains an empty value still renders it (external facts may define empty strings deliberately).
- No change to nil-valued registered facts (resolved-nil stays queryable as `(nil, nil)` per facts-library-api).
- No removal of `processors.extensions` — accurate extra data stays, documented.

## Decisions

**1. Fix at the resolver, not the formatter or engine.**
Each resolver returns no fact (or omits the key) when its probe fails: this matches Ruby's resolvable model, keeps the engine generic, and avoids a fragile output-side "strip empties" pass that would also strip operator-supplied empty external-fact values.

**2. Platform gates follow Ruby's actual emission, not the kernel's theoretical capability.**
`fips_enabled`: Linux and Windows only. `os.selinux`: Linux only. The authority is what Ruby Facter 4.10.0 emits per platform (verified in the parity captures), not what could be probed.

**3. `processors.extensions` stays.**
It is accurate hardware data. Removing truth to match Ruby would be parity for its own sake; the deviation is documented in the man page Go-port notes instead.

**4. Empty-map vs absent for `disks`/`partitions`.**
On Linux/FreeBSD where devices exist these stay populated; when enumeration yields zero entries the fact is absent (matching Ruby, which never emits `disks`/`partitions` on macOS at all).

## Risks / Trade-offs

- [A consumer relied on `fips_enabled => false` on macOS] → Unreleased project; CHANGELOG notes the convergence to Ruby's per-platform set; querying it under `--strict` now fails loudly.
- [Omitting `processors.speed` hides a probe bug on platforms where speed should resolve] → Platform tests pin speed presence where Ruby resolves it (Intel macs, Linux) and absence where it does not (Apple Silicon).
- [Some other resolver still emits placeholders not caught by the host comparison] → Add the spec rule plus a sweep test asserting no empty-string/empty-map top-level facts in a default discovery on each gate platform.

## Migration Plan

Single PR: resolver gates + per-fact omission tests, sweep test, man page note, CHANGELOG. Verify with `go test ./...` on all platform gates and a rerun of the Ruby comparison on macOS.

Rollback: revert the PR.

## Open Questions

None.
