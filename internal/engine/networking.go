package engine

import (
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var linuxSystemdDHCPServerPattern = regexp.MustCompile(`(?m)^SERVER_ADDRESS=(\S+)`)

var linuxDHCPCDServerPattern = regexp.MustCompile(`(?m)^dhcp_server_identifier='([^']+)'`)

var openBSDDHCPServerPattern = regexp.MustCompile(`\sdhcp server (\S+)`)

var darwinDHCPServerPattern = regexp.MustCompile(`^[\d.a-f:\s]+$`)

type routeSourceBinding struct {
	Interface string
	IP        string
}

func primaryInterface(interfaces map[string]any, primaryIP string) string {
	if primaryIP == "" {
		return ""
	}
	for name, value := range interfaces {
		iface, ok := value.(map[string]any)
		if !ok {
			continue
		}
		bindings, ok := iface["bindings"].([]any)
		if !ok {
			continue
		}
		for _, bindingValue := range bindings {
			binding, ok := bindingValue.(map[string]any)
			if !ok {
				continue
			}
			if binding["address"] == primaryIP {
				return name
			}
		}
	}
	return ""
}

func primaryIPv4Binding(interfaces map[string]any, primaryIP string) map[string]any {
	if primaryIP == "" {
		return nil
	}
	for _, value := range interfaces {
		iface, ok := value.(map[string]any)
		if !ok {
			continue
		}
		bindings, ok := iface["bindings"].([]any)
		if !ok {
			continue
		}
		for _, bindingValue := range bindings {
			binding, ok := bindingValue.(map[string]any)
			if ok && binding["address"] == primaryIP {
				return binding
			}
		}
	}
	return nil
}

func primaryIPv6Binding(interfaces map[string]any, primaryIP string) map[string]any {
	if primaryIP == "" {
		return nil
	}
	for _, value := range interfaces {
		iface, ok := value.(map[string]any)
		if !ok {
			continue
		}
		bindings, ok := iface["bindings6"].([]any)
		if !ok {
			continue
		}
		for _, bindingValue := range bindings {
			binding, ok := bindingValue.(map[string]any)
			if ok && binding["address"] == primaryIP {
				return binding
			}
		}
	}
	return nil
}

func primaryIPv6Scope(interfaces map[string]any, primaryIP string) string {
	if primaryIP == "" {
		return ""
	}
	binding := primaryIPv6Binding(interfaces, primaryIP)
	if scope, _ := binding["scope6"].(string); scope != "" {
		return scope
	}
	return "global"
}

func primaryInterfaceFact(interfaces map[string]any, name, fact string) any {
	if name == "" {
		return nil
	}
	iface, ok := interfaces[name].(map[string]any)
	if !ok {
		return nil
	}
	return iface[fact]
}

func networkingDHCPFact(interfaces map[string]any, primaryIP string) string {
	primaryDHCP, _ := primaryInterfaceFact(interfaces, primaryInterface(interfaces, primaryIP), "dhcp").(string)
	return primaryDHCP
}

func networkingDHCPValue(goos string, interfaces map[string]any, primaryIP string) any {
	dhcp := networkingDHCPFact(interfaces, primaryIP)
	if (goos == "netbsd" || goos == "plan9") && dhcp == "" {
		return nil
	}
	return dhcp
}

func optionalNetworkingString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type networkInterfaceSnapshot struct {
	Interface net.Interface
	Addrs     []net.Addr
}

func currentNetworkInterfaceSnapshots() ([]networkInterfaceSnapshot, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	snapshots := make([]networkInterfaceSnapshot, 0, len(interfaces))
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		snapshots = append(snapshots, networkInterfaceSnapshot{Interface: iface, Addrs: addrs})
	}
	return snapshots, nil
}

func networkingInterfaces(s *Session) map[string]any {
	return networkingInterfacesForPlatform(s, s.goos(), currentNetworkInterfaceSnapshots)
}

func networkingInterfacesForPlatform(s *Session, goos string, snapshotProvider func() ([]networkInterfaceSnapshot, error)) map[string]any {
	if goos == "plan9" {
		return currentPlan9Interfaces(s.readFile, s.glob)
	}
	snapshots, err := snapshotProvider()
	if err != nil {
		if goos == "windows" {
			s.debug("Unable to retrieve networking facts!")
		}
		return nil
	}

	values := networkingInterfacesFromSnapshots(snapshots, goos)
	if goos == "dragonfly" {
		mergeMissingInterfaceFacts(values, dragonFlyInterfacesFromIfconfig(s.commandOutput("ifconfig")))
	}
	if goos == "linux" {
		addLinuxDHCPServersFromSnapshots(s, values, snapshots)
		addLinuxRouteSourceBindings(s, values)
		addLinuxIfInet6Flags(values, parseLinuxIfInet6Flags(readText("/proc/net/if_inet6", s.readFile)))
		addLinuxBondingSlaveMACs(values, s.host)
		addLinuxInterfaceMetadata(values, s.host)
	}
	return values
}

func mergeMissingInterfaceFacts(values, fallback map[string]any) {
	for name, fallbackValue := range fallback {
		fallbackInterface, ok := fallbackValue.(map[string]any)
		if !ok {
			continue
		}
		value, ok := values[name].(map[string]any)
		if !ok {
			values[name] = fallbackInterface
			continue
		}
		for key, fact := range fallbackInterface {
			if key == "bindings" || key == "bindings6" {
				if bindings, ok := mergeInterfaceBindings(value[key], fact); ok {
					value[key] = bindings
					continue
				}
			}
			if value[key] == nil {
				value[key] = fact
			}
		}
	}
}

func mergeInterfaceBindings(value, fallback any) ([]any, bool) {
	bindings, ok := value.([]any)
	if !ok {
		return nil, false
	}
	fallbackBindings, ok := fallback.([]any)
	if !ok {
		return nil, false
	}

	merged := append([]any(nil), bindings...)
	byAddress := map[string]map[string]any{}
	for _, binding := range merged {
		fields, ok := binding.(map[string]any)
		if !ok {
			continue
		}
		if address, ok := fields["address"].(string); ok && address != "" {
			byAddress[address] = fields
		}
	}
	for _, binding := range fallbackBindings {
		fields, ok := binding.(map[string]any)
		if !ok {
			continue
		}
		address, _ := fields["address"].(string)
		if existing := byAddress[address]; address != "" && existing != nil {
			for key, fact := range fields {
				if existing[key] == nil {
					existing[key] = fact
				}
			}
			continue
		}
		merged = append(merged, fields)
	}
	return merged, true
}

