## Decisions

- **All-on, disable-only (ADR-0015).** No opt-in tier, so no per-fact visibility metadata is introduced.
- **One disabled set, unioned** from `--disable`, `FACTS_DISABLE`, and the `disable` config key. `disable` is the facts-native spelling; the Facter `blocklist` key is its compatibility alias (native wins on collision) and is never removed, because `facter.conf` is part of the binding input contract.
- **Resolution-gating by class.** Standalone-resolver facts skip their resolver when disabled (`packages`). Multi-output category resolvers — `osCoreFacts` → `os`/`kernel`/`filesystems`, `disksCoreFacts` → `disks`/`partitions`/`mountpoints`/`zfs`/`zpool`, `uptimeCoreFacts` → `load_averages`/`system_uptime` — skip only when *all* their outputs are disabled, otherwise resolve-then-prune. Inline facts (`facterversion`, `is_virtual`/`virtual`, `path`) and sub-facts are resolve-then-prune. Gating is per shared **probe**: a memoized probe is skipped only when all its consumers are disabled (`cachedIdentity` feeds `identity` and the SSH privilege gate; DMI feeds `gce`/virtualization). `packages` shares no probe, so it gates cleanly.
- **`--no-block` is the master override** (clears the set). **Disable beats query**, matching the engine's existing order (the disabled set is applied before query projection). An explicitly-queried fact disabled by a non-command-line source emits a one-line stderr diagnostic with empty stdout, so a silently-empty result is diagnosable.
- **`FACTS_DISABLE` reserved by resolved name.** `environmentFactName` lowercases and strips any of `facts_`/`facts`/`facter_`/`facter`, so the loader drops any variable whose resolved fact name is `disable` (covering `FACTS_DISABLE`, `FACTSDISABLE`, `FACTER_DISABLE`, `FACTERDISABLE` uniformly) and routes it to the disabled set. Cost: no external environment fact may be named `disable`.
- **Cache invariant.** Subtract the disabled set before the cache is consulted and before any cache write; never persist a pruned sub-fact into a cached group.

## Implementation Notes

- `internal/engine/config.go`: parse a `disable` config key into the disabled set; keep `blocklist` feeding the same set; native wins on collision.
- `internal/cli/options.go` + `internal/app/app.go`: add `--disable` (valued, comma-split, repeatable) to the shared option vocabulary, feed the disabled set, and ensure `--no-block` clears the unioned set; document it for the CLI-option-contract tests.
- `internal/engine/external.go` (`environmentFactName` / loader): drop any env var whose resolved fact name is `disable` and route it to the disabled set instead of creating a fact.
- `internal/engine/discovery_plan.go`: union CLI + env + config into the disabled set.
- `internal/engine/core.go` / `buildCoreFacts`: introduce a fact/group → resolver mapping and pass the disabled set in; skip a resolver only when all its top-level outputs are disabled; gate per shared probe; standalone resolvers (`packages`) skip cleanly.
- `internal/engine/engine.go`: keep disabled-set subtraction before cache resolution and `Select`; emit the stderr diagnostic for an explicitly-queried-but-disabled fact.
- Re-audit `BuiltinFactGroups` to drop the legacy flat names removed by ADR-0007 from group membership (no-ops today), so `--disable <group>` relies on the structured root entry.
