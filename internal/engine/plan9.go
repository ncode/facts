package engine

import (
	"net"
	"path"
	"strconv"
	"strings"
	"time"
)

func parsePlan9Sysname(input string) string {
	return plan9CleanString(input)
}

func plan9Architecture(readFile fileReader, fallback string) string {
	if architecture := plan9EnvValue("/env/objtype", readFile); architecture != "" {
		return architecture
	}
	return fallback
}

func plan9ProcessorISA(readFile fileReader, fallback string) string {
	if isa := plan9EnvValue("/env/cputype", readFile); isa != "" {
		return isa
	}
	if isa := plan9EnvValue("/env/objtype", readFile); isa != "" {
		return isa
	}
	return fallback
}

func plan9EnvValue(path string, readFile fileReader) string {
	data, err := readFile(path)
	if err != nil {
		return ""
	}
	return plan9CleanString(string(data))
}

func plan9CleanString(input string) string {
	input = strings.ReplaceAll(input, "\x00", "")
	return strings.TrimSpace(input)
}

func parsePlan9SwapMemoryTotal(input string) int {
	for line := range strings.Lines(input) {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != "memory" {
			continue
		}
		total, err := strconv.Atoi(fields[0])
		if err == nil && total > 0 {
			return total
		}
	}
	return 0
}

func parsePlan9SysstatProcessorCount(input string) int {
	count := 0
	for line := range strings.Lines(input) {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func parsePlan9ProcessorModels(cputype, archctl string, count int) []string {
	model := plan9CleanString(cputype)
	if model == "" {
		model = parsePlan9ArchctlCPUModel(archctl)
	}
	if model == "" {
		return nil
	}
	if count <= 0 {
		count = 1
	}
	models := make([]string, count)
	for i := range models {
		models[i] = model
	}
	return models
}

func parsePlan9ArchctlCPUModel(input string) string {
	for line := range strings.Lines(input) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "cpu "))
		if len(fields) == 0 {
			return ""
		}
		lastModelField := len(fields) - 1
		for i := len(fields) - 1; i >= 0; i-- {
			if _, err := strconv.Atoi(fields[i]); err == nil {
				lastModelField = i
				break
			}
		}
		return strings.Join(fields[:lastModelField+1], " ")
	}
	return ""
}

func currentPlan9ProcessorInfo(readFile fileReader) processorInfo {
	count := parsePlan9SysstatProcessorCount(readText("/dev/sysstat", readFile))
	return processorInfo{
		LogicalCount: count,
		Models: parsePlan9ProcessorModels(
			readText("/dev/cputype", readFile),
			readText("/dev/archctl", readFile),
			count,
		),
	}
}

func parsePlan9IPIFCStatus(input string) map[string]any {
	interfaces := map[string]any{}
	name := ""
	for line := range strings.Lines(input) {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "device" {
			deviceName := path.Base(fields[1])
			if deviceName == "." || deviceName == "/" {
				name = ""
				continue
			}
			name = deviceName
			if _, ok := interfaces[name]; !ok {
				interfaces[name] = map[string]any{}
			}
			continue
		}
		if name == "" || len(fields) < 2 {
			continue
		}
		ip := net.ParseIP(fields[0]).To4()
		if ip == nil {
			continue
		}
		prefix, err := strconv.Atoi(strings.TrimPrefix(fields[1], "/"))
		if err != nil {
			continue
		}
		prefix = plan9IPv4Prefix(prefix)
		if prefix < 0 || prefix > 32 {
			continue
		}
		iface, _ := interfaces[name].(map[string]any)
		bindings, _ := iface["bindings"].([]any)
		bindings = append(bindings, interfaceBinding(ip, &net.IPNet{IP: ip, Mask: net.CIDRMask(prefix, 32)}))
		iface["bindings"] = bindings
	}
	if len(interfaces) == 0 {
		return nil
	}
	return interfaces
}

func plan9IPv4Prefix(prefix int) int {
	if prefix >= 96 {
		return prefix - 96
	}
	return prefix
}

func parsePlan9MACAddress(input string) string {
	value := strings.ToLower(plan9CleanString(input))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, ":", "")
	if len(value) != 12 {
		return ""
	}
	parts := make([]string, 0, 6)
	for i := 0; i < len(value); i += 2 {
		part := value[i : i+2]
		if _, err := strconv.ParseUint(part, 16, 8); err != nil {
			return ""
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ":")
}

