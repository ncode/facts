## Why

Several supported facts still carry Ruby-era flat names or encode collections as delimiter-separated strings. That makes `docs/schema/facts.yaml` less useful as a cross-platform contract: consumers have to know that `kernelrelease` belongs with `kernel`, that `filesystems` is a comma-separated list, and that `path` is a platform-delimited list hiding inside one string.

ADR-0011 makes the schema the owner of canonical fact spelling. This change applies that rule to the remaining obvious flat/string-list facts.

## What Changes

- Replace `kernel`, `kernelmajversion`, `kernelrelease`, and `kernelversion` with a structured `kernel.*` subtree.
- Change `filesystems` from a comma-separated string to an array of strings.
- Change `path` from a raw PATH string to an array of path entries, split with the platform path-list separator.
- Replace ZFS/Zpool flat facts with structured `zfs.*` and `zpool.*` facts.
- Update `docs/schema/facts.yaml` and generated supported-facts pages to remove the old flat/string-list names and document the new shapes.
- Do not emit compatibility duplicates.

## Target Shape

```json
{
  "filesystems": ["apfs", "autofs", "devfs"],
  "kernel": {
    "name": "Darwin",
    "release": {
      "full": "25.5.0",
      "major": "25",
      "minor": "5",
      "patch": "0"
    },
    "version": {
      "full": "25.5.0"
    }
  },
  "path": ["/usr/local/bin", "/usr/bin", "/bin"],
  "zfs": {
    "feature_numbers": ["1", "2", "3", "4", "5", "6"],
    "version": "6"
  },
  "zpool": {
    "feature_flags": ["async_destroy", "empty_bpobj", "lz4_compress"],
    "feature_numbers": ["1", "2", "3"],
    "version": "5000"
  }
}
```

## Impact

- **Behavior**: output shape changes for the listed facts on supported platforms.
- **Schema/docs**: `docs/schema/facts.yaml` removes flat facts and delimiter-encoded string lists, then regenerated supported-facts pages show arrays and nested paths.
- **Compatibility**: breaking before release only; no alias layer is added.
- **Out of scope**: `virtual`/`is_virtual` and other scalar facts that are not delimiter-encoded collections or obvious flat groups.
