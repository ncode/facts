package engine

import (
	"runtime"
	"testing"
	"time"
)

func TestWindowsTimezoneKeepsValidUTF8ZoneName(t *testing.T) {
	t.Parallel()

	zone := "Hora estándar"
	got := currentWindowsTimezone("windows", zone, "850", func() string {
		t.Fatal("registry codepage should not be used for valid UTF-8")
		return ""
	})

	if got != zone {
		t.Fatalf("currentWindowsTimezone() = %q, want %q", got, zone)
	}
}

func TestWindowsTimezoneUsesAPICodepage(t *testing.T) {
	t.Parallel()

	got := currentWindowsTimezone("windows", "Central Europ\x82en Standard Time", "850", func() string {
		t.Fatal("registry codepage should not be used when API returns a value")
		return ""
	})

	if got != "Central Européen Standard Time" {
		t.Fatalf("currentWindowsTimezone() = %q, want %q", got, "Central Européen Standard Time")
	}
}

func TestWindowsTimezoneFallsBackToRegistryCodepage(t *testing.T) {
	t.Parallel()

	got := currentWindowsTimezone("windows", "Hora est\xa0ndar", "", func() string {
		return "850"
	})

	if got != "Hora estándar" {
		t.Fatalf("currentWindowsTimezone() = %q, want %q", got, "Hora estándar")
	}
}

func TestWindowsTimezoneKeepsOriginalForInvalidCodepage(t *testing.T) {
	t.Parallel()

	zone := "UTC"
	got := currentWindowsTimezone("windows", zone, "not-a-codepage", func() string { return "" })
	if got != zone {
		t.Fatalf("currentWindowsTimezone() = %q, want %q", got, zone)
	}
}

func TestCurrentWindowsTimezoneRunsOnlyOnWindows(t *testing.T) {
	t.Parallel()

	called := false
	got := currentWindowsTimezone("linux", "UTC", "850", func() string {
		called = true
		return "850"
	})
	if got != "" {
		t.Fatalf("currentWindowsTimezone(non-windows) = %q, want empty", got)
	}
	if called {
		t.Fatal("currentWindowsTimezone(non-windows) read registry codepage")
	}
}

func TestCurrentWindowsTimezoneReturnsEmptyForEmptyZone(t *testing.T) {
	t.Parallel()

	called := false
	got := currentWindowsTimezone("windows", "", "850", func() string {
		called = true
		return "850"
	})
	if got != "" {
		t.Fatalf("currentWindowsTimezone(empty zone) = %q, want empty", got)
	}
	if called {
		t.Fatal("currentWindowsTimezone(empty zone) read registry codepage")
	}
}

func TestCurrentTimezoneForZoneDecodesWindowsZoneWithSessionCodepage(t *testing.T) {
	s := NewSession()
	s.host = &fakeHostOS{
		platform: "windows",
		runOutputs: map[string]string{
			fakeRunKey("cmd", "/c", "chcp"): "Active code page: 850.\n",
		},
	}

	got := currentTimezoneForZone(s, "windows", "Hora est\xa0ndar")
	if got != "Hora estándar" {
		t.Fatalf("currentTimezoneForZone(windows) = %q, want Hora estándar", got)
	}
}

func TestCurrentTimezoneForZoneKeepsZoneOutsideWindows(t *testing.T) {
	got := currentTimezoneForZone(testSession, "linux", "UTC")
	if got != "UTC" {
		t.Fatalf("currentTimezoneForZone(linux) = %q, want UTC", got)
	}
}

func TestCurrentTimezoneForZoneKeepsEmptyWindowsZone(t *testing.T) {
	got := currentTimezoneForZone(testSession, "windows", "")
	if got != "" {
		t.Fatalf("currentTimezoneForZone(empty windows zone) = %q, want empty", got)
	}
}

func TestCurrentWindowsCodepageProbesReadWindowsCommands(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows codepage probes only run on Windows")
	}

	host := &fakeHostOS{runOutputs: map[string]string{
		fakeRunKey("cmd", "/c", "chcp"): "Active code page: 850.\n",
		fakeRunKey("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Control\Nls\CodePage`, "/v", "ACP"): `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Nls\CodePage
    ACP    REG_SZ    936
`,
	}}
	s := NewSession()
	s.host = host

	api := currentWindowsAPICodepage(s)
	registry := currentWindowsRegistryCodepage(s)
	if api != "850" || registry != "936" {
		t.Fatalf("codepages = %q, %q; want 850, 936", api, registry)
	}
	if len(host.runCalls) != 2 {
		t.Fatalf("run calls = %#v, want API and registry probes", host.runCalls)
	}
}

func TestCurrentWindowsCodepageProbesDoNotRunOutsideWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-Windows guard assertion")
	}

	host := &fakeHostOS{runOutputs: map[string]string{
		fakeRunKey("cmd", "/c", "chcp"): "Active code page: 850.\n",
		fakeRunKey("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Control\Nls\CodePage`, "/v", "ACP"): `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Nls\CodePage
    ACP    REG_SZ    936
`,
	}}
	s := NewSession()
	s.host = host

	api := currentWindowsAPICodepage(s)
	registry := currentWindowsRegistryCodepage(s)
	if api != "" || registry != "" {
		t.Fatalf("non-Windows codepages = %q, %q; want empty", api, registry)
	}
	if len(host.runCalls) != 0 {
		t.Fatalf("non-Windows run calls = %#v, want none", host.runCalls)
	}
}

