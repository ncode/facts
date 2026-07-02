package engine

import (
	"reflect"
	"strings"
	"testing"
)

// globStub returns canned matches per pattern and records the plutil argv the
// reader would spawn, so tests exercise the real glob->plutil->parse pipeline
// with injected probes.
func globStub(matches map[string][]string) pathGlobber {
	return func(pattern string) ([]string, error) {
		return matches[pattern], nil
	}
}

// runStub returns fixed output and captures the file arguments (argv after
// "-p"), letting a test assert the plutil block order matches the glob order.
func runStub(output string, captured *[]string) commandRunner {
	return func(_ string, args ...string) string {
		if len(args) > 0 && captured != nil {
			*captured = append([]string(nil), args[1:]...)
		}
		return output
	}
}

// receiptsFixture is verbatim `plutil -p /var/db/receipts/*.plist` output from a
// macOS host, trimmed to three real blocks plus a synthetic block missing
// PackageVersion (which must be dropped).
const receiptsFixture = `{
  "InstallDate" => 2026-01-17 09:08:36 +0000
  "InstallPrefixPath" => "Applications/GarageBand.app/Contents/"
  "InstallProcessName" => "installer"
  "PackageFileName" => "GarageBand_MASReceipt.pkg"
  "PackageIdentifier" => "com.apple.cdm.pkg.GarageBand_MASReceipt"
  "PackageVersion" => "1.0"
}
{
  "InstallDate" => 2026-01-17 09:09:55 +0000
  "InstallPrefixPath" => "/"
  "InstallProcessName" => "installer"
  "PackageFileName" => "Keynote14.pkg"
  "PackageIdentifier" => "com.apple.pkg.Keynote14"
  "PackageVersion" => "14.4.1.1742681647"
}
{
  "InstallDate" => 2026-01-17 09:10:32 +0000
  "InstallPrefixPath" => "/"
  "PackageFileName" => "orphan.pkg"
  "PackageIdentifier" => "com.example.noversion"
}
`

