## 1. Disk Serial Spelling

- [x] 1.1 Add a failing Linux disk test expecting `serial_number` instead of `serial`.
- [x] 1.2 Rename the Linux emitted disk serial key to `serial_number` while still reading `lsblk`'s `serial` column.
- [x] 1.3 Remove `disks.*.serial` from the schema and add Linux to `disks.*.serial_number`.
- [x] 1.4 Regenerate `docs/supported-facts/`.
- [x] 1.5 Record the output-contract decision in `CONTEXT.md` and ADR-0011.
- [x] 1.6 Run focused tests, schema/docs checks, `go test ./...`, `go vet ./...`, and OpenSpec validation.
