package engine

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNetworkingDHCPFactUsesPrimaryInterfaceDHCPValue(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"eth0": map[string]any{
			"bindings": []any{map[string]any{"address": "10.0.0.12"}},
			"dhcp":     "10.0.0.1",
		},
		"en1": map[string]any{
			"bindings": []any{map[string]any{"address": "192.168.1.20"}},
			"dhcp":     "192.168.1.1",
		},
	}

	if got, want := networkingDHCPFact(interfaces, "10.0.0.12"), "10.0.0.1"; got != want {
		t.Fatalf("networkingDHCPFact() = %q, want %q", got, want)
	}
}

func TestCurrentNetworkingDataExpandsWindowsInterfaceBindingsAndPrimary(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"Ethernet0": map[string]any{
			"bindings": []any{map[string]any{
				"address": "10.16.127.3",
				"netmask": "255.255.255.0",
				"network": "10.16.127.0",
			}},
			"bindings6": []any{map[string]any{
				"address": "fe80::7ca0:ab22:703a:b329",
				"netmask": "ffff:ff00::",
				"network": "fe80::",
				"scope6":  "link",
			}},
			"mac": "00:50:56:9A:F8:6B",
			"mtu": 1500,
		},
	}

	primary, got := currentNetworkingData("windows", interfaces, nil)
	if primary != "Ethernet0" {
		t.Fatalf("primary = %q, want Ethernet0", primary)
	}
	want := map[string]any{
		"Ethernet0": map[string]any{
			"bindings": []any{map[string]any{
				"address": "10.16.127.3",
				"netmask": "255.255.255.0",
				"network": "10.16.127.0",
			}},
			"bindings6": []any{map[string]any{
				"address": "fe80::7ca0:ab22:703a:b329",
				"netmask": "ffff:ff00::",
				"network": "fe80::",
				"scope6":  "link",
			}},
			"dhcp":     nil,
			"ip":       "10.16.127.3",
			"ip6":      "fe80::7ca0:ab22:703a:b329",
			"mac":      "00:50:56:9A:F8:6B",
			"mtu":      1500,
			"netmask":  "255.255.255.0",
			"netmask6": "ffff:ff00::",
			"network":  "10.16.127.0",
			"network6": "fe80::",
			"scope6":   "link",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentNetworkingData(windows) = %#v, want %#v", got, want)
	}
}

func TestNetworkInterfaceIsUsableSkipsDownInterfacesLikeWindowsResolver(t *testing.T) {
	t.Parallel()

	if networkInterfaceIsUsable(net.Interface{Name: "Ethernet0"}) {
		t.Fatal("networkInterfaceIsUsable(down interface) = true, want false")
	}
	if !networkInterfaceIsUsable(net.Interface{Name: "Ethernet0", Flags: net.FlagUp}) {
		t.Fatal("networkInterfaceIsUsable(up interface) = false, want true")
	}
}

func TestNetworkingInterfacesSkipsWindowsLoopbackLikeRubyNetworkingResolver(t *testing.T) {
	t.Parallel()

	_, loopback, err := net.ParseCIDR("127.0.0.1/8")
	if err != nil {
		t.Fatal(err)
	}
	loopback.IP = net.ParseIP("127.0.0.1")
	_, ethernet, err := net.ParseCIDR("10.16.127.3/24")
	if err != nil {
		t.Fatal(err)
	}
	ethernet.IP = net.ParseIP("10.16.127.3")

	got := networkingInterfacesFromSnapshots([]networkInterfaceSnapshot{
		{
			Interface: net.Interface{
				Name:  "Loopback Pseudo-Interface 1",
				MTU:   1500,
				Flags: net.FlagUp | net.FlagLoopback,
			},
			Addrs: []net.Addr{loopback},
		},
		{
			Interface: net.Interface{
				Name:  "Ethernet0",
				MTU:   1500,
				Flags: net.FlagUp,
			},
			Addrs: []net.Addr{ethernet},
		},
	}, "windows")

	if _, ok := got["Loopback Pseudo-Interface 1"]; ok {
		t.Fatalf("windows loopback interface was included: %#v", got)
	}
	if _, ok := got["Ethernet0"]; !ok {
		t.Fatalf("Ethernet0 missing from networking interfaces: %#v", got)
	}
}

func TestNetworkingInterfacesWindowsLogsFailureLikeRubyResolver(t *testing.T) {
	var messages []string
	s := NewSession()
	s.logger = captureLogger(&messages, nil, nil)

	got := networkingInterfacesForPlatform(s, "windows", func() ([]networkInterfaceSnapshot, error) {
		return nil, errors.New("adapter failure")
	})

	if got != nil {
		t.Fatalf("networkingInterfacesForPlatform(testSession) = %#v, want nil", got)
	}
	want := []string{"Unable to retrieve networking facts!"}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("debug messages = %#v, want %#v", messages, want)
	}
}

func TestNetworkingInterfacesWindowsReplacesInvalidFriendlyNameLikeRubyResolver(t *testing.T) {
	t.Parallel()

	interfaces := networkingInterfacesFromSnapshots([]networkInterfaceSnapshot{
		{
			Interface: net.Interface{
				Name:  string([]byte{0xf0, 0x9f, 0x92}),
				MTU:   1500,
				Flags: net.FlagUp,
			},
		},
	}, "windows")

	if _, ok := interfaces["\uFFFD"]; !ok {
		t.Fatalf("networkingInterfacesFromSnapshots() = %#v, want invalid Windows interface name replaced", interfaces)
	}
}

func TestFormatInterfaceMACUppercasesWindowsLikeRubyNetworkUtils(t *testing.T) {
	t.Parallel()

	hw := net.HardwareAddr{0x00, 0x50, 0x56, 0x9a, 0xf8, 0x6b}
	if got, want := formatInterfaceMAC("windows", hw), "00:50:56:9A:F8:6B"; got != want {
		t.Fatalf("formatInterfaceMAC(windows) = %q, want %q", got, want)
	}
	if got, want := formatInterfaceMAC("linux", hw), "00:50:56:9a:f8:6b"; got != want {
		t.Fatalf("formatInterfaceMAC(linux) = %q, want %q", got, want)
	}
}

func TestCurrentNetworkingDataWindowsIgnoresLinkLocalPrimaryBindings(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"Ethernet0": map[string]any{
			"bindings":  []any{map[string]any{"address": "169.254.10.20"}},
			"bindings6": []any{map[string]any{"address": "fe80::7ca0:ab22:703a:b329"}},
		},
	}

	primary, _ := currentNetworkingData("windows", interfaces, nil)
	if primary != "" {
		t.Fatalf("primary = %q, want empty for link-local-only interface", primary)
	}
}

