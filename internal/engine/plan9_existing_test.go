package engine

import (
	"os"
	"reflect"
	"testing"
)

func TestPlan9IdentityNamesUseCanonicalSpelling(t *testing.T) {
	t.Parallel()

	if got := osFamily("plan9", linuxDistro{}); got != "Plan 9" {
		t.Fatalf("osFamily(plan9) = %q, want Plan 9", got)
	}
	if got := osName("plan9", linuxDistro{}); got != "Plan 9" {
		t.Fatalf("osName(plan9) = %q, want Plan 9", got)
	}
	if got := kernelName("plan9"); got != "Plan 9" {
		t.Fatalf("kernelName(plan9) = %q, want Plan 9", got)
	}
}

func TestCurrentOSReleasePlan9OmitsOSVersionProtocol(t *testing.T) {
	t.Parallel()

	s := NewSession()
	s.host = &fakeHostOS{runOutput: "2000\n"}
	readFile := func(path string) ([]byte, error) {
		if path != "/dev/osversion" {
			return nil, os.ErrNotExist
		}
		return []byte("2000\n"), nil
	}

	if got := currentOSRelease(s, "plan9", readFile, func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("currentOSRelease(plan9) = %#v, want nil", got)
	}
}

func TestCurrentLoadAveragesPlan9OmitsLoadAverages(t *testing.T) {
	t.Parallel()

	got := currentLoadAverages("plan9", func(string) ([]byte, error) {
		return []byte("0.00 0.01 0.02\n"), nil
	}, func(string, ...string) string {
		return "cirno up 0 days, 01:35:26"
	})
	if got != nil {
		t.Fatalf("currentLoadAverages(plan9) = %#v, want nil", got)
	}
}

func TestParseUptimeCommandSecondsPlan9Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "days and clock", input: "cirno up 0 days, 01:35:26", want: 5726},
		{name: "clock only", input: "cirno up 23:15:17", want: 83717},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseUptimeCommandSeconds(tt.input); got != tt.want {
				t.Fatalf("parseUptimeCommandSeconds(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCurrentUptimeInfoPlan9UsesNativeUptimeFormat(t *testing.T) {
	t.Parallel()

	got := currentUptimeInfo(testSession, "plan9", func(path string) ([]byte, error) {
		return nil, os.ErrNotExist
	}, func(name string, args ...string) string {
		if name != "uptime" {
			return ""
		}
		return "cirno up 0 days, 01:35:26"
	}, nil)

	want := uptimeInfo{Duration: 5726 * 1_000_000_000, Known: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentUptimeInfo(plan9) = %#v, want %#v", got, want)
	}
}
