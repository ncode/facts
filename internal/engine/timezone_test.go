package engine

import (
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

func assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t *testing.T, goos string) {
	t.Helper()

	before := time.Now().Format("MST")
	got := currentTimezone(testSession, goos)
	after := time.Now().Format("MST")
	if got != before && got != after {
		t.Fatalf("currentTimezone(testSession, %s) = %q, want local timezone abbreviation %q or %q", goos, got, before, after)
	}
}
