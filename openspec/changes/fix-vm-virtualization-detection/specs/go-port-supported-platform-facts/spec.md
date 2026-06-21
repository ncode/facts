## MODIFIED Requirements

### Requirement: Virtualization and cloud parity
The Go port SHALL match Ruby-compatible virtualization, hypervisor, and cloud metadata behavior on supported platforms, and MAY expose accurate Facts-native virtualization detection when a supported platform has stable native indicators not covered by Ruby Facter.

#### Scenario: Virtualization and hypervisor facts
- **WHEN** supported-platform virtualization facts are resolved from Linux `virt-what`, cgroups, DMI, VMware, Xen, OpenVZ, Windows OEM/netkvm/WMI indicators, macOS indicators, FreeBSD virtualization indicators, OpenBSD DMI product/vendor indicators, NetBSD DMI indicators, DragonFly DMI/PCI indicators, illumos SMBIOS/PCI indicators, or other indicators identified by the parity audit
- **THEN** the Go port MUST match Ruby `virtual`, `is_virtual`, `hypervisors.*`, Xen, container, and nil/unknown behavior for supported detection paths
- **AND** it MUST report QEMU/KVM guests as virtual when those supported native indicators expose QEMU, SeaBIOS, or Virtio metadata

### Requirement: Supported platform networking facts
The Go port SHALL omit optional primary networking facts when the platform
does not expose a primary value, instead of rendering empty string
placeholders.

#### Scenario: Optional primary networking values are absent when unresolved
- **WHEN** supported-platform networking facts are rendered for full structured output
- **THEN** optional top-level primary fields such as `networking.ip`, `networking.ip6`, `networking.mac`, `networking.netmask`, `networking.netmask6`, `networking.network`, `networking.network6`, `networking.primary`, and `networking.scope6` MUST be absent when unresolved
- **AND** populated primary networking values MUST still be emitted

### Requirement: Supported platform mountpoint facts
The Go port SHALL omit mountpoint entries that have no resolved stat, device,
filesystem, or option data.

#### Scenario: Empty mountpoint entries are absent
- **WHEN** supported-platform mountpoint facts are rendered for full structured output
- **THEN** entries with no mountpoint fields MUST be absent instead of rendering an empty map
- **AND** populated mountpoint entries MUST still be emitted
