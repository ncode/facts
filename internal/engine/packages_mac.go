package engine

import (
	"path"
	"strings"
)

// receiptsPackages resolves the "receipts" source (ADR-0014): the macOS
// installer receipt database under /var/db/receipts. Each *.plist is a binary
// property list, so a single `plutil -p` reads all of them in one spawn and
// prints one dict block per file. A block yields a record only when it carries
// both PackageIdentifier (name) and PackageVersion (version).
func receiptsPackages(glob pathGlobber, run commandRunner) []any {
	paths, err := glob("/var/db/receipts/*.plist")
	if err != nil || len(paths) == 0 {
		return nil
	}
	var records []any
	runPlutilChunks(run, paths, func(_ []string, _ int, fields map[string]string) {
		name, version := fields["PackageIdentifier"], fields["PackageVersion"]
		if name == "" || version == "" {
			return
		}
		records = append(records, packageRecord(name, version))
	})
	sortPackages(records)
	return records
}

// runPlutilChunks batches plist paths through `plutil -p` (argv-chunked by
// plutilArgChunks) and invokes perBlock for every parsed block with the
// successful invocation's own path list and the block's invocation-local
// index. plutil prints blocks in argument order and never echoes filenames, so
// this positional pairing is the only path<->block link — and it only holds
// per invocation, which is why perBlock receives the invocation's paths rather
// than a global offset.
//
// plutil is all-or-nothing per invocation: it exits non-zero if ANY file fails,
// and the engine's run() returns "" on non-zero exit. On success `plutil -p`
// always prints one non-empty block per input file, so empty output on a
// non-empty chunk unambiguously means failure. To keep one corrupt file from
// dropping every good file in its chunk, a failed multi-path chunk is bisected
// and each half retried (O(log n) extra spawns per bad file); a failed
// single-path chunk is the corrupt/unreadable file itself and is skipped.
func runPlutilChunks(run commandRunner, paths []string, perBlock func(chunkPaths []string, blockIndex int, fields map[string]string)) {
	for _, chunk := range plutilArgChunks(paths) {
		runPlutilChunk(run, chunk, perBlock)
	}
}

func runPlutilChunk(run commandRunner, chunk []string, perBlock func([]string, int, map[string]string)) {
	out := run("plutil", append([]string{"-p"}, chunk...)...)
	if out == "" {
		if len(chunk) > 1 {
			mid := len(chunk) / 2
			runPlutilChunk(run, chunk[:mid], perBlock)
			runPlutilChunk(run, chunk[mid:], perBlock)
		}
		return
	}
	index := 0
	parsePlutilBlocks(out, func(fields map[string]string) {
		perBlock(chunk, index, fields)
		index++
	})
}

// plutilArgChunks splits plist paths into batches whose joined argv length stays
// well under ARG_MAX, so one `plutil -p <paths>` can never overflow the command
// line and silently drop the whole source on a host with a large receipt/app set.
func plutilArgChunks(paths []string) [][]string {
	const maxBytes = 120 * 1024 // conservative; macOS ARG_MAX is 1 MiB
	var chunks [][]string
	var chunk []string
	size := 0
	for _, p := range paths {
		if len(chunk) > 0 && size+len(p)+1 > maxBytes {
			chunks = append(chunks, chunk)
			chunk, size = nil, 0
		}
		chunk = append(chunk, p)
		size += len(p) + 1
	}
	if len(chunk) > 0 {
		chunks = append(chunks, chunk)
	}
	return chunks
}

