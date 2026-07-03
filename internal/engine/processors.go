package engine

import (
	"log/slog"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type processorInfo struct {
	ISA            string
	SpeedHz        int
	Models         []string
	LogicalCount   int
	PhysicalCount  int
	CoresPerSocket int
	ThreadsPerCore int
}

func currentProcessorISA(s *Session, goos, fallback string) string {
	if goos == "windows" {
		if isa := s.cachedPlatformProcessorInfo().ISA; isa != "" {
			return isa
		}
		return fallback
	}
	if goos == "plan9" {
		return plan9ProcessorISA(s.readFile, fallback)
	}
	processor := strings.TrimSpace(s.commandOutput("uname", "-p"))
	if processor == "" || processor == "unknown" {
		return fallback
	}
	return processor
}

func currentWindowsProcessors(goos string, run commandRunner, log *slog.Logger) processorInfo {
	if goos != "windows" {
		return processorInfo{}
	}
	return parseWindowsProcessors(windowsWMIOutput(run, "cpu", "Name,Architecture,NumberOfLogicalProcessors,NumberOfCores"), log)
}

func parseWindowsProcessors(input string, log *slog.Logger) processorInfo {
	records := parseWindowsWMIRecords(input)
	if len(records) == 0 {
		log.Debug("WMI query returned no resultsfor Win32_Processor with values Name, Architecture and NumberOfLogicalProcessors.")
		return processorInfo{}
	}

	info := processorInfo{
		Models:        make([]string, 0, len(records)),
		PhysicalCount: len(records),
	}
	var logicalTotal int
	var coreTotal int
	for _, record := range records {
		info.Models = append(info.Models, record["Name"])
		if info.ISA == "" {
			info.ISA = windowsProcessorISA(record["Architecture"], log)
		}
		logical, err := strconv.Atoi(record["NumberOfLogicalProcessors"])
		if err == nil && logical > 0 {
			logicalTotal += logical
		}
		cores, err := strconv.Atoi(record["NumberOfCores"])
		if err == nil && cores > 0 {
			coreTotal += cores
		}
	}
	if logicalTotal > 0 {
		info.LogicalCount = logicalTotal
	} else {
		info.LogicalCount = len(records)
	}
	if coreTotal > 0 {
		info.CoresPerSocket = max(1, coreTotal/len(records))
	}
	if info.CoresPerSocket > 0 {
		info.ThreadsPerCore = max(1, info.LogicalCount/(info.PhysicalCount*info.CoresPerSocket))
	}
	return info
}

func windowsProcessorISA(architecture string, log *slog.Logger) string {
	switch strings.TrimSpace(architecture) {
	case "0":
		return "x86"
	case "1":
		return "MIPS"
	case "2":
		return "Alpha"
	case "3":
		return "PowerPC"
	case "5":
		return "ARM"
	case "6":
		return "Itanium"
	case "9":
		return "x64"
	default:
		log.Debug("Unable to determine processor type: unknown architecture")
		return ""
	}
}

// processorSpeedFacts returns the processors.speed fact, or nothing when the
// speed probe yielded no value (e.g. Apple Silicon, where Ruby Facter has no
// speed key): an unresolvable key is absent, never an empty string.
func processorSpeedFacts(speed string) []ResolvedFact {
	if speed == "" {
		return nil
	}
	return []ResolvedFact{{Name: "processors.speed", Value: speed}}
}

func probeProcessorSpeed(s *Session) string {
	switch s.goos() {
	case "darwin":
		if speed := s.cachedPlatformProcessorInfo().SpeedHz; speed > 0 {
			return hertzToHumanReadable(int64(speed))
		}
	case "linux":
		data, err := s.readFile("/proc/cpuinfo")
		if err != nil {
			return ""
		}
		return parseLinuxProcessorSpeed(string(data))
	case "freebsd", "dragonfly":
		return hertzToHumanReadable(s.cachedPlatformProcessorInfo().SpeedHz)
	case "netbsd", "openbsd", "illumos":
		return hertzToHumanReadable(s.cachedPlatformProcessorInfo().SpeedHz)
	}
	return ""
}

func probeProcessorModels(s *Session) []string {
	goos := s.goos()
	architecture := architectureName(goos, s.cachedHardwareModel())
	switch goos {
	case "darwin":
		models := s.cachedPlatformProcessorInfo().Models
		if len(models) > 0 {
			return append([]string(nil), models...)
		}
	case "linux":
		data, err := s.readFile("/proc/cpuinfo")
		if err == nil {
			models := parseLinuxProcessorModels(string(data))
			if len(models) > 0 {
				return models
			}
		}
	case "freebsd", "netbsd", "openbsd", "dragonfly", "illumos", "windows", "plan9":
		models := s.cachedPlatformProcessorInfo().Models
		if len(models) > 0 {
			return append([]string(nil), models...)
		}
	}
	return []string{architecture}
}

func probeProcessorTopology(s *Session) (int, int) {
	logical := runtime.NumCPU()
	if logical <= 0 {
		logical = 1
	}
	switch s.goos() {
	case "darwin":
		processors := s.cachedPlatformProcessorInfo()
		if processors.CoresPerSocket > 0 && processors.ThreadsPerCore > 0 {
			return processors.CoresPerSocket, processors.ThreadsPerCore
		}
	case "linux":
		data, err := s.readFile("/proc/cpuinfo")
		if err == nil {
			cores, threads := parseLinuxProcessorTopology(string(data))
			if cores > 0 && threads > 0 {
				return cores, threads
			}
		}
	case "freebsd", "netbsd", "openbsd", "dragonfly", "illumos", "windows":
		processors := s.cachedPlatformProcessorInfo()
		if processors.CoresPerSocket > 0 && processors.ThreadsPerCore > 0 {
			return processors.CoresPerSocket, processors.ThreadsPerCore
		}
	}
	return logical, 1
}

func probePlatformProcessorInfo(s *Session) processorInfo {
	if s.goos() == "plan9" {
		return currentPlan9ProcessorInfo(s.readFile)
	}
	return currentProcessorInfo(s.goos(), s.commandOutput, s.logr())
}

func currentProcessorInfo(goos string, run func(string, ...string) string, log *slog.Logger) processorInfo {
	switch goos {
	case "darwin":
		return parseDarwinProcessors(run("sysctl",
			"hw.logicalcpu_max",
			"hw.physicalcpu_max",
			"machdep.cpu.brand_string",
			"hw.cpufrequency_max",
			"machdep.cpu.core_count",
			"machdep.cpu.thread_count",
		))
	case "freebsd":
		return parseFreeBSDProcessors(
			run("sysctl", "-n", "hw.ncpu"),
			run("sysctl", "-n", "hw.model"),
			run("sysctl", "-n", "hw.clockrate"),
		)
	case "dragonfly":
		return parseFreeBSDProcessors(
			run("sysctl", "-n", "hw.ncpu"),
			run("sysctl", "-n", "hw.model"),
			run("sysctl", "-n", "hw.clockrate"),
		)
	case "netbsd", "openbsd":
		return parseFreeBSDProcessors(
			run("sysctl", "-n", "hw.ncpu"),
			run("sysctl", "-n", "hw.model"),
			run("sysctl", "-n", "hw.cpuspeed"),
		)
	case "illumos":
		return parseIllumosProcessors(run("psrinfo", "-pv"))
	case "windows":
		return currentWindowsProcessors(goos, run, log)
	default:
		return processorInfo{}
	}
}

func parseDarwinProcessors(input string) processorInfo {
	values := make(map[string]string)
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	info := processorInfo{}
	info.LogicalCount = positiveInt(values["hw.logicalcpu_max"])
	info.PhysicalCount = positiveInt(values["hw.physicalcpu_max"])
	info.SpeedHz = positiveInt(values["hw.cpufrequency_max"])
	cores := positiveInt(values["machdep.cpu.core_count"])
	threads := positiveInt(values["machdep.cpu.thread_count"])
	if cores > 0 {
		info.CoresPerSocket = cores
	}
	if cores > 0 && threads > 0 {
		info.ThreadsPerCore = max(1, threads/cores)
	}
	model := strings.TrimSpace(values["machdep.cpu.brand_string"])
	if info.LogicalCount > 0 && model != "" {
		info.Models = make([]string, info.LogicalCount)
		for i := range info.Models {
			info.Models[i] = model
		}
	}
	return info
}

func positiveInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func parseFreeBSDProcessors(countOutput, modelOutput, speedOutput string) processorInfo {
	info := processorInfo{}
	count, err := strconv.Atoi(strings.TrimSpace(countOutput))
	if err == nil && count > 0 {
		info.LogicalCount = count
	}
	model := strings.TrimSpace(modelOutput)
	if info.LogicalCount > 0 && model != "" {
		info.Models = make([]string, info.LogicalCount)
		for i := range info.Models {
			info.Models[i] = model
		}
	}
	speedMHz, err := strconv.Atoi(strings.TrimSpace(speedOutput))
	if err == nil && speedMHz > 0 {
		info.SpeedHz = speedMHz * 1000 * 1000
	}
	if info.LogicalCount > 0 {
		info.CoresPerSocket = info.LogicalCount
		info.ThreadsPerCore = 1
	}
	return info
}

func parseIllumosProcessors(input string) processorInfo {
	info := processorInfo{}
	var wantModel bool
	coreTotal := 0
	nextModelCount := 1
	for line := range strings.SplitSeq(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "The physical processor has ") {
			info.PhysicalCount++
			nextModelCount = 1
			if count := illumosVirtualProcessorCount(line); count > 0 {
				info.LogicalCount += count
				nextModelCount = count
			}
			if cores := illumosCoreCount(line); cores > 0 {
				coreTotal += cores
			}
			continue
		}
		if clock := illumosClockMHz(line); clock > 0 {
			info.SpeedHz = clock * 1000 * 1000
			if fields := strings.Fields(line); len(fields) > 0 && info.ISA == "" {
				info.ISA = fields[0]
			}
			wantModel = true
			continue
		}
		if wantModel {
			for range nextModelCount {
				info.Models = append(info.Models, line)
			}
			nextModelCount = 1
			wantModel = false
		}
	}
	if info.LogicalCount == 0 && len(info.Models) > 0 {
		info.LogicalCount = len(info.Models)
	}
	if info.PhysicalCount == 0 && info.LogicalCount > 0 {
		info.PhysicalCount = 1
	}
	if info.LogicalCount > 0 && info.PhysicalCount > 0 {
		if coreTotal > 0 {
			info.CoresPerSocket = max(1, coreTotal/info.PhysicalCount)
			info.ThreadsPerCore = max(1, info.LogicalCount/(info.PhysicalCount*info.CoresPerSocket))
		} else {
			info.CoresPerSocket = max(1, info.LogicalCount/info.PhysicalCount)
			info.ThreadsPerCore = 1
		}
	}
	return info
}