func TestCurrentNetworkingDataAddsWindowsDHCPServerFromIPConfig(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"Ethernet0": map[string]any{
			"bindings": []any{map[string]any{"address": "10.16.127.3", "netmask": "255.255.255.0", "network": "10.16.127.0"}},
		},
		"Ethernet1": map[string]any{
			"bindings": []any{map[string]any{"address": "10.16.128.3", "netmask": "255.255.255.0", "network": "10.16.128.0"}},
		},
	}
	run := func(name string, args ...string) string {
		if name != "ipconfig" || !reflect.DeepEqual(args, []string{"/all"}) {
			t.Fatalf("run(%q, %#v), want ipconfig /all", name, args)
		}
		return strings.Join([]string{
			"Windows IP Configuration",
			"",
			"Ethernet adapter Ethernet0:",
			"   Connection-specific DNS Suffix  . : example.test",
			"   IPv4 Address. . . . . . . . . . . : 10.16.127.3(Preferred)",
			"   DHCP Server . . . . . . . . . . . : 10.16.127.1",
			"",
			"Ethernet adapter Ethernet1:",
			"   IPv4 Address. . . . . . . . . . . : 10.16.128.3(Preferred)",
		}, "\n")
	}

	primary, got := currentNetworkingData("windows", interfaces, run)

	if primary != "Ethernet0" {
		t.Fatalf("primary = %q, want Ethernet0", primary)
	}
	if got := got["Ethernet0"].(map[string]any)["dhcp"]; got != "10.16.127.1" {
		t.Fatalf("Ethernet0[dhcp] = %#v, want 10.16.127.1", got)
	}
	if got := got["Ethernet1"].(map[string]any)["dhcp"]; got != nil {
		t.Fatalf("Ethernet1[dhcp] = %#v, want nil", got)
	}
}

func TestCurrentNetworkingDataAddsWindowsDNSSuffixFromIPConfig(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"Ethernet0": map[string]any{
			"bindings": []any{map[string]any{"address": "10.16.127.3", "netmask": "255.255.255.0", "network": "10.16.127.0"}},
		},
	}
	run := func(name string, args ...string) string {
		if name != "ipconfig" || !reflect.DeepEqual(args, []string{"/all"}) {
			t.Fatalf("run(%q, %#v), want ipconfig /all", name, args)
		}
		return strings.Join([]string{
			"Windows IP Configuration",
			"",
			"Ethernet adapter Ethernet0:",
			"   Connection-specific DNS Suffix  . : adapter.example",
			"   IPv4 Address. . . . . . . . . . . : 10.16.127.3(Preferred)",
		}, "\n")
	}

	_, got := currentNetworkingData("windows", interfaces, run)
	if got := got["Ethernet0"].(map[string]any)["dns_suffix"]; got != "adapter.example" {
		t.Fatalf("Ethernet0[dns_suffix] = %#v, want adapter.example", got)
	}
}

func TestCurrentWindowsNetworkingDomainPrefersInterfaceDNSSuffix(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"Ethernet0": map[string]any{"dns_suffix": "adapter.example"},
	}
	run := func(name string, args ...string) string {
		t.Fatalf("unexpected command = %s %#v", name, args)
		return ""
	}

	if got := currentWindowsNetworkingDomain(interfaces, run); got != "adapter.example" {
		t.Fatalf("currentWindowsNetworkingDomain() = %q, want adapter.example", got)
	}
}

func TestCurrentWindowsNetworkingDomainFallsBackToRegistryDomain(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		if name != "reg" || !reflect.DeepEqual(args, []string{"query", `HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`, "/v", "Domain"}) {
			t.Fatalf("command = %s %#v", name, args)
		}
		return strings.Join([]string{
			`HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`,
			"    Domain    REG_SZ    registry.example",
		}, "\n")
	}

	if got := currentWindowsNetworkingDomain(map[string]any{}, run); got != "registry.example" {
		t.Fatalf("currentWindowsNetworkingDomain() = %q, want registry.example", got)
	}
}

func TestCurrentWindowsNetworkingDomainReturnsEmptyWhenNetworkingCollectionFails(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		t.Fatalf("unexpected command = %s %#v", name, args)
		return ""
	}

	if got := currentWindowsNetworkingDomain(nil, run); got != "" {
		t.Fatalf("currentWindowsNetworkingDomain() = %q, want empty domain", got)
	}
}

func TestLinuxRouteSourceBindingsParsesRouteShowSourceAddresses(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		"default via 10.16.112.1 dev ens192 proto dhcp src 10.16.125.217 metric 100",
		"10.16.112.0/20 dev ens192 proto kernel scope link src 10.16.125.217",
		"fe80::/64 dev ens160 proto kernel metric 256 pref medium",
	}, "\n")

	got := linuxRouteSourceBindings(content)
	want := []routeSourceBinding{{Interface: "ens192", IP: "10.16.125.217"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linuxRouteSourceBindings() = %#v, want %#v", got, want)
	}
}

func TestLinuxRouteSourceBindingsSkipsLinkdownRoutes(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		"default via 10.16.112.1 dev ens192 proto dhcp src 10.16.125.217 metric 100 linkdown",
		"default via 10.16.112.1 dev ens160 proto dhcp src 10.16.125.218 metric 100",
	}, "\n")

	got := linuxRouteSourceBindings(content)
	want := []routeSourceBinding{{Interface: "ens160", IP: "10.16.125.218"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linuxRouteSourceBindings() = %#v, want %#v", got, want)
	}
}

