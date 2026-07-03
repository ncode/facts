package engine

import (
	"context"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestParseWindowsProcessorsMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"Name=Pretty_Name",
		"Architecture=0",
		"NumberOfLogicalProcessors=2",
		"NumberOfCores=2",
	}, "\r\n")

	got := parseWindowsProcessors(input, discardLog())
	want := processorInfo{
		ISA:            "x86",
		Models:         []string{"Pretty_Name"},
		LogicalCount:   2,
		PhysicalCount:  1,
		CoresPerSocket: 2,
		ThreadsPerCore: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWindowsProcessors() = %#v, want %#v", got, want)
	}
}

func TestParseWindowsProcessorsUsesRubyISATable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		architecture string
		want         string
	}{
		{architecture: "0", want: "x86"},
		{architecture: "1", want: "MIPS"},
		{architecture: "2", want: "Alpha"},
		{architecture: "3", want: "PowerPC"},
		{architecture: "5", want: "ARM"},
		{architecture: "6", want: "Itanium"},
		{architecture: "9", want: "x64"},
		{architecture: "12", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.architecture, func(t *testing.T) {
			input := strings.Join([]string{
				"Name=Pretty_Name",
				"Architecture=" + tt.architecture,
				"NumberOfLogicalProcessors=2",
				"NumberOfCores=2",
			}, "\r\n")

			got := parseWindowsProcessors(input, discardLog())
			if got.ISA != tt.want {
				t.Fatalf("ISA = %q, want %q", got.ISA, tt.want)
			}
		})
	}
}

func TestParseWindowsProcessorsFallsBackWhenLogicalCountIsZero(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"Name=Pretty_Name",
		"Architecture=0",
		"NumberOfLogicalProcessors=0",
		"NumberOfCores=2",
		"",
		"Name=Awesome_Name",
		"Architecture=10",
		"NumberOfLogicalProcessors=0",
		"NumberOfCores=2",
	}, "\r\n")

	got := parseWindowsProcessors(input, discardLog())
	want := processorInfo{
		ISA:            "x86",
		Models:         []string{"Pretty_Name", "Awesome_Name"},
		LogicalCount:   2,
		PhysicalCount:  2,
		CoresPerSocket: 2,
		ThreadsPerCore: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWindowsProcessors() = %#v, want %#v", got, want)
	}
}

func TestParseWindowsProcessorsHandlesMissingCoreCounts(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"Name=Pretty_Name",
		"Architecture=0",
		"NumberOfLogicalProcessors=2",
		"NumberOfCores=bad",
	}, "\r\n")

	got := parseWindowsProcessors(input, discardLog())
	want := processorInfo{
		ISA:           "x86",
		Models:        []string{"Pretty_Name"},
		LogicalCount:  2,
		PhysicalCount: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWindowsProcessors() = %#v, want %#v", got, want)
	}
}

func TestParseWindowsProcessorsLogsUnknownArchitectureLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	logger := captureLogger(&debugMessages, nil, nil)

	got := parseWindowsProcessors("Name=Pretty_Name\r\nArchitecture=10\r\nNumberOfLogicalProcessors=2\r\nNumberOfCores=2\r\n", logger)
	if got.ISA != "" {
		t.Fatalf("ISA = %q, want empty for unknown architecture", got.ISA)
	}
	want := []string{"Unable to determine processor type: unknown architecture"}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestCurrentWindowsProcessorsQueriesWMIC(t *testing.T) {
	t.Parallel()

	got := currentWindowsProcessors("windows", func(name string, args ...string) string {
		if name != "wmic" {
			t.Fatalf("command = %q %v, want wmic", name, args)
		}
		wantArgs := []string{"cpu", "get", "Name,Architecture,NumberOfLogicalProcessors,NumberOfCores", "/value"}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Fatalf("wmic args = %#v, want %#v", args, wantArgs)
		}
		return "Name=Pretty_Name\r\nArchitecture=0\r\nNumberOfLogicalProcessors=2\r\nNumberOfCores=2\r\n"
	}, discardLog())

	if got.LogicalCount != 2 || got.PhysicalCount != 1 || got.CoresPerSocket != 2 || got.ThreadsPerCore != 1 {
		t.Fatalf("currentWindowsProcessors() = %#v, want Ruby-compatible counts", got)
	}
	if got.ISA != "x86" {
		t.Fatalf("ISA = %q, want x86", got.ISA)
	}
	if !reflect.DeepEqual(got.Models, []string{"Pretty_Name"}) {
		t.Fatalf("Models = %#v, want Pretty_Name", got.Models)
	}
}

