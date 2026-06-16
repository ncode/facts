package engine

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	osuser "os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// Version is overridden at release time via
// -ldflags "-X github.com/ncode/facts/internal/engine.Version=<git tag>".
// The literal below is the dev-build fallback when no tag is injected.
var Version = "dev" // ponytail: var, not const, so the build pipeline can inject the git tag

type freeBSDVersions struct {
	InstalledKernel   string
	RunningKernel     string
	InstalledUserland string
}

type freeBSDGeomMesh struct {
	Classes []freeBSDGeomClass `xml:"class"`
}

type freeBSDGeomClass struct {
	Name  string            `xml:"name"`
	Geoms []freeBSDGeomGeom `xml:"geom"`
}

type freeBSDGeomGeom struct {
	Providers []freeBSDGeomProvider `xml:"provider"`
}

type freeBSDGeomProvider struct {
	Name      string            `xml:"name"`
	MediaSize string            `xml:"mediasize"`
	Config    freeBSDGeomConfig `xml:"config"`
}

type freeBSDGeomConfig struct {
	Descr   string `xml:"descr"`
	Ident   string `xml:"ident"`
	Label   string `xml:"label"`
	RawUUID string `xml:"rawuuid"`
}

var aioAgentVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(\.\d+)?`)
var linuxSystemdDHCPServerPattern = regexp.MustCompile(`(?m)^SERVER_ADDRESS=(\S+)`)
var linuxDHClientServerPattern = regexp.MustCompile(`dhcp-server-identifier\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`)
var linuxDHCPCDServerPattern = regexp.MustCompile(`(?m)^dhcp_server_identifier='([^']+)'`)
var openBSDDHCPServerPattern = regexp.MustCompile(`\sdhcp server (\S+)`)
var darwinDHCPServerPattern = regexp.MustCompile(`^[\d.a-f:\s]+$`)
var linuxKernelVersionPattern = regexp.MustCompile(`^\d+(?:\.\d+){0,2}`)
var bsdKernelVersionPattern = regexp.MustCompile(`^\d+(?:\.\d+)?`)
var mageiaReleasePattern = regexp.MustCompile(`Mageia release ([0-9.]+)`)
var openWrtReleasePattern = regexp.MustCompile(`^(\d+\.\d+.*)`)
var gentooReleasePattern = regexp.MustCompile(`release (\d[\d.]*)`)
var slackwareReleasePattern = regexp.MustCompile(`Slackware ([0-9.]+)`)
var amazonReleasePattern = regexp.MustCompile(`(?:release |Amazon Linux )(\d[\d.]*)`)
var photonReleasePattern = regexp.MustCompile(`DISTRIB_RELEASE="(\d+)\.(\d+)`)
var marinerReleasePattern = regexp.MustCompile(`CBL-Mariner ([0-9.]+)`)
var azureLinuxReleasePattern = regexp.MustCompile(`AZURELINUX_BUILD_NUMBER=([0-9.]+)`)
var linuxMintReleasePattern = regexp.MustCompile(`(?m)^RELEASE=(\d+)`)
var firstLineReleasePattern = regexp.MustCompile(`release\s+([0-9.]+)`)
var linuxLSBLKVersionPattern = regexp.MustCompile(`\b(\d+)\.(\d+)\b`)
var linuxDistroVersionWordsRE = regexp.MustCompile(`[A-Za-z]+\s[A-Za-z]+`)

type routeSourceBinding struct {
	Interface string
	IP        string
}

type processorInfo struct {
	ISA            string
	SpeedHz        int
	Models         []string
	LogicalCount   int
	PhysicalCount  int
	CoresPerSocket int
	ThreadsPerCore int
}

type freeBSDMemoryInfo struct {
	System map[string]any
	Swap   map[string]any
}

// CoreFacts returns the small cross-platform fact set used by the initial Go CLI.
func CoreFacts(s *Session) []ResolvedFact {
	return CoreFactsWithRuby(s, true)
}

// CoreFactsWithRuby returns core facts, optionally omitting facts that require Ruby.
func CoreFactsWithRuby(s *Session, includeRuby bool) []ResolvedFact {
	s.coreFacts.mu.Lock()
	defer s.coreFacts.mu.Unlock()
	if s.coreFacts.facts == nil {
		s.coreFacts.facts = make(map[bool][]ResolvedFact, 2)
	}
	if s.coreFacts.facts[includeRuby] == nil {
		s.coreFacts.facts[includeRuby] = buildCoreFacts(s, includeRuby)
	}
	return append([]ResolvedFact(nil), s.coreFacts.facts[includeRuby]...)
}

func buildCoreFacts(s *Session, includeRuby bool) []ResolvedFact {
	nodeName, nodeNameValue := hostName()
	resolvedFQDN := fqdn(nodeName)
	hostname, fqdn, domain := currentHostnameFacts(runtime.GOOS, nodeName, resolvedFQDN, "/etc/resolv.conf")
	var hostnameValue any
	if nodeNameValue != nil {
		hostnameValue = hostname
	}
	fqdnValue, domainValue := hostnameFactValues(hostnameValue, fqdn, domain)
	ipv4 := primaryIPv4()
	interfaces := networkingInterfaces(s)
	configuredPrimary, interfaces := currentNetworkingData(runtime.GOOS, interfaces, s.commandOutput)
	if runtime.GOOS == "windows" {
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
	primaryDHCP := networkingDHCPFact(interfaces, ipv4)
	hardwareModel := s.cachedHardwareModel()
	architecture := s.cachedArchitectureName()
	linuxDistro := s.cachedLinuxDistro()
	osFamily := osFamily(runtime.GOOS, linuxDistro)
	osName := osName(runtime.GOOS, linuxDistro)
	kernelName := kernelName(runtime.GOOS)
	kernelRelease := s.cachedKernelRelease()
	osRelease := s.cachedOSRelease()
	kernelVersion := kernelVersionFact(runtime.GOOS, kernelRelease, "")
	kernelMajorVersion := kernelMajorVersionFact(runtime.GOOS, kernelRelease, osRelease)
	if runtime.GOOS == "windows" {
		kernelFacts := currentWindowsKernelFacts(s.cachedWindowsOSVersionInput())
		for _, fact := range kernelFacts {
			switch fact.Name {
			case "kernelrelease":
				kernelRelease, _ = fact.Value.(string)
			case "kernelversion":
				kernelVersion, _ = fact.Value.(string)
			case "kernelmajversion":
				kernelMajorVersion, _ = fact.Value.(string)
			}
		}
	}
	aioVersion := currentAioAgentVersion(runtime.GOOS, s.commandOutput)
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
	loadAverages := s.cachedLoadAverages()
	platformProcessors := processorInfo{}
	if runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" || runtime.GOOS == "windows" {
		platformProcessors = s.cachedPlatformProcessorInfo()
	}
	if runtime.GOOS == "linux" {
		platformProcessors.PhysicalCount = currentLinuxProcessorPhysicalCount("/proc/cpuinfo", "/sys/devices/system/cpu")
	}
	processorCount := runtime.NumCPU()
	if platformProcessors.LogicalCount > 0 {
		processorCount = platformProcessors.LogicalCount
	}
	physicalProcessorCount := processorCount
	if platformProcessors.PhysicalCount > 0 {
		physicalProcessorCount = platformProcessors.PhysicalCount
	}
	processorISA := currentProcessorISA(s, runtime.GOOS, architecture, s.commandOutput)
	processorModels := s.cachedProcessorModels()
	processorSpeed := s.cachedProcessorSpeed()
	processorCores, processorThreads := s.cachedProcessorTopology()
	processorExtensions := s.cachedProcessorExtensions()
	uptime := s.cachedUptime()
	virtualization := detectVirtualization(s)
	virtualFact, isVirtualFact := virtualizationFactValues(virtualization)
	disks := currentDisks(runtime.GOOS, s.commandOutput)
	dmi := dmiFact("/sys/class/dmi/id")
	var mountEntries []mountEntry
	if runtime.GOOS == "linux" {
		mountEntries = currentMountEntries(s)
	}
	mountpoints := rootMountpoint(s)
	identity := identityFact(s)
	facts := []ResolvedFact{
		{Name: "facterversion", Value: Version},
		{Name: "identity", Value: identity},
		{Name: "is_virtual", Value: isVirtualFact},
		{Name: "kernel", Value: kernelName},
		{Name: "kernelmajversion", Value: kernelMajorVersion},
		{Name: "kernelrelease", Value: kernelRelease},
		{Name: "kernelversion", Value: kernelVersion},
		{Name: "load_averages", Value: loadAverages},
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
		{Name: "mountpoints", Value: mountpoints},
		{Name: "networking.hostname", Value: hostnameValue},
		{Name: "networking.fqdn", Value: fqdnValue},
		{Name: "networking.domain", Value: domainValue},
		{Name: "networking.dhcp", Value: primaryDHCP},
		{Name: "networking.ip", Value: ipv4},
		{Name: "networking.ip6", Value: ipv6},
		{Name: "networking.interfaces", Value: interfaces},
		{Name: "networking.mac", Value: primaryMAC},
		{Name: "networking.mtu", Value: primaryMTU},
		{Name: "networking.netmask", Value: primaryNetmask},
		{Name: "networking.netmask6", Value: primaryNetmask6},
		{Name: "networking.network", Value: primaryNetwork},
		{Name: "networking.network6", Value: primaryNetwork6},
		{Name: "networking.primary", Value: primaryInterfaceName},
		{Name: "networking.scope6", Value: primaryScope6},
		{Name: "os.architecture", Value: architecture},
		{Name: "os.family", Value: osFamily},
		{Name: "os.hardware", Value: hardwareModel},
		{Name: "os.name", Value: osName},
		{Name: "os.release", Value: osRelease},
		{Name: "path", Value: os.Getenv("PATH")},
		{Name: "processors.count", Value: processorCount},
		{Name: "processors.cores", Value: processorCores},
		{Name: "processors.extensions", Value: processorExtensions},
		{Name: "processors.isa", Value: processorISA},
		{Name: "processors.models", Value: processorModels},
		{Name: "processors.physicalcount", Value: physicalProcessorCount},
		{Name: "processors.threads", Value: processorThreads},
		{Name: "system_uptime.days", Value: int(uptime.Duration.Hours()) / 24},
		{Name: "system_uptime.hours", Value: int(uptime.Duration.Hours())},
		{Name: "system_uptime.seconds", Value: int(uptime.Duration.Seconds())},
		{Name: "system_uptime.uptime", Value: uptimeString(uptime)},
		{Name: "timezone", Value: currentTimezone(s, runtime.GOOS)},
		{Name: "virtual", Value: virtualFact},
	}
	facts = append(facts, augeasVersionFacts(s.cachedAugeasVersion())...)
	facts = append(facts, disksFacts(disks)...)
	facts = append(facts, dmiFacts(dmi)...)
	facts = append(facts, filesystemsFacts(s.cachedFilesystems())...)
	facts = append(facts, fipsEnabledFacts(runtime.GOOS, "/proc/sys/crypto/fips_enabled", s.commandOutput)...)
	facts = append(facts, partitionsFacts(partitionsFactWithMountEntries(currentPartitions(s), mountEntries, mountpoints))...)
	facts = append(facts, processorSpeedFacts(processorSpeed)...)
	facts = append(facts, currentLinuxHypervisorFacts(s)...)
	if includeRuby {
		facts = append(facts, rubyFacts(resolveRubyInfo())...)
	}
	if aioVersion != "" {
		facts = append(facts, ResolvedFact{Name: "aio_agent_version", Value: aioVersion})
	}
	facts = append(facts, selinuxFactsForPlatform(runtime.GOOS, "/proc/self/mounts", "/etc/selinux/config")...)
	facts = append(facts, linuxDistroFacts(linuxDistro)...)
	macOSInfo := s.cachedMacOSInfo()
	facts = append(facts, macOSVersionFacts(macOSInfo.ProductVersion, macOSInfo.ProductVersionExtra)...)
	facts = append(facts, macOSStringFact("os.macosx.product", macOSInfo.ProductName)...)
	facts = append(facts, macOSStringFact("os.macosx.build", macOSInfo.BuildVersion)...)
	facts = append(facts, macOSDMIFacts(s.cachedMacOSModel())...)
	facts = append(facts, macOSSystemProfilerFacts(s.cachedMacOSSystemProfilerHardware())...)
	facts = append(facts, macOSSystemProfilerSoftwareFacts(s.cachedMacOSSystemProfilerSoftware())...)
	facts = append(facts, macOSSystemProfilerEthernetFacts(s.cachedMacOSSystemProfilerEthernet())...)
	facts = append(facts, windowsSystem32Facts(currentWindowsSystem32(runtime.GOOS, os.Getenv("SystemRoot"), currentWindowsProcessWOW64))...)
	facts = append(facts, windowsProductReleaseFacts(currentWindowsProductRelease(runtime.GOOS, s.commandOutput))...)
	facts = append(facts, windowsDMIFacts(currentWindowsDMI(runtime.GOOS, s.commandOutput))...)
	facts = append(facts, currentWindowsHypervisorFacts(s)...)
	facts = append(facts, sshFactsForPlatformWithPrivilege(runtime.GOOS, identityPrivileged(identity), discoverSSHHostKeys)...)
	facts = append(facts, currentFreeBSDDMIFacts(s)...)
	facts = append(facts, currentOpenBSDDMIFacts(s)...)
	facts = append(facts, currentXenFacts()...)
	facts = append(facts, azureFacts(s.Context(), newAzureClient(azureMetadataBaseURL, nil), virtualization)...)
	facts = append(facts, ec2Facts(s, newEC2Client(ec2MetadataBaseURL, nil), virtualization)...)
	facts = append(facts, platformGCEFacts(s.Context(), runtime.GOOS, virtualization, dmiBIOSVendor(dmi), newGCEClient(gceMetadataBaseURL, nil))...)
	return facts
}

func virtualizationFactValues(v virtualization) (any, any) {
	if v.Unknown {
		return nil, nil
	}
	return v.Name, v.IsVirtual
}

func fipsEnabled(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

func currentFIPSEnabled(goos, linuxPath string, run commandRunner) bool {
	if goos == "windows" {
		return parseWindowsFIPSEnabled(run("reg", "query", `HKLM\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`, "/v", "Enabled"))
	}
	return fipsEnabled(linuxPath)
}

// fipsEnabledFacts resolves fips_enabled only on Linux and Windows, the
// platforms where Ruby Facter emits the fact; elsewhere the fact is absent
// instead of a placeholder false.
func fipsEnabledFacts(goos, linuxPath string, run commandRunner) []ResolvedFact {
	if goos != "linux" && goos != "windows" {
		return nil
	}
	return []ResolvedFact{{Name: "fips_enabled", Value: currentFIPSEnabled(goos, linuxPath, run)}}
}

func parseWindowsFIPSEnabled(input string) bool {
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 || fields[0] != "Enabled" || fields[1] != "REG_DWORD" {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimPrefix(fields[2], "0x"), 16, 64)
		return err == nil && value != 0
	}
	return false
}

func aioAgentVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return aioAgentVersionPattern.FindString(strings.TrimSpace(string(data)))
}

func currentAioAgentVersion(goos string, run commandRunner) string {
	if goos != "windows" {
		return aioAgentVersion("/opt/puppetlabs/puppet/VERSION")
	}

	const registryPath = `HKLM\SOFTWARE\Puppet Labs\Puppet`
	installDir, ok := parseWindowsRegistryStringValue(run("reg", "query", registryPath, "/v", "RememberedInstallDir64"), "RememberedInstallDir64")
	if !ok {
		debug("Could not read Puppet AIO path from 64 bit registry")
		installDir, ok = parseWindowsRegistryStringValue(run("reg", "query", registryPath, "/v", "RememberedInstallDir"), "RememberedInstallDir")
		if !ok {
			debug("Could not read Puppet AIO path from 32 bit registry")
		}
	}
	if !ok || installDir == "" {
		return ""
	}
	return aioAgentVersion(filepath.Join(installDir, "VERSION"))
}

type rubyInfo struct {
	Version  string
	Platform string
	Sitedir  string
}

func resolveRubyInfo() rubyInfo {
	cmd := exec.Command("ruby", "-rrbconfig", "-e", `puts RUBY_VERSION; puts RUBY_PLATFORM; puts RbConfig::CONFIG["sitedir"]; puts RbConfig::CONFIG["sitelibdir"]`)
	data, err := cmd.Output()
	if err != nil {
		return rubyInfo{}
	}
	return parseRubyInfo(string(data))
}