func parsePlan9PrimaryRouteIP(input string) string {
	candidate := ""
	for line := range strings.Lines(input) {
		fields := strings.Fields(line)
		if len(fields) < 8 || fields[0] != "0.0.0.0" {
			continue
		}
		source := fields[6]
		ip := net.ParseIP(source).To4()
		if ip == nil || ip.Equal(net.IPv4zero) {
			continue
		}
		if fields[7] == "/128" {
			return source
		}
		if candidate == "" {
			candidate = source
		}
	}
	return candidate
}

func currentPlan9Interfaces(readFile fileReader, glob pathGlobber) map[string]any {
	paths, err := glob("/net/ipifc/*/status")
	if err != nil {
		return nil
	}
	interfaces := map[string]any{}
	for _, statusPath := range paths {
		mergeMissingInterfaceFacts(interfaces, parsePlan9IPIFCStatus(readText(statusPath, readFile)))
	}
	for name, value := range interfaces {
		iface, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if mac := parsePlan9MACAddress(readText("/net/"+name+"/addr", readFile)); mac != "" {
			iface["mac"] = mac
		}
	}
	if len(interfaces) == 0 {
		return nil
	}
	return interfaces
}

func plan9PrimaryInterface(iproute string, interfaces map[string]any) string {
	if primaryIP := parsePlan9PrimaryRouteIP(iproute); primaryIP != "" {
		if primary := primaryInterface(interfaces, primaryIP); primary != "" {
			return primary
		}
	}
	return firstNonIgnoredInterface(interfaces)
}

func plan9MemoryCoreFacts(totalBytes int) []ResolvedFact {
	if totalBytes <= 0 {
		return nil
	}
	return []ResolvedFact{
		{Name: "memory.system.total", Value: bytesToHumanReadable(totalBytes)},
		{Name: "memory.system.total_bytes", Value: totalBytes},
	}
}

func plan9ProcessorsCoreFacts(info processorInfo, isa string) []ResolvedFact {
	facts := make([]ResolvedFact, 0, 3)
	if info.LogicalCount > 0 {
		facts = append(facts, ResolvedFact{Name: "processors.count", Value: info.LogicalCount})
	}
	if isa != "" {
		facts = append(facts, ResolvedFact{Name: "processors.isa", Value: isa})
	}
	if len(info.Models) > 0 {
		facts = append(facts, ResolvedFact{Name: "processors.models", Value: info.Models})
	}
	return facts
}

func plan9NetworkingCoreFacts(s *Session) []ResolvedFact {
	return plan9NetworkingCoreFactsWithGlob(s, s.glob)
}

func plan9NetworkingCoreFactsWithGlob(s *Session, glob pathGlobber) []ResolvedFact {
	hostname := parsePlan9Sysname(readFileString("/dev/sysname", s.readFile))
	interfaces := currentPlan9Interfaces(s.readFile, glob)
	primary, interfaces := currentNetworkingData("plan9", interfaces, s.commandOutput, s.readFile)
	ipv4, _ := primaryInterfaceFact(interfaces, primary, "ip").(string)
	primaryBinding := primaryIPv4Binding(interfaces, ipv4)
	netmask, _ := primaryBinding["netmask"].(string)
	network, _ := primaryBinding["network"].(string)
	mac, _ := primaryInterfaceFact(interfaces, primary, "mac").(string)

	var hostnameValue any
	if hostname != "" {
		hostnameValue = hostname
	}
	return []ResolvedFact{
		{Name: "networking.hostname", Value: hostnameValue},
		{Name: "networking.interfaces", Value: interfaces},
		{Name: "networking.ip", Value: optionalNetworkingString(ipv4)},
		{Name: "networking.mac", Value: optionalNetworkingString(mac)},
		{Name: "networking.netmask", Value: optionalNetworkingString(netmask)},
		{Name: "networking.network", Value: optionalNetworkingString(network)},
		{Name: "networking.primary", Value: optionalNetworkingString(primary)},
	}
}

func plan9UptimeCoreFacts(uptime uptimeInfo) []ResolvedFact {
	if !uptime.Known {
		return nil
	}
	return []ResolvedFact{
		{Name: "system_uptime.days", Value: int64(uptime.Duration.Hours()) / 24},
		{Name: "system_uptime.hours", Value: int64(uptime.Duration.Hours())},
		{Name: "system_uptime.seconds", Value: int64(uptime.Duration.Seconds())},
		{Name: "system_uptime.uptime", Value: uptimeString(uptime)},
	}
}

func currentPlan9Uptime(run commandRunner) uptimeInfo {
	seconds := parseUptimeCommandSeconds(run("uptime"))
	if seconds <= 0 {
		return uptimeInfo{}
	}
	return uptimeInfo{Duration: time.Duration(seconds) * time.Second, Known: true}
}
