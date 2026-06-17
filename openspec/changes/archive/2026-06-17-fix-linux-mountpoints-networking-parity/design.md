## Context

The Go port's `mountpoints` and `networking` facts were validated against the real Facter 4.10.0 binary on Ubuntu 24.04 aarch64 (Lima VM `facts-dev`). Two divergences were root-caused to specific code:

- `internal/engine/statfs_supported.go` (`statMountpoint`, build-tagged `linux || darwin || freebsd`) multiplies block counts by `stat.Bsize` and derives capacity as `used/size` from `Bfree`.
- `internal/engine/core.go` `currentNetworkingData` calls `expandInterfaceBindings(interfaces)` for darwin, freebsd, openbsd, and windows, but **not** in the `linux` branch. `expandInterfaceBindings` (core.go:1922) flattens `bindings[0]`/`bindings6[0]` into interface-level `ip`/`netmask`/`network` and `ip6`/`netmask6`/`network6`/`scope6`. The per-binding data is already built by `interfaceBinding` (core.go:2285); only the flatten step is missing.

Key platform constraint (verified with `go doc syscall.Statfs_t` on both OSes): Linux `Statfs_t` exposes both `Bsize` and `Frsize`; darwin `Statfs_t` exposes only `Bsize` (no `Frsize` field at all). So a `Frsize`-based fix cannot live in the shared build-tagged file — it would fail to compile on darwin/freebsd.

## Goals / Non-Goals

**Goals:**
- Linux `mountpoints` `size_bytes`/`used_bytes`/`available_bytes`/`capacity` match `df`/Facter, including filesystems where `f_bsize != f_frsize` (virtiofs) and full read-only mounts.
- Linux `networking.<iface>` emits the same interface-level `ip`/`ip6`/`netmask`/`netmask6`/`network`/`network6`/`scope6` keys as Facter and as the other POSIX platforms.
- macOS/Darwin and FreeBSD `mountpoints` output is byte-for-byte unchanged.

**Non-Goals:**
- `processors.models` on ARM (ours `["aarch64"]` vs Facter `[]`) — deliberate Go-port deviation, left as-is.
- The Go-vs-getopt CLI flag-ordering quirk and case-insensitive queries — separate, pre-existing, not in this change.
- Any new fact, flag, format, or dependency.

## Decisions

**1. Split a Linux-only statfs file; keep darwin/freebsd on the shared `Bsize` path.**
Move the Linux implementation of `statMountpoint` into a `//go:build linux` file using `Frsize` for the block multiplier. Restrict the existing `statfs_supported.go` to `//go:build darwin || freebsd` keeping `Bsize`. Rationale: `Frsize` does not exist on the darwin `Statfs_t`, so a single shared function cannot reference it. macOS's `f_bsize` is already the fundamental fragment size, so darwin/freebsd are correct unchanged. Alternative considered: reflection or a per-OS `blockSize()` helper in one file — rejected because the field is absent from the darwin struct, so even an unreached reference fails to compile.

**2. Capacity = `used/(used+available)` using `Bavail`, applied to all platforms.**
The capacity formula change is platform-agnostic (`Bavail` exists everywhere) and is the `df`/Facter definition. Change `mountpointsFactWithSkip` (or `filesystemCapacity`) so capacity is computed from `used_bytes` and `available_bytes` rather than `used/size`. Because darwin/freebsd statfs already produce correct `available_bytes` (from `Bavail`) and `used_bytes`, this aligns them with `df` too; verify their fixtures/output stay matching Facter `df`-style percentages. Note `used_bytes` itself stays `(Blocks - Bfree) * frsize` — that already matches `df`'s "Used" column; only the percentage denominator changes.

**3. Add `expandInterfaceBindings(interfaces)` to the Linux branch.**
One-line parity fix mirroring the four other platforms. The data is already present in each binding, so no binding-builder change is needed.

## Risks / Trade-offs

- **[Capacity formula change touches darwin/freebsd too]** → The denominator change applies on all platforms. Mitigation: it is the `df` definition Facter also uses, so it should *improve* darwin/freebsd parity; gate with the existing darwin/freebsd mountpoints fixtures and the FreeBSD Lima smoke path, and add a fixture asserting a reserved-block filesystem reports `used/(used+available)`.
- **[Frsize on exotic Linux filesystems]** → A filesystem reporting `f_frsize == 0` would zero the sizes. Mitigation: fall back to `Bsize` when `Frsize == 0` (matches `df`/coreutils, which treat frsize 0 as "use bsize").
- **[Build-tag split regression]** → Splitting the file risks a missing-symbol or duplicate-build-tag error on some target. Mitigation: `go vet ./...` plus `go build` for linux, darwin, and freebsd GOOS in CI; the existing platform matrix covers this.
- **[Re-validation is host-specific]** → The 256× virtiofs case only reproduces on a virtiofs mount. Mitigation: the deterministic fix is unit-tested with synthetic `mountStat` inputs (frsize != bsize, bavail=0); the Lima virtiofs mount is the integration check.