func networkingInterfacesFromSnapshots(snapshots []networkInterfaceSnapshot, goos string) map[string]any {
	values := make(map[string]any, len(snapshots))
	for _, snapshot := range snapshots {
		iface := snapshot.Interface
		if !networkInterfaceIsUsableForGOOS(goos, iface) {
			continue
		}
		addrs := snapshot.Addrs
		bindings := make([]any, 0, len(addrs))
		bindings6 := make([]any, 0, len(addrs))
		for _, addr := range addrs {
			ip, ipNet, ok := parseInterfaceAddr(addr)
			if !ok {
				continue
			}
			binding := interfaceBinding(ip, ipNet)
			if ip.To4() != nil {
				bindings = append(bindings, binding)
				continue
			}
			bindings6 = append(bindings6, binding)
		}

		value := make(map[string]any)
		if iface.MTU > 0 {
			value["mtu"] = iface.MTU
		}
		if mac := formatInterfaceMAC(goos, iface.HardwareAddr); mac != "" {
			value["mac"] = mac
		}
		if len(bindings) > 0 {
			value["bindings"] = bindings
		}
		if len(bindings6) > 0 {
			value["bindings6"] = bindings6
		}
		// Address-less interfaces (for example macOS gif0/stf0 tunnels) still
		// appear with their reported MTU, matching Ruby's getifaddrs-driven map.
		values[networkInterfaceName(goos, iface.Name)] = value
	}
	return values
}

func addLinuxDHCPServersFromSnapshots(s *Session, values map[string]any, snapshots []networkInterfaceSnapshot) {
	for _, snapshot := range snapshots {
		iface := snapshot.Interface
		value, ok := values[iface.Name].(map[string]any)
		if !ok {
			continue
		}
		if dhcp := linuxDHCPServer(s, iface.Name, iface.Index); dhcp != "" {
			value["dhcp"] = dhcp
		}
	}
}

func networkInterfaceIsUsable(iface net.Interface) bool {
	return iface.Flags&net.FlagUp != 0
}

func networkInterfaceIsUsableForGOOS(goos string, iface net.Interface) bool {
	if goos != "windows" {
		// POSIX enumeration mirrors getifaddrs: every interface appears, even
		// ones that are down or carry no addresses (macOS gif0/stf0 tunnels).
		return true
	}
	return networkInterfaceIsUsable(iface) && iface.Flags&net.FlagLoopback == 0
}

func networkInterfaceName(goos, name string) string {
	if goos == "windows" {
		return strings.ToValidUTF8(name, "\uFFFD")
	}
	return name
}

func formatInterfaceMAC(goos string, hw net.HardwareAddr) string {
	mac := hw.String()
	if goos == "windows" {
		return strings.ToUpper(mac)
	}
	return mac
}

func currentNetworkingData(goos string, interfaces map[string]any, run commandRunner, readFiles ...fileReader) (string, map[string]any) {
	readFile := missingFileReader
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	switch goos {
	case "darwin":
		addDarwinDHCPServers(interfaces, run)
		expandInterfaceBindings(interfaces)
		return primaryInterfaceFromRoute(run("route", "-n", "get", "default")), interfaces
	case "freebsd":
		addBSDInterfaceOperationalStates(interfaces, run)
		addFreeBSDInterfaceMedia(interfaces, run)
		addFreeBSDDHCPServers(interfaces, readFile)
		expandInterfaceBindings(interfaces)
		return primaryInterfaceFromRoute(run("route", "-n", "get", "default")), interfaces
	case "netbsd":
		addBSDInterfaceOperationalStates(interfaces, run)
		expandInterfaceBindings(interfaces)
		return primaryInterfaceFromRoute(run("route", "-n", "get", "default")), interfaces
	case "dragonfly":
		addBSDInterfaceOperationalStates(interfaces, run)
		addFreeBSDDHCPServers(interfaces, readFile)
		expandInterfaceBindings(interfaces)
		return primaryInterfaceFromRoute(run("route", "-n", "get", "default")), interfaces
	case "illumos":
		addIllumosDHCPServers(interfaces, run)
		expandInterfaceBindings(interfaces)
		return primaryInterfaceFromRoute(run("route", "-n", "get", "default")), interfaces
	case "openbsd":
		addOpenBSDDHCPServers(interfaces, run)
		addBSDInterfaceOperationalStates(interfaces, run)
		expandInterfaceBindings(interfaces)
		return primaryInterfaceFromRoute(run("route", "-n", "get", "default")), interfaces
	case "windows":
		if run != nil {
			addWindowsDHCPServers(interfaces, run)
		}
		for _, name := range sortedKeys(interfaces) {
			iface, ok := interfaces[name].(map[string]any)
			if !ok {
				continue
			}
			if _, hasDHCP := iface["dhcp"]; !hasDHCP {
				iface["dhcp"] = nil
			}
		}
		expandInterfaceBindings(interfaces)
		return windowsPrimaryInterface(interfaces), interfaces
	case "linux":
		expandInterfaceBindings(interfaces)
		return linuxPrimaryInterface(readText("/proc/net/route", readFile), interfaces, run), interfaces
	case "plan9":
		expandInterfaceBindings(interfaces)
		return plan9PrimaryInterface(readText("/net/iproute", readFile), interfaces), interfaces
	default:
		return "", interfaces
	}
}

func addDarwinDHCPServers(interfaces map[string]any, run commandRunner) {
	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		if server := darwinDHCPServer(run("ipconfig", "getoption", name, "server_identifier")); server != "" {
			iface["dhcp"] = server
		}
	}
}

func darwinDHCPServer(output string) string {
	output = strings.TrimSpace(output)
	if output == "" || !darwinDHCPServerPattern.MatchString(output) {
		return ""
	}
	return output
}

func addWindowsDHCPServers(interfaces map[string]any, run commandRunner) {
	info := windowsIPConfigAdapters(run("ipconfig", "/all"))
	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		adapter := info[name]
		if server := adapter.DHCPServer; server != "" {
			iface["dhcp"] = server
		}
		if suffix := adapter.DNSSuffix; suffix != "" {
			iface["dns_suffix"] = suffix
		}
	}
}

type windowsIPConfigAdapter struct {
	DHCPServer string
	DNSSuffix  string
}

func windowsIPConfigAdapters(output string) map[string]windowsIPConfigAdapter {
	adapters := map[string]windowsIPConfigAdapter{}
	adapter := ""
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasSuffix(line, ":") && !strings.Contains(line, " . ") {
			adapter = windowsIPConfigAdapterName(line)
			continue
		}
		if adapter == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		current := adapters[adapter]
		switch {
		case strings.Contains(key, "DHCP Server"):
			current.DHCPServer = strings.TrimSpace(value)
		case strings.Contains(key, "Connection-specific DNS Suffix"):
			current.DNSSuffix = strings.TrimSpace(value)
		}
		if current != (windowsIPConfigAdapter{}) {
			adapters[adapter] = current
		}
	}
	return adapters
}

