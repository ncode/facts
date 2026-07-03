package engine

import (
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
	s.host = &fakeHostOS{platform: "plan9"}

	if got := currentOSRelease(s); got != nil {
		t.Fatalf("currentOSRelease(plan9) = %#v, want nil", got)
	}
}

func TestCurrentOSReleaseUnsupportedTargetOmitsOSRelease(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"solaris", "aix"} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			s := NewSession()
			s.host = &fakeHostOS{platform: goos, runOutput: "5.11\n"}
			if got := currentOSRelease(s); got != nil {
				t.Fatalf("currentOSRelease(%s) = %#v, want nil", goos, got)
			}
		})
	}
}

func TestCurrentLoadAveragesPlan9OmitsLoadAverages(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{
		platform: "plan9",
		files: map[string][]byte{
			"/proc/loadavg": []byte("0.00 0.01 0.02\n"),
		},
		runOutputs: map[string]string{
			fakeRunKey("uptime"): "cirno up 0 days, 01:35:26",
		},
	}
	s := NewSession()
	s.host = host

	got := currentLoadAverages(s)
	if got != nil {
		t.Fatalf("currentLoadAverages(plan9) = %#v, want nil", got)
	}
	if len(host.readFileCalls) != 0 || len(host.runCalls) != 0 {
		t.Fatalf("currentLoadAverages(plan9) consulted host: reads=%#v runs=%#v", host.readFileCalls, host.runCalls)
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

	host := &fakeHostOS{
		platform:        "plan9",
		emptyRunDefault: true,
		runOutputs: map[string]string{
			fakeRunKey("uptime"): "cirno up 0 days, 01:35:26",
		},
	}
	s := NewSession()
	s.host = host

	got := currentUptimeInfo(s, nil)

	want := uptimeInfo{Duration: 5726 * 1_000_000_000, Known: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentUptimeInfo(plan9) = %#v, want %#v", got, want)
	}
}