func illumosVirtualProcessorCount(line string) int {
	after, ok := strings.CutPrefix(line, "The physical processor has ")
	if !ok {
		return 0
	}
	beforeVirtual, _, ok := strings.Cut(after, " virtual processor")
	if !ok {
		return 0
	}
	fields := strings.Fields(beforeVirtual)
	if len(fields) == 0 {
		return 0
	}
	return positiveInt(fields[len(fields)-1])
}

func illumosCoreCount(line string) int {
	after, ok := strings.CutPrefix(line, "The physical processor has ")
	if !ok {
		return 0
	}
	beforeCore, _, ok := strings.Cut(after, " core")
	if !ok {
		return 0
	}
	fields := strings.Fields(beforeCore)
	if len(fields) == 0 {
		return 0
	}
	return positiveInt(fields[len(fields)-1])
}

func illumosClockMHz(line string) int {
	_, after, ok := strings.Cut(line, " clock ")
	if !ok {
		return 0
	}
	value, _, _ := strings.Cut(after, " MHz")
	return positiveInt(value)
}

func probeProcessorExtensions(s *Session) []string {
	goos := s.goos()
	architecture := architectureName(goos, s.cachedHardwareModel())
	if goos != "linux" {
		return sortedProcessorExtensions(map[string]bool{architecture: true})
	}
	data, err := s.readFile("/proc/cpuinfo")
	if err != nil {
		return sortedProcessorExtensions(map[string]bool{architecture: true})
	}
	return parseLinuxProcessorExtensions(string(data), architecture)
}