func windowsIPConfigAdapterName(header string) string {
	header = strings.TrimSuffix(strings.TrimSpace(header), ":")
	if before, after, ok := strings.Cut(header, " adapter "); ok && before != "" && after != "" {
		return after
	}
	return ""
}

func currentWindowsNetworkingDomain(interfaces map[string]any, run commandRunner) string {
	if interfaces == nil {
		return ""
	}

	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		if suffix, ok := iface["dns_suffix"].(string); ok && suffix != "" {
			return suffix
		}
	}
	return parseWindowsRegistryString(run("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`, "/v", "Domain"), "Domain")
}

func parseWindowsRegistryString(input, key string) string {
	value, _ := parseWindowsRegistryStringValue(input, key)
	return value
}

func parseWindowsRegistryStringValue(input, key string) (string, bool) {
	for line := range strings.SplitSeq(input, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == key && fields[1] == "REG_SZ" {
			if len(fields) == 2 {
				return "", true
			}
			return strings.Join(fields[2:], " "), true
		}
	}
	return "", false
}

func windowsFQDN(hostname, domain string) string {
	if hostname == "" {
		return ""
	}
	if domain == "" || strings.Contains(hostname, ".") {
		return hostname
	}
	return hostname + "." + domain
}

func windowsPrimaryInterface(interfaces map[string]any) string {
	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		if hasNonIgnoredBinding(iface, "bindings") || hasNonIgnoredBinding(iface, "bindings6") {
			return name
		}
	}
	return ""
}

func hasNonIgnoredBinding(iface map[string]any, key string) bool {
	bindings, ok := iface[key].([]any)
	if !ok {
		return false
	}
	for _, raw := range bindings {
		binding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		address, _ := binding["address"].(string)
		if !ignoredIPAddress(address) {
			return true
		}
	}
	return false
}

func ignoredIPAddress(address string) bool {
	if address == "" {
		return true
	}
	ip := net.ParseIP(address)
	if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 169 && ip4[1] == 254
	}
	return len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc
}

func primaryInterfaceFromRoute(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && key == "interface" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func linuxPrimaryInterfaceFromProcRoute(content string) string {
	for index, line := range strings.Split(content, "\n") {
		if index == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 7 && fields[1] == "00000000" && fields[7] == "00000000" && fields[0] != "*" {
			return fields[0]
		}
	}
	return ""
}

func linuxPrimaryInterface(procRoute string, interfaces map[string]any, run commandRunner) string {
	if primary := linuxPrimaryInterfaceFromProcRoute(procRoute); primary != "" {
		return primary
	}
	if run != nil {
		if primary := linuxPrimaryInterfaceFromIPRoute(run("ip", "route", "show", "default")); primary != "" {
			return primary
		}
	}
	return firstNonIgnoredInterface(interfaces)
}

func linuxPrimaryInterfaceFromIPRoute(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "default") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 4 {
			return fields[4]
		}
	}
	return ""
}

func firstNonIgnoredInterface(interfaces map[string]any) string {
	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		if hasNonIgnoredBinding(iface, "bindings") || hasNonIgnoredBinding(iface, "bindings6") {
			return name
		}
	}
	return ""
}

func addOpenBSDDHCPServers(interfaces map[string]any, run commandRunner) {
	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		if server := openBSDDHCPServer(run("dhcpleasectl", "-l", name)); server != "" {
			iface["dhcp"] = server
		}
	}
}

func addIllumosDHCPServers(interfaces map[string]any, run commandRunner) {
	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		if server := illumosDHCPServer(run("dhcpinfo", "-i", name, "ServerID")); server != "" {
			iface["dhcp"] = server
		}
	}
}

func illumosDHCPServer(output string) string {
	server := strings.TrimSpace(output)
	if net.ParseIP(server) == nil {
		return ""
	}
	return server
}

func addBSDInterfaceOperationalStates(interfaces map[string]any, run commandRunner) {
	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		if state := bsdInterfaceStatus(run("ifconfig", name)); state != "" {
			iface["operational_state"] = state
		}
	}
}

func addFreeBSDInterfaceMedia(interfaces map[string]any, run commandRunner) {
	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		speed, duplex := freeBSDInterfaceMedia(run("ifconfig", "-m", name))
		if speed > 0 {
			iface["speed"] = speed
		}
		if duplex != "" {
			iface["duplex"] = duplex
		}
	}
}

func addFreeBSDDHCPServers(interfaces map[string]any, readFile fileReader) {
	for _, name := range sortedKeys(interfaces) {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		content := readText("/var/db/dhclient.leases."+name, readFile)
		if server := linuxDHClientDHCPServer(content); server != "" {
			iface["dhcp"] = server
		}
	}
}

func dragonFlyInterfacesFromIfconfig(output string) map[string]any {
	interfaces := map[string]any{}
	var current map[string]any
	for line := range strings.SplitSeq(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			name, rest, ok := strings.Cut(line, ":")
			if !ok || name == "" {
				current = nil
				continue
			}
			current = map[string]any{}
			if mtu := ifconfigMTU(rest); mtu > 0 {
				current["mtu"] = mtu
			}
			interfaces[name] = current
			continue
		}
		if current == nil {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "ether":
			current["mac"] = fields[1]
		case "inet":
			if binding, ok := dragonFlyIPv4Binding(fields); ok {
				appendInterfaceBinding(current, "bindings", binding)
			}
		case "inet6":
			if binding, ok := dragonFlyIPv6Binding(fields); ok {
				appendInterfaceBinding(current, "bindings6", binding)
			}
		}
	}
	return interfaces
}

func ifconfigMTU(text string) int {
	fields := strings.Fields(text)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] != "mtu" {
			continue
		}
		mtu, err := strconv.Atoi(fields[i+1])
		if err == nil && mtu > 0 {
			return mtu
		}
	}
	return 0
}

func dragonFlyIPv4Binding(fields []string) (map[string]any, bool) {
	ip := net.ParseIP(fields[1]).To4()
	if ip == nil {
		return nil, false
	}
	mask := dragonFlyIPv4Mask(fieldAfter(fields, "netmask"))
	if mask == nil {
		return interfaceBinding(ip, nil), true
	}
	return interfaceBinding(ip, &net.IPNet{IP: ip, Mask: mask}), true
}

