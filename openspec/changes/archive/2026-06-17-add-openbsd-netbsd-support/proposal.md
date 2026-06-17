## Why

Facts currently treats OpenBSD and NetBSD as out of scope even though Puppet Facter supports them and local test guests are now available. Promoting both BSDs to supported targets closes that parity gap and prevents future platform behavior from staying partially implemented and untested.

## What Changes

- Add OpenBSD and NetBSD to the supported release-target set for core fact parity, schema conformance, distribution, and verification.
- Complete OpenBSD fact coverage where it is currently partial, including memory, processors, release-gate validation, schema coverage, and CI.
- Add NetBSD fact coverage for the same supported structured fact categories, using fixture-backed resolvers plus live VM smoke validation.
- Add audited OpenBSD/NetBSD disk and partition facts, and conditional ZFS/zpool facts when the platform provides usable `zfs`/`zpool` upgrade output.
- Add live OpenBSD and NetBSD GitHub pipeline gates that build the real `facts` CLI, run platform-sensitive tests, and verify a shared release-gate fact set.
- Extend cross-compile and release artifact matrices with OpenBSD and NetBSD targets.
- Keep the existing compatibility boundaries: no Ruby custom fact DSL support, no legacy alias facts, no Puppet package-version facts, and platform-inapplicable facts remain absent.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `go-port-supported-platform-facts`: expands supported release-target fact parity from Linux, macOS/Darwin, Windows, and FreeBSD to include OpenBSD and NetBSD.
- `go-port-ci-platform-gates`: adds blocking OpenBSD and NetBSD CI validation and updates cross-compile scope.
- `go-port-distribution-and-cutover`: adds OpenBSD and NetBSD release artifacts and acceptance coverage to the supported target matrix.
- `go-port-completion-verification`: adds OpenBSD and NetBSD to the release-completion verification matrix.
- `facts-schema`: requires OpenBSD and NetBSD platform entries and schema conformance in their platform gates.

## Impact

- Core fact resolvers in `internal/engine`, primarily OS, networking, memory, processors, DMI, disks/partitions/mountpoints, ZFS/zpool, virtualization, SSH, uptime, and schema-facing assembly.
- Fixture and public-surface tests for platform-specific parser behavior and not-applicable fact omission.
- `docs/schema/facts.yaml`, `CHANGELOG.md`, and contributor/release documentation.
- `Makefile`, `tools/*release-gate*`, `.github/workflows/*`, and release artifact generation.
