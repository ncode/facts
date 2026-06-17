# Schema owns canonical fact spelling

Facts uses one canonical dot-notation path for each supported fact concept across supported release targets, even when Ruby Facter exposes different spellings for the same concept on different platforms. The structured tree is the only fact surface (ADR-0007), so Facts does not emit normalized names plus compatibility duplicates; the schema-owned spelling is the output contract.

The first application is disk serial numbers: the canonical key is `disks.*.serial_number`, matching the existing `dmi.*.serial_number` spelling. `disks.*.serial` is not kept as a compatibility alias.

Existing flat Ruby-compatible core facts such as `kernelrelease`, `kernelversion`, and `kernelmajversion` are out of scope for this ADR's first application. Replacing those with a structured Facts-native subtree requires a separate OpenSpec change.

## Considered Options

- **Preserve upstream spelling per platform** — rejected: it makes the schema less useful as a cross-platform contract and forces consumers to learn platform-specific aliases for the same concept.
- **Emit both upstream and normalized spellings** — rejected: it recreates the alias layer ADR-0007 removed.
- **Schema-owned canonical spelling (chosen)** — accepted while the project is unreleased, so inconsistent names can be corrected before they become real compatibility debt.
