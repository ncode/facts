# DragonFly and illumos are separate candidate release targets

Facts will evaluate DragonFly BSD and illumos as candidate release targets before promoting them to supported release targets. The illumos target is validated through OmniOS; Oracle Solaris is a separate future candidate target and is not validated by OmniOS, though Solaris Facter behavior may inform shared SunOS-compatible facts.

Candidate-target facts must be schema-documented, fixture-backed, and validated in the native lab before they are claimed. Ruby Facter byte parity is not the ceiling: Facts may expose Facts-native extensions when the host source is stable, the canonical fact spelling is documented, and native validation proves the behavior.

## Considered Options

- **Treat OmniOS as Solaris validation** — rejected: Go exposes `illumos` and `solaris` separately, and Oracle Solaris needs its own repeatable host.
- **Require Ruby Facter parity only** — rejected: DragonFly has no useful upstream Facter target, and Facts already intentionally supports accurate schema-owned facts beyond Ruby Facter in some areas.
- **Promote both targets immediately** — rejected: the schema and native gates must be honest before the targets become release-blocking.
