## Why

The BSD disks/partitions probe enumerates every device from `kern.disks` and probes each with `disklabel`; on DragonFly it runs `disklabel` on five slice targets (`s1`–`s4` plus the device) per device. On a host with many pseudo-devices this fans out to hundreds of subprocess spawns per discovery. The nlab/CI DragonFly guest lists **132 devices, 127 of them `md` memory disks**, so one discovery spawns **660 `disklabel` processes** (~1m42s of system time), and every full `Run()` discovery pays it — which is why the emulated DragonFly VM's `internal/app` integration suite runs for ~11 minutes. The `md`/`vn` pseudo-devices are also not physical host storage and should not be reported as disks/partitions at all.

## What Changes

- The BSD disks/partitions probes **skip pseudo-devices** — memory disks (`md*`), vnode/file-backed disks (`vn*`), and optical devices (`cd*`) — when enumerating `kern.disks`.
- The DragonFly probe stops blindly spawning `disklabel` on non-existent slice targets for every device; it probes only real devices and their actual slices.
- Result: on hosts with many pseudo-devices, discovery drops from hundreds of subprocess spawns to a handful, and `disks`/`partitions` no longer list phantom memory/vnode disks.

## Impact

- **Behavior**: `disks`/`partitions` on the BSDs exclude `md*`/`vn*`/`cd*` pseudo-devices — a correctness improvement (these were never real host storage).
- **Performance**: full discovery is markedly faster on BSD hosts with many pseudo-devices; the emulated CI/lab DragonFly `internal/app` suite speeds up.
- **Compatibility**: real disks (`da*`, `ada*`, `ad*`, `wd*`, `sd*`, `vtbd*`, …) and their partitions are unchanged.
- **Out of scope**: the FreeBSD path (single `sysctl kern.geom.confxml` call, no per-device fan-out); the Linux/darwin/Windows/illumos disk probes; the CI `go test -timeout` bump (already landed on the disable-mechanism PR).