func TestCurrentWindowsCodepageProbesUseSessionPlatform(t *testing.T) {
	host := &fakeHostOS{
		platform: "windows",
		runOutputs: map[string]string{
			fakeRunKey("cmd", "/c", "chcp"): "Active code page: 850.\n",
			fakeRunKey("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Control\Nls\CodePage`, "/v", "ACP"): `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Nls\CodePage
    ACP    REG_SZ    936
`,
		},
	}
	s := NewSession()
	s.host = host

	api := currentWindowsAPICodepage(s)
	registry := currentWindowsRegistryCodepage(s)
	if api != "850" || registry != "936" {
		t.Fatalf("codepages = %q, %q; want 850, 936", api, registry)
	}
}

func TestParseWindowsACPRegistry(t *testing.T) {
	t.Parallel()

	input := `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Nls\CodePage
    ACP    REG_SZ    936
    OEMCP  REG_SZ    437
`
	if got := parseWindowsACPRegistry(input); got != "936" {
		t.Fatalf("parseWindowsACPRegistry() = %q, want 936", got)
	}
	if got := parseWindowsACPRegistry("OEMCP REG_SZ 437"); got != "" {
		t.Fatalf("parseWindowsACPRegistry(no ACP) = %q, want empty", got)
	}
}

func TestFirstNumberExtractsTrimmedCodepage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "Active code page: 850.", want: "850"},
		{input: "Codepage CP1252", want: ""},
		{input: "no number here", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := firstNumber(tt.input); got != tt.want {
				t.Fatalf("firstNumber(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecodeWindowsCodepageSupportsRubyCompatibleCodepages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		codepage string
		want     string
		wantOK   bool
	}{
		{name: "cp437", value: "\x9b", codepage: "CP437", want: "¢", wantOK: true},
		{name: "cp850", value: "\x9b", codepage: "850", want: "ø", wantOK: true},
		{name: "windows 1252", value: "\xe9", codepage: "1252", want: "é", wantOK: true},
		{name: "unsupported", value: "\xe9", codepage: "not-a-codepage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeWindowsCodepage(tt.value, tt.codepage)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("decodeWindowsCodepage(%q, %q) = (%q, %v), want (%q, %v)", tt.value, tt.codepage, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestCurrentTimezoneLinuxMatchesRubyResolverFormat(t *testing.T) {
	t.Parallel()
	assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t, "linux")
}

func TestCurrentTimezoneDarwinMatchesRubyResolverFormat(t *testing.T) {
	t.Parallel()
	assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t, "darwin")
}

func TestCurrentTimezoneFreeBSDMatchesRubyResolverFormat(t *testing.T) {
	t.Parallel()
	assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t, "freebsd")
}

func TestCurrentTimezoneOpenBSDMatchesRubyResolverFormat(t *testing.T) {
	t.Parallel()
	assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t, "openbsd")
}

func TestCurrentTimezoneNetBSDMatchesRubyResolverFormat(t *testing.T) {
	t.Parallel()
	assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t, "netbsd")
}

func TestCurrentTimezoneWindowsKeepsValidLocalZone(t *testing.T) {
	s := NewSession()
	s.host = &fakeHostOS{
		platform: "windows",
		runOutputs: map[string]string{
			fakeRunKey("cmd", "/c", "chcp"): "",
		},
	}

	before := time.Now().Format("MST")
	got := currentTimezone(s, "windows")
	after := time.Now().Format("MST")
	if got != before && got != after {
		t.Fatalf("currentTimezone(windows) = %q, want local timezone abbreviation %q or %q", got, before, after)
	}
}

func assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t *testing.T, goos string) {
	t.Helper()

	before := time.Now().Format("MST")
	got := currentTimezone(testSession, goos)
	after := time.Now().Format("MST")
	if got != before && got != after {
		t.Fatalf("currentTimezone(testSession, %s) = %q, want local timezone abbreviation %q or %q", goos, got, before, after)
	}
}
