package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCurrentWindowsDMIMatchesRubyResolvers(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		if name != "wmic" {
			t.Fatalf("command = %q %v, want wmic", name, args)
		}
		query := strings.Join(args, " ")
		switch query {
		case "bios get Manufacturer,SerialNumber /value":
			return "Manufacturer=VMware, Inc.\r\nSerialNumber=VMware-42 1a 38 c5 9d 35 5b f1-7a 62 4b 6e cb a0 79 de\r\n"
		case "computersystemproduct get Name,UUID /value":
			return "Name=VMware7,1\r\nUUID=C5381A42-359D-F15B-7A62-4B6ECBA079DE\r\n"
		default:
			t.Fatalf("wmic args = %q", query)
		}
		return ""
	}

	got := currentWindowsDMI("windows", run, discardLog())
	want := windowsDMI{
		Manufacturer: "VMware, Inc.",
		SerialNumber: "VMware-42 1a 38 c5 9d 35 5b f1-7a 62 4b 6e cb a0 79 de",
		ProductName:  "VMware7,1",
		ProductUUID:  "C5381A42-359D-F15B-7A62-4B6ECBA079DE",
	}
	if got != want {
		t.Fatalf("currentWindowsDMI() = %#v, want %#v", got, want)
	}
}

func TestCurrentWindowsDMILogsNoResultDiagnosticsLikeRubyResolvers(t *testing.T) {
	debugMessages := []string{}
	logger := captureLogger(&debugMessages, nil, nil)

	got := currentWindowsDMI("windows", func(string, ...string) string { return "" }, logger)
	if got != (windowsDMI{}) {
		t.Fatalf("currentWindowsDMI(empty WMI) = %#v, want empty DMI", got)
	}
	want := []string{
		"WMI query returned no results for Win32_BIOS with values Manufacturer and SerialNumber.",
		"WMI query returned no results for Win32_ComputerSystemProduct with values Name and UUID.",
	}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestWindowsDMIFactsReturnStructuredFacts(t *testing.T) {
	t.Parallel()

	core := windowsDMIFacts(windowsDMI{
		Manufacturer: "VMware, Inc.",
		SerialNumber: "VMware-42 1a 0d 03 0a b7 98 28-78 98 5e 85 a0 ad 18 47",
		ProductName:  "VMware7,1",
		ProductUUID:  "030D1A42-B70A-2898-7898-5E85A0AD1847",
	})

	if got := Collection(core); !reflect.DeepEqual(got, map[string]any{
		"dmi": map[string]any{
			"manufacturer": "VMware, Inc.",
			"product": map[string]any{
				"name":          "VMware7,1",
				"serial_number": "VMware-42 1a 0d 03 0a b7 98 28-78 98 5e 85 a0 ad 18 47",
				"uuid":          "030D1A42-B70A-2898-7898-5E85A0AD1847",
			},
		},
	}) {
		t.Fatalf("core facts = %#v", got)
	}
}

func TestDMIFacts_omittedWhenNoDataResolves(t *testing.T) {
	t.Parallel()

	if got := dmiFacts(nil); got != nil {
		t.Fatalf("dmiFacts(nil) = %#v, want nil", got)
	}
	if got := dmiFacts(map[string]any{}); got != nil {
		t.Fatalf("dmiFacts(empty) = %#v, want nil", got)
	}

	dmi := map[string]any{"manufacturer": "QEMU"}
	want := []ResolvedFact{{Name: "dmi", Value: dmi}}
	if got := dmiFacts(dmi); !reflect.DeepEqual(got, want) {
		t.Fatalf("dmiFacts() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxOSRelease_splitsDebianMajorAndMinorRelease(t *testing.T) {
	got := parseLinuxOSRelease("ID=debian\nVERSION_ID=10.02\n")

	want := map[string]any{"full": "10.02", "major": "10", "minor": "2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxOSRelease() = %#v, want %#v", got, want)
	}
}

func TestDMIFact_readsLinuxSysfsValues(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"bios_vendor":       "Acme BIOS\n",
		"bios_version":      "1.2.3\n",
		"bios_date":         "04/01/2026\n",
		"board_vendor":      "Acme Board\n",
		"board_name":        "Board 9000\n",
		"board_serial":      "BOARD123\n",
		"board_asset_tag":   "BOARDTAG\n",
		"chassis_type":      "Laptop\n",
		"chassis_asset_tag": "CHASSISTAG\n",
		"product_name":      "NodeBook\n",
		"product_version":   "Pro\n",
		"product_serial":    "SER123\n",
		"product_uuid":      "uuid-123\n",
		"sys_vendor":        "Acme Systems\n",
		"product_family":    "ignored\n",
	}
	for name, value := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got := dmiFact(dir)
	want := map[string]any{
		"bios": map[string]any{
			"vendor":       "Acme BIOS",
			"version":      "1.2.3",
			"release_date": "04/01/2026",
		},
		"board": map[string]any{
			"manufacturer":  "Acme Board",
			"product":       "Board 9000",
			"serial_number": "BOARD123",
			"asset_tag":     "BOARDTAG",
		},
		"chassis": map[string]any{
			"type":      "Laptop",
			"asset_tag": "CHASSISTAG",
		},
		"product": map[string]any{
			"name":          "NodeBook",
			"version":       "Pro",
			"serial_number": "SER123",
			"uuid":          "uuid-123",
		},
		"manufacturer": "Acme Systems",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dmiFact() = %#v, want %#v", got, want)
	}
}

func TestDMIFact_mapsLinuxNumericChassisType(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chassis_type"), []byte("4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := dmiFact(dir)
	want := map[string]any{
		"chassis": map[string]any{
			"type": "Low Profile Desktop",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dmiFact() = %#v, want %#v", got, want)
	}
}

func TestDMIFact_replacesInvalidUTF8InLinuxSysfsValues(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sys_vendor"), []byte("Supermicro^L\x8dD$Pptal0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := dmiFact(dir)
	want := map[string]any{
		"manufacturer": "Supermicro^L�D$Pptal0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dmiFact() = %#v, want %#v", got, want)
	}
}

func TestFreeBSDDMIFacts_returnsStructuredFacts(t *testing.T) {
	values := map[string]string{
		"smbios.bios.reldate":     "12/12/2018",
		"smbios.bios.vendor":      "Phoenix Technologies LTD",
		"smbios.bios.version":     "6.00",
		"smbios.system.maker":     "VMware, Inc.",
		"smbios.system.product":   "VMware Virtual Platform",
		"smbios.system.serial":    "VMware-42 1a 46 19 2d fc 12 90-73 48 ea 8f 1a 37 cb 95",
		"smbios.system.uuid":      "421a4619-2dfc-1290-7348-ea8f1a37cb95",
		"smbios.system.unrelated": "ignored",
	}

	facts := freeBSDDMIFacts(values)
	collection := Collection(facts)

	want := map[string]any{
		"dmi": map[string]any{
			"bios": map[string]any{
				"vendor":       "Phoenix Technologies LTD",
				"version":      "6.00",
				"release_date": "12/12/2018",
			},
			"manufacturer": "VMware, Inc.",
			"product": map[string]any{
				"name":          "VMware Virtual Platform",
				"serial_number": "VMware-42 1a 46 19 2d fc 12 90-73 48 ea 8f 1a 37 cb 95",
				"uuid":          "421a4619-2dfc-1290-7348-ea8f1a37cb95",
			},
		},
	}
	if !reflect.DeepEqual(collection, want) {
		t.Fatalf("freeBSDDMIFacts() = %#v, want %#v", collection, want)
	}
}

func TestOpenBSDDMIFacts_returnsStructuredFacts(t *testing.T) {
	values := map[string]string{
		"hw.vendor":    "Phoenix Technologies LTD",
		"hw.version":   "6.00",
		"hw.product":   "VMware Virtual Platform",
		"hw.serialno":  "VMware-42 1a 02 ea e6 27 76 b8-a1 23 a7 8a d3 12 ee cf",
		"hw.uuid":      "ea021a42-27e6-b876-a123-a78ad312eecf",
		"hw.unrelated": "ignored",
	}

	facts := openBSDDMIFacts(values)
	collection := Collection(facts)

	want := map[string]any{
		"dmi": map[string]any{
			"bios": map[string]any{
				"vendor":  "Phoenix Technologies LTD",
				"version": "6.00",
			},
			"manufacturer": "Phoenix Technologies LTD",
			"product": map[string]any{
				"name":          "VMware Virtual Platform",
				"serial_number": "VMware-42 1a 02 ea e6 27 76 b8-a1 23 a7 8a d3 12 ee cf",
				"uuid":          "ea021a42-27e6-b876-a123-a78ad312eecf",
				"version":       "6.00",
			},
		},
	}
	if !reflect.DeepEqual(collection, want) {
		t.Fatalf("openBSDDMIFacts() = %#v, want %#v", collection, want)
	}
}

func TestNetBSDDMIFacts_returnsStructuredFacts(t *testing.T) {
	values := map[string]string{
		"machdep.dmi.system-vendor":    "QEMU",
		"machdep.dmi.system-product":   "QEMU Virtual Machine",
		"machdep.dmi.system-version":   "virt-11.0",
		"machdep.dmi.system-serial":    "",
		"machdep.dmi.system-uuid":      "00000000-0000-0000-0000-000000000000",
		"machdep.dmi.system-unrelated": "ignored",
	}

	facts := netBSDDMIFacts(values)
	collection := Collection(facts)

	want := map[string]any{
		"dmi": map[string]any{
			"bios": map[string]any{
				"vendor":  "QEMU",
				"version": "virt-11.0",
			},
			"manufacturer": "QEMU",
			"product": map[string]any{
				"name":    "QEMU Virtual Machine",
				"uuid":    "00000000-0000-0000-0000-000000000000",
				"version": "virt-11.0",
			},
		},
	}
	if !reflect.DeepEqual(collection, want) {
		t.Fatalf("netBSDDMIFacts() = %#v, want %#v", collection, want)
	}
}

func TestMacOSDMIFacts_returnsProductName(t *testing.T) {
	core := macOSDMIFacts("MacBookPro11,4")

	wantCore := []ResolvedFact{{Name: "dmi.product.name", Value: "MacBookPro11,4"}}
	if !reflect.DeepEqual(core, wantCore) {
		t.Fatalf("macOSDMIFacts() core = %#v, want %#v", core, wantCore)
	}
}

func TestMacOSDMIFacts_skipsEmptyProductName(t *testing.T) {
	core := macOSDMIFacts("")

	if core != nil {
		t.Fatalf("macOSDMIFacts() = %#v, want nil facts", core)
	}
}

func TestCurrentLinuxDistro_mapsAmazonAMISystemReleaseIDAndMissingCodename(t *testing.T) {
	files := map[string]string{
		"/etc/os-release":     "ID=amzn\nVERSION_ID=2017.03\n",
		"/etc/system-release": "Amazon Linux AMI release 2017.03\n",
	}
	readFile := func(path string) ([]byte, error) {
		value, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(value), nil
	}

	got := currentLinuxDistro("linux", func(string) (string, error) { return "", os.ErrNotExist }, func(string, ...string) string { return "" }, readFile)
	want := linuxDistro{
		ID:           "AmazonAMI",
		Description:  "Amazon Linux AMI release 2017.03",
		Codename:     "n/a",
		Release:      map[string]any{"full": "2017.03", "major": "2017", "minor": "03"},
		ReleaseKnown: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxDistro() = %#v, want %#v", got, want)
	}
}

func TestOpenBSDNames_matchRubyFactNames(t *testing.T) {
	t.Parallel()

	if got := osName("openbsd", linuxDistro{}); got != "OpenBSD" {
		t.Fatalf("osName(openbsd) = %q, want OpenBSD", got)
	}
	if got := kernelName("openbsd"); got != "OpenBSD" {
		t.Fatalf("kernelName(openbsd) = %q, want OpenBSD", got)
	}
}

func TestFreeBSDNames_matchRubyFactNames(t *testing.T) {
	t.Parallel()

	if got := osName("freebsd", linuxDistro{}); got != "FreeBSD" {
		t.Fatalf("osName(freebsd) = %q, want FreeBSD", got)
	}
	if got := kernelName("freebsd"); got != "FreeBSD" {
		t.Fatalf("kernelName(freebsd) = %q, want FreeBSD", got)
	}
	if got := osFamily("freebsd", linuxDistro{}); got != "FreeBSD" {
		t.Fatalf("osFamily(freebsd) = %q, want FreeBSD", got)
	}
	if got := architectureName("freebsd", "amd64"); got != "amd64" {
		t.Fatalf("architectureName(freebsd) = %q, want amd64", got)
	}
}