func parseRubyInfo(output string) rubyInfo {
	lines := strings.Split(strings.TrimRight(output, "\r\n"), "\n")
	if len(lines) < 4 {
		return rubyInfo{}
	}
	sitedir := strings.TrimSpace(lines[2])
	sitelibdir := strings.TrimSpace(lines[3])
	if sitedir == "" {
		sitelibdir = ""
	}
	return rubyInfo{
		Version:  strings.TrimSpace(lines[0]),
		Platform: strings.TrimSpace(lines[1]),
		Sitedir:  sitelibdir,
	}
}

func rubyFacts(info rubyInfo) []ResolvedFact {
	ruby := make(map[string]any, 3)
	if info.Version != "" {
		ruby["version"] = info.Version
	}
	if info.Platform != "" {
		ruby["platform"] = info.Platform
	}
	if info.Sitedir != "" {
		ruby["sitedir"] = info.Sitedir
	}
	if len(ruby) == 0 {
		return nil
	}
	return []ResolvedFact{{Name: "ruby", Value: ruby}}
}

// selinuxFactsForPlatform resolves os.selinux only on Linux, the only
// platform where Ruby Facter emits SELinux data; elsewhere the fact is
// absent.
func selinuxFactsForPlatform(goos, mountsPath, configPath string) []ResolvedFact {
	if goos != "linux" {
		return nil
	}
	return selinuxFacts(mountsPath, configPath)
}

func selinuxFacts(mountsPath, configPath string) []ResolvedFact {
	mountpoint := selinuxMountpoint(mountsPath)
	configMode, configPolicy, hasConfig := readSELinuxConfig(configPath)
	enabled := mountpoint != "" && hasConfig
	values := map[string]any{"enabled": enabled}
	if enabled {
		values["config_mode"] = configMode
		values["config_policy"] = configPolicy
		values["policy_version"] = readOptionalText(filepath.Join(mountpoint, "policyvers"))
		enforced := strings.TrimSpace(readText(filepath.Join(mountpoint, "enforce"))) == "1"
		values["enforced"] = enforced
		if enforced {
			values["current_mode"] = "enforcing"
		} else {
			values["current_mode"] = "permissive"
		}
	}

	coreNames := map[string]string{
		"config_mode":    "os.selinux.config_mode",
		"config_policy":  "os.selinux.config_policy",
		"current_mode":   "os.selinux.current_mode",
		"enabled":        "os.selinux.enabled",
		"enforced":       "os.selinux.enforced",
		"policy_version": "os.selinux.policy_version",
	}
	keys := []string{"config_mode", "config_policy", "current_mode", "enabled", "enforced", "policy_version"}
	core := make([]ResolvedFact, 0, len(values))
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		core = append(core, ResolvedFact{Name: coreNames[key], Value: value})
	}
	return core
}

func selinuxMountpoint(path string) string {
	for line := range strings.SplitSeq(readText(path), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "selinuxfs" {
			return fields[1]
		}
	}
	return ""
}

func readSELinuxConfig(path string) (mode, policy string, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return "", "", false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if value, found := strings.CutPrefix(line, "SELINUX="); found {
			mode = strings.TrimSpace(value)
		}
		if value, found := strings.CutPrefix(line, "SELINUXTYPE="); found {
			policy = strings.TrimSpace(value)
		}
	}
	return mode, policy, true
}

func readText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func readOptionalText(path string) any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.TrimSpace(string(data))
}

type sshHostKey struct {
	Name   string
	Type   string
	Key    string
	SHA1   string
	SHA256 string
}

func discoverSSHHostKeys() []sshHostKey {
	return discoverSSHHostKeysForPlatform(runtime.GOOS, os.Getenv("programdata"), os.ReadFile)
}

func discoverSSHHostKeysForPlatform(goos, programData string, readFile fileReader) []sshHostKey {
	const maxHostKeys = 20
	paths := []string{"/etc/ssh", "/usr/local/etc/ssh", "/etc", "/usr/local/etc", "/etc/opt/ssh"}
	if goos == "windows" {
		if programData == "" {
			return nil
		}
		paths = []string{filepath.Join(programData, "ssh")}
	}
	files := []string{"ssh_host_rsa_key.pub", "ssh_host_dsa_key.pub", "ssh_host_ecdsa_key.pub", "ssh_host_ed25519_key.pub"}
	keys := make([]sshHostKey, 0, len(files))
	for _, dir := range paths {
		for _, file := range files {
			data, err := readFile(filepath.Join(dir, file))
			if err != nil {
				continue
			}
			key, ok := parseSSHHostPublicKey(string(data))
			if !ok {
				continue
			}
			keys = append(keys, key)
			if len(keys) >= maxHostKeys {
				return keys
			}
		}
	}
	return keys
}

func parseSSHHostPublicKey(line string) (sshHostKey, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return sshHostKey{}, false
	}
	name, fingerprintAlgorithm, ok := sshKeyName(fields[0])
	if !ok {
		return sshHostKey{}, false
	}
	encodedKey := sshBase64Key(fields[1])
	if encodedKey == "" {
		return sshHostKey{}, false
	}
	decoded, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return sshHostKey{}, false
	}
	sha1Sum := sha1.Sum(decoded)
	sha256Sum := sha256.Sum256(decoded)
	return sshHostKey{
		Name:   name,
		Type:   fields[0],
		Key:    fields[1],
		SHA1:   fmt.Sprintf("SSHFP %d 1 %x", fingerprintAlgorithm, sha1Sum),
		SHA256: fmt.Sprintf("SSHFP %d 2 %x", fingerprintAlgorithm, sha256Sum),
	}, true
}

func sshBase64Key(key string) string {
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range key {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sshKeyName(keyType string) (string, int, bool) {
	switch keyType {
	case "ssh-rsa":
		return "rsa", 1, true
	case "ssh-dss":
		return "dsa", 2, true
	case "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521":
		return "ecdsa", 3, true
	case "ssh-ed25519":
		return "ed25519", 4, true
	default:
		return "", 0, false
	}
}

func sshFacts(keys []sshHostKey) []ResolvedFact {
	return sshFactsForPlatform("", keys)
}

func sshFactsForPlatform(goos string, keys []sshHostKey) []ResolvedFact {
	if len(keys) == 0 {
		if goos == "openbsd" {
			return []ResolvedFact{{Name: "ssh", Value: map[string]any{}}}
		}
		return []ResolvedFact{{Name: "ssh", Value: nil}}
	}
	structured := make(map[string]any, len(keys))
	for _, key := range keys {
		structured[key.Name] = map[string]any{
			"fingerprints": map[string]any{
				"sha1":   key.SHA1,
				"sha256": key.SHA256,
			},
			"key":  key.Key,
			"type": key.Type,
		}
	}
	return []ResolvedFact{{Name: "ssh", Value: structured}}
}

func sshFactsForPlatformWithPrivilege(goos string, privileged bool, discover func() []sshHostKey) []ResolvedFact {
	if goos == "windows" && !privileged {
		return []ResolvedFact{{Name: "ssh", Value: nil}}
	}
	return sshFactsForPlatform(goos, discover())
}

func identityPrivileged(identity map[string]any) bool {
	privileged, _ := identity["privileged"].(bool)
	return privileged
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

func dmiFact(root string) map[string]any {
	bios := mapFromDMI(root, map[string]string{
		"vendor":       "bios_vendor",
		"version":      "bios_version",
		"release_date": "bios_date",
	})
	board := mapFromDMI(root, map[string]string{
		"manufacturer":  "board_vendor",
		"product":       "board_name",
		"serial_number": "board_serial",
		"asset_tag":     "board_asset_tag",
	})
	chassis := mapFromDMI(root, map[string]string{
		"type":      "chassis_type",
		"asset_tag": "chassis_asset_tag",
	})
	product := mapFromDMI(root, map[string]string{
		"name":          "product_name",
		"version":       "product_version",
		"serial_number": "product_serial",
		"uuid":          "product_uuid",
	})

	dmi := make(map[string]any, 5)
	if len(bios) > 0 {
		dmi["bios"] = bios
	}
	if len(board) > 0 {
		dmi["board"] = board
	}
	if len(chassis) > 0 {
		dmi["chassis"] = chassis
	}
	if len(product) > 0 {
		dmi["product"] = product
	}
	if manufacturer := readDMIString(root, "sys_vendor"); manufacturer != "" {
		dmi["manufacturer"] = manufacturer
	}
	return dmi
}

// dmiFacts returns the dmi fact, or nothing when no DMI data resolved: an
// unresolvable fact is absent, never an empty map. Platform resolvers
// (macOS, Windows, BSD) still contribute their own dmi.* facts.
func dmiFacts(dmi map[string]any) []ResolvedFact {
	if len(dmi) == 0 {
		return nil
	}
	return []ResolvedFact{{Name: "dmi", Value: dmi}}
}

func dmiBIOSVendor(dmi map[string]any) string {
	bios, ok := dmi["bios"].(map[string]any)
	if !ok {
		return ""
	}
	vendor, _ := bios["vendor"].(string)
	return vendor
}

func currentFreeBSDDMIFacts(s *Session) []ResolvedFact {
	if runtime.GOOS != "freebsd" {
		return nil
	}
	values := make(map[string]string, len(freeBSDDMIKeys))
	for _, key := range freeBSDDMIKeys {
		values[key] = s.commandOutput("/bin/kenv", key)
	}
	return freeBSDDMIFacts(values)
}

func currentOpenBSDDMIFacts(s *Session) []ResolvedFact {
	if runtime.GOOS != "openbsd" {
		return nil
	}
	values := make(map[string]string, len(openBSDDMIKeys))
	for _, key := range openBSDDMIKeys {
		values[key] = s.commandOutput("/sbin/sysctl", "-n", key)
	}
	return openBSDDMIFacts(values)
}

var freeBSDDMIKeys = []string{
	"smbios.bios.reldate",
	"smbios.bios.vendor",
	"smbios.bios.version",
	"smbios.system.maker",
	"smbios.system.product",
	"smbios.system.serial",
	"smbios.system.uuid",
}

var openBSDDMIKeys = []string{
	"hw.vendor",
	"hw.product",
	"hw.version",
	"hw.serialno",
	"hw.uuid",
}

func freeBSDDMIFacts(values map[string]string) []ResolvedFact {
	dmi := make(map[string]any, 3)
	bios := mapFromValues(values, map[string]string{
		"vendor":       "smbios.bios.vendor",
		"version":      "smbios.bios.version",
		"release_date": "smbios.bios.reldate",
	})
	if len(bios) > 0 {
		dmi["bios"] = bios
	}
	product := mapFromValues(values, map[string]string{
		"name":          "smbios.system.product",
		"serial_number": "smbios.system.serial",
		"uuid":          "smbios.system.uuid",
	})
	if len(product) > 0 {
		dmi["product"] = product
	}
	if manufacturer := strings.TrimSpace(values["smbios.system.maker"]); manufacturer != "" {
		dmi["manufacturer"] = manufacturer
	}
	if len(dmi) == 0 {
		return nil
	}
	return []ResolvedFact{{Name: "dmi", Value: dmi}}
}

func openBSDDMIFacts(values map[string]string) []ResolvedFact {
	dmi := make(map[string]any, 3)
	bios := mapFromValues(values, map[string]string{
		"vendor":  "hw.vendor",
		"version": "hw.version",
	})
	if len(bios) > 0 {
		dmi["bios"] = bios
	}
	product := mapFromValues(values, map[string]string{
		"name":          "hw.product",
		"serial_number": "hw.serialno",
		"uuid":          "hw.uuid",
	})
	if len(product) > 0 {
		dmi["product"] = product
	}
	if manufacturer := strings.TrimSpace(values["hw.vendor"]); manufacturer != "" {
		dmi["manufacturer"] = manufacturer
	}
	if len(dmi) == 0 {
		return nil
	}
	return []ResolvedFact{{Name: "dmi", Value: dmi}}
}

func disksFact(root string) map[string]any {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	disks := make(map[string]any, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		deviceDir := filepath.Join(root, name, "device")
		info, err := os.Stat(deviceDir)
		if err != nil || !info.IsDir() {
			continue
		}

		disk := make(map[string]any, 5)
		if model := readSysfsString(root, name, "device/model"); model != "" {
			disk["model"] = model
		}
		if vendor := readSysfsString(root, name, "device/vendor"); vendor != "" {
			disk["vendor"] = vendor
		}
		if rotational := readSysfsString(root, name, "queue/rotational"); rotational != "" {
			if rotational == "0" {
				disk["type"] = "ssd"
			} else {
				disk["type"] = "hdd"
			}
		}
		if sectors, err := strconv.Atoi(readSysfsString(root, name, "size")); err == nil && sectors > 0 {
			sizeBytes := sectors * 512
			disk["size_bytes"] = sizeBytes
			disk["size"] = bytesToHumanReadable(sizeBytes)
		}
		if len(disk) > 0 {
			disks[name] = disk
		}
	}
	if len(disks) == 0 {
		return nil
	}
	return disks
}

// disksFacts returns the disks fact, or nothing when device enumeration
// yields no entries: Ruby Facter omits the fact instead of emitting an empty
// map (the resting state on macOS).
func disksFacts(disks map[string]any) []ResolvedFact {
	if len(disks) == 0 {
		return nil
	}
	return []ResolvedFact{{Name: "disks", Value: disks}}
}

func currentDisks(goos string, run commandRunner) map[string]any {
	switch goos {
	case "freebsd":
		return parseFreeBSDGeomDisks(run("sysctl", "-n", "kern.geom.confxml"))
	case "linux":
		return currentLinuxDisks("/sys/block", run)
	default:
		return disksFact("/sys/block")
	}
}

func currentLinuxDisks(root string, run commandRunner) map[string]any {
	disks := disksFact(root)
	if len(disks) == 0 || run == nil {
		return disks
	}

	for _, name := range sortedKeys(disks) {
		disk, ok := disks[name].(map[string]any)
		if !ok {
			continue
		}
		for _, field := range []string{"serial", "wwn"} {
			if value := strings.TrimSpace(run("lsblk", "-dn", "-o", field, "/dev/"+name)); value != "" {
				disk[field] = value
			}
		}
	}
	return disks
}

func parseFreeBSDGeomDisks(input string) map[string]any {
	providers := freeBSDGeomProviders(input, "DISK")
	if len(providers) == 0 {
		return nil
	}

	disks := make(map[string]any, len(providers))
	for _, provider := range providers {
		name := strings.TrimSpace(provider.Name)
		if name == "" {
			continue
		}
		disk := make(map[string]any, 4)
		if model := strings.TrimSpace(provider.Config.Descr); model != "" {
			disk["model"] = model
		}
		if serialNumber := strings.TrimSpace(provider.Config.Ident); serialNumber != "" {
			disk["serial_number"] = serialNumber
		}
		addFreeBSDGeomSize(disk, provider.MediaSize)
		disks[name] = disk
	}
	if len(disks) == 0 {
		return nil
	}
	return disks
}

var augeasVersionPattern = regexp.MustCompile(`\b(\d+\.\d+(?:\.\d+)?)\b`)

func probeAugeasVersion(s *Session) string {
	return currentAugeasVersion(fileExists, func(name string, args ...string) string {
		out, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			return ""
		}
		return string(out)
	})
}

func currentAugeasVersion(exists func(string) bool, run commandRunner) string {
	augparse := "augparse"
	if exists("/opt/puppetlabs/puppet/bin/augparse") {
		augparse = "/opt/puppetlabs/puppet/bin/augparse"
	}
	return parseAugeasVersion(run(augparse, "--version"))
}

func parseAugeasVersion(out string) string {
	return augeasVersionPattern.FindString(out)
}

func augeasFacts(out string) []ResolvedFact {
	return augeasVersionFacts(parseAugeasVersion(out))
}

// augeasVersionFacts returns the augeas fact, or nothing when no augparse
// binary produced a version: Ruby Facter omits the fact instead of emitting
// an empty version string.
func augeasVersionFacts(version string) []ResolvedFact {
	if version == "" {
		return nil
	}
	return []ResolvedFact{
		{Name: "augeas.version", Value: version},
	}
}

func currentXenFacts() []ResolvedFact {
	vm := detectXenVM()
	var domains []string
	if vm == "xen0" {
		domains = detectXenDomains()
	}
	return xenFacts(vm, domains)
}

func xenFacts(vm string, domains []string) []ResolvedFact {
	if vm != "xen0" {
		return []ResolvedFact{{Name: "xen", Value: nil}}
	}
	if domains == nil {
		domains = []string{}
	}
	return []ResolvedFact{
		{Name: "xen", Value: map[string]any{"domains": domains}},
	}
}

func detectXenVM() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	if strings.Contains(readFileString("/proc/xen/capabilities"), "control_d") {
		return "xen0"
	}
	return detectXenVMFromSignals(fileExists("/dev/xen/evtchn"), dirExists("/proc/xen"), fileExists("/dev/xvda1"), isSymlink("/dev/xvda1"))
}

func detectXenVMFromSignals(evtchn, procXen, xvda1, xvda1Symlink bool) string {
	if evtchn {
		return "xen0"
	}
	if procXen || (xvda1 && !xvda1Symlink) {
		return "xenu"
	}
	return ""
}

func detectXenDomains() []string {
	bin := selectXenCommand(fileExists)
	if bin == "" {
		return nil
	}
	out, err := exec.Command(bin, "list").Output()
	if err != nil {
		return nil
	}
	return parseXenDomains(string(out))
}

func selectXenCommand(exists func(string) bool) string {
	const toolstack = "/usr/lib/xen-common/bin/xen-toolstack"
	commands := []string{"/usr/sbin/xl", "/usr/sbin/xm"}

	stacks := 0
	for _, command := range commands {
		if exists(command) {
			stacks++
		}
	}
	if stacks > 1 && exists(toolstack) {
		return toolstack
	}
	for _, command := range commands {
		if exists(command) {
			return command
		}
	}
	return ""
}

func parseXenDomains(out string) []string {
	domains := make([]string, 0)
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "Name" || fields[0] == "Domain-0" {
			continue
		}
		domains = append(domains, fields[0])
	}
	return domains
}

