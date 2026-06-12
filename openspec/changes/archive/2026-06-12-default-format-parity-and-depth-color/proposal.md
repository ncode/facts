# Fix default text format parity and add depth-colored keys under --color

## Why

The 2026-06-12 side-by-side run against Ruby Facter 4.10.0 showed the default text format diverging on every nested line (~799 diff lines): missing commas between entries, unquoted nested strings in full output (while query mode quotes them — two inconsistent code paths), and arrays flattened to one line. Ruby's formatter is pretty-printed JSON with rewritten keys, so the fix is to build ours the same way. While rebuilding the formatter, `--color` gains a Facts-native feature: keys colored by nesting depth, making deep trees scannable.

## What Changes

- Rebuild `FormatLegacy*` on JSON pretty-printing with Ruby's exact transform pipeline (verified against the gem source `legacy_fact_formatter.rb`): 2-space-indent JSON → `": ` becomes `" => ` → key quotes stripped → doubled backslashes collapsed; for full output additionally strip the enclosing braces, de-indent one level, drop top-level trailing commas and unescaped quotes, and expand literal `\n` in values; multi-query output unquotes top-level string values and renders nil as empty; single-query output keeps enclosing braces and prints plain strings raw. One code path for all three modes.
- Nested strings become double-quoted, map entries and array elements get trailing commas, arrays render multi-line — matching Ruby byte-for-byte.
- Omit the `memory.swap` subtree when there is no swap (zero total), matching Ruby and the not-applicable rule — the comparison surfaced it as the last placeholder.
- **New feature**: keys in the default text format are colorized by nesting depth (cycling ANSI palette); values are untouched. Color is on by default when stdout is a terminal and off otherwise (so piped output stays clean); `--color` forces it on, `--no-color` disables it, and machine formats (`--json`, `--yaml`, `--hocon`) are never colorized. Diagnostic coloring follows the same per-stream rule on stderr.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `go-port-framework-parity`: adds a requirement for depth-colored keys in the default format under `--color` (a documented Facts extension; Ruby's `--color` only affects diagnostics). The formatter parity fix itself implements the existing "Query and formatter behavior" scenario (string quoting, scalar formatting) and needs no spec change.

## Impact

- **Code**: `internal/facter/formatter.go` (`FormatLegacy`, `FormatLegacyWithDottedFacts`, and the query-mode path collapse into the JSON-based pipeline); the darwin/freebsd memory resolver for swap omission; `internal/app/app.go` threads the color flag into the formatter.
- **Tests**: formatter tests re-pinned to Ruby-verified strings; app/engine contract tests with exact legacy-format expectations updated; new color tests (depth palette, off by default, machine formats unaffected); swap omission tests.
- **Docs**: `--color` help/man text gains the depth-coloring note; CHANGELOG entry (format fix is breaking only versus our own broken output, converging on Ruby's).
- **Behavioral**: default and query text output match Ruby Facter modulo documented deviations; `--color` output gains colored keys.