func dragonFlyIPv4Mask(value string) net.IPMask {
	if value == "" {
		return nil
	}
	if raw, err := strconv.ParseUint(value, 0, 32); err == nil {
		return net.IPv4Mask(byte(raw>>24), byte(raw>>16), byte(raw>>8), byte(raw))
	}
	ip := net.ParseIP(value).To4()
	if ip == nil {
		return nil
	}
	return net.IPMask(ip)
}

func dragonFlyIPv6Binding(fields []string) (map[string]any, bool) {
	address, _, _ := strings.Cut(fields[1], "%")
	ip := net.ParseIP(address)
	if ip == nil || ip.To4() != nil {
		return nil, false
	}
	prefix, err := strconv.Atoi(fieldAfter(fields, "prefixlen"))
	if err != nil || prefix < 0 || prefix > 128 {
		return interfaceBinding(ip, nil), true
	}
	return interfaceBinding(ip, &net.IPNet{IP: ip, Mask: net.CIDRMask(prefix, 128)}), true
}

func fieldAfter(fields []string, key string) string {
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == key {
			return fields[i+1]
		}
	}
	return ""
}

func appendInterfaceBinding(iface map[string]any, key string, binding map[string]any) {
	bindings, _ := iface[key].([]any)
	iface[key] = append(bindings, binding)
}

func bsdInterfaceStatus(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		value, ok := strings.CutPrefix(line, "status:")
		if ok {
			return strings.ToLower(strings.TrimSpace(value))
		}
	}
	return ""
}

func freeBSDInterfaceMedia(output string) (int, string) {
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		value, ok := strings.CutPrefix(line, "media:")
		if !ok {
			continue
		}
		return bsdMediaSpeedAndDuplex(value)
	}
	return 0, ""
}

func bsdMediaSpeedAndDuplex(media string) (int, string) {
	media = strings.ToLower(media)
	media = strings.NewReplacer("(", " ", ")", " ", "<", " ", ">", " ").Replace(media)
	duplex := ""
	switch {
	case strings.Contains(media, "full-duplex"):
		duplex = "full"
	case strings.Contains(media, "half-duplex"):
		duplex = "half"
	}
	for _, field := range strings.Fields(media) {
		if speed := bsdMediaSpeed(field); speed > 0 {
			return speed, duplex
		}
	}
	return 0, duplex
}

func bsdMediaSpeed(field string) int {
	prefix, _, ok := strings.Cut(strings.ToLower(field), "base")
	if !ok {
		return 0
	}
	multiplier := 1
	if strings.HasSuffix(prefix, "g") {
		multiplier = 1000
		prefix = strings.TrimSuffix(prefix, "g")
	}
	value, err := strconv.ParseFloat(prefix, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return int(value * float64(multiplier))
}

func openBSDDHCPServer(output string) string {
	match := openBSDDHCPServerPattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func expandInterfaceBindings(interfaces map[string]any) {
	for _, raw := range interfaces {
		iface, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		expandFirstInterfaceBinding(iface, "bindings", map[string]string{
			"address": "ip",
			"netmask": "netmask",
			"network": "network",
		})
		expandFirstInterfaceBinding(iface, "bindings6", map[string]string{
			"address": "ip6",
			"netmask": "netmask6",
			"network": "network6",
			"scope6":  "scope6",
		})
	}
}

func expandFirstInterfaceBinding(iface map[string]any, bindingKey string, factKeys map[string]string) {
	binding := firstInterfaceBinding(iface, bindingKey)
	if bindingKey == "bindings6" {
		if preferred := preferredIPv6InterfaceBinding(iface); preferred != nil {
			binding = preferred
		}
	}
	for bindingFact, ifaceFact := range factKeys {
		if value := binding[bindingFact]; value != nil && value != "" {
			iface[ifaceFact] = value
		}
	}
}

func preferredIPv6InterfaceBinding(iface map[string]any) map[string]any {
	bindings, ok := iface["bindings6"].([]any)
	if !ok {
		return nil
	}
	for _, raw := range bindings {
		binding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		scope, _ := binding["scope6"].(string)
		if scope != "link" {
			return binding
		}
	}
	return nil
}

func addLinuxRouteSourceBindings(s *Session, interfaces map[string]any) {
	if len(interfaces) == 0 {
		return
	}
	if output := s.commandOutput("ip", "route", "show"); output != "" {
		addRouteSourceBindings(interfaces, "bindings", linuxRouteSourceBindings(output))
	}
	if output := s.commandOutput("ip", "-6", "route", "show"); output != "" {
		addRouteSourceBindings(interfaces, "bindings6", linuxRouteSourceBindings(output))
	}
}

func addRouteSourceBindings(interfaces map[string]any, bindingKey string, routes []routeSourceBinding) {
	for _, route := range routes {
		iface, ok := interfaces[route.Interface].(map[string]any)
		if !ok {
			continue
		}
		binding := map[string]any{"address": route.IP}
		bindings, ok := iface[bindingKey].([]any)
		if !ok {
			iface[bindingKey] = []any{binding}
			continue
		}
		if bindingsContainAddress(bindings, route.IP) {
			continue
		}
		iface[bindingKey] = append(bindings, binding)
	}
}

func bindingsContainAddress(bindings []any, address string) bool {
	for _, value := range bindings {
		binding, ok := value.(map[string]any)
		if ok && binding["address"] == address {
			return true
		}
	}
	return false
}

func parseLinuxIfInet6Flags(content string) map[string]map[string][]string {
	flagsByInterface := map[string]map[string][]string{}
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		ip := linuxIfInet6IP(fields[0])
		if ip == "" {
			continue
		}
		flagsValue, err := strconv.ParseUint(fields[4], 16, 8)
		if err != nil {
			continue
		}
		flags := linuxIfInet6FlagNames(flagsValue)
		if len(flags) == 0 {
			continue
		}
		iface := fields[5]
		if flagsByInterface[iface] == nil {
			flagsByInterface[iface] = map[string][]string{}
		}
		flagsByInterface[iface][ip] = flags
	}
	return flagsByInterface
}

func linuxIfInet6IP(hexIP string) string {
	if len(hexIP) != 32 {
		return ""
	}
	parts := make([]string, 8)
	for i := range parts {
		parts[i] = hexIP[i*4 : i*4+4]
	}
	ip := net.ParseIP(strings.Join(parts, ":"))
	if ip == nil {
		return ""
	}
	return ip.String()
}

func linuxIfInet6FlagNames(flags uint64) []string {
	allFlags := []struct {
		bit  uint64
		name string
	}{
		{0x01, "temporary"},
		{0x02, "noad"},
		{0x04, "optimistic"},
		{0x08, "dadfailed"},
		{0x10, "homeaddress"},
		{0x20, "deprecated"},
		{0x40, "tentative"},
		{0x80, "permanent"},
	}
	values := make([]string, 0, len(allFlags))
	for _, flag := range allFlags {
		if flags&flag.bit != 0 {
			values = append(values, flag.name)
		}
	}
	return values
}

func addLinuxIfInet6Flags(interfaces map[string]any, flags map[string]map[string][]string) {
	for name, flagsByAddress := range flags {
		iface, ok := interfaces[name].(map[string]any)
		if !ok {
			continue
		}
		bindings, ok := iface["bindings6"].([]any)
		if !ok {
			continue
		}
		for _, value := range bindings {
			binding, ok := value.(map[string]any)
			if !ok {
				continue
			}
			address, _ := binding["address"].(string)
			if address == "" {
				continue
			}
			if addressFlags := flagsByAddress[address]; len(addressFlags) > 0 {
				binding["flags"] = addressFlags
			}
		}
	}
}

func addLinuxInterfaceMetadata(interfaces map[string]any, host hostOS) {
	for name, raw := range interfaces {
		iface, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ifaceRoot := filepath.Join("/sys/class/net", name)
		if state := strings.TrimSpace(readText(filepath.Join(ifaceRoot, "operstate"), host.readFile)); state != "" {
			iface["operational_state"] = state
		}
		_, err := host.stat(filepath.Join(ifaceRoot, "device"))
		iface["physical"] = err == nil
		if speed, err := strconv.Atoi(strings.TrimSpace(readText(filepath.Join(ifaceRoot, "speed"), host.readFile))); err == nil {
			iface["speed"] = speed
		}
		if duplex := strings.TrimSpace(readText(filepath.Join(ifaceRoot, "duplex"), host.readFile)); duplex != "" {
			iface["duplex"] = duplex
		}
	}
}

func addLinuxBondingSlaveMACs(interfaces map[string]any, host hostOS) {
	entries, err := host.readDir("/proc/net/bonding")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for slave, mac := range parseLinuxBondingSlaveMACs(readText(filepath.Join("/proc/net/bonding", entry.Name()), host.readFile)) {
			iface, ok := interfaces[slave].(map[string]any)
			if ok {
				iface["mac"] = mac
			}
		}
	}
}

