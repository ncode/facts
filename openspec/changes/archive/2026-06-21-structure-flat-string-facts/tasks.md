## 1. Tests First

- [x] 1.1 Add tests that kernel output is a structured map and old flat kernel facts are absent.
- [x] 1.2 Add tests that `filesystems` is an ordered array of filesystem names, not a comma-separated string.
- [x] 1.3 Add tests that `path` is an ordered array split with the host platform path-list separator and empty entries omitted.
- [x] 1.4 Add tests that ZFS/Zpool feature numbers and feature flags are arrays under `zfs.*` and `zpool.*`.

## 2. Implementation

- [x] 2.1 Replace flat kernel fact assembly with the structured `kernel` map.
- [x] 2.2 Return filesystem collections as arrays from Linux and Darwin filesystem probes.
- [x] 2.3 Return `path` as an array of entries instead of the raw environment string.
- [x] 2.4 Replace flat ZFS/Zpool fact names with structured `zfs` and `zpool` maps.
- [x] 2.5 Remove old flat names from core output; do not add aliases.

## 3. Schema and Docs

- [x] 3.1 Update `docs/schema/facts.yaml` with `kernel.*`, array-typed `filesystems`, array-typed `path`, `zfs.*`, and `zpool.*`.
- [x] 3.2 Remove `kernel`, `kernelmajversion`, `kernelrelease`, `kernelversion`, `zfs_featurenumbers`, `zfs_version`, `zpool_featurenumbers`, `zpool_featureflags`, and `zpool_version` from the schema.
- [x] 3.3 Regenerate `docs/supported-facts/` and update examples to show the new shapes.
- [x] 3.4 Update README/man page/changelog references that mention the old flat names.

## 4. Verification

- [x] 4.1 Run focused resolver tests for kernel, filesystems, path, ZFS, and Zpool.
- [x] 4.2 Run `go test ./...`.
- [x] 4.3 Run `go vet ./...`.
- [x] 4.4 Run `openspec validate structure-flat-string-facts --strict`.