// appsPackages resolves the "apps" source: installed application bundles. It is
// a secondary view of installed software and is never merged into "receipts".
// /Applications and /System/Applications are scanned along with their
// Utilities subfolders (glob "*" is single-level, so Utilities needs its own
// pattern), and every matched Info.plist is read by a single `plutil -p`. The
// patterns match only *.app bundles — nothing else has that Contents layout.
//
// plutil -p prints one block per file, in argument order, and does NOT echo
// filenames. osHost.run (cmd.Output) returns "" unless plutil exits 0, so a
// non-empty result means every file in that invocation parsed successfully;
// the block stream therefore aligns one-to-one with the invocation's argument
// order, letting the .app path be recovered positionally. Only the top-level
// (depth-1) keys of each block are consulted — Info.plists nest arbitrarily
// deep.
func appsPackages(glob pathGlobber, run commandRunner) []any {
	var paths []string
	for _, pattern := range []string{
		"/Applications/*.app/Contents/Info.plist",
		"/Applications/Utilities/*.app/Contents/Info.plist",
		"/System/Applications/*.app/Contents/Info.plist",
		"/System/Applications/Utilities/*.app/Contents/Info.plist",
	} {
		matches, err := glob(pattern)
		if err != nil {
			continue
		}
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		return nil
	}
	var records []any
	// runPlutilChunks keeps argv under ARG_MAX and bisects around corrupt
	// plists; blocks align one-to-one with each successful invocation's paths,
	// so the block index is always invocation-local and cannot desynchronise a
	// later chunk.
	runPlutilChunks(run, paths, func(chunkPaths []string, blockIndex int, fields map[string]string) {
		if blockIndex >= len(chunkPaths) {
			return
		}
		appPath := appBundlePath(chunkPaths[blockIndex])
		name := firstNonEmpty(fields["CFBundleName"], fields["CFBundleDisplayName"], appBundleName(appPath))
		version := firstNonEmpty(fields["CFBundleShortVersionString"], fields["CFBundleVersion"])
		if name == "" || version == "" {
			return
		}
		records = append(records, packageRecord(name, version,
			"bundle_id", fields["CFBundleIdentifier"],
			"path", appPath))
	})
	sortPackages(records)
	return records
}

// homebrewPackages resolves the "homebrew" source, auto-detected by the presence
// of a Cellar under a known prefix (/opt/homebrew on Apple Silicon, /usr/local on
// Intel). Formulae live at <prefix>/Cellar/<name>/<version>; casks at
// <prefix>/Caskroom/<name>/<version>. The source is omitted entirely when no
// Cellar exists. Only glob is needed — the name/version pair is the directory
// layout itself.
func homebrewPackages(glob pathGlobber) []any {
	var records []any
	for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
		if formulae, err := glob(prefix + "/Cellar/*/*"); err == nil {
			records = appendBrewRecords(records, formulae, "formula", prefix)
		}
		// Probe Caskroom independently of Cellar: a cask-only install has an
		// empty Cellar but real casks to report.
		if casks, err := glob(prefix + "/Caskroom/*/*"); err == nil {
			records = appendBrewRecords(records, casks, "cask", prefix)
		}
	}
	if len(records) == 0 {
		return nil
	}
	sortPackages(records)
	return records
}

// appendBrewRecords turns <name>/<version> leaf paths into records. Homebrew
// keeps a dotfile sidecar (Caskroom/<name>/.metadata) that filepath.Glob's "*"
// matches — unlike the shell — so dot-prefixed leaves are skipped. The prefix
// is recorded as an identity field: an Apple Silicon (/opt/homebrew) and a
// Rosetta Intel (/usr/local) brew can carry the same formula@version, and
// without it the two installs would be byte-identical duplicates.
func appendBrewRecords(records []any, paths []string, kind, prefix string) []any {
	for _, p := range paths {
		version := path.Base(p)
		name := path.Base(path.Dir(p))
		if name == "" || version == "" || strings.HasPrefix(version, ".") {
			continue
		}
		records = append(records, packageRecord(name, version, "type", kind, "prefix", prefix))
	}
	return records
}