func TestCurrentProcessorInfoWiresWindowsWMICOutput(t *testing.T) {
	t.Parallel()

	got := currentProcessorInfo("windows", func(name string, args ...string) string {
		if name != "wmic" || !reflect.DeepEqual(args, []string{"cpu", "get", "Name,Architecture,NumberOfLogicalProcessors,NumberOfCores", "/value"}) {
			t.Fatalf("run(%q, %#v), want wmic processor query", name, args)
		}
		return "Name=Pretty_Name\r\nArchitecture=9\r\nNumberOfLogicalProcessors=4\r\nNumberOfCores=2\r\n"
	}, discardLog())

	if got.LogicalCount != 4 || got.CoresPerSocket != 2 || got.ThreadsPerCore != 2 || got.ISA != "x64" {
		t.Fatalf("currentProcessorInfo(windows) = %#v", got)
	}
}

func TestCurrentProcessorInfoSkipsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	got := currentProcessorInfo("plan9", func(string, ...string) string {
		t.Fatal("currentProcessorInfo(unsupported) ran command")
		return ""
	}, discardLog())
	if !reflect.DeepEqual(got, processorInfo{}) {
		t.Fatalf("currentProcessorInfo(unsupported) = %#v, want empty", got)
	}
}

func TestCurrentWindowsProcessorsLogsNoResultDiagnosticsLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	logger := captureLogger(&debugMessages, nil, nil)

	got := currentWindowsProcessors("windows", func(string, ...string) string { return "" }, logger)
	if !reflect.DeepEqual(got, processorInfo{}) {
		t.Fatalf("currentWindowsProcessors(empty WMI) = %#v, want empty processor info", got)
	}
	want := []string{"WMI query returned no resultsfor Win32_Processor with values Name, Architecture and NumberOfLogicalProcessors."}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestParseWindowsProcessorsOmitsUnknownISA(t *testing.T) {
	t.Parallel()

	got := parseWindowsProcessors("Name=Pretty_Name\r\nArchitecture=10\r\nNumberOfLogicalProcessors=2\r\nNumberOfCores=2\r\n", discardLog())
	if got.ISA != "" {
		t.Fatalf("ISA = %q, want empty for unknown architecture", got.ISA)
	}
}

func TestCurrentWindowsProcessorsSkipsNonWindows(t *testing.T) {
	t.Parallel()

	got := currentWindowsProcessors("linux", func(string, ...string) string {
		t.Fatal("currentWindowsProcessors(non-windows) ran command")
		return ""
	}, discardLog())
	if !reflect.DeepEqual(got, processorInfo{}) {
		t.Fatalf("currentWindowsProcessors(linux) = %#v, want empty", got)
	}
}

func TestCurrentProcessorInfoWiresFreeBSDSysctlOutput(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	run := func(path string, args ...string) string {
		if path != "sysctl" || len(args) != 2 || args[0] != "-n" {
			t.Fatalf("run(%q, %#v), want sysctl -n <name>", path, args)
		}
		seen[args[1]] = true
		switch args[1] {
		case "hw.ncpu":
			return "2\n"
		case "hw.model":
			return "Intel(r) Xeon(r) Gold 6138 CPU @ 2.00GHz\n"
		case "hw.clockrate":
			return "2592\n"
		default:
			t.Fatalf("unexpected sysctl name %q", args[1])
			return ""
		}
	}

	got := currentProcessorInfo("freebsd", run, discardLog())
	wantModels := []string{
		"Intel(r) Xeon(r) Gold 6138 CPU @ 2.00GHz",
		"Intel(r) Xeon(r) Gold 6138 CPU @ 2.00GHz",
	}
	if got.LogicalCount != 2 || got.SpeedHz != 2592000000 || !reflect.DeepEqual(got.Models, wantModels) {
		t.Fatalf("currentProcessorInfo() = %#v", got)
	}
	for _, name := range []string{"hw.ncpu", "hw.model", "hw.clockrate"} {
		if !seen[name] {
			t.Fatalf("currentProcessorInfo() did not query %s", name)
		}
	}
}

func TestProcessorProbesUseSessionPlatformForFreeBSD(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "freebsd",
		runOutputs: map[string]string{
			fakeRunKey("sysctl", "-n", "hw.ncpu"):      "4\n",
			fakeRunKey("sysctl", "-n", "hw.model"):     "Intel CPU\n",
			fakeRunKey("sysctl", "-n", "hw.clockrate"): "2400\n",
		},
	}

	info := probePlatformProcessorInfo(s)
	if info.LogicalCount != 4 || info.SpeedHz != 2_400_000_000 || info.CoresPerSocket != 4 || info.ThreadsPerCore != 1 {
		t.Fatalf("probePlatformProcessorInfo() = %#v", info)
	}

	if got := probeProcessorSpeed(s); got != "2.40 GHz" {
		t.Fatalf("probeProcessorSpeed() = %q, want 2.40 GHz", got)
	}
	wantModels := []string{"Intel CPU", "Intel CPU", "Intel CPU", "Intel CPU"}
	if got := probeProcessorModels(s); !reflect.DeepEqual(got, wantModels) {
		t.Fatalf("probeProcessorModels() = %#v, want %#v", got, wantModels)
	}
	if cores, threads := probeProcessorTopology(s); cores != 4 || threads != 1 {
		t.Fatalf("probeProcessorTopology() = %d, %d; want 4, 1", cores, threads)
	}
}

