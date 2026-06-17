## 1. Baseline

- [ ] 1.1 Record the current `go test ./...` pass/fail baseline (counts + any failing names) to diff against
- [ ] 1.2 Capture a local, uncommitted same-host core-fact output baseline (`facts --json` or equivalent `CoreFacts` dump) for before/after comparison; do not add a committed host-specific value golden
- [ ] 1.3 Confirm `GOOS=linux|darwin|windows|freebsd go build ./...` all compile before starting

## 2. Carve by category (one commit each, green test between)

- [ ] 2.1 `networking.go` — hostname/fqdn/domain, interfaces, primary selection, DHCP, ifinet6, route bindings (all platforms); extract `networkingCoreFacts(s)`
- [ ] 2.2 `processors.go` — count/cores/threads/models/isa/speed/extensions/physicalcount; extract `processorsCoreFacts(s)`
- [ ] 2.3 `memory.go` — system + swap memory, capacity; extract `memoryCoreFacts(s)`
- [ ] 2.4 `os.go` — name/family/release/architecture/hardware, kernel, distro, macOS, Windows product; extract `osCoreFacts(s)`
- [ ] 2.5 `dmi.go` — `/sys/class/dmi`, Windows/FreeBSD/OpenBSD/macOS DMI; extract `dmiCoreFacts(s)`
- [ ] 2.6 `disks.go` — disks + partitions + mountpoints; extract `disksCoreFacts(s)`
- [ ] 2.7 `ssh.go` — host key discovery + fingerprints; extract `sshCoreFacts(s)`
- [ ] 2.8 `identity.go`, `uptime.go`, `selinux.go`, `fips.go`, `timezone.go`, `augeas.go`, and `xen.go`; extract their `*CoreFacts(s)`
- [ ] 2.9 Hybrid-split any category file that is unwieldy using a NON-GOOS suffix (e.g. `networking_msft.go`); never `*_windows.go`/`*_linux.go`/`*_darwin.go`/`*_freebsd.go`

## 3. Orchestrator

- [ ] 3.1 Reduce `buildCoreFacts` to the ordered composition of the `*CoreFacts(s)` functions; keep only genuinely shared helpers in `core.go`
- [ ] 3.2 Confirm each `*CoreFacts(s)` function returns only that category's facts and does not call `CoreFacts`, `buildCoreFacts`, or another category assembly function
- [ ] 3.3 Confirm no helper signatures changed (the `commandRunner`/`fileReader` collapse stays the separate deferred change)

## 4. Tests follow the code

- [ ] 4.1 Split `core_test.go` along the same category lines (tests move with their functions; no assertions changed unless a function's file-local name changes)
- [ ] 4.2 Add focused category-assembly tests where the new `*CoreFacts(s)` function owns non-trivial assembly logic; avoid duplicating parser tests that already moved with the code
- [ ] 4.3 Compare the post-carve same-host core-fact output to the 1.2 baseline, accepting only expected volatile host values

## 5. Verification

- [ ] 5.1 Same-host core-fact comparison completed (tasks 1.2 and 4.3)
- [ ] 5.2 `go test ./...` green vs the 1.1 baseline; `go vet ./...` clean
- [ ] 5.3 `GOOS=linux|darwin|windows|freebsd go build ./...` all compile (no accidental GOOS-suffix build constraint)
- [ ] 5.4 `docs/adr/0010-core-facts-split-by-category.md` present and accurate
- [ ] 5.5 `openspec validate 2026-06-17-split-core-facts --strict`