// parsePlutilBlocks splits the text of `plutil -p file...` into one block per
// input file and invokes fn with that block's top-level (depth-1) scalar keys,
// in file order. plutil prints the outer container's delimiters alone on a
// line and opens nested containers with a "key" => { or "key" => [ suffix;
// depth is tracked structurally rather than by column, so multi-line string
// values (an unindented copyright continuation, say) cannot desynchronise
// block or key boundaries.
//
// Two hostile shapes are handled explicitly:
//   - A multi-line string value can contain lines that are exactly "{", "}",
//     "[", or "]". When a value opens a quote it does not close on the same
//     line, the parser enters in-string mode and suspends ALL structural
//     interpretation until a line ends with an unescaped closing quote.
//   - A plist whose root is an array prints a bare "[" ... "]" block. It
//     carries no dict keys, but it still opens a block (fields stay empty) so
//     fn fires at its close and positional path pairing stays in step.
func parsePlutilBlocks(out string, fn func(fields map[string]string)) {
	var fields map[string]string
	depth := 0
	rootIsArray := false
	inString := false
	for line := range strings.Lines(out) {
		if inString {
			if endsWithUnescapedQuote(strings.TrimRight(line, "\r\n")) {
				inString = false
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "{":
			if depth == 0 {
				fields = map[string]string{}
				rootIsArray = false
			}
			depth++
		case trimmed == "[":
			// Array-rooted plist: open a keyless block so pairing advances.
			if depth == 0 {
				fields = map[string]string{}
				rootIsArray = true
			}
			depth++
		case trimmed == "}" || trimmed == "]":
			if depth > 0 {
				depth--
			}
			if depth == 0 && fields != nil {
				fn(fields)
				fields = nil
			}
		default:
			// A depth-1 scalar is a real top-level key; a "key" => { or
			// "key" => [ line opens a nested container we descend past.
			opensContainer := strings.HasSuffix(trimmed, " => {") || strings.HasSuffix(trimmed, " => [")
			if opensContainer {
				depth++
				continue
			}
			if depth == 1 && !rootIsArray {
				if key, value, ok := plutilKeyValue(trimmed); ok {
					fields[key] = value
				}
			}
			// A string value whose closing quote is not on this line spans
			// further physical lines; suspend structure until it closes.
			if _, rawValue, found := strings.Cut(trimmed, " => "); found && stringRemainsOpen(rawValue) {
				inString = true
			}
		}
	}
}

// stringRemainsOpen reports whether a plutil value beginning with a quote
// continues onto later physical lines (its closing quote is not on this line).
func stringRemainsOpen(rawValue string) bool {
	if !strings.HasPrefix(rawValue, `"`) {
		return false
	}
	return !endsWithUnescapedQuote(rawValue[1:])
}

// endsWithUnescapedQuote reports whether s terminates with a `"` that is not
// escaped — an odd run of backslashes before the quote escapes it.
func endsWithUnescapedQuote(s string) bool {
	if !strings.HasSuffix(s, `"`) {
		return false
	}
	backslashes := 0
	for i := len(s) - 2; i >= 0 && s[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 0
}

// plutilKeyValue parses a `"key" => value` line, unquoting both sides. It
// reports ok=false for lines without the " => " separator (blank-line padding,
// wrapped multi-line string continuations).
func plutilKeyValue(line string) (key, value string, ok bool) {
	rawKey, rawValue, found := strings.Cut(line, " => ")
	if !found {
		return "", "", false
	}
	return unquotePlutil(rawKey), unquotePlutil(rawValue), true
}

func unquotePlutil(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	if strings.HasPrefix(s, `"`) {
		// A string value whose closing quote is on a later physical line (a
		// multi-line plutil value); keep this first line without the opener.
		return s[1:]
	}
	return s
}

// appBundlePath recovers the .app bundle path from its Info.plist path
// (.../Xxx.app/Contents/Info.plist -> .../Xxx.app).
func appBundlePath(infoPlist string) string {
	return path.Dir(path.Dir(infoPlist))
}

// appBundleName derives a display name from a bundle path when the plist carries
// no CFBundleName/CFBundleDisplayName (.../Xxx.app -> Xxx).
func appBundleName(appPath string) string {
	return strings.TrimSuffix(path.Base(appPath), ".app")
}
