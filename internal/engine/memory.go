package engine

import (
	"log/slog"
	"runtime"
	"strconv"
	"strings"
)

type freeBSDMemoryInfo struct {
	System map[string]any
	Swap   map[string]any
}

type windowsMemory struct {
	TotalBytes     int
	AvailableBytes int
	UsedBytes      int
	Capacity       string
}

func currentWindowsMemory(goos string, run commandRunner, log *slog.Logger) windowsMemory {
	if goos != "windows" {
		return windowsMemory{}
	}
	return parseWindowsMemory(windowsWMIOutput(run, "os", "FreePhysicalMemory,TotalVisibleMemorySize"), log)
}

func parseWindowsMemory(input string, log *slog.Logger) windowsMemory {
	if strings.TrimSpace(input) == "" {
		log.Debug("Resolving memory facts failed")
		return windowsMemory{}
	}
	values := parseWindowsWMIValues(input)
	totalKB, err := strconv.Atoi(values["TotalVisibleMemorySize"])
	if err != nil || totalKB <= 0 {
		log.Debug("Available or Total bytes are zero could not proceed further")
		return windowsMemory{}
	}
	availableKB, err := strconv.Atoi(values["FreePhysicalMemory"])
	if err != nil || availableKB <= 0 {
		log.Debug("Available or Total bytes are zero could not proceed further")
		return windowsMemory{}
	}
	totalBytes := totalKB * 1024
	availableBytes := availableKB * 1024
	usedBytes := max(0, totalBytes-availableBytes)
	return windowsMemory{
		TotalBytes:     totalBytes,
		AvailableBytes: availableBytes,
		UsedBytes:      usedBytes,
		Capacity:       memoryCapacity(usedBytes, totalBytes),
	}
}

func probeTotalPhysicalMemoryBytes(s *Session) int {
	switch runtime.GOOS {
	case "darwin":
		out := s.commandOutput("sysctl", "-n", "hw.memsize")
		if out == "" {
			return 0
		}
		value, err := strconv.ParseInt(strings.TrimSpace(out), 10, 0)
		if err != nil {
			return 0
		}
		return int(value)
	case "freebsd":
		return freeBSDMemoryValue(s.cachedFreeBSDMemoryInfo().System, "total_bytes")
	case "linux":
		return parseLinuxMeminfoBytes(s.cachedLinuxMeminfo(), "MemTotal")
	case "windows":
		return s.cachedWindowsMemory().TotalBytes
	}
	return 0
}

func probeAvailablePhysicalMemoryBytes(s *Session) int {
	switch runtime.GOOS {
	case "darwin":
		out := s.commandOutput("vm_stat")
		if out == "" {
			return 0
		}
		return parseDarwinVMStatAvailableBytes(out)
	case "freebsd":
		return freeBSDMemoryValue(s.cachedFreeBSDMemoryInfo().System, "available_bytes")
	case "linux":
		return parseLinuxMeminfoBytes(s.cachedLinuxMeminfo(), "MemAvailable")
	case "windows":
		return s.cachedWindowsMemory().AvailableBytes
	default:
		return 0
	}
}

func probeTotalSwapMemoryBytes(s *Session) int {
	switch runtime.GOOS {
	case "darwin":
		return s.cachedDarwinSwapUsage().TotalBytes
	case "freebsd":
		return freeBSDMemoryValue(s.cachedFreeBSDMemoryInfo().Swap, "total_bytes")
	case "linux":
		return parseLinuxMeminfoBytes(s.cachedLinuxMeminfo(), "SwapTotal")
	default:
		return 0
	}
}

func probeAvailableSwapMemoryBytes(s *Session) int {
	switch runtime.GOOS {
	case "darwin":
		return s.cachedDarwinSwapUsage().AvailableBytes
	case "freebsd":
		return freeBSDMemoryValue(s.cachedFreeBSDMemoryInfo().Swap, "available_bytes")
	case "linux":
		return parseLinuxMeminfoBytes(s.cachedLinuxMeminfo(), "SwapFree")
	default:
		return 0
	}
}

func probeSwapEncrypted(s *Session) bool {
	if runtime.GOOS == "darwin" {
		return s.cachedDarwinSwapUsage().Encrypted
	}
	if runtime.GOOS == "freebsd" {
		value, _ := s.cachedFreeBSDMemoryInfo().Swap["encrypted"].(bool)
		return value
	}
	return false
}