func parseLinuxBondingSlaveMACs(content string) map[string]string {
	macs := map[string]string{}
	slave := ""
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "Slave Interface:"); ok {
			slave = strings.TrimSpace(value)
			continue
		}
		if value, ok := strings.CutPrefix(line, "Permanent HW addr:"); ok && slave != "" {
			if mac := strings.TrimSpace(value); mac != "" {
				macs[slave] = mac
			}
		}
	}
	return macs
}

func linuxDHCPServer(s *Session, interfaceName string, interfaceIndex int) string {
	if interfaceIndex > 0 {
		leasePath := filepath.Join("/run/systemd/netif/leases", strconv.Itoa(interfaceIndex))
		if server := linuxSystemdDHCPServer(readText(leasePath, s.readFile)); server != "" {
			return server
		}
	}
	for _, dir := range []string{"/var/lib/dhclient", "/var/lib/dhcp", "/var/lib/dhcp3", "/var/lib/NetworkManager", "/var/db"} {
		server := linuxDHCPServerFromLeaseDir(dir, interfaceName, s.host)
		if server != "" {
			return server
		}
	}
	if server := linuxDHCPCDDHCPServer(s.commandOutput("dhcpcd", "-U", interfaceName)); server != "" {
		return server
	}
	return ""
}

func linuxDHCPServerFromLeaseDir(dir, interfaceName string, host hostOS) string {
	entries, err := host.readDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.Contains(name, "lease") {
			continue
		}
		content := readText(filepath.Join(dir, name), host.readFile)
		if server, matched, explicit := linuxDHClientDHCPServerForInterfaceState(content, interfaceName); explicit {
			if matched {
				return server
			}
			continue
		}
		if !leaseFilenameMatchesInterface(name, interfaceName) {
			continue
		}
		if server := linuxDHClientDHCPServer(content); server != "" {
			return server
		}
		if server := linuxSystemdDHCPServer(content); server != "" {
			return server
		}
	}
	return ""
}

func linuxDHClientDHCPServerForInterfaceState(content, interfaceName string) (string, bool, bool) {
	if !dhclientContentHasInterface(content) {
		return "", false, false
	}
	server := ""
	blocks := linuxDHClientLeaseBlocks(content)
	if len(blocks) == 0 {
		if dhclientContentMatchesInterface(content, interfaceName) {
			return linuxDHClientDHCPServer(content), true, true
		}
		return "", false, true
	}
	matched := false
	sawInterfaceBlock := false
	for _, block := range blocks {
		if !dhclientContentHasInterface(block) {
			continue
		}
		sawInterfaceBlock = true
		if !dhclientContentMatchesInterface(block, interfaceName) {
			continue
		}
		matched = true
		server = linuxDHClientDHCPServer(block)
	}
	if matched {
		return server, true, true
	}
	if sawInterfaceBlock {
		return server, dhclientContentMatchesInterface(content, interfaceName), true
	}
	if !sawInterfaceBlock && dhclientContentMatchesInterface(content, interfaceName) {
		return linuxDHClientDHCPServer(content), true, true
	}
	return server, false, true
}

func linuxDHClientLeaseBlocks(content string) []string {
	var blocks []string
	for i := 0; i < len(content); {
		switch content[i] {
		case '#':
			i = skipDHClientComment(content, i)
			continue
		case '"':
			i = skipDHClientQuotedString(content, i)
			continue
		}
		if !dhclientKeywordAt(content, i, "lease") {
			i++
			continue
		}
		open := skipDHClientSpaceAndComments(content, i+len("lease"))
		if open == len(content) || content[open] != '{' {
			i++
			continue
		}
		end, next := dhclientBlockEnd(content, open)
		if end < 0 {
			if next <= i {
				i++
			} else {
				i = next
			}
			continue
		}
		blocks = append(blocks, content[i:end])
		i = end
	}
	return blocks
}

func dhclientBlockEnd(content string, open int) (int, int) {
	depth := 0
	for i := open; i < len(content); {
		switch content[i] {
		case '#':
			i = skipDHClientComment(content, i)
		case '"':
			i = skipDHClientQuotedString(content, i)
		case '{':
			depth++
			i++
		case '}':
			depth--
			i++
			if depth == 0 {
				return i, i
			}
		default:
			if depth == 1 && i != open && dhclientLeaseBlockStart(content, i) {
				return -1, i
			}
			i++
		}
	}
	return -1, len(content)
}

