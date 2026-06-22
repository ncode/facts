## Context

Facts already has the right high-level boundaries: the public library API stays idiomatic Go, the `facts` CLI is the compatibility surface, and the canonical structured tree is the only fact surface. The current friction is inside those boundaries.

Four internal contracts are doing more work than their code shape admits:

- Schema semantics live in both `schema_test.go` and `tools/supportedfacts`.
- CLI option metadata lives across validation, app runtime, help/man text, and installed docs.
- Platform target vocabulary lives across schema, docs generation, Makefile targets, CI, and category policy.
- Session owns run-scoped discovery state, but category modules still call host/runtime APIs directly in places.

## Goals / Non-Goals

**Goals:**

- Give schema loading, validation, path matching, and platform inclusion one internal implementation.
- Give `facts` CLI option metadata one small internal source used by validation and documentation checks.
- Give platform target identity and coarse capability policy one internal profile table.
- Keep `Session` as the run-scoped discovery module while extending the host probe seam only where it improves testability.
- Preserve current fact output except for stricter schema failures and corrected CLI documentation.

**Non-Goals:**

- No public Facts API changes.
- No new CLI framework.
- No resolver registry.
- No GOOS-suffixed resolver split; ADR-0010 category organization remains.
- No attempt to fake native platform gates with unit tests.

## Decisions

### Use one change with four small workstreams

The four candidates share the same problem: shallow internal contracts. Keeping them in one proposal lets the design preserve consistent boundaries while implementation can still land as small independent commits.

Alternative considered: four separate changes. That adds process overhead and repeats the same context in each proposal.

### Add an internal schema contract first

Create an internal schema package that owns entry loading, platform vocabulary, validation, tree flattening, and schema path matching. `schema_test.go` and `tools/supportedfacts` should become adapters over it.

The matching rule should distinguish dynamic keyed maps from explicitly open subtrees. Entries such as `disks.*` and `mountpoints.*` are not blanket approval for arbitrary descendants; open provider-shaped metadata subtrees can remain open when explicitly marked.

Alternative considered: move only the duplicated `schemaEntry` struct. That is cleanup, not depth.

### Keep CLI option work as metadata, not a framework

Centralize option names, aliases, arity, repeatability, task flags, conflicts, and documentation rows in `internal/cli`. Keep `flag.FlagSet` and app execution logic in place.

Alternative considered: generate the entire CLI parser/help/man output. That is unnecessary for current drift.

### Make platform profile policy-only

Add a target profile keyed by GOOS for identity, labels, support tier, schema visibility, compile/dist target membership, and coarse capability policy. Category modules still own resolver implementation and parsing.

Alternative considered: use the profile as a resolver registry. That would fight ADR-0010 and make category code harder to reason about.

### Extend Session's host seam only where needed

Do not split Session's memoized discovery state. Add host seam operations only when a category currently reaches around Session for host/runtime data and tests need injection. The first useful slice is disks because it uses commands, files, stats, directory reads, globbing, and platform selection.

Alternative considered: a new probe module for every category. That risks shallow wrappers around `os` and `runtime`.

## Risks / Trade-offs

- Stricter schema matching may expose undocumented emitted leaves -> treat failures as contract signal and update schema or resolver output deliberately.
- Platform profile can become a dumping ground -> keep it to target policy and coarse capabilities, not parser bodies.
- CLI option metadata can mirror `flag` poorly -> centralize only vocabulary and docs metadata, not parsing mechanics.
- Host seam expansion can become a wrapper for every standard library call -> add methods only when they remove direct host access from fact resolution paths and improve fixture tests.

## Migration Plan

1. Land schema contract extraction and stricter matching with tests.
2. Land CLI option metadata and documentation drift checks.
3. Land platform profile for vocabulary and low-risk identity/capability policy.
4. Land the narrow Session host seam slice, starting with disk/mount/partition probes.
5. Run local `go test ./...`, `go vet ./...`, and docs drift checks after each slice.
6. Use facts-lab native gates only for slices that change platform probe behavior or target policy.
