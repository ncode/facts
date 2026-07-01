package engine

import (
	"path/filepath"
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
	for _, chunk := range plutilArgChunks(paths) {
		out := run("plutil", append([]string{"-p"}, chunk...)...)
		parsePlutilBlocks(out, func(fields map[string]string) {
			name, version := fields["PackageIdentifier"], fields["PackageVersion"]
			if name == "" || version == "" {
				return
			}
			records = append(records, packageRecord(name, version))
		})
	}
	sortPackages(records)
	return records
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
// Both /Applications and /System/Applications are scanned, and every matched
// Info.plist is read by a single `plutil -p`.
//
// plutil -p prints one dict block per file, in argument order, and does NOT
// echo filenames. osHost.run (cmd.Output) returns "" unless plutil exits 0, so
// a non-empty result means every file parsed successfully; the block stream
// therefore aligns one-to-one with the glob order, letting the .app path be
// recovered positionally. Only the top-level (depth-1) keys of each block are
// consulted — Info.plists nest arbitrarily deep.
func appsPackages(glob pathGlobber, run commandRunner) []any {
	var paths []string
	for _, pattern := range []string{
		"/Applications/*/Contents/Info.plist",
		"/System/Applications/*/Contents/Info.plist",
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
	// Chunk so a large app set never overflows argv. plutil is all-or-nothing
	// per invocation (cmd.Output is "" on any parse failure), so blocks align
	// one-to-one with the chunk's paths; a failed chunk yields no blocks and
	// cannot desynchronise a later chunk (the path index is chunk-local).
	for _, chunk := range plutilArgChunks(paths) {
		out := run("plutil", append([]string{"-p"}, chunk...)...)
		index := 0
		parsePlutilBlocks(out, func(fields map[string]string) {
			if index >= len(chunk) {
				return
			}
			appPath := appBundlePath(chunk[index])
			index++
			name := firstNonEmpty(fields["CFBundleName"], fields["CFBundleDisplayName"], appBundleName(appPath))
			version := firstNonEmpty(fields["CFBundleShortVersionString"], fields["CFBundleVersion"])
			if name == "" || version == "" {
				return
			}
			records = append(records, packageRecord(name, version,
				"bundle_id", fields["CFBundleIdentifier"],
				"path", appPath))
		})
	}
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
			records = appendBrewRecords(records, formulae, "formula")
		}
		// Probe Caskroom independently of Cellar: a cask-only install has an
		// empty Cellar but real casks to report.
		if casks, err := glob(prefix + "/Caskroom/*/*"); err == nil {
			records = appendBrewRecords(records, casks, "cask")
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
// matches — unlike the shell — so dot-prefixed leaves are skipped.
func appendBrewRecords(records []any, paths []string, kind string) []any {
	for _, path := range paths {
		version := filepath.Base(path)
		name := filepath.Base(filepath.Dir(path))
		if name == "" || version == "" || strings.HasPrefix(version, ".") {
			continue
		}
		records = append(records, packageRecord(name, version, "type", kind))
	}
	return records
}

// parsePlutilBlocks splits the text of `plutil -p file...` into one dict per
// input file and invokes fn with that dict's top-level (depth-1) scalar keys,
// in file order. plutil prints the outer dict's braces alone on a line and
// opens nested containers with a "key" => { or "key" => [ suffix; depth is
// tracked structurally rather than by column, so multi-line string values (an
// unindented copyright continuation, say) cannot desynchronise block or key
// boundaries.
func parsePlutilBlocks(out string, fn func(fields map[string]string)) {
	var fields map[string]string
	depth := 0
	for line := range strings.Lines(out) {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "{":
			if depth == 0 {
				fields = map[string]string{}
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
			if depth == 1 && !opensContainer {
				if key, value, ok := plutilKeyValue(trimmed); ok {
					fields[key] = value
				}
			}
			if opensContainer {
				depth++
			}
		}
	}
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
	return filepath.Dir(filepath.Dir(infoPlist))
}

// appBundleName derives a display name from a bundle path when the plist carries
// no CFBundleName/CFBundleDisplayName (.../Xxx.app -> Xxx).
func appBundleName(appPath string) string {
	return strings.TrimSuffix(filepath.Base(appPath), ".app")
}
