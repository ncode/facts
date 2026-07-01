## 1. Tests First

- [ ] 1.1 Add a probe test (fake `commandRunner` recording calls) proving `md*`/`vn*`/`cd*` devices from `kern.disks` are skipped — no `disklabel` (or equivalent) subprocess is issued for them.
- [ ] 1.2 Add a test proving real devices (`da0`, `ada0`, `wd0`, …) are still probed and their partitions parsed unchanged.
- [ ] 1.3 Add a DragonFly test proving the probe does not spawn `disklabel` for non-existent slice targets of every device.

## 2. Implementation

- [ ] 2.1 Add a shared `isPseudoDisk`/`filterRealDisks` helper and apply it in `currentDragonFlyPartitions`, `currentOpenBSDPartitions`, `currentNetBSDPartitions`, and the disks-list enumeration (`disks.go`).
- [ ] 2.2 DragonFly: bound the slice-target fan-out to slices that actually exist (or derive slices from a single `disklabel <device>`).

## 3. Verification

- [ ] 3.1 Run `go test ./...` and `go vet ./...`.
- [ ] 3.2 nlab validation on the DragonFly/OpenBSD/NetBSD guests: measure the drop in `disklabel` spawns and discovery time, confirm `partitions` excludes `md*`, confirm real-disk facts unchanged, and run the native release gates.
- [ ] 3.3 Run `openspec validate optimize-bsd-disks-probe --strict`.
