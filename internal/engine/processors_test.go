package engine

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestCurrentProcessorISAUsesOpenBSDUnameProcessor(t *testing.T) {
	got := currentProcessorISA(testSession, "openbsd", "amd64", func(name string, args ...string) string {
		if name != "uname" || !reflect.DeepEqual(args, []string{"-p"}) {
			t.Fatalf("command = %s %#v, want uname -p", name, args)
		}
		return "i386\n"
	})

	if got != "i386" {
		t.Fatalf("currentProcessorISA(testSession, openbsd) = %q, want i386", got)
	}
}

func TestCoreFacts_processorSpeedOmittedWhenProbeYieldsNothing(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
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
	collection := Collection(CoreFacts(testSession))
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
	sysCPU := t.TempDir()
	for cpu, packageID := range map[string]string{"cpu0": "0", "cpu1": "1"} {
		topology := filepath.Join(sysCPU, cpu, "topology")
		if err := os.MkdirAll(topology, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", topology, err)
		}
		if err := os.WriteFile(filepath.Join(topology, "physical_package_id"), []byte(packageID), 0o644); err != nil {
			t.Fatalf("WriteFile package id: %v", err)
		}
	}
	if err := os.Mkdir(filepath.Join(sysCPU, "cpuindex"), 0o755); err != nil {
		t.Fatalf("Mkdir cpuindex: %v", err)
	}

	cpuinfo := "processor\t: 0\nmodel name\t: CPU\nprocessor\t: 1\nmodel name\t: CPU\n"
	if got, want := linuxProcessorPhysicalCount(cpuinfo, sysCPU), 2; got != want {
		t.Fatalf("linuxProcessorPhysicalCount() = %d, want %d", got, want)
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
