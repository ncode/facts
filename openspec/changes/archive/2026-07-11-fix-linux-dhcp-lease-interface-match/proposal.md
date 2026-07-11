## Why

Linux DHCP lease discovery can assign a lease for a similarly named interface to the requested interface because `leaseMatchesInterface` uses a broad substring check against lease content. For example, a lease declaring `interface "eth0-backup"` can match `eth0`.

## What Changes

- Match explicit dhclient `interface "name"` declarations exactly when lease content contains interface declarations.
- Preserve the existing filename fallback for lease files that do not declare an interface in their content.
- Render YAML maps inside sequence items as flow maps so multi-key nested map values remain valid YAML.
- Emit Plan 9 uptime duration fields with the same 64-bit numeric types as other platforms.
- Clone mutable values, including pointer targets, when returning Snapshot values and defensive copies.
- Add deterministic unit coverage for the substring-collision case.

## Impact

- **Code**: `internal/engine/networking.go`, `internal/engine/networking_test.go`, `internal/engine/formatter.go`, `internal/engine/formatter_test.go`, `internal/engine/plan9.go`, `internal/engine/plan9_parser_test.go`, `internal/engine/snapshot.go`, `internal/engine/snapshot_test.go`.
- **Behavior**: Linux `networking.interfaces.<name>.dhcp` avoids using DHCP server data from a different interface whose name merely contains the requested interface name.
- **Behavior**: YAML output preserves nested multi-key sequence maps, Plan 9 uptime facts use the shared 64-bit duration value types, and Snapshot value/copy accessors do not share mutable state.
- **Docs/schema**: No schema update; `CHANGELOG.md` records the user-visible fixes.
