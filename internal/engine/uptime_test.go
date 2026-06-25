package engine

import (
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestCoreFacts_includeSystemUptime(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	systemUptime, ok := collection["system_uptime"].(map[string]any)
	if !ok {
		t.Fatalf("system_uptime fact = %#v, want map", collection["system_uptime"])
	}

	for _, key := range []string{"days", "hours", "seconds", "uptime"} {
		if systemUptime[key] == nil {
			t.Fatalf("system_uptime = %#v, want key %q", systemUptime, key)
		}
	}
	if collection["uptime_hours"] != nil {
		t.Fatalf("uptime_hours = %#v, want legacy alias hidden from core collection", collection["uptime_hours"])
	}
}

func TestUptimeString_matchesRubyFacterFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		seconds int
		want    string
	}{
		{name: "less than one minute", seconds: 20, want: "0:00 hours"},
		{name: "minutes", seconds: 620, want: "0:10 hours"},
		{name: "hours", seconds: 11_420, want: "3:10 hours"},
		{name: "one day", seconds: 97_820, want: "1 day"},
		{name: "multiple days", seconds: 184_220, want: "2 days"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := uptimeString(uptimeInfo{Duration: time.Duration(tt.seconds) * time.Second, Known: true})
			if got != tt.want {
				t.Fatalf("uptimeString(%d seconds) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestUptimeStringReturnsUnknownWhenSecondsAreUnknown(t *testing.T) {
	t.Parallel()

	if got := uptimeString(uptimeInfo{}); got != "unknown" {
		t.Fatalf("uptimeString(unknown) = %q, want unknown", got)
	}
}

func TestUptimeFactsOmitNumericFieldsWhenUptimeUnknown(t *testing.T) {
	got := Collection(uptimeFacts(uptimeInfo{}, emptyLoadAverages()))
	systemUptime, ok := got["system_uptime"].(map[string]any)
	if !ok {
		t.Fatalf("system_uptime = %#v, want map", got["system_uptime"])
	}
	if got := systemUptime["uptime"]; got != "unknown" {
		t.Fatalf("system_uptime.uptime = %#v, want unknown", got)
	}
	for _, key := range []string{"days", "hours", "seconds"} {
		if _, ok := systemUptime[key]; ok {
			t.Fatalf("system_uptime.%s present for unknown uptime: %#v", key, systemUptime)
		}
	}
}

func TestUptimeFactsUseInt64DurationFields(t *testing.T) {
	got := Collection(uptimeFacts(uptimeInfo{Duration: time.Duration(1<<33) * time.Second, Known: true}, emptyLoadAverages()))
	systemUptime := got["system_uptime"].(map[string]any)
	if seconds, ok := systemUptime["seconds"].(int64); !ok || seconds != int64(1<<33) {
		t.Fatalf("system_uptime.seconds = %#v, want int64 %d", systemUptime["seconds"], int64(1<<33))
	}
}

func TestUptimeCoreFactsUsesSessionPlatform(t *testing.T) {
	s := NewSessionContext(t.Context())
	s.host = &fakeHostOS{
		platform: "plan9",
		runOutputs: map[string]string{
			fakeRunKey("uptime"): "10:00AM up 1 day, 2:03, 1 user\n",
		},
	}

	got := Collection(uptimeCoreFacts(s))
	systemUptime, ok := got["system_uptime"].(map[string]any)
	if !ok {
		t.Fatalf("system_uptime = %#v, want map", got["system_uptime"])
	}
	if got := systemUptime["seconds"]; got != int64(93_780) {
		t.Fatalf("system_uptime.seconds = %#v, want 93780", got)
	}
	if _, ok := got["load_averages"]; ok {
		t.Fatalf("load_averages present for Plan 9 uptime facts: %#v", got)
	}
}

func TestCurrentUptimeInfoUsesPID1ElapsedTimeForKubernetes(t *testing.T) {
	s := NewSession()
	s.host = &fakeHostOS{
		platform: "linux",
		files: map[string][]byte{
			"/proc/1/cgroup": []byte("0::/kubepods.slice/pod123\n"),
		},
	}
	run := func(name string, args ...string) string {
		if name == "ps" && reflect.DeepEqual(args, []string{"-o", "etime=", "-p", "1"}) {
			return "01:02"
		}
		return ""
	}

	got := currentUptimeInfo(s, "linux", s.readFile, run, time.Now)
	want := uptimeInfo{Duration: 62 * time.Second, Known: true}
	if got != want {
		t.Fatalf("currentUptimeInfo(kubernetes) = %#v, want %#v", got, want)
	}
}

func TestParseUptimeCommandSeconds_matchesRubyFacterUptimeParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "days hours and minutes", input: "10:00AM up 2 days, 1:00, 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "days and hours", input: "10:00AM up 2 days, 1 hr(s), 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "days and minutes", input: "10:00AM up 2 days, 60 min(s), 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "days", input: "10:00AM up 2 days, 1 user, load average: 1.00, 0.75, 0.66", want: 172_800},
		{name: "hours and minutes", input: "10:00AM up 49:00, 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "hours", input: "10:00AM up 49 hr(s), 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "minutes", input: "10:00AM up 2940 mins, 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "invalid", input: "running for 2 days", want: 0},
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

func TestParseDockerElapsedTimeSecondsMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "less than one minute", input: "20", want: 20},
		{name: "minutes", input: "10:20", want: 620},
		{name: "hours", input: "3:10:20", want: 11_420},
		{name: "one day", input: "1-3:10:20", want: 97_820},
		{name: "multiple days", input: "2-3:10:20", want: 184_220},
		{name: "invalid", input: "not uptime", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseDockerElapsedTimeSeconds(tt.input); got != tt.want {
				t.Fatalf("parseDockerElapsedTimeSeconds(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCurrentUptimeFallsBackToKernelBoottime(t *testing.T) {
	t.Parallel()

	readFile := func(path string) ([]byte, error) {
		if path != "/proc/uptime" {
			t.Fatalf("readFile path = %q, want /proc/uptime", path)
		}
		return nil, os.ErrNotExist
	}
	run := func(name string, args ...string) string {
		if name != "sysctl" || !reflect.DeepEqual(args, []string{"-n", "kern.boottime"}) {
			t.Fatalf("run = %s %v, want sysctl -n kern.boottime", name, args)
		}
		return "{ sec = 60, usec = 0 } Tue Oct 10 10:59:00 2019"
	}
	now := func() time.Time { return time.Unix(120, 0) }

	got := currentUptime(testSession, "freebsd", readFile, run, now)
	if got != time.Minute {
		t.Fatalf("currentUptime(testSession) = %s, want 1m0s", got)
	}
}

func TestCurrentLinuxUptimeUsesDockerPIDOneElapsedTime(t *testing.T) {
	t.Parallel()

	got := currentLinuxUptimeInfo(func(path string) ([]byte, error) {
		t.Fatalf("currentLinuxUptimeInfo() read %q, want Docker ps only", path)
		return nil, os.ErrNotExist
	}, func(name string, args ...string) string {
		if name != "ps" || !reflect.DeepEqual(args, []string{"-o", "etime=", "-p", "1"}) {
			t.Fatalf("run = %s %v, want ps -o etime= -p 1", name, args)
		}
		return "1-3:10:20"
	}, time.Now, true)

	if !got.Known || got.Duration != 97_820*time.Second {
		t.Fatalf("currentLinuxUptimeInfo() = %#v, want known 97820s", got)
	}
}

func TestCurrentLinuxUptimeFallsBackWhenDockerElapsedTimeInvalid(t *testing.T) {
	t.Parallel()

	got := currentLinuxUptimeInfo(func(path string) ([]byte, error) {
		if path != "/proc/uptime" {
			t.Fatalf("readFile path = %q, want /proc/uptime", path)
		}
		return []byte("60.00 10.00"), nil
	}, func(name string, args ...string) string {
		if name != "ps" {
			t.Fatalf("run = %s %v, want ps for Docker fallback", name, args)
		}
		return "invalid"
	}, time.Now, true)

	if !got.Known || got.Duration != time.Minute {
		t.Fatalf("currentLinuxUptimeInfo() = %#v, want known 1m", got)
	}
}

func TestCurrentUptimeInfoMarksMissingSourcesUnknown(t *testing.T) {
	t.Parallel()

	readFile := func(path string) ([]byte, error) {
		if path != "/proc/uptime" {
			t.Fatalf("readFile path = %q, want /proc/uptime", path)
		}
		return nil, os.ErrNotExist
	}
	run := func(name string, args ...string) string {
		switch name {
		case "sysctl":
			return ""
		case "uptime":
			return "running for a while"
		default:
			t.Fatalf("unexpected command %q %v", name, args)
			return ""
		}
	}

	got := currentUptimeInfo(testSession, "freebsd", readFile, run, time.Now)
	if got.Known || got.Duration != 0 {
		t.Fatalf("currentUptimeInfo(testSession) = %#v, want unknown zero duration", got)
	}
}

func TestCurrentUptimeInfoBSDsFallBackToUptimeCommand(t *testing.T) {
	t.Parallel()

	readFile := func(string) ([]byte, error) {
		return nil, os.ErrNotExist
	}
	for _, goos := range []string{"openbsd", "netbsd"} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			got := currentUptimeInfo(testSession, goos, readFile, func(name string, args ...string) string {
				switch name {
				case "sysctl":
					return ""
				case "uptime":
					return "10:00AM up 1 day, 2:03, 1 user, load averages: 0.01, 0.02, 0.03"
				default:
					t.Fatalf("run = %s %#v, want sysctl or uptime", name, args)
					return ""
				}
			}, time.Now)
			want := uptimeInfo{Duration: 26*time.Hour + 3*time.Minute, Known: true}
			if got != want {
				t.Fatalf("currentUptimeInfo(%s) = %#v, want %#v", goos, got, want)
			}
		})
	}
}

func TestCurrentUptimeUsesWindowsWMITimes(t *testing.T) {
	t.Parallel()

	got := currentUptime(testSession, "windows", func(path string) ([]byte, error) {
		t.Fatalf("currentUptime(testSession, windows) read %q, want WMI only", path)
		return nil, os.ErrNotExist
	}, func(name string, args ...string) string {
		if name != "wmic" {
			t.Fatalf("command = %q %v, want wmic", name, args)
		}
		wantArgs := []string{"os", "get", "LocalDateTime,LastBootUpTime", "/value"}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Fatalf("wmic args = %#v, want %#v", args, wantArgs)
		}
		return "LocalDateTime=20010203045006+0700\r\nLastBootUpTime=20010203030506+0700\r\n"
	}, time.Now)

	if got != 105*time.Minute {
		t.Fatalf("currentUptime(testSession, windows) = %s, want 1h45m0s", got)
	}
}

func TestCurrentWindowsUptimeSkipsNonWindows(t *testing.T) {
	t.Parallel()

	got := currentWindowsUptime("linux", func(string, ...string) string {
		t.Fatal("currentWindowsUptime(non-windows) ran command")
		return ""
	}, discardLog())
	if got != (uptimeInfo{}) {
		t.Fatalf("currentWindowsUptime(non-windows) = %#v, want empty", got)
	}
}

func TestCurrentUptimeReturnsZeroForInvalidWindowsWMITimes(t *testing.T) {
	t.Parallel()

	got := currentUptime(testSession, "windows", func(string) ([]byte, error) {
		t.Fatal("currentUptime(testSession, windows) read file, want WMI only")
		return nil, os.ErrNotExist
	}, func(string, ...string) string {
		return "LocalDateTime=20010201110506+0700\r\nLastBootUpTime=20010201120506+0700\r\n"
	}, time.Now)

	if got != 0 {
		t.Fatalf("currentUptime(testSession, windows invalid times) = %s, want 0", got)
	}
}

func TestCurrentWindowsUptimeInfoMarksInvalidWMITimesUnknown(t *testing.T) {
	t.Parallel()

	got := currentUptimeInfo(testSession, "windows", func(string) ([]byte, error) {
		t.Fatal("currentUptimeInfo(testSession, windows) read file, want WMI only")
		return nil, os.ErrNotExist
	}, func(string, ...string) string {
		return "LocalDateTime=20010201110506+0700\r\nLastBootUpTime=20010201120506+0700\r\n"
	}, time.Now)

	if got.Known || got.Duration != 0 {
		t.Fatalf("currentUptimeInfo(testSession, windows invalid times) = %#v, want unknown zero duration", got)
	}
}

func TestCurrentWindowsUptimeInfoLogsNoResultDiagnosticsLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	s := NewSession()
	s.logger = captureLogger(&debugMessages, nil, nil)

	got := currentUptimeInfo(s, "windows", func(string) ([]byte, error) {
		t.Fatal("currentUptimeInfo(testSession, windows) read file, want WMI only")
		return nil, os.ErrNotExist
	}, func(string, ...string) string {
		return ""
	}, time.Now)

	if got.Known || got.Duration != 0 {
		t.Fatalf("currentUptimeInfo(testSession, windows empty WMI) = %#v, want unknown zero duration", got)
	}
	want := []string{
		"WMI query returned no resultsfor Win32_OperatingSystem with values LocalDateTime and LastBootUpTime.",
		"Unable to determine system uptime!",
	}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestCurrentWindowsUptimeInfoLogsUnparseableWMITimes(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{name: "invalid local time", output: "LocalDateTime=bad\r\nLastBootUpTime=20010201120506+0700\r\n"},
		{name: "invalid boot time", output: "LocalDateTime=20010201130506+0700\r\nLastBootUpTime=bad\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var debugMessages []string
			s := NewSession()
			s.logger = captureLogger(&debugMessages, nil, nil)

			got := currentWindowsUptime("windows", func(string, ...string) string {
				return tt.output
			}, s.logr())
			if got.Known || got.Duration != 0 {
				t.Fatalf("currentWindowsUptime() = %#v, want unknown zero duration", got)
			}
			want := []string{"Unable to determine system uptime!"}
			if !reflect.DeepEqual(debugMessages, want) {
				t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
			}
		})
	}
}