func TestLinuxProcGetenvForPIDMatchesRubyProcHelper(t *testing.T) {
	t.Parallel()

	readLines := func(path string, defaultValue []string) ([]string, bool) {
		if path != "/proc/1/environ" {
			t.Fatalf("path = %q, want /proc/1/environ", path)
		}
		return []string{"container=podman", "bubbles=", "HOME=/root"}, true
	}

	tests := []struct {
		name  string
		field string
		want  string
		ok    bool
	}{
		{name: "field exists", field: "container", want: "podman", ok: true},
		{name: "field missing", field: "butter"},
		{name: "field empty", field: "bubbles", want: "", ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := linuxProcGetenvForPID(1, tt.field, readLines)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("linuxProcGetenvForPID() = %q, %v, want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestAddRouteSourceBindingsAddsMissingInterfaceAddresses(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"ens192": map[string]any{
			"bindings": []any{map[string]any{"address": "10.16.112.10", "netmask": "255.255.240.0"}},
		},
		"ens160": map[string]any{"mac": "00:50:56:9a:61:46"},
	}
	routes := []routeSourceBinding{
		{Interface: "ens192", IP: "10.16.125.217"},
		{Interface: "ens192", IP: "10.16.112.10"},
		{Interface: "missing", IP: "192.0.2.1"},
		{Interface: "ens160", IP: "192.0.2.2"},
	}

	addRouteSourceBindings(interfaces, "bindings", routes)

	ens192 := interfaces["ens192"].(map[string]any)
	if got := ens192["bindings"]; !reflect.DeepEqual(got, []any{
		map[string]any{"address": "10.16.112.10", "netmask": "255.255.240.0"},
		map[string]any{"address": "10.16.125.217"},
	}) {
		t.Fatalf("ens192 bindings = %#v", got)
	}
	ens160 := interfaces["ens160"].(map[string]any)
	if got := ens160["bindings"]; !reflect.DeepEqual(got, []any{map[string]any{"address": "192.0.2.2"}}) {
		t.Fatalf("ens160 bindings = %#v", got)
	}
	if _, ok := interfaces["missing"]; ok {
		t.Fatalf("interfaces = %#v, want unknown route interface ignored", interfaces)
	}
}

func TestParseLinuxIfInet6Flags_matchesRubyIfInet6(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"00000000000000000000000000000001 01 80 10 80       lo",
		"20010db8000000000000000000000001 02 40 20 01       temporary",
		"20010db8000000000000000000000002 02 40 20 02       noad",
		"20010db8000000000000000000000003 02 40 20 04       optimistic",
		"20010db8000000000000000000000004 02 40 20 08       dadfailed",
		"20010db8000000000000000000000005 02 40 20 10       homeaddress",
		"20010db8000000000000000000000006 02 40 20 20       deprecated",
		"20010db8000000000000000000000007 02 40 20 40       tentative",
		"20010db8000000000000000000000008 02 40 20 80       permanent",
		"20010db8000000000000000000000009 02 40 20 ff       everything",
	}, "\n")

	got := parseLinuxIfInet6Flags(input)
	want := map[string]map[string][]string{
		"lo":          {"::1": {"permanent"}},
		"temporary":   {"2001:db8::1": {"temporary"}},
		"noad":        {"2001:db8::2": {"noad"}},
		"optimistic":  {"2001:db8::3": {"optimistic"}},
		"dadfailed":   {"2001:db8::4": {"dadfailed"}},
		"homeaddress": {"2001:db8::5": {"homeaddress"}},
		"deprecated":  {"2001:db8::6": {"deprecated"}},
		"tentative":   {"2001:db8::7": {"tentative"}},
		"permanent":   {"2001:db8::8": {"permanent"}},
		"everything":  {"2001:db8::9": {"temporary", "noad", "optimistic", "dadfailed", "homeaddress", "deprecated", "tentative", "permanent"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxIfInet6Flags() = %#v, want %#v", got, want)
	}
}

func TestAddLinuxIfInet6FlagsToBindings(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"ens160": map[string]any{
			"bindings6": []any{map[string]any{"address": "fe80::250:56ff:fe9a:8481", "scope6": "link"}},
		},
	}
	flags := map[string]map[string][]string{
		"ens160": {"fe80::250:56ff:fe9a:8481": {"permanent"}},
	}

	addLinuxIfInet6Flags(interfaces, flags)
	binding := interfaces["ens160"].(map[string]any)["bindings6"].([]any)[0].(map[string]any)
	if got, want := binding["flags"], []string{"permanent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("binding flags = %#v, want %#v", got, want)
	}
}

func TestAddLinuxInterfaceMetadata_matchesRubyNetworkingResolver(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sys/class/net/lo/operstate"), "unknown\n")
	writeFile(t, filepath.Join(root, "sys/class/net/ens160/operstate"), "up\n")
	writeFile(t, filepath.Join(root, "sys/class/net/ens160/speed"), "1000\n")
	writeFile(t, filepath.Join(root, "sys/class/net/ens160/duplex"), "full\n")
	if err := os.MkdirAll(filepath.Join(root, "sys/class/net/ens160/device"), 0o755); err != nil {
		t.Fatal(err)
	}
	interfaces := map[string]any{
		"lo":     map[string]any{"mtu": 65536},
		"ens160": map[string]any{"mtu": 1500},
	}

	addLinuxInterfaceMetadataFromRoot(root, interfaces)

	lo := interfaces["lo"].(map[string]any)
	if got := lo["operational_state"]; got != "unknown" {
		t.Fatalf("lo operational_state = %#v, want unknown", got)
	}
	if got := lo["physical"]; got != false {
		t.Fatalf("lo physical = %#v, want false", got)
	}
	ens160 := interfaces["ens160"].(map[string]any)
	if got := ens160["operational_state"]; got != "up" {
		t.Fatalf("ens160 operational_state = %#v, want up", got)
	}
	if got := ens160["physical"]; got != true {
		t.Fatalf("ens160 physical = %#v, want true", got)
	}
	if got := ens160["speed"]; got != 1000 {
		t.Fatalf("ens160 speed = %#v, want 1000", got)
	}
	if got := ens160["duplex"]; got != "full" {
		t.Fatalf("ens160 duplex = %#v, want full", got)
	}
}

func TestNetworkingDHCPFactIsEmptyWithoutPrimaryDHCPValue(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"eth0": map[string]any{"bindings": []any{map[string]any{"address": "10.0.0.12"}}},
	}

	if got := networkingDHCPFact(interfaces, "10.0.0.12"); got != "" {
		t.Fatalf("networkingDHCPFact() = %q, want empty", got)
	}
}

func TestCurrentNetworkingDataExpandsFreeBSDInterfaceBindings(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"mtu": 1500,
			"mac": "64:5a:ed:ea:5c:81",
			"bindings": []any{
				map[string]any{"address": "192.168.1.2", "netmask": "255.255.255.0", "network": "192.168.1.0"},
			},
		},
		"lo0": map[string]any{
			"mtu": 16384,
			"bindings6": []any{
				map[string]any{"address": "::1", "netmask": "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "network": "::1", "scope6": "host"},
			},
		},
	}

	primary, got := currentNetworkingData("freebsd", interfaces, func(string, ...string) string { return "" })

	if primary != "" {
		t.Fatalf("currentNetworkingData() primary = %q, want empty", primary)
	}
	en0 := got["en0"].(map[string]any)
	for key, want := range map[string]any{
		"ip":      "192.168.1.2",
		"netmask": "255.255.255.0",
		"network": "192.168.1.0",
	} {
		if en0[key] != want {
			t.Fatalf("en0[%s] = %#v, want %#v", key, en0[key], want)
		}
	}
	lo0 := got["lo0"].(map[string]any)
	for key, want := range map[string]any{
		"ip6":      "::1",
		"netmask6": "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		"network6": "::1",
	} {
		if lo0[key] != want {
			t.Fatalf("lo0[%s] = %#v, want %#v", key, lo0[key], want)
		}
	}
}