func TestReceiptsPackages(t *testing.T) {
	pattern := "/var/db/receipts/*.plist"
	files := []string{
		"/var/db/receipts/com.apple.pkg.GarageBand.plist",
		"/var/db/receipts/com.apple.pkg.Keynote14.plist",
		"/var/db/receipts/com.example.noversion.plist",
	}
	var argv []string
	got := receiptsPackages(globStub(map[string][]string{pattern: files}), runStub(receiptsFixture, &argv))

	want := []any{
		packageRecord("com.apple.cdm.pkg.GarageBand_MASReceipt", "1.0"),
		packageRecord("com.apple.pkg.Keynote14", "14.4.1.1742681647"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("receiptsPackages = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(argv, files) {
		t.Fatalf("plutil argv = %v, want %v", argv, files)
	}
}

// plutilStub emulates the engine's run() around a real plutil: it prints one
// block per path in argument order, but returns "" for the WHOLE invocation
// when any argument is a corrupt path (plutil exits non-zero on any bad file,
// and run() yields "" on non-zero exit). Every invocation's path list is
// recorded so tests can assert the bisection shape.
func plutilStub(blocks map[string]string, corrupt map[string]bool, invocations *[][]string) commandRunner {
	return func(_ string, args ...string) string {
		paths := args[1:] // args[0] is "-p"
		if invocations != nil {
			*invocations = append(*invocations, append([]string(nil), paths...))
		}
		var out strings.Builder
		for _, p := range paths {
			if corrupt[p] {
				return ""
			}
			out.WriteString(blocks[p])
		}
		return out.String()
	}
}

func TestReceiptsPackages_corruptPlistBisection(t *testing.T) {
	// plutil is all-or-nothing per invocation: one corrupt receipt would
	// otherwise drop the whole chunk. The reader must bisect the failed chunk
	// and lose only the corrupt file.
	paths := []string{
		"/var/db/receipts/com.example.good1.plist",
		"/var/db/receipts/com.example.corrupt.plist",
		"/var/db/receipts/com.example.good2.plist",
	}
	blocks := map[string]string{
		paths[0]: `{
  "PackageIdentifier" => "com.example.good1"
  "PackageVersion" => "1.0"
}
`,
		paths[2]: `{
  "PackageIdentifier" => "com.example.good2"
  "PackageVersion" => "2.0"
}
`,
	}
	var invocations [][]string
	got := receiptsPackages(
		globStub(map[string][]string{"/var/db/receipts/*.plist": paths}),
		plutilStub(blocks, map[string]bool{paths[1]: true}, &invocations))

	want := []any{
		packageRecord("com.example.good1", "1.0"),
		packageRecord("com.example.good2", "2.0"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("receiptsPackages = %#v, want %#v", got, want)
	}
	wantInvocations := [][]string{
		{paths[0], paths[1], paths[2]}, // full chunk fails
		{paths[0]},                     // left half: good1 recovered
		{paths[1], paths[2]},           // right half fails
		{paths[1]},                     // corrupt alone: skipped
		{paths[2]},                     // good2 recovered
	}
	if !reflect.DeepEqual(invocations, wantInvocations) {
		t.Fatalf("plutil invocations = %v, want %v", invocations, wantInvocations)
	}
}

func TestReceiptsPackagesAbsent(t *testing.T) {
	if got := receiptsPackages(globStub(nil), runStub("", nil)); got != nil {
		t.Fatalf("receiptsPackages with no receipts = %#v, want nil", got)
	}
}

// appsFixture blocks are in glob-concatenation order (/Applications, then
// /Applications/Utilities, then /System/Applications, then
// /System/Applications/Utilities): 1Password, DisplayOnly, Nameless, Disk
// Helper, Calculator, Terminal. It mixes a real block (1Password, whose path
// has a space) and Calculator from /System (with both CFBundleName and a
// multi-line copyright whose continuation line is unindented) with synthetic
// blocks exercising every fallback: CFBundleDisplayName-only, name derived
// from the .app path, and CFBundleVersion-only. Nested array and dict values
// verify the depth tracker neither leaks nested keys nor splits blocks.
const appsFixture = `{
  "CFBundleIdentifier" => "com.agilebits.onepassword7"
  "CFBundleName" => "1Password 7"
  "CFBundleShortVersionString" => "7.9.11"
  "CFBundleSupportedPlatforms" => [
    0 => "MacOSX"
  ]
  "CFBundleVersion" => "70911001"
}
{
  "CFBundleDisplayName" => "Display Only"
  "CFBundleDocumentTypes" => [
    0 => {
      "CFBundleTypeName" => "public.item"
    }
  ]
  "CFBundleIdentifier" => "com.example.displayonly"
  "CFBundleVersion" => "3"
}
{
  "CFBundleIdentifier" => "com.example.nameless"
  "CFBundleVersion" => "9"
}
{
  "CFBundleIdentifier" => "com.example.diskhelper"
  "CFBundleName" => "Disk Helper"
  "CFBundleShortVersionString" => "1.1"
}
{
  "CFBundleDisplayName" => "Calculator"
  "CFBundleIdentifier" => "com.apple.calculator"
  "CFBundleName" => "Calculator"
  "CFBundleShortVersionString" => "12.0"
  "CFBundleVersion" => "225"
  "NSHumanReadableCopyright" => "Copyright © 2022-2025 Apple Inc.
All rights reserved."
}
{
  "CFBundleIdentifier" => "com.apple.Terminal"
  "CFBundleName" => "Terminal"
  "CFBundleShortVersionString" => "2.14"
  "CFBundleVersion" => "455"
}
`

var appsPaths = []string{
	"/Applications/1Password 7.app/Contents/Info.plist",
	"/System/Applications/Calculator.app/Contents/Info.plist",
	"/Applications/DisplayOnly.app/Contents/Info.plist",
	"/Applications/Nameless.app/Contents/Info.plist",
	"/Applications/Utilities/Disk Helper.app/Contents/Info.plist",
	"/System/Applications/Utilities/Terminal.app/Contents/Info.plist",
}

func TestAppsPackages(t *testing.T) {
	glob := globStub(map[string][]string{
		"/Applications/*.app/Contents/Info.plist":                  {appsPaths[0], appsPaths[2], appsPaths[3]},
		"/Applications/Utilities/*.app/Contents/Info.plist":        {appsPaths[4]},
		"/System/Applications/*.app/Contents/Info.plist":           {appsPaths[1]},
		"/System/Applications/Utilities/*.app/Contents/Info.plist": {appsPaths[5]},
	})
	var argv []string
	got := appsPackages(glob, runStub(appsFixture, &argv))

	// argv (and therefore plutil block order) follows the glob-pattern order:
	// /Applications, /Applications/Utilities, /System/Applications,
	// /System/Applications/Utilities — matching the fixture order.
	wantArgv := []string{appsPaths[0], appsPaths[2], appsPaths[3], appsPaths[4], appsPaths[1], appsPaths[5]}
	if !reflect.DeepEqual(argv, wantArgv) {
		t.Fatalf("plutil argv = %v, want %v", argv, wantArgv)
	}

	want := []any{
		packageRecord("1Password 7", "7.9.11",
			"bundle_id", "com.agilebits.onepassword7",
			"path", "/Applications/1Password 7.app"),
		packageRecord("Calculator", "12.0",
			"bundle_id", "com.apple.calculator",
			"path", "/System/Applications/Calculator.app"),
		packageRecord("Disk Helper", "1.1", // /Applications/Utilities bundle
			"bundle_id", "com.example.diskhelper",
			"path", "/Applications/Utilities/Disk Helper.app"),
		packageRecord("Display Only", "3", // CFBundleDisplayName fallback
			"bundle_id", "com.example.displayonly",
			"path", "/Applications/DisplayOnly.app"),
		packageRecord("Nameless", "9", // name derived from .app path
			"bundle_id", "com.example.nameless",
			"path", "/Applications/Nameless.app"),
		packageRecord("Terminal", "2.14", // /System/Applications/Utilities bundle
			"bundle_id", "com.apple.Terminal",
			"path", "/System/Applications/Utilities/Terminal.app"),
	}
	sortPackages(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appsPackages = %#v, want %#v", got, want)
	}
}

func TestAppsPackagesAbsent(t *testing.T) {
	if got := appsPackages(globStub(nil), runStub("", nil)); got != nil {
		t.Fatalf("appsPackages with no bundles = %#v, want nil", got)
	}
}

func TestAppsPackages_skipsAppWithoutVersion(t *testing.T) {
	// An app bundle with no CFBundleShortVersionString/CFBundleVersion must be
	// dropped (name+version invariant); its slot must not shift the positional
	// block<->path pairing for the remaining apps.
	glob := globStub(map[string][]string{
		"/Applications/*.app/Contents/Info.plist": {
			"/Applications/HasVer.app/Contents/Info.plist",
			"/Applications/NoVer.app/Contents/Info.plist",
		},
	})
	run := func(string, ...string) string {
		return `{
  "CFBundleName" => "HasVer"
  "CFBundleShortVersionString" => "2.0"
}
{
  "CFBundleName" => "NoVer"
}
`
	}
	got := appsPackages(glob, run)
	want := []any{map[string]any{"name": "HasVer", "version": "2.0", "path": "/Applications/HasVer.app"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appsPackages() = %#v\nwant %#v", got, want)
	}
}

func TestAppsPackages_corruptPlistBisection(t *testing.T) {
	// One corrupt Info.plist fails the whole plutil invocation. The reader must
	// bisect the failed chunk, keep every good app's record paired with its OWN
	// path (pairing is per successful invocation), and skip only the corrupt
	// file.
	paths := []string{
		"/Applications/Alpha.app/Contents/Info.plist",
		"/Applications/Broken.app/Contents/Info.plist",
		"/Applications/Gamma.app/Contents/Info.plist",
		"/Applications/Delta.app/Contents/Info.plist",
	}
	blocks := map[string]string{
		paths[0]: `{
  "CFBundleName" => "Alpha"
  "CFBundleShortVersionString" => "1.0"
}
`,
		paths[2]: `{
  "CFBundleName" => "Gamma"
  "CFBundleShortVersionString" => "3.0"
}
`,
		paths[3]: `{
  "CFBundleName" => "Delta"
  "CFBundleShortVersionString" => "4.0"
}
`,
	}
	glob := globStub(map[string][]string{"/Applications/*.app/Contents/Info.plist": paths})
	var invocations [][]string
	got := appsPackages(glob, plutilStub(blocks, map[string]bool{paths[1]: true}, &invocations))

	want := []any{
		packageRecord("Alpha", "1.0", "path", "/Applications/Alpha.app"),
		packageRecord("Delta", "4.0", "path", "/Applications/Delta.app"),
		packageRecord("Gamma", "3.0", "path", "/Applications/Gamma.app"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appsPackages = %#v, want %#v", got, want)
	}
	wantInvocations := [][]string{
		{paths[0], paths[1], paths[2], paths[3]}, // full chunk fails
		{paths[0], paths[1]},                     // left half fails
		{paths[0]},                               // Alpha recovered
		{paths[1]},                               // corrupt alone: skipped
		{paths[2], paths[3]},                     // right half succeeds intact
	}
	if !reflect.DeepEqual(invocations, wantInvocations) {
		t.Fatalf("plutil invocations = %v, want %v", invocations, wantInvocations)
	}
}

func TestAppsPackages_arrayRootedPlistKeepsPairing(t *testing.T) {
	// `plutil -p` of a plist whose root is an array prints a "[" ... "]" block
	// with no dict keys. It must still count as a block: the array-rooted app
	// yields no record (no name/version), but every LATER app in the chunk must
	// keep its own path — not inherit the array-rooted app's.
	glob := globStub(map[string][]string{
		"/Applications/*.app/Contents/Info.plist": {
			"/Applications/First.app/Contents/Info.plist",
			"/Applications/Weird.app/Contents/Info.plist",
			"/Applications/Third.app/Contents/Info.plist",
		},
	})
	run := func(string, ...string) string {
		return `{
  "CFBundleName" => "First"
  "CFBundleShortVersionString" => "1.0"
}
[
  0 => "array-rooted plist"
]
{
  "CFBundleName" => "Third"
  "CFBundleShortVersionString" => "3.0"
}
`
	}
	got := appsPackages(glob, run)
	want := []any{
		packageRecord("First", "1.0", "path", "/Applications/First.app"),
		packageRecord("Third", "3.0", "path", "/Applications/Third.app"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appsPackages = %#v, want %#v", got, want)
	}
}

func TestParsePlutilBlocks_arrayRootBlock(t *testing.T) {
	// An array-rooted block fires fn with an EMPTY fields map (array elements
	// like `0 => "x"` are not dict keys) and must not disturb its neighbours.
	const out = `{
  "A" => "1"
}
[
  0 => "not a key"
]
{
  "B" => "2"
}
`
	var got []map[string]string
	parsePlutilBlocks(out, func(f map[string]string) {
		got = append(got, f)
	})
	want := []map[string]string{{"A": "1"}, {}, {"B": "2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blocks = %#v, want %#v", got, want)
	}
}

func TestHomebrewPackages(t *testing.T) {
	glob := globStub(map[string][]string{
		"/opt/homebrew/Cellar/*/*": {
			"/opt/homebrew/Cellar/ada-url/3.4.4",
			"/opt/homebrew/Cellar/brotli/1.2.0",
		},
		"/opt/homebrew/Caskroom/*/*": {
			"/opt/homebrew/Caskroom/amethyst/.metadata", // dotfile sidecar, skipped
			"/opt/homebrew/Caskroom/amethyst/0.24.3",
			"/opt/homebrew/Caskroom/ghostty/1.3.1",
		},
	})
	got := homebrewPackages(glob)

	want := []any{
		packageRecord("ada-url", "3.4.4", "type", "formula", "prefix", "/opt/homebrew"),
		packageRecord("amethyst", "0.24.3", "type", "cask", "prefix", "/opt/homebrew"),
		packageRecord("brotli", "1.2.0", "type", "formula", "prefix", "/opt/homebrew"),
		packageRecord("ghostty", "1.3.1", "type", "cask", "prefix", "/opt/homebrew"),
	}
	sortPackages(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("homebrewPackages = %#v, want %#v", got, want)
	}
}

func TestHomebrewPackagesIntelPrefix(t *testing.T) {
	glob := globStub(map[string][]string{
		"/usr/local/Cellar/*/*": {"/usr/local/Cellar/wget/1.25.0"},
	})
	got := homebrewPackages(glob)
	want := []any{packageRecord("wget", "1.25.0", "type", "formula", "prefix", "/usr/local")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("homebrewPackages (intel) = %#v, want %#v", got, want)
	}
}

func TestHomebrewPackagesDualPrefix(t *testing.T) {
	// Apple Silicon brew (/opt/homebrew) and Rosetta Intel brew (/usr/local)
	// can coexist with the same formula@version in both Cellars. The two
	// installs are distinct and must yield two records distinguished by the
	// prefix identity field rather than collapsing into byte-identical twins.
	glob := globStub(map[string][]string{
		"/opt/homebrew/Cellar/*/*": {"/opt/homebrew/Cellar/wget/1.25.0"},
		"/usr/local/Cellar/*/*":    {"/usr/local/Cellar/wget/1.25.0"},
	})
	got := homebrewPackages(glob)
	want := []any{
		packageRecord("wget", "1.25.0", "type", "formula", "prefix", "/opt/homebrew"),
		packageRecord("wget", "1.25.0", "type", "formula", "prefix", "/usr/local"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("homebrewPackages (dual prefix) = %#v, want %#v", got, want)
	}
}

func TestHomebrewPackagesCaskOnly(t *testing.T) {
	// A cask-only install has an empty Cellar but real casks to report.
	glob := globStub(map[string][]string{
		"/opt/homebrew/Caskroom/*/*": {"/opt/homebrew/Caskroom/amethyst/0.24.3"},
	})
	got := homebrewPackages(glob)
	want := []any{packageRecord("amethyst", "0.24.3", "type", "cask", "prefix", "/opt/homebrew")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("homebrewPackages (cask-only) = %#v, want %#v", got, want)
	}
}

func TestHomebrewPackagesNoHomebrew(t *testing.T) {
	// Neither a Cellar nor a Caskroom under any prefix: the source is omitted.
	if got := homebrewPackages(globStub(map[string][]string{})); got != nil {
		t.Fatalf("homebrewPackages with no Homebrew = %#v, want nil", got)
	}
}

func TestParsePlutilBlocks_multilineStringWithBraceLines(t *testing.T) {
	// plutil prints multi-line string values raw, so a value can contain lines
	// that are exactly "{", "}", "[", or "]". Those must not be interpreted
	// structurally: keys after the string must still parse, the block must not
	// close early, and the second block must still be counted. A value whose
	// first line ends in an escaped quote (\") must stay open too.
	const out = `{
  "Before" => "yes"
  "Notes" => "opening
{
}
[
]
closing line"
  "Tricky" => "ends with \"
still inside
done"
  "After" => "seen"
}
{
  "Second" => "block"
}
`
	var got []map[string]string
	parsePlutilBlocks(out, func(f map[string]string) {
		got = append(got, f)
	})
	want := []map[string]string{
		{
			"Before": "yes",
			"Notes":  "opening",     // first physical line only, by design
			"Tricky": `ends with \`, // first-line capture strips the quotes
			"After":  "seen",
		},
		{"Second": "block"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blocks = %#v, want %#v", got, want)
	}
}

func TestParsePlutilBlocksNestedAndMultiline(t *testing.T) {
	// A single block whose top level carries a scalar, a nested array (with a
	// dict element), a nested dict, and a multi-line string. Only depth-1
	// scalars must survive; the nested "leaked" keys must not.
	const block = `{
  "Top" => "value"
  "Arr" => [
    0 => {
      "leaked" => "nested"
    }
  ]
  "Dict" => {
    "leaked" => "nested"
  }
  "Multi" => "line one
line two"
  "After" => "seen"
}
`
	var fields map[string]string
	count := 0
	parsePlutilBlocks(block, func(f map[string]string) {
		fields = f
		count++
	})
	if count != 1 {
		t.Fatalf("block count = %d, want 1", count)
	}
	// Exactly the depth-1 scalars survive: the multi-line value is captured (first
	// physical line only), and the nested "leaked" keys and container openers
	// (Arr, Dict) are absent. Full-map equality asserts all three at once.
	want := map[string]string{
		"Top":   "value",
		"Multi": "line one",
		"After": "seen",
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("top-level scalars = %#v, want exactly %#v", fields, want)
	}
}
