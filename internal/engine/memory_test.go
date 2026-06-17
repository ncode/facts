package engine

import (
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

	collection := Collection(CoreFacts(testSession))
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

	collection := Collection(CoreFacts(testSession))
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
	collection := Collection(CoreFacts(testSession))
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
