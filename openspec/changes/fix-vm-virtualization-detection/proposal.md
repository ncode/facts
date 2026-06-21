## Why

Native fleet content validation found several lab VMs emitting
`virtual: "physical"` and `is_virtual: false` even though their host metadata
exposes QEMU/KVM indicators.

## What Changes

- Detect QEMU/KVM from DMI/SMBIOS/PCI indicators on OpenBSD, NetBSD,
  DragonFly BSD, and illumos.
- Detect Windows Server 2025 QEMU/KVM VMs through WMI manufacturer/model/BIOS
  indicators.
- Omit empty optional top-level networking strings found by full-tree native
  validation, instead of rendering placeholder values.
- Omit empty mountpoint entries that have no stat, device, filesystem, or
  option data.

## Impact

- VM guests now report `virtual: "kvm"` and `is_virtual: true` instead of
  physical when QEMU/KVM metadata is present.
- Full output no longer renders empty primary networking placeholders when a
  supported platform has no primary value for that field.
- Full output no longer renders empty mountpoint maps when no mountpoint data
  can be resolved.
