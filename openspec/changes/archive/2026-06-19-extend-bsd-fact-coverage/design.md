## Context

OpenBSD and NetBSD are already supported release targets, and the current schema documents the facts they can emit. A focused audit found several fields where the operating systems expose stable native data but Facts either omits the field or only supports it on Linux/FreeBSD.

The validated sources are small and category-local: `df -P`/`mount` for NetBSD mountpoint byte values, BSD `ifconfig` and lease files/tools for network metadata, FreeBSD GEOM XML for partition and disk metadata, and BSD DMI sysctls for product version disposition.

## Goals / Non-Goals

**Goals:**

- Extend the BSD fact contract only where native sources were validated.
- Keep unsupported or unstable sources absent rather than adding guessed values.
- Preserve the existing category-owned resolver layout and fixture-backed test style.
- Update schema and generated supported-fact docs with every emitted field.

**Non-Goals:**

- No new supported platforms.
- No legacy alias facts.
- No Ruby custom fact DSL support.
- No broad "generic BSD" abstraction.
- No NetBSD DHCP unless the implementation selects a stable text source.

## Decisions

### Reuse Existing Category Resolvers

Storage work stays in `disks.go`, network work stays in `networking.go`, and DMI work stays in `dmi.go`. New parsing helpers should take `goos` or raw command output so tests can run on the host platform, matching ADR-0010.

Alternative considered: split new BSD code into `_openbsd.go`/`_netbsd.go` files. Rejected because these are parser/command-output paths, not syscall-only code, and GOOS-suffixed files would hide cross-platform tests.

### Prefer Text Probes Over New Dependencies

Use existing system commands and files:

- NetBSD mount bytes from `df -P` output, like OpenBSD already does.
- FreeBSD DHCP from `/var/db/dhclient.leases.<iface>` when present.
- OpenBSD DHCP remains `dhcpleasectl`.
- BSD interface status from `ifconfig`.
- FreeBSD speed/duplex from `ifconfig -m`.
- FreeBSD partition type from GEOM XML `config/type` or `rawtype`.

Alternative considered: add platform-specific libraries or cgo. Rejected; command fixtures already cover this repo's platform logic and keep cross-compilation simple.

### Make Ambiguous DMI Explicit

OpenBSD `hw.version` and NetBSD `machdep.dmi.system-version` are system product versions in the live guests, while current code maps them to `dmi.bios.version`. Implementation must choose and test one of:

- duplicate the value into `dmi.product.version`, preserving current `dmi.bios.version`; or
- move the value to `dmi.product.version` and update schema/tests for the behavior change.

The proposal prefers adding `dmi.product.version` only if this is documented as an intentional compatibility disposition.

## Risks / Trade-offs

- NetBSD DHCP source instability -> keep it absent unless the selected source is text, stable, and fixture-backed.
- BSD `ifconfig` formatting drift -> parse only small, named fields and cover FreeBSD/OpenBSD/NetBSD fixtures.
- Overclaiming schema support -> mark host-dependent fields conditional and run schema conformance in BSD smoke gates.
- FreeBSD disk rotation is often unknown -> emit `disks.*.type` only when the native value is known.

## Migration Plan

1. Add failing parser tests from the captured FreeBSD, OpenBSD, and NetBSD probe outputs.
2. Implement the smallest resolver additions in the existing category files.
3. Update `docs/schema/facts.yaml`, regenerate supported-fact docs, and update `CHANGELOG.md`.
4. Run `go test ./...`, `go vet ./...`, and the local BSD smoke checks.
