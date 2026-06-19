## Why

OpenBSD, NetBSD, and FreeBSD already emit partial structured storage and networking facts, but the schema still leaves several native, validated fields unsupported. Extending only the facts with clear platform sources closes useful gaps without reopening the full platform-port scope.

## What Changes

- Add NetBSD mountpoint capacity/byte facts from its `df -P` output, matching the existing OpenBSD path.
- Add BSD networking enrichments where native probes expose them: interface DHCP for FreeBSD, operational state for FreeBSD/OpenBSD/NetBSD, and speed/duplex for FreeBSD.
- Add FreeBSD partition type metadata from GEOM XML.
- Add conditional FreeBSD disk type when GEOM reports a known rotation rate.
- Resolve the OpenBSD/NetBSD product-version DMI disposition and update emitted/schema facts if the value should be exposed as `dmi.product.version`.
- Keep unsupported or unstable sources absent, especially NetBSD DHCP unless a stable text source is selected.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `go-port-supported-platform-facts`: expands the supported BSD fact contract for validated storage, mountpoint, networking, and DMI fields.
- `facts-schema`: updates platform metadata for every newly emitted BSD fact and keeps unsupported fields absent or conditional.

## Impact

- Core resolvers in `internal/engine`, primarily `disks.go`, `networking.go`, `dmi.go`, and the NetBSD statfs/df path.
- Fixture-backed parser tests and live BSD smoke checks for FreeBSD, OpenBSD, and NetBSD.
- `docs/schema/facts.yaml`, generated `docs/supported-facts/` pages, and `CHANGELOG.md`.
