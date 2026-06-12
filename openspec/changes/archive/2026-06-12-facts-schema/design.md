# Design: The facts schema

## Context

The canonical tree has no central registry: resolvers in `internal/engine` emit facts per platform, some unconditionally (`os`, `kernel`, `networking`), some conditionally (cloud metadata, ZFS, `disks`, swap, virtualization detail). On macOS the tree is 22 top-level facts / ~369 leaf nodes; the four-platform union is larger. Upstream Facter documented its tree in `lib/schema/facter.yaml` using dotted paths with `.*` patterns for dynamic segments, but nothing enforced it. Our advantage: four blocking platform gates already run the full suite on real hosts — a conformance test riding them makes the schema self-policing.

## Goals / Non-Goals

**Goals:**
- One YAML file answers "what does Facts emit?" per platform, with types and descriptions.
- Drift is impossible in either direction: undocumented facts fail the gates; overclaimed facts fail the gates.
- Adding a fact has an obvious, tool-assisted documentation step.

**Non-Goals:**
- No behavior changes: the schema describes, never filters or validates output.
- No `facts --schema` CLI surface yet (possible follow-up; the file is the contract for now).
- No per-value validation (types are documentation granularity — `string`/`integer`/`double`/`boolean`/`map`/`array` — not a JSON-Schema type system).
- No documentation of operator-supplied facts (external/registered facts are by definition outside the schema).

## Decisions

**1. Hand-authored YAML, machine-enforced — not generated.**
Generation needs a host that exhibits every conditional fact (no such host exists) or fixture synthesis for every resolver (enormous). Hand-authoring with two-sided gate enforcement gets the same trust guarantee: the union of four live gates plus the no-undocumented-facts rule means the file can only describe reality. Initial authoring is bootstrapped by running the report mode on each gate platform (and locally via the captures in `/tmp/facter-parity` style runs).

**2. Schema shape: flat map of dotted patterns.**
```yaml
networking.interfaces.*.mtu:
  type: integer
  description: MTU of the interface.
  platforms: [linux, darwin, windows, freebsd]
ec2_metadata:
  type: map
  description: EC2 instance metadata tree.
  platforms: [linux, windows]
  conditional: true   # only on EC2 instances
```
`*` matches exactly one path segment (interface names, mountpoint paths, disk names, array indices). Subtrees that are operator- or provider-shaped (e.g. `ec2_metadata`, `system_profiler`) are documented at the subtree root with `type: map` rather than per-leaf — the leaves are provider-defined. A `conditional` entry participates in rule (a) (no undocumented facts) but is exempt from rule (b) (must-be-present).

**3. Matching semantics in the conformance test.**
Flatten the discovery tree to leaf paths (arrays contribute `path.*` once, not one path per index). For each leaf, longest-prefix match against schema entries: an exact or pattern match wins; a `type: map` ancestor entry covers all deeper leaves. This keeps the schema readable (~250 entries, not 1,500) while still pinning every stable key.

**4. Enforcement lives in the standard suite.**
A single `TestFactsSchemaConformance` in the root package: hermetic-engine discovery (core facts only — operator sources are out of schema scope by construction), flatten, two assertions, report mode behind a flag. It runs wherever `go test ./...` runs — all four gates — with zero new CI surface.

**5. Types vocabulary mirrors upstream.**
`string`, `integer`, `double`, `boolean`, `map`, `array` — familiar to anyone coming from Facter's schema, and sufficient for documentation.

## Risks / Trade-offs

- [A conditional fact never appears on any gate (e.g. EC2 metadata in CI)] → rule (b) exempts conditionals; their entries are reviewed by humans and exercised by the resolver fixture tests instead. Honest cost: conditional descriptions can go stale; acceptable for documentation.
- [Gate hosts differ run to run (interfaces, disks)] → `*` patterns absorb instance-specific keys; the test matches patterns, not literals.
- [Schema review fatigue: every new fact needs an entry] → that is the point; the report mode makes it a 30-second task.
- [Root-package test needs engine internals] → it uses only the public `facts` API (`New()`, `Discover`, `Tree()`), keeping it a consumer-grade check.

## Migration Plan

Single PR after `consolidate-port-history-docs`: conformance test (failing), author the schema until all four gates pass (the report mode drives it; Linux/Windows/FreeBSD entries verified by their gates on the PR), CONTRIBUTING section, README link, CHANGELOG entry.

Rollback: revert; documentation only.

## Open Questions

None.
