package engine

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestParseWindowsMemoryMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"FreePhysicalMemory=1024",
		"TotalVisibleMemorySize=4096",
	}, "\n")

	got := parseWindowsMemory(input, discardLog())
	want := windowsMemory{
		TotalBytes:     4096 * 1024,
		AvailableBytes: 1024 * 1024,
		UsedBytes:      3072 * 1024,
		Capacity:       "75.00%",
	}
	if got != want {
		t.Fatalf("parseWindowsMemory() = %#v, want %#v", got, want)
	}
}

func TestParseWindowsMemoryRejectsZeroValues(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"FreePhysicalMemory=1024\nTotalVisibleMemorySize=0\n",
		"FreePhysicalMemory=0\nTotalVisibleMemorySize=4096\n",
		"FreePhysicalMemory=bad\nTotalVisibleMemorySize=4096\n",
	} {
		if got := parseWindowsMemory(input, discardLog()); got != (windowsMemory{}) {
			t.Fatalf("parseWindowsMemory(%q) = %#v, want empty", input, got)
		}
	}
}

func TestParseWindowsMemoryLogsZeroValueDiagnosticLikeRubyResolver(t *testing.T) {
	var messages []string
	logger := captureLogger(&messages, nil, nil)

	got := parseWindowsMemory("FreePhysicalMemory=1024\nTotalVisibleMemorySize=0\n", logger)
	if got != (windowsMemory{}) {
		t.Fatalf("parseWindowsMemory() = %#v, want empty", got)
	}

	wantMessages := []string{"Available or Total bytes are zero could not proceed further"}
	if !reflect.DeepEqual(messages, wantMessages) {
		t.Fatalf("debug messages = %#v, want %#v", messages, wantMessages)
	}
}

func TestParseWindowsMemoryLogsFailureDiagnosticLikeRubyResolver(t *testing.T) {
	var messages []string
	logger := captureLogger(&messages, nil, nil)

	got := parseWindowsMemory("", logger)
	if got != (windowsMemory{}) {
		t.Fatalf("parseWindowsMemory() = %#v, want empty", got)
	}

	wantMessages := []string{"Resolving memory facts failed"}
	if !reflect.DeepEqual(messages, wantMessages) {
		t.Fatalf("debug messages = %#v, want %#v", messages, wantMessages)
	}
}

func TestCurrentWindowsMemoryRunsOnlyOnWindows(t *testing.T) {
	t.Parallel()

	called := false
	got := currentWindowsMemory("linux", func(name string, args ...string) string {
		called = true
		return ""
	}, discardLog())
	if got != (windowsMemory{}) {
		t.Fatalf("currentWindowsMemory(non-windows) = %#v, want empty", got)
	}
	if called {
		t.Fatal("currentWindowsMemory(non-windows) ran command")
	}
}

