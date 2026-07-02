package engine

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	dmiDecodeVirtualBoxVersionPattern  = regexp.MustCompile(`vboxVer_(\S+)`)
	dmiDecodeVirtualBoxRevisionPattern = regexp.MustCompile(`vboxRev_(\S+)`)
	dmiDecodeAddressPattern            = regexp.MustCompile(`Address:\s(0x[a-zA-Z0-9]*)`)
)

var dmiDecodeVMwareVersions = map[int64]string{
	0xe8480: "ESXi 2.5",
	0xe7c70: "ESXi 3.0",
	0xe66c0: "ESXi 3.5",
	0xe7910: "ESXi 3.5",
	0xea550: "ESXi 4.0",
	0xea6c0: "ESXi 4.0",
	0xea2e0: "ESXi 4.1",
	0xe72c0: "ESXi 5.0",
	0xea0c0: "ESXi 5.1",
	0xea050: "ESXi 5.5",
	0xe99e0: "ESXi 6.0",
	0xe9a40: "ESXi 6.0",
	0xea580: "ESXi 6.5",
	0xea520: "ESXi 6.7",
	0xea490: "ESXi 6.7",
	0xea5e0: "Fusion 8.5",
}

var dmiProductHypervisors = []struct {
	substring string
	name      string
}{
	{"VMware", "vmware"},
	{"VirtualBox", "virtualbox"},
	{"Parallels", "parallels"},
	{"KVM", "kvm"},
	{"Virtual Machine", "hyperv"},
	{"RHEV Hypervisor", "rhev"},
	{"oVirt Node", "ovirt"},
	{"HVM domU", "xenhvm"},
	{"Bochs", "bochs"},
	{"OpenBSD", "vmm"},
	{"BHYVE", "bhyve"},
}

var openBSDProductHypervisors = map[string]string{
	"VMM":     "vmm",
	"vServer": "vserver",
	"oracle":  "virtualbox",
	"xen":     "xenu",
	"none":    "",
}

type dmiDecodeHypervisorInfo struct {
	VirtualBoxVersion  string
	VirtualBoxRevision string
	VMwareVersion      string
}

type virtualization struct {
	Name      string
	IsVirtual bool
	Unknown   bool
}

type linuxVirtualizationInput struct {
	CGroup           string
	DockerEnv        bool
	ContainerEnv     bool
	ProcVZ           bool
	LVEList          bool
	ProcVZEntries    int
	ProcStatus       string
	ContainerRuntime string
	KernelVersion    string
	MachineID        string
	DMIBIOSVendor    string
	DMIProductName   string
	DMISysVendor     string
	DMIDecodeInfo    dmiDecodeHypervisorInfo
	VirtWhatOutput   string
	VMwareCommand    string
	LspciOutput      string
}

type freeBSDVirtualizationInput struct {
	Jailed  bool
	VMGuest string
}

type openBSDVirtualizationInput struct {
	ProductName string
	Vendor      string
}

type dmiVirtualizationInput struct {
	Manufacturer string
	ProductName  string
	BIOSVendor   string
	PCIOutput    string
}

type windowsVirtualizationInput struct {
	Manufacturer     string
	Model            string
	OEMStrings       []string
	BIOSManufacturer string
	NetKVM           bool
	WMIResult        bool
}

func detectVirtualization(s *Session) virtualization {
	switch s.goos() {
	case "linux":
		return detectLinuxVirtualization(s.cachedLinuxVirtualizationInput())
	case "darwin":
		return detectMacOSVirtualization(s.cachedMacOSSystemProfilerHardware())
	case "freebsd":
		return detectFreeBSDVirtualization(currentFreeBSDVirtualizationInput(s.commandOutput))
	case "openbsd":
		return detectOpenBSDVirtualization(currentOpenBSDVirtualizationInput(s.commandOutput))
	case "netbsd":
		return detectDMIHostVirtualization(currentNetBSDVirtualizationInput(s.commandOutput))
	case "dragonfly":
		return detectDMIHostVirtualization(currentDragonFlyVirtualizationInput(s.commandOutput))
	case "illumos":
		return detectDMIHostVirtualization(currentIllumosVirtualizationInput(s.commandOutput))
	case "windows":
		return detectWindowsVirtualization(s.cachedWindowsVirtualizationInput())
	case "plan9":
		return virtualization{Unknown: true}
	default:
		return virtualization{Name: "physical"}
	}
}

