# Omit not-applicable and empty facts from output

## Why

The 2026-06-11 host comparison against Ruby Facter 4.10.0 showed the Go port emitting facts Ruby correctly omits: `augeas => { version => "" }` when augparse is absent, empty `disks => {}` and `partitions => {}` maps on macOS, an empty `processors.speed` string, and the Linux/Windows-only `fips_enabled` and `os.selinux` facts on macOS. Empty and not-applicable facts make output look broken and contradict the project's own principle (facts-library-api: not-applicable facts are absent, never errors or placeholders).

## What Changes

- Facts that cannot resolve a value are omitted entirely, never emitted with empty strings or empty maps: `augeas` when no augparse, `disks`/`partitions` when no devices enumerate, `processors.speed` when unknown.
- Platform-inapplicable facts are gated to the platforms where Ruby Facter resolves them: `fips_enabled` (Linux, Windows), `os.selinux` and the related SELinux data (Linux only).
- `processors.extensions` (emitted on ARM macs where Ruby has no such key) is kept as accurate additional data and documented as a deliberate deviation — additional truth is "the future", empty placeholders are not.
- No formatter changes: the fix is upstream (resolvers stop producing empty facts), so formatting of legitimately empty *values inside* resolved facts is untouched.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-supported-platform-facts`: adds a requirement that unresolvable and platform-inapplicable facts are omitted from the canonical tree, and that accurate additional data beyond Ruby's set is allowed only as a documented deviation.

## Impact

- **Code**: resolvers in `internal/facter/` for augeas, disks/partitions, processor speed, FIPS, and SELinux gain omit-when-empty / platform gates; no engine or CLI surface changes.
- **Tests**: per-fact tests asserting omission on the not-applicable platform/state; existing structured-tree tests unchanged.
- **Docs**: the `processors.extensions` deviation noted in the man page GO PORT NOTES; CHANGELOG entry (non-breaking — output only loses placeholder noise).
- **Behavioral**: macOS default output drops `augeas`, `disks`, `partitions`, `fips_enabled`, `os.selinux`, and the empty `processors.speed` key, converging on Ruby's fact set.
- **Dependencies**: independent of, but complementary to, `remove-legacy-facts`; both shrink default output toward Ruby's 22-fact baseline.