func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

func currentPartitions(s *Session) map[string]any {
	switch runtime.GOOS {
	case "freebsd":
		return parseFreeBSDGeomPartitions(s.commandOutput("sysctl", "-n", "kern.geom.confxml"))
	case "linux":
		return currentLinuxPartitions("/sys/class/block", s.commandOutput)
	default:
		return nil
	}
}

func currentLinuxPartitions(root string, run commandRunner) map[string]any {
	partitions := discoverPartitions(root)
	if len(partitions) == 0 || run == nil {
		return partitions
	}

	major, minor, ok := linuxLSBLKVersion(run("lsblk", "--version"))
	if !ok || !linuxVersionAtLeast(major, minor, 2, 23) {
		return partitions
	}

	fields := "NAME,FSTYPE,UUID,LABEL,PARTUUID,PARTLABEL"
	if linuxVersionAtLeast(major, minor, 2, 25) {
		fields += ",PARTTYPE"
	}
	lsblkInfo := parseLinuxLSBLKProperties(run("lsblk", "-p", "-P", "-o", fields))
	for _, name := range sortedKeys(partitions) {
		partition, ok := partitions[name].(map[string]any)
		if !ok {
			continue
		}
		addLinuxPartitionMetadata(partition, lsblkInfo[name])
	}
	return partitions
}

func discoverPartitions(root string) map[string]any {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	partitions := make(map[string]any, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if _, err := os.Stat(filepath.Join(root, name, "partition")); err != nil {
			if _, err := os.Stat(filepath.Join(root, name, "dm")); err == nil {
				device := "/dev/" + name
				if mapName := readSysfsString(root, filepath.Join(name, "dm"), "name"); mapName != "" {
					device = "/dev/mapper/" + mapName
				}
				partition := make(map[string]any, 2)
				addLinuxPartitionSize(partition, root, name)
				partitions[device] = partition
				continue
			}
			if _, err := os.Stat(filepath.Join(root, name, "loop")); err == nil {
				partition := make(map[string]any, 3)
				if backingFile := readSysfsString(root, filepath.Join(name, "loop"), "backing_file"); backingFile != "" {
					partition["backing_file"] = backingFile
				}
				addLinuxPartitionSize(partition, root, name)
				partitions["/dev/"+name] = partition
				continue
			}
			continue
		}

		partition := make(map[string]any, 2)
		addLinuxPartitionSize(partition, root, name)
		partitions["/dev/"+name] = partition
	}
	if len(partitions) == 0 {
		return nil
	}
	return partitions
}

func addLinuxPartitionSize(partition map[string]any, root, name string) {
	sectors, err := strconv.Atoi(readSysfsString(root, name, "size"))
	if err != nil || sectors < 0 {
		sectors = 0
	}
	sizeBytes := sectors * 512
	partition["size_bytes"] = sizeBytes
	partition["size"] = bytesToHumanReadable(sizeBytes)
}