func detectMacOSVirtualization(hardware macOSSystemProfilerHardware) virtualization {
	switch {
	case strings.HasPrefix(hardware.ModelIdentifier, "VMware"):
		return virtualization{Name: "vmware", IsVirtual: true}
	case strings.HasPrefix(hardware.BootROMVersion, "VirtualBox"):
		return virtualization{Name: "virtualbox", IsVirtual: true}
	case strings.HasPrefix(hardware.SubsystemVendorID, "0x1ab8"):
		return virtualization{Name: "parallels", IsVirtual: true}
	default:
		return virtualization{Name: "physical"}
	}
}

func currentFreeBSDVirtualizationInput(run func(string, ...string) string) freeBSDVirtualizationInput {
	return freeBSDVirtualizationInput{
		Jailed:  strings.TrimSpace(run("sysctl", "-n", "security.jail.jailed")) == "1",
		VMGuest: strings.TrimSpace(run("sysctl", "-n", "kern.vm_guest")),
	}
}

func detectFreeBSDVirtualization(input freeBSDVirtualizationInput) virtualization {
	switch {
	case input.Jailed:
		return virtualization{Name: "jail", IsVirtual: true}
	case input.VMGuest == "", input.VMGuest == "none":
		return virtualization{Name: "physical"}
	case input.VMGuest == "xen":
		return virtualization{Name: "xenu", IsVirtual: true}
	default:
		return virtualization{Name: input.VMGuest, IsVirtual: true}
	}
}

func currentOpenBSDVirtualizationInput(run func(string, ...string) string) openBSDVirtualizationInput {
	return openBSDVirtualizationInput{
		ProductName: strings.TrimSpace(run("sysctl", "-n", "hw.product")),
		Vendor:      strings.TrimSpace(run("sysctl", "-n", "hw.vendor")),
	}
}

func detectOpenBSDVirtualization(input openBSDVirtualizationInput) virtualization {
	if name, ok := openBSDProductHypervisors[input.ProductName]; ok {
		if name != "" {
			return virtualization{Name: name, IsVirtual: true}
		}
		return virtualization{Name: "physical"}
	}
	if virtual := detectDMIHostVirtualization(dmiVirtualizationInput{
		Manufacturer: input.Vendor,
		ProductName:  input.ProductName,
	}); virtual.IsVirtual {
		return virtual
	}
	return virtualization{Name: "physical"}
}

func currentNetBSDVirtualizationInput(run func(string, ...string) string) dmiVirtualizationInput {
	return dmiVirtualizationInput{
		Manufacturer: strings.TrimSpace(run("/sbin/sysctl", "-n", "machdep.dmi.system-vendor")),
		ProductName:  strings.TrimSpace(run("/sbin/sysctl", "-n", "machdep.dmi.system-product")),
	}
}

func currentDragonFlyVirtualizationInput(run func(string, ...string) string) dmiVirtualizationInput {
	system := parseColonValues(run("/usr/local/sbin/dmidecode", "-t", "system"))
	bios := parseColonValues(run("/usr/local/sbin/dmidecode", "-t", "bios"))
	return dmiVirtualizationInput{
		Manufacturer: strings.TrimSpace(firstNonEmpty(run("kenv", "smbios.system.maker"), system["Manufacturer"])),
		ProductName:  strings.TrimSpace(firstNonEmpty(run("kenv", "smbios.system.product"), system["Product Name"])),
		BIOSVendor:   strings.TrimSpace(firstNonEmpty(run("kenv", "smbios.bios.vendor"), bios["Vendor"])),
		PCIOutput:    run("pciconf", "-lv"),
	}
}