func probeWindowsMemory(s *Session) windowsMemory {
	return currentWindowsMemory(runtime.GOOS, s.commandOutput, s.logr())
}

type darwinSwapUsage struct {
	TotalBytes     int
	UsedBytes      int
	AvailableBytes int
	Encrypted      bool
}

func probeFreeBSDMemoryInfo(s *Session) freeBSDMemoryInfo {
	if runtime.GOOS != "freebsd" {
		return freeBSDMemoryInfo{}
	}
	return parseFreeBSDMemory(map[string]int{
		"vm.stats.vm.v_page_size":    freeBSDSysctlInt(s, "vm.stats.vm.v_page_size"),
		"vm.stats.vm.v_page_count":   freeBSDSysctlInt(s, "vm.stats.vm.v_page_count"),
		"vm.stats.vm.v_active_count": freeBSDSysctlInt(s, "vm.stats.vm.v_active_count"),
		"vm.stats.vm.v_wire_count":   freeBSDSysctlInt(s, "vm.stats.vm.v_wire_count"),
	}, s.commandOutput("swapinfo", "-k"))
}

func freeBSDSysctlInt(s *Session, name string) int {
	value, err := strconv.Atoi(strings.TrimSpace(s.commandOutput("sysctl", "-n", name)))
	if err != nil {
		return 0
	}
	return value
}

func freeBSDMemoryValue(values map[string]any, key string) int {
	value, ok := values[key].(int)
	if !ok {
		return 0
	}
	return value
}

func parseFreeBSDMemory(sysctlValues map[string]int, swapinfoOutput string) freeBSDMemoryInfo {
	return freeBSDMemoryInfo{
		System: parseFreeBSDSystemMemory(sysctlValues),
		Swap:   parseFreeBSDSwapMemory(swapinfoOutput),
	}
}

func parseFreeBSDSystemMemory(values map[string]int) map[string]any {
	pagesize := values["vm.stats.vm.v_page_size"]
	pageCount := values["vm.stats.vm.v_page_count"]
	if pagesize <= 0 || pageCount <= 0 {
		return nil
	}
	total := pageCount * pagesize
	used := (values["vm.stats.vm.v_active_count"] + values["vm.stats.vm.v_wire_count"]) * pagesize
	available := max(0, total-used)
	return map[string]any{
		"available_bytes": available,
		"capacity":        memoryCapacity(used, total),
		"total_bytes":     total,
		"used_bytes":      used,
	}
}

func parseFreeBSDSwapMemory(input string) map[string]any {
	if input == "" {
		return nil
	}
	total := 0
	used := 0
	available := 0
	encrypted := true
	found := false
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] == "Device" {
			continue
		}
		lineTotal, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		lineUsed, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		lineAvailable, err := strconv.Atoi(fields[3])
		if err != nil {
			continue
		}
		found = true
		total += lineTotal * 1024
		used += lineUsed * 1024
		available += lineAvailable * 1024
		encrypted = encrypted && strings.HasSuffix(fields[0], ".eli")
	}
	if !found {
		return nil
	}
	capacity := memoryCapacity(used, total)
	if used == 0 {
		capacity = "0%"
	}
	return map[string]any{
		"available_bytes": available,
		"capacity":        capacity,
		"encrypted":       encrypted,
		"total_bytes":     total,
		"used_bytes":      used,
	}
}

func probeDarwinSwapUsage(s *Session) darwinSwapUsage {
	return currentDarwinSwapUsage(runtime.GOOS, s.commandOutput)
}

func currentDarwinSwapUsage(goos string, run commandRunner) darwinSwapUsage {
	if goos != "darwin" {
		return darwinSwapUsage{}
	}
	return parseDarwinSwapUsage(run("sysctl", "-n", "vm.swapusage"))
}

func probeLinuxMeminfo(s *Session) string {
	data, err := s.readFile("/proc/meminfo")
	if err != nil {
		return ""
	}
	return string(data)
}

func parseLinuxMeminfoBytes(input, key string) int {
	if key == "MemAvailable" {
		if value := parseLinuxMeminfoValue(input, key); value != 0 {
			return value
		}
		return parseLinuxMeminfoValue(input, "MemFree") + parseLinuxMeminfoValue(input, "Buffers") + parseLinuxMeminfoValue(input, "Cached")
	}
	return parseLinuxMeminfoValue(input, key)
}