func linuxLSBLKVersion(output string) (int, int, bool) {
	match := linuxLSBLKVersionPattern.FindStringSubmatch(output)
	if match == nil {
		return 0, 0, false
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

func linuxVersionAtLeast(major, minor, wantMajor, wantMinor int) bool {
	return major > wantMajor || major == wantMajor && minor >= wantMinor
}

func parseLinuxLSBLKProperties(output string) map[string]map[string]string {
	rows := make(map[string]map[string]string)
	for line := range strings.Lines(output) {
		values := parseLinuxLSBLKPropertyLine(strings.TrimSpace(line))
		name := values["NAME"]
		if name == "" {
			continue
		}
		delete(values, "NAME")
		if len(values) > 0 {
			rows[name] = values
		}
	}
	return rows
}

func parseLinuxLSBLKPropertyLine(line string) map[string]string {
	values := map[string]string{}
	for i := 0; i < len(line); {
		for i < len(line) && line[i] == ' ' {
			i++
		}
		keyStart := i
		for i < len(line) && line[i] != '=' && line[i] != ' ' {
			i++
		}
		if keyStart == i || i >= len(line) || line[i] != '=' {
			break
		}
		key := line[keyStart:i]
		i++

		var value string
		if i < len(line) && line[i] == '"' {
			i++
			var builder strings.Builder
			for i < len(line) {
				switch line[i] {
				case '\\':
					if i+1 < len(line) {
						i++
						builder.WriteByte(line[i])
					}
				case '"':
					i++
					value = builder.String()
					goto parsedValue
				default:
					builder.WriteByte(line[i])
				}
				i++
			}
			value = builder.String()
		} else {
			valueStart := i
			for i < len(line) && line[i] != ' ' {
				i++
			}
			value = line[valueStart:i]
		}

	parsedValue:
		if value != "" {
			values[key] = value
		}
	}
	return values
}

func addLinuxPartitionMetadata(partition map[string]any, metadata map[string]string) {
	if len(metadata) == 0 {
		return
	}
	if value := metadata["FSTYPE"]; value != "" {
		partition["filesystem"] = value
	}
	if value := metadata["UUID"]; value != "" {
		partition["uuid"] = value
	}
	if value := metadata["LABEL"]; value != "" {
		partition["label"] = value
	}
	if value := metadata["PARTUUID"]; value != "" {
		partition["partuuid"] = value
	}
	if value := metadata["PARTLABEL"]; value != "" {
		partition["partlabel"] = value
	}
	if value := metadata["PARTTYPE"]; value != "" {
		partition["parttype"] = value
	}
}

func parseFreeBSDGeomPartitions(input string) map[string]any {
	providers := freeBSDGeomProviders(input, "PART")
	if len(providers) == 0 {
		return nil
	}

	partitions := make(map[string]any, len(providers))
	for _, provider := range providers {
		name := strings.TrimSpace(provider.Name)
		if name == "" {
			continue
		}
		partition := make(map[string]any, 4)
		if label := strings.TrimSpace(provider.Config.Label); label != "" {
			partition["partlabel"] = label
		}
		if rawUUID := strings.TrimSpace(provider.Config.RawUUID); rawUUID != "" {
			partition["partuuid"] = rawUUID
		}
		addFreeBSDGeomSize(partition, provider.MediaSize)
		partitions[name] = partition
	}
	if len(partitions) == 0 {
		return nil
	}
	return partitions
}

func freeBSDGeomProviders(input, className string) []freeBSDGeomProvider {
	var mesh freeBSDGeomMesh
	if err := xml.Unmarshal([]byte(input), &mesh); err != nil {
		return nil
	}
	var providers []freeBSDGeomProvider
	for _, class := range mesh.Classes {
		if strings.TrimSpace(class.Name) != className {
			continue
		}
		for _, geom := range class.Geoms {
			providers = append(providers, geom.Providers...)
		}
	}
	return providers
}

func addFreeBSDGeomSize(values map[string]any, mediaSize string) {
	sizeBytes, err := strconv.Atoi(strings.TrimSpace(mediaSize))
	if err != nil {
		return
	}
	values["size_bytes"] = sizeBytes
	values["size"] = bytesToHumanReadable(sizeBytes)
}

func partitionsFact(partitions, mountpoints map[string]any) map[string]any {
	return partitionsFactWithMountEntries(partitions, nil, mountpoints)
}

// partitionsFacts returns the partitions fact, or nothing when device
// enumeration yields no entries: Ruby Facter omits the fact instead of
// emitting an empty map (the resting state on macOS).
func partitionsFacts(partitions map[string]any) []ResolvedFact {
	if len(partitions) == 0 {
		return nil
	}
	return []ResolvedFact{{Name: "partitions", Value: partitions}}
}

func partitionsFactWithMountEntries(partitions map[string]any, mountEntries []mountEntry, mountpoints map[string]any) map[string]any {
	if len(partitions) == 0 {
		return nil
	}
	if len(mountpoints) == 0 {
		return partitions
	}

	if len(mountEntries) > 0 {
		for _, entry := range mountEntries {
			if skipMountEntry(entry) {
				continue
			}
			mountpoint, ok := mountpoints[entry.Path].(map[string]any)
			if !ok {
				continue
			}
			addPartitionMount(partitions, entry.Path, mountpoint)
		}
		return partitions
	}

	for _, path := range sortedKeys(mountpoints) {
		mountpoint, ok := mountpoints[path].(map[string]any)
		if !ok {
			continue
		}
		addPartitionMount(partitions, path, mountpoint)
	}
	return partitions
}

func addPartitionMount(partitions map[string]any, path string, mountpoint map[string]any) {
	device, _ := mountpoint["device"].(string)
	partition, ok := partitions[device].(map[string]any)
	if !ok || partition["mount"] != nil {
		return
	}
	partition["mount"] = path
}

func readSysfsString(root, device, name string) string {
	data, err := os.ReadFile(filepath.Join(root, device, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func mapFromDMI(root string, names map[string]string) map[string]any {
	values := make(map[string]any, len(names))
	for key, filename := range names {
		if value := readDMIString(root, filename); value != "" {
			if filename == "chassis_type" {
				value = dmiChassisTypeName(value)
			}
			values[key] = value
		}
	}
	return values
}

func mapFromValues(source map[string]string, names map[string]string) map[string]any {
	values := make(map[string]any, len(names))
	for key, name := range names {
		if value := strings.TrimSpace(source[name]); value != "" {
			values[key] = value
		}
	}
	return values
}

func dmiChassisTypeName(value string) string {
	types := []string{
		"Other", "", "Desktop", "Low Profile Desktop", "Pizza Box", "Mini Tower", "Tower",
		"Portable", "Laptop", "Notebook", "Hand Held", "Docking Station", "All in One", "Sub Notebook",
		"Space-Saving", "Lunch Box", "Main System Chassis", "Expansion Chassis", "SubChassis",
		"Bus Expansion Chassis", "Peripheral Chassis", "Storage Chassis", "Rack Mount Chassis",
		"Sealed-Case PC", "Multi-system", "CompactPCI", "AdvancedTCA", "Blade", "Blade Enclosure",
		"Tablet", "Convertible", "Detachable", "IoT Gateway", "Embedded PC", "Mini PC", "Stick PC",
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > len(types) {
		return value
	}
	return types[n-1]
}

func readDMIString(root, name string) string {
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ToValidUTF8(string(data), "\uFFFD"))
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
	return networkingInterfacesForPlatform(s, runtime.GOOS, currentNetworkInterfaceSnapshots)
}

func networkingInterfacesForPlatform(s *Session, goos string, snapshotProvider func() ([]networkInterfaceSnapshot, error)) map[string]any {
	snapshots, err := snapshotProvider()
	if err != nil {
		if goos == "windows" {
			debug("Unable to retrieve networking facts!")
		}
		return nil
	}

	values := networkingInterfacesFromSnapshots(snapshots, goos)
	if goos == "linux" {
		addLinuxDHCPServersFromSnapshots(s, values, snapshots)
		addLinuxRouteSourceBindings(values)
		addLinuxIfInet6Flags(values, parseLinuxIfInet6Flags(readText("/proc/net/if_inet6")))
		addLinuxBondingSlaveMACsFromRoot("/", values)
		addLinuxInterfaceMetadataFromRoot("/", values)
	}
	return values
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

		value := map[string]any{"mtu": iface.MTU}
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
		// appear with their MTU, matching Ruby's getifaddrs-driven map.
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

type commandRunner func(name string, args ...string) string

func currentNetworkingData(goos string, interfaces map[string]any, run commandRunner) (string, map[string]any) {
	switch goos {
	case "darwin":
		addDarwinDHCPServers(interfaces, run)
		expandInterfaceBindings(interfaces)
		return primaryInterfaceFromRoute(run("route", "-n", "get", "default")), interfaces
	case "freebsd":
		expandInterfaceBindings(interfaces)
		return primaryInterfaceFromRoute(run("route", "-n", "get", "default")), interfaces
	case "openbsd":
		addOpenBSDDHCPServers(interfaces, run)
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
		return linuxPrimaryInterface(readText("/proc/net/route"), interfaces, run), interfaces
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
	for bindingFact, ifaceFact := range factKeys {
		if value := binding[bindingFact]; value != nil && value != "" {
			iface[ifaceFact] = value
		}
	}
}

func addLinuxRouteSourceBindings(interfaces map[string]any) {
	if len(interfaces) == 0 {
		return
	}
	if output, err := exec.Command("ip", "route", "show").Output(); err == nil {
		addRouteSourceBindings(interfaces, "bindings", linuxRouteSourceBindings(string(output)))
	}
	if output, err := exec.Command("ip", "-6", "route", "show").Output(); err == nil {
		addRouteSourceBindings(interfaces, "bindings6", linuxRouteSourceBindings(string(output)))
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

func addLinuxInterfaceMetadataFromRoot(root string, interfaces map[string]any) {
	for name, raw := range interfaces {
		iface, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ifaceRoot := rootedPath(root, filepath.Join("sys/class/net", name))
		if state := strings.TrimSpace(readText(filepath.Join(ifaceRoot, "operstate"))); state != "" {
			iface["operational_state"] = state
		}
		_, err := os.Stat(filepath.Join(ifaceRoot, "device"))
		iface["physical"] = err == nil
		if speed, err := strconv.Atoi(strings.TrimSpace(readText(filepath.Join(ifaceRoot, "speed")))); err == nil {
			iface["speed"] = speed
		}
		if duplex := strings.TrimSpace(readText(filepath.Join(ifaceRoot, "duplex"))); duplex != "" {
			iface["duplex"] = duplex
		}
	}
}

func addLinuxBondingSlaveMACsFromRoot(root string, interfaces map[string]any) {
	entries, err := os.ReadDir(rootedPath(root, "proc/net/bonding"))
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for slave, mac := range parseLinuxBondingSlaveMACs(readText(rootedPath(root, filepath.Join("proc/net/bonding", entry.Name())))) {
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
	return linuxDHCPServerFromRoot(s, "/", interfaceName, interfaceIndex)
}

func linuxDHCPServerFromRoot(s *Session, root, interfaceName string, interfaceIndex int) string {
	return linuxDHCPServerFromRootWithRunner(root, interfaceName, interfaceIndex, s.commandOutput)
}

func linuxDHCPServerFromRootWithRunner(root, interfaceName string, interfaceIndex int, run commandRunner) string {
	if interfaceIndex > 0 {
		leasePath := rootedPath(root, filepath.Join("run/systemd/netif/leases", strconv.Itoa(interfaceIndex)))
		if server := linuxSystemdDHCPServer(readText(leasePath)); server != "" {
			return server
		}
	}
	for _, dir := range []string{"var/lib/dhclient", "var/lib/dhcp", "var/lib/dhcp3", "var/lib/NetworkManager", "var/db"} {
		server := linuxDHCPServerFromLeaseDir(rootedPath(root, dir), interfaceName)
		if server != "" {
			return server
		}
	}
	if server := linuxDHCPCDDHCPServer(run("dhcpcd", "-U", interfaceName)); server != "" {
		return server
	}
	return ""
}

func linuxDHCPServerFromLeaseDir(dir, interfaceName string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.Contains(name, "lease") {
			continue
		}
		content := readText(filepath.Join(dir, name))
		if !leaseMatchesInterface(name, content, interfaceName) {
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

func leaseMatchesInterface(name, content, interfaceName string) bool {
	if strings.Contains(content, "interface") && strings.Contains(content, interfaceName) {
		return true
	}
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
	match := linuxDHClientServerPattern.FindStringSubmatch(content)
	if len(match) != 2 {
		return ""
	}
	return match[1]
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

func rootedPath(root, path string) string {
	if root == "/" {
		return "/" + strings.TrimPrefix(path, "/")
	}
	return filepath.Join(root, path)
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

type mountEntry struct {
	Device     string
	Path       string
	Filesystem string
	Options    []string
}

type mountStat struct {
	SizeBytes      int
	AvailableBytes int
	UsedBytes      int
}

func rootMountpoint(s *Session) map[string]any {
	if runtime.GOOS == "openbsd" {
		return currentOpenBSDMountpoints()
	}

	entries := currentMountEntries(s)
	if len(entries) == 0 {
		entries = []mountEntry{{Path: "/"}}
	}
	if runtime.GOOS == "darwin" {
		return darwinMountpointsFact(entries, statMountpoint)
	}
	return mountpointsFact(entries, statMountpoint)
}

func currentOpenBSDMountpoints() map[string]any {
	mountOutput, err := exec.Command("mount").Output()
	if err != nil {
		return mountpointsFact([]mountEntry{{Path: "/"}}, statMountpoint)
	}
	dfOutput, _ := exec.Command("df", "-P").Output()
	return openBSDMountpointsFact(string(mountOutput), string(dfOutput))
}

func currentMountEntries(s *Session) []mountEntry {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("mount").Output()
		if err != nil {
			return nil
		}
		return parseDarwinMountEntries(string(out))
	case "freebsd":
		out, err := exec.Command("mount").Output()
		if err != nil {
			return nil
		}
		return parseFreeBSDMountEntries(string(out))
	case "linux":
		data, err := os.ReadFile("/proc/self/mounts")
		if err != nil {
			return nil
		}
		return linuxMountEntriesWithRootDevice(parseLinuxMountEntries(string(data)), os.ReadFile, s.commandOutput)
	default:
		return []mountEntry{{Path: "/"}}
	}
}

func parseLinuxMountEntries(input string) []mountEntry {
	entries := make([]mountEntry, 0, strings.Count(input, "\n"))
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		entries = append(entries, mountEntry{
			Device:     unescapeMountField(fields[0]),
			Path:       unescapeMountField(fields[1]),
			Filesystem: fields[2],
			Options:    strings.Split(fields[3], ","),
		})
	}
	return entries
}

func linuxMountEntriesWithRootDevice(entries []mountEntry, readFile fileReader, run commandRunner) []mountEntry {
	needsRoot := false
	for _, entry := range entries {
		if entry.Device == "/dev/root" {
			needsRoot = true
			break
		}
	}
	if !needsRoot {
		return entries
	}

	root := resolveLinuxRootMountDevice(readFile, run)
	resolved := append([]mountEntry(nil), entries...)
	for i, entry := range resolved {
		if entry.Device == "/dev/root" {
			resolved[i].Device = root
		}
	}
	return resolved
}

func resolveLinuxRootMountDevice(readFile fileReader, run commandRunner) string {
	data, err := readFile("/proc/cmdline")
	if err != nil {
		return ""
	}
	root := rootFromLinuxCmdline(string(data))
	if !strings.Contains(root, "=") {
		return root
	}
	if device := linuxDeviceForPartitionID(root, run("blkid")); device != "" {
		return device
	}
	return root
}

func rootFromLinuxCmdline(input string) string {
	for field := range strings.FieldsSeq(input) {
		if root, ok := strings.CutPrefix(field, "root="); ok {
			return root
		}
	}
	return ""
}

func linuxDeviceForPartitionID(partitionID, blkidOutput string) string {
	_, id, ok := strings.Cut(partitionID, "=")
	if !ok || id == "" {
		return ""
	}
	for line := range strings.SplitSeq(blkidOutput, "\n") {
		if !strings.Contains(line, id) {
			continue
		}
		device, _, ok := strings.Cut(line, ":")
		if ok {
			return strings.TrimSpace(device)
		}
	}
	return ""
}

func parseDarwinMountEntries(input string) []mountEntry {
	entries := make([]mountEntry, 0, strings.Count(input, "\n"))
	for line := range strings.SplitSeq(input, "\n") {
		device, rest, ok := strings.Cut(line, " on ")
		if !ok {
			continue
		}
		path, rawOptions, ok := strings.Cut(rest, " (")
		if !ok {
			continue
		}
		rawOptions = strings.TrimSuffix(rawOptions, ")")
		fields := strings.Split(rawOptions, ",")
		if len(fields) == 0 {
			continue
		}
		filesystem := strings.TrimSpace(fields[0])
		options := make([]string, 0, len(fields)-1)
		for _, field := range fields[1:] {
			option := normalizeDarwinMountOption(strings.TrimSpace(field))
			if option != "" {
				options = append(options, option)
			}
		}
		entries = append(entries, mountEntry{Device: unescapeMountField(device), Path: unescapeMountField(path), Filesystem: filesystem, Options: options})
	}
	return entries
}

func parseFreeBSDMountEntries(input string) []mountEntry {
	entries := make([]mountEntry, 0, strings.Count(input, "\n"))
	for line := range strings.SplitSeq(input, "\n") {
		device, rest, ok := strings.Cut(line, " on ")
		if !ok {
			continue
		}
		path, rawOptions, ok := strings.Cut(rest, " (")
		if !ok {
			continue
		}
		rawOptions = strings.TrimSuffix(rawOptions, ")")
		fields := strings.Split(rawOptions, ",")
		if len(fields) == 0 {
			continue
		}
		filesystem := strings.TrimSpace(fields[0])
		options := make([]string, 0, len(fields)-1)
		for _, field := range fields[1:] {
			option := strings.TrimSpace(field)
			if option != "" {
				options = append(options, option)
			}
		}
		entries = append(entries, mountEntry{Device: unescapeMountField(device), Path: unescapeMountField(path), Filesystem: filesystem, Options: options})
	}
	return entries
}

func openBSDMountpointsFact(mountOutput, dfOutput string) map[string]any {
	stats := parseDFP512Stats(dfOutput)
	return mountpointsFact(parseOpenBSDMountEntries(mountOutput), func(path string) (mountStat, bool) {
		stat, ok := stats[path]
		return stat, ok
	})
}

func darwinMountpointsFact(entries []mountEntry, stat func(string) (mountStat, bool)) map[string]any {
	missingStats := make(map[string]bool)
	mountpoints := mountpointsFactWithSkip(entries, func(path string) (mountStat, bool) {
		stats, ok := stat(path)
		if !ok {
			missingStats[path] = true
			return mountStat{}, true
		}
		return stats, true
	}, skipMountEntry)
	for path := range missingStats {
		if mountpoint, ok := mountpoints[path].(map[string]any); ok {
			mountpoint["capacity"] = "100%"
		}
	}
	return mountpoints
}

func parseOpenBSDMountEntries(input string) []mountEntry {
	entries := make([]mountEntry, 0, strings.Count(input, "\n"))
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[1] != "on" || fields[3] != "type" {
			continue
		}
		options := make([]string, 0, len(fields)-5)
		for _, field := range fields[5:] {
			option := strings.Trim(field, "(),")
			if option != "" {
				options = append(options, option)
			}
		}
		entries = append(entries, mountEntry{Device: fields[0], Path: fields[2], Filesystem: fields[4], Options: options})
	}
	return entries
}

func parseDFP512Stats(input string) map[string]mountStat {
	stats := make(map[string]mountStat)
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[0] == "Filesystem" || fields[1] == "-" {
			continue
		}

		sizeBlocks, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		usedBlocks, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		availableBlocks, err := strconv.Atoi(fields[3])
		if err != nil {
			continue
		}
		path := fields[len(fields)-1]
		stats[path] = mountStat{
			SizeBytes:      sizeBlocks * 512,
			AvailableBytes: availableBlocks * 512,
			UsedBytes:      usedBlocks * 512,
		}
	}
	return stats
}

func normalizeDarwinMountOption(option string) string {
	switch option {
	case "read-only":
		return "readonly"
	case "asynchronous":
		return "async"
	case "synchronous":
		return "noasync"
	case "quotas":
		return "quota"
	case "rootfs":
		return "root"
	case "defwrite":
		return "deferwrites"
	default:
		return option
	}
}

func unescapeMountField(value string) string {
	replacer := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return replacer.Replace(value)
}

func mountpointsFact(entries []mountEntry, stat func(string) (mountStat, bool)) map[string]any {
	return mountpointsFactWithSkip(entries, stat, skipMountEntry)
}

func mountpointsFactWithSkip(entries []mountEntry, stat func(string) (mountStat, bool), skip func(mountEntry) bool) map[string]any {
	mountpoints := make(map[string]any, len(entries))
	for _, entry := range entries {
		if skip(entry) {
			continue
		}
		stats, ok := stat(entry.Path)
		mountpoint := make(map[string]any, 10)
		if ok {
			mountpoint["available"] = bytesToHumanReadable(stats.AvailableBytes)
			mountpoint["available_bytes"] = stats.AvailableBytes
			mountpoint["capacity"] = filesystemCapacity(stats.UsedBytes, stats.SizeBytes)
			mountpoint["size"] = bytesToHumanReadable(stats.SizeBytes)
			mountpoint["size_bytes"] = stats.SizeBytes
			mountpoint["used"] = bytesToHumanReadable(stats.UsedBytes)
			mountpoint["used_bytes"] = stats.UsedBytes
		}
		if entry.Device != "" {
			mountpoint["device"] = entry.Device
		}
		if entry.Filesystem != "" {
			mountpoint["filesystem"] = entry.Filesystem
		}
		if len(entry.Options) > 0 {
			mountpoint["options"] = append([]string(nil), entry.Options...)
		}
		mountpoints[entry.Path] = mountpoint
	}
	if len(mountpoints) == 0 {
		return nil
	}
	return mountpoints
}

func skipMountEntry(entry mountEntry) bool {
	return (strings.HasPrefix(entry.Path, "/proc") || strings.HasPrefix(entry.Path, "/sys")) && entry.Filesystem != "tmpfs" || entry.Filesystem == "autofs"
}

type identityInfo struct {
	User       string
	UID        string
	GID        string
	Group      string
	Privileged *bool
}

func identityFact(s *Session) map[string]any {
	if runtime.GOOS == "windows" {
		return identityFactFromInfo(runtime.GOOS, currentWindowsIdentityInfo(s.commandOutput))
	}

	privileged := os.Geteuid() == 0
	info := identityInfo{Privileged: &privileged}
	current, err := osuser.Current()
	if err != nil {
		return identityFactFromInfo(runtime.GOOS, info)
	}
	info.UID = current.Uid
	info.GID = current.Gid
	info.User = current.Username
	if group, err := osuser.LookupGroupId(current.Gid); err == nil {
		info.Group = group.Name
	}
	return identityFactFromInfo(runtime.GOOS, info)
}

func currentWindowsIdentityInfo(run commandRunner) identityInfo {
	info := identityInfo{}
	if run == nil {
		return info
	}
	info.User = strings.TrimSpace(run("whoami"))
	if info.User == "" {
		debug("failure resolving identity facts: ")
		return info
	}
	if privileged, ok := parseWindowsAdministratorGroups(run("whoami", "/groups")); ok {
		info.Privileged = &privileged
	}
	return info
}

func parseWindowsAdministratorGroups(output string) (bool, bool) {
	if strings.TrimSpace(output) == "" {
		return false, false
	}
	for line := range strings.SplitSeq(output, "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, `\administrators`) && !strings.Contains(lower, "s-1-5-32-544") {
			continue
		}
		return strings.Contains(lower, "enabled") && !strings.Contains(lower, "deny only"), true
	}
	return false, true
}

func identityFactFromInfo(goos string, info identityInfo) map[string]any {
	identity := make(map[string]any, 5)
	if info.Privileged != nil {
		identity["privileged"] = *info.Privileged
	}
	if info.User != "" {
		identity["user"] = info.User
	}
	if goos == "windows" {
		return identity
	}
	if info.UID != "" {
		identity["uid"] = numericIdentityValue(info.UID)
	}
	if info.GID != "" {
		identity["gid"] = numericIdentityValue(info.GID)
	}
	if info.Group != "" {
		identity["group"] = info.Group
	}
	return identity
}

func numericIdentityValue(value string) any {
	n, err := strconv.Atoi(value)
	if err != nil {
		return value
	}
	return n
}

func majorVersion(version string) string {
	major, _, _ := strings.Cut(version, ".")
	return major
}

func linuxMajorVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) <= 1 {
		return version
	}
	return parts[0] + "." + parts[1]
}

func kernelVersionFact(goos, kernelRelease, unameVersion string) string {
	switch goos {
	case "linux":
		if version := linuxKernelVersionPattern.FindString(kernelRelease); version != "" {
			return version
		}
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		if version := bsdKernelVersionPattern.FindString(kernelRelease); version != "" {
			return version
		}
	case "darwin":
		return kernelRelease
	}
	return majorVersion(kernelRelease)
}

func kernelMajorVersionFact(goos, kernelRelease string, osRelease any) string {
	switch goos {
	case "linux", "darwin":
		return linuxMajorVersion(kernelRelease)
	}
	return majorVersion(kernelRelease)
}

func (s *Session) commandOutput(name string, args ...string) string {
	data, err := exec.CommandContext(s.ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return string(data)
}

type uptimeInfo struct {
	Duration time.Duration
	Known    bool
}

func uptimeString(uptime uptimeInfo) string {
	if !uptime.Known {
		return "unknown"
	}
	totalHours := int(uptime.Duration.Hours())
	days := totalHours / 24
	if days == 0 {
		minutes := int(uptime.Duration.Minutes()) % 60
		return fmt.Sprintf("%d:%02d hours", totalHours, minutes)
	}
	if days == 1 {
		return "1 day"
	}
	return strconv.Itoa(days) + " days"
}

func bytesToMB(value any) any {
	number, ok := numericValue(value)
	if !ok {
		return nil
	}
	if number <= 0 {
		return 0.0
	}
	return number / (1024.0 * 1024.0)
}

func bytesToHumanReadable(value any) any {
	number, original, ok := byteValue(value)
	if !ok {
		return nil
	}
	if number < 0 {
		return ""
	}
	if number < 1024 {
		return formatByteNumber(number, original) + " bytes"
	}

	prefixes := [...]string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	exp := int(math.Floor(math.Log2(number) / 10.0))
	converted := math.Round(100.0*(number/math.Pow(1024.0, float64(exp)))) / 100.0
	if converted == 1024.0 {
		exp++
		converted = 1.0
	}
	if exp < 1 || exp > len(prefixes) {
		return formatByteNumber(number, original) + " bytes"
	}
	return strconv.FormatFloat(converted, 'f', 2, 64) + " " + prefixes[exp-1]
}

func numericValue(value any) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case string:
		return rubyToI(v), true
	default:
		return 0, false
	}
}

func rubyToI(value string) float64 {
	value = strings.TrimLeft(value, " \t\n\v\f\r")
	if value == "" {
		return 0
	}

	end := 0
	if value[0] == '+' || value[0] == '-' {
		end = 1
	}
	startDigits := end
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == startDigits {
		return 0
	}

	n, err := strconv.ParseInt(value[:end], 10, 64)
	if err != nil {
		return 0
	}
	return float64(n)
}

func byteValue(value any) (float64, string, bool) {
	switch v := value.(type) {
	case nil:
		return 0, "", false
	case int:
		return float64(v), strconv.Itoa(v), true
	case int64:
		return float64(v), strconv.FormatInt(v, 10), true
	case float64:
		return v, strconv.FormatFloat(v, 'f', -1, 64), true
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, v, err == nil
	default:
		return 0, "", false
	}
}

func formatByteNumber(number float64, original string) string {
	if original != "" && math.Abs(number) >= 1e18 {
		return original
	}
	return strconv.FormatFloat(number, 'f', -1, 64)
}

func memoryCapacity(used, total int) string {
	if total <= 0 || used <= 0 {
		return "0.00%"
	}
	percent := 100.0 * float64(used) / float64(total)
	return strconv.FormatFloat(percent, 'f', 2, 64) + "%"
}

type memorySwapFactValues struct {
	available      any
	availableBytes any
	capacity       any
	total          any
	totalBytes     any
	used           any
	usedBytes      any
}

// memorySwapValues omits the whole swap subtree when there is no swap
// (zero total bytes), matching Ruby Facter on every platform.
func memorySwapValues(totalBytes, availableBytes int) memorySwapFactValues {
	if totalBytes <= 0 {
		return memorySwapFactValues{}
	}
	usedBytes := max(0, totalBytes-availableBytes)
	return memorySwapFactValues{
		available:      bytesToHumanReadable(availableBytes),
		availableBytes: availableBytes,
		capacity:       memoryCapacity(usedBytes, totalBytes),
		total:          bytesToHumanReadable(totalBytes),
		totalBytes:     totalBytes,
		used:           bytesToHumanReadable(usedBytes),
		usedBytes:      usedBytes,
	}
}

func filesystemCapacity(used, total int) string {
	if total > 0 && used == total {
		return "100%"
	}
	if used <= 0 || total <= 0 {
		return "0%"
	}
	percent := 100.0 * float64(used) / float64(total)
	return strconv.FormatFloat(percent, 'f', 2, 64) + "%"
}

func architectureName(goos, machine string) string {
	if machine == "" {
		return runtime.GOARCH
	}
	if goos == "windows" {
		return windowsArchitectureFromHardware(machine)
	}
	if goos == "linux" {
		if linuxI386ArchitectureRE.MatchString(machine) {
			return "i386"
		}
	}
	return machine
}

func windowsHardwareArchitecture(processor string, level int) (string, string) {
	switch strings.ToUpper(strings.TrimSpace(processor)) {
	case "AMD64":
		return "x86_64", "x64"
	case "ARM", "ARM64":
		return "arm", "arm"
	case "IA64":
		return "ia64", "ia64"
	case "INTEL", "386":
		if level > 0 && level < 5 {
			return "i" + strconv.Itoa(level) + "86", "x86"
		}
		return "i686", "x86"
	default:
		return "unknown", "unknown"
	}
}

func windowsHardwareFromGoArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "386":
		return "i686"
	case "arm", "arm64":
		return "arm"
	default:
		return goarch
	}
}

func windowsArchitectureFromHardware(hardware string) string {
	if strings.HasPrefix(hardware, "i") && strings.HasSuffix(hardware, "86") {
		return "x86"
	}
	switch hardware {
	case "x86_64":
		return "x64"
	case "arm", "ia64", "unknown":
		return hardware
	default:
		return hardware
	}
}

var linuxI386ArchitectureRE = regexp.MustCompile(`^(?:i[3-6]86|pentium)$`)

func probeArchitectureName(s *Session) string {
	return currentArchitectureName(runtime.GOOS, s.cachedHardwareModel())
}

func currentArchitectureName(goos, machine string) string {
	return architectureName(goos, machine)
}

func currentProcessorISA(s *Session, goos, fallback string, run commandRunner) string {
	if goos == "windows" {
		if isa := s.cachedPlatformProcessorInfo().ISA; isa != "" {
			return isa
		}
		return ""
	}
	processor := strings.TrimSpace(run("uname", "-p"))
	if processor == "" || processor == "unknown" {
		return fallback
	}
	return processor
}

func probeKernelRelease(s *Session) string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func probeHardwareModel(s *Session) string {
	if runtime.GOOS == "windows" {
		return windowsHardwareFromGoArch(runtime.GOARCH)
	}
	out, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return runtime.GOARCH
	}
	return strings.TrimSpace(string(out))
}

func probeMacOSModel(s *Session) string {
	return currentMacOSModel(runtime.GOOS, s.commandOutput)
}

func currentMacOSModel(goos string, run commandRunner) string {
	if goos != "darwin" {
		return ""
	}
	return strings.TrimSpace(run("sysctl", "-n", "hw.model"))
}

func probeOSRelease(s *Session) any {
	if runtime.GOOS == "windows" {
		return currentWindowsOSRelease(s.cachedWindowsOSVersionInput())
	}
	return currentOSRelease(s, runtime.GOOS, os.ReadFile, s.commandOutput)
}

func probeWindowsOSVersionInput(s *Session) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	return windowsWMIOutput(s.commandOutput, "os", "OtherTypeDescription,ProductType,Version")
}

func currentOSRelease(s *Session, goos string, readFile fileReader, run commandRunner) any {
	switch goos {
	case "linux":
		data, err := readFile("/etc/os-release")
		if err != nil {
			return s.cachedKernelRelease()
		}
		id := linuxOSReleaseID(string(data))
		if release := specificLinuxOSRelease(id, readFile, run); len(release) > 0 {
			return release
		}
		return parseLinuxOSRelease(string(data))
	case "freebsd":
		versions := parseFreeBSDVersions(run("/bin/freebsd-version", "-k"), run("/bin/freebsd-version", "-ru"))
		if versions.InstalledUserland != "" {
			return parseFreeBSDOSRelease(versions.InstalledUserland)
		}
	case "openbsd":
		return parseOpenBSDOSRelease(run("uname", "-r"))
	case "darwin":
		return parseDarwinOSRelease(run("uname", "-r"))
	case "windows":
		return currentWindowsOSRelease(windowsWMIOutput(run, "os", "OtherTypeDescription,ProductType,Version"))
	}
	return s.cachedKernelRelease()
}

type windowsOSVersionInfo struct {
	Description string
	ProductType string
	Version     string
	MajorMinor  string
}

type windowsOSDescription struct {
	ConsumerRelease bool
	Description     string
}

func currentWindowsOSDescription(input string) *windowsOSDescription {
	info := parseWindowsOSVersionInfo(input)
	if info.ProductType == "" && info.Description == "" {
		return nil
	}
	return &windowsOSDescription{
		ConsumerRelease: info.ProductType == "1",
		Description:     info.Description,
	}
}

func currentWindowsKernelFacts(input string) []ResolvedFact {
	info := parseWindowsOSVersionInfo(input)
	if info.Version == "" {
		debug("Calling Windows RtlGetVersion failed")
		return nil
	}
	return []ResolvedFact{
		{Name: "kernel", Value: "windows"},
		{Name: "kernelmajversion", Value: info.MajorMinor},
		{Name: "kernelrelease", Value: info.Version},
		{Name: "kernelversion", Value: info.Version},
	}
}

func currentWindowsOSRelease(input string) map[string]any {
	info := parseWindowsOSVersionInfo(input)
	if info.Version == "" {
		return nil
	}
	release := windowsRelease(info.MajorMinor, info.ProductType == "1", info.Description, info.Version)
	if release == "" {
		return nil
	}
	return map[string]any{"full": release, "major": release}
}

func parseWindowsOSVersionInfo(input string) windowsOSVersionInfo {
	values := map[string]string{}
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		values[key] = strings.TrimSpace(value)
	}
	version := values["Version"]
	majorMinor := version
	if first := strings.IndexByte(majorMinor, '.'); first >= 0 {
		if second := strings.IndexByte(majorMinor[first+1:], '.'); second >= 0 {
			majorMinor = majorMinor[:first+1+second]
		}
	}
	return windowsOSVersionInfo{
		Description: values["OtherTypeDescription"],
		ProductType: values["ProductType"],
		Version:     version,
		MajorMinor:  majorMinor,
	}
}

func windowsRelease(version string, consumer bool, description, kernelVersion string) string {
	if version == "" {
		return ""
	}
	if strings.Contains(version, "10.0") {
		return windows10Release(consumer, kernelVersion)
	}
	if release := windows6Release(version, consumer); release != "" {
		return release
	}
	if strings.Contains(version, "5.2") {
		if consumer {
			return "XP"
		}
		if description == "R2" {
			return "2003 R2"
		}
		return "2003"
	}
	return version
}

func windows10Release(consumer bool, kernelVersion string) string {
	_, buildText, ok := strings.Cut(strings.TrimSpace(kernelVersion), ".")
	for ok {
		var next string
		buildText, next, ok = strings.Cut(buildText, ".")
		if !ok {
			break
		}
		buildText = next
	}
	build, _ := strconv.Atoi(buildText)
	if consumer {
		if build >= 22000 {
			return "11"
		}
		return "10"
	}
	if build >= 26100 {
		return "2025"
	}
	if build >= 20348 {
		return "2022"
	}
	if build >= 17623 {
		return "2019"
	}
	return "2016"
}

func windows6Release(version string, consumer bool) string {
	if consumer {
		switch version {
		case "6.3":
			return "8.1"
		case "6.2":
			return "8"
		case "6.1":
			return "7"
		case "6.0":
			return "Vista"
		}
		return ""
	}
	switch version {
	case "6.3":
		return "2012 R2"
	case "6.2":
		return "2012"
	case "6.1":
		return "2008 R2"
	case "6.0":
		return "2008"
	}
	return ""
}

func linuxOSReleaseID(input string) string {
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && key == "ID" {
			return parseOSReleaseValue(value)
		}
	}
	return ""
}

