## Why

Facts has only an opt-out blocklist that filters already-resolved facts out of output (Facter `blocklist` parity). It has no CLI or environment control, no facts-native spelling, and it still resolves every fact before discarding it — so a voluminous fact like `packages` (ADR-0014) would pay its full collection cost on every run even when unwanted. ADR-0015 decides the model; this change implements it.

## What Changes

- Every fact is on by default; a fact is removed only by disabling it. There is no opt-in, allowlist, or default-off tier.
- The disabled set is the union of three inputs: `--disable a,b,c` (CLI), `FACTS_DISABLE=a,b,c` (environment), and the `facts.conf` `disable` key. `disable` is the facts-native term across all three; the Facter `blocklist` key is retained as its compatibility alias (native wins on collision) and is never removed.
- A disable target is a fact name or a fact group, reusing the existing group expansion.
- Disabling is **resolution-gated**: a fact produced by its own resolver is not resolved when disabled (e.g. `packages`); a fact sharing a multi-output category resolver is gated only when *all* that resolver's outputs are disabled, otherwise resolve-then-prune; a disabled sub-fact is resolve-then-prune. Gating is per shared probe.
- `--no-block` clears the entire disabled set for a run. Disable beats query; a fact disabled by environment/config (not the same command line) that is explicitly queried emits a one-line stderr diagnostic with empty stdout.
- `FACTS_DISABLE` is reserved by its resolved fact name `disable`, so it is not ingested as an external environment fact under any accepted prefix.
- The disabled set is subtracted before the cache is consulted and before any cache write.

## Impact

- **Behavior**: new `--disable` / `FACTS_DISABLE` / `disable` controls; disabling now skips resolution for standalone-resolver facts; a Facts-native `packages` group diverges the Facter-mirrored `--list-block-groups`/`--list-cache-groups` output.
- **Compatibility**: additive — the Facter `blocklist` config key keeps working unchanged, and existing `--no-block` semantics are preserved.
- **Out of scope**: any opt-in / default-off / allowlist tier and per-fact "optional" metadata; the `packages` fact itself (separate change `add-packages-fact`); splitting existing multi-output category resolvers into single-output resolvers (they stay resolve-then-prune until split).
