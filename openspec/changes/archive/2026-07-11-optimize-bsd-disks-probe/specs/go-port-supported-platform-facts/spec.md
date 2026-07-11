## ADDED Requirements

### Requirement: DragonFly disk probes drop empty memory disks without dropping real storage

Facts SHALL exclude DragonFly memory-disk and optical pseudo-devices (`md`, `cd`, matched by driver class — the device name with trailing digits stripped) from the DragonFly `disks` and `partitions` facts, while keeping real disks, their real slices, and attached file-backed (`vn`) disks.

#### Scenario: empty memory disks are neither probed nor reported

- **WHEN** `kern.disks` lists empty `md` memory disks alongside a real disk
- **THEN** the probe MUST NOT spawn a `disklabel` subprocess for the `md` devices
- **AND** `disks` and `partitions` MUST NOT report them

#### Scenario: real disks and attached file-backed disks are unchanged

- **WHEN** a real disk (`da0`) or an attached file-backed disk (a configured `vn0`) is present
- **THEN** its `disks` and `partitions` facts MUST be identical to before this change

### Requirement: The DragonFly partition probe enumerates only existing slices

Facts SHALL probe only the slices that actually exist for a DragonFly device — discovered by enumerating `/dev/<device>s<N>` slice nodes — rather than a fixed `s1`–`s4` set plus the whole disk, so discovery does not spawn wasted `disklabel` processes on hosts with many devices.

#### Scenario: no fan-out to non-existent slices

- **WHEN** the DragonFly probe enumerates a device's partitions
- **THEN** it MUST issue `disklabel` only for slice targets that exist on the device
- **AND** it MUST NOT issue a fixed `device + s1..s4` set of `disklabel` calls per device
- **AND** it MUST NOT issue `disklabel` on the whole-disk device (the DragonFly label lives on the slice)