func currentIllumosVirtualizationInput(run func(string, ...string) string) dmiVirtualizationInput {
	system := parseIllumosSMBIOSValues(run("/usr/sbin/smbios", "-t", "SMB_TYPE_SYSTEM"))
	bios := parseIllumosSMBIOSValues(run("/usr/sbin/smbios", "-t", "SMB_TYPE_BIOS"))
	return dmiVirtualizationInput{
		Manufacturer: system["Manufacturer"],
		ProductName:  system["Product"],
		BIOSVendor:   bios["Vendor"],
		PCIOutput:    run("/usr/sbin/prtconf", "-pv"),
	}
}

func detectDMIHostVirtualization(input dmiVirtualizationInput) virtualization {
	manufacturerLower := strings.ToLower(input.Manufacturer)
	productNameLower := strings.ToLower(input.ProductName)
	if strings.Contains(manufacturerLower, "qemu") || strings.Contains(productNameLower, "qemu") {
		return virtualization{Name: "kvm", IsVirtual: true}
	}
	biosVendorLower := strings.ToLower(input.BIOSVendor)
	biosIndicatesKVM := strings.Contains(biosVendorLower, "qemu") || strings.Contains(biosVendorLower, "seabios")
	name := dmiProductHypervisor(input.ProductName)
	if name == "hyperv" && biosIndicatesKVM {
		return virtualization{Name: "kvm", IsVirtual: true}
	}
	if name != "" {
		return virtualization{Name: name, IsVirtual: true}
	}
	if biosIndicatesKVM {
		return virtualization{Name: "kvm", IsVirtual: true}
	}
	if name := lspciHypervisor(input.PCIOutput); name != "" {
		return virtualization{Name: name, IsVirtual: true}
	}
	switch {
	case strings.Contains(strings.ToLower(input.PCIOutput), "virtio"):
		return virtualization{Name: "kvm", IsVirtual: true}
	default:
		return virtualization{Name: "physical"}
	}
}

func currentWindowsVirtualizationInput(goos string, run commandRunner) windowsVirtualizationInput {
	if goos != "windows" {
		return windowsVirtualizationInput{}
	}
	records := parseWindowsWMIRecords(windowsWMIOutput(run, "computersystem", "Manufacturer,Model,OEMStringArray"))
	bios := parseWindowsWMIValues(windowsWMIOutput(run, "bios", "Manufacturer"))
	input := windowsVirtualizationInput{
		BIOSManufacturer: bios["Manufacturer"],
		NetKVM:           parseWindowsNetKVM(run("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Services`)),
	}
	if len(records) > 0 {
		input.WMIResult = true
		input.Manufacturer = records[0]["Manufacturer"]
		input.Model = records[0]["Model"]
		input.OEMStrings = parseWindowsOEMStrings(records)
	}
	return input
}

func detectWindowsVirtualization(input windowsVirtualizationInput) virtualization {
	if !input.WMIResult && input.Manufacturer == "" && input.Model == "" && len(input.OEMStrings) == 0 && input.BIOSManufacturer == "" && !input.NetKVM {
		return virtualization{Unknown: true}
	}
	model := strings.TrimSpace(input.Model)
	manufacturer := strings.TrimSpace(input.Manufacturer)
	biosManufacturer := strings.TrimSpace(input.BIOSManufacturer)
	modelLower := strings.ToLower(model)
	manufacturerLower := strings.ToLower(manufacturer)
	biosManufacturerLower := strings.ToLower(biosManufacturer)
	switch {
	case strings.Contains(model, "VirtualBox"):
		return virtualization{Name: "virtualbox", IsVirtual: true}
	case strings.Contains(model, "VMware"):
		return virtualization{Name: "vmware", IsVirtual: true}
	case strings.Contains(model, "KVM"):
		return virtualization{Name: "kvm", IsVirtual: true}
	case strings.Contains(model, "OpenStack"):
		return virtualization{Name: "openstack", IsVirtual: true}
	case strings.Contains(model, "AHV"):
		return virtualization{Name: "ahv", IsVirtual: true}
	case model == "Virtual Machine" && strings.Contains(manufacturer, "Microsoft"):
		return virtualization{Name: "hyperv", IsVirtual: true}
	case strings.Contains(manufacturer, "Xen"):
		return virtualization{Name: "xen", IsVirtual: true}
	case strings.Contains(manufacturer, "Amazon EC2"):
		return virtualization{Name: "kvm", IsVirtual: true}
	case strings.Contains(manufacturerLower, "qemu"), strings.Contains(modelLower, "qemu"), strings.Contains(modelLower, "standard pc (i440fx"), strings.Contains(biosManufacturerLower, "seabios"):
		return virtualization{Name: "kvm", IsVirtual: true}
	case input.NetKVM && strings.Contains(biosManufacturer, "Google"):
		return virtualization{Name: "gce", IsVirtual: true}
	case input.NetKVM:
		return virtualization{Name: "kvm", IsVirtual: true}
	default:
		return virtualization{Name: "physical"}
	}
}

func parseWindowsNetKVM(output string) bool {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), "netkvm") {
			return true
		}
	}
	return false
}

