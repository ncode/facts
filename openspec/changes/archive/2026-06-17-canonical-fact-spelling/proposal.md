## Why

`docs/schema/facts.yaml` should be a predictable cross-platform contract, not a list of platform aliases for the same concept. Linux currently documents and emits `disks.*.serial`, while FreeBSD documents and emits `disks.*.serial_number`; both mean the same disk serial number.

## What Changes

- Adopt schema-owned canonical fact spelling for supported facts.
- Rename Linux disk serial output from `disks.*.serial` to `disks.*.serial_number`.
- Remove `disks.*.serial` from the schema and generated supported-facts pages.
- Do not emit compatibility duplicates.

## Impact

- **Behavior**: Linux disk serials move to `disks.*.serial_number`.
- **Docs/schema**: `docs/schema/facts.yaml` and generated supported-facts pages use the canonical key.
- **Compatibility**: Breaking only before release; no alias layer is added.
