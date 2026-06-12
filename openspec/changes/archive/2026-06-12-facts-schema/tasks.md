# Tasks: The facts schema

## 1. Conformance machinery

- [x] 1.1 Implement tree flattening (leaf paths; arrays contribute `path.*`) and longest-prefix pattern matching over schema entries (`*` = one segment; `type: map` ancestors cover subtrees) in a root-package test using only the public API
- [x] 1.2 Implement `TestFactsSchemaConformance`: (a) every emitted leaf matches a platform-applicable entry, failing with the unmatched paths; (b) every non-conditional entry for the platform is present, failing with the missing entries
- [x] 1.3 Add the authoring report mode (flag) that prints undocumented paths grouped by top-level fact

## 2. Author the schema

- [x] 2.1 Bootstrap `docs/schema/facts.yaml` from a macOS host run via the report mode (types, descriptions, platforms, conditional markers)
- [x] 2.2 Extend entries for Linux/Windows/FreeBSD facts (selinux, fips_enabled, system32, disks/partitions, ZFS/Zpool, hypervisors, cloud metadata) from resolver fixtures and gate runs; mark conditionals
- [x] 2.3 Iterate on the PR until all four platform gates pass both conformance rules

## 3. Docs and verification

- [x] 3.1 Add the schema rule and link to `CONTRIBUTING.md`; link the schema from README's docs table
- [x] 3.2 CHANGELOG entry (Added: published, gate-enforced fact schema)
- [x] 3.3 `go test ./...`, vet/gofmt clean locally; `openspec validate facts-schema` passes
- [x] 3.4 Platform CI gates green on the final commit (the conformance test riding all four is the acceptance proof)