func parseWindowsOEMStrings(records []map[string]string) []string {
	stringsByValue := make([]string, 0, len(records))
	for _, record := range records {
		value := strings.TrimSpace(record["OEMStringArray"])
		if value != "" {
			stringsByValue = append(stringsByValue, value)
		}
	}
	return stringsByValue
}

func currentWindowsHypervisorFacts(s *Session) []ResolvedFact {
	if s.goos() != "windows" {
		return nil
	}
	return windowsHypervisorFacts(s.cachedWindowsVirtualizationInput())
}

func windowsHypervisorFacts(input windowsVirtualizationInput) []ResolvedFact {
	virtual := detectWindowsVirtualization(input).Name
	switch virtual {
	case "virtualbox":
		return []ResolvedFact{{Name: "hypervisors.virtualbox", Value: windowsVirtualBoxInfo(input.OEMStrings)}}
	case "vmware":
		return []ResolvedFact{{Name: "hypervisors.vmware", Value: map[string]any{}}}
	case "openstack":
		return []ResolvedFact{{Name: "hypervisors.kvm", Value: map[string]any{"openstack": true}}}
	case "gce":
		return []ResolvedFact{{Name: "hypervisors.kvm", Value: map[string]any{"google": true}}}
	case "kvm":
		return []ResolvedFact{{Name: "hypervisors.kvm", Value: map[string]any{}}}
	case "hyperv":
		return []ResolvedFact{{Name: "hypervisors.hyperv", Value: map[string]any{}}}
	case "xen":
		return []ResolvedFact{{Name: "hypervisors.xen", Value: map[string]any{"context": windowsXenContext(input.Model)}}}
	default:
		return nil
	}
}

func windowsVirtualBoxInfo(oemStrings []string) map[string]any {
	info := map[string]any{"version": "", "revision": ""}
	for _, value := range oemStrings {
		if version, ok := strings.CutPrefix(value, "vboxVer_"); ok {
			info["version"] = version
		}
		if revision, ok := strings.CutPrefix(value, "vboxRev_"); ok {
			info["revision"] = revision
		}
	}
	return info
}

func windowsXenContext(model string) string {
	if strings.Contains(model, "HVM") {
		return "hvm"
	}
	return "pv"
}

func currentLinuxVirtualizationInput(s *Session) linuxVirtualizationInput {
	return currentLinuxVirtualizationInputWithCommands(s, s.commandOutput)
}