func dhclientLeaseBlockStart(content string, i int) bool {
	if !dhclientKeywordAt(content, i, "lease") {
		return false
	}
	open := skipDHClientSpaceAndComments(content, i+len("lease"))
	return open < len(content) && content[open] == '{'
}

func skipDHClientComment(content string, i int) int {
	for i < len(content) && content[i] != '\n' {
		i++
	}
	return i
}

func skipDHClientSpaceAndComments(content string, i int) int {
	for i < len(content) {
		if isDHClientSpace(content[i]) {
			i++
			continue
		}
		if content[i] == '#' {
			i = skipDHClientComment(content, i)
			continue
		}
		return i
	}
	return i
}

func skipDHClientQuotedString(content string, i int) int {
	i++
	for i < len(content) {
		if content[i] == '\n' || content[i] == '\r' {
			return i
		}
		if content[i] == '\\' {
			if i+1 < len(content) && (content[i+1] == '\n' || content[i+1] == '\r') {
				return i + 1
			}
			i += 2
			continue
		}
		if content[i] == '"' {
			return i + 1
		}
		i++
	}
	return i
}

func dhclientKeywordAt(content string, i int, keyword string) bool {
	if i > 0 && isDHClientWordByte(content[i-1]) {
		return false
	}
	if !strings.HasPrefix(content[i:], keyword) {
		return false
	}
	end := i + len(keyword)
	return end == len(content) || !isDHClientWordByte(content[end])
}

func isDHClientSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isDHClientWordByte(b byte) bool {
	return b == '_' || b == '-' || b == '.' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func dhclientContentHasInterface(content string) bool {
	return len(dhclientInterfaceNames(content)) > 0
}

func dhclientContentMatchesInterface(content, interfaceName string) bool {
	for _, name := range dhclientInterfaceNames(content) {
		if name == interfaceName {
			return true
		}
	}
	return false
}

func dhclientInterfaceNames(content string) []string {
	var names []string
	for i := 0; i < len(content); {
		switch content[i] {
		case '#':
			i = skipDHClientComment(content, i)
			continue
		case '"':
			i = skipDHClientQuotedString(content, i)
			continue
		}
		if !dhclientKeywordAt(content, i, "interface") {
			i++
			continue
		}
		valueStart := skipDHClientSpaceAndComments(content, i+len("interface"))
		value, next, ok := dhclientQuotedStringValue(content, valueStart)
		if ok {
			names = append(names, value)
			i = next
			continue
		}
		i++
	}
	return names
}

func dhclientQuotedStringValue(content string, i int) (string, int, bool) {
	if i == len(content) || content[i] != '"' {
		return "", i, false
	}
	start := i + 1
	i = start
	var out strings.Builder
	escaped := false
	for i < len(content) {
		switch content[i] {
		case '\n', '\r':
			return "", i, false
		case '\\':
			if i+1 < len(content) && (content[i+1] == '\n' || content[i+1] == '\r') {
				return "", i + 1, false
			}
			if !escaped {
				out.WriteString(content[start:i])
				escaped = true
			}
			if i+1 < len(content) {
				out.WriteByte(content[i+1])
			}
			i += 2
			start = i
		case '"':
			if escaped {
				out.WriteString(content[start:i])
				return out.String(), i + 1, true
			}
			return content[start:i], i + 1, true
		default:
			i++
		}
	}
	return "", i, false
}

func leaseFilenameMatchesInterface(name, interfaceName string) bool {
	return strings.HasSuffix(name, "-"+interfaceName+".lease") ||
		strings.HasSuffix(name, "."+interfaceName+".lease") ||
		strings.HasSuffix(name, "."+interfaceName+".leases")
}

func linuxSystemdDHCPServer(content string) string {
	match := linuxSystemdDHCPServerPattern.FindStringSubmatch(content)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func linuxDHClientDHCPServer(content string) string {
	server := ""
	for i := 0; i < len(content); {
		switch content[i] {
		case '#':
			i = skipDHClientComment(content, i)
			continue
		case '"':
			i = skipDHClientQuotedString(content, i)
			continue
		}
		if !dhclientKeywordAt(content, i, "option") {
			i++
			continue
		}
		valueStart := skipDHClientSpaceAndComments(content, i+len("option"))
		if !dhclientKeywordAt(content, valueStart, "dhcp-server-identifier") {
			i++
			continue
		}
		valueStart = skipDHClientSpaceAndComments(content, valueStart+len("dhcp-server-identifier"))
		valueEnd := valueStart
		for valueEnd < len(content) && !isDHClientSpace(content[valueEnd]) && content[valueEnd] != ';' {
			valueEnd++
		}
		value := content[valueStart:valueEnd]
		if ip := net.ParseIP(value).To4(); ip != nil {
			server = ip.String()
		}
		i = valueEnd
	}
	return server
}

func linuxDHCPCDDHCPServer(content string) string {
	match := linuxDHCPCDServerPattern.FindStringSubmatch(content)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func linuxRouteSourceBindings(content string) []routeSourceBinding {
	seen := map[routeSourceBinding]bool{}
	bindings := []routeSourceBinding{}
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.Fields(line)
		if slices.Contains(fields, "linkdown") {
			continue
		}
		binding := routeSourceBinding{}
		for i := 0; i+1 < len(fields); i++ {
			switch fields[i] {
			case "dev":
				binding.Interface = fields[i+1]
			case "src":
				binding.IP = fields[i+1]
			}
		}
		if binding.Interface == "" || binding.IP == "" || seen[binding] {
			continue
		}
		seen[binding] = true
		bindings = append(bindings, binding)
	}
	return bindings
}

func parseInterfaceAddr(addr net.Addr) (net.IP, *net.IPNet, bool) {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP, v, true
	case *net.IPAddr:
		return v.IP, nil, true
	default:
		return nil, nil, false
	}
}

func interfaceBinding(ip net.IP, ipNet *net.IPNet) map[string]any {
	binding := map[string]any{"address": ip.String()}
	if ip.To4() == nil {
		binding["scope6"] = ipv6Scope(ip)
	}
	if ipNet == nil {
		return binding
	}
	ipNet = &net.IPNet{IP: ip, Mask: ipNet.Mask}
	if netmask := netmaskString(ipNet.Mask); netmask != "" {
		binding["netmask"] = netmask
	}
	if network := networkAddress(ipNet); network != "" {
		binding["network"] = network
	}
	return binding
}

func ipv6Scope(ip net.IP) string {
	prefix := ""
	if isIPv4CompatibleIPv6(ip) {
		prefix = "compat,"
	}
	if ip.IsLoopback() {
		return prefix + "host"
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return prefix + "link"
	}
	if isIPv6SiteLocal(ip) {
		return prefix + "site"
	}
	return prefix + "global"
}

func isIPv4CompatibleIPv6(ip net.IP) bool {
	ip = ip.To16()
	if ip == nil || ip.To4() != nil || ip.IsUnspecified() || ip.IsLoopback() {
		return false
	}
	for _, b := range ip[:12] {
		if b != 0 {
			return false
		}
	}
	return true
}

func isIPv6SiteLocal(ip net.IP) bool {
	ip = ip.To16()
	if ip == nil {
		return false
	}
	return ip[0] == 0xfe && ip[1]&0xc0 == 0xc0
}

func networkAddress(ipNet *net.IPNet) string {
	if ipNet == nil {
		return ""
	}
	return ipNet.IP.Mask(ipNet.Mask).String()
}

func netmaskString(mask net.IPMask) string {
	if len(mask) == net.IPv4len {
		return net.IP(mask).String()
	}
	if len(mask) != net.IPv6len {
		return ""
	}
	return net.IP(mask).String()
}

func firstInterfaceBinding(iface map[string]any, key string) map[string]any {
	bindings, ok := iface[key].([]any)
	if !ok {
		return nil
	}
	for _, value := range bindings {
		binding, ok := value.(map[string]any)
		if ok {
			return binding
		}
	}
	return nil
}

func hostName(s *Session) (string, any) {
	return hostNameForPlatform(s.goos(), os.Hostname, func() string {
		return readLinuxKernelHostname(s.readFile)
	}, s.logr())
}

func hostNameForPlatform(goos string, lookup func() (string, error), linuxFallback func() string, log *slog.Logger) (string, any) {
	if goos == "linux" {
		return linuxHostNameFromLookups(lookup, linuxFallback, log)
	}
	return hostNameFromLookup(lookup, log)
}

func linuxHostNameFromLookups(lookup func() (string, error), fallback func() string, log *slog.Logger) (string, any) {
	hostname, value := hostNameFromLookup(lookup, log)
	if linuxHostnameUsable(hostname) {
		return hostname, value
	}
	if fallback == nil {
		return "", nil
	}
	hostname = strings.TrimSpace(fallback())
	if !linuxHostnameUsable(hostname) {
		return "", nil
	}
	return hostname, hostname
}

func linuxHostnameUsable(hostname string) bool {
	return hostname != "" && !strings.Contains(hostname, "0.0.0.0")
}

func readLinuxKernelHostname(readFiles ...fileReader) string {
	readFile := osHost{}.readFile
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	data, err := readFile("/proc/sys/kernel/hostname")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func hostNameFromLookup(lookup func() (string, error), log *slog.Logger) (string, any) {
	hostname, err := lookup()
	if err != nil {
		log.Debug("Socket.gethostname failed to return hostname")
		return "", nil
	}
	return hostname, hostname
}

func fqdn(hostname string) string {
	return fqdnWithLookup(hostname, net.LookupAddr)
}

func fqdnWithLookup(hostname string, lookup func(string) ([]string, error)) string {
	if hostname == "" || strings.Contains(hostname, ".") {
		return hostname
	}
	addrs, err := lookup(hostname)
	if err != nil || len(addrs) == 0 {
		return hostname
	}
	return strings.TrimSuffix(addrs[0], ".")
}

// currentHostnameFacts splits the node name like Ruby Facter: hostname is the
// node name up to the first dot, domain is the remainder (falling back to
// resolver search/domain configuration when the node name is undotted), and
// fqdn is hostname + "." + domain when a domain exists, else the bare
// hostname.
func currentHostnameFacts(goos, nodeName, resolvedFQDN, resolvConfPath string, readFiles ...fileReader) (string, string, string) {
	readFile := osHost{}.readFile
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	hostname := hostnameFromNodeName(nodeName)
	fqdnName, domain := currentHostnameFQDNAndDomain(goos, hostname, resolvedFQDN, resolvConfPath, readFile)
	return hostname, fqdnName, domain
}

// hostnameFromNodeName returns the short host name: the node name up to the
// first dot.
func hostnameFromNodeName(nodeName string) string {
	hostname, _, _ := strings.Cut(nodeName, ".")
	return hostname
}

func currentHostnameFQDNAndDomain(goos, hostname, resolvedFQDN, resolvConfPath string, readFile fileReader) (string, string) {
	switch goos {
	case "linux", "darwin":
		return currentResolvConfFQDNAndDomain(hostname, resolvedFQDN, resolvConfPath, readFile)
	default:
		return resolvedFQDN, domainFromFQDN(hostname, resolvedFQDN)
	}
}

func currentResolvConfFQDNAndDomain(hostname, resolvedFQDN, resolvConfPath string, readFile fileReader) (string, string) {
	content, err := readFile(resolvConfPath)
	if err != nil {
		return linuxFQDNAndDomain(hostname, resolvedFQDN, "")
	}
	return linuxFQDNAndDomain(hostname, resolvedFQDN, string(content))
}

func linuxFQDNAndDomain(hostname, resolvedFQDN, resolvConf string) (string, string) {
	domain := domainFromFQDN(hostname, resolvedFQDN)
	if hostname == "" || domain != "" {
		return resolvedFQDN, domain
	}

	domain = domainFromResolvConf(resolvConf)
	if domain == "" {
		return resolvedFQDN, ""
	}
	return hostname + "." + domain, domain
}

func hostnameFactValues(hostnameValue any, fqdn, domain string) (any, any) {
	if hostnameValue == nil {
		return nil, nil
	}
	var domainValue any = domain
	if domain == "" {
		domainValue = nil
	}
	return fqdn, domainValue
}

func domainFromResolvConf(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "domain" && !strings.HasPrefix(fields[1], ".") {
			return fields[1]
		}
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "search" && !strings.HasPrefix(fields[1], ".") {
			return fields[1]
		}
	}
	return ""
}

func domainFromFQDN(hostname, fqdn string) string {
	if fqdn == "" {
		return ""
	}
	if strings.Contains(hostname, ".") {
		_, domain, _ := strings.Cut(hostname, ".")
		return domain
	}
	prefix := hostname + "."
	if domain, ok := strings.CutPrefix(fqdn, prefix); ok {
		return domain
	}
	_, domain, ok := strings.Cut(fqdn, ".")
	if !ok {
		return ""
	}
	return domain
}

func primaryIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ip, ok := ipFromAddr(addr)
		if !ok || ip.IsLoopback() {
			continue
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	return ""
}

// primaryIPv6 scans every interface address for a routable IPv6 address,
// preferring global scope over unique-local. It is the fallback when the
// primary interface carries no IPv6 bindings.
func primaryIPv6() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	return primaryIPv6FromAddrs(addrs)
}

