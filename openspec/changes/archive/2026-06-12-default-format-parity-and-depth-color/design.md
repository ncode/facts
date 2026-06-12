# Design: Default text format parity and depth-colored keys

## Context

Ruby Facter 4.10.0's `LegacyFactFormatter` (gem source `lib/facter/framework/formatters/legacy_fact_formatter.rb`) produces the default text format by transforming `JSON.pretty_generate` output. The Go port hand-rolls the tree walk in `internal/facter/formatter.go` and diverges: no commas, no nested-string quotes in full output (query mode quotes them — a second code path), single-line arrays. The 2026-06-12 host comparison showed ~799 differing lines. The same run surfaced `memory.swap` rendered as zeros where Ruby omits the subtree.

## Goals / Non-Goals

**Goals:**
- Byte-for-byte parity with Ruby's pipeline for all three modes (no query, single query, multiple queries), pinned by tests derived from captured Ruby output.
- One formatting code path instead of two.
- Depth-colored keys by default on terminals, keys only, default format only; `--color` forces, `--no-color` disables, detection per stream (stdout for facts, stderr for diagnostics) so piped output is always clean.
- `memory.swap` absent when there is no swap.

**Non-Goals:**
- No changes to `--json`/`--yaml`/`--hocon` output (byte-identical regardless of color settings).
- No `NO_COLOR`/`TERM` environment handling — flags plus terminal detection only (revisit on demand).
- No renaming of `FormatLegacy*` identifiers ("legacy" names Ruby's format, which survives as our default).

## Decisions

**1. Replicate Ruby's pipeline literally, quirks included.**
Marshal each value as 2-space-indented JSON (Go: `json.Encoder` with `SetEscapeHTML(false)`, map keys sorted — matching Ruby's sorted collection), then apply Ruby's transforms in order: `": ` → `" => `; per-line greedy key-unquote (`"(.*)" =>` → `$1 =>`); `\\\\` → `\`. Full output additionally: strip enclosing braces, drop empty lines, de-indent two spaces, `^},` → `}`, then on non-indented lines strip trailing commas and unescaped quotes (keeping `\"` → `"`), and expand literal `\n` to newlines. Multi-query: nil → empty string, then unquote top-level string values. Single query: plain strings print raw; structures keep their enclosing braces; a fully quoted single-line result is unquoted. Replicating the quirks (greedy key regex, value-content edge cases) is deliberate — divergence is what we are fixing.

**2. Color is applied during key rewriting, not as a post-pass.**
The key-unquote step knows each key's indentation, and depth = indent/2. When color is enabled the key is wrapped in the palette color for its depth (cycling: cyan, yellow, green, magenta, blue). Values, punctuation, and braces stay uncolored. A post-hoc regex pass over finished output would have to re-derive structure and would break on values containing `=>`.

**2b. Default-on with terminal detection, resolved per stream.**
`resolveColor(force, disable, w)`: `--no-color` always wins, `--color` always forces, otherwise color is on exactly when the writer is an `*os.File` whose mode has `ModeCharDevice` (dependency-free TTY check, works for the Windows console). Fact output keys follow stdout's answer; diagnostic coloring follows stderr's. Tests pass `bytes.Buffer` writers, which are never terminals, so all piped/captured output — including CI gates and the acceptance suite — stays escape-free without special-casing.

**3. Swap omission lives in the resolver.**
Same pattern as `omit-not-applicable-facts`: the memory resolver omits the `swap` subtree when total bytes are zero (Ruby omits it on hosts without swap). Not a formatter concern.

**4. Test strategy: pin captured Ruby output.**
Formatter tests assert exact strings taken from the Ruby comparison captures (nested quoting, commas, multi-line arrays, top-level unquoting, `\n` expansion, Windows backslash collapse via fixture). Color tests assert palette-by-depth on a fixed tree, no-color default, and `--json --color` byte-equality.

## Risks / Trade-offs

- [Existing tests pin the old broken format] → they are wrong by the already-synced spec ("string quoting", "scalar formatting" must match Ruby); re-pin them to Ruby-verified strings in the same commit.
- [Ruby's regex quirks mangle exotic values (e.g. a value containing `": `)] → identical mangling on both sides is parity; pin one such case in a test as documentation.
- [Color codes leak into greps/pipes] → opt-in flag only; documented; machine formats proven untouched by byte-equality test.
- [Go JSON escaping differences (HTML escaping, float rendering)] → disable HTML escaping; floats marshal identically for fact values seen in practice; pin `1.35`-style load averages in tests.

## Migration Plan

Single PR: formatter rebuild + color + swap omission + re-pinned tests + help/man/CHANGELOG. Verify with the full suite and a fresh side-by-side against the installed Ruby Facter (diff clean modulo volatile values and documented deviations). Rollback: revert.

## Open Questions

None — palette order is an aesthetic default; trivial to change later.
