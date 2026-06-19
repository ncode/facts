## 1. Tests First

- [x] 1.1 Add a cache traversal regression test for unsafe fact group names.
- [x] 1.2 Add static and executable external fact byte-limit regression tests.
- [x] 1.3 Add YAML and HOCON unsafe-key escaping regression tests.
- [x] 1.4 Add a built-in command environment sanitization regression test.
- [x] 1.5 Add a statfs overflow-clamping regression test.

## 2. Implementation

- [x] 2.1 Validate cache group names before cache read, write, freshness, and delete paths.
- [x] 2.2 Bound external fact file reads and executable stdout/stderr collection.
- [x] 2.3 Quote unsafe YAML and HOCON output keys.
- [x] 2.4 Resolve built-in probe commands from a fixed system path and sanitize `PATH`.
- [x] 2.5 Clamp statfs byte math before converting to `int`.
- [x] 2.6 Add `govulncheck` as a tracked Go tool and CI step.

## 3. Documentation

- [x] 3.1 Record the security-boundary behavior in OpenSpec.
- [x] 3.2 Update `CHANGELOG.md`.

## 4. Verification

- [x] 4.1 Run focused regression tests.
- [x] 4.2 Run `go test ./...`.
- [x] 4.3 Run `go vet ./...`.
- [x] 4.4 Run `go test -race . ./internal/engine ./internal/app`.
- [x] 4.5 Run `go tool govulncheck ./...`.
- [x] 4.6 Run `openspec validate harden-security-boundaries --strict`.
