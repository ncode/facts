## 1. Mountpoints — Tests First

- [x] 1.1 Add a table test for the Linux statfs block math asserting `size`/`used`/`available` use `f_frsize` when `f_frsize != f_bsize`, and fall back to `f_bsize` when `f_frsize == 0` (statfs_linux_test.go, linux-tagged)
- [x] 1.2 Add a capacity test asserting `capacity = used/(used+available)` (via `f_bavail`): a reserved-block filesystem reports the `df` percentage and a full read-only mount (`f_bavail == 0`) reports `100%`, not `0%` (parity_fix_test.go + statfs_linux_test.go)
- [x] 1.3 Add/confirm a darwin or freebsd fixture test asserting their `mountpoints` byte values stay on `f_bsize` and remain unchanged by this change (existing TestFreeBSDMountpointsFactParsesMountOutput keeps f_bsize byte values; its capacity literal needs the df-formula update — see blocker note)

## 2. Mountpoints — Implementation

- [x] 2.1 Split `internal/engine/statfs_supported.go`: keep a `//go:build darwin || freebsd` file using `Bsize`, and add a `//go:build linux` `statMountpoint` using `Frsize` (fallback to `Bsize` when `Frsize == 0`)
- [x] 2.2 Change the capacity computation (`mountpointsFactWithSkip`/`filesystemCapacity` in `internal/engine/core.go`) to `used/(used+available)` using `AvailableBytes`, applied on all platforms
- [x] 2.3 Confirm `used_bytes` stays `(Blocks - Bfree) * blockSize` (matches `df` Used) and only the capacity denominator changed (used_bytes math unchanged in statfs + parseDFP512Stats; only filesystemCapacity denominator changed)

## 3. Networking — Tests First

- [x] 3.1 Add a Linux test asserting `networking.<iface>` exposes flattened `ip`/`netmask`/`network` (from the first IPv4 binding) and `ip6`/`netmask6`/`network6`/`scope6` (from the first IPv6 binding) (parity_fix_test.go)
- [x] 3.2 Add a Linux test asserting an interface with no usable bindings emits no empty `ip`/`netmask`/`network`/`scope6` keys (parity_fix_test.go)

## 4. Networking — Implementation

- [x] 4.1 Add the missing `expandInterfaceBindings(interfaces)` call to the `linux` branch of `currentNetworkingData` in `internal/engine/core.go`, matching the other POSIX platforms

## 5. Verification

- [x] 5.1 Run targeted tests for `./internal/engine` (mountpoints + networking)
- [x] 5.2 Run `go test ./...` (1299 passed, 0 failed)
- [x] 5.3 Run `go vet ./...` (clean), and `GOOS=linux`/`GOOS=darwin`/`GOOS=freebsd go build ./...` (all compile — statfs split verified)
- [x] 5.4 Re-validate on the `facts-dev` Lima VM: rebuilt in-VM and diffed against Facter 4.10.0 — mountpoints capacity/size now match (only volatile free-space bytes drift), networking is a byte-identical exact match
- [x] 5.5 `processors.models` stays `["aarch64"]` (intended, out of scope); no other facts changed; no schema/changelog update required
