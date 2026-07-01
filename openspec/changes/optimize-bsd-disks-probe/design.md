## Decisions

- **Denylist known pseudo-devices, do not whitelist real ones.** Filter the `kern.disks` device list to exclude the pseudo-device prefixes `md` (memory disk), `vn` (vnode/file-backed disk), and `cd` (optical) before probing. A denylist of well-known pseudo-devices is robust to new real-disk driver prefixes; a whitelist would silently drop unfamiliar real disks.
- **DragonFly: stop the unconditional five-target fan-out.** Instead of spawning `disklabel` on `<device>`, `<device>s1`…`<device>s4` for every device, probe the device and only the slices that actually exist. Investigate whether one `disklabel <device>` already enumerates the slices, avoiding per-slice spawns entirely.
- **Behavior-preserving for real disks.** The `disks`/`partitions` facts for real storage must be byte-identical to today; only pseudo-devices are removed and only wasted subprocesses are eliminated.

## Implementation Notes

- Add a shared helper (`isPseudoDisk(name) bool` / `filterRealDisks([]string) []string`) and apply it wherever a BSD probe iterates `kern.disks`: `currentDragonFlyPartitions`, `currentOpenBSDPartitions`, `currentNetBSDPartitions`, and the disks-list enumeration in `disks.go`.
- TDD: table tests with a fake `commandRunner` asserting (a) `md*`/`vn*`/`cd*` devices are skipped — no `disklabel` spawned for them (record the runner's calls), (b) real devices are still probed and parsed, (c) partition output for real disks is unchanged.
- Validate on the nlab DragonFly/OpenBSD/NetBSD guests: discovery time and `disklabel` spawn count drop sharply, `partitions` excludes `md*`, real-disk facts are unchanged, and the native release gates still pass.