func specificLinuxOSRelease(id string, readFile fileReader, run commandRunner) map[string]any {
	switch strings.ToLower(id) {
	case "mageia":
		data, err := readFile("/etc/mageia-release")
		if err != nil {
			return nil
		}
		match := mageiaReleasePattern.FindStringSubmatch(strings.TrimSpace(string(data)))
		if len(match) < 2 {
			return nil
		}
		return releaseMap(match[1])
	case "openwrt":
		data, err := readFile("/etc/openwrt_version")
		if err != nil {
			return nil
		}
		match := openWrtReleasePattern.FindStringSubmatch(strings.TrimSpace(string(data)))
		if len(match) < 2 {
			return nil
		}
		return releaseMap(match[1])
	case "gentoo":
		data, err := readFile("/etc/gentoo-release")
		if err != nil {
			return nil
		}
		match := gentooReleasePattern.FindStringSubmatch(strings.TrimSpace(string(data)))
		if len(match) < 2 {
			return nil
		}
		return releaseMap(match[1])
	case "alpine":
		data, err := readFile("/etc/alpine-release")
		if err != nil {
			return nil
		}
		return releaseStringMap(strings.TrimSpace(string(data)))
	case "slackware":
		data, err := readFile("/etc/slackware-version")
		if err != nil {
			return nil
		}
		match := slackwareReleasePattern.FindStringSubmatch(strings.TrimSpace(string(data)))
		if len(match) < 2 {
			return nil
		}
		return releaseMap(match[1])
	case "amzn", "amazon":
		data, err := readFile("/etc/system-release")
		if err != nil {
			return nil
		}
		match := amazonReleasePattern.FindStringSubmatch(strings.TrimSpace(string(data)))
		if len(match) < 2 {
			return nil
		}
		if match[1] == "2023" {
			if version := amazonOSReleaseRPMVersion(run); version != "" {
				return releaseHashFromString(version, false)
			}
		}
		return releaseHashFromString(match[1], false)
	case "photon":
		data, err := readFile("/etc/lsb-release")
		if err != nil {
			return nil
		}
		match := photonReleasePattern.FindStringSubmatch(strings.TrimSpace(string(data)))
		if len(match) < 3 {
			return nil
		}
		return releaseHashFromString(match[1]+"."+match[2], false)
	case "mariner":
		data, err := readFile("/etc/mariner-release")
		if err != nil {
			return nil
		}
		match := marinerReleasePattern.FindStringSubmatch(strings.TrimSpace(string(data)))
		if len(match) < 2 {
			return nil
		}
		return releaseHashFromString(match[1], false)
	case "azurelinux":
		data, err := readFile("/etc/azurelinux-release")
		if err != nil {
			return nil
		}
		match := azureLinuxReleasePattern.FindStringSubmatch(strings.TrimSpace(string(data)))
		if len(match) < 2 {
			return nil
		}
		return releaseHashFromString(match[1], false)
	case "linuxmint":
		data, err := readFile("/etc/linuxmint/info")
		if err != nil {
			return nil
		}
		match := linuxMintReleasePattern.FindStringSubmatch(string(data))
		if len(match) < 2 {
			return nil
		}
		return releaseHashFromString(match[1], false)
	case "devuan":
		data, err := readFile("/etc/devuan_version")
		if err != nil {
			return nil
		}
		return releaseHashFromString(strings.TrimSpace(string(data)), false)
	case "meego":
		data, err := readFile("/etc/meego-release")
		if err != nil {
			return nil
		}
		return releaseHashFromString(strings.TrimSpace(string(data)), false)
	case "ovs":
		data, err := readFile("/etc/ovs-release")
		if err != nil {
			return nil
		}
		version := releaseFromFirstLine(string(data))
		if version == "" {
			return nil
		}
		return releaseHashFromString(version, false)
	case "eos":
		data, err := readFile("/etc/Eos-release")
		if err != nil {
			return nil
		}
		_, version, ok := strings.Cut(strings.TrimSpace(string(data)), " ")
		if !ok {
			return nil
		}
		return releaseHashFromString(strings.TrimSpace(version), false)
	case "oel", "ol":
		path := "/etc/enterprise-release"
		if strings.EqualFold(id, "ol") {
			path = "/etc/oracle-release"
		}
		data, err := readFile(path)
		if err != nil {
			return nil
		}
		match := firstLineReleasePattern.FindStringSubmatch(strings.TrimSpace(string(data)))
		if len(match) < 2 {
			return nil
		}
		return releaseHashFromString(match[1], false)
	case "debian":
		data, err := readFile("/etc/debian_version")
		if err != nil {
			return nil
		}
		return releaseMap(strings.TrimSpace(string(data)))
	}
	return nil
}

func releaseFromFirstLine(input string) string {
	line := strings.TrimSpace(strings.Split(input, "\n")[0])
	if strings.Contains(line, "Rawhide") {
		return "Rawhide"
	}
	if match := amazonReleasePattern.FindStringSubmatch(line); len(match) >= 2 {
		return match[1]
	}
	if match := firstLineReleasePattern.FindStringSubmatch(line); len(match) >= 2 {
		return match[1]
	}
	return ""
}

func amazonOSReleaseRPMVersion(run commandRunner) string {
	output := run("rpm", "-q", "--qf", "%{NAME}\n%{VERSION}\n%{RELEASE}\n%{VENDOR}", "-f", "/etc/os-release")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "system-release" {
		return ""
	}
	return strings.TrimSpace(lines[1])
}

func parseFreeBSDVersions(installedKernelOutput, runningAndUserlandOutput string) freeBSDVersions {
	versions := freeBSDVersions{InstalledKernel: strings.TrimSpace(installedKernelOutput)}
	lines := strings.Split(strings.TrimSpace(runningAndUserlandOutput), "\n")
	if len(lines) > 0 {
		versions.RunningKernel = strings.TrimSpace(lines[0])
	}
	if len(lines) > 1 {
		versions.InstalledUserland = strings.TrimSpace(lines[1])
	}
	return versions
}

func parseFreeBSDOSRelease(installedUserland string) map[string]any {
	installedUserland = strings.TrimSpace(installedUserland)
	version, branch, _ := strings.Cut(installedUserland, "-")
	major, minor, _ := strings.Cut(version, ".")
	release := map[string]any{
		"full":   installedUserland,
		"major":  major,
		"branch": branch,
	}
	if minor != "" {
		release["minor"] = minor
	}
	if _, patchlevel, ok := strings.Cut(branch, "-p"); ok {
		release["patchlevel"] = patchlevel
	}
	return release
}

func parseOpenBSDOSRelease(output string) map[string]any {
	full := strings.TrimSpace(output)
	if full == "" {
		return nil
	}
	major, minor, _ := strings.Cut(full, ".")
	release := map[string]any{"full": full, "major": major}
	if minor != "" {
		release["minor"] = minor
	}
	return release
}

func parseDarwinOSRelease(output string) map[string]any {
	full := strings.TrimSpace(output)
	if full == "" {
		return nil
	}
	major, rest, _ := strings.Cut(full, ".")
	minor, _, _ := strings.Cut(rest, ".")
	release := map[string]any{"full": full, "major": major}
	if minor != "" {
		release["minor"] = minor
	}
	return release
}

func parseLinuxOSRelease(input string) map[string]any {
	id := ""
	versionID := ""
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = parseOSReleaseValue(value)
		switch key {
		case "ID":
			id = value
		case "VERSION_ID":
			versionID = value
		}
	}
	if versionID == "" {
		return nil
	}

	release := linuxOSReleaseMap(id, versionID)
	return release
}

