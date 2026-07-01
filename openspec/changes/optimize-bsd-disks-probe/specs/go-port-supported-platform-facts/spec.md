## ADDED Requirements

### Requirement: BSD disk probes skip pseudo-devices and bound subprocess fan-out

Facts SHALL exclude pseudo-devices — memory disks (`md*`), vnode/file-backed disks (`vn*`), and optical devices (`cd*`) — from the BSD `disks`/`partitions` probes, and SHALL NOT spawn a partition-probe subprocess for non-existent slice targets, so discovery stays fast on hosts with many pseudo-devices while real-disk facts are unchanged.

#### Scenario: pseudo-devices are neither probed nor reported

- **WHEN** `kern.disks` lists `md*`, `vn*`, or `cd*` pseudo-devices alongside real disks
- **THEN** the disks/partitions probe MUST NOT spawn a `disklabel` (or equivalent) subprocess for the pseudo-devices
- **AND** `disks` and `partitions` MUST NOT report the pseudo-devices

#### Scenario: real-disk facts are unchanged

- **WHEN** a real disk such as `da0`, `ada0`, or `wd0` is present
- **THEN** its `disks` and `partitions` facts MUST be identical to before this change

#### Scenario: DragonFly does not fan out to non-existent slices

- **WHEN** the DragonFly probe enumerates a device's partitions
- **THEN** it MUST NOT spawn a `disklabel` subprocess for slice targets that do not exist on the device
