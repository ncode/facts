## Why

Facts now has repeatable native lab access for DragonFly BSD and OmniOS/illumos. We should use that lab to promote both from candidate targets to honest supported release targets, without pretending OmniOS validates Oracle Solaris or that Ruby Facter byte parity is the ceiling.

## What Changes

- Define DragonFly BSD and illumos as candidate release targets with explicit promotion criteria.
- Add native validation paths for DragonFly, illumos, and amd64 lab variants of FreeBSD/OpenBSD/NetBSD through untracked `.local` SSH wrappers.
- Add DragonFly and illumos schema platform vocabulary and only claim fact paths proven by fixtures plus native lab gates.
- Add DragonFly-first, illumos-second fact coverage, allowing Facts-native extensions when native sources are stable and schema-documented.
- Add tracked DragonFly and illumos release-gate scripts, with lab connection details kept out of git.
- Promote release artifacts only for Go-supported tuples: `dragonfly/amd64` and `illumos/amd64`; Oracle Solaris remains a separate future candidate target.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `go-port-supported-platform-facts`: add candidate-target lifecycle, DragonFly fact coverage, illumos fact coverage, and Facts-native extension rules for targets without useful Ruby Facter parity.
- `facts-schema`: add `dragonfly` and `illumos` platform vocabulary and require strict schema claims for newly emitted target facts.
- `go-port-ci-platform-gates`: add candidate native gates and eventual blocking gates for DragonFly/illumos without hardcoding lab addresses.
- `go-port-distribution-and-cutover`: add Go-supported DragonFly/illumos release artifact tuples after promotion.
- `go-port-completion-verification`: add local lab-backed verification paths and keep existing Lima FreeBSD coverage until a separate cleanup removes it.

## Impact

- Code: OS, networking, memory, processors, disks, mountpoints, uptime, virtualization, SSH, schema conformance, and release-gate scripts.
- Docs: `CONTEXT.md`, ADR-0012, `CONTRIBUTING.md`, `docs/schema/facts.yaml`, generated supported-fact pages, and OpenSpec platform specs.
- Tooling: Makefile wrapper variables/targets for untracked `.local` SSH wrappers; no lab hostnames, IPs, or keys are committed.