func TestProcessorProbesUseSessionPlatformForDarwin(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "darwin",
		runOutputs: map[string]string{
			fakeRunKey("sysctl",
				"hw.logicalcpu_max",
				"hw.physicalcpu_max",
				"machdep.cpu.brand_string",
				"hw.cpufrequency_max",
				"machdep.cpu.core_count",
				"machdep.cpu.thread_count",
			): strings.Join([]string{
				"hw.logicalcpu_max: 8",
				"hw.physicalcpu_max: 4",
				"machdep.cpu.brand_string: Apple M2",
				"hw.cpufrequency_max: 3200000000",
				"machdep.cpu.core_count: 4",
				"machdep.cpu.thread_count: 8",
			}, "\n"),
		},
	}

	if got := probeProcessorSpeed(s); got != "3.20 GHz" {
		t.Fatalf("probeProcessorSpeed() = %q, want 3.20 GHz", got)
	}
	models := probeProcessorModels(s)
	if len(models) != 8 || models[0] != "Apple M2" || models[7] != "Apple M2" {
		t.Fatalf("probeProcessorModels() = %#v, want eight Apple M2 entries", models)
	}
	if cores, threads := probeProcessorTopology(s); cores != 4 || threads != 2 {
		t.Fatalf("probeProcessorTopology() = %d, %d; want 4, 2", cores, threads)
	}
}

func TestProcessorProbesUseSessionPlatformForWindows(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "windows",
		runOutputs: map[string]string{
			fakeRunKey("wmic", "cpu", "get", "Name,Architecture,NumberOfLogicalProcessors,NumberOfCores", "/value"): "Name=Pretty CPU\r\nArchitecture=9\r\nNumberOfLogicalProcessors=4\r\nNumberOfCores=2\r\n",
		},
	}

	info := probePlatformProcessorInfo(s)
	if info.ISA != "x64" || info.LogicalCount != 4 || info.PhysicalCount != 1 || info.CoresPerSocket != 2 || info.ThreadsPerCore != 2 {
		t.Fatalf("probePlatformProcessorInfo() = %#v", info)
	}
	if got := probeProcessorModels(s); !reflect.DeepEqual(got, []string{"Pretty CPU"}) {
		t.Fatalf("probeProcessorModels() = %#v, want Pretty CPU", got)
	}
	if cores, threads := probeProcessorTopology(s); cores != 2 || threads != 2 {
		t.Fatalf("probeProcessorTopology() = %d, %d; want 2, 2", cores, threads)
	}
}

func TestProcessorProbesUseSessionPlatformForLinux(t *testing.T) {
	cpuinfo := strings.Join([]string{
		"processor\t: 0",
		"model name\t: Pretty CPU",
		"cpu MHz\t\t: 1800.000",
		"siblings\t: 4",
		"cpu cores\t: 2",
		"flags\t\t: fpu cx8 cmov mmx fxsr sse2 syscall lm cx16 lahf_lm popcnt sse4_1 sse4_2 ssse3 abm avx avx2 bmi1 bmi2 f16c fma movbe xsave",
		"processor\t: 1",
		"model name\t: Pretty CPU",
	}, "\n")
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "linux",
		files: map[string][]byte{
			"/proc/cpuinfo": []byte(cpuinfo),
		},
		runOutputs: map[string]string{
			fakeRunKey("uname", "-m"): "x86_64\n",
		},
	}

	if got := probeProcessorSpeed(s); got != "1.80 GHz" {
		t.Fatalf("probeProcessorSpeed() = %q, want 1.80 GHz", got)
	}
	if got := probeProcessorModels(s); !reflect.DeepEqual(got, []string{"Pretty CPU", "Pretty CPU"}) {
		t.Fatalf("probeProcessorModels() = %#v, want two Pretty CPU entries", got)
	}
	if cores, threads := probeProcessorTopology(s); cores != 2 || threads != 2 {
		t.Fatalf("probeProcessorTopology() = %d, %d; want 2, 2", cores, threads)
	}
	wantExtensions := []string{"x86_64", "x86_64-v1", "x86_64-v2", "x86_64-v3"}
	if got := probeProcessorExtensions(s); !reflect.DeepEqual(got, wantExtensions) {
		t.Fatalf("probeProcessorExtensions() = %#v, want %#v", got, wantExtensions)
	}
}