func TestCurrentNetworkingDataFreeBSDPreservesDHCPForPrimaryInterface(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"em0": map[string]any{
			"dhcp": "192.168.158.6",
			"bindings": []any{
				map[string]any{"address": "192.168.1.2", "netmask": "255.255.255.0", "network": "192.168.1.0"},
			},
		},
	}
	run := func(name string, args ...string) string {
		if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
			return "   route to: default\ninterface: em0\n"
		}
		return ""
	}

	primary, got := currentNetworkingData("freebsd", interfaces, run)

	if primary != "em0" {
		t.Fatalf("primary = %q, want em0", primary)
	}
	if dhcp := networkingDHCPFact(got, "192.168.1.2"); dhcp != "192.168.158.6" {
		t.Fatalf("networkingDHCPFact() = %q, want 192.168.158.6", dhcp)
	}
}

func TestCurrentNetworkingDataUsesBSDAndDarwinRoutePrimaryInterface(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"darwin", "freebsd", "openbsd"} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			interfaces := map[string]any{
				"em0": map[string]any{
					"bindings": []any{
						map[string]any{"address": "192.168.1.2", "netmask": "255.255.255.0", "network": "192.168.1.0"},
					},
				},
			}
			run := func(name string, args ...string) string {
				if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
					return "   route to: default\ninterface: em0\n"
				}
				return ""
			}

			primary, _ := currentNetworkingData(goos, interfaces, run)

			if got, want := primary, "em0"; got != want {
				t.Fatalf("currentNetworkingData(%s) primary = %q, want %q", goos, got, want)
			}
		})
	}
}

func TestCurrentNetworkingDataExpandsOpenBSDInterfaceBindings(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"mtu": 1500,
			"mac": "64:5a:ed:ea:5c:81",
			"bindings": []any{
				map[string]any{"address": "192.168.1.2", "netmask": "255.255.255.0", "network": "192.168.1.0"},
			},
		},
		"lo0": map[string]any{
			"mtu": 16384,
			"bindings6": []any{
				map[string]any{"address": "::1", "netmask": "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "network": "::1"},
			},
		},
	}

	primary, got := currentNetworkingData("openbsd", interfaces, func(string, ...string) string { return "" })

	if primary != "" {
		t.Fatalf("currentNetworkingData() primary = %q, want empty", primary)
	}
	en0 := got["en0"].(map[string]any)
	for key, want := range map[string]any{
		"ip":      "192.168.1.2",
		"netmask": "255.255.255.0",
		"network": "192.168.1.0",
	} {
		if en0[key] != want {
			t.Fatalf("en0[%s] = %#v, want %#v", key, en0[key], want)
		}
	}
	lo0 := got["lo0"].(map[string]any)
	for key, want := range map[string]any{
		"ip6":      "::1",
		"netmask6": "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		"network6": "::1",
	} {
		if lo0[key] != want {
			t.Fatalf("lo0[%s] = %#v, want %#v", key, lo0[key], want)
		}
	}
}

func TestCurrentNetworkingDataAddsOpenBSDDHCPServer(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"em0": map[string]any{
			"mtu": 1500,
			"bindings": []any{
				map[string]any{"address": "192.168.158.20", "netmask": "255.255.255.0", "network": "192.168.158.0"},
			},
		},
		"em1": map[string]any{"mtu": 1500},
	}
	run := func(name string, args ...string) string {
		if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
			return ""
		}
		if name != "dhcpleasectl" || len(args) != 2 || args[0] != "-l" {
			t.Fatalf("run(%q, %#v), want dhcpleasectl -l <interface>", name, args)
		}
		if args[1] == "em0" {
			return "lease 0\n\tdhcp server 192.168.158.6\n"
		}
		return ""
	}

	_, got := currentNetworkingData("openbsd", interfaces, run)

	em0 := got["em0"].(map[string]any)
	if got, want := em0["dhcp"], "192.168.158.6"; got != want {
		t.Fatalf("em0[dhcp] = %#v, want %#v", got, want)
	}
	em1 := got["em1"].(map[string]any)
	if got := em1["dhcp"]; got != nil {
		t.Fatalf("em1[dhcp] = %#v, want nil", got)
	}
}

func TestCurrentNetworkingDataAddsDarwinDHCPServer(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"bindings": []any{map[string]any{"address": "192.168.158.10"}},
		},
		"en1": map[string]any{},
	}
	run := func(name string, args ...string) string {
		if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
			return ""
		}
		if name != "ipconfig" || len(args) != 3 || args[0] != "getoption" || args[2] != "server_identifier" {
			t.Fatalf("run(%q, %#v), want ipconfig getoption <interface> server_identifier", name, args)
		}
		if args[1] == "en0" {
			return "192.168.158.6\n"
		}
		return ""
	}

	_, got := currentNetworkingData("darwin", interfaces, run)
	en0 := got["en0"].(map[string]any)
	if got, want := en0["dhcp"], "192.168.158.6"; got != want {
		t.Fatalf("en0[dhcp] = %#v, want %#v", got, want)
	}
	if got := got["en1"].(map[string]any)["dhcp"]; got != nil {
		t.Fatalf("en1[dhcp] = %#v, want nil", got)
	}
}

func TestCurrentNetworkingDataExpandsDarwinInterfaceBindingsLikeRubyResolver(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"mtu": 1500,
			"mac": "64:5a:ed:ea:5c:81",
			"bindings": []any{
				map[string]any{"address": "192.168.143.212", "netmask": "255.255.255.0", "network": "192.168.143.0"},
			},
		},
		"lo0": map[string]any{
			"mtu": 16384,
			"bindings6": []any{
				map[string]any{"address": "::1", "netmask": "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "network": "::1", "scope6": "host"},
			},
		},
	}

	primary, got := currentNetworkingData("darwin", interfaces, func(string, ...string) string { return "" })

	if primary != "" {
		t.Fatalf("currentNetworkingData() primary = %q, want empty", primary)
	}
	en0 := got["en0"].(map[string]any)
	for key, want := range map[string]any{
		"ip":      "192.168.143.212",
		"netmask": "255.255.255.0",
		"network": "192.168.143.0",
	} {
		if en0[key] != want {
			t.Fatalf("en0[%s] = %#v, want %#v", key, en0[key], want)
		}
	}
	lo0 := got["lo0"].(map[string]any)
	for key, want := range map[string]any{
		"ip6":      "::1",
		"netmask6": "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		"network6": "::1",
		"scope6":   "host",
	} {
		if lo0[key] != want {
			t.Fatalf("lo0[%s] = %#v, want %#v", key, lo0[key], want)
		}
	}
}

