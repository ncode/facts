## Why

Facts records two architecture contracts that the implementation does not yet fully satisfy: disabling every output of a shared core resolver must skip that resolver, and immutable Engines must not depend on package-global mutable hooks. Deep inspection also found a small set of test-only and shadow paths that make those gaps harder to see, while rejecting a broader networking refactor as unnecessary.

## What Changes

- Make the core-fact descriptor catalog decide whether every gateable resolver runs, using its exact emitted top-level roots; preserve eager inline facts and the `selinux` compatibility exception.
- Defer shared DMI acquisition until a kept DMI or GCE output needs it, while retaining eager virtualization discovery for `virtual` and `is_virtual`.
- Replace mutable config, cache, and default external-directory hooks with invocation-local immutable discovery inputs at the existing Engine/CLI seams.
- Remove mutable cache I/O test hooks and make external-fact timeout and byte limits fixed policy, while preserving warning, timeout, size-limit, and error semantics.
- Delete three unreachable resolver helpers and the two Plan 9 networking shadow seams, retargeting useful assertions to production paths.
- Add structural and public-surface tests that enforce descriptor agreement, resolver gating, invocation isolation, input precedence, and unchanged output behavior.
- Update `CHANGELOG.md` for the corrected disabled-resolver behavior. No fact names or schema entries change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `fact-disable-controls`: gate a multi-output or shared-output resolver when every top-level root it can emit is disabled, while running it whenever any sibling root remains enabled.
- `facts-library-api`: derive ambient system-following defaults into immutable invocation-local discovery inputs so concurrent Engines never require process-global mutation.

## Impact

The change affects core-fact descriptor scheduling, discovery input planning, cache and external-fact internals, CLI construction, and their tests under `internal/engine`, `internal/app`, and the root `facts` package. Public Go APIs, CLI flags, output bytes, input precedence, external-fact limits, supported fact schema, and dependencies remain unchanged.