func TestProbePlatformProcessorInfoUsesSessionPlatformForPlan9(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "plan9",
		files: map[string][]byte{
			"/dev/sysstat": []byte("0 1 2\n\n1 2 3\n"),
			"/dev/cputype": []byte("\n"),
			"/dev/archctl": []byte(plan9Fixture(t, "archctl")),
		},
	}

	got := probePlatformProcessorInfo(s)
	want := processorInfo{
		LogicalCount: 2,
		Models:       []string{"Core 2/Xeon 3600", "Core 2/Xeon 3600"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("probePlatformProcessorInfo() = %#v, want %#v", got, want)
	}
}

func TestProcessorsCoreFactsUsesSessionPlatformForISAFallback(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "windows",
		runOutputs: map[string]string{
			fakeRunKey("uname", "-m"): "x86_64\n",
			fakeRunKey("wmic", "cpu", "get", "Name,Architecture,NumberOfLogicalProcessors,NumberOfCores", "/value"): "Name=Pretty CPU\r\nArchitecture=10\r\nNumberOfLogicalProcessors=4\r\nNumberOfCores=2\r\n",
		},
	}

	got := Collection(processorsCoreFacts(s))
	processors, ok := got["processors"].(map[string]any)
	if !ok {
		t.Fatalf("processors facts = %#v, want map", got["processors"])
	}
	want := architectureName("windows", windowsHardwareFromGoArch(runtime.GOARCH))
	if processors["isa"] != want {
		t.Fatalf("processors.isa = %#v, want Windows-normalized fallback %#v", processors["isa"], want)
	}
}

func TestCurrentProcessorInfoWiresGenericBSDSysctlOutput(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"netbsd", "openbsd"} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			seen := map[string]bool{}
			run := func(path string, args ...string) string {
				if path != "sysctl" || len(args) != 2 || args[0] != "-n" {
					t.Fatalf("run(%q, %#v), want sysctl -n <name>", path, args)
				}
				seen[args[1]] = true
				switch args[1] {
				case "hw.ncpu":
					return "2\n"
				case "hw.model":
					return "netbsd,generic-acpi\n"
				case "hw.cpuspeed":
					return "2400\n"
				default:
					t.Fatalf("unexpected sysctl name %q", args[1])
					return ""
				}
			}

			got := currentProcessorInfo(goos, run, discardLog())
			wantModels := []string{"netbsd,generic-acpi", "netbsd,generic-acpi"}
			if got.LogicalCount != 2 || got.SpeedHz != 2400000000 || !reflect.DeepEqual(got.Models, wantModels) {
				t.Fatalf("currentProcessorInfo(%s) = %#v", goos, got)
			}
			for _, name := range []string{"hw.ncpu", "hw.model", "hw.cpuspeed"} {
				if !seen[name] {
					t.Fatalf("currentProcessorInfo(%s) did not query %s", goos, name)
				}
			}
		})
	}
}

func TestCurrentProcessorInfoWiresDragonFlySysctlOutput(t *testing.T) {
	t.Parallel()

	got := currentProcessorInfo("dragonfly", func(path string, args ...string) string {
		if path != "sysctl" || len(args) != 2 || args[0] != "-n" {
			t.Fatalf("run(%q, %#v), want sysctl -n <name>", path, args)
		}
		switch args[1] {
		case "hw.ncpu":
			return "2\n"
		case "hw.model":
			return "Intel(R) Core(TM) i7-7700 CPU @ 3.60GHz\n"
		case "hw.clockrate":
			return "3600\n"
		default:
			t.Fatalf("unexpected sysctl name %q", args[1])
			return ""
		}
	}, discardLog())

	wantModels := []string{
		"Intel(R) Core(TM) i7-7700 CPU @ 3.60GHz",
		"Intel(R) Core(TM) i7-7700 CPU @ 3.60GHz",
	}
	if got.LogicalCount != 2 || got.SpeedHz != 3_600_000_000 || !reflect.DeepEqual(got.Models, wantModels) {
		t.Fatalf("currentProcessorInfo(dragonfly) = %#v", got)
	}
}

func TestParseFreeBSDProcessorsRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	got := parseFreeBSDProcessors("not-a-count\n", "Intel CPU\n", "not-a-speed\n")
	if !reflect.DeepEqual(got, processorInfo{}) {
		t.Fatalf("parseFreeBSDProcessors(invalid) = %#v, want empty processor info", got)
	}
}