func TestCoreFacts_includeRealSystemMemoryTotal(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("memory total resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession, nil))
	memory, ok := collection["memory"].(map[string]any)
	if !ok {
		t.Fatalf("memory fact = %#v, want map", collection["memory"])
	}
	system, ok := memory["system"].(map[string]any)
	if !ok {
		t.Fatalf("memory.system fact = %#v, want map", memory["system"])
	}
	totalBytes, ok := system["total_bytes"].(int)
	if !ok {
		t.Fatalf("memory.system.total_bytes = %#v, want int", system["total_bytes"])
	}
	if totalBytes <= 0 {
		t.Fatalf("memory.system.total_bytes = %d, want positive physical memory", totalBytes)
	}
}

func TestCoreFacts_includeMemorySystemTotal(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("memory total resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession, nil))
	memory, ok := collection["memory"].(map[string]any)
	if !ok {
		t.Fatalf("memory fact = %#v, want map", collection["memory"])
	}
	system, ok := memory["system"].(map[string]any)
	if !ok {
		t.Fatalf("memory.system fact = %#v, want map", memory["system"])
	}
	total, ok := system["total"].(string)
	if !ok || total == "" {
		t.Fatalf("memory.system.total = %#v, want human-readable string", system["total"])
	}
}

func TestCoreFacts_includeMemoryUsageAndSwap(t *testing.T) {
	collection := Collection(CoreFacts(testSession, nil))
	memory, ok := collection["memory"].(map[string]any)
	if !ok {
		t.Fatalf("memory fact = %#v, want map", collection["memory"])
	}
	system, ok := memory["system"].(map[string]any)
	if !ok {
		t.Fatalf("memory.system fact = %#v, want map", memory["system"])
	}
	for _, key := range []string{"available", "available_bytes", "capacity", "total", "total_bytes", "used", "used_bytes"} {
		if _, ok := system[key]; !ok {
			t.Fatalf("memory.system = %#v, want key %q", system, key)
		}
	}
	swap, ok := memory["swap"].(map[string]any)
	if !ok {
		// No swap on this host: Ruby Facter omits the swap subtree entirely,
		// so memory must carry no swap key at all (not even encrypted).
		if value, exists := memory["swap"]; exists {
			t.Fatalf("memory.swap = %#v, want subtree omitted on a host without swap", value)
		}
		return
	}
	for _, key := range []string{"available", "available_bytes", "capacity", "total", "total_bytes", "used", "used_bytes"} {
		if _, ok := swap[key]; !ok {
			t.Fatalf("memory.swap = %#v, want key %q", swap, key)
		}
	}
	totalBytes, ok := swap["total_bytes"].(int)
	if !ok || totalBytes <= 0 {
		t.Fatalf("memory.swap.total_bytes = %#v, want positive total when the subtree is present", swap["total_bytes"])
	}
	if _, ok := swap["encrypted"]; !ok {
		t.Fatalf("memory.swap = %#v, want encrypted key", swap)
	}
}

func TestMemorySwapValuesIncludesUsageFieldsWhenSwapExists(t *testing.T) {
	got := memorySwapValues(1024, 512)

	if got.available == nil || got.availableBytes == nil || got.capacity == nil || got.total == nil || got.totalBytes == nil || got.used == nil || got.usedBytes == nil {
		t.Fatalf("memorySwapValues() = %#v, want populated usage fields", got)
	}
}

func TestMemorySwapValuesNoSwapMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	meminfo := "MemTotal: 4037608 kB\nMemAvailable: 3665024 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n"
	got := memorySwapValues(
		parseLinuxMeminfoBytes(meminfo, "SwapTotal"),
		parseLinuxMeminfoBytes(meminfo, "SwapFree"))

	if got.total != nil {
		t.Fatalf("swap total = %#v, want nil", got.total)
	}
	if got.totalBytes != nil {
		t.Fatalf("swap total_bytes = %#v, want nil", got.totalBytes)
	}
	if got.available != nil {
		t.Fatalf("swap available = %#v, want nil", got.available)
	}
	if got.availableBytes != nil {
		t.Fatalf("swap available_bytes = %#v, want nil", got.availableBytes)
	}
	if got.capacity != nil {
		t.Fatalf("swap capacity = %#v, want nil", got.capacity)
	}
	if got.used != nil {
		t.Fatalf("swap used = %#v, want nil", got.used)
	}
	if got.usedBytes != nil {
		t.Fatalf("swap used_bytes = %#v, want nil", got.usedBytes)
	}
}

func TestParseLinuxMeminfoBytes(t *testing.T) {
	input := "MemTotal:       16384000 kB\nMemAvailable:    4096000 kB\nSwapTotal:       1048576 kB\nSwapFree:         262144 kB\n"
	tests := []struct {
		name string
		key  string
		want int
	}{
		{name: "mem total", key: "MemTotal", want: 16_777_216_000},
		{name: "mem available", key: "MemAvailable", want: 4_194_304_000},
		{name: "swap total", key: "SwapTotal", want: 1_073_741_824},
		{name: "swap free", key: "SwapFree", want: 268_435_456},
		{name: "missing", key: "Buffers", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLinuxMeminfoBytes(input, tt.key); got != tt.want {
				t.Fatalf("parseLinuxMeminfoBytes(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

func TestParseLinuxMeminfoBytesFallsBackToFreeBuffersCachedForAvailableMemory(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"MemTotal:        4036680 kB",
		"MemFree:         3547792 kB",
		"Buffers:            4288 kB",
		"Cached:           298624 kB",
		"SwapCached:            0 kB",
	}, "\n")

	if got, want := parseLinuxMeminfoBytes(input, "MemAvailable"), 3_943_120_896; got != want {
		t.Fatalf("parseLinuxMeminfoBytes(MemAvailable fallback) = %d, want %d", got, want)
	}
}

func TestParseDarwinVMStatAvailableBytes(t *testing.T) {
	input := `Mach Virtual Memory Statistics: (page size of 4096 bytes)
Pages free:                             1364873.
Pages active:                           2653007.
Pages inactive:                         1583485.
Pages speculative:                      1061442.
Pages wired down:                        986842.
`

	if got, want := parseDarwinVMStatAvailableBytes(input), 5_590_519_808; got != want {
		t.Fatalf("parseDarwinVMStatAvailableBytes() = %d, want %d", got, want)
	}
}

func TestParseDarwinSwapUsage(t *testing.T) {
	got := parseDarwinSwapUsage("total = 3072.00M  used = 1422.75M  free = 1649.25M  (encrypted)")
	want := darwinSwapUsage{
		TotalBytes:     3_221_225_472,
		UsedBytes:      1_491_861_504,
		AvailableBytes: 1_729_363_968,
		Encrypted:      true,
	}

	if got != want {
		t.Fatalf("parseDarwinSwapUsage() = %#v, want %#v", got, want)
	}
}

func TestParseDarwinMemoryAmountBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int
	}{
		{input: "", want: 0},
		{input: "bad", want: 0},
		{input: "512K", want: 524_288},
		{input: "1.5M", want: 1_572_864},
		{input: "1G", want: 1_073_741_824},
		{input: "4096", want: 4096},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseDarwinMemoryAmountBytes(tt.input); got != tt.want {
				t.Fatalf("parseDarwinMemoryAmountBytes(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCurrentDarwinSwapUsageMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	got := currentDarwinSwapUsage("darwin", func(name string, args ...string) string {
		if name != "sysctl" || !reflect.DeepEqual(args, []string{"-n", "vm.swapusage"}) {
			t.Fatalf("run = %s %v, want sysctl -n vm.swapusage", name, args)
		}
		return "total = 3072.00M  used = 1422.75M  free = 1649.25M  (encrypted)"
	})

	want := darwinSwapUsage{
		TotalBytes:     3_221_225_472,
		UsedBytes:      1_491_861_504,
		AvailableBytes: 1_729_363_968,
		Encrypted:      true,
	}
	if got != want {
		t.Fatalf("currentDarwinSwapUsage() = %#v, want %#v", got, want)
	}
}

func TestDarwinMemoryParsersHandleMalformedAndNonDarwinInputs(t *testing.T) {
	t.Parallel()

	called := false
	got := currentDarwinSwapUsage("linux", func(string, ...string) string {
		called = true
		return "total = 1G"
	})
	if got != (darwinSwapUsage{}) {
		t.Fatalf("currentDarwinSwapUsage(non-darwin) = %#v, want empty", got)
	}
	if called {
		t.Fatal("currentDarwinSwapUsage(non-darwin) ran command")
	}

	if got := parseDarwinVMStatAvailableBytes("Pages free: 10.\n"); got != 0 {
		t.Fatalf("parseDarwinVMStatAvailableBytes(missing page size) = %d, want 0", got)
	}
	if got := parseDarwinSwapUsage("total : 1G used = 256M unknown = 3G free = 768M"); got != (darwinSwapUsage{UsedBytes: 268_435_456, AvailableBytes: 805_306_368}) {
		t.Fatalf("parseDarwinSwapUsage(noisy input) = %#v, want used/free only", got)
	}
	if got := parseDarwinMemoryAmountBytes("12B"); got != 0 {
		t.Fatalf("parseDarwinMemoryAmountBytes(12B) = %d, want 0", got)
	}
}

func TestBytesToMB(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  any
	}{
		{"bytes", 256_586_343, 244.6998052597046},
		{"string bytes", "2343455", 2.2348928451538086},
		{"string bytes with suffix", "2343455abc", 2.2348928451538086},
		{"non numeric string", "not-a-number", 0.0},
		{"zero", 0, 0.0},
		{"nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bytesToMB(tt.value); got != tt.want {
				t.Fatalf("bytesToMB(%#v) = %#v, want %#v", tt.value, got, tt.want)
			}
		})
	}
}

