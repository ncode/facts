## Decisions

- `kernel` becomes a map. `kernel.name` replaces the old kernel-name string. `kernel.release.full` replaces `kernelrelease`. `kernel.version.full` replaces `kernelversion`.
- `kernelmajversion` is not preserved as a separate leaf. It is represented by `kernel.release.major` and `kernel.release.minor`; callers that need the old two-component display can join those two values.
- `filesystems` remains the top-level fact name but changes type from `string` to `array`.
- `path` remains the top-level fact name but changes type from `string` to `array`, preserving path entry order and omitting empty entries.
- ZFS/Zpool move under `zfs.*` and `zpool.*`; feature numbers and flags are arrays of strings because the source data is version/flag text, not arithmetic input.
- Existing flat names are removed rather than duplicated, following ADR-0007 and ADR-0011.

## Implementation Notes

- Build parser tests first for comma-separated ZFS/Zpool values, `/proc/filesystems`, macOS filesystem discovery, and PATH splitting on POSIX and Windows separators.
- Update schema conformance expectations together with resolver output so generated docs and live gates fail on drift.
- Keep platform support unchanged: do not add ZFS facts to OpenBSD, and keep ZFS/Zpool conditional on usable tool output.
