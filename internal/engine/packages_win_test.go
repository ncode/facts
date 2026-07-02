package engine

import (
	"reflect"
	"strings"
	"testing"
)

// registryUninstallFixture is the delimited output of registryPackagesScript.
// Columns are arch/PSChildName/DisplayVersion/SystemComponent/DisplayName joined
// by the Unit Separator (0x1f) — DisplayVersion and the subkey name are free-text
// REG_SZ values that can legally contain '|', so the script emits a control
// character that cannot appear in them. The native hive is emitted before the
// x86 WOW6432Node hive. It exercises the real shapes seen in the wild: an MSI
// GUID subkey (product_code populated) and a bespoke Inno Setup "*_is1" subkey
// (no product_code) in each architecture, plus SystemComponent=1 runtime entries
// that must be dropped. Values are real Microsoft/third-party identifiers; the
// nlab guest ships bare (only a property-less "WIC" key per hive, filtered by
// Where-Object DisplayName), so the multi-entry cases use authentic package
// identities in that live format.
const registryUninstallFixture = "x64\x1f{e46eca4f-393b-40df-9f49-076faf788d83}\x1f14.34.31931.0\x1f\x1fMicrosoft Visual C++ 2015-2022 Redistributable (x64) - 14.34.31931\n" +
	"x64\x1f{f1b0fb2f-3d5f-4c1e-9b2a-0a1b2c3d4e5f}\x1f14.34.31931\x1f1\x1fMicrosoft Visual C++ 2022 X64 Minimum Runtime - 14.34.31931\n" +
	"x64\x1fGit_is1\x1f2.43.0.2\x1f\x1fGit\n" +
	"x86\x1f{a1c31ba0-9a3b-4f2d-8c7e-1234567890ab}\x1f14.34.31931.0\x1f\x1fMicrosoft Visual C++ 2015-2022 Redistributable (x86) - 14.34.31931\n" +
	"x86\x1fNotepad++\x1f8.6.9\x1f\x1fNotepad++ (32-bit x86)\n" +
	"x86\x1f{deadbeef-0000-1111-2222-333344445555}\x1f1.0.0\x1f1\x1fSome 32-bit Runtime\n"