func TestCurrentProcessorInfoWiresIllumosPsrinfoOutput(t *testing.T) {
	t.Parallel()

	got := currentProcessorInfo("illumos", func(path string, args ...string) string {
		if path != "psrinfo" || !reflect.DeepEqual(args, []string{"-pv"}) {
			t.Fatalf("run(%q, %#v), want psrinfo -pv", path, args)
		}
		return `The physical processor has 1 virtual processor (0)
  x86 (GenuineIntel 906E9 family 6 model 158 step 9 clock 3600 MHz)
	Intel(r) Core(tm) i7-7700 CPU @ 3.60GHz
The physical processor has 1 virtual processor (1)
  x86 (GenuineIntel 906E9 family 6 model 158 step 9 clock 3600 MHz)
	Intel(r) Core(tm) i7-7700 CPU @ 3.60GHz`
	}, discardLog())

	wantModels := []string{
		"Intel(r) Core(tm) i7-7700 CPU @ 3.60GHz",
		"Intel(r) Core(tm) i7-7700 CPU @ 3.60GHz",
	}
	if got.LogicalCount != 2 || got.PhysicalCount != 2 || got.SpeedHz != 3_600_000_000 || !reflect.DeepEqual(got.Models, wantModels) {
		t.Fatalf("currentProcessorInfo(illumos) = %#v", got)
	}
}

func TestCurrentProcessorInfoIllumosParsesCoreAndThreadCounts(t *testing.T) {
	t.Parallel()

	got := currentProcessorInfo("illumos", func(path string, args ...string) string {
		return `The physical processor has 2 cores and 4 virtual processors (0-3)
  x86 (GenuineIntel 906E9 family 6 model 158 step 9 clock 3600 MHz)
	Intel(r) Core(tm) i7-7700 CPU @ 3.60GHz`
	}, discardLog())

	if got.LogicalCount != 4 || got.PhysicalCount != 1 || got.CoresPerSocket != 2 || got.ThreadsPerCore != 2 || len(got.Models) != 4 {
		t.Fatalf("currentProcessorInfo(illumos) = %#v", got)
	}
}

func TestParseIllumosProcessorsInfersCountsFromClockedModels(t *testing.T) {
	t.Parallel()

	got := parseIllumosProcessors(`
  x86 (GenuineIntel 906E9 family 6 model 158 step 9 clock 3600 MHz)
	Intel(r) Core(tm) i7-7700 CPU @ 3.60GHz
`)
	want := processorInfo{
		ISA:            "x86",
		SpeedHz:        3_600_000_000,
		Models:         []string{"Intel(r) Core(tm) i7-7700 CPU @ 3.60GHz"},
		LogicalCount:   1,
		PhysicalCount:  1,
		CoresPerSocket: 1,
		ThreadsPerCore: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseIllumosProcessors() = %#v, want %#v", got, want)
	}
}

func TestCurrentProcessorISAUsesOpenBSDUnameProcessor(t *testing.T) {
	s := NewSession()
	host := &fakeHostOS{runOutputs: map[string]string{fakeRunKey("uname", "-p"): "i386\n"}}
	s.host = host

	got := currentProcessorISA(s, "openbsd", "amd64")

	if got != "i386" {
		t.Fatalf("currentProcessorISA(openbsd) = %q, want i386", got)
	}
	want := []fakeHostRunCall{{name: "uname", args: []string{"-p"}}}
	if !reflect.DeepEqual(host.runCalls, want) {
		t.Fatalf("commands = %#v, want uname -p", host.runCalls)
	}
}

func TestCurrentProcessorISAFallsBackWhenUnameProcessorIsUnknown(t *testing.T) {
	t.Parallel()

	for _, output := range []string{"", "unknown\n"} {
		s := NewSession()
		s.host = &fakeHostOS{runOutputs: map[string]string{fakeRunKey("uname", "-p"): output}}
		got := currentProcessorISA(s, "linux", "x86_64")
		if got != "x86_64" {
			t.Fatalf("currentProcessorISA(%q) = %q, want fallback", output, got)
		}
	}
}

func TestCurrentProcessorISAWindowsFallsBackWhenWMIHasNoISA(t *testing.T) {
	s := NewSession()
	s.host = &fakeHostOS{platform: "windows", runOutput: ""}

	if got := currentProcessorISA(s, "windows", "amd64"); got != "amd64" {
		t.Fatalf("currentProcessorISA(windows) = %q, want amd64 fallback", got)
	}
}

func TestCoreFacts_processorSpeedOmittedWhenProbeYieldsNothing(t *testing.T) {
	collection := Collection(CoreFacts(testSession, nil))
	processors, ok := collection["processors"].(map[string]any)
	if !ok {
		t.Fatalf("processors fact = %#v, want map", collection["processors"])
	}
	speed, ok := processors["speed"]
	if !ok {
		return // unresolvable speed (e.g. Apple Silicon) is absent, matching Ruby
	}
	if got, isString := speed.(string); !isString || got == "" {
		t.Fatalf("processors.speed = %#v, want non-empty string when present", speed)
	}
}

