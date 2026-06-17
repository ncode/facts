package engine

import (
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type freeBSDVersions struct {
	InstalledKernel   string
	RunningKernel     string
	InstalledUserland string
}

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

func probeKernelRelease(s *Session) string {
	return strings.TrimSpace(s.commandOutput("uname", "-r"))
}

func probeHardwareModel(s *Session) string {
	if runtime.GOOS == "windows" {
		return windowsHardwareFromGoArch(runtime.GOARCH)
	}
	out := s.commandOutput("uname", "-m")
	if out == "" {
		return runtime.GOARCH
	}
	return strings.TrimSpace(out)
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
	return currentOSRelease(s, runtime.GOOS, s.readFile, s.commandOutput)
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

func currentWindowsKernelFacts(input string, log *slog.Logger) []ResolvedFact {
	info := parseWindowsOSVersionInfo(input)
	if info.Version == "" {
		log.Debug("Calling Windows RtlGetVersion failed")
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
	out := s.commandOutput("system_profiler", "SPHardwareDataType")
	if out == "" {
		return macOSSystemProfilerHardware{}
	}
	return parseMacOSSystemProfilerHardware(out)
}

func probeMacOSSystemProfilerSoftware(s *Session) macOSSystemProfilerSoftware {
	if runtime.GOOS != "darwin" {
		return macOSSystemProfilerSoftware{}
	}
	out := s.commandOutput("system_profiler", "SPSoftwareDataType")
	if out == "" {
		return macOSSystemProfilerSoftware{}
	}
	return parseMacOSSystemProfilerSoftware(out)
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

type windowsProductRelease struct {
	EditionID        string
	InstallationType string
	ProductName      string
	ReleaseID        string
	DisplayVersion   string
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
	return currentLinuxDistro(runtime.GOOS, exec.LookPath, s.commandOutput, s.readFile)
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

func probeFilesystems(s *Session) any {
	return currentFilesystems(runtime.GOOS, s.readFile, s.commandOutput)
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

// osCoreFacts assembles the os category facts (kernel name/release/version,
// os name/family/release/architecture/hardware, filesystems, the Linux distro
// facts, and the macOS and Windows OS-description facts) for the current host.
func osCoreFacts(s *Session) []ResolvedFact {
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
		kernelFacts := currentWindowsKernelFacts(s.cachedWindowsOSVersionInput(), s.logr())
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
	facts := []ResolvedFact{
		{Name: "kernel", Value: kernelName},
		{Name: "kernelmajversion", Value: kernelMajorVersion},
		{Name: "kernelrelease", Value: kernelRelease},
		{Name: "kernelversion", Value: kernelVersion},
		{Name: "os.architecture", Value: architecture},
		{Name: "os.family", Value: osFamily},
		{Name: "os.hardware", Value: hardwareModel},
		{Name: "os.name", Value: osName},
		{Name: "os.release", Value: osRelease},
	}
	facts = append(facts, filesystemsFacts(s.cachedFilesystems())...)
	facts = append(facts, linuxDistroFacts(linuxDistro)...)
	macOSInfo := s.cachedMacOSInfo()
	facts = append(facts, macOSVersionFacts(macOSInfo.ProductVersion, macOSInfo.ProductVersionExtra)...)
	facts = append(facts, macOSStringFact("os.macosx.product", macOSInfo.ProductName)...)
	facts = append(facts, macOSStringFact("os.macosx.build", macOSInfo.BuildVersion)...)
	facts = append(facts, macOSSystemProfilerFacts(s.cachedMacOSSystemProfilerHardware())...)
	facts = append(facts, macOSSystemProfilerSoftwareFacts(s.cachedMacOSSystemProfilerSoftware())...)
	facts = append(facts, macOSSystemProfilerEthernetFacts(s.cachedMacOSSystemProfilerEthernet())...)
	facts = append(facts, windowsSystem32Facts(currentWindowsSystem32(runtime.GOOS, os.Getenv("SystemRoot"), currentWindowsProcessWOW64))...)
	facts = append(facts, windowsProductReleaseFacts(currentWindowsProductRelease(runtime.GOOS, s.commandOutput))...)
	return facts
}