func linuxOSReleaseMap(id, full string) map[string]any {
	if full == "" {
		return nil
	}
	if strings.EqualFold(id, "debian") {
		return debianReleaseMap(full)
	}
	switch strings.ToLower(id) {
	case "mariner", "azurelinux", "linuxmint", "gentoo", "mageia":
		return releaseHashFromString(full, false)
	}
	return map[string]any{"full": full, "major": full}
}

func parseOSReleaseValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	if value[0] != '"' || value[len(value)-1] != '"' {
		return strings.Trim(value, "'")
	}
	unquoted, err := strconv.Unquote(value)
	if err != nil {
		return strings.Trim(value, "\"")
	}
	return unquoted
}

type macOSInfo struct {
	ProductName         string
	ProductVersion      string
	ProductVersionExtra string
	BuildVersion        string
}

func probeMacOSInfo(s *Session) macOSInfo {
	return currentMacOSInfo(runtime.GOOS, s.commandOutput)
}

func currentMacOSInfo(goos string, run commandRunner) macOSInfo {
	if goos != "darwin" {
		return macOSInfo{}
	}
	return parseSwVers(run("sw_vers"))
}

func parseSwVers(input string) macOSInfo {
	info := macOSInfo{}
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "ProductName":
			info.ProductName = value
		case "ProductVersion":
			info.ProductVersion = value
		case "ProductVersionExtra":
			info.ProductVersionExtra = value
		case "BuildVersion":
			info.BuildVersion = value
		}
	}
	return info
}

type macOSSystemProfilerHardware struct {
	ModelName          string
	ModelIdentifier    string
	ProcessorName      string
	ProcessorSpeed     string
	NumberOfProcessors string
	TotalCores         string
	L2CachePerCore     string
	L3Cache            string
	Memory             string
	BootROMVersion     string
	SMCVersion         string
	SerialNumber       string
	HardwareUUID       string
	SubsystemVendorID  string
}

type macOSSystemProfilerSoftware struct {
	SystemVersion       string
	KernelVersion       string
	BootVolume          string
	BootMode            string
	ComputerName        string
	UserName            string
	SecureVirtualMemory string
	TimeSinceBoot       string
}

type macOSSystemProfilerEthernet struct {
	Type              string
	Bus               string
	VendorID          string
	DeviceID          string
	SubsystemVendorID string
	SubsystemID       string
	RevisionID        string
	BSDName           string
	KextName          string
	Location          string
	Version           string
}

func probeMacOSSystemProfilerHardware(s *Session) macOSSystemProfilerHardware {
	if runtime.GOOS != "darwin" {
		return macOSSystemProfilerHardware{}
	}
	out, err := exec.Command("system_profiler", "SPHardwareDataType").Output()
	if err != nil {
		return macOSSystemProfilerHardware{}
	}
	return parseMacOSSystemProfilerHardware(string(out))
}

func probeMacOSSystemProfilerSoftware(s *Session) macOSSystemProfilerSoftware {
	if runtime.GOOS != "darwin" {
		return macOSSystemProfilerSoftware{}
	}
	out, err := exec.Command("system_profiler", "SPSoftwareDataType").Output()
	if err != nil {
		return macOSSystemProfilerSoftware{}
	}
	return parseMacOSSystemProfilerSoftware(string(out))
}

func probeMacOSSystemProfilerEthernet(s *Session) macOSSystemProfilerEthernet {
	return currentMacOSSystemProfilerEthernet(runtime.GOOS, s.commandOutput)
}

func currentMacOSSystemProfilerEthernet(goos string, run commandRunner) macOSSystemProfilerEthernet {
	if goos != "darwin" || run == nil {
		return macOSSystemProfilerEthernet{}
	}
	return parseMacOSSystemProfilerEthernet(run("system_profiler", "SPEthernetDataType"))
}

func parseMacOSSystemProfilerHardware(input string) macOSSystemProfilerHardware {
	hardware := macOSSystemProfilerHardware{}
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := cutMacOSSystemProfilerLine(line)
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Model Name":
			hardware.ModelName = value
		case "Model Identifier":
			hardware.ModelIdentifier = value
		case "Processor Name":
			hardware.ProcessorName = value
		case "Processor Speed":
			hardware.ProcessorSpeed = value
		case "Number of Processors":
			hardware.NumberOfProcessors = value
		case "Total Number of Cores":
			hardware.TotalCores = value
		case "L2 Cache (per Core)":
			hardware.L2CachePerCore = value
		case "L3 Cache":
			hardware.L3Cache = value
		case "Memory":
			hardware.Memory = value
		case "System Firmware Version", "Boot ROM Version":
			hardware.BootROMVersion = value
		case "SMC Version (system)":
			hardware.SMCVersion = value
		case "Serial Number (system)":
			hardware.SerialNumber = value
		case "Hardware UUID":
			hardware.HardwareUUID = value
		case "Subsystem Vendor ID":
			hardware.SubsystemVendorID = value
		}
	}
	return hardware
}

func parseMacOSSystemProfilerSoftware(input string) macOSSystemProfilerSoftware {
	software := macOSSystemProfilerSoftware{}
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := cutMacOSSystemProfilerLine(line)
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "System Version":
			software.SystemVersion = value
		case "Kernel Version":
			software.KernelVersion = value
		case "Boot Volume":
			software.BootVolume = value
		case "Boot Mode":
			software.BootMode = value
		case "Computer Name":
			software.ComputerName = value
		case "User Name":
			software.UserName = value
		case "Secure Virtual Memory":
			software.SecureVirtualMemory = value
		case "Time since boot":
			software.TimeSinceBoot = value
		}
	}
	return software
}

func parseMacOSSystemProfilerEthernet(input string) macOSSystemProfilerEthernet {
	ethernet := macOSSystemProfilerEthernet{}
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := cutMacOSSystemProfilerLine(line)
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Type":
			ethernet.Type = value
		case "Bus":
			ethernet.Bus = value
		case "Vendor ID":
			ethernet.VendorID = value
		case "Device ID":
			ethernet.DeviceID = value
		case "Subsystem Vendor ID":
			ethernet.SubsystemVendorID = value
		case "Subsystem ID":
			ethernet.SubsystemID = value
		case "Revision ID":
			ethernet.RevisionID = value
		case "BSD name":
			ethernet.BSDName = value
		case "Kext name":
			ethernet.KextName = value
		case "Location":
			ethernet.Location = value
		case "Version":
			ethernet.Version = value
		}
	}
	return ethernet
}

func cutMacOSSystemProfilerLine(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, ": ")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), true
}

func macOSSystemProfilerFacts(hardware macOSSystemProfilerHardware) []ResolvedFact {
	fields := []struct {
		core  string
		value string
	}{
		{core: "system_profiler.model_name", value: hardware.ModelName},
		{core: "system_profiler.model_identifier", value: hardware.ModelIdentifier},
		{core: "system_profiler.processor_name", value: hardware.ProcessorName},
		{core: "system_profiler.processor_speed", value: hardware.ProcessorSpeed},
		{core: "system_profiler.processors", value: hardware.NumberOfProcessors},
		{core: "system_profiler.cores", value: hardware.TotalCores},
		{core: "system_profiler.l2_cache_per_core", value: hardware.L2CachePerCore},
		{core: "system_profiler.l3_cache", value: hardware.L3Cache},
		{core: "system_profiler.memory", value: hardware.Memory},
		{core: "system_profiler.boot_rom_version", value: hardware.BootROMVersion},
		{core: "system_profiler.smc_version", value: hardware.SMCVersion},
		{core: "system_profiler.serial_number", value: hardware.SerialNumber},
		{core: "system_profiler.hardware_uuid", value: hardware.HardwareUUID},
		{core: "system_profiler.subsystem_vendor_id", value: hardware.SubsystemVendorID},
	}
	core := make([]ResolvedFact, 0, len(fields))
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		core = append(core, ResolvedFact{Name: field.core, Value: field.value})
	}
	return core
}

func macOSSystemProfilerSoftwareFacts(software macOSSystemProfilerSoftware) []ResolvedFact {
	fields := []struct {
		core  string
		value string
	}{
		{core: "system_profiler.system_version", value: software.SystemVersion},
		{core: "system_profiler.kernel_version", value: software.KernelVersion},
		{core: "system_profiler.boot_volume", value: software.BootVolume},
		{core: "system_profiler.boot_mode", value: software.BootMode},
		{core: "system_profiler.computer_name", value: software.ComputerName},
		{core: "system_profiler.username", value: software.UserName},
		{core: "system_profiler.secure_virtual_memory", value: software.SecureVirtualMemory},
		{core: "system_profiler.uptime", value: software.TimeSinceBoot},
	}
	core := make([]ResolvedFact, 0, len(fields))
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		core = append(core, ResolvedFact{Name: field.core, Value: field.value})
	}
	return core
}

func macOSSystemProfilerEthernetFacts(ethernet macOSSystemProfilerEthernet) []ResolvedFact {
	fields := []struct {
		core  string
		value string
	}{
		{core: "system_profiler.type", value: ethernet.Type},
		{core: "system_profiler.bus", value: ethernet.Bus},
		{core: "system_profiler.vendor_id", value: ethernet.VendorID},
		{core: "system_profiler.device_id", value: ethernet.DeviceID},
		{core: "system_profiler.subsystem_vendor_id", value: ethernet.SubsystemVendorID},
		{core: "system_profiler.subsystem_id", value: ethernet.SubsystemID},
		{core: "system_profiler.revision_id", value: ethernet.RevisionID},
		{core: "system_profiler.bsd_name", value: ethernet.BSDName},
		{core: "system_profiler.kext_name", value: ethernet.KextName},
		{core: "system_profiler.location", value: ethernet.Location},
		{core: "system_profiler.version", value: ethernet.Version},
	}
	core := make([]ResolvedFact, 0, len(fields))
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		core = append(core, ResolvedFact{Name: field.core, Value: field.value})
	}
	return core
}

func macOSVersionFacts(productVersion, extra string) []ResolvedFact {
	if productVersion == "" {
		return nil
	}
	version := macOSVersion(productVersion, extra)
	return []ResolvedFact{{Name: "os.macosx.version", Value: version}}
}

func macOSStringFact(coreName, value string) []ResolvedFact {
	if value == "" {
		return nil
	}
	return []ResolvedFact{{Name: coreName, Value: value}}
}

func currentWindowsSystem32(goos, systemRoot string, isWOW64 func() (bool, bool)) string {
	if goos != "windows" || systemRoot == "" {
		return ""
	}
	wow64, ok := isWOW64()
	if !ok {
		return ""
	}
	if wow64 {
		return systemRoot + `\sysnative`
	}
	return systemRoot + `\system32`
}

func currentWindowsProcessWOW64() (bool, bool) {
	return os.Getenv("PROCESSOR_ARCHITEW6432") != "", true
}

func windowsSystem32Facts(path string) []ResolvedFact {
	if path == "" {
		return nil
	}
	return []ResolvedFact{{Name: "os.windows.system32", Value: path}}
}

func currentTimezone(s *Session, goos string) string {
	zone := time.Now().Format("MST")
	if goos != "windows" {
		return zone
	}
	if windowsZone := currentWindowsTimezone(goos, zone, currentWindowsAPICodepage(s), func() string { return currentWindowsRegistryCodepage(s) }); windowsZone != "" {
		return windowsZone
	}
	return zone
}

func currentWindowsTimezone(goos, zone, apiCodepage string, registryCodepage func() string) string {
	if goos != "windows" || zone == "" {
		return ""
	}
	codepage := apiCodepage
	if codepage == "" {
		codepage = registryCodepage()
	}
	decoded, ok := decodeWindowsCodepage(zone, codepage)
	if !ok {
		return zone
	}
	return decoded
}

func currentWindowsAPICodepage(s *Session) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	return firstNumber(s.commandOutput("cmd", "/c", "chcp"))
}

func currentWindowsRegistryCodepage(s *Session) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	return parseWindowsACPRegistry(s.commandOutput("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Control\Nls\CodePage`, "/v", "ACP"))
}

func parseWindowsACPRegistry(input string) string {
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && fields[0] == "ACP" {
			return fields[len(fields)-1]
		}
	}
	return ""
}

func firstNumber(input string) string {
	for field := range strings.FieldsSeq(input) {
		field = strings.TrimRight(field, ":.")
		if _, err := strconv.Atoi(field); err == nil {
			return field
		}
	}
	return ""
}

func decodeWindowsCodepage(value, codepage string) (string, bool) {
	decoder := windowsCodepageDecoder(codepage)
	if decoder == nil {
		return "", false
	}
	reader := transform.NewReader(bytes.NewReader([]byte(value)), decoder.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}

func windowsCodepageDecoder(codepage string) encoding.Encoding {
	switch strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(codepage), "CP")) {
	case "437":
		return charmap.CodePage437
	case "850":
		return charmap.CodePage850
	case "1252":
		return charmap.Windows1252
	default:
		return nil
	}
}

type windowsProductRelease struct {
	EditionID        string
	InstallationType string
	ProductName      string
	ReleaseID        string
	DisplayVersion   string
}

type windowsMemory struct {
	TotalBytes     int
	AvailableBytes int
	UsedBytes      int
	Capacity       string
}

func currentWindowsMemory(goos string, run commandRunner) windowsMemory {
	if goos != "windows" {
		return windowsMemory{}
	}
	return parseWindowsMemory(windowsWMIOutput(run, "os", "FreePhysicalMemory,TotalVisibleMemorySize"))
}

func parseWindowsMemory(input string) windowsMemory {
	if strings.TrimSpace(input) == "" {
		debug("Resolving memory facts failed")
		return windowsMemory{}
	}
	values := parseWindowsWMIValues(input)
	totalKB, err := strconv.Atoi(values["TotalVisibleMemorySize"])
	if err != nil || totalKB <= 0 {
		debug("Available or Total bytes are zero could not proceed further")
		return windowsMemory{}
	}
	availableKB, err := strconv.Atoi(values["FreePhysicalMemory"])
	if err != nil || availableKB <= 0 {
		debug("Available or Total bytes are zero could not proceed further")
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

func currentWindowsProcessors(goos string, run commandRunner) processorInfo {
	if goos != "windows" {
		return processorInfo{}
	}
	return parseWindowsProcessors(windowsWMIOutput(run, "cpu", "Name,Architecture,NumberOfLogicalProcessors,NumberOfCores"))
}

func parseWindowsProcessors(input string) processorInfo {
	records := parseWindowsWMIRecords(input)
	if len(records) == 0 {
		debug("WMI query returned no resultsfor Win32_Processor with values Name, Architecture and NumberOfLogicalProcessors.")
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
			info.ISA = windowsProcessorISA(record["Architecture"])
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

func windowsProcessorISA(architecture string) string {
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
		debug("Unable to determine processor type: unknown architecture")
		return ""
	}
}

func currentWindowsProductRelease(goos string, run commandRunner) windowsProductRelease {
	if goos != "windows" {
		return windowsProductRelease{}
	}
	return parseWindowsProductRelease(run("reg", "query", `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`))
}

func parseWindowsProductRelease(input string) windowsProductRelease {
	values := map[string]string{}
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 || fields[1] != "REG_SZ" {
			continue
		}
		values[fields[0]] = strings.Join(fields[2:], " ")
	}

	releaseID := values["ReleaseId"]
	if values["DisplayVersion"] != "" {
		releaseID = values["DisplayVersion"]
	}
	return windowsProductRelease{
		EditionID:        values["EditionID"],
		InstallationType: values["InstallationType"],
		ProductName:      values["ProductName"],
		ReleaseID:        releaseID,
		DisplayVersion:   values["DisplayVersion"],
	}
}

func windowsProductReleaseFacts(release windowsProductRelease) []ResolvedFact {
	fields := []struct {
		core  string
		value string
	}{
		{core: "os.windows.edition_id", value: release.EditionID},
		{core: "os.windows.installation_type", value: release.InstallationType},
		{core: "os.windows.product_name", value: release.ProductName},
		{core: "os.windows.release_id", value: release.ReleaseID},
		{core: "os.windows.display_version", value: release.DisplayVersion},
	}
	core := make([]ResolvedFact, 0, len(fields))
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		core = append(core, ResolvedFact{Name: field.core, Value: field.value})
	}
	return core
}

type windowsDMI struct {
	Manufacturer string
	SerialNumber string
	ProductName  string
	ProductUUID  string
}

func currentWindowsDMI(goos string, run commandRunner) windowsDMI {
	if goos != "windows" {
		return windowsDMI{}
	}
	bios := parseWindowsWMIValues(windowsWMIOutput(run, "bios", "Manufacturer,SerialNumber"))
	product := parseWindowsWMIValues(windowsWMIOutput(run, "computersystemproduct", "Name,UUID"))
	if len(bios) == 0 {
		debug("WMI query returned no results for Win32_BIOS with values Manufacturer and SerialNumber.")
	}
	if len(product) == 0 {
		debug("WMI query returned no results for Win32_ComputerSystemProduct with values Name and UUID.")
	}
	return windowsDMI{
		Manufacturer: bios["Manufacturer"],
		SerialNumber: bios["SerialNumber"],
		ProductName:  product["Name"],
		ProductUUID:  product["UUID"],
	}
}

func parseWindowsWMIValues(input string) map[string]string {
	values := map[string]string{}
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		values[key] = strings.TrimSpace(value)
	}
	return values
}