func TestCurrentProcessorInfoDarwinMatchesRubyMacOSResolver(t *testing.T) {
	output := strings.Join([]string{
		"hw.logicalcpu_max: 4",
		"hw.physicalcpu_max: 1",
		"machdep.cpu.brand_string: Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
		"hw.cpufrequency_max: 2300000000",
		"machdep.cpu.core_count: 4",
		"machdep.cpu.thread_count: 4",
	}, "\n")
	got := currentProcessorInfo("darwin", func(name string, args ...string) string {
		wantArgs := []string{
			"hw.logicalcpu_max",
			"hw.physicalcpu_max",
			"machdep.cpu.brand_string",
			"hw.cpufrequency_max",
			"machdep.cpu.core_count",
			"machdep.cpu.thread_count",
		}
		if name != "sysctl" || !reflect.DeepEqual(args, wantArgs) {
			t.Fatalf("command = %s %#v, want sysctl %#v", name, args, wantArgs)
		}
		return output
	}, discardLog())

	want := processorInfo{
		SpeedHz:        2_300_000_000,
		LogicalCount:   4,
		PhysicalCount:  1,
		CoresPerSocket: 4,
		ThreadsPerCore: 1,
		Models: []string{
			"Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
			"Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
			"Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
			"Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentProcessorInfo(darwin) = %#v, want %#v", got, want)
	}
}

func TestParseDarwinProcessorsKeepsRubyCoreCountAsCoresPerSocket(t *testing.T) {
	input := strings.Join([]string{
		"hw.logicalcpu_max: 16",
		"hw.physicalcpu_max: 2",
		"machdep.cpu.brand_string: Apple CPU",
		"hw.cpufrequency_max: 0",
		"machdep.cpu.core_count: 8",
		"machdep.cpu.thread_count: 16",
	}, "\n")

	got := parseDarwinProcessors(input)
	if got.CoresPerSocket != 8 {
		t.Fatalf("CoresPerSocket = %d, want raw Ruby core_count 8", got.CoresPerSocket)
	}
	if got.ThreadsPerCore != 2 {
		t.Fatalf("ThreadsPerCore = %d, want 2", got.ThreadsPerCore)
	}
}

func TestCoreFacts_includeProcessorTopology(t *testing.T) {
	collection := Collection(CoreFacts(testSession, nil))
	processors, ok := collection["processors"].(map[string]any)
	if !ok {
		t.Fatalf("processors fact = %#v, want map", collection["processors"])
	}

	for _, key := range []string{"cores", "threads"} {
		value, ok := processors[key].(int)
		if !ok || value <= 0 {
			t.Fatalf("processors.%s = %#v, want positive int", key, processors[key])
		}
	}
}

func TestProcessorSpeedFacts_omittedWhenProbeYieldsNothing(t *testing.T) {
	t.Parallel()

	if got := processorSpeedFacts(""); got != nil {
		t.Fatalf("processorSpeedFacts(\"\") = %#v, want nil", got)
	}
	want := []ResolvedFact{{Name: "processors.speed", Value: "2.30 GHz"}}
	if got := processorSpeedFacts("2.30 GHz"); !reflect.DeepEqual(got, want) {
		t.Fatalf("processorSpeedFacts() = %#v, want %#v", got, want)
	}
}

func TestHertzToHumanReadable(t *testing.T) {
	tests := []struct {
		name string
		hz   any
		want string
	}{
		{"zero", 0, ""},
		{"non numeric string", "test", ""},
		{"gigahertz", 2_300_000_000, "2.30 GHz"},
		{"megahertz", 800_000_000, "800.00 MHz"},
		{"hertz", 42, "42.00 Hz"},
		{"string kilohertz", "2400", "2.40 kHz"},
		{"rounds down", 2_363_000_000, "2.36 GHz"},
		{"rounds half up", 2_365_000_000, "2.37 GHz"},
		{"rounds up", 2_367_000_000, "2.37 GHz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hertzToHumanReadable(tt.hz); got != tt.want {
				t.Fatalf("hertzToHumanReadable(%#v) = %q, want %q", tt.hz, got, tt.want)
			}
		})
	}
}

func TestParseLinuxProcessorSpeed(t *testing.T) {
	input := "processor\t: 0\nmodel name\t: CPU\ncpu MHz\t\t: 1800.000\n"

	if got, want := parseLinuxProcessorSpeed(input), "1.80 GHz"; got != want {
		t.Fatalf("parseLinuxProcessorSpeed() = %q, want %q", got, want)
	}
}

func TestParseLinuxProcessorSpeedRejectsMissingInvalidAndZeroMHz(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"processor\t: 0\nmodel name\t: CPU\n",
		"cpu MHz\t\t: not-a-number\n",
		"cpu MHz\t\t: 0\n",
	} {
		if got := parseLinuxProcessorSpeed(input); got != "" {
			t.Fatalf("parseLinuxProcessorSpeed(%q) = %q, want empty", input, got)
		}
	}
}

