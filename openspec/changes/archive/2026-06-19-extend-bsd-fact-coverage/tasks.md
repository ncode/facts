## 1. Fixtures And Tests

- [x] 1.1 Add fixture-backed tests for NetBSD `df -P` mountpoint byte and capacity parsing.
- [x] 1.2 Add fixture-backed tests for FreeBSD/OpenBSD/NetBSD `ifconfig` operational state parsing.
- [x] 1.3 Add fixture-backed tests for FreeBSD `ifconfig -m` speed and duplex parsing.
- [x] 1.4 Add fixture-backed tests for FreeBSD DHCP lease parsing from `/var/db/dhclient.leases.<iface>`.
- [x] 1.5 Add fixture-backed tests for FreeBSD GEOM partition type and disk rotation parsing.
- [x] 1.6 Add DMI tests that lock the OpenBSD/NetBSD product-version disposition.

## 2. Core Implementation

- [x] 2.1 Extend NetBSD mountpoint resolution to emit byte and capacity fields from `df -P` when available.
- [x] 2.2 Extend BSD networking enrichment for interface operational state.
- [x] 2.3 Extend FreeBSD networking enrichment for interface speed and duplex.
- [x] 2.4 Extend FreeBSD DHCP resolution from interface lease files and primary-interface selection.
- [x] 2.5 Extend FreeBSD GEOM parsing for `partitions.*.parttype`.
- [x] 2.6 Emit FreeBSD `disks.*.type` only when GEOM reports a known rotation rate.
- [x] 2.7 Implement or explicitly document the OpenBSD/NetBSD `dmi.product.version` decision.
- [x] 2.8 Keep NetBSD DHCP absent unless a stable text source is selected and tested.

## 3. Schema And Docs

- [x] 3.1 Update `docs/schema/facts.yaml` platform lists and conditional markers for newly emitted BSD fact paths.
- [x] 3.2 Regenerate `docs/supported-facts/` from the schema.
- [x] 3.3 Update `CHANGELOG.md` for the user-visible BSD fact coverage changes.
- [x] 3.4 Update compatibility documentation if the DMI product-version decision is a documented deviation.

## 4. Validation

- [x] 4.1 Run `gofmt -w` on edited Go files.
- [x] 4.2 Run `go test ./...`.
- [x] 4.3 Run `go vet ./...`.
- [x] 4.4 Run schema conformance with `go test -run TestFactsSchemaConformance .`.
- [x] 4.5 Run local FreeBSD, OpenBSD, and NetBSD smoke checks where available.
