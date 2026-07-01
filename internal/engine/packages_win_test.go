package engine

import (
	"reflect"
	"strings"
	"testing"
)

// registryUninstallFixture is the delimited output of registryPackagesScript.
// Columns are arch|PSChildName|DisplayVersion|SystemComponent|DisplayName, with
// the x64 native hive emitted before the x86 WOW6432Node hive. It exercises the
// real shapes seen in the wild: an MSI GUID subkey (product_code populated) and
// a bespoke Inno Setup "*_is1" subkey (no product_code) in each architecture,
// plus SystemComponent=1 runtime entries that must be dropped. Values are real
// Microsoft/third-party identifiers; the nlab guest ships bare (only a
// property-less "WIC" key per hive, filtered by Where-Object DisplayName), so
// the multi-entry cases use authentic package identities in that live format.
const registryUninstallFixture = `x64|{e46eca4f-393b-40df-9f49-076faf788d83}|14.34.31931.0||Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.34.31931
x64|{f1b0fb2f-3d5f-4c1e-9b2a-0a1b2c3d4e5f}|14.34.31931|1|Microsoft Visual C++ 2022 X64 Minimum Runtime - 14.34.31931
x64|Git_is1|2.43.0.2||Git
x86|{a1c31ba0-9a3b-4f2d-8c7e-1234567890ab}|14.34.31931.0||Microsoft Visual C++ 2015-2022 Redistributable (x86) - 14.34.31931
x86|Notepad++|8.6.9||Notepad++ (32-bit x86)
x86|{deadbeef-0000-1111-2222-333344445555}|1.0.0|1|Some 32-bit Runtime
`

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
	got := registryPackages(func(string, ...string) string { return "x64|WIC|||\n" })
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
// Get-AppxPackage set (Name|Version|Architecture). It exercises architecture
// lowercasing, cross-view deduplication (the VCLibs line appears twice), and the
// empty-version skip. Values are real Windows package identities; the nlab guest
// ships zero appx (DISM provisioned = 0, Get-AppxPackage = 0), so the parse is
// validated against this authentic live format rather than empty guest output.
const appxFixture = `Windows Calculator|11.2210.0.0|X64
Microsoft.WindowsStore|22210.1401.7.0|X64
Microsoft.VCLibs.140.00|14.0.30704.0|X64
Microsoft.VCLibs.140.00|14.0.30704.0|X64
Microsoft.WindowsTerminal|1.18.3181.0|X86
Microsoft.NET.Native.Runtime.2.2||X64
Microsoft.UI.Xaml.2.8|8.2306.22001.0|neutral
`

func TestAppxPackages_dedupLowercaseArchSkipsEmptyVersion(t *testing.T) {
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
		map[string]any{"name": "Microsoft.UI.Xaml.2.8", "version": "8.2306.22001.0", "architecture": "neutral"},
		map[string]any{"name": "Microsoft.VCLibs.140.00", "version": "14.0.30704.0", "architecture": "x64"},
		map[string]any{"name": "Microsoft.WindowsStore", "version": "22210.1401.7.0", "architecture": "x64"},
		map[string]any{"name": "Microsoft.WindowsTerminal", "version": "1.18.3181.0", "architecture": "x86"},
		map[string]any{"name": "Windows Calculator", "version": "11.2210.0.0", "architecture": "x64"},
	}
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
		return "x64|{6F320B93-EE3C-4826-85E0-ADF79F8D4C61}|1.2.3|0|Real App\n" +
			"x64|SomeKey_is1||0|No Version App\n"
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