func currentLinuxVirtualizationInputWithCommands(s *Session, run commandRunner) linuxVirtualizationInput {
	return linuxVirtualizationInput{
		CGroup:        readLinuxCGroup(s.readFile),
		DockerEnv:     fileExistsWithHost(s.host, "/.dockerenv"),
		ContainerEnv:  fileExistsWithHost(s.host, "/run/.containerenv"),
		ProcVZ:        dirExistsWithHost(s.host, "/proc/vz"),
		LVEList:       fileExistsWithHost(s.host, "/proc/lve/list"),
		ProcVZEntries: procVZEntryCount("/proc/vz"),
		ProcStatus:    readText("/proc/self/status", s.readFile),
		ContainerRuntime: containerRuntimeFromEnviron(
			readText("/proc/1/environ", s.readFile),
		),
		KernelVersion:  s.cachedKernelRelease(),
		MachineID:      strings.TrimSpace(readText("/etc/machine-id", s.readFile)),
		DMIBIOSVendor:  readDMIString("/sys/class/dmi/id", "bios_vendor", s.readFile),
		DMIProductName: readDMIString("/sys/class/dmi/id", "product_name", s.readFile),
		DMISysVendor:   readDMIString("/sys/class/dmi/id", "sys_vendor", s.readFile),
		DMIDecodeInfo:  parseDMIDecodeHypervisorInfo(run("dmidecode")),
		VirtWhatOutput: run("virt-what"),
		VMwareCommand:  run("vmware", "-v"),
		LspciOutput:    run("lspci"),
	}
}

func detectLinuxVirtualization(input linuxVirtualizationInput) virtualization {
	switch {
	case input.DockerEnv:
		return virtualization{Name: "docker", IsVirtual: true}
	case input.ContainerEnv:
		return virtualization{Name: "podman", IsVirtual: true}
	}
	if name := openVZVirtualization(input); name != "" {
		return virtualization{Name: name, IsVirtual: true}
	}
	cgroup := strings.ToLower(input.CGroup)
	switch {
	case strings.Contains(cgroup, "kubepods"):
		return virtualization{Name: "kubernetes", IsVirtual: true}
	case strings.Contains(cgroup, "docker") || strings.Contains(cgroup, "containerd"):
		return virtualization{Name: "docker", IsVirtual: true}
	case strings.Contains(cgroup, "/lxc"):
		return virtualization{Name: "lxc", IsVirtual: true}
	case input.ContainerRuntime == "systemd-nspawn":
		return virtualization{Name: "systemd_nspawn", IsVirtual: true}
	case input.ContainerRuntime == "lxc" && strings.Contains(input.KernelVersion, "BrandZ virtual linux"):
		return virtualization{Name: "illumos-lx", IsVirtual: true}
	case input.ContainerRuntime == "lxc" || input.ContainerRuntime == "lxc-virtwhat":
		return virtualization{Name: input.ContainerRuntime, IsVirtual: true}
	default:
		if strings.Contains(input.DMIBIOSVendor, "Google") {
			return virtualization{Name: "gce", IsVirtual: true}
		}
		if name := virtWhatVirtualization(input.VirtWhatOutput, input.ProcStatus); name != "" {
			return virtualization{Name: name, IsVirtual: true}
		}
		if virtual := detectDMIHostVirtualization(dmiVirtualizationInput{
			Manufacturer: input.DMISysVendor,
			ProductName:  input.DMIProductName,
			BIOSVendor:   input.DMIBIOSVendor,
			PCIOutput:    input.LspciOutput,
		}); virtual.IsVirtual {
			return virtual
		}
		if name := parseVMwareCommand(input.VMwareCommand); name != "" {
			return virtualization{Name: name, IsVirtual: true}
		}
		if name := lspciHypervisor(input.LspciOutput); name != "" {
			return virtualization{Name: name, IsVirtual: true}
		}
		return virtualization{Name: "physical"}
	}
}

func virtWhatVirtualization(output, procStatus string) string {
	if strings.HasPrefix(output, "xen\n") {
		switch {
		case strings.Contains(output, "xen-domu"):
			return "xenu"
		case strings.Contains(output, "xen-hvm"):
			return "xenhvm"
		case strings.Contains(output, "xen-dom0"):
			return "xen0"
		}
	}
	values := strings.FieldsSeq(output)
	for value := range values {
		if value == "redhat" {
			continue
		}
		if value == "linux_vserver" {
			return vserverVirtualization(procStatus)
		}
		return value
	}
	return ""
}

func vserverVirtualization(procStatus string) string {
	for line := range strings.SplitSeq(procStatus, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || (fields[0] != "s_context:" && fields[0] != "VxID:") {
			continue
		}
		if fields[1] == "0" {
			return "vserver_host"
		}
		return "vserver"
	}
	return ""
}

