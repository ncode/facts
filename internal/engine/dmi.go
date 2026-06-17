package engine

import (
	"log/slog"
	"runtime"
	"strconv"
	"strings"
)

func dmiFact(root string, readFiles ...fileReader) map[string]any {
	readFile := osHost{}.readFile
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	bios := mapFromDMI(root, map[string]string{
		"vendor":       "bios_vendor",
		"version":      "bios_version",
		"release_date": "bios_date",
	}, readFile)
	board := mapFromDMI(root, map[string]string{
		"manufacturer":  "board_vendor",
		"product":       "board_name",
		"serial_number": "board_serial",
		"asset_tag":     "board_asset_tag",
	}, readFile)
	chassis := mapFromDMI(root, map[string]string{
		"type":      "chassis_type",
		"asset_tag": "chassis_asset_tag",
	}, readFile)
	product := mapFromDMI(root, map[string]string{
		"name":          "product_name",
		"version":       "product_version",
		"serial_number": "product_serial",
		"uuid":          "product_uuid",
	}, readFile)

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
	if manufacturer := readDMIString(root, "sys_vendor", readFile); manufacturer != "" {
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

func mapFromDMI(root string, names map[string]string, readFiles ...fileReader) map[string]any {
	readFile := osHost{}.readFile
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	values := make(map[string]any, len(names))
	for key, filename := range names {
		if value := readDMIString(root, filename, readFile); value != "" {
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

type windowsDMI struct {
	Manufacturer string
	SerialNumber string
	ProductName  string
	ProductUUID  string
}

func currentWindowsDMI(goos string, run commandRunner, log *slog.Logger) windowsDMI {
	if goos != "windows" {
		return windowsDMI{}
	}
	bios := parseWindowsWMIValues(windowsWMIOutput(run, "bios", "Manufacturer,SerialNumber"))
	product := parseWindowsWMIValues(windowsWMIOutput(run, "computersystemproduct", "Name,UUID"))
	if len(bios) == 0 {
		log.Debug("WMI query returned no results for Win32_BIOS with values Manufacturer and SerialNumber.")
	}
	if len(product) == 0 {
		log.Debug("WMI query returned no results for Win32_ComputerSystemProduct with values Name and UUID.")
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

// dmiCoreFacts assembles the dmi category facts (the /sys/class/dmi bios/board/
// chassis/product facts plus the platform-specific FreeBSD, OpenBSD, Windows,
// and macOS DMI facts) for the current host.
func dmiCoreFacts(s *Session) []ResolvedFact {
	dmi := s.cachedDMI()
	facts := dmiFacts(dmi)
	facts = append(facts, macOSDMIFacts(s.cachedMacOSModel())...)
	facts = append(facts, windowsDMIFacts(currentWindowsDMI(runtime.GOOS, s.commandOutput, s.logr()))...)
	facts = append(facts, currentFreeBSDDMIFacts(s)...)
	facts = append(facts, currentOpenBSDDMIFacts(s)...)
	return facts
}
