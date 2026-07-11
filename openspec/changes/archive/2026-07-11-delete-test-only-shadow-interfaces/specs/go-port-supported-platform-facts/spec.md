## ADDED Requirements

### Requirement: Core-fact test-surface cleanup preserves production coordination

Removing obsolete internal test entrances SHALL NOT change core-fact match state, probe order, fallback behavior, or resolved output. Regression coverage for coordination decisions affected by this cleanup MUST observe production-owned category assembly or pure seams rather than a narrower parallel contract.

#### Scenario: DHCP match state remains fully covered

- **WHEN** DHCP lease-interface matching is tested through the production state-returning seam
- **THEN** coverage MUST include `(matched=false, explicit=false)`, `(matched=false, explicit=true)`, and `(matched=true, explicit=true)`
- **AND** the server value MUST be asserted for every relevant lease form, including a matched lease with an empty server value

#### Scenario: Uptime source selection remains covered

- **WHEN** uptime behavior is tested after the duration-only wrapper is removed
- **THEN** coverage MUST assert duration and `Known` through the production result
- **AND** fake-host call assertions MUST preserve the platform-specific probe/source-selection order

#### Scenario: DragonFly DMI fallback remains lazy

- **WHEN** DragonFly DMI coordination is tested after the convenience wrapper is removed
- **THEN** kenv data MUST short-circuit dmidecode through the production host seam
- **AND** dmidecode MUST remain a lazy fallback when kenv cannot supply the fact

#### Scenario: Test-surface cleanup preserves supported facts

- **WHEN** test-only entrances and dead utilities are removed
- **THEN** resolved fact names, values, platform gating, fallback order, and diagnostics MUST remain unchanged on every supported and candidate release target