func parseVMwareCommand(output string) string {
	parts := strings.Fields(output)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[0]) + "_" + strings.ToLower(parts[1])
}

func lspciHypervisor(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(line, "VirtualBox"):
			return "virtualbox"
		case strings.Contains(line, "XenSource"):
			return "xenhvm"
		case strings.Contains(line, "Microsoft Corporation Hyper-V"):
			return "hyperv"
		case strings.Contains(line, "Class 8007: Google, Inc"):
			return "gce"
		case strings.Contains(line, "VMware") || strings.Contains(line, "VMWare"):
			return "vmware"
		case strings.Contains(line, "1ab8:") || strings.Contains(lower, "parallels"):
			return "parallels"
		case strings.Contains(lower, "virtio"):
			return "kvm"
		}
	}
	return ""
}

func dmiProductHypervisor(productName string) string {
	productName = strings.ToLower(productName)
	for _, hypervisor := range dmiProductHypervisors {
		if strings.Contains(productName, strings.ToLower(hypervisor.substring)) {
			return hypervisor.name
		}
	}
	return ""
}

func containerRuntimeFromEnviron(environ string) string {
	for entry := range strings.SplitSeq(environ, "\x00") {
		value, ok := strings.CutPrefix(entry, "container=")
		if ok {
			return value
		}
	}
	return ""
}

func openVZVirtualization(input linuxVirtualizationInput) string {
	id, ok := openVZEnvID(input)
	if !ok {
		return ""
	}
	if id == 0 {
		return "openvzhn"
	}
	return "openvzve"
}

func openVZEnvID(input linuxVirtualizationInput) (int, bool) {
	if !input.ProcVZ || input.LVEList || input.ProcVZEntries <= 2 {
		return 0, false
	}
	for line := range strings.SplitSeq(input.ProcStatus, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "envID:" {
			continue
		}
		id, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, false
		}
		return id, true
	}
	return 0, false
}

func readLinuxCGroup(readFiles ...fileReader) string {
	readFile := osHost{}.readFile
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	data, err := readFile("/proc/1/cgroup")
	if err != nil {
		return ""
	}
	return string(data)
}

func fileExists(path string) bool {
	return fileExistsWithHost(osHost{}, path)
}

func fileExistsWithHost(host hostOS, path string) bool {
	_, err := host.stat(path)
	return err == nil
}

func dirExistsWithHost(host hostOS, path string) bool {
	info, err := host.stat(path)
	return err == nil && info.IsDir()
}

func procVZEntryCount(path string) int {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	return len(entries) + 2
}

func currentLinuxHypervisorFacts(s *Session) []ResolvedFact {
	if s.goos() != "linux" {
		return nil
	}
	return linuxHypervisorFacts(s.cachedLinuxVirtualizationInput())
}

