# Fix Darwin networking facts: hostname split, interface enumeration, IPv6 selection

## Why

The 2026-06-11 host comparison against Ruby Facter 4.10.0 on macOS found three networking defects: `networking.hostname` holds the FQDN (`dream-factory.lan`) instead of the short hostname (`dream-factory`), `networking.interfaces` misses address-less tunnel interfaces (`gif0`, `stf0`), and the primary IPv6 selection differs from Ruby (Go picks the routable ULA, Ruby picks the first-bound link-local). The first two are plain bugs against the existing macOS parity requirement; the third is Ruby behavior we deliberately do not want.

## What Changes

- Fix `networking.hostname` to the short host name; `networking.fqdn` stays `hostname.domain`; `networking.domain` unchanged. (Same fix applies wherever the hostname probe is shared.)
- Fix interface enumeration to include interfaces without addresses (macOS `gif0`/`stf0` tunnels), with their MTU and flags as Ruby reports them, under `networking.interfaces`.
- Keep Go's primary-IPv6 selection: prefer routable (global/unique-local) addresses over link-local for `networking.ip6`, `networking.network6`, and `networking.scope6`. Ruby's first-bound-wins rule (which surfaces `fe80::` link-locals) is recorded as a deliberate, documented deviation — a link-local address is rarely the answer anyone querying "the host's IPv6 address" wants.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-supported-platform-facts`: adds a requirement pinning primary-IPv6 selection (routable preferred over link-local) as a documented deviation from Ruby; hostname and interface-enumeration fixes are bug fixes under the existing parity requirement and need no spec change.

## Impact

- **Code**: the Darwin hostname probe and shared hostname/domain splitting in `internal/facter/` (core networking resolver); interface enumeration on Darwin (and any shared getifaddrs-style path); IPv6 primary-address selection logic.
- **Tests**: fixture-backed tests for short-hostname splitting (host with and without a domain), address-less interface inclusion, and IPv6 selection order (global > unique-local > link-local); existing networking tests updated where they pinned the buggy values.
- **Docs**: IPv6-selection deviation noted in the man page GO PORT NOTES; CHANGELOG entry.
- **Behavioral**: `networking.hostname` shortens on hosts with a domain; `networking.interfaces` gains tunnel interfaces; `networking.ip6`/`scope6` keep their current (routable) values, now as documented behavior.
- **Dependencies**: independent; pairs with `remove-legacy-facts` (which deletes the flat `hostname`/`interfaces`/`ipaddress6`/`mtu_*` aliases this would otherwise also affect).