func parseLinuxProcessorTopology(input string) (int, int) {
	cores := 0
	siblings := 0
	threadsPerCore := 0
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "cpu cores", "'Cores(s) per socket'", "Core(s) per socket", "Cores(s) per socket":
			cores, _ = strconv.Atoi(strings.TrimSpace(value))
		case "siblings":
			siblings, _ = strconv.Atoi(strings.TrimSpace(value))
		case "'Thread(s) per core'", "Thread(s) per core":
			threadsPerCore, _ = strconv.Atoi(strings.TrimSpace(value))
		}
		if cores > 0 && threadsPerCore > 0 {
			return cores, threadsPerCore
		}
		if cores > 0 && siblings > 0 {
			return cores, max(1, siblings/cores)
		}
	}
	return 0, 0
}

func linuxProcessorPhysicalCount(cpuinfo string, host hostOS) int {
	physicalIDs := make(map[string]struct{})
	for line := range strings.SplitSeq(cpuinfo, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "physical id" {
			continue
		}
		id := strings.TrimSpace(value)
		if id != "" {
			physicalIDs[id] = struct{}{}
		}
	}
	if len(physicalIDs) > 0 {
		return len(physicalIDs)
	}

	const sysCPUPath = "/sys/devices/system/cpu"
	entries, err := host.readDir(sysCPUPath)
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		name := entry.Name()
		if !linuxCPUEntryName(name) {
			continue
		}
		data, err := host.readFile(filepath.Join(sysCPUPath, name, "topology", "physical_package_id"))
		if err != nil {
			continue
		}
		id := strings.TrimSpace(string(data))
		if id != "" {
			physicalIDs[id] = struct{}{}
		}
	}
	return len(physicalIDs)
}

