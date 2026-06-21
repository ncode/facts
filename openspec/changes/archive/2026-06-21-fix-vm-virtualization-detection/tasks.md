## 1. Implementation

- [x] 1.1 Add fixture-backed detector coverage for QEMU/KVM VM signals.
- [x] 1.2 Update supported-platform virtualization detection.
- [x] 1.3 Omit empty optional top-level networking strings found by full-tree
      native validation.
- [x] 1.4 Omit empty mountpoint entries found by full-tree native validation.

## 2. Documentation

- [x] 2.1 Update CHANGELOG and supported-platform facts spec.

## 3. Verification

- [x] 3.1 Run focused virtualization tests.
- [x] 3.2 Re-run native full-tree content validation.
- [x] 3.3 Run `go test ./...` and `go vet ./...`.