func TestParseLinuxProcessorModels(t *testing.T) {
	input := "processor\t: 0\nmodel name\t: Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz\n" +
		"processor\t: 1\nmodel name\t: Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz\n"

	got := parseLinuxProcessorModels(input)
	want := []string{
		"Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz",
		"Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxProcessorModels() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxProcessorModelsMatchesRubyPowerPCCPUKey(t *testing.T) {
	input := "processor\t: 0\ncpu\t\t: POWER8 (raw), altivec supported\n" +
		"processor\t: 1\ncpu\t\t: POWER8 (raw), altivec supported\n"

	got := parseLinuxProcessorModels(input)
	want := []string{
		"POWER8 (raw), altivec supported",
		"POWER8 (raw), altivec supported",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxProcessorModels() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxProcessorTopology(t *testing.T) {
	input := "processor\t: 0\nsiblings\t: 8\ncpu cores\t: 4\n"

	cores, threads := parseLinuxProcessorTopology(input)
	if cores != 4 || threads != 2 {
		t.Fatalf("parseLinuxProcessorTopology() = %d, %d, want 4, 2", cores, threads)
	}
}

func TestParseLinuxProcessorTopologyMatchesRubyLscpuResolver(t *testing.T) {
	input := "'Thread(s) per core': 2\n'Cores(s) per socket': 1\n"

	cores, threads := parseLinuxProcessorTopology(input)
	if cores != 1 || threads != 2 {
		t.Fatalf("parseLinuxProcessorTopology() = %d, %d, want 1, 2", cores, threads)
	}
}

func TestLinuxProcessorPhysicalCountFallsBackToSysfsPackageIDsLikeRuby(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{
		files: map[string][]byte{
			"/sys/devices/system/cpu/cpu0/topology/physical_package_id": []byte("0"),
			"/sys/devices/system/cpu/cpu1/topology/physical_package_id": []byte("1"),
		},
		dirs: map[string][]os.DirEntry{
			"/sys/devices/system/cpu": fakeDirEntries("cpu0", "cpu1", "cpuindex"),
		},
	}

	cpuinfo := "processor\t: 0\nmodel name\t: CPU\nprocessor\t: 1\nmodel name\t: CPU\n"
	if got, want := linuxProcessorPhysicalCount(cpuinfo, host), 2; got != want {
		t.Fatalf("linuxProcessorPhysicalCount() = %d, want %d", got, want)
	}
}

func TestLinuxProcessorPhysicalCountUsesHostSysfsWhenCPUInfoEmpty(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{
		files: map[string][]byte{
			"/sys/devices/system/cpu/cpu0/topology/physical_package_id": []byte("0\n"),
			"/sys/devices/system/cpu/cpu1/topology/physical_package_id": []byte("1\n"),
		},
		dirs: map[string][]os.DirEntry{
			"/sys/devices/system/cpu": fakeDirEntries("cpu0", "cpu1"),
		},
	}

	got := linuxProcessorPhysicalCount("", host)
	if got != 2 {
		t.Fatalf("linuxProcessorPhysicalCount() = %d, want sysfs fallback count 2", got)
	}
	if !reflect.DeepEqual(host.readDirCalls, []string{"/sys/devices/system/cpu"}) {
		t.Fatalf("readDir calls = %#v, want sysfs path", host.readDirCalls)
	}
}

func TestLinuxProcessorPhysicalCountUsesCPUInfoFirst(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{
		dirs: map[string][]os.DirEntry{
			"/sys/devices/system/cpu": fakeDirEntries("cpu0", "cpu1"),
		},
	}

	got := linuxProcessorPhysicalCount("processor: 0\nphysical id: 0\nprocessor: 1\nphysical id: 1\n", host)
	if got != 2 {
		t.Fatalf("linuxProcessorPhysicalCount() = %d, want cpuinfo physical count 2", got)
	}
	if len(host.readDirCalls) != 0 {
		t.Fatalf("readDir calls = %#v, want none when cpuinfo has physical IDs", host.readDirCalls)
	}
}

func TestLinuxProcessorPhysicalCountUsesCPUInfoPhysicalIDs(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{}
	cpuinfo := "processor\t: 0\nphysical id\t: 0\nprocessor\t: 1\nphysical id\t: 1\n"
	got := linuxProcessorPhysicalCount(cpuinfo, host)
	if got != 2 {
		t.Fatalf("linuxProcessorPhysicalCount() = %d, want 2", got)
	}
	if len(host.readDirCalls) != 0 {
		t.Fatalf("readDir calls = %#v, want none: read sysfs despite cpuinfo physical IDs", host.readDirCalls)
	}
}

func TestLinuxProcessorPhysicalCountHandlesSysfsReadFailures(t *testing.T) {
	t.Parallel()

	denied := &fakeHostOS{
		dirErrs: map[string]error{"/sys/devices/system/cpu": os.ErrPermission},
	}
	if got := linuxProcessorPhysicalCount("", denied); got != 0 {
		t.Fatalf("linuxProcessorPhysicalCount(readDir error) = %d, want 0", got)
	}

	unreadable := &fakeHostOS{
		dirs: map[string][]os.DirEntry{
			"/sys/devices/system/cpu": fakeDirEntries("cpu0"),
		},
		fileErrs: map[string]error{
			"/sys/devices/system/cpu/cpu0/topology/physical_package_id": os.ErrPermission,
		},
	}
	if got := linuxProcessorPhysicalCount("", unreadable); got != 0 {
		t.Fatalf("linuxProcessorPhysicalCount(readFile error) = %d, want 0", got)
	}
}

func TestLinuxProcessorPhysicalCountReadsPackageIDsThroughHost(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{
		files: map[string][]byte{
			"/sys/devices/system/cpu/cpu0/topology/physical_package_id": []byte("7\n"),
		},
		dirs: map[string][]os.DirEntry{
			"/sys/devices/system/cpu": fakeDirEntries("cpu0"),
		},
	}

	if got := linuxProcessorPhysicalCount("", host); got != 1 {
		t.Fatalf("linuxProcessorPhysicalCount() = %d, want 1", got)
	}
	want := []string{"/sys/devices/system/cpu/cpu0/topology/physical_package_id"}
	if !reflect.DeepEqual(host.readFileCalls, want) {
		t.Fatalf("readFile calls = %#v, want %#v", host.readFileCalls, want)
	}
}

func TestLinuxCPUEntryNameAcceptsOnlyNumberedCPUEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want bool
	}{
		{name: "cpu0", want: true},
		{name: "cpu12", want: true},
		{name: "cpu", want: false},
		{name: "cpuindex", want: false},
		{name: "node0", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linuxCPUEntryName(tt.name); got != tt.want {
				t.Fatalf("linuxCPUEntryName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIllumosProcessorHelpersParseCountsAndClock(t *testing.T) {
	t.Parallel()

	line := "The physical processor has 2 cores and 4 virtual processors (0-3)"
	if got := illumosVirtualProcessorCount(line); got != 4 {
		t.Fatalf("illumosVirtualProcessorCount() = %d, want 4", got)
	}
	if got := illumosCoreCount(line); got != 2 {
		t.Fatalf("illumosCoreCount() = %d, want 2", got)
	}
	if got := illumosClockMHz("x86 (GenuineIntel clock 3600 MHz)"); got != 3600 {
		t.Fatalf("illumosClockMHz() = %d, want 3600", got)
	}
	for _, input := range []string{"not a processor line", "The physical processor has virtual processors", "x86 no clock"} {
		if got := illumosVirtualProcessorCount(input); got != 0 {
			t.Fatalf("illumosVirtualProcessorCount(%q) = %d, want 0", input, got)
		}
		if got := illumosCoreCount(input); got != 0 {
			t.Fatalf("illumosCoreCount(%q) = %d, want 0", input, got)
		}
		if got := illumosClockMHz(input); got != 0 {
			t.Fatalf("illumosClockMHz(%q) = %d, want 0", input, got)
		}
	}
}

func TestParseLinuxProcessorExtensions_derivesX86Levels(t *testing.T) {
	input := "flags : fpu cx8 cmov mmx fxsr sse2 syscall lm cx16 lahf_lm popcnt sse4_1 sse4_2 ssse3 abm avx avx2 bmi1 bmi2 f16c fma movbe xsave\n"

	got := parseLinuxProcessorExtensions(input, "x86_64")
	want := []string{"x86_64", "x86_64-v1", "x86_64-v2", "x86_64-v3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxProcessorExtensions() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxProcessorExtensionsHandlesFallbackAndV4(t *testing.T) {
	t.Parallel()

	if got := parseLinuxProcessorExtensions("flags : ignored\n", "arm64"); !reflect.DeepEqual(got, []string{"arm64"}) {
		t.Fatalf("parseLinuxProcessorExtensions(non-x86) = %#v, want architecture only", got)
	}

	input := "vendor_id : GenuineIntel\nflags : fpu cx8 cmov mmx fxsr sse2 syscall lm cx16 lahf_lm popcnt sse4_1 sse4_2 ssse3 abm avx avx2 bmi1 bmi2 f16c fma movbe xsave avx512f avx512bw avx512cd avx512dq avx512vl\n"
	got := parseLinuxProcessorExtensions(input, "x86_64")
	want := []string{"x86_64", "x86_64-v1", "x86_64-v2", "x86_64-v3", "x86_64-v4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxProcessorExtensions(v4) = %#v, want %#v", got, want)
	}

	if containsAll(wordsSet("fpu mmx"), []string{"fpu", "sse2"}) {
		t.Fatal("containsAll() = true, want false when a required flag is missing")
	}
	if got := sortedProcessorExtensions(map[string]bool{"": true, "x86_64": true}); !reflect.DeepEqual(got, []string{"x86_64"}) {
		t.Fatalf("sortedProcessorExtensions() = %#v, want empty extension omitted", got)
	}
}
