# A machine-verified schema of every supported fact

## Why

There is no single answer to "what facts does Facts support?" — the canonical tree is spread across resolvers in `internal/engine`, per-platform, with conditional facts (cloud metadata, ZFS, virtualization) appearing only where applicable. Upstream Facter answered this with `lib/schema/facter.yaml`; our inherited `CONTRIBUTING.md` still pointed contributors at that nonexistent file. Facts should ship its own schema — and unlike upstream's, ours should be enforced by the platform gates so it can never drift from reality.

## What Changes

- Add **`docs/schema/facts.yaml`**: every supported fact as a dotted path with `type`, `description`, `platforms` (linux/darwin/windows/freebsd), and `conditional: true` where presence depends on host state (cloud metadata, swap, ZFS, DMI, virtualization details). Dynamic key segments use `*` patterns (`networking.interfaces.*.mtu`, `mountpoints.*`, `disks.*`, `processors.models.*`), the same convention upstream used.
- Add a **schema conformance test** that runs in the normal suite on every platform gate: discover on the host, flatten the canonical tree to leaf paths, and assert (a) every emitted path matches a schema entry whose `platforms` includes the host — no undocumented facts can ship; (b) every non-`conditional` schema entry for the host platform is present — the schema cannot overclaim.
- Add an authoring helper (`go test -run TestFactsSchema -schema-report` style flag or a small tool) that prints unmatched paths, so adding a fact tells you exactly what to document.
- `CONTRIBUTING.md` gains the rule the upstream file always wanted: new facts MUST be added to the schema (the gate enforces it). `README.md` links the schema as the "what do I get?" reference.

## Capabilities

### New Capabilities

- `facts-schema`: the schema document, its conformance guarantees, and the contributor rule.

### Modified Capabilities

(none — the schema constrains documentation, not fact behavior)

## Impact

- **New files**: `docs/schema/facts.yaml` (initial content authored from the live trees on the four gate platforms plus fixtures); a conformance test in the root `facts` package (or `tests/`), using the existing `gopkg.in/yaml.v3` dependency.
- **Docs**: CONTRIBUTING rule, README link, man page FILES/SEE ALSO pointer optional.
- **CI**: no new jobs — the test rides the existing four platform gates, which is exactly what makes the schema trustworthy.
- **Dependencies**: none added (`yaml.v3` already present).
- **Sequencing**: apply after `consolidate-port-history-docs` (it rewrites `CONTRIBUTING.md`; this change adds the schema section to it).