func linuxCPUEntryName(name string) bool {
	if !strings.HasPrefix(name, "cpu") || len(name) == len("cpu") {
		return false
	}
	for _, r := range name[len("cpu"):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseLinuxProcessorSpeed(input string) string {
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "cpu MHz" {
			continue
		}
		mhz, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || mhz <= 0 {
			return ""
		}
		return hertzToHumanReadable(int64(mhz * 1_000_000))
	}
	return ""
}

func parseLinuxProcessorModels(input string) []string {
	models := make([]string, 0)
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "model name", "cpu":
		default:
			continue
		}
		model := strings.TrimSpace(value)
		if model != "" {
			models = append(models, model)
		}
	}
	return models
}

func parseLinuxProcessorExtensions(input, architecture string) []string {
	extensions := map[string]bool{architecture: true}
	if architecture != "x86_64" {
		return sortedProcessorExtensions(extensions)
	}
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "flags" {
			continue
		}
		flags := wordsSet(value)
		if containsAll(flags, []string{"cmov", "cx8", "fpu", "fxsr", "lm", "mmx", "syscall", "sse2"}) {
			extensions["x86_64-v1"] = true
		}
		if containsAll(flags, []string{"cx16", "lahf_lm", "popcnt", "sse4_1", "sse4_2", "ssse3"}) {
			extensions["x86_64-v2"] = true
		}
		if containsAll(flags, []string{"abm", "avx", "avx2", "bmi1", "bmi2", "f16c", "fma", "movbe", "xsave"}) {
			extensions["x86_64-v3"] = true
		}
		if containsAll(flags, []string{"avx512f", "avx512bw", "avx512cd", "avx512dq", "avx512vl"}) {
			extensions["x86_64-v4"] = true
		}
	}
	return sortedProcessorExtensions(extensions)
}

func wordsSet(input string) map[string]bool {
	words := strings.Fields(input)
	out := make(map[string]bool, len(words))
	for _, word := range words {
		out[word] = true
	}
	return out
}

func containsAll(haystack map[string]bool, needles []string) bool {
	for _, needle := range needles {
		if !haystack[needle] {
			return false
		}
	}
	return true
}

func sortedProcessorExtensions(extensions map[string]bool) []string {
	out := make([]string, 0, len(extensions))
	for extension := range extensions {
		if extension != "" {
			out = append(out, extension)
		}
	}
	sort.Strings(out)
	return out
}

func hertzToHumanReadable(hz any) string {
	value, ok := numericValue(hz)
	if !ok || value <= 0 {
		return ""
	}
	units := [...]string{"Hz", "kHz", "MHz", "GHz", "THz"}
	unit := 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	return strconv.FormatFloat(value, 'f', 2, 64) + " " + units[unit]
}

// processorsCoreFacts assembles the processors category facts (logical and
// physical counts, cores, threads, models, ISA, speed, and extensions) for the
// current host.
func processorsCoreFacts(s *Session) []ResolvedFact {
	goos := s.goos()
	architecture := architectureName(goos, s.cachedHardwareModel())
	if goos == "plan9" {
		return plan9ProcessorsCoreFacts(s.cachedPlatformProcessorInfo(), currentProcessorISA(s, goos, architecture))
	}
	platformProcessors := processorInfo{}
	if goos == "darwin" || goos == "freebsd" || goos == "netbsd" || goos == "openbsd" || goos == "dragonfly" || goos == "illumos" || goos == "windows" {
		platformProcessors = s.cachedPlatformProcessorInfo()
	}
	if goos == "linux" {
		cpuinfo := ""
		if data, err := s.readFile("/proc/cpuinfo"); err == nil {
			cpuinfo = string(data)
		}
		platformProcessors.PhysicalCount = linuxProcessorPhysicalCount(cpuinfo, s.host)
	}
	processorCount := runtime.NumCPU()
	if platformProcessors.LogicalCount > 0 {
		processorCount = platformProcessors.LogicalCount
	}
	physicalProcessorCount := processorCount
	if platformProcessors.PhysicalCount > 0 {
		physicalProcessorCount = platformProcessors.PhysicalCount
	}
	processorISA := currentProcessorISA(s, goos, architecture)
	processorModels := s.cachedProcessorModels()
	processorSpeed := s.cachedProcessorSpeed()
	processorCores, processorThreads := s.cachedProcessorTopology()
	processorExtensions := s.cachedProcessorExtensions()
	facts := []ResolvedFact{
		{Name: "processors.count", Value: processorCount},
		{Name: "processors.cores", Value: processorCores},
		{Name: "processors.extensions", Value: processorExtensions},
		{Name: "processors.isa", Value: processorISA},
		{Name: "processors.models", Value: processorModels},
		{Name: "processors.physicalcount", Value: physicalProcessorCount},
		{Name: "processors.threads", Value: processorThreads},
	}
	facts = append(facts, processorSpeedFacts(processorSpeed)...)
	return facts
}