func parseWindowsWMIRecords(input string) []map[string]string {
	records := make([]map[string]string, 0)
	current := map[string]string{}
	for line := range strings.SplitSeq(strings.ReplaceAll(input, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(current) > 0 {
				records = append(records, current)
				current = map[string]string{}
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "Name" && len(current) > 0 && current["Name"] != "" {
			records = append(records, current)
			current = map[string]string{}
		}
		current[key] = strings.TrimSpace(value)
	}
	if len(current) > 0 {
		records = append(records, current)
	}
	return records
}

func windowsDMIFacts(dmi windowsDMI) []ResolvedFact {
	fields := []struct {
		core  string
		value string
	}{
		{core: "dmi.manufacturer", value: dmi.Manufacturer},
		{core: "dmi.product.name", value: dmi.ProductName},
		{core: "dmi.product.serial_number", value: dmi.SerialNumber},
		{core: "dmi.product.uuid", value: dmi.ProductUUID},
	}
	core := make([]ResolvedFact, 0, len(fields))
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		core = append(core, ResolvedFact{Name: field.core, Value: field.value})
	}
	return core
}

func macOSDMIFacts(model string) []ResolvedFact {
	if model == "" {
		return nil
	}
	return []ResolvedFact{{Name: "dmi.product.name", Value: model}}
}

func macOSVersion(productVersion, extra string) map[string]any {
	parts := strings.Split(productVersion, ".")
	version := map[string]any{"full": productVersion}
	if len(parts) > 0 && parts[0] == "10" {
		major := parts[0]
		if len(parts) > 1 {
			major += "." + parts[1]
		}
		version["major"] = major
		version["minor"] = parts[len(parts)-1]
		return version
	}
	version["major"] = partOrDefault(parts, 0, "")
	version["minor"] = partOrDefault(parts, 1, "0")
	version["patch"] = partOrDefault(parts, 2, "0")
	if extra != "" && parts[0] != "11" && parts[0] != "12" {
		version["extra"] = extra
	}
	return version
}

func partOrDefault(parts []string, index int, fallback string) string {
	if index >= len(parts) || parts[index] == "" {
		return fallback
	}
	return parts[index]
}

type linuxDistro struct {
	Name           string
	ID             string
	Description    string
	Codename       string
	Specification  string
	Release        map[string]any
	ReleaseKnown   bool
	LSBID          string
	LSBDescription string
	LSBCodename    string
}

func probeLinuxDistro(s *Session) linuxDistro {
	return currentLinuxDistro(runtime.GOOS, exec.LookPath, s.commandOutput, os.ReadFile)
}

func currentLinuxDistro(goos string, lookPath func(string) (string, error), run commandRunner, readFile fileReader) linuxDistro {
	if goos != "linux" {
		return linuxDistro{}
	}
	lsbDistro := linuxDistro{}
	if _, err := lookPath("lsb_release"); err == nil {
		out := run("lsb_release", "-a")
		if out != "" {
			lsbDistro = parseLSBRelease(out)
		}
	}
	data, err := readFile("/etc/os-release")
	if err != nil {
		if linuxDistroHasData(lsbDistro) {
			return lsbDistro
		}
		return currentSuseRelease(readFile)
	}
	distro := parseLinuxDistroOSRelease(string(data))
	if usesRedHatReleaseDistro(distro.ID) {
		if redHat := currentRedHatRelease(readFile); redHat.ID != "" || redHat.Description != "" || redHat.Codename != "" || len(redHat.Release) > 0 {
			distro = mergeRedHatDistro(distro, redHat)
		}
		if linuxDistroHasData(lsbDistro) {
			distro = mergeRHELLSBLegacyDistro(distro, lsbDistro)
		}
	} else if linuxDistroHasData(lsbDistro) {
		return lsbDistro
	}
	if strings.EqualFold(distro.ID, "amzn") && distro.Release["full"] == "2023" {
		if version := amazonOSReleaseRPMVersion(run); version != "" {
			distro.Release = releaseHashFromString(version, true)
		}
	}
	if strings.EqualFold(distro.ID, "amzn") {
		if systemRelease := currentAmazonSystemRelease(readFile); systemRelease.Description != "" {
			distro.ID = systemRelease.ID
			distro.Description = systemRelease.Description
			distro.Codename = systemRelease.Codename
		}
	}
	return distro
}

func linuxDistroHasData(distro linuxDistro) bool {
	return distro.ID != "" || distro.Description != "" || distro.Codename != "" || distro.Specification != "" || len(distro.Release) > 0 || distro.ReleaseKnown
}

func currentSuseRelease(readFile fileReader) linuxDistro {
	data, err := readFile("/etc/SuSE-release")
	if err != nil {
		return linuxDistro{}
	}
	return parseSuseRelease(string(data))
}

func parseSuseRelease(input string) linuxDistro {
	distro := linuxDistro{}
	for line := range strings.SplitSeq(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			if strings.EqualFold(strings.TrimSpace(key), "VERSION") {
				distro.Release = releaseMap(strings.TrimSpace(value))
				distro.ReleaseKnown = len(distro.Release) > 0
			}
			continue
		}
		if distro.Name == "" {
			if fields := strings.Fields(line); len(fields) > 0 {
				distro.Name = fields[0]
				distro.ID = strings.ToLower(fields[0])
			}
		}
	}
	return distro
}

func currentAmazonSystemRelease(readFile fileReader) linuxDistro {
	data, err := readFile("/etc/system-release")
	if err != nil {
		return linuxDistro{}
	}
	return parseAmazonSystemRelease(string(data))
}

func parseAmazonSystemRelease(input string) linuxDistro {
	description := strings.TrimSpace(input)
	if description == "" {
		return linuxDistro{}
	}
	id := "Amazon"
	if strings.Contains(description, "Amazon Linux AMI") {
		id = "AmazonAMI"
	}
	codename := redHatReleaseCodename(description)
	if codename == "" {
		codename = "n/a"
	}
	return linuxDistro{ID: id, Description: description, Codename: codename}
}

func currentRedHatRelease(readFile fileReader) linuxDistro {
	data, err := readFile("/etc/redhat-release")
	if err != nil {
		return linuxDistro{}
	}
	return parseRedHatRelease(string(data))
}

func mergeRedHatDistro(distro, redHat linuxDistro) linuxDistro {
	if distro.Name == "" {
		distro.Name = redHat.Name
	}
	if redHat.ID != "" {
		distro.ID = redHat.ID
	}
	if redHat.Description != "" {
		distro.Description = redHat.Description
	}
	if redHat.Codename != "" {
		distro.Codename = redHat.Codename
	}
	if len(redHat.Release) > 0 {
		distro.Release = redHat.Release
		distro.ReleaseKnown = redHat.ReleaseKnown
	}
	return distro
}

func mergeRHELLSBLegacyDistro(distro, lsb linuxDistro) linuxDistro {
	distro.LSBID = lsb.ID
	distro.LSBDescription = lsb.Description
	distro.LSBCodename = lsb.Codename
	return distro
}

func parseLSBRelease(input string) linuxDistro {
	values := make(map[string]string)
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(strings.ReplaceAll(line, "\t", ""), ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), " ", "_"))
		values[key] = strings.TrimSpace(value)
	}

	return linuxDistro{
		Name:          values["distributor_id"],
		ID:            values["distributor_id"],
		Description:   values["description"],
		Codename:      values["codename"],
		Specification: values["lsb_version"],
		Release:       linuxDistroReleaseMap(values["distributor_id"], values["release"]),
		ReleaseKnown:  strings.TrimSpace(values["release"]) != "",
	}
}

func parseLinuxDistroOSRelease(input string) linuxDistro {
	distro := linuxDistro{}
	release := ""
	releaseKnown := false
	version := ""
	for len(input) > 0 {
		line := input
		if i := strings.IndexByte(input, '\n'); i >= 0 {
			line = input[:i]
			input = input[i+1:]
		} else {
			input = ""
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = parseOSReleaseValue(value)
		switch key {
		case "NAME":
			distro.Name = linuxDistroName(value)
		case "ID":
			distro.ID = linuxDistroID(value)
		case "PRETTY_NAME":
			distro.Description = value
		case "VERSION":
			version = value
		case "VERSION_CODENAME":
			distro.Codename = value
		case "UBUNTU_CODENAME":
			if distro.Codename == "" {
				distro.Codename = value
			}
		case "VERSION_ID":
			release = value
			releaseKnown = true
		}
	}

	if distro.Codename == "" {
		distro.Codename = linuxDistroCodenameFromVersion(version)
	}
	isSLES := strings.EqualFold(distro.ID, "sles")
	if isSLES && distro.Codename == "" {
		distro.Codename = "n/a"
	}
	distro.ReleaseKnown = releaseKnown
	if strings.EqualFold(distro.ID, "devuan") {
		distro.ReleaseKnown = true
	} else {
		distro.Release = linuxDistroReleaseMap(distro.ID, release)
	}
	distro.ID = slesDistroID(distro.ID, release)
	return distro
}

func linuxDistroCodenameFromVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if strings.HasSuffix(version, ")") {
		if start := strings.LastIndex(version, "("); start >= 0 {
			return strings.TrimSpace(version[start+1 : len(version)-1])
		}
	}
	words := linuxDistroVersionWordsRE.FindString(version)
	if words == "" {
		return ""
	}
	return strings.ToLower(strings.Fields(words)[0])
}

func linuxDistroName(name string) string {
	if strings.EqualFold(name, "Ubuntu Linux") {
		return "Ubuntu"
	}
	if startsWithFold(name, "sles") {
		return "SLES"
	}
	if startsWithFold(name, "oracle") {
		words := strings.Fields(name)
		if len(words) > 2 {
			words = words[:2]
		}
		return strings.Join(words, "")
	}
	if endsWithFold(name, "azure linux") {
		words := strings.Fields(name)
		if len(words) >= 3 {
			return strings.Join(words[1:3], "")
		}
	}
	if endsWithFold(name, "mariner") {
		words := strings.Fields(name)
		if len(words) > 0 {
			return words[len(words)-1]
		}
	}
	if startsWithFold(name, "arch") || startsWithFold(name, "manjaro") {
		words := strings.Fields(name)
		if len(words) > 2 {
			words = words[:2]
		}
		return capitalizeASCII(strings.Join(words, ""))
	}
	if startsWithFold(name, "virtuozzo") {
		words := strings.Fields(name)
		if len(words) == 0 {
			return name
		}
		return words[0] + "Linux"
	}
	words := strings.Fields(name)
	if len(words) == 0 {
		return name
	}
	return words[0]
}

func startsWithFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func endsWithFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

func capitalizeASCII(s string) string {
	if s == "" {
		return ""
	}
	b := []byte(strings.ToLower(s))
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}

func linuxDistroID(id string) string {
	if strings.EqualFold(id, "sles_sap") {
		return "sles"
	}
	if strings.EqualFold(id, "opensuse-leap") {
		return "opensuse"
	}
	return id
}

func slesDistroID(id, release string) string {
	if !strings.EqualFold(id, "sles") {
		return id
	}
	if majorVersion(release) == "12" {
		return "SUSE LINUX"
	}
	return "SUSE"
}

func linuxDistroReleaseMap(id, full string) map[string]any {
	if strings.EqualFold(id, "ubuntu") {
		return ubuntuReleaseMap(full)
	}
	if strings.EqualFold(id, "sles") {
		return slesReleaseMap(full)
	}
	if strings.EqualFold(id, "debian") {
		return debianReleaseMap(full)
	}
	return releaseMap(full)
}

func debianReleaseMap(full string) map[string]any {
	if !strings.Contains(full, ".") {
		full += ".0"
	}
	release := releaseMap(full)
	if release == nil {
		return nil
	}
	minor, ok := release["minor"].(string)
	if ok && len(minor) >= 2 && minor[0] == '0' && minor[1] >= '1' && minor[1] <= '9' {
		release["minor"] = minor[1:]
	}
	return release
}

func ubuntuReleaseMap(full string) map[string]any {
	if full == "" {
		return nil
	}
	return map[string]any{"full": full, "major": full}
}

func slesReleaseMap(full string) map[string]any {
	release := releaseMap(full)
	if release != nil {
		if _, ok := release["minor"]; !ok {
			release["minor"] = nil
		}
	}
	return release
}

func parseRedHatRelease(input string) linuxDistro {
	description := strings.TrimSpace(input)
	if description == "" {
		return linuxDistro{}
	}

	parts := strings.Split(description, "release")
	if len(parts) < 2 {
		return linuxDistro{Description: description}
	}
	prefix := strings.TrimSpace(parts[0])
	version := ""
	if fields := strings.Fields(strings.TrimSpace(parts[1])); len(fields) > 0 {
		version = fields[0]
	}

	return linuxDistro{
		Name:         redHatReleaseName(prefix),
		ID:           redHatDistributorID(prefix),
		Description:  description,
		Codename:     redHatReleaseCodename(description),
		Release:      releaseHashFromString(version, false),
		ReleaseKnown: version != "",
	}
}

func redHatReleaseName(prefix string) string {
	words := redHatReleaseWords(prefix)
	if len(words) > 2 {
		words = words[:2]
	}
	return strings.Join(words, "")
}

func redHatDistributorID(prefix string) string {
	return strings.Join(redHatReleaseWords(prefix), "")
}

func redHatReleaseWords(prefix string) []string {
	words := strings.Fields(prefix)
	kept := make([]string, 0, len(words))
	for _, word := range words {
		if strings.EqualFold(word, "linux") {
			continue
		}
		kept = append(kept, word)
	}
	return kept
}

func redHatReleaseCodename(description string) string {
	start := strings.IndexByte(description, '(')
	end := strings.LastIndexByte(description, ')')
	if start < 0 || end <= start+1 {
		return ""
	}
	return description[start+1 : end]
}

func releaseMap(full string) map[string]any {
	if full == "" {
		return nil
	}
	release := map[string]any{"full": full, "major": majorVersion(full)}
	if _, minor, ok := strings.Cut(full, "."); ok {
		release["minor"] = minor
	}
	return release
}

func releaseStringMap(full string) map[string]any {
	if full == "" {
		return nil
	}
	parts := strings.Split(full, ".")
	release := map[string]any{"full": full, "major": parts[0]}
	if len(parts) > 1 {
		release["minor"] = parts[1]
	}
	return release
}

func linuxDistroFacts(distro linuxDistro) []ResolvedFact {
	if distro.ID == "" && distro.Description == "" && distro.Codename == "" && distro.Specification == "" && len(distro.Release) == 0 && !distro.ReleaseKnown {
		return nil
	}
	core := make([]ResolvedFact, 0, 5)
	if distro.ID != "" {
		core = append(core, ResolvedFact{Name: "os.distro.id", Value: distro.ID})
	}
	if distro.Description != "" {
		core = append(core, ResolvedFact{Name: "os.distro.description", Value: distro.Description})
	}
	if distro.Codename != "" {
		core = append(core, ResolvedFact{Name: "os.distro.codename", Value: distro.Codename})
	}
	if distro.Specification != "" {
		core = append(core, ResolvedFact{Name: "os.distro.specification", Value: distro.Specification})
	}
	if len(distro.Release) > 0 {
		core = append(core, ResolvedFact{Name: "os.distro.release", Value: distro.Release})
	} else if distro.ReleaseKnown {
		core = append(core, ResolvedFact{Name: "os.distro.release", Value: nil})
	}
	return core
}

