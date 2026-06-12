# Delta: go-port-supported-platform-facts

## ADDED Requirements

### Requirement: Primary IPv6 selection prefers routable addresses
When selecting the primary IPv6 address for `networking.ip6`, `networking.network6`, and `networking.scope6`, the Go port SHALL prefer routable addresses (global scope, then unique-local) over link-local addresses on the primary interface. This is a deliberate, documented deviation from Ruby Facter's first-bound-address rule, which can surface `fe80::` link-locals.

#### Scenario: Routable address wins over link-local
- **WHEN** the primary interface carries both a link-local (`fe80::/10`) and a routable (global or unique-local) IPv6 address
- **THEN** `networking.ip6` MUST report the routable address and `networking.scope6` its scope, regardless of binding order

#### Scenario: Link-local only
- **WHEN** the primary interface carries only link-local IPv6 addresses
- **THEN** `networking.ip6` MUST report the link-local address with `networking.scope6` of `link`

#### Scenario: Deviation is documented
- **WHEN** an operator reads the man page Go-port notes
- **THEN** the IPv6 selection deviation from Ruby Facter MUST be stated there