func TestCurrentNetworkingDataIgnoresInvalidDarwinDHCPServer(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"bindings": []any{map[string]any{"address": "192.168.158.10"}},
		},
	}
	run := func(name string, args ...string) string {
		if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
			return ""
		}
		if name != "ipconfig" || len(args) != 3 || args[0] != "getoption" || args[1] != "en0" || args[2] != "server_identifier" {
			t.Fatalf("run(%q, %#v), want ipconfig getoption en0 server_identifier", name, args)
		}
		return "invalid output\n"
	}

	_, got := currentNetworkingData("darwin", interfaces, run)
	if got := got["en0"].(map[string]any)["dhcp"]; got != nil {
		t.Fatalf("en0[dhcp] = %#v, want nil", got)
	}
}

func TestLinuxDHCPServerReadsSystemdNetifLease(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "run/systemd/netif/leases/2"), "CLIENT_ID=01:23\nSERVER_ADDRESS=10.16.122.163\n")

	if got, want := linuxDHCPServerFromRoot(testSession, root, "eth0", 2), "10.16.122.163"; got != want {
		t.Fatalf("linuxDHCPServerFromRoot(testSession) = %q, want %q", got, want)
	}
}

func TestLinuxDHCPServerReadsDHClientLeaseForInterface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "var/lib/dhcp/dhclient.eth0.lease"), `lease {
  interface "eth0";
  option dhcp-server-identifier 10.32.10.163;
}`)
	writeFile(t, filepath.Join(root, "var/lib/dhcp/dhclient.en1.lease"), `lease {
  interface "en1";
  option dhcp-server-identifier 10.99.99.99;
}`)

	if got, want := linuxDHCPServerFromRoot(testSession, root, "eth0", 0), "10.32.10.163"; got != want {
		t.Fatalf("linuxDHCPServerFromRoot(testSession) = %q, want %q", got, want)
	}
}

func TestLinuxDHCPServerReadsNetworkManagerInternalLeaseForInterface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "var/lib/NetworkManager/internal-fdgh45-345356fg-dfg-dsfge5er4-sdfghgf45ty-lo.lease"), `# This is private data. Do not parse.
ADDRESS=11.22.36.241
SERVER_ADDRESS=35.32.82.9
`)
	writeFile(t, filepath.Join(root, "var/lib/NetworkManager/internal-fdgh45-345356fg-dfg-dsfge5er4-sdfghgf45ty-eth0.lease"), `SERVER_ADDRESS=10.99.99.99
`)

	if got, want := linuxDHCPServerFromRoot(testSession, root, "lo", 1), "35.32.82.9"; got != want {
		t.Fatalf("linuxDHCPServerFromRoot(testSession) = %q, want %q", got, want)
	}
}

func TestLinuxDHCPServerFallsBackToDHCPCDCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	run := func(name string, args ...string) string {
		if name != "dhcpcd" || !reflect.DeepEqual(args, []string{"-U", "ens160"}) {
			t.Fatalf("run(%q, %#v), want dhcpcd -U ens160", name, args)
		}
		return strings.Join([]string{
			"broadcast_address='10.16.127.255'",
			"dhcp_server_identifier='10.32.22.9'",
			"domain_name='delivery.puppetlabs.net'",
		}, "\n")
	}

	if got, want := linuxDHCPServerFromRootWithRunner(root, "ens160", 1, run), "10.32.22.9"; got != want {
		t.Fatalf("linuxDHCPServerFromRootWithRunner() = %q, want %q", got, want)
	}
}

func TestCoreFacts_networkingIncludesIP6(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	networking, ok := collection["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking fact = %#v, want map", collection["networking"])
	}

	if _, ok := networking["ip6"]; !ok {
		t.Fatalf("networking fact missing ip6 in %#v", networking)
	}
}

func TestCoreFacts_networkingIncludesPrimaryIPv4Binding(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	networking, ok := collection["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking fact = %#v, want map", collection["networking"])
	}

	if networking["ip"] != "" && networking["netmask"] == "" {
		t.Fatalf("networking = %#v, want netmask for primary IPv4", networking)
	}
	if networking["ip"] != "" && networking["network"] == "" {
		t.Fatalf("networking = %#v, want network for primary IPv4", networking)
	}
}

func TestCoreFacts_networkingIncludesPrimaryIPv6Binding(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	networking, ok := collection["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking fact = %#v, want map", collection["networking"])
	}
	if networking["ip6"] == "" {
		t.Skip("host has no primary IPv6 address")
	}

	for _, key := range []string{"netmask6", "network6", "scope6"} {
		if networking[key] == "" {
			t.Fatalf("networking = %#v, want %s for primary IPv6", networking, key)
		}
	}
}

func TestPrimaryIPv6BindingReturnsMatchingBinding(t *testing.T) {
	interfaces := map[string]any{
		"en0": map[string]any{
			"bindings6": []any{
				map[string]any{"address": "2001:db8::10", "netmask": "64", "network": "2001:db8::"},
			},
		},
	}

	got := primaryIPv6Binding(interfaces, "2001:db8::10")
	if got["netmask"] != "64" || got["network"] != "2001:db8::" {
		t.Fatalf("primaryIPv6Binding() = %#v, want matching IPv6 binding", got)
	}
}

func TestPrimaryIPv6ScopeReturnsBindingScope(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"bindings6": []any{
				map[string]any{"address": "fe80::250:56ff:fe9a:8481", "scope6": "link"},
			},
		},
	}

	if got := primaryIPv6Scope(interfaces, "fe80::250:56ff:fe9a:8481"); got != "link" {
		t.Fatalf("primaryIPv6Scope() = %q, want link", got)
	}
}