func TestBytesToHumanReadable(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  any
	}{
		{"nil", nil, nil},
		{"zero", 0, "0 bytes"},
		{"rounds to next unit", 1_048_575.7, "1.00 MiB"},
		{"bytes", 1023, "1023 bytes"},
		{"kibibyte", 1024, "1.00 KiB"},
		{"too large", "3296472651763232323235", "3296472651763232323235 bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bytesToHumanReadable(tt.value); got != tt.want {
				t.Fatalf("bytesToHumanReadable(%#v) = %#v, want %#v", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseFreeBSDMemory_returnsRubyCompatibleSystemAndSwapFacts(t *testing.T) {
	sysctlValues := map[string]int{
		"vm.stats.vm.v_page_size":    4096,
		"vm.stats.vm.v_page_count":   4_161_024,
		"vm.stats.vm.v_active_count": 2_335_139,
		"vm.stats.vm.v_wire_count":   1_167_569,
	}
	swapinfo := strings.Join([]string{
		"Device          1K-blocks     Used    Avail Capacity",
		"/dev/ada0p2.eli   2097152        0  2097152     0%",
		"/dev/ada1p2.eli   2097152        0  2097152     0%",
	}, "\n")

	got := parseFreeBSDMemory(sysctlValues, swapinfo)
	want := freeBSDMemoryInfo{
		System: map[string]any{
			"available_bytes": 2_696_462_336,
			"capacity":        "84.18%",
			"total_bytes":     17_043_554_304,
			"used_bytes":      14_347_091_968,
		},
		Swap: map[string]any{
			"available_bytes": 4_294_967_296,
			"capacity":        "0%",
			"encrypted":       true,
			"total_bytes":     4_294_967_296,
			"used_bytes":      0,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFreeBSDMemory() = %#v, want %#v", got, want)
	}
}

func TestMemorySysctlHelpersParseCommandOutput(t *testing.T) {
	s := NewSessionContext(context.Background())
	host := &fakeHostOS{runOutputs: map[string]string{
		fakeRunKey("sysctl", "-n", "vm.stats.vm.v_page_size"): "4096\n",
		fakeRunKey("sysctl", "-n", "bad"):                     "not-a-number\n",
		fakeRunKey("sysctl", "-n", "hw.physmem"):              "1049231360\n",
		fakeRunKey("sysctl", "-n", "hw.usermem"):              "2048\n",
	}}
	s.host = host

	if got := freeBSDSysctlInt(s, "vm.stats.vm.v_page_size"); got != 4096 {
		t.Fatalf("freeBSDSysctlInt() = %d, want 4096", got)
	}
	if got := freeBSDSysctlInt(s, "bad"); got != 0 {
		t.Fatalf("freeBSDSysctlInt(bad) = %d, want 0", got)
	}
	host.runCalls = nil
	if got := bsdSysctlInt(s, "bad", "hw.physmem"); got != 1_049_231_360 {
		t.Fatalf("bsdSysctlInt() = %d, want first positive fallback", got)
	}
	wantFallbackCalls := []fakeHostRunCall{
		{name: "sysctl", args: []string{"-n", "bad"}},
		{name: "sysctl", args: []string{"-n", "hw.physmem"}},
	}
	if !reflect.DeepEqual(host.runCalls, wantFallbackCalls) {
		t.Fatalf("fallback run calls = %#v, want %#v", host.runCalls, wantFallbackCalls)
	}
	if got := bsdSysctlInt(s, "hw.usermem", "hw.physmem"); got != 2048 {
		t.Fatalf("bsdSysctlInt() = %d, want first positive value in order", got)
	}
	if got := bsdSysctlInt(s, "bad"); got != 0 {
		t.Fatalf("bsdSysctlInt(all bad) = %d, want 0", got)
	}

	host.runCalls = nil
	if got := bsdSysctlInt(s, "hw.usermem", "hw.physmem"); got != 2048 {
		t.Fatalf("bsdSysctlInt() = %d, want first positive value in order", got)
	}
	wantCalls := []fakeHostRunCall{
		{name: "sysctl", args: []string{"-n", "hw.usermem"}},
	}
	if !reflect.DeepEqual(host.runCalls, wantCalls) {
		t.Fatalf("run calls = %#v, want %#v", host.runCalls, wantCalls)
	}
}

func TestMemoryProbesUseSessionPlatformForFreeBSD(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "freebsd",
		runOutputs: map[string]string{
			fakeRunKey("sysctl", "-n", "vm.stats.vm.v_page_size"):    "4096\n",
			fakeRunKey("sysctl", "-n", "vm.stats.vm.v_page_count"):   "100\n",
			fakeRunKey("sysctl", "-n", "vm.stats.vm.v_active_count"): "30\n",
			fakeRunKey("sysctl", "-n", "vm.stats.vm.v_wire_count"):   "20\n",
			fakeRunKey("swapinfo", "-k"): strings.Join([]string{
				"Device          1K-blocks     Used    Avail Capacity",
				"/dev/ada0p2.eli       200       50      150      25%",
			}, "\n"),
		},
	}

	if got := probeTotalPhysicalMemoryBytes(s); got != 409_600 {
		t.Fatalf("probeTotalPhysicalMemoryBytes() = %d, want 409600", got)
	}
	if got := probeAvailablePhysicalMemoryBytes(s); got != 204_800 {
		t.Fatalf("probeAvailablePhysicalMemoryBytes() = %d, want 204800", got)
	}
	if got := probeTotalSwapMemoryBytes(s); got != 204_800 {
		t.Fatalf("probeTotalSwapMemoryBytes() = %d, want 204800", got)
	}
	if got := probeAvailableSwapMemoryBytes(s); got != 153_600 {
		t.Fatalf("probeAvailableSwapMemoryBytes() = %d, want 153600", got)
	}
	if got := probeSwapEncrypted(s); !got {
		t.Fatalf("probeSwapEncrypted() = false, want true")
	}
}

func TestMemoryProbesUseSessionPlatformForBSDAndIllumos(t *testing.T) {
	tests := []struct {
		name       string
		platform   string
		runOutputs map[string]string
	}{
		{
			name:     "openbsd",
			platform: "openbsd",
			runOutputs: map[string]string{
				fakeRunKey("sysctl", "-n", "hw.physmem64"): "409600\n",
				fakeRunKey("vmstat", "-s"): strings.Join([]string{
					"4096 bytes per page",
					"30 pages active",
					"20 pages wired",
				}, "\n"),
				fakeRunKey("swapctl", "-sk"): "total: 200 1K-blocks allocated, 50 used, 150 available",
			},
		},
		{
			name:     "dragonfly",
			platform: "dragonfly",
			runOutputs: map[string]string{
				fakeRunKey("sysctl", "-n", "hw.physmem"): "409600\n",
				fakeRunKey("vmstat", "-s"): strings.Join([]string{
					"4096 bytes per page",
					"30 pages active",
					"20 pages wired",
				}, "\n"),
				fakeRunKey("swapinfo", "-k"): strings.Join([]string{
					"Device          1K-blocks     Used    Avail Capacity",
					"/dev/da0s1b           200       50      150      25%",
				}, "\n"),
			},
		},
		{
			name:     "illumos",
			platform: "illumos",
			runOutputs: map[string]string{
				fakeRunKey("kstat", "-p", "unix:0:system_pages:physmem", "unix:0:system_pages:freemem"): strings.Join([]string{
					"unix:0:system_pages:physmem\t100",
					"unix:0:system_pages:freemem\t50",
				}, "\n"),
				fakeRunKey("pagesize"):   "4096\n",
				fakeRunKey("swap", "-s"): "total: 50k used, 150k available",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSessionContext(context.Background())
			s.host = &fakeHostOS{platform: tt.platform, runOutputs: tt.runOutputs}

			if got := probeTotalPhysicalMemoryBytes(s); got != 409_600 {
				t.Fatalf("probeTotalPhysicalMemoryBytes() = %d, want 409600", got)
			}
			if got := probeAvailablePhysicalMemoryBytes(s); got != 204_800 {
				t.Fatalf("probeAvailablePhysicalMemoryBytes() = %d, want 204800", got)
			}
			if got := probeTotalSwapMemoryBytes(s); got != 204_800 {
				t.Fatalf("probeTotalSwapMemoryBytes() = %d, want 204800", got)
			}
			if got := probeAvailableSwapMemoryBytes(s); got != 153_600 {
				t.Fatalf("probeAvailableSwapMemoryBytes() = %d, want 153600", got)
			}
		})
	}
}

func TestMemoryProbesUseSessionPlatformForLinux(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "linux",
		files: map[string][]byte{
			"/proc/meminfo": []byte("MemTotal: 400 kB\nMemAvailable: 200 kB\nSwapTotal: 100 kB\nSwapFree: 25 kB\n"),
		},
	}

	if got := probeLinuxMeminfo(s); got != "MemTotal: 400 kB\nMemAvailable: 200 kB\nSwapTotal: 100 kB\nSwapFree: 25 kB\n" {
		t.Fatalf("probeLinuxMeminfo() = %q", got)
	}
	if got := probeTotalPhysicalMemoryBytes(s); got != 409_600 {
		t.Fatalf("probeTotalPhysicalMemoryBytes() = %d, want 409600", got)
	}
	if got := probeAvailablePhysicalMemoryBytes(s); got != 204_800 {
		t.Fatalf("probeAvailablePhysicalMemoryBytes() = %d, want 204800", got)
	}
	if got := probeTotalSwapMemoryBytes(s); got != 102_400 {
		t.Fatalf("probeTotalSwapMemoryBytes() = %d, want 102400", got)
	}
	if got := probeAvailableSwapMemoryBytes(s); got != 25_600 {
		t.Fatalf("probeAvailableSwapMemoryBytes() = %d, want 25600", got)
	}
}

func TestProbeLinuxMeminfoReturnsEmptyWhenMissing(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{platform: "linux"}

	if got := probeLinuxMeminfo(s); got != "" {
		t.Fatalf("probeLinuxMeminfo(missing) = %q, want empty", got)
	}
}

func TestMemoryTotalProbeUsesSessionPlatformForPlan9(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "plan9",
		files: map[string][]byte{
			"/dev/swap": []byte("1067843584 memory\n4096 pagesize\n"),
		},
	}

	if got := probeTotalPhysicalMemoryBytes(s); got != 1_067_843_584 {
		t.Fatalf("probeTotalPhysicalMemoryBytes(plan9) = %d, want 1067843584", got)
	}
}

func TestMemoryProbesReturnZeroForUnsupportedSessionPlatform(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{platform: "hurd"}

	if got := probeTotalPhysicalMemoryBytes(s); got != 0 {
		t.Fatalf("probeTotalPhysicalMemoryBytes(unsupported) = %d, want 0", got)
	}
	if got := probeAvailablePhysicalMemoryBytes(s); got != 0 {
		t.Fatalf("probeAvailablePhysicalMemoryBytes(unsupported) = %d, want 0", got)
	}
	if got := probeTotalSwapMemoryBytes(s); got != 0 {
		t.Fatalf("probeTotalSwapMemoryBytes(unsupported) = %d, want 0", got)
	}
	if got := probeAvailableSwapMemoryBytes(s); got != 0 {
		t.Fatalf("probeAvailableSwapMemoryBytes(unsupported) = %d, want 0", got)
	}
	if got := probeFreeBSDMemoryInfo(s); !reflect.DeepEqual(got, freeBSDMemoryInfo{}) {
		t.Fatalf("probeFreeBSDMemoryInfo(unsupported) = %#v, want empty", got)
	}
}

func TestMemoryProbesUseSessionPlatformForDarwinAndWindows(t *testing.T) {
	t.Run("darwin", func(t *testing.T) {
		s := NewSessionContext(context.Background())
		s.host = &fakeHostOS{
			platform: "darwin",
			runOutputs: map[string]string{
				fakeRunKey("sysctl", "-n", "hw.memsize"):   "409600\n",
				fakeRunKey("vm_stat"):                      "Mach Virtual Memory Statistics: (page size of 4096 bytes)\nPages free: 50.\n",
				fakeRunKey("sysctl", "-n", "vm.swapusage"): "total = 200K used = 50K free = 150K (encrypted)",
			},
		}

		if got := probeTotalPhysicalMemoryBytes(s); got != 409_600 {
			t.Fatalf("probeTotalPhysicalMemoryBytes() = %d, want 409600", got)
		}
		if got := probeAvailablePhysicalMemoryBytes(s); got != 204_800 {
			t.Fatalf("probeAvailablePhysicalMemoryBytes() = %d, want 204800", got)
		}
		if got := probeTotalSwapMemoryBytes(s); got != 204_800 {
			t.Fatalf("probeTotalSwapMemoryBytes() = %d, want 204800", got)
		}
		if got := probeAvailableSwapMemoryBytes(s); got != 153_600 {
			t.Fatalf("probeAvailableSwapMemoryBytes() = %d, want 153600", got)
		}
		if got := probeSwapEncrypted(s); !got {
			t.Fatalf("probeSwapEncrypted() = false, want true")
		}
	})

	t.Run("windows", func(t *testing.T) {
		s := NewSessionContext(context.Background())
		s.host = &fakeHostOS{
			platform: "windows",
			runOutputs: map[string]string{
				fakeRunKey("wmic", "os", "get", "FreePhysicalMemory,TotalVisibleMemorySize", "/value"): "FreePhysicalMemory=200\nTotalVisibleMemorySize=400\n",
			},
		}

		if got := probeWindowsMemory(s); got != (windowsMemory{TotalBytes: 409_600, AvailableBytes: 204_800, UsedBytes: 204_800, Capacity: "50.00%"}) {
			t.Fatalf("probeWindowsMemory() = %#v, want populated Windows memory", got)
		}
		if got := probeTotalPhysicalMemoryBytes(s); got != 409_600 {
			t.Fatalf("probeTotalPhysicalMemoryBytes() = %d, want 409600", got)
		}
		if got := probeAvailablePhysicalMemoryBytes(s); got != 204_800 {
			t.Fatalf("probeAvailablePhysicalMemoryBytes() = %d, want 204800", got)
		}
	})
}

func TestFreeBSDMemoryValueReturnsOnlyIntegerFields(t *testing.T) {
	values := map[string]any{
		"total_bytes": "1024",
		"used_bytes":  512,
	}

	if got := freeBSDMemoryValue(values, "used_bytes"); got != 512 {
		t.Fatalf("freeBSDMemoryValue(used_bytes) = %d, want 512", got)
	}
	if got := freeBSDMemoryValue(values, "total_bytes"); got != 0 {
		t.Fatalf("freeBSDMemoryValue(non-int) = %d, want 0", got)
	}
	if got := freeBSDMemoryValue(values, "missing"); got != 0 {
		t.Fatalf("freeBSDMemoryValue(missing) = %d, want 0", got)
	}
}

func TestFreeBSDMemoryParsersRejectInvalidInputs(t *testing.T) {
	t.Parallel()

	if got := parseFreeBSDSystemMemory(map[string]int{"vm.stats.vm.v_page_size": 4096}); got != nil {
		t.Fatalf("parseFreeBSDSystemMemory(missing page count) = %#v, want nil", got)
	}
	if got := parseFreeBSDSwapMemory(""); got != nil {
		t.Fatalf("parseFreeBSDSwapMemory(empty) = %#v, want nil", got)
	}
	input := strings.Join([]string{
		"Device 1K-blocks Used Avail Capacity",
		"bad row",
		"/dev/ada0p2 bad 0 0 0%",
		"/dev/ada1p2 10 bad 0 0%",
		"/dev/ada2p2 10 0 bad 0%",
	}, "\n")
	if got := parseFreeBSDSwapMemory(input); got != nil {
		t.Fatalf("parseFreeBSDSwapMemory(invalid rows) = %#v, want nil", got)
	}
}

func TestParseFreeBSDSwapMemoryAggregatesPlainAndEncryptedDevices(t *testing.T) {
	t.Parallel()

	input := `Device          1K-blocks     Used    Avail Capacity
/dev/ada0p2.eli   1024        512      512     50%
/dev/ada1p2       2048       1024     1024     50%`

	got := parseFreeBSDSwapMemory(input)
	want := map[string]any{
		"available_bytes": 1_572_864,
		"capacity":        "50.00%",
		"encrypted":       false,
		"total_bytes":     3_145_728,
		"used_bytes":      1_572_864,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFreeBSDSwapMemory() = %#v, want %#v", got, want)
	}
}

func TestParseBSDMemory_returnsSystemAndSwapFacts(t *testing.T) {
	sysctlValues := map[string]int{
		"hw.physmem":            1_049_231_360,
		"vmstat.bytes_per_page": 4096,
		"vmstat.pages_active":   19_007,
		"vmstat.pages_wired":    4_424,
		"vmstat.pages_free":     210_994,
		"vmstat.pages_inactive": 0,
		"vmstat.pages_managed":  247_459,
	}

	got := parseBSDMemory(sysctlValues, "total: 262144 1K-blocks allocated, 12664 used, 249480 available")
	systemUsed := (19_007 + 4_424) * 4096
	want := bsdMemoryInfo{
		System: map[string]any{
			"available_bytes": 1_049_231_360 - systemUsed,
			"capacity":        memoryCapacity(systemUsed, 1_049_231_360),
			"total_bytes":     1_049_231_360,
			"used_bytes":      systemUsed,
		},
		Swap: map[string]any{
			"available_bytes": 255_467_520,
			"capacity":        "4.83%",
			"total_bytes":     268_435_456,
			"used_bytes":      12_967_936,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseBSDMemory() = %#v, want %#v", got, want)
	}
}

func TestParseDragonFlyMemory_returnsSystemAndSwapFacts(t *testing.T) {
	got := parseDragonFlyMemory(map[string]int{
		"hw.physmem":            2_110_259_200,
		"vmstat.bytes_per_page": 4096,
		"vmstat.pages_active":   45_537,
		"vmstat.pages_wired":    98_459,
	}, `Device          1K-blocks     Used    Avail Capacity  Type
/dev/da0s1b       2097152        0  2097152     0%    Interleaved`)

	if got.System["total_bytes"] != 2_110_259_200 {
		t.Fatalf("System total_bytes = %#v, want 2110259200", got.System["total_bytes"])
	}
	if got.Swap["total_bytes"] != 2_147_483_648 {
		t.Fatalf("Swap total_bytes = %#v, want 2147483648", got.Swap["total_bytes"])
	}
}

func TestParseIllumosMemory_returnsSystemAndSwapFacts(t *testing.T) {
	got := parseIllumosMemory(
		"unix:0:system_pages:physmem\t1046390\nunix:0:system_pages:freemem\t744767\n",
		"4096\n",
		"total: 138896k bytes allocated + 16308k reserved = 155204k used, 2459052k available",
	)

	if got.System["total_bytes"] != 4_286_013_440 {
		t.Fatalf("System total_bytes = %#v, want 4286013440", got.System["total_bytes"])
	}
	if got.Swap["available_bytes"] != 2_518_069_248 {
		t.Fatalf("Swap available_bytes = %#v, want 2518069248", got.Swap["available_bytes"])
	}
}

func TestParseIllumosMemoryOmitsSystemWithoutFreePages(t *testing.T) {
	got := parseIllumosMemory(
		"unix:0:system_pages:physmem\t1046390\n",
		"4096\n",
		"",
	)

	if got.System != nil {
		t.Fatalf("System = %#v, want nil", got.System)
	}
}

func TestParseIllumosMemoryParsersRejectMalformedValues(t *testing.T) {
	t.Parallel()

	if got := parseIllumosMemory("unix:0:system_pages:physmem\tbad\n", "4096\n", ""); got.System != nil {
		t.Fatalf("parseIllumosMemory(bad kstat).System = %#v, want nil", got.System)
	}
	if got := parseIllumosSwapMemory("total: bad used, also bad available"); got != nil {
		t.Fatalf("parseIllumosSwapMemory(bad tokens) = %#v, want nil", got)
	}
	if got := parseIllumosKToken("badk"); got != 0 {
		t.Fatalf("parseIllumosKToken(badk) = %d, want 0", got)
	}
}

func TestParseBSDVMStatCounters(t *testing.T) {
	input := `       4096 bytes per page
not-a-number pages active
garbage
         10 files open
     241757 pages managed
      50327 pages free
       7715 pages active
     117925 pages inactive
          2 pages wired
`

	got := parseBSDVMStatCounters(input)
	want := map[string]int{
		"vmstat.bytes_per_page": 4096,
		"vmstat.pages_active":   7715,
		"vmstat.pages_wired":    2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseBSDVMStatCounters() = %#v, want %#v", got, want)
	}
}

func TestParseBSDMemory_omitsSwapWhenNoneConfigured(t *testing.T) {
	got := parseBSDMemory(map[string]int{
		"hw.physmem":            1024,
		"vmstat.bytes_per_page": 1,
		"vmstat.pages_active":   256,
		"vmstat.pages_wired":    128,
	}, "no swap devices configured")
	if got.Swap != nil {
		t.Fatalf("parseBSDMemory().Swap = %#v, want nil", got.Swap)
	}
}

func TestBSDMemoryParsersRejectInvalidInputs(t *testing.T) {
	t.Parallel()

	invalidSystems := []map[string]int{
		{"hw.physmem": 1024, "vmstat.pages_active": 1, "vmstat.pages_wired": 1},
		{"hw.physmem": 1024, "vmstat.bytes_per_page": 0, "vmstat.pages_active": 1, "vmstat.pages_wired": 1},
		{"hw.physmem": 1024, "vmstat.bytes_per_page": 1, "vmstat.pages_active": -1, "vmstat.pages_wired": 1},
	}
	for _, values := range invalidSystems {
		if got := parseBSDSystemMemory(values); got != nil {
			t.Fatalf("parseBSDSystemMemory(%#v) = %#v, want nil", values, got)
		}
	}

	for _, input := range []string{
		"total: bad 1K-blocks allocated, 1 used, 1 available",
		"total: 100 1K-blocks allocated, bad used, 1 available",
		"total: 100 1K-blocks allocated, 1 used, bad available",
	} {
		if got := parseBSDSwapMemory(input); got != nil {
			t.Fatalf("parseBSDSwapMemory(%q) = %#v, want nil", input, got)
		}
	}
}
