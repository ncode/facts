## Why

A security review found several framework boundaries that trusted operator input too far: configured cache group names could escape the cache directory, external fact files and executable outputs were unbounded, YAML/HOCON keys were rendered without escaping, built-in probes inherited an untrusted `PATH`, and filesystem byte math could overflow before formatting.

## What Changes

- Reject unsafe cache group names before reading, writing, or deleting cache files.
- Bound static external fact files, executable stdout, and executable stderr to a fixed byte limit.
- Escape unsafe YAML and HOCON keys instead of rendering raw key text.
- Resolve built-in host probe commands from a fixed system search path and pass a sanitized `PATH`.
- Clamp statfs byte multiplication to the largest representable `int`.
- Add `govulncheck` to the tracked Go tool set and CI checks.

## Impact

- Oversized external fact sources now fail discovery instead of consuming unbounded memory.
- Unsafe cache TTL/group configuration is ignored with a warning rather than touching paths outside the cache directory.
- Machine-format output remains parseable when fact keys contain control characters or other unsafe syntax.
- Built-in probes no longer depend on caller-provided `PATH` entries.