// Primary IPv6 selection is a deliberate, documented deviation from Ruby
// Facter's first-bound-address rule: routable addresses win over link-locals
// on the primary interface — global scope first, then unique-local, then
// link-local (openspec change fix-darwin-networking-facts).
func TestPrimaryIPv6AddressPrefersRoutableOverLinkLocalUnlikeRubyFirstBound(t *testing.T) {
	t.Parallel()

	interfacesWith := func(addresses ...string) map[string]any {
		bindings := make([]any, 0, len(addresses))
		for _, address := range addresses {
			binding := map[string]any{"address": address}
			if ip := net.ParseIP(address); ip != nil && ip.To4() == nil {
				binding["scope6"] = ipv6Scope(ip)
			}
			bindings = append(bindings, binding)
		}
		iface := map[string]any{"mtu": 1500}
		if len(bindings) > 0 {
			iface["bindings6"] = bindings
		}
		return map[string]any{
			"en0": iface,
			"utun0": map[string]any{
				"bindings6": []any{map[string]any{"address": "2001:db8:ffff::99", "scope6": "global"}},
			},
		}
	}

	tests := []struct {
		name      string
		addresses []string
		want      string
	}{
		{
			name:      "global wins over earlier-bound link-local and unique-local",
			addresses: []string{"fe80::ce7:a5db:2992:ff71", "fd79:4151:e9d2:40de::10", "2001:db8::10"},
			want:      "2001:db8::10",
		},
		{
			name:      "binding order does not change the winner",
			addresses: []string{"2001:db8::10", "fd79:4151:e9d2:40de::10", "fe80::ce7:a5db:2992:ff71"},
			want:      "2001:db8::10",
		},
		{
			name:      "unique-local wins over earlier-bound link-local",
			addresses: []string{"fe80::ce7:a5db:2992:ff71", "fd79:4151:e9d2:40de::10"},
			want:      "fd79:4151:e9d2:40de::10",
		},
		{
			name:      "link-local only reports the link-local",
			addresses: []string{"fe80::ce7:a5db:2992:ff71"},
			want:      "fe80::ce7:a5db:2992:ff71",
		},
		{
			name:      "loopback is not a candidate",
			addresses: []string{"::1"},
			want:      "",
		},
		{
			name:      "no IPv6 bindings",
			addresses: nil,
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			interfaces := interfacesWith(tt.addresses...)
			if got := primaryIPv6Address(interfaces, "en0"); got != tt.want {
				t.Fatalf("primaryIPv6Address() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A primary interface that only carries a link-local IPv6 address still
// reports it, with scope6 "link" — this case matches Ruby Facter.
func TestPrimaryIPv6AddressLinkLocalOnlyReportsLinkScopeLikeRuby(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"bindings6": []any{
				map[string]any{"address": "fe80::ce7:a5db:2992:ff71", "scope6": "link"},
			},
		},
	}

	ipv6 := primaryIPv6Address(interfaces, "en0")
	if ipv6 != "fe80::ce7:a5db:2992:ff71" {
		t.Fatalf("primaryIPv6Address() = %q, want the link-local address", ipv6)
	}
	if got := primaryIPv6Scope(interfaces, ipv6); got != "link" {
		t.Fatalf("primaryIPv6Scope() = %q, want link", got)
	}
}

func TestPrimaryIPv6AddressIgnoresMissingPrimaryInterface(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"bindings6": []any{map[string]any{"address": "2001:db8::10", "scope6": "global"}},
		},
	}

	if got := primaryIPv6Address(interfaces, ""); got != "" {
		t.Fatalf("primaryIPv6Address(no primary) = %q, want empty", got)
	}
	if got := primaryIPv6Address(interfaces, "en1"); got != "" {
		t.Fatalf("primaryIPv6Address(unknown primary) = %q, want empty", got)
	}
}

func TestInterfaceBindingIncludesIPv6Scope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ip   string
		want string
	}{
		{"global", "2001:db8::10", "global"},
		{"link local", "fe80::1", "link"},
		{"loopback", "::1", "host"},
		{"IPv4 compatible", "::192.0.2.128", "compat,global"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interfaceBinding(net.ParseIP(tt.ip), &net.IPNet{Mask: net.CIDRMask(64, 128)})
			if got["scope6"] != tt.want {
				t.Fatalf("interfaceBinding(%s)[scope6] = %#v, want %q", tt.ip, got["scope6"], tt.want)
			}
		})
	}
}

func TestInterfaceBindingIPv6NetmaskMatchesRubyBuildBinding(t *testing.T) {
	t.Parallel()

	ip := net.ParseIP("fe80::dc20:a2b9:5253:9b46")
	got := interfaceBinding(ip, &net.IPNet{IP: ip, Mask: net.CIDRMask(64, 128)})

	if got["netmask"] != "ffff:ffff:ffff:ffff::" {
		t.Fatalf("interfaceBinding()[netmask] = %#v, want Ruby IPv6 mask string", got["netmask"])
	}
}

func TestCoreFacts_networkingIncludesPrimaryMTU(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	networking, ok := collection["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking fact = %#v, want map", collection["networking"])
	}
	interfaces, ok := networking["interfaces"].(map[string]any)
	if !ok || len(interfaces) == 0 {
		t.Fatalf("networking.interfaces = %#v, want discovered interfaces", networking["interfaces"])
	}

	primary, _ := networking["primary"].(string)
	if primary != "" {
		iface, ok := interfaces[primary].(map[string]any)
		if !ok {
			t.Fatalf("networking.interfaces[%s] = %#v, want map", primary, interfaces[primary])
		}
		if networking["mtu"] != iface["mtu"] {
			t.Fatalf("networking.mtu = %#v, want primary interface MTU %#v", networking["mtu"], iface["mtu"])
		}
	}
}

func TestCoreFacts_includeNetworkingInterfaces(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	networking, ok := collection["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking fact = %#v, want map", collection["networking"])
	}
	interfaces, ok := networking["interfaces"].(map[string]any)
	if !ok || len(interfaces) == 0 {
		t.Fatalf("networking.interfaces = %#v, want discovered interfaces", networking["interfaces"])
	}

	for name, raw := range interfaces {
		iface, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("networking.interfaces[%s] = %#v, want map", name, raw)
		}
		if iface["bindings"] != nil || iface["bindings6"] != nil {
			return
		}
	}
	t.Fatalf("networking.interfaces = %#v, want at least one interface with IPv4 or IPv6 bindings", interfaces)
}

func TestNetworkingInterfacesKeepsLinuxLoopbackBindingsLikeRubySocketParser(t *testing.T) {
	_, ipv4, err := net.ParseCIDR("127.0.0.1/8")
	if err != nil {
		t.Fatal(err)
	}
	ipv4.IP = net.ParseIP("127.0.0.1")
	_, ipv6, err := net.ParseCIDR("::1/128")
	if err != nil {
		t.Fatal(err)
	}
	ipv6.IP = net.ParseIP("::1")

	got := networkingInterfacesFromSnapshots([]networkInterfaceSnapshot{
		{
			Interface: net.Interface{
				Name:  "lo",
				MTU:   65536,
				Flags: net.FlagUp,
			},
			Addrs: []net.Addr{ipv4, ipv6},
		},
	}, "linux")
	lo, ok := got["lo"].(map[string]any)
	if !ok {
		t.Fatalf("networking.interfaces[lo] = %#v, want map", got["lo"])
	}

	wantBindings := []any{map[string]any{
		"address": "127.0.0.1",
		"netmask": "255.0.0.0",
		"network": "127.0.0.0",
	}}
	if !reflect.DeepEqual(lo["bindings"], wantBindings) {
		t.Fatalf("lo.bindings = %#v, want %#v", lo["bindings"], wantBindings)
	}
	wantBindings6 := []any{map[string]any{
		"address": "::1",
		"netmask": "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		"network": "::1",
		"scope6":  "host",
	}}
	if !reflect.DeepEqual(lo["bindings6"], wantBindings6) {
		t.Fatalf("lo.bindings6 = %#v, want %#v", lo["bindings6"], wantBindings6)
	}
}