func parseLinuxMeminfoValue(input, key string) int {
	want := key + ":"
	for len(input) > 0 {
		line := input
		if before, after, ok := strings.Cut(input, "\n"); ok {
			line = before
			input = after
		} else {
			input = ""
		}
		if !strings.HasPrefix(line, want) {
			continue
		}
		value := strings.TrimLeft(line[len(want):], " \t")
		value, _, _ = strings.Cut(value, " ")
		value, _, _ = strings.Cut(value, "\t")
		kib, err := strconv.ParseInt(value, 10, 0)
		if err != nil {
			return 0
		}
		return int(kib * 1024)
	}
	return 0
}

func parseDarwinVMStatAvailableBytes(input string) int {
	pageSize := 0
	freePages := 0
	for line := range strings.SplitSeq(input, "\n") {
		if value, ok := strings.CutPrefix(line, "Mach Virtual Memory Statistics: (page size of "); ok {
			value, _, _ = strings.Cut(value, " bytes)")
			pageSize, _ = strconv.Atoi(strings.TrimSpace(value))
			continue
		}
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "Pages free:"); ok {
			value = strings.Trim(strings.TrimSpace(value), ".")
			freePages, _ = strconv.Atoi(value)
		}
	}
	return pageSize * freePages
}

func parseDarwinSwapUsage(input string) darwinSwapUsage {
	fields := strings.Fields(input)
	usage := darwinSwapUsage{Encrypted: strings.Contains(input, "(encrypted)")}
	for i := 0; i+2 < len(fields); i++ {
		if fields[i+1] != "=" {
			continue
		}
		bytes := parseDarwinMemoryAmountBytes(fields[i+2])
		switch fields[i] {
		case "total":
			usage.TotalBytes = bytes
		case "used":
			usage.UsedBytes = bytes
		case "free":
			usage.AvailableBytes = bytes
		}
	}
	return usage
}

func parseDarwinMemoryAmountBytes(input string) int {
	if input == "" {
		return 0
	}
	unit := input[len(input)-1]
	number := input[:len(input)-1]
	value, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0
	}
	switch unit {
	case 'K':
		value *= 1024
	case 'M':
		value *= 1024 * 1024
	case 'G':
		value *= 1024 * 1024 * 1024
	default:
		parsed, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return 0
		}
		value = parsed
	}
	return int(value)
}

// memoryCoreFacts assembles the memory category facts (system memory totals and
// usage plus swap totals, usage, capacity, and encryption state) for the
// current host.
func memoryCoreFacts(s *Session) []ResolvedFact {
	memoryTotalBytes := s.cachedTotalPhysicalMemoryBytes()
	memoryAvailableBytes := s.cachedAvailablePhysicalMemoryBytes()
	memoryUsedBytes := max(0, memoryTotalBytes-memoryAvailableBytes)
	swapTotalBytes := s.cachedTotalSwapMemoryBytes()
	swapAvailableBytes := s.cachedAvailableSwapMemoryBytes()
	swapMemory := memorySwapValues(swapTotalBytes, swapAvailableBytes)
	var swapEncrypted any
	if swapMemory.totalBytes != nil {
		swapEncrypted = s.cachedSwapEncrypted()
	}
	return []ResolvedFact{
		{Name: "memory.system.available", Value: bytesToHumanReadable(memoryAvailableBytes)},
		{Name: "memory.system.available_bytes", Value: memoryAvailableBytes},
		{Name: "memory.system.capacity", Value: memoryCapacity(memoryUsedBytes, memoryTotalBytes)},
		{Name: "memory.system.total", Value: bytesToHumanReadable(memoryTotalBytes)},
		{Name: "memory.system.total_bytes", Value: memoryTotalBytes},
		{Name: "memory.system.used", Value: bytesToHumanReadable(memoryUsedBytes)},
		{Name: "memory.system.used_bytes", Value: memoryUsedBytes},
		{Name: "memory.swap.available", Value: swapMemory.available},
		{Name: "memory.swap.available_bytes", Value: swapMemory.availableBytes},
		{Name: "memory.swap.capacity", Value: swapMemory.capacity},
		{Name: "memory.swap.encrypted", Value: swapEncrypted},
		{Name: "memory.swap.total", Value: swapMemory.total},
		{Name: "memory.swap.total_bytes", Value: swapMemory.totalBytes},
		{Name: "memory.swap.used", Value: swapMemory.used},
		{Name: "memory.swap.used_bytes", Value: swapMemory.usedBytes},
	}
}