func primaryIPv6FromAddrs(addrs []net.Addr) string {
	best := ""
	bestRank := 0
	for _, addr := range addrs {
		ip, ok := ipFromAddr(addr)
		if !ok {
			continue
		}
		rank, ok := ipv6SelectionRank(ip)
		if !ok || rank >= ipv6RankLinkLocal {
			// Link-local addresses are candidates only on the primary
			// interface, never in the any-interface fallback scan.
			continue
		}
		if best == "" || rank < bestRank {
			best, bestRank = ip.String(), rank
		}
		if bestRank == ipv6RankGlobal {
			break
		}
	}
	return best
}

// primaryIPv6Address selects the primary IPv6 address from the primary
// interface's IPv6 bindings, preferring global scope, then unique-local,
// then link-local. This is a deliberate, documented deviation from Ruby
// Facter's first-bound-address rule, which can surface fe80:: link-locals
// (see the man page GO PORT NOTES). The first binding wins within a rank, so
// the selection is deterministic regardless of binding order.
func primaryIPv6Address(interfaces map[string]any, primaryInterfaceName string) string {
	if primaryInterfaceName == "" {
		return ""
	}
	iface, ok := interfaces[primaryInterfaceName].(map[string]any)
	if !ok {
		return ""
	}
	bindings, ok := iface["bindings6"].([]any)
	if !ok {
		return ""
	}
	best := ""
	bestRank := 0
	for _, raw := range bindings {
		binding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		address, _ := binding["address"].(string)
		rank, ok := ipv6SelectionRank(net.ParseIP(address))
		if !ok {
			continue
		}
		if best == "" || rank < bestRank {
			best, bestRank = address, rank
		}
	}
	return best
}

const (
	ipv6RankGlobal = iota
	ipv6RankUniqueLocal
	ipv6RankLinkLocal
)

// ipv6SelectionRank orders candidate primary IPv6 addresses: global scope
// first, then unique-local (fc00::/7), then link-local (fe80::/10).
// Loopback, unspecified, and IPv4 addresses are not candidates.
func ipv6SelectionRank(ip net.IP) (int, bool) {
	if ip == nil || ip.To4() != nil || ip.IsLoopback() || ip.IsUnspecified() {
		return 0, false
	}
	ip = ip.To16()
	if ip == nil {
		return 0, false
	}
	switch {
	case ip.IsLinkLocalUnicast():
		return ipv6RankLinkLocal, true
	case ip[0]&0xfe == 0xfc:
		return ipv6RankUniqueLocal, true
	default:
		return ipv6RankGlobal, true
	}
}

func ipFromAddr(addr net.Addr) (net.IP, bool) {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP, true
	case *net.IPAddr:
		return v.IP, true
	default:
		return nil, false
	}
}

// networkingCoreFacts assembles the networking category facts (hostname/fqdn/
// domain, interfaces, primary interface and address selection, DHCP, and the
// IPv4/IPv6 binding facts) for the current host.
func networkingCoreFacts(s *Session) []ResolvedFact {
	goos := s.goos()
	if goos == "plan9" {
		return plan9NetworkingCoreFacts(s)
	}
	nodeName, nodeNameValue := hostName(s)
	resolvedFQDN := fqdn(nodeName)
	hostname, fqdn, domain := currentHostnameFacts(goos, nodeName, resolvedFQDN, "/etc/resolv.conf", s.readFile)
	var hostnameValue any
	if nodeNameValue != nil {
		hostnameValue = hostname
	}
	fqdnValue, domainValue := hostnameFactValues(hostnameValue, fqdn, domain)
	ipv4 := primaryIPv4()
	interfaces := networkingInterfaces(s)
	configuredPrimary, interfaces := currentNetworkingData(goos, interfaces, s.commandOutput, s.readFile)
	if goos == "windows" {
		domain = currentWindowsNetworkingDomain(interfaces, s.commandOutput)
		fqdn = windowsFQDN(hostname, domain)
		fqdnValue, domainValue = hostnameFactValues(hostnameValue, fqdn, domain)
	}
	primaryInterfaceName := configuredPrimary
	if primaryInterfaceName == "" {
		primaryInterfaceName = primaryInterface(interfaces, ipv4)
	}
	if ipv4 == "" {
		ipv4, _ = primaryInterfaceFact(interfaces, primaryInterfaceName, "ip").(string)
	}
	ipv6 := primaryIPv6Address(interfaces, primaryInterfaceName)
	if ipv6 == "" {
		ipv6 = primaryIPv6()
	}
	primaryBinding := primaryIPv4Binding(interfaces, ipv4)
	primaryNetmask, _ := primaryBinding["netmask"].(string)
	primaryNetwork, _ := primaryBinding["network"].(string)
	ipv6Binding := primaryIPv6Binding(interfaces, ipv6)
	primaryNetmask6, _ := ipv6Binding["netmask"].(string)
	primaryNetwork6, _ := ipv6Binding["network"].(string)
	primaryScope6 := primaryIPv6Scope(interfaces, ipv6)
	primaryMAC, _ := primaryInterfaceFact(interfaces, primaryInterfaceName, "mac").(string)
	primaryMTU := primaryInterfaceFact(interfaces, primaryInterfaceName, "mtu")
	primaryDHCP := networkingDHCPValue(goos, interfaces, ipv4)
	return []ResolvedFact{
		{Name: "networking.hostname", Value: hostnameValue},
		{Name: "networking.fqdn", Value: fqdnValue},
		{Name: "networking.domain", Value: domainValue},
		{Name: "networking.dhcp", Value: primaryDHCP},
		{Name: "networking.ip", Value: optionalNetworkingString(ipv4)},
		{Name: "networking.ip6", Value: optionalNetworkingString(ipv6)},
		{Name: "networking.interfaces", Value: interfaces},
		{Name: "networking.mac", Value: optionalNetworkingString(primaryMAC)},
		{Name: "networking.mtu", Value: primaryMTU},
		{Name: "networking.netmask", Value: optionalNetworkingString(primaryNetmask)},
		{Name: "networking.netmask6", Value: optionalNetworkingString(primaryNetmask6)},
		{Name: "networking.network", Value: optionalNetworkingString(primaryNetwork)},
		{Name: "networking.network6", Value: optionalNetworkingString(primaryNetwork6)},
		{Name: "networking.primary", Value: optionalNetworkingString(primaryInterfaceName)},
		{Name: "networking.scope6", Value: optionalNetworkingString(primaryScope6)},
	}
}