func TestInterfaceBindingNetworkAddress(t *testing.T) {
	_, cidr, err := net.ParseCIDR("10.16.119.155/20")
	if err != nil {
		t.Fatal(err)
	}
	cidr.IP = net.ParseIP("10.16.119.155")

	got := interfaceBinding(cidr.IP, cidr)
	want := map[string]any{"address": "10.16.119.155", "netmask": "255.255.240.0", "network": "10.16.112.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interfaceBinding() = %#v, want %#v", got, want)
	}
}

func TestPrimaryIPv4BindingFindsNetmaskAndNetwork(t *testing.T) {
	interfaces := map[string]any{
		"lo": map[string]any{
			"bindings": []any{map[string]any{"address": "127.0.0.1", "netmask": "255.0.0.0", "network": "127.0.0.0"}},
		},
		"eth0": map[string]any{
			"bindings": []any{map[string]any{"address": "10.16.119.155", "netmask": "255.255.240.0", "network": "10.16.112.0"}},
		},
	}

	got := primaryIPv4Binding(interfaces, "10.16.119.155")
	want := map[string]any{"address": "10.16.119.155", "netmask": "255.255.240.0", "network": "10.16.112.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("primaryIPv4Binding() = %#v, want %#v", got, want)
	}
}

func TestPrimaryInterfaceFactFindsMAC(t *testing.T) {
	interfaces := map[string]any{
		"eth0": map[string]any{"mac": "00:50:56:9a:61:46"},
		"eth1": map[string]any{"mac": "00:50:56:9a:61:47"},
	}

	got := primaryInterfaceFact(interfaces, "eth1", "mac")
	if got != "00:50:56:9a:61:47" {
		t.Fatalf("primaryInterfaceFact() = %#v, want eth1 MAC", got)
	}
}