func probeTotalPhysicalMemoryBytes(s *Session) int {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		value, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 0)
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
		out, err := exec.Command("vm_stat").Output()
		if err != nil {
			return 0
		}
		return parseDarwinVMStatAvailableBytes(string(out))
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
	return currentWindowsMemory(runtime.GOOS, s.commandOutput)
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
	data, err := os.ReadFile("/proc/meminfo")
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
	switch runtime.GOOS {
	case "darwin":
		if speed := s.cachedPlatformProcessorInfo().SpeedHz; speed > 0 {
			return hertzToHumanReadable(int64(speed))
		}
	case "linux":
		data, err := os.ReadFile("/proc/cpuinfo")
		if err != nil {
			return ""
		}
		return parseLinuxProcessorSpeed(string(data))
	case "freebsd":
		return hertzToHumanReadable(s.cachedPlatformProcessorInfo().SpeedHz)
	}
	return ""
}

func probeProcessorModels(s *Session) []string {
	architecture := architectureName(runtime.GOOS, s.cachedHardwareModel())
	switch runtime.GOOS {
	case "darwin":
		models := s.cachedPlatformProcessorInfo().Models
		if len(models) > 0 {
			return append([]string(nil), models...)
		}
	case "linux":
		data, err := os.ReadFile("/proc/cpuinfo")
		if err == nil {
			models := parseLinuxProcessorModels(string(data))
			if len(models) > 0 {
				return models
			}
		}
	case "freebsd", "windows":
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
	switch runtime.GOOS {
	case "darwin":
		processors := s.cachedPlatformProcessorInfo()
		if processors.CoresPerSocket > 0 && processors.ThreadsPerCore > 0 {
			return processors.CoresPerSocket, processors.ThreadsPerCore
		}
	case "linux":
		data, err := os.ReadFile("/proc/cpuinfo")
		if err == nil {
			cores, threads := parseLinuxProcessorTopology(string(data))
			if cores > 0 && threads > 0 {
				return cores, threads
			}
		}
	case "freebsd", "windows":
		processors := s.cachedPlatformProcessorInfo()
		if processors.CoresPerSocket > 0 && processors.ThreadsPerCore > 0 {
			return processors.CoresPerSocket, processors.ThreadsPerCore
		}
	}
	return logical, 1
}

func probePlatformProcessorInfo(s *Session) processorInfo {
	return currentProcessorInfo(runtime.GOOS, s.commandOutput)
}

func currentProcessorInfo(goos string, run func(string, ...string) string) processorInfo {
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
	case "windows":
		return currentWindowsProcessors(goos, run)
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

func probeProcessorExtensions(s *Session) []string {
	architecture := architectureName(runtime.GOOS, s.cachedHardwareModel())
	if runtime.GOOS != "linux" {
		return sortedProcessorExtensions(map[string]bool{architecture: true})
	}
	data, err := os.ReadFile("/proc/cpuinfo")
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

func currentLinuxProcessorPhysicalCount(cpuinfoPath, sysCPUPath string) int {
	data, err := os.ReadFile(cpuinfoPath)
	if err != nil || len(data) == 0 {
		return 0
	}
	return linuxProcessorPhysicalCount(string(data), sysCPUPath)
}

func linuxProcessorPhysicalCount(cpuinfo, sysCPUPath string) int {
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

	entries, err := os.ReadDir(sysCPUPath)
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		name := entry.Name()
		if !linuxCPUEntryName(name) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sysCPUPath, name, "topology", "physical_package_id"))
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

func probeUptime(s *Session) uptimeInfo {
	return currentUptimeInfo(s, runtime.GOOS, os.ReadFile, s.commandOutput, time.Now)
}

func currentUptime(s *Session, goos string, readFile fileReader, run commandRunner, now func() time.Time) time.Duration {
	return currentUptimeInfo(s, goos, readFile, run, now).Duration
}

func currentUptimeInfo(s *Session, goos string, readFile fileReader, run commandRunner, now func() time.Time) uptimeInfo {
	if goos == "windows" {
		return currentWindowsUptime(goos, run)
	}
	if goos == "linux" {
		virtual := detectLinuxVirtualization(currentLinuxVirtualizationInputWithCommands(s, run))
		return currentLinuxUptimeInfo(readFile, run, now, virtual.Name == "docker")
	}
	return currentPosixUptime(readFile, run, now)
}

func currentLinuxUptimeInfo(readFile fileReader, run commandRunner, now func() time.Time, docker bool) uptimeInfo {
	if docker {
		seconds := parseDockerElapsedTimeSeconds(run("ps", "-o", "etime=", "-p", "1"))
		if seconds > 0 {
			return uptimeInfo{Duration: time.Duration(seconds) * time.Second, Known: true}
		}
	}
	return currentPosixUptime(readFile, run, now)
}

func currentPosixUptime(readFile fileReader, run commandRunner, now func() time.Time) uptimeInfo {
	if uptime := uptimeFromProc(readFile); uptime > 0 {
		return uptimeInfo{Duration: uptime, Known: true}
	}
	if uptime := uptimeFromKernelBoottime(run("sysctl", "-n", "kern.boottime"), now); uptime > 0 {
		return uptimeInfo{Duration: uptime, Known: true}
	}
	if out := run("uptime"); out != "" {
		seconds := parseUptimeCommandSeconds(out)
		if seconds > 0 {
			return uptimeInfo{Duration: time.Duration(seconds) * time.Second, Known: true}
		}
	}
	return uptimeInfo{}
}

func currentWindowsUptime(goos string, run commandRunner) uptimeInfo {
	if goos != "windows" {
		return uptimeInfo{}
	}
	values := parseWindowsWMIValues(windowsWMIOutput(run, "os", "LocalDateTime,LastBootUpTime"))
	if len(values) == 0 {
		debug("WMI query returned no resultsfor Win32_OperatingSystem with values LocalDateTime and LastBootUpTime.")
		debug("Unable to determine system uptime!")
		return uptimeInfo{}
	}
	local, ok := parseWindowsWMITime(values["LocalDateTime"])
	if !ok {
		debug("Unable to determine system uptime!")
		return uptimeInfo{}
	}
	boot, ok := parseWindowsWMITime(values["LastBootUpTime"])
	if !ok {
		debug("Unable to determine system uptime!")
		return uptimeInfo{}
	}
	uptime := local.Sub(boot)
	if uptime <= 0 {
		debug("Unable to determine system uptime!")
		return uptimeInfo{}
	}
	return uptimeInfo{Duration: uptime, Known: true}
}

func parseWindowsWMITime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if len(value) < len("20060102150405") {
		return time.Time{}, false
	}
	date := value[:len("20060102150405")]
	offset := value[len("20060102150405"):]
	if strings.HasPrefix(offset, ".") {
		plus := strings.IndexAny(offset, "+-")
		if plus == -1 {
			return time.Time{}, false
		}
		offset = offset[plus:]
	}
	offset, ok := windowsWMIOffset(offset)
	if !ok {
		return time.Time{}, false
	}
	parsed, err := time.Parse("20060102150405-0700", date+offset)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func windowsWMIOffset(offset string) (string, bool) {
	if len(offset) >= len("-0700") {
		return offset[:len("-0700")], true
	}
	if len(offset) != len("-420") {
		return "", false
	}
	minutes, err := strconv.Atoi(offset[1:])
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%s%02d%02d", offset[:1], minutes/60, minutes%60), true
}

func uptimeFromProc(readFile fileReader) time.Duration {
	data, err := readFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func uptimeFromKernelBoottime(input string, now func() time.Time) time.Duration {
	start := strings.Index(input, "sec = ")
	if start == -1 {
		return 0
	}
	start += len("sec = ")
	end := strings.IndexByte(input[start:], ',')
	if end == -1 {
		return 0
	}
	boot, err := strconv.ParseInt(input[start:start+end], 10, 64)
	if err != nil {
		return 0
	}
	return now().Sub(time.Unix(boot, 0))
}

func parseUptimeCommandSeconds(input string) int {
	_, duration, ok := strings.Cut(input, " up ")
	if !ok {
		return 0
	}
	fields := strings.Fields(strings.NewReplacer(",", " ").Replace(duration))
	seconds := 0
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "user", "users", "load", "loadavg":
			return seconds
		}
		if hours, minutes, ok := parseUptimeHoursMinutes(field); ok {
			seconds += hours*3600 + minutes*60
			continue
		}
		if i+1 >= len(fields) {
			continue
		}
		value, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		switch fields[i+1] {
		case "day", "days":
			seconds += value * 24 * 3600
			i++
		case "hr", "hrs", "hr(s)", "hour", "hours", "hour(s)":
			seconds += value * 3600
			i++
		case "min", "mins", "min(s)", "minute", "minutes", "minute(s)":
			seconds += value * 60
			i++
		case "user", "users":
			return seconds
		}
	}
	return seconds
}

func parseDockerElapsedTimeSeconds(input string) int {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0
	}

	var days int
	if before, after, ok := strings.Cut(input, "-"); ok {
		value, err := strconv.Atoi(before)
		if err != nil {
			return 0
		}
		days = value
		input = after
	}

	parts := strings.Split(input, ":")
	seconds := days * 24 * 3600
	switch len(parts) {
	case 1:
		value, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0
		}
		return seconds + value
	case 2:
		minutes, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0
		}
		value, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0
		}
		return seconds + minutes*60 + value
	case 3:
		hours, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0
		}
		minutes, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0
		}
		value, err := strconv.Atoi(parts[2])
		if err != nil {
			return 0
		}
		return seconds + hours*3600 + minutes*60 + value
	default:
		return 0
	}
}

func parseUptimeHoursMinutes(input string) (int, int, bool) {
	hoursText, minutesText, ok := strings.Cut(input, ":")
	if !ok {
		return 0, 0, false
	}
	hours, err := strconv.Atoi(hoursText)
	if err != nil {
		return 0, 0, false
	}
	minutes, err := strconv.Atoi(minutesText)
	if err != nil {
		return 0, 0, false
	}
	return hours, minutes, true
}

func probeLoadAverages(s *Session) map[string]any {
	return currentLoadAverages(runtime.GOOS, os.ReadFile, s.commandOutput)
}

type fileReader func(string) ([]byte, error)

func currentLoadAverages(goos string, readFile fileReader, run commandRunner) map[string]any {
	switch goos {
	case "darwin", "freebsd", "netbsd", "openbsd":
		out := run("sysctl", "-n", "vm.loadavg")
		if out == "" {
			return emptyLoadAverages()
		}
		return parseLoadAverages(out)
	case "linux":
		data, err := readFile("/proc/loadavg")
		if err != nil {
			return emptyLoadAverages()
		}
		return parseLoadAverages(string(data))
	default:
		return emptyLoadAverages()
	}
}

func probeFilesystems(s *Session) any {
	return currentFilesystems(runtime.GOOS, os.ReadFile, s.commandOutput)
}

// filesystemsFacts returns the filesystems fact, or nothing when the
// platform probe resolved no value (Windows and the BSDs, where Ruby Facter
// has no filesystems fact): an unresolvable fact is absent, never an empty
// string.
func filesystemsFacts(value any) []ResolvedFact {
	if value == nil || value == "" {
		return nil
	}
	return []ResolvedFact{{Name: "filesystems", Value: value}}
}

func currentFilesystems(goos string, readFile fileReader, run commandRunner) any {
	switch goos {
	case "darwin":
		if run == nil {
			return ""
		}
		return parseDarwinFilesystems(run("mount"))
	case "linux":
		data, err := readFile("/proc/filesystems")
		if err != nil {
			return nil
		}
		return parseLinuxFilesystems(string(data))
	default:
		return ""
	}
}

func parseLinuxFilesystems(input string) string {
	filesystems := make([]string, 0)
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 1 || fields[0] == "fuseblk" {
			continue
		}
		filesystems = append(filesystems, fields[0])
	}
	sort.Strings(filesystems)
	return strings.Join(filesystems, ",")
}

func parseDarwinFilesystems(input string) string {
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(input, "\n") {
		_, after, ok := strings.Cut(line, "(")
		if !ok {
			continue
		}
		value := after
		end := strings.IndexByte(value, ',')
		if end == -1 {
			end = strings.IndexByte(value, ')')
		}
		if end == -1 {
			continue
		}
		filesystem := value[:end]
		if filesystem != "" {
			seen[filesystem] = true
		}
	}
	filesystems := make([]string, 0, len(seen))
	for filesystem := range seen {
		filesystems = append(filesystems, filesystem)
	}
	sort.Strings(filesystems)
	return strings.Join(filesystems, ",")
}

func parseLoadAverages(input string) map[string]any {
	fields := strings.Fields(strings.Trim(input, "{} \t\r\n"))
	if len(fields) < 3 {
		return emptyLoadAverages()
	}

	averages := make(map[string]any, 3)
	for i, key := range []string{"1m", "5m", "15m"} {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return emptyLoadAverages()
		}
		averages[key] = value
	}
	return averages
}

func emptyLoadAverages() map[string]any {
	return map[string]any{"1m": nil, "5m": nil, "15m": nil}
}

func hostName() (string, any) {
	return hostNameForPlatform(runtime.GOOS, os.Hostname, readLinuxKernelHostname)
}

func hostNameForPlatform(goos string, lookup func() (string, error), linuxFallback func() string) (string, any) {
	if goos == "linux" {
		return linuxHostNameFromLookups(lookup, linuxFallback)
	}
	return hostNameFromLookup(lookup)
}

func linuxHostNameFromLookups(lookup func() (string, error), fallback func() string) (string, any) {
	hostname, value := hostNameFromLookup(lookup)
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

func readLinuxKernelHostname() string {
	data, err := os.ReadFile("/proc/sys/kernel/hostname")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func hostNameFromLookup(lookup func() (string, error)) (string, any) {
	hostname, err := lookup()
	if err != nil {
		debug("Socket.gethostname failed to return hostname")
		return "", nil
	}
	return hostname, hostname
}

func fqdn(hostname string) string {
	if hostname == "" || strings.Contains(hostname, ".") {
		return hostname
	}
	addrs, err := net.LookupAddr(hostname)
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
func currentHostnameFacts(goos, nodeName, resolvedFQDN, resolvConfPath string) (string, string, string) {
	hostname := hostnameFromNodeName(nodeName)
	fqdnName, domain := currentHostnameFQDNAndDomain(goos, hostname, resolvedFQDN, resolvConfPath)
	return hostname, fqdnName, domain
}

// hostnameFromNodeName returns the short host name: the node name up to the
// first dot.
func hostnameFromNodeName(nodeName string) string {
	hostname, _, _ := strings.Cut(nodeName, ".")
	return hostname
}

func currentHostnameFQDNAndDomain(goos, hostname, resolvedFQDN, resolvConfPath string) (string, string) {
	switch goos {
	case "linux", "darwin":
		return currentResolvConfFQDNAndDomain(hostname, resolvedFQDN, resolvConfPath)
	default:
		return resolvedFQDN, domainFromFQDN(hostname, resolvedFQDN)
	}
}

func currentResolvConfFQDNAndDomain(hostname, resolvedFQDN, resolvConfPath string) (string, string) {
	content, err := os.ReadFile(resolvConfPath)
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

const (
	ipv6RankGlobal = iota
	ipv6RankUniqueLocal
	ipv6RankLinkLocal
)

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

func osFamily(goos string, distro linuxDistro) string {
	switch goos {
	case "darwin":
		return "Darwin"
	case "windows":
		return "windows"
	case "linux":
		if distro.ID != "" {
			return discoverFamily(distro.ID)
		}
		return "Linux"
	case "freebsd":
		return "FreeBSD"
	case "netbsd":
		return "NetBSD"
	case "openbsd":
		return "OpenBSD"
	default:
		return goos
	}
}

func osName(goos string, distro linuxDistro) string {
	switch goos {
	case "darwin":
		return "Darwin"
	case "windows":
		return "windows"
	case "linux":
		if distro.Name != "" {
			return distro.Name
		}
		if distro.ID == "linuxmint" {
			return "Linuxmint"
		}
		if distro.ID != "" {
			return distro.ID
		}
		return "Linux"
	case "freebsd":
		return "FreeBSD"
	case "openbsd":
		return "OpenBSD"
	default:
		return goos
	}
}

func kernelName(goos string) string {
	switch goos {
	case "darwin":
		return "Darwin"
	case "windows":
		return "windows"
	case "linux":
		return "Linux"
	case "freebsd":
		return "FreeBSD"
	case "openbsd":
		return "OpenBSD"
	default:
		return goos
	}
}
