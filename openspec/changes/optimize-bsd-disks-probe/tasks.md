## 1. Tests First

- [x] 1.1 DragonFly disks: with a fake `commandRunner`, assert `md*` devices from `kern.disks` are absent from the `disks` fact and no `diskinfo` phantom leaks, while `da0` is byte-identical to today.
- [x] 1.2 DragonFly partitions: with a recording `commandRunner` + fake `pathGlobber`, assert **zero** `disklabel` spawns for `md*`, and that a device is probed only on the slices the globber reports — `disklabel da0s1` runs; `disklabel da0`, `da0s2`, `da0s3`, `da0s4` do not.
- [x] 1.3 `dragonFlyDisklabelTargets`: given a globber returning `/dev/da0s1 /dev/da0s1a /dev/da0s1b /dev/da0s1d`, assert it returns exactly `[da0s1]` (slice only, not the partition-within-slice nodes, not the whole disk).
- [x] 1.4 Unattached `vn`: given an empty glob for `/dev/vn0s*`, assert the partition probe issues no `disklabel` for `vn0`. Attached `vn`: a positive test asserts a configured `vn0` (positive `diskinfo`, `/dev/vn0s1` slice node) stays in `disks` and its partitions are probed.

## 2. Implementation

- [x] 2.1 Add `dragonFlyPseudoDisk(name string) bool` (driver class ∈ `{md, cd}`); apply in `currentDragonFlyDisks` and `currentDragonFlyPartitions`.
- [x] 2.2 Rewrite `dragonFlyDisklabelTargets` to take a `pathGlobber`, glob `/dev/<dev>s*`, and return only `<dev>s<digits>` slice names; drop the whole-disk target.
- [x] 2.3 Thread `s.glob` into `currentDragonFlyPartitions`; update the `currentPartitions` dispatch.

## 3. Verification

- [x] 3.1 `go test ./...` and `go vet ./...`.
- [x] 3.2 nlab validation on the DragonFly guest: release gate passes; `disks` = `{da0}` only (127 `md` phantoms gone); `partitions` = real `da0s1a`/`b`/`d` unchanged; discovery `disks partitions` drops from ~1m42s system time to **0.65s**; `disklabel` spawns drop from ~660 to ~1. OpenBSD/NetBSD code paths are untouched (DragonFly-only change), so their facts are byte-identical by construction.
- [x] 3.3 `openspec validate optimize-bsd-disks-probe --strict`.
