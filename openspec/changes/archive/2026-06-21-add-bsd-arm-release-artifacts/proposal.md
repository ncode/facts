## Why

Go supports `arm` and `arm64` for FreeBSD, OpenBSD, and NetBSD, but Facts only
published amd64 artifacts for those BSD release targets.

## What Changes

- Add `arm` and `arm64` artifacts for FreeBSD, OpenBSD, and NetBSD to
  `make dist`.
- Add the same tuples to cross-compile CI and Lima cross-compile defaults.
- Update release docs, specs, and changelog.

## Impact

- Release artifact matrix expands; runtime fact behavior is unchanged.
- Native BSD release gates remain per-OS gates; the new ARM/ARM64 tuples are
  compile-gated.
