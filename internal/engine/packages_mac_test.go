package engine

import (
	"reflect"
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

func TestReceiptsPackagesAbsent(t *testing.T) {
	if got := receiptsPackages(globStub(nil), runStub("", nil)); got != nil {
		t.Fatalf("receiptsPackages with no receipts = %#v, want nil", got)
	}
}

// appsFixture blocks are in glob-concatenation order (all /Applications first,
// then /System/Applications): 1Password, DisplayOnly, Nameless, Calculator. It
// mixes a real block (1Password, whose path has a space) and Calculator from
// /System (with both CFBundleName and a multi-line copyright whose continuation
// line is unindented) with synthetic blocks exercising every fallback:
// CFBundleDisplayName-only, name derived from the .app path, and
// CFBundleVersion-only. Nested array and dict values verify the depth tracker
// neither leaks nested keys nor splits blocks.
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
  "CFBundleDisplayName" => "Calculator"
  "CFBundleIdentifier" => "com.apple.calculator"
  "CFBundleName" => "Calculator"
  "CFBundleShortVersionString" => "12.0"
  "CFBundleVersion" => "225"
  "NSHumanReadableCopyright" => "Copyright © 2022-2025 Apple Inc.
All rights reserved."
}
`

var appsPaths = []string{
	"/Applications/1Password 7.app/Contents/Info.plist",
	"/System/Applications/Calculator.app/Contents/Info.plist",
	"/Applications/DisplayOnly.app/Contents/Info.plist",
	"/Applications/Nameless.app/Contents/Info.plist",
}

func TestAppsPackages(t *testing.T) {
	glob := globStub(map[string][]string{
		"/Applications/*/Contents/Info.plist":        {appsPaths[0], appsPaths[2], appsPaths[3]},
		"/System/Applications/*/Contents/Info.plist": {appsPaths[1]},
	})
	var argv []string
	got := appsPackages(glob, runStub(appsFixture, &argv))

	// argv (and therefore plutil block order) is /Applications first, then
	// /System/Applications, matching the fixture order.
	wantArgv := []string{appsPaths[0], appsPaths[2], appsPaths[3], appsPaths[1]}
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
		packageRecord("Display Only", "3", // CFBundleDisplayName fallback
			"bundle_id", "com.example.displayonly",
			"path", "/Applications/DisplayOnly.app"),
		packageRecord("Nameless", "9", // name derived from .app path
			"bundle_id", "com.example.nameless",
			"path", "/Applications/Nameless.app"),
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
		"/Applications/*/Contents/Info.plist": {
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
		packageRecord("ada-url", "3.4.4", "type", "formula"),
		packageRecord("amethyst", "0.24.3", "type", "cask"),
		packageRecord("brotli", "1.2.0", "type", "formula"),
		packageRecord("ghostty", "1.3.1", "type", "cask"),
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
	want := []any{packageRecord("wget", "1.25.0", "type", "formula")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("homebrewPackages (intel) = %#v, want %#v", got, want)
	}
}

func TestHomebrewPackagesCaskOnly(t *testing.T) {
	// A cask-only install has an empty Cellar but real casks to report.
	glob := globStub(map[string][]string{
		"/opt/homebrew/Caskroom/*/*": {"/opt/homebrew/Caskroom/amethyst/0.24.3"},
	})
	got := homebrewPackages(glob)
	want := []any{packageRecord("amethyst", "0.24.3", "type", "cask")}
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
