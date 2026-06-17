## Why

Validating the Go port side-by-side against the real Facter 4.10.0 binary on Linux (Ubuntu 24.04 aarch64) showed 19 of 24 top-level facts byte-identical, but two facts diverge in ways that violate the existing "Linux fact parity" requirement:

- `mountpoints` size and capacity are wrong. The shared statfs helper multiplies block counts by `f_bsize` instead of `f_frsize`, so on filesystems where the two differ (virtiofs reports `f_bsize` 256× larger than `f_frsize`) sizes inflate massively — a mount Facter reports as `228 GiB` we report as `57.07 TiB`. Capacity is also computed as `used/size` from `f_bfree`, while `df`/Facter use `used/(used+available)` from `f_bavail`, so reserved-block filesystems read low (root fs `6.75%` vs Facter `7.26%`) and full read-only mounts read `0%` instead of `100%`.
- `networking` interfaces omit the interface-level `ip`/`ip6`/`netmask`/`netmask6`/`network`/`network6`/`scope6` keys that Facter populates. Every other POSIX platform flattens these from the first binding; the Linux path is the only one that skips that step.

Both are concrete parity regressions against a fact set the spec already requires to match Ruby.

## What Changes

- Fix Linux `mountpoints` block math to use `f_frsize` (not `f_bsize`) so `size`/`used`/`available` match `df`/Facter, including on virtiofs and other filesystems where `f_bsize != f_frsize`.
- Fix `mountpoints` capacity to the `df`/Facter formula `used/(used+available)` using `f_bavail`, so reserved-block and full read-only mounts report the same percentage Facter does.
- Populate Linux `networking.<iface>` with the flattened `ip`/`ip6`/`netmask`/`netmask6`/`network`/`network6`/`scope6` keys, matching the other POSIX platforms and Facter.
- Keep macOS/Darwin and FreeBSD `mountpoints` behavior unchanged (their `Statfs_t` has no `Frsize` field; `f_bsize` is already correct there).
- Out of scope (explicitly NOT changed): `processors.models` reporting `["aarch64"]` on ARM where Facter reports `[]` — this is a deliberate Go-port deviation (more accurate data), not a bug.

No new user-visible flags, fact names, formats, or dependencies. This narrows existing facts back toward Ruby parity.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-supported-platform-facts`: The "Core fact parity → Linux fact parity" requirement is sharpened so that Linux `mountpoints` size/used/available/capacity and Linux `networking.<iface>` interface-level binding fields are explicitly required to match Facter, closing the two observed divergences.

## Impact

- **Code**: `internal/engine/statfs_supported.go` (split a Linux-specific statfs that uses `Frsize`; keep darwin/freebsd on `Bsize`), capacity computation in `internal/engine/core.go` (`mountpointsFactWithSkip`/`filesystemCapacity`), and the Linux branch of `currentNetworkingData` in `internal/engine/core.go` (add the missing `expandInterfaceBindings` call). Focused Linux-targeted tests for both.
- **Behavior**: Linux `mountpoints` and `networking` output moves toward exact Facter parity. macOS/FreeBSD output unchanged.
- **Docs/schema**: No fact schema or changelog change expected; this restores documented parity rather than adding surface.
