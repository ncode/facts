# Delta: go-port-framework-fidelity

## REMOVED Requirements

### Requirement: Bounded and documented --puppet behavior
**Reason**: `--puppet` is removed entirely (ADR-0009). Facts implements Facter's input/output contract, not Puppet's runtime behavior; auto-discovering Puppet's agent plugin-fact destination and warning about pluginsync'd Ruby plugin custom facts are out of scope.
**Migration**: Puppet's plugin-fact destination is reachable like any directory via `--external-dir /opt/puppetlabs/puppet/cache/facts.d` (or the platform/user equivalent). Facter's own external-fact directories — the `puppetlabs/facter/facts.d` defaults in the always-on default set — are unaffected. A `.rb` file in any external directory is still skipped with a warning (see `go-port-custom-fact-dsl-contract` → "No Ruby DSL is read anywhere").
