package engine

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

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

func disksFact(root string, hosts ...hostOS) map[string]any {
	var host hostOS = osHost{}
	if len(hosts) > 0 {
		host = hosts[0]
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	disks := make(map[string]any, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		deviceDir := filepath.Join(root, name, "device")
		info, err := host.stat(deviceDir)
		if err != nil || !info.IsDir() {
			continue
		}

		disk := make(map[string]any, 5)
		if model := readSysfsString(root, name, "device/model", host.readFile); model != "" {
			disk["model"] = model
		}
		if vendor := readSysfsString(root, name, "device/vendor", host.readFile); vendor != "" {
			disk["vendor"] = vendor
		}
		if rotational := readSysfsString(root, name, "queue/rotational", host.readFile); rotational != "" {
			if rotational == "0" {
				disk["type"] = "ssd"
			} else {
				disk["type"] = "hdd"
			}
		}
		if sectors, err := strconv.Atoi(readSysfsString(root, name, "size", host.readFile)); err == nil && sectors > 0 {
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

func currentDisks(goos string, run commandRunner, hosts ...hostOS) map[string]any {
	var host hostOS = osHost{}
	if len(hosts) > 0 {
		host = hosts[0]
	}
	switch goos {
	case "freebsd":
		return parseFreeBSDGeomDisks(run("sysctl", "-n", "kern.geom.confxml"))
	case "openbsd", "netbsd":
		return currentBSDDisks(goos, run)
	case "linux":
		return currentLinuxDisks("/sys/block", run, host)
	default:
		return nil
	}
}

func currentLinuxDisks(root string, run commandRunner, hosts ...hostOS) map[string]any {
	var host hostOS = osHost{}
	if len(hosts) > 0 {
		host = hosts[0]
	}
	disks := disksFact(root, host)
	if len(disks) == 0 || run == nil {
		return disks
	}

	for _, name := range sortedKeys(disks) {
		disk, ok := disks[name].(map[string]any)
		if !ok {
			continue
		}
		for _, field := range []struct {
			lsblk string
			fact  string
		}{
			{lsblk: "serial", fact: "serial_number"},
			{lsblk: "wwn", fact: "wwn"},
		} {
			if value := strings.TrimSpace(run("lsblk", "-dn", "-o", field.lsblk, "/dev/"+name)); value != "" {
				disk[field.fact] = value
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

func currentPartitions(s *Session) map[string]any {
	switch runtime.GOOS {
	case "freebsd":
		return parseFreeBSDGeomPartitions(s.commandOutput("sysctl", "-n", "kern.geom.confxml"))
	case "openbsd":
		return currentOpenBSDPartitions(s.commandOutput)
	case "netbsd":
		return currentNetBSDPartitions(s.commandOutput)
	case "linux":
		return currentLinuxPartitions("/sys/class/block", s.commandOutput, s.host)
	default:
		return nil
	}
}

func currentLinuxPartitions(root string, run commandRunner, hosts ...hostOS) map[string]any {
	host := hostOS(osHost{})
	if len(hosts) > 0 && hosts[0] != nil {
		host = hosts[0]
	}
	partitions := discoverPartitions(root, host)
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

func discoverPartitions(root string, hosts ...hostOS) map[string]any {
	host := hostOS(osHost{})
	if len(hosts) > 0 && hosts[0] != nil {
		host = hosts[0]
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	partitions := make(map[string]any, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if _, err := host.stat(filepath.Join(root, name, "partition")); err != nil {
			if _, err := host.stat(filepath.Join(root, name, "dm")); err == nil {
				device := "/dev/" + name
				if mapName := readSysfsString(root, filepath.Join(name, "dm"), "name", host.readFile); mapName != "" {
					device = "/dev/mapper/" + mapName
				}
				partition := make(map[string]any, 2)
				addLinuxPartitionSize(partition, root, name, host.readFile)
				partitions[device] = partition
				continue
			}
			if _, err := host.stat(filepath.Join(root, name, "loop")); err == nil {
				partition := make(map[string]any, 3)
				if backingFile := readSysfsString(root, filepath.Join(name, "loop"), "backing_file", host.readFile); backingFile != "" {
					partition["backing_file"] = backingFile
				}
				addLinuxPartitionSize(partition, root, name, host.readFile)
				partitions["/dev/"+name] = partition
				continue
			}
			continue
		}

		partition := make(map[string]any, 2)
		addLinuxPartitionSize(partition, root, name, host.readFile)
		partitions["/dev/"+name] = partition
	}
	if len(partitions) == 0 {
		return nil
	}
	return partitions
}

func addLinuxPartitionSize(partition map[string]any, root, name string, readFiles ...fileReader) {
	readFile := osHost{}.readFile
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	sectors, err := strconv.Atoi(readSysfsString(root, name, "size", readFile))
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

func currentBSDDisks(goos string, run commandRunner) map[string]any {
	if run == nil {
		return nil
	}
	devices := parseBSDDiskNames(goos, run("sysctl", "-n", "hw.disknames"))
	if len(devices) == 0 {
		return nil
	}

	disks := make(map[string]any, len(devices))
	for _, device := range devices {
		disk := parseBSDDisklabelDisk(currentBSDDisklabel(goos, device, run))
		if len(disk) > 0 {
			disks[device] = disk
		}
	}
	if len(disks) == 0 {
		return nil
	}
	return disks
}

func currentBSDDisklabel(goos, device string, run commandRunner) string {
	if goos == "netbsd" && isBSDDiskName(device) {
		return run("sh", "-c", "disklabel "+device+" 2>/dev/null || true")
	}
	return run("disklabel", device)
}

func currentOpenBSDPartitions(run commandRunner) map[string]any {
	if run == nil {
		return nil
	}
	partitions := map[string]any{}
	for _, device := range parseBSDDiskNames("openbsd", run("sysctl", "-n", "hw.disknames")) {
		for name, partition := range parseBSDDisklabelPartitions(device, run("disklabel", device)) {
			partitions[name] = partition
		}
	}
	if len(partitions) == 0 {
		return nil
	}
	return partitions
}

func currentNetBSDPartitions(run commandRunner) map[string]any {
	if run == nil {
		return nil
	}
	partitions := map[string]any{}
	for _, device := range parseBSDDiskNames("netbsd", run("sysctl", "-n", "hw.disknames")) {
		disklabel := currentBSDDisklabel("netbsd", device, run)
		sectorSize := parseBSDDisklabelSectorSize(disklabel)
		wedges := parseNetBSDDkctlWedges(run("dkctl", device, "listwedges"), sectorSize)
		for name, partition := range wedges {
			partitions[name] = partition
		}
		if len(wedges) == 0 {
			for name, partition := range parseBSDDisklabelPartitions(device, disklabel) {
				partitions[name] = partition
			}
		}
	}
	if len(partitions) == 0 {
		return nil
	}
	return partitions
}

func parseBSDDiskNames(goos, output string) []string {
	output = strings.TrimSpace(output)
	if _, after, ok := strings.Cut(output, "="); ok {
		output = strings.TrimSpace(after)
	}

	var devices []string
	for _, field := range strings.FieldsFunc(output, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		name := field
		if before, _, ok := strings.Cut(name, ":"); ok {
			name = before
		}
		name = strings.TrimSpace(name)
		if name == "" || goos == "netbsd" && isNetBSDWedgeName(name) {
			continue
		}
		devices = append(devices, name)
	}
	return devices
}

func isNetBSDWedgeName(name string) bool {
	if !strings.HasPrefix(name, "dk") || len(name) == 2 {
		return false
	}
	for _, r := range name[2:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isBSDDiskName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func parseBSDDisklabelDisk(input string) map[string]any {
	sectorSize := parseBSDDisklabelSectorSize(input)
	totalSectors := parseBSDDisklabelInt(input, "total sectors:")
	if sectorSize <= 0 || totalSectors <= 0 {
		return nil
	}
	sizeBytes := totalSectors * sectorSize
	return map[string]any{
		"size":       bytesToHumanReadable(sizeBytes),
		"size_bytes": sizeBytes,
	}
}

func parseBSDDisklabelSectorSize(input string) int {
	sectorSize := parseBSDDisklabelInt(input, "bytes/sector:")
	if sectorSize <= 0 {
		return 512
	}
	return sectorSize
}

func parseBSDDisklabelInt(input, key string) int {
	for line := range strings.Lines(input) {
		line = strings.TrimSpace(line)
		value, ok := strings.CutPrefix(line, key)
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			return 0
		}
		number, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0
		}
		return number
	}
	return 0
}

func parseBSDDisklabelPartitions(device, input string) map[string]any {
	sectorSize := parseBSDDisklabelSectorSize(input)
	if sectorSize <= 0 {
		return nil
	}
	partitions := map[string]any{}
	for line := range strings.Lines(input) {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 4 || !strings.HasSuffix(fields[0], ":") {
			continue
		}
		label := strings.TrimSuffix(fields[0], ":")
		if len(label) != 1 || label == "c" {
			continue
		}
		sizeSectors, err := strconv.Atoi(fields[1])
		if err != nil || sizeSectors <= 0 {
			continue
		}
		filesystem := fields[3]
		if filesystem == "" || filesystem == "unused" {
			continue
		}
		sizeBytes := sizeSectors * sectorSize
		partitions["/dev/"+device+label] = map[string]any{
			"filesystem": filesystem,
			"size":       bytesToHumanReadable(sizeBytes),
			"size_bytes": sizeBytes,
		}
	}
	if len(partitions) == 0 {
		return nil
	}
	return partitions
}

func parseNetBSDDkctlWedges(input string, sectorSize int) map[string]any {
	if sectorSize <= 0 {
		sectorSize = 512
	}
	partitions := map[string]any{}
	for line := range strings.Lines(input) {
		name, rest, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || !isNetBSDWedgeName(name) {
			continue
		}
		label, _, ok := strings.Cut(strings.TrimSpace(rest), ",")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		var blocks int
		filesystem := ""
		for i := 0; i < len(fields); i++ {
			if i+1 < len(fields) && fields[i+1] == "blocks" {
				if value, err := strconv.Atoi(strings.Trim(fields[i], ",")); err == nil {
					blocks = value
				}
			}
			if fields[i] == "type:" && i+1 < len(fields) {
				filesystem = strings.Trim(fields[i+1], ",")
			}
		}
		if blocks <= 0 || filesystem == "" {
			continue
		}
		sizeBytes := blocks * sectorSize
		partitions["/dev/"+name] = map[string]any{
			"filesystem": filesystem,
			"partlabel":  strings.TrimSpace(label),
			"size":       bytesToHumanReadable(sizeBytes),
			"size_bytes": sizeBytes,
		}
	}
	if len(partitions) == 0 {
		return nil
	}
	return partitions
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
		return currentOpenBSDMountpoints(s)
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

func currentOpenBSDMountpoints(s *Session) map[string]any {
	mountOutput := s.commandOutput("mount")
	if mountOutput == "" {
		return mountpointsFact([]mountEntry{{Path: "/"}}, statMountpoint)
	}
	dfOutput := s.commandOutput("df", "-P")
	return openBSDMountpointsFact(mountOutput, dfOutput)
}

func currentMountEntries(s *Session) []mountEntry {
	switch runtime.GOOS {
	case "darwin":
		out := s.commandOutput("mount")
		if out == "" {
			return nil
		}
		return parseDarwinMountEntries(out)
	case "freebsd":
		out := s.commandOutput("mount")
		if out == "" {
			return nil
		}
		return parseFreeBSDMountEntries(out)
	case "netbsd":
		out := s.commandOutput("mount")
		if out == "" {
			return nil
		}
		return parseBSDMountEntries(out)
	case "linux":
		data, err := s.readFile("/proc/self/mounts")
		if err != nil {
			return nil
		}
		return linuxMountEntriesWithRootDevice(parseLinuxMountEntries(string(data)), s.readFile, s.commandOutput)
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

func parseBSDMountOptions(rawOptions string) []string {
	rawOptions = strings.TrimSuffix(rawOptions, ")")
	fields := strings.Split(rawOptions, ",")
	options := make([]string, 0, len(fields))
	for _, field := range fields {
		option := strings.TrimSpace(field)
		if option != "" {
			options = append(options, option)
		}
	}
	return options
}

func parseFreeBSDMountEntries(input string) []mountEntry {
	return parseBSDMountEntries(input)
}

func parseBSDMountEntries(input string) []mountEntry {
	entries := make([]mountEntry, 0, strings.Count(input, "\n"))
	for line := range strings.SplitSeq(input, "\n") {
		device, rest, ok := strings.Cut(line, " on ")
		if !ok {
			continue
		}
		path, rawOptions, ok := strings.Cut(rest, " (")
		if !ok {
			var afterType string
			path, afterType, ok = strings.Cut(rest, " type ")
			if !ok {
				continue
			}
			filesystem, options, ok := strings.Cut(afterType, " (")
			if !ok {
				continue
			}
			entries = append(entries, mountEntry{Device: unescapeMountField(device), Path: unescapeMountField(path), Filesystem: strings.TrimSpace(filesystem), Options: parseBSDMountOptions(options)})
			continue
		}
		if cleanPath, filesystem, ok := strings.Cut(path, " type "); ok {
			entries = append(entries, mountEntry{Device: unescapeMountField(device), Path: unescapeMountField(cleanPath), Filesystem: strings.TrimSpace(filesystem), Options: parseBSDMountOptions(rawOptions)})
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
			mountpoint["capacity"] = filesystemCapacity(stats.UsedBytes, stats.AvailableBytes)
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

func currentZFSFacts(goos string, run commandRunner) []ResolvedFact {
	if run == nil || goos != "freebsd" && goos != "netbsd" {
		return nil
	}
	facts := zfsFactsFromUpgradeOutput(run("zfs", "upgrade", "-v"))
	facts = append(facts, zpoolFactsFromUpgradeOutput(run("zpool", "upgrade", "-v"))...)
	return facts
}

func zfsFactsFromUpgradeOutput(output string) []ResolvedFact {
	versions := zfsUpgradeNumbers(output)
	if len(versions) == 0 {
		return nil
	}
	return []ResolvedFact{
		{Name: "zfs.feature_numbers", Value: versions},
		{Name: "zfs.version", Value: versions[len(versions)-1]},
	}
}

func zpoolFactsFromUpgradeOutput(output string) []ResolvedFact {
	versions := zfsUpgradeNumbers(output)
	if len(versions) == 0 {
		return nil
	}
	featureFlags := zpoolFeatureFlags(output)
	facts := []ResolvedFact{
		{Name: "zpool.feature_numbers", Value: versions},
	}
	if len(featureFlags) > 0 {
		facts = append(facts, ResolvedFact{Name: "zpool.feature_flags", Value: featureFlags})
		facts = append(facts, ResolvedFact{Name: "zpool.version", Value: "5000"})
		return facts
	}
	facts = append(facts, ResolvedFact{Name: "zpool.version", Value: versions[len(versions)-1]})
	return facts
}

func zfsUpgradeNumbers(output string) []string {
	var versions []string
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) > 0 && isDecimalString(fields[0]) {
			versions = append(versions, fields[0])
		}
	}
	return versions
}

func zpoolFeatureFlags(output string) []string {
	var flags []string
	for line := range strings.Lines(output) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, suffix, _ := strings.Cut(line, " ")
		if !isZpoolFeatureFlagName(name) {
			continue
		}
		suffix = strings.TrimSpace(suffix)
		if suffix == "" || suffix == "(read-only compatible)" {
			flags = append(flags, name)
		}
	}
	return flags
}

func isZpoolFeatureFlagName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func isDecimalString(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// disksCoreFacts assembles the disks category facts (block devices, partitions,
// and mountpoints) for the current host.
func disksCoreFacts(s *Session) []ResolvedFact {
	disks := currentDisks(runtime.GOOS, s.commandOutput, s.host)
	var mountEntries []mountEntry
	if runtime.GOOS == "linux" {
		mountEntries = currentMountEntries(s)
	}
	mountpoints := rootMountpoint(s)
	facts := []ResolvedFact{
		{Name: "mountpoints", Value: mountpoints},
	}
	facts = append(facts, currentZFSFacts(runtime.GOOS, s.commandOutput)...)
	facts = append(facts, disksFacts(disks)...)
	facts = append(facts, partitionsFacts(partitionsFactWithMountEntries(currentPartitions(s), mountEntries, mountpoints))...)
	return facts
}