func TestCurrentWindowsUptimeInfoLogsInvalidDurationLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	s := NewSession()
	s.logger = captureLogger(&debugMessages, nil, nil)

	got := currentUptimeInfo(s, "windows", func(string) ([]byte, error) {
		t.Fatal("currentUptimeInfo(testSession, windows) read file, want WMI only")
		return nil, os.ErrNotExist
	}, func(string, ...string) string {
		return "LocalDateTime=20010201110506+0700\r\nLastBootUpTime=20010201120506+0700\r\n"
	}, time.Now)

	if got.Known || got.Duration != 0 {
		t.Fatalf("currentUptimeInfo(testSession, windows invalid times) = %#v, want unknown zero duration", got)
	}
	want := []string{"Unable to determine system uptime!"}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestParseWindowsWMITimeAcceptsCIMDatetimeOffsetMinutes(t *testing.T) {
	t.Parallel()

	got, ok := parseWindowsWMITime("20010203040506.123456+060")
	if !ok {
		t.Fatal("parseWindowsWMITime() ok = false, want true")
	}
	want := time.Date(2001, 2, 3, 4, 5, 6, 0, time.FixedZone("", 60*60))
	if !got.Equal(want) {
		t.Fatalf("parseWindowsWMITime() = %s, want %s", got, want)
	}
}

func TestParseWindowsWMITimeRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"200102030405",
		"20010203040506.123456",
		"20010203040506+",
		"20010203040506+bad",
		"bad-date-time!!+0700",
	} {
		if got, ok := parseWindowsWMITime(input); ok || !got.IsZero() {
			t.Fatalf("parseWindowsWMITime(%q) = %s, %v, want zero false", input, got, ok)
		}
	}
}

func TestUptimeSourceParsersRejectMalformedInputs(t *testing.T) {
	t.Parallel()

	if got := uptimeFromProc(func(string) ([]byte, error) { return []byte(""), nil }); got != 0 {
		t.Fatalf("uptimeFromProc(empty) = %s, want 0", got)
	}
	if got := uptimeFromProc(func(string) ([]byte, error) { return []byte("not-a-number"), nil }); got != 0 {
		t.Fatalf("uptimeFromProc(invalid) = %s, want 0", got)
	}

	now := func() time.Time { return time.Unix(120, 0) }
	for _, input := range []string{"", "{ sec = 60 }", "{ sec = bad, usec = 0 }"} {
		if got := uptimeFromKernelBoottime(input, now); got != 0 {
			t.Fatalf("uptimeFromKernelBoottime(%q) = %s, want 0", input, got)
		}
	}
}

func TestCoreFacts_includeLoadAverages(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("load averages resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession))
	loadAverages, ok := collection["load_averages"].(map[string]any)
	if !ok {
		t.Fatalf("load_averages fact = %#v, want map", collection["load_averages"])
	}

	for _, key := range []string{"1m", "5m", "15m"} {
		value, ok := loadAverages[key].(float64)
		if !ok || value < 0 {
			t.Fatalf("load_averages[%q] = %#v, want non-negative float64", key, loadAverages[key])
		}
	}
}

func TestParseLinuxLoadAverages(t *testing.T) {
	got := parseLoadAverages("0.00 0.03 0.03 1/1153 20372")
	want := map[string]any{"1m": 0.00, "5m": 0.03, "15m": 0.03}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestParseMacOSLoadAverages(t *testing.T) {
	got := parseLoadAverages("{ 2.10 2.20 2.30 }")
	want := map[string]any{"1m": 2.10, "5m": 2.20, "15m": 2.30}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestCurrentLoadAverages_wiresBSDVMLoadavg(t *testing.T) {
	for _, goos := range []string{"freebsd", "openbsd", "netbsd", "dragonfly"} {
		t.Run(goos, func(t *testing.T) {
			got := currentLoadAverages(goos, nil, func(path string, args ...string) string {
				if path != "sysctl" || !reflect.DeepEqual(args, []string{"-n", "vm.loadavg"}) {
					t.Fatalf("run(%q, %#v), want sysctl -n vm.loadavg", path, args)
				}
				return "{ 0.01 0.02 0.03 }"
			})
			want := map[string]any{"1m": 0.01, "5m": 0.02, "15m": 0.03}

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("currentLoadAverages() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestCurrentLoadAverages_wiresIllumosUptime(t *testing.T) {
	got := currentLoadAverages("illumos", nil, func(path string, args ...string) string {
		if path != "uptime" || len(args) != 0 {
			t.Fatalf("run(%q, %#v), want uptime", path, args)
		}
		return "22:09:38    up  3:04,  0 users,  load average: 0.00, 0.01, 0.02"
	})
	want := map[string]any{"1m": 0.00, "5m": 0.01, "15m": 0.02}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLoadAverages(illumos) = %#v, want %#v", got, want)
	}
}

func TestCurrentLoadAverages_wiresDarwinVMLoadavg(t *testing.T) {
	got := currentLoadAverages("darwin", nil, func(path string, args ...string) string {
		if path != "sysctl" || !reflect.DeepEqual(args, []string{"-n", "vm.loadavg"}) {
			t.Fatalf("run(%q, %#v), want sysctl -n vm.loadavg", path, args)
		}
		return "{ 0.00 0.03 0.03 }"
	})
	want := map[string]any{"1m": 0.00, "5m": 0.03, "15m": 0.03}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestCurrentLoadAverages_linuxUnreadableProcLoadavgMatchesRubyResolver(t *testing.T) {
	got := currentLoadAverages("linux", func(path string) ([]byte, error) {
		if path != "/proc/loadavg" {
			t.Fatalf("readFile(%q), want /proc/loadavg", path)
		}
		return nil, os.ErrPermission
	}, nil)
	want := map[string]any{"1m": nil, "5m": nil, "15m": nil}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestParseLoadAveragesInvalidInput(t *testing.T) {
	got := parseLoadAverages("not enough")
	want := map[string]any{"1m": nil, "5m": nil, "15m": nil}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestUptimeCommandParsersRejectMalformedDurations(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"10:00AM up users",
		"10:00AM up trailing",
		"10:00AM up about days",
	} {
		if got := parseUptimeCommandSeconds(input); got != 0 {
			t.Fatalf("parseUptimeCommandSeconds(%q) = %d, want 0", input, got)
		}
	}

	for _, input := range []string{"bad-01:02:03", "bad:02", "01:bad", "bad:02:03", "01:bad:03", "01:02:bad", "01:02:03:04"} {
		if got := parseDockerElapsedTimeSeconds(input); got != 0 {
			t.Fatalf("parseDockerElapsedTimeSeconds(%q) = %d, want 0", input, got)
		}
	}

	for _, input := range []string{"bad:02", "01:bad"} {
		if hours, minutes, ok := parseUptimeHoursMinutes(input); ok || hours != 0 || minutes != 0 {
			t.Fatalf("parseUptimeHoursMinutes(%q) = %d, %d, %v, want zero false", input, hours, minutes, ok)
		}
	}
}

func TestLoadAverageParsersUseEmptyFallbacks(t *testing.T) {
	t.Parallel()

	want := emptyLoadAverages()
	cases := []struct {
		name string
		got  map[string]any
	}{
		{name: "bsd empty sysctl", got: currentLoadAverages("freebsd", nil, func(string, ...string) string { return "" })},
		{name: "illumos empty uptime", got: currentLoadAverages("illumos", nil, func(string, ...string) string { return "" })},
		{name: "unknown goos", got: currentLoadAverages("aix", nil, nil)},
		{name: "bad float", got: parseLoadAverages("0.01 bad 0.03")},
		{name: "illumos no marker", got: parseIllumosLoadAverages("22:09:38 up 3:04")},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, want) {
				t.Fatalf("%s = %#v, want %#v", tt.name, tt.got, want)
			}
		})
	}

	if got := currentLoadAverages("plan9", nil, nil); got != nil {
		t.Fatalf("currentLoadAverages(plan9) = %#v, want nil", got)
	}
}
