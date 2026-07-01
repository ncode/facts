## Why

The DragonFly `disks`/`partitions` probe has two independent problems, both confirmed on the nlab DragonFly guest while diagnosing the CI timeout:

1. **Fan-out.** `dragonFlyDisklabelTargets` blindly probes `device + s1..s4` (plus the whole disk) for *every* device in `kern.disks`. The guest lists **132 devices** (`da0`, `vn0..3`, `md0..md126`), so one discovery spawns **~660 `disklabel` processes** (~1m42s system time), of which exactly one (`disklabel da0s1`) is productive. The whole-disk target is never productive on DragonFly — the label lives on the slice, so `disklabel da0` returns `Operation not supported by device`.
2. **Phantom disks.** The 127 empty `md` memory disks also **leak into the `disks` fact** as phantom 9.77 MB disks, because `diskinfo /dev/md0` returns a positive size.

**Ground truth from the nlab guests reshaped the scope to DragonFly-only:**

| OS | `hw.disknames` / `kern.disks` | Phantom leak? |
|----|-------------------------------|---------------|
| **DragonFly** | `md1..md126 da0 vn0..3 md0` (132 devices) | **Yes** — 127 `md` phantoms, ~660 disklabel spawns |
| OpenBSD | `sd0:<duid>,fd0:` | No — `rd0`/`cd0` are **not** listed; `fd0` errors on `disklabel` and is dropped |
| NetBSD | `sd0 dk0 dk1` | No — `md0`/`cd0` are **not** listed; `dk` wedges are already handled |

On OpenBSD and NetBSD the kernel curates `hw.disknames` down to *attached* instances, so ram/memory/optical pseudo-devices never enter the probe in the first place. Adding `rd`/`md`/`cd` filtering there would be dead code — and filtering `rd` would be actively **harmful** in a `bsd.rd` recovery boot, where `rd0` *is* the root disk. So this change touches DragonFly only; OpenBSD/NetBSD keep their current behavior, including the existing NetBSD `dk`-wedge handling.

Ruby Facter has **no BSD disks/partitions resolver** (both are Linux-only), so there is no parity constraint — reporting only real host storage is a strict improvement.

## What Changes

**DragonFly only:**
- `dragonFlyDisklabelTargets` enumerates the slices that actually exist for a device (glob `/dev/<dev>s<N>`) instead of the fixed `device + s1..s4`, and drops the never-productive whole-disk target. This eliminates the fan-out.
- Drop DragonFly memory-disk / optical pseudo-devices (`md`, `cd`) from both the `disks` fact and the partition probe, matched by **driver class** (name with trailing digits stripped), so the 127 `md` phantoms disappear and no `disklabel` is spawned for them.
- Unattached `vn` needs no special-casing: it is already excluded from `disks` by the existing size gate (`diskinfo` returns no size), and the glob yields no slice nodes for it (`/dev/vn0s*` is empty), so it contributes zero `disklabel` spawns. *Attached* `vn` still has slice nodes and a positive size, so it is kept.

**OpenBSD / NetBSD:** no change.

## Impact

- **Behavior**: DragonFly `disks`/`partitions` no longer report empty `md` phantoms; the real `da0` disk, its `da0s1` slice partitions, and any *attached* `vn` are unchanged. OpenBSD/NetBSD facts are byte-for-byte unchanged.
- **Performance**: DragonFly discovery drops from ~660 `disklabel` spawns to one. Other platforms unaffected (no fan-out to begin with).
- **Not lost**: mounted pseudo-devices (tmpmfs `/tmp`, attached `vn` images) still surface in the **`mountpoints`** fact, which is intentionally not device-class filtered.
- **Out of scope**: OpenBSD/NetBSD/FreeBSD disk probes; the Linux/darwin/Windows/illumos disk probes; the CI `go test -timeout` bump (already landed).