func linuxHypervisorFacts(input linuxVirtualizationInput) []ResolvedFact {
	if id, ok := openVZEnvID(input); ok {
		return []ResolvedFact{{Name: "hypervisors.openvz", Value: map[string]any{"id": id, "host": id == 0}}}
	}

	if input.DockerEnv || input.ContainerEnv {
		return []ResolvedFact{{Name: "hypervisors.docker", Value: map[string]any{}}, {Name: "hypervisors.lxc", Value: nil}, {Name: "hypervisors.systemd_nspawn", Value: nil}}
	}

	cgroup := strings.ToLower(input.CGroup)
	virtWhat := virtWhatVirtualization(input.VirtWhatOutput, input.ProcStatus)
	lspci := lspciHypervisor(input.LspciOutput)
	switch {
	case input.ContainerRuntime == "systemd-nspawn":
		info := map[string]any{}
		if input.MachineID != "" {
			info["id"] = input.MachineID
		}
		return []ResolvedFact{{Name: "hypervisors.systemd_nspawn", Value: info}}
	case input.ContainerRuntime == "lxc" || input.ContainerRuntime == "lxc-virtwhat":
		return []ResolvedFact{{Name: "hypervisors." + input.ContainerRuntime, Value: map[string]any{}}, {Name: "hypervisors.docker", Value: nil}}
	case strings.Contains(cgroup, "/lxc"):
		return []ResolvedFact{{Name: "hypervisors.lxc", Value: map[string]any{"name": lastCGroupPathSegment(input.CGroup)}}, {Name: "hypervisors.docker", Value: nil}}
	case strings.Contains(cgroup, "docker") || strings.Contains(cgroup, "containerd") || strings.Contains(cgroup, "kubepods"):
		return []ResolvedFact{{Name: "hypervisors.docker", Value: map[string]any{"id": lastCGroupPathSegment(input.CGroup)}}}
	case input.DMIProductName == "VirtualBox" || strings.Contains(virtWhat, "virtualbox"):
		info := map[string]any{}
		if input.DMIDecodeInfo.VirtualBoxVersion != "" {
			info["version"] = input.DMIDecodeInfo.VirtualBoxVersion
		}
		if input.DMIDecodeInfo.VirtualBoxRevision != "" {
			info["revision"] = input.DMIDecodeInfo.VirtualBoxRevision
		}
		return []ResolvedFact{{Name: "hypervisors.virtualbox", Value: info}}
	case input.DMIProductName == "VMware" || input.DMISysVendor == "VMware, Inc." || virtWhat == "vmware":
		info := map[string]any{}
		if input.DMIDecodeInfo.VMwareVersion != "" {
			info["version"] = input.DMIDecodeInfo.VMwareVersion
		}
		return []ResolvedFact{{Name: "hypervisors.vmware", Value: info}}
	case strings.Contains(input.DMISysVendor, "Microsoft") || input.DMIProductName == "Virtual Machine":
		return []ResolvedFact{{Name: "hypervisors.hyperv", Value: map[string]any{}}}
	case strings.Contains(virtWhat, "xen"):
		return []ResolvedFact{{Name: "hypervisors.xen", Value: xenHypervisorInfo(virtWhat)}}
	case virtWhat == "kvm":
		return []ResolvedFact{{Name: "hypervisors.kvm", Value: map[string]any{}}}
	case lspci == "virtualbox":
		return []ResolvedFact{{Name: "hypervisors.virtualbox", Value: map[string]any{}}}
	case lspci == "vmware":
		return []ResolvedFact{{Name: "hypervisors.vmware", Value: map[string]any{}}}
	case lspci == "kvm":
		return []ResolvedFact{{Name: "hypervisors.kvm", Value: map[string]any{}}}
	case lspci == "xenhvm":
		return []ResolvedFact{{Name: "hypervisors.xen", Value: map[string]any{}}}
	default:
		return []ResolvedFact{{Name: "hypervisors.openvz", Value: nil}, {Name: "hypervisors.docker", Value: nil}, {Name: "hypervisors.lxc", Value: nil}, {Name: "hypervisors.systemd_nspawn", Value: nil}}
	}
}

func xenHypervisorInfo(vm string) map[string]any {
	context := "pv"
	if vm == "xenhvm" {
		context = "hvm"
	}
	return map[string]any{"context": context, "privileged": vm == "xen0"}
}

func parseDMIDecodeHypervisorInfo(output string) dmiDecodeHypervisorInfo {
	info := dmiDecodeHypervisorInfo{
		VirtualBoxVersion:  firstRegexpCapture(dmiDecodeVirtualBoxVersionPattern, output),
		VirtualBoxRevision: firstRegexpCapture(dmiDecodeVirtualBoxRevisionPattern, output),
	}
	address := firstRegexpCapture(dmiDecodeAddressPattern, output)
	if address == "" {
		return info
	}
	value, err := strconv.ParseInt(address, 0, 64)
	if err != nil {
		return info
	}
	info.VMwareVersion = dmiDecodeVMwareVersions[value]
	return info
}

func firstRegexpCapture(pattern *regexp.Regexp, text string) string {
	matches := pattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func lastCGroupPathSegment(cgroup string) string {
	for line := range strings.SplitSeq(cgroup, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		_, path, ok := strings.Cut(line, ":/")
		if !ok {
			continue
		}
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 0 {
			continue
		}
		return parts[len(parts)-1]
	}
	return ""
}
