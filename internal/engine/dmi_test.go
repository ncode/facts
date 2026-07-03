package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCurrentWindowsDMIMatchesRubyResolvers(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{
		platform: "windows",
		runOutputs: map[string]string{
			fakeRunKey("wmic", "bios", "get", "Manufacturer,SerialNumber", "/value"):  "Manufacturer=VMware, Inc.\r\nSerialNumber=VMware-42 1a 38 c5 9d 35 5b f1-7a 62 4b 6e cb a0 79 de\r\n",
			fakeRunKey("wmic", "computersystemproduct", "get", "Name,UUID", "/value"): "Name=VMware7,1\r\nUUID=C5381A42-359D-F15B-7A62-4B6ECBA079DE\r\n",
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	got := currentWindowsDMI(s)
	want := windowsDMI{
		Manufacturer: "VMware, Inc.",
		SerialNumber: "VMware-42 1a 38 c5 9d 35 5b f1-7a 62 4b 6e cb a0 79 de",
		ProductName:  "VMware7,1",
		ProductUUID:  "C5381A42-359D-F15B-7A62-4B6ECBA079DE",
	}
	if got != want {
		t.Fatalf("currentWindowsDMI() = %#v, want %#v", got, want)
	}
	wantCalls := []fakeHostRunCall{
		{name: "wmic", args: []string{"bios", "get", "Manufacturer,SerialNumber", "/value"}},
		{name: "wmic", args: []string{"computersystemproduct", "get", "Name,UUID", "/value"}},
	}
	if !reflect.DeepEqual(host.runCalls, wantCalls) {
		t.Fatalf("run calls = %#v, want %#v", host.runCalls, wantCalls)
	}
}

func TestCurrentWindowsDMILogsNoResultDiagnosticsLikeRubyResolvers(t *testing.T) {
	debugMessages := []string{}
	host := &fakeHostOS{platform: "windows", emptyRunDefault: true}
	s := NewSessionContext(context.Background())
	s.host = host
	s.logger = captureLogger(&debugMessages, nil, nil)

	got := currentWindowsDMI(s)
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

func TestDMIBIOSVendorReadsNestedVendorOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dmi  map[string]any
		want string
	}{
		{name: "missing bios", dmi: map[string]any{"manufacturer": "Acme"}, want: ""},
		{name: "wrong bios shape", dmi: map[string]any{"bios": "Acme BIOS"}, want: ""},
		{name: "missing vendor", dmi: map[string]any{"bios": map[string]any{"version": "1.0"}}, want: ""},
		{name: "vendor", dmi: map[string]any{"bios": map[string]any{"vendor": "SeaBIOS"}}, want: "SeaBIOS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := dmiBIOSVendor(tt.dmi); got != tt.want {
				t.Fatalf("dmiBIOSVendor() = %q, want %q", got, tt.want)
			}
		})
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

func TestDMIFact_mapsUnknownLinuxNumericChassisType(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chassis_type"), []byte("2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := dmiFact(dir)
	want := map[string]any{
		"chassis": map[string]any{
			"type": "Unknown",
		},
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

func TestDragonFlyDMIFacts_fallsBackToDMIDecodeWhenKenvHasNoSMBIOS(t *testing.T) {
	bios := `BIOS Information
	Vendor: SeaBIOS
	Version: 1.16.3-debian-1.16.3-2
	Release Date: 04/01/2014
`
	system := `System Information
	Manufacturer: QEMU
	Product Name: Standard PC (i440FX + PIIX, 1996)
	Version: pc-i440fx-10.0
	Serial Number: Not Specified
	UUID: 5641762d-4a52-4876-8ea2-46464bba9824
`
	chassis := `Chassis Information
	Manufacturer: QEMU
	Type: Other
	Version: pc-i440fx-10.0
	Serial Number: Not Specified
	Asset Tag: Not Specified
`

	facts := dragonFlyDMIFacts(map[string]string{}, bios, system, chassis)
	collection := Collection(facts)

	want := map[string]any{
		"dmi": map[string]any{
			"bios": map[string]any{
				"vendor":       "SeaBIOS",
				"version":      "1.16.3-debian-1.16.3-2",
				"release_date": "04/01/2014",
			},
			"chassis": map[string]any{
				"asset_tag": "Not Specified",
				"type":      "Other",
			},
			"manufacturer": "QEMU",
			"product": map[string]any{
				"name":          "Standard PC (i440FX + PIIX, 1996)",
				"serial_number": "Not Specified",
				"uuid":          "5641762d-4a52-4876-8ea2-46464bba9824",
			},
		},
	}
	if !reflect.DeepEqual(collection, want) {
		t.Fatalf("dragonFlyDMIFacts() = %#v, want %#v", collection, want)
	}
}

func TestDragonFlyDMIFactsPrefersKenvSMBIOSValues(t *testing.T) {
	values := map[string]string{
		"smbios.system.maker":   "DragonFly Maker",
		"smbios.system.product": "DragonFly Product",
	}

	facts := dragonFlyDMIFacts(values, "Vendor: dmidecode BIOS", "Manufacturer: dmidecode", "Type: Other")
	collection := Collection(facts)

	want := map[string]any{
		"dmi": map[string]any{
			"manufacturer": "DragonFly Maker",
			"product": map[string]any{
				"name": "DragonFly Product",
			},
		},
	}
	if !reflect.DeepEqual(collection, want) {
		t.Fatalf("dragonFlyDMIFacts() = %#v, want %#v", collection, want)
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

func TestBSDDMIFactsOmitEmptyValues(t *testing.T) {
	if got := openBSDDMIFacts(nil); got != nil {
		t.Fatalf("openBSDDMIFacts(nil) = %#v, want nil", got)
	}
	if got := netBSDDMIFacts(map[string]string{"machdep.dmi.system-vendor": "   "}); got != nil {
		t.Fatalf("netBSDDMIFacts(blank) = %#v, want nil", got)
	}
}

func TestCurrentBSDDMIFactsQueryPlatformSources(t *testing.T) {
	t.Parallel()

	t.Run("freebsd", func(t *testing.T) {
		t.Parallel()

		host := &fakeHostOS{
			platform:        "freebsd",
			emptyRunDefault: true,
			runOutputs: map[string]string{
				fakeRunKey("/bin/kenv", "smbios.system.maker"): "FreeBSD Maker\n",
			},
		}
		s := NewSessionContext(context.Background())
		s.host = host

		facts := currentFreeBSDDMIFacts(s)
		if got := Collection(facts)["dmi"].(map[string]any)["manufacturer"]; got != "FreeBSD Maker" {
			t.Fatalf("freebsd manufacturer = %#v, want FreeBSD Maker", got)
		}
		wantCalls := make([]fakeHostRunCall, 0, len(freeBSDDMIKeys))
		for _, key := range freeBSDDMIKeys {
			wantCalls = append(wantCalls, fakeHostRunCall{name: "/bin/kenv", args: []string{key}})
		}
		if !reflect.DeepEqual(host.runCalls, wantCalls) {
			t.Fatalf("freebsd DMI run calls = %#v, want %#v", host.runCalls, wantCalls)
		}
	})

	t.Run("dragonfly", func(t *testing.T) {
		t.Parallel()

		host := &fakeHostOS{
			platform:        "dragonfly",
			emptyRunDefault: true,
			runOutputs: map[string]string{
				fakeRunKey("kenv", "smbios.system.maker"):               "DragonFly Maker\n",
				fakeRunKey("/usr/local/sbin/dmidecode", "-t", "system"): "Manufacturer: fallback\n",
			},
		}
		s := NewSessionContext(context.Background())
		s.host = host

		facts := currentDragonFlyDMIFacts(s)
		if got := Collection(facts)["dmi"].(map[string]any)["manufacturer"]; got != "DragonFly Maker" {
			t.Fatalf("dragonfly manufacturer = %#v, want DragonFly Maker", got)
		}
		for _, call := range host.runCalls {
			if call.name == "/usr/local/sbin/dmidecode" {
				t.Fatal("dragonfly DMI queried dmidecode despite kenv SMBIOS data")
			}
		}
		wantCalls := make([]fakeHostRunCall, 0, len(freeBSDDMIKeys))
		for _, key := range freeBSDDMIKeys {
			wantCalls = append(wantCalls, fakeHostRunCall{name: "kenv", args: []string{key}})
		}
		if !reflect.DeepEqual(host.runCalls, wantCalls) {
			t.Fatalf("dragonfly DMI run calls = %#v, want %#v", host.runCalls, wantCalls)
		}
	})

	t.Run("openbsd", func(t *testing.T) {
		t.Parallel()

		host := &fakeHostOS{
			platform:        "openbsd",
			emptyRunDefault: true,
			runOutputs: map[string]string{
				fakeRunKey("/sbin/sysctl", "-n", "hw.vendor"): "OpenBSD Vendor\n",
			},
		}
		s := NewSessionContext(context.Background())
		s.host = host

		facts := currentOpenBSDDMIFacts(s)
		if got := Collection(facts)["dmi"].(map[string]any)["manufacturer"]; got != "OpenBSD Vendor" {
			t.Fatalf("openbsd manufacturer = %#v, want OpenBSD Vendor", got)
		}
		wantCalls := make([]fakeHostRunCall, 0, len(openBSDDMIKeys))
		for _, key := range openBSDDMIKeys {
			wantCalls = append(wantCalls, fakeHostRunCall{name: "/sbin/sysctl", args: []string{"-n", key}})
		}
		if !reflect.DeepEqual(host.runCalls, wantCalls) {
			t.Fatalf("openbsd DMI run calls = %#v, want %#v", host.runCalls, wantCalls)
		}
	})

	t.Run("netbsd", func(t *testing.T) {
		t.Parallel()

		host := &fakeHostOS{
			platform:        "netbsd",
			emptyRunDefault: true,
			runOutputs: map[string]string{
				fakeRunKey("/sbin/sysctl", "-n", "machdep.dmi.system-vendor"): "NetBSD Vendor\n",
			},
		}
		s := NewSessionContext(context.Background())
		s.host = host

		facts := currentNetBSDDMIFacts(s)
		if got := Collection(facts)["dmi"].(map[string]any)["manufacturer"]; got != "NetBSD Vendor" {
			t.Fatalf("netbsd manufacturer = %#v, want NetBSD Vendor", got)
		}
		wantCalls := make([]fakeHostRunCall, 0, len(netBSDDMIKeys))
		for _, key := range netBSDDMIKeys {
			wantCalls = append(wantCalls, fakeHostRunCall{name: "/sbin/sysctl", args: []string{"-n", key}})
		}
		if !reflect.DeepEqual(host.runCalls, wantCalls) {
			t.Fatalf("netbsd DMI run calls = %#v, want %#v", host.runCalls, wantCalls)
		}
	})

	t.Run("illumos", func(t *testing.T) {
		t.Parallel()

		host := &fakeHostOS{
			platform:        "illumos",
			emptyRunDefault: true,
			runOutputs: map[string]string{
				fakeRunKey("/usr/sbin/smbios", "-t", "SMB_TYPE_SYSTEM"): "Manufacturer: illumos Maker\n",
			},
		}
		s := NewSessionContext(context.Background())
		s.host = host

		facts := currentIllumosDMIFacts(s)
		if got := Collection(facts)["dmi"].(map[string]any)["manufacturer"]; got != "illumos Maker" {
			t.Fatalf("illumos manufacturer = %#v, want illumos Maker", got)
		}
		wantCalls := []fakeHostRunCall{
			{name: "/usr/sbin/smbios", args: []string{"-t", "SMB_TYPE_BIOS"}},
			{name: "/usr/sbin/smbios", args: []string{"-t", "SMB_TYPE_SYSTEM"}},
			{name: "/usr/sbin/smbios", args: []string{"-t", "SMB_TYPE_CHASSIS"}},
		}
		if !reflect.DeepEqual(host.runCalls, wantCalls) {
			t.Fatalf("illumos DMI run calls = %#v, want %#v", host.runCalls, wantCalls)
		}
	})
}

func TestCurrentPlatformDMIFactsSkipOtherPlatforms(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{platform: "linux", emptyRunDefault: true}
	s := NewSessionContext(context.Background())
	s.host = host

	if got := currentFreeBSDDMIFacts(s); got != nil {
		t.Fatalf("currentFreeBSDDMIFacts(linux) = %#v, want nil", got)
	}
	if got := currentDragonFlyDMIFacts(s); got != nil {
		t.Fatalf("currentDragonFlyDMIFacts(linux) = %#v, want nil", got)
	}
	if got := currentOpenBSDDMIFacts(s); got != nil {
		t.Fatalf("currentOpenBSDDMIFacts(linux) = %#v, want nil", got)
	}
	if got := currentNetBSDDMIFacts(s); got != nil {
		t.Fatalf("currentNetBSDDMIFacts(linux) = %#v, want nil", got)
	}
	if got := currentIllumosDMIFacts(s); got != nil {
		t.Fatalf("currentIllumosDMIFacts(linux) = %#v, want nil", got)
	}
	if len(host.runCalls) != 0 {
		t.Fatalf("DMI platform helper ran command for non-matching platform: %#v", host.runCalls)
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

func TestIllumosDMIFacts_returnsStructuredFactsFromSMBIOS(t *testing.T) {
	bios := `ID    SIZE TYPE
0     65   SMB_TYPE_BIOS (type 0) (BIOS information)
  Vendor: SeaBIOS
  Version String: 1.16.3-debian-1.16.3-2
  Release Date: 04/01/2014
`
	system := `ID    SIZE TYPE
256   80   SMB_TYPE_SYSTEM (type 1) (system information)
  Manufacturer: QEMU
  Product: Standard PC (i440FX + PIIX, 1996)
  Version: pc-i440fx-10.0
  Serial Number: vm-42
  UUID: ee850d94-5288-714f-b992-0a71f8df356f
  UUID (Endian-corrected): 940d85ee-8852-4f71-b992-0a71f8df356f
`
	chassis := `ID    SIZE TYPE
768   41   SMB_TYPE_CHASSIS (type 3) (system enclosure or chassis)
  Asset Tag: chassis-42
  Chassis Type: 0x1 (other)
`

	facts := illumosDMIFacts(bios, system, chassis)
	collection := Collection(facts)

	want := map[string]any{
		"dmi": map[string]any{
			"bios": map[string]any{
				"vendor":       "SeaBIOS",
				"version":      "1.16.3-debian-1.16.3-2",
				"release_date": "04/01/2014",
			},
			"chassis": map[string]any{
				"asset_tag": "chassis-42",
				"type":      "0x1 (other)",
			},
			"manufacturer": "QEMU",
			"product": map[string]any{
				"name":          "Standard PC (i440FX + PIIX, 1996)",
				"serial_number": "vm-42",
				"uuid":          "ee850d94-5288-714f-b992-0a71f8df356f",
			},
		},
	}
	if !reflect.DeepEqual(collection, want) {
		t.Fatalf("illumosDMIFacts() = %#v, want %#v", collection, want)
	}
}

func TestIllumosDMIFactsFallsBackToChassisTypeKey(t *testing.T) {
	chassis := `ID    SIZE TYPE
768   41   SMB_TYPE_CHASSIS (type 3) (system enclosure or chassis)
  Type: rack
`

	facts := illumosDMIFacts("", "", chassis)
	collection := Collection(facts)

	want := map[string]any{
		"dmi": map[string]any{
			"chassis": map[string]any{"type": "rack"},
		},
	}
	if !reflect.DeepEqual(collection, want) {
		t.Fatalf("illumosDMIFacts() = %#v, want %#v", collection, want)
	}
}

func TestIllumosDMIFacts_omitsDMIWhenSMBIOSHasNoValues(t *testing.T) {
	if got := illumosDMIFacts("", "", ""); got != nil {
		t.Fatalf("illumosDMIFacts(empty) = %#v, want nil", got)
	}
}

func TestParseWindowsWMIRecordsSkipsMalformedLinesAndSplitsRepeatedNames(t *testing.T) {
	input := strings.Join([]string{
		"Name=CPU One",
		"malformed",
		"NumberOfCores=2",
		"Name=CPU Two",
		"NumberOfCores=4",
	}, "\r\n")

	got := parseWindowsWMIRecords(input)
	want := []map[string]string{
		{"Name": "CPU One", "NumberOfCores": "2"},
		{"Name": "CPU Two", "NumberOfCores": "4"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWindowsWMIRecords() = %#v, want %#v", got, want)
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
	host := &fakeHostOS{
		platform:        "linux",
		emptyRunDefault: true,
		files: map[string][]byte{
			"/etc/os-release":     []byte("ID=amzn\nVERSION_ID=2017.03\n"),
			"/etc/system-release": []byte("Amazon Linux AMI release 2017.03\n"),
		},
	}
	s := NewSessionContext(t.Context())
	s.host = host

	got := currentLinuxDistro(s, func(string) (string, error) { return "", os.ErrNotExist })
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