func TestRegistryPackages_bothHivesGUIDAndSystemComponent(t *testing.T) {
	t.Parallel()
	// Feed CRLF line endings to mirror real PowerShell output on Windows.
	got := registryPackages(func(name string, args ...string) string {
		if name != "powershell" {
			t.Fatalf("command = %q %v", name, args)
		}
		if args[len(args)-1] != registryPackagesScript {
			t.Fatalf("script arg = %q", args[len(args)-1])
		}
		return strings.ReplaceAll(registryUninstallFixture, "\n", "\r\n")
	})
	want := []any{
		map[string]any{"name": "Git", "version": "2.43.0.2", "architecture": "x64"},
		map[string]any{
			"name":         "Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.34.31931",
			"version":      "14.34.31931.0",
			"product_code": "{e46eca4f-393b-40df-9f49-076faf788d83}",
			"architecture": "x64",
		},
		map[string]any{
			"name":         "Microsoft Visual C++ 2015-2022 Redistributable (x86) - 14.34.31931",
			"version":      "14.34.31931.0",
			"product_code": "{a1c31ba0-9a3b-4f2d-8c7e-1234567890ab}",
			"architecture": "x86",
		},
		map[string]any{"name": "Notepad++ (32-bit x86)", "version": "8.6.9", "architecture": "x86"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registryPackages() = %#v\nwant %#v", got, want)
	}
}

// TestRegistryPackages_skipsPropertylessKey mirrors the real nlab guest: the
// sole Uninstall subkey is "WIC" with no DisplayName. Where-Object filters it
// upstream, and the reader's guard drops any such line that slips through.
func TestRegistryPackages_skipsPropertylessKey(t *testing.T) {
	t.Parallel()
	got := registryPackages(func(string, ...string) string { return "x64\x1fWIC\x1f\x1f\x1f\n" })
	if got != nil {
		t.Fatalf("registryPackages(propertyless) = %#v, want nil", got)
	}
}

func TestRegistryPackages_absentYieldsNothing(t *testing.T) {
	t.Parallel()
	if got := registryPackages(func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("registryPackages(absent) = %#v, want nil", got)
	}
}

func TestRegistryMsiProductCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"{e46eca4f-393b-40df-9f49-076faf788d83}", "{e46eca4f-393b-40df-9f49-076faf788d83}"},
		{"{DEADBEEF-0000-1111-2222-333344445555}", "{DEADBEEF-0000-1111-2222-333344445555}"},
		{"Git_is1", ""},
		{"Notepad++", ""},
		{"{e46eca4f-393b-40df-9f49-076faf788d83", ""},  // missing closing brace
		{"{g46eca4f-393b-40df-9f49-076faf788d83}", ""}, // non-hex digit
		{"{e46eca4f393b40df9f49076faf788d83xxxx}", ""}, // wrong dash layout
		{"", ""},
	}
	for _, c := range cases {
		if got := msiProductCode(c.in); got != c.want {
			t.Fatalf("msiProductCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// appxFixture is the delimited output of appxPackagesScript: the provisioned
// set (DisplayName|Version|Architecture) followed by the collector-context
// Get-AppxPackage set (Name|Version|Architecture). Get-AppxProvisionedPackage
// exposes Architecture as the raw DISM UInt32 (x86=0, arm=5, x64=9, neutral=11,
// arm64=12) while Get-AppxPackage renders the enum name (X64, Neutral), so the
// provisioned half uses the numeric shape and the collector half the enum names.
// The fixture exercises the numeric→label mapping, enum lowercasing, cross-view
// deduplication (numeric and enum spellings of the same package must collapse to
// one record), and the empty-version skip. Values are real Windows package
// identities; the nlab guest ships zero appx (DISM provisioned = 0,
// Get-AppxPackage = 0), so the parse is validated against this authentic live
// format rather than empty guest output.
const appxFixture = `Microsoft.WindowsStore|22210.1401.7.0|9
Microsoft.VCLibs.140.00|14.0.30704.0|9
Microsoft.VCLibs.140.00|14.0.30704.0|0
Microsoft.VCLibs.140.00.UWPDesktop|14.0.33728.0|5
Microsoft.UI.Xaml.2.8|8.2306.22001.0|11
Microsoft.SecHealthUI|1000.25992.9000.0|12
Microsoft.NET.Native.Runtime.2.2||9
Windows Calculator|11.2210.0.0|X64
Microsoft.WindowsStore|22210.1401.7.0|X64
Microsoft.VCLibs.140.00|14.0.30704.0|X64
Microsoft.VCLibs.140.00|14.0.30704.0|X86
Microsoft.UI.Xaml.2.8|8.2306.22001.0|Neutral
Microsoft.WindowsTerminal|1.18.3181.0|X86
`

func TestAppxPackages_mapsDISMArchDedupsAcrossViewsSkipsEmptyVersion(t *testing.T) {
	t.Parallel()
	got := appxPackages(func(name string, args ...string) string {
		if name != "powershell" {
			t.Fatalf("command = %q %v", name, args)
		}
		if args[len(args)-1] != appxPackagesScript {
			t.Fatalf("script arg = %q", args[len(args)-1])
		}
		return appxFixture
	})
	want := []any{
		map[string]any{"name": "Microsoft.SecHealthUI", "version": "1000.25992.9000.0", "architecture": "arm64"},
		map[string]any{"name": "Microsoft.UI.Xaml.2.8", "version": "8.2306.22001.0", "architecture": "neutral"},
		map[string]any{"name": "Microsoft.VCLibs.140.00", "version": "14.0.30704.0", "architecture": "x64"},
		map[string]any{"name": "Microsoft.VCLibs.140.00", "version": "14.0.30704.0", "architecture": "x86"},
		map[string]any{"name": "Microsoft.VCLibs.140.00.UWPDesktop", "version": "14.0.33728.0", "architecture": "arm"},
		map[string]any{"name": "Microsoft.WindowsStore", "version": "22210.1401.7.0", "architecture": "x64"},
		map[string]any{"name": "Microsoft.WindowsTerminal", "version": "1.18.3181.0", "architecture": "x86"},
		map[string]any{"name": "Windows Calculator", "version": "11.2210.0.0", "architecture": "x64"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appxPackages() = %#v\nwant %#v", got, want)
	}
}

// TestAppxPackages_unknownNumericArchPassesThrough pins the mapping to the five
// documented DISM values: an unrecognized code (6 = ia64) must survive as-is
// rather than being guessed at.
func TestAppxPackages_unknownNumericArchPassesThrough(t *testing.T) {
	t.Parallel()
	got := appxPackages(func(string, ...string) string { return "Some.Package|1.0.0.0|6\n" })
	want := []any{map[string]any{"name": "Some.Package", "version": "1.0.0.0", "architecture": "6"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appxPackages() = %#v\nwant %#v", got, want)
	}
}

func TestAppxPackages_absentYieldsNothing(t *testing.T) {
	t.Parallel()
	if got := appxPackages(func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("appxPackages(absent) = %#v, want nil", got)
	}
}

func TestRegistryPackages_skipsEmptyDisplayVersion(t *testing.T) {
	t.Parallel()
	// An uninstall entry with a DisplayName but no DisplayVersion must be dropped
	// (the name+version invariant), while a fully-populated entry is kept.
	run := func(string, ...string) string {
		return "x64\x1f{6F320B93-EE3C-4826-85E0-ADF79F8D4C61}\x1f1.2.3\x1f0\x1fReal App\n" +
			"x64\x1fSomeKey_is1\x1f\x1f0\x1fNo Version App\n"
	}
	got := registryPackages(run)
	want := []any{map[string]any{
		"name": "Real App", "version": "1.2.3",
		"product_code": "{6F320B93-EE3C-4826-85E0-ADF79F8D4C61}", "architecture": "x64",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registryPackages() = %#v\nwant %#v", got, want)
	}
}

// TestRegistryPackages_pipeInFreeTextDoesNotShiftColumns pins the reason for the
// Unit Separator delimiter: DisplayVersion and DisplayName are free-text REG_SZ
// values, so a '|' inside either must survive as data instead of shifting the
// five fixed columns.
func TestRegistryPackages_pipeInFreeTextDoesNotShiftColumns(t *testing.T) {
	t.Parallel()
	run := func(string, ...string) string {
		return "x64\x1fVLC media player\x1f3.0.20|nightly\x1f\x1fVLC media player | 64-bit\n"
	}
	got := registryPackages(run)
	want := []any{map[string]any{
		"name": "VLC media player | 64-bit", "version": "3.0.20|nightly", "architecture": "x64",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registryPackages() = %#v\nwant %#v", got, want)
	}
}

// TestWinPackagesScripts_utf8OutputEncoding pins the UTF-8 console-encoding
// prefix on both scripts: redirected PowerShell stdout otherwise uses the OEM
// codepage (CP437 on en-US), which mangles every non-ASCII DisplayName (ö
// becomes the invalid byte 0x94, ® best-fits to "r").
func TestWinPackagesScripts_utf8OutputEncoding(t *testing.T) {
	t.Parallel()
	const prefix = `[Console]::OutputEncoding=[Text.Encoding]::UTF8;`
	for name, script := range map[string]string{
		"registry": registryPackagesScript,
		"appx":     appxPackagesScript,
	} {
		if !strings.HasPrefix(script, prefix) {
			t.Errorf("%s script does not set UTF-8 output encoding first:\n%s", name, script)
		}
	}
}

// TestWinPackagesScripts_exitZero pins the ";exit 0" terminator on both scripts:
// when a script's LAST statement errors, powershell exits 1 even under
// SilentlyContinue, and the engine's run() discards ALL stdout on any non-zero
// exit — e.g. Get-AppxPackage failing under the SYSTEM context would otherwise
// kill the already-emitted provisioned lines, and a missing WOW6432Node hive on
// 32-bit Windows would kill the whole registry source.
func TestWinPackagesScripts_exitZero(t *testing.T) {
	t.Parallel()
	for name, script := range map[string]string{
		"registry": registryPackagesScript,
		"appx":     appxPackagesScript,
	} {
		if !strings.HasSuffix(script, ";exit 0") {
			t.Errorf("%s script does not terminate with ;exit 0:\n%s", name, script)
		}
	}
}

// TestRegistryPackagesScript_derivesNativeArch pins that native-hive rows carry
// an architecture computed from $env:PROCESSOR_ARCHITECTURE (AMD64→x64,
// ARM64→arm64, X86→x86, anything else lowercased) instead of a hardcoded "x64",
// which would be wrong on Windows-on-ARM. WOW6432Node rows stay literal x86 —
// that hive only ever holds 32-bit x86 redirects.
func TestRegistryPackagesScript_derivesNativeArch(t *testing.T) {
	t.Parallel()
	const naExpr = `$na=@{'AMD64'='x64';'ARM64'='arm64';'X86'='x86'}[$env:PROCESSOR_ARCHITECTURE];` +
		`if(-not $na){$na="$env:PROCESSOR_ARCHITECTURE".ToLower()};`
	if !strings.Contains(registryPackagesScript, naExpr) {
		t.Fatalf("registry script does not compute $na from PROCESSOR_ARCHITECTURE:\n%s", registryPackagesScript)
	}
	if !strings.Contains(registryPackagesScript, `ForEach-Object{"$na$([char]31)`) {
		t.Fatalf("registry script does not emit $na for native-hive rows:\n%s", registryPackagesScript)
	}
	if !strings.Contains(registryPackagesScript, `ForEach-Object{"x86$([char]31)`) {
		t.Fatalf("registry script must keep literal x86 for WOW6432Node rows:\n%s", registryPackagesScript)
	}
	if strings.Contains(registryPackagesScript, `"x64`) {
		t.Fatalf("registry script still hardcodes x64 for the native hive:\n%s", registryPackagesScript)
	}
}

// TestWinPackagesScripts_delimiters pins the wire format each script emits: the
// registry script joins its five columns with $([char]31) (the Unit Separator,
// which free-text REG_SZ values cannot contain), while the appx script keeps '|'
// (appx names/versions are strict identifiers where '|' cannot occur).
func TestWinPackagesScripts_delimiters(t *testing.T) {
	t.Parallel()
	const emitTail = `$([char]31)$($_.PSChildName)$([char]31)$($_.DisplayVersion)$([char]31)$($_.SystemComponent)$([char]31)$($_.DisplayName)"`
	if got := strings.Count(registryPackagesScript, emitTail); got != 2 {
		t.Fatalf("registry script emits %d unit-separator column tails, want 2 (native + WOW6432Node):\n%s", got, registryPackagesScript)
	}
	if strings.Contains(registryPackagesScript, `)|$(`) {
		t.Fatalf("registry script still joins columns with '|':\n%s", registryPackagesScript)
	}
	if strings.Contains(appxPackagesScript, "$([char]31)") {
		t.Fatalf("appx script must keep the '|' delimiter:\n%s", appxPackagesScript)
	}
}
