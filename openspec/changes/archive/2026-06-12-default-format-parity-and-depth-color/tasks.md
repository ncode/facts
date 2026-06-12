# Tasks: Default text format parity and depth-colored keys

## 1. Formatter parity

- [x] 1.1 Rebuild the default-format pipeline in `internal/facter/formatter.go` on indented JSON plus Ruby's transform order (key rewrite, key unquote, backslash collapse); collapse the full-output and query-mode paths into it
- [x] 1.2 Implement the full-output post-passes: strip enclosing braces, drop empty lines, de-indent, top-level comma/quote stripping, literal `\n` expansion
- [x] 1.3 Implement multi-query (nil → empty, top-level string unquote) and single-query (raw plain strings, kept braces, whole-result unquote) behaviors
- [x] 1.4 Re-pin formatter and app/engine contract tests to Ruby-verified output strings (nested quoting, commas, multi-line arrays, top-level unquoting, `\n` expansion, backslash fixture)

## 2. Swap omission

- [x] 2.1 Omit `memory.swap` when total bytes are zero (resolver level); keep populated swap intact; tests for both shapes

## 3. Depth-colored keys

- [x] 3.1 Apply per-depth ANSI palette to keys during key rewriting when color is enabled; thread `--color`/`--no-color` from `internal/app` into the default formatter only
- [x] 3.2 Tests: palette by depth on a fixed tree; no ANSI without `--color` and with `--no-color`; `--json`/`--yaml`/`--hocon` byte-identical with and without `--color`
- [x] 3.3 Update `--color` help text and man page (depth-colored keys note); CHANGELOG entry for the format fix and the feature
- [x] 3.4 Color on by default when the stream is a terminal: `resolveColor` (`--no-color` wins, `--color` forces, else `*os.File` + `ModeCharDevice`), resolved separately for stdout (fact keys) and stderr (diagnostics); piped output stays clean; PTY smoke verifies terminal default; help/man/CHANGELOG updated

## 4. Verification

- [x] 4.1 `go test ./...`, `go test -race ./...`, vet/gofmt clean
- [x] 4.2 Side-by-side rerun against Ruby Facter 4.10.0: default and query text outputs diff clean modulo volatile values and documented deviations; `--json` output unchanged from before this change
- [x] 4.3 Platform CI gates green on the final commit