func TestLinuxPrimaryInterfaceFromProcRouteMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fixture string
		want    string
	}{
		{name: "default route", fixture: "proc_net_route", want: "ens160"},
		{name: "empty", fixture: "proc_net_route_empty", want: ""},
		{name: "blackhole default", fixture: "proc_net_route_blackhole", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input, err := os.ReadFile(filepath.Join("testdata", tt.fixture))
			if err != nil {
				t.Fatal(err)
			}

			if got := linuxPrimaryInterfaceFromProcRoute(string(input)); got != tt.want {
				t.Fatalf("linuxPrimaryInterfaceFromProcRoute() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentNetworkingDataLinuxPrimaryInterfaceFallsBackLikeRubyResolver(t *testing.T) {
	interfaces := map[string]any{
		"lo": map[string]any{
			"bindings": []any{map[string]any{"address": "127.0.0.1"}},
		},
		"ens160": map[string]any{
			"bindings": []any{map[string]any{"address": "10.16.119.155"}},
		},
	}
	run := func(name string, args ...string) string {
		if name != "ip" || !reflect.DeepEqual(args, []string{"route", "show", "default"}) {
			t.Fatalf("run(%q, %#v), want ip route show default", name, args)
		}
		return "default via 10.16.112.1 dev lo proto dhcp src 10.16.119.155 metric 100\n"
	}

	primary := linuxPrimaryInterface("", interfaces, run)
	if primary != "lo" {
		t.Fatalf("primary = %q, want ip route fallback interface lo", primary)
	}
}

func TestCurrentNetworkingDataLinuxPrimaryInterfaceFallsBackToFirstNonIgnoredBinding(t *testing.T) {
	interfaces := map[string]any{
		"lo": map[string]any{
			"bindings": []any{map[string]any{"address": "127.0.0.1"}},
		},
		"ens160": map[string]any{
			"bindings": []any{map[string]any{"address": "10.16.119.155"}},
		},
	}

	primary := linuxPrimaryInterface("", interfaces, func(string, ...string) string { return "" })
	if primary != "ens160" {
		t.Fatalf("primary = %q, want first non-ignored binding ens160", primary)
	}
}

func TestDomainFromFQDN(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		fqdn     string
		want     string
	}{
		{name: "fqdn suffix", hostname: "node", fqdn: "node.example.test", want: "example.test"},
		{name: "hostname already fqdn", hostname: "node.example.test", fqdn: "node.example.test", want: "example.test"},
		{name: "short hostname", hostname: "node", fqdn: "node", want: ""},
		{name: "empty fqdn", hostname: "node", fqdn: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domainFromFQDN(tt.hostname, tt.fqdn); got != tt.want {
				t.Fatalf("domainFromFQDN(%q, %q) = %q, want %q", tt.hostname, tt.fqdn, got, tt.want)
			}
		})
	}
}

func TestDomainFromResolvConfMatchesRubyLinuxHostnameResolver(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "domain", input: "domain bar\n", want: "bar"},
		{name: "domain before search", input: "search baz\ndomain bar\n", want: "bar"},
		{name: "search", input: "search foo.bar example.com\n", want: "foo.bar"},
		{name: "empty", input: "", want: ""},
		{name: "root search", input: "search .\n", want: ""},
		{name: "root search with later entries", input: "search . foo.bar\n", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domainFromResolvConf(tt.input); got != tt.want {
				t.Fatalf("domainFromResolvConf(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLinuxFQDNFallsBackToResolvConfDomain(t *testing.T) {
	tests := []struct {
		name       string
		hostname   string
		resolved   string
		resolvConf string
		wantFQDN   string
		wantDomain string
	}{
		{name: "short hostname uses domain", hostname: "foo", resolved: "foo", resolvConf: "domain bar\n", wantFQDN: "foo.bar", wantDomain: "bar"},
		{name: "short hostname uses search", hostname: "foo", resolved: "foo", resolvConf: "search foo.bar example.com\n", wantFQDN: "foo.foo.bar", wantDomain: "foo.bar"},
		{name: "empty resolv conf keeps short hostname", hostname: "foo", resolved: "foo", resolvConf: "", wantFQDN: "foo", wantDomain: ""},
		{name: "resolved fqdn wins", hostname: "foo", resolved: "foo.lookup", resolvConf: "domain bar\n", wantFQDN: "foo.lookup", wantDomain: "lookup"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFQDN, gotDomain := linuxFQDNAndDomain(tt.hostname, tt.resolved, tt.resolvConf)
			if gotFQDN != tt.wantFQDN || gotDomain != tt.wantDomain {
				t.Fatalf("linuxFQDNAndDomain() = %q, %q; want %q, %q", gotFQDN, gotDomain, tt.wantFQDN, tt.wantDomain)
			}
		})
	}
}

func TestDarwinFQDNAndDomainFallsBackToResolvConfDomainLikeRubyHostnameResolver(t *testing.T) {
	dir := t.TempDir()
	resolvConfPath := filepath.Join(dir, "resolv.conf")
	if err := os.WriteFile(resolvConfPath, []byte("nameserver 10.10.0.10\nsearch baz\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	gotFQDN, gotDomain := currentHostnameFQDNAndDomain("darwin", "foo", "foo", resolvConfPath, os.ReadFile)
	if gotFQDN != "foo.baz" || gotDomain != "baz" {
		t.Fatalf("currentHostnameFQDNAndDomain(darwin) = %q, %q; want foo.baz, baz", gotFQDN, gotDomain)
	}
}

func TestCurrentHostnameFactsSplitNodeNameLikeRubyResolver(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing-resolv.conf")
	domainConf := writeHostnameTestResolvConf(t, "nameserver 10.10.0.10\ndomain bar\n")
	searchConf := writeHostnameTestResolvConf(t, "nameserver 10.10.0.10\nsearch baz.example\n")

	tests := []struct {
		name           string
		goos           string
		nodeName       string
		resolvedFQDN   string
		resolvConfPath string
		wantHostname   string
		wantFQDN       string
		wantDomain     string
	}{
		{
			name:           "darwin dotted node name",
			goos:           "darwin",
			nodeName:       "dream-factory.lan",
			resolvedFQDN:   "dream-factory.lan",
			resolvConfPath: missing,
			wantHostname:   "dream-factory",
			wantFQDN:       "dream-factory.lan",
			wantDomain:     "lan",
		},
		{
			name:           "linux dotted node name",
			goos:           "linux",
			nodeName:       "node.example.test",
			resolvedFQDN:   "node.example.test",
			resolvConfPath: missing,
			wantHostname:   "node",
			wantFQDN:       "node.example.test",
			wantDomain:     "example.test",
		},
		{
			name:           "freebsd dotted node name",
			goos:           "freebsd",
			nodeName:       "node.example.test",
			resolvedFQDN:   "node.example.test",
			resolvConfPath: missing,
			wantHostname:   "node",
			wantFQDN:       "node.example.test",
			wantDomain:     "example.test",
		},
		{
			name:           "windows dotted node name",
			goos:           "windows",
			nodeName:       "node.example.test",
			resolvedFQDN:   "node.example.test",
			resolvConfPath: missing,
			wantHostname:   "node",
			wantFQDN:       "node.example.test",
			wantDomain:     "example.test",
		},
		{
			name:           "darwin undotted node name with resolver domain",
			goos:           "darwin",
			nodeName:       "foo",
			resolvedFQDN:   "foo",
			resolvConfPath: domainConf,
			wantHostname:   "foo",
			wantFQDN:       "foo.bar",
			wantDomain:     "bar",
		},
		{
			name:           "linux undotted node name with resolver search",
			goos:           "linux",
			nodeName:       "foo",
			resolvedFQDN:   "foo",
			resolvConfPath: searchConf,
			wantHostname:   "foo",
			wantFQDN:       "foo.baz.example",
			wantDomain:     "baz.example",
		},
		{
			name:           "darwin undotted node name with no domain",
			goos:           "darwin",
			nodeName:       "foo",
			resolvedFQDN:   "foo",
			resolvConfPath: missing,
			wantHostname:   "foo",
			wantFQDN:       "foo",
			wantDomain:     "",
		},
		{
			name:           "freebsd undotted node name with no domain",
			goos:           "freebsd",
			nodeName:       "foo",
			resolvedFQDN:   "foo",
			resolvConfPath: missing,
			wantHostname:   "foo",
			wantFQDN:       "foo",
			wantDomain:     "",
		},
		{
			name:           "freebsd undotted node name with reverse-resolved fqdn",
			goos:           "freebsd",
			nodeName:       "foo",
			resolvedFQDN:   "foo.lookup",
			resolvConfPath: missing,
			wantHostname:   "foo",
			wantFQDN:       "foo.lookup",
			wantDomain:     "lookup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hostname, fqdn, domain := currentHostnameFacts(tt.goos, tt.nodeName, tt.resolvedFQDN, tt.resolvConfPath)
			if hostname != tt.wantHostname || fqdn != tt.wantFQDN || domain != tt.wantDomain {
				t.Fatalf("currentHostnameFacts(%s, %q) = %q, %q, %q; want %q, %q, %q",
					tt.goos, tt.nodeName, hostname, fqdn, domain, tt.wantHostname, tt.wantFQDN, tt.wantDomain)
			}
		})
	}
}

func writeHostnameTestResolvConf(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resolv.conf")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHostnameFactValuesExposeNilDomainForShortLinuxHostnameLikeRubyResolver(t *testing.T) {
	fqdnValue, domainValue := hostnameFactValues("foo", "foo", "")

	if fqdnValue != "foo" {
		t.Fatalf("fqdn fact value = %#v, want short hostname", fqdnValue)
	}
	if domainValue != nil {
		t.Fatalf("domain fact value = %#v, want nil", domainValue)
	}
}

func TestHostNameFromLookupReturnsNilValueWhenLookupFailsLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	logger := captureLogger(&debugMessages, nil, nil)

	hostname, value := hostNameFromLookup(func() (string, error) {
		return "", errors.New("hostname unavailable")
	}, logger)

	if hostname != "" {
		t.Fatalf("hostname = %q, want empty internal fallback", hostname)
	}
	if value != nil {
		t.Fatalf("hostname fact value = %#v, want nil", value)
	}
	want := []string{"Socket.gethostname failed to return hostname"}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestLinuxHostNameFromLookupsFallsBackWhenPrimaryLookupIsEmptyLikeRubyResolver(t *testing.T) {
	hostname, value := linuxHostNameFromLookups(
		func() (string, error) { return "", nil },
		func() string { return "kernel-host" },
		discardLog(),
	)

	if hostname != "kernel-host" {
		t.Fatalf("hostname = %q, want kernel-host", hostname)
	}
	if value != "kernel-host" {
		t.Fatalf("hostname fact value = %#v, want kernel-host", value)
	}
}

func TestLinuxHostNameFromLookupsFallsBackWhenPrimaryLookupReturnsZeroAddressLikeRubyResolver(t *testing.T) {
	hostname, value := linuxHostNameFromLookups(
		func() (string, error) { return "0.0.0.0", nil },
		func() string { return "kernel-host" },
		discardLog(),
	)

	if hostname != "kernel-host" {
		t.Fatalf("hostname = %q, want kernel-host", hostname)
	}
	if value != "kernel-host" {
		t.Fatalf("hostname fact value = %#v, want kernel-host", value)
	}
}
