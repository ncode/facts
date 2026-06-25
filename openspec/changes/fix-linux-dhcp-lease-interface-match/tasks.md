## 1. Implementation

- [x] 1.1 Add exact matching for explicit dhclient interface declarations.
- [x] 1.2 Keep filename fallback for lease files without interface declarations.
- [x] 1.3 Cover the substring-collision regression with a focused unit test.
- [x] 1.4 Ensure the latest matching lease block controls the DHCP server value.
- [x] 1.5 Ignore commented or quoted DHCP server option text in dhclient leases.

## 2. Verification

- [x] 2.1 Run focused networking tests.
- [x] 2.2 Run `go test ./...`.
- [x] 2.3 Run `go vet ./...`.
- [x] 2.4 Run release gates.
- [x] 2.5 Run `openspec validate fix-linux-dhcp-lease-interface-match --strict`.
