## 1. Schema Contract

- [x] 1.1 Add an internal schema contract package for loading `docs/schema/facts.yaml`, validating entries, flattening fact trees, and matching schema paths.
- [x] 1.2 Move schema platform vocabulary and conditional/open-subtree validation into the shared schema contract.
- [x] 1.3 Update `schema_test.go` to use the shared schema contract instead of local schema parsing and matching logic.
- [x] 1.4 Update `tools/supportedfacts` to use the same parsed schema entries and platform vocabulary.
- [x] 1.5 Add tests for exact paths, one-segment `*` matches, documented dynamic children, unknown dynamic children, and explicit open subtrees.
- [x] 1.6 Regenerate supported-facts docs and verify generated docs are current.

## 2. CLI Option Contract

- [x] 2.1 Add shared CLI option metadata for canonical names, aliases, arity, repeatability, task flags, conflicts, and documentation rows.
- [x] 2.2 Update CLI validation and argument helpers to read option metadata from the shared contract.
- [x] 2.3 Update app helper paths that inspect raw args, including config path discovery, external dir discovery, and group-listing value handling.
- [x] 2.4 Add `--force-dot-resolution` to help/man surfaces while it remains accepted.
- [x] 2.5 Add drift tests proving accepted non-hidden options appear in help/man output and the installed man page.

## 3. Platform Target Profile

- [ ] 3.1 Add an internal platform target profile table keyed by GOOS with ID, label, support tier, schema visibility, compile targets, distribution targets, gate metadata, and coarse capability policy.
- [ ] 3.2 Replace duplicated schema/docs platform vocabulary with the target profile source.
- [ ] 3.3 Replace low-risk OS identity helpers with target profile data while preserving current `os`, `kernel`, and release facts.
- [ ] 3.4 Wire low-risk capability gates for filesystems, ZFS, and Plan 9 intentionally absent release facts.
- [ ] 3.5 Add tests for target IDs, target set separation, excluded `solaris`/`aix`, OS identity, and schema/docs vocabulary alignment.

## 4. Session Host Probe Seam

- [ ] 4.1 Extend the Session host seam only for direct host/runtime calls needed by disk, partition, and mountpoint resolution.
- [ ] 4.2 Route disk, partition, and mountpoint resolution through injectable host seam operations for platform identity, directory reads, globbing, command output, file reads, and stat data.
- [ ] 4.3 Preserve existing command timeout, context cancellation, logging, and sanitized environment behavior.
- [ ] 4.4 Add fake-host tests proving disk, partition, and mountpoint facts do not read the developer host directly.
- [ ] 4.5 Add regression tests proving canonical disk, partition, and mountpoint output is unchanged.

## 5. Verification

- [ ] 5.1 Run `gofmt -w` on edited Go files.
- [ ] 5.2 Run `go test ./...`.
- [ ] 5.3 Run `go vet ./...`.
- [ ] 5.4 Run native facts-lab gates only for slices that change platform probe behavior or target policy.
- [ ] 5.5 Confirm the OpenSpec status reports this change apply-ready.
