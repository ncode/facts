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

func TestOptionalNetworkingStringOmitsEmptyValues(t *testing.T) {
	t.Parallel()

	collection := Collection([]ResolvedFact{
		{Name: "networking.ip", Value: optionalNetworkingString("192.0.2.10")},
		{Name: "networking.ip6", Value: optionalNetworkingString("")},
		{Name: "networking.netmask6", Value: optionalNetworkingString("")},
		{Name: "networking.network6", Value: optionalNetworkingString("")},
		{Name: "networking.scope6", Value: optionalNetworkingString("")},
	})
	networking, ok := collection["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking fact = %#v, want map", collection["networking"])
	}
	if got := networking["ip"]; got != "192.0.2.10" {
		t.Fatalf("networking.ip = %#v, want 192.0.2.10", got)
	}
	for _, key := range []string{"ip6", "netmask6", "network6", "scope6"} {
		if _, ok := networking[key]; ok {
			t.Fatalf("networking.%s present in %#v, want omitted", key, networking)
		}
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

func TestNetworkingInterfacesForPlatformPlan9UsesSessionGlob(t *testing.T) {
	t.Parallel()

	s := NewSession()
	s.host = &fakeHostOS{
		files: map[string][]byte{
			"/net/ipifc/0/status": []byte(plan9Fixture(t, "ipifc_status")),
			"/net/ether0/addr":    []byte(plan9Fixture(t, "ether0_addr")),
		},
		globs: map[string][]string{
			"/net/ipifc/*/status": {"/net/ipifc/0/status"},
		},
	}
	calledSnapshotProvider := false

	got := networkingInterfacesForPlatform(s, "plan9", func() ([]networkInterfaceSnapshot, error) {
		calledSnapshotProvider = true
		return nil, nil
	})
	if calledSnapshotProvider {
		t.Fatal("networkingInterfacesForPlatform(plan9) called snapshot provider")
	}
	if _, ok := got["ether0"]; !ok {
		t.Fatalf("networkingInterfacesForPlatform(plan9) = %#v, want ether0", got)
	}
}

func TestNetworkingInterfacesForPlatformLinuxAddsSessionMetadata(t *testing.T) {
	t.Parallel()

	_, ipv4, err := net.ParseCIDR("10.16.122.20/24")
	if err != nil {
		t.Fatal(err)
	}
	ipv4.IP = net.ParseIP("10.16.122.20")
	_, ipv6, err := net.ParseCIDR("fe80::250:56ff:fe9a:8481/64")
	if err != nil {
		t.Fatal(err)
	}
	ipv6.IP = net.ParseIP("fe80::250:56ff:fe9a:8481")

	s := NewSession()
	s.host = &fakeHostOS{
		platform: "linux",
		files: map[string][]byte{
			"/run/systemd/netif/leases/7":   []byte("SERVER_ADDRESS=10.16.122.163\n"),
			"/proc/net/if_inet6":            []byte("fe80000000000000025056fffe9a8481 02 40 20 80 eth0\n"),
			"/sys/class/net/eth0/operstate": []byte("up\n"),
			"/sys/class/net/eth0/speed":     []byte("1000\n"),
			"/sys/class/net/eth0/duplex":    []byte("full\n"),
		},
		stats: map[string]os.FileInfo{
			"/sys/class/net/eth0/device": fakeFileInfo{name: "device"},
		},
		runOutputs: map[string]string{
			fakeRunKey("ip", "route", "show"):       "default via 10.16.122.1 dev eth0 src 10.16.122.21\n",
			fakeRunKey("ip", "-6", "route", "show"): "default via fe80::1 dev eth0 src 2001:db8::10 metric 1024\n",
		},
	}

	got := networkingInterfacesForPlatform(s, "linux", func() ([]networkInterfaceSnapshot, error) {
		return []networkInterfaceSnapshot{{
			Interface: net.Interface{
				Name:         "eth0",
				Index:        7,
				MTU:          1500,
				Flags:        net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
				HardwareAddr: net.HardwareAddr{0x00, 0x50, 0x56, 0x9a, 0xf8, 0x6b},
			},
			Addrs: []net.Addr{ipv4, ipv6},
		}}, nil
	})

	eth0, ok := got["eth0"].(map[string]any)
	if !ok {
		t.Fatalf("eth0 = %#v, want map", got["eth0"])
	}
	for key, want := range map[string]any{
		"dhcp":              "10.16.122.163",
		"operational_state": "up",
		"physical":          true,
		"speed":             1000,
		"duplex":            "full",
	} {
		if got := eth0[key]; got != want {
			t.Fatalf("eth0.%s = %#v, want %#v", key, got, want)
		}
	}
	bindings, _ := eth0["bindings"].([]any)
	if !bindingsContainAddress(bindings, "10.16.122.21") {
		t.Fatalf("eth0.bindings = %#v, want route source address", eth0["bindings"])
	}
	bindings6, _ := eth0["bindings6"].([]any)
	if !bindingsContainAddress(bindings6, "2001:db8::10") {
		t.Fatalf("eth0.bindings6 = %#v, want IPv6 route source address", eth0["bindings6"])
	}
	for _, raw := range bindings6 {
		binding, _ := raw.(map[string]any)
		if binding["address"] == "fe80::250:56ff:fe9a:8481" {
			if got, want := binding["flags"], []string{"permanent"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("link-local IPv6 flags = %#v, want %#v", got, want)
			}
			return
		}
	}
	t.Fatalf("eth0.bindings6 = %#v, want link-local binding with flags", eth0["bindings6"])
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

func TestNetworkingInterfacesOmitsZeroMTUFromAddresslessPOSIXInterfaces(t *testing.T) {
	t.Parallel()

	got := networkingInterfacesFromSnapshots([]networkInterfaceSnapshot{
		{
			Interface: net.Interface{Name: "enc0"},
		},
		{
			Interface: net.Interface{Name: "gif0", MTU: 1280},
		},
	}, "openbsd")

	if got := got["enc0"].(map[string]any)["mtu"]; got != nil {
		t.Fatalf("enc0 mtu = %#v, want omitted", got)
	}
	if got := got["gif0"].(map[string]any)["mtu"]; got != 1280 {
		t.Fatalf("gif0 mtu = %#v, want 1280", got)
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
		return []string{"container=podman\x00bubbles=\x00HOME=/root\x00"}, true
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

func TestAddLinuxRouteSourceBindingsUsesIPv4AndIPv6Routes(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{runOutputs: map[string]string{
		fakeRunKey("ip", "route", "show"):       "default via 10.16.112.1 dev ens192 src 10.16.125.217\n",
		fakeRunKey("ip", "-6", "route", "show"): "2001:db8::/64 dev ens192 src 2001:db8::20\n",
	}}
	s := NewSession()
	s.host = host
	interfaces := map[string]any{
		"ens192": map[string]any{
			"bindings":  []any{map[string]any{"address": "10.16.112.10"}},
			"bindings6": []any{map[string]any{"address": "2001:db8::10"}},
		},
	}

	addLinuxRouteSourceBindings(s, interfaces)

	if got := interfaces["ens192"].(map[string]any)["bindings"]; !reflect.DeepEqual(got, []any{
		map[string]any{"address": "10.16.112.10"},
		map[string]any{"address": "10.16.125.217"},
	}) {
		t.Fatalf("bindings = %#v", got)
	}
	if got := interfaces["ens192"].(map[string]any)["bindings6"]; !reflect.DeepEqual(got, []any{
		map[string]any{"address": "2001:db8::10"},
		map[string]any{"address": "2001:db8::20"},
	}) {
		t.Fatalf("bindings6 = %#v", got)
	}
}

func TestWindowsFQDNCombinesHostnameAndDomainLikeRubyResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		hostname string
		domain   string
		want     string
	}{
		{name: "empty hostname", domain: "example.test", want: ""},
		{name: "no domain", hostname: "host", want: "host"},
		{name: "already fqdn", hostname: "host.example.test", domain: "ignored.test", want: "host.example.test"},
		{name: "short hostname and domain", hostname: "host", domain: "example.test", want: "host.example.test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := windowsFQDN(tt.hostname, tt.domain); got != tt.want {
				t.Fatalf("windowsFQDN(%q, %q) = %q, want %q", tt.hostname, tt.domain, got, tt.want)
			}
		})
	}
}

func TestWindowsIPConfigAdapterNameParsesOnlyAdapterHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		header string
		want   string
	}{
		{header: "Ethernet adapter Ethernet0:", want: "Ethernet0"},
		{header: "Wireless LAN adapter Wi-Fi:", want: "Wi-Fi"},
		{header: "Tunnel adapter isatap.example.test:", want: "isatap.example.test"},
		{header: "Connection-specific DNS Suffix  . : example.test", want: ""},
		{header: "Ethernet:", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			if got := windowsIPConfigAdapterName(tt.header); got != tt.want {
				t.Fatalf("windowsIPConfigAdapterName(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestParseWindowsRegistryStringValue(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`,
		"    Domain    REG_SZ    example.test",
		"    SearchList    REG_SZ",
		"    Hostname    REG_DWORD    0x1",
	}, "\n")

	tests := []struct {
		name   string
		key    string
		want   string
		wantOK bool
	}{
		{name: "value present", key: "Domain", want: "example.test", wantOK: true},
		{name: "empty REG_SZ value is present", key: "SearchList", want: "", wantOK: true},
		{name: "wrong registry type", key: "Hostname", want: "", wantOK: false},
		{name: "missing key", key: "DhcpDomain", want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseWindowsRegistryStringValue(input, tt.key)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("parseWindowsRegistryStringValue(%q) = %q, %v; want %q, %v", tt.key, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestIgnoredIPAddressFiltersNonRoutableAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		address string
		want    bool
	}{
		{address: "", want: true},
		{address: "not-an-ip", want: true},
		{address: "127.0.0.1", want: true},
		{address: "::1", want: true},
		{address: "169.254.10.20", want: true},
		{address: "fe80::1", want: true},
		{address: "fc00::1", want: true},
		{address: "192.0.2.10", want: false},
		{address: "2001:db8::1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			if got := ignoredIPAddress(tt.address); got != tt.want {
				t.Fatalf("ignoredIPAddress(%q) = %v, want %v", tt.address, got, tt.want)
			}
		})
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

func TestParseLinuxIfInet6FlagsSkipsMalformedRows(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"too few fields",
		"not-32-hex 02 40 20 80 eth0",
		"20010db8000000000000000000000001 02 40 20 not-hex eth0",
		"20010db8000000000000000000000002 02 40 20 00 eth0",
	}, "\n")
	if got := parseLinuxIfInet6Flags(input); len(got) != 0 {
		t.Fatalf("parseLinuxIfInet6Flags(malformed) = %#v, want empty", got)
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

func TestAddLinuxIfInet6FlagsIgnoresMalformedInterfacesAndBindings(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"not-map":       "ignored",
		"no-bindings":   map[string]any{"mac": "00:00:5e:00:53:01"},
		"bad-bindings":  map[string]any{"bindings6": "ignored"},
		"mixed-binding": map[string]any{"bindings6": []any{"ignored", map[string]any{}, map[string]any{"address": "2001:db8::10"}}},
	}
	flags := map[string]map[string][]string{
		"missing":       {"2001:db8::10": {"permanent"}},
		"not-map":       {"2001:db8::10": {"permanent"}},
		"no-bindings":   {"2001:db8::10": {"permanent"}},
		"bad-bindings":  {"2001:db8::10": {"permanent"}},
		"mixed-binding": {"2001:db8::20": {"temporary"}},
	}

	addLinuxIfInet6Flags(interfaces, flags)

	bindings := interfaces["mixed-binding"].(map[string]any)["bindings6"].([]any)
	for _, raw := range bindings {
		binding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := binding["flags"]; ok {
			t.Fatalf("addLinuxIfInet6Flags() added flags to malformed or non-matching binding %#v", binding)
		}
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

func TestNetworkingDHCPValueKeepsNetBSDAbsentWithoutSource(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"vioif0": map[string]any{"bindings": []any{map[string]any{"address": "10.0.0.12"}}},
	}

	if got := networkingDHCPValue("netbsd", interfaces, "10.0.0.12"); got != nil {
		t.Fatalf("networkingDHCPValue(netbsd) = %#v, want nil", got)
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

	for _, goos := range []string{"darwin", "freebsd", "netbsd", "openbsd"} {
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

func TestCurrentNetworkingDataOpenBSDSummaryPrefersRoutableIPv6Binding(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"vio0": map[string]any{
			"bindings6": []any{
				map[string]any{
					"address": "fe80::5054:ff:fe12:3422",
					"netmask": "ffff:ffff:ffff:ffff::",
					"network": "fe80::",
					"scope6":  "link",
				},
				map[string]any{
					"address": "fec0::533d:e3fd:7582:e611",
					"netmask": "ffff:ffff:ffff:ffff::",
					"network": "fec0::",
					"scope6":  "site",
				},
			},
		},
	}

	_, got := currentNetworkingData("openbsd", interfaces, func(string, ...string) string { return "" })

	vio0 := got["vio0"].(map[string]any)
	for key, want := range map[string]any{
		"ip6":      "fec0::533d:e3fd:7582:e611",
		"netmask6": "ffff:ffff:ffff:ffff::",
		"network6": "fec0::",
		"scope6":   "site",
	} {
		if vio0[key] != want {
			t.Fatalf("vio0[%s] = %#v, want %#v", key, vio0[key], want)
		}
	}
}

func TestCurrentNetworkingDataOpenBSDSummaryKeepsHostIPv6Binding(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"lo0": map[string]any{
			"bindings6": []any{
				map[string]any{
					"address": "::1",
					"netmask": "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
					"network": "::1",
					"scope6":  "host",
				},
				map[string]any{
					"address": "fe80::1",
					"netmask": "ffff:ffff:ffff:ffff::",
					"network": "fe80::",
					"scope6":  "link",
				},
			},
		},
	}

	_, got := currentNetworkingData("openbsd", interfaces, func(string, ...string) string { return "" })

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
		if name == "ifconfig" {
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

func TestCurrentNetworkingDataAddsBSDOperationalState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos     string
		ifconfig string
	}{
		{
			goos: "freebsd",
			ifconfig: `em0: flags=1008843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST,LOWER_UP> metric 0 mtu 1500
	status: active
`,
		},
		{
			goos: "openbsd",
			ifconfig: `em0: flags=808843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST,AUTOCONF4> mtu 1500
	status: active
`,
		},
		{
			goos: "netbsd",
			ifconfig: `vioif0: flags=0x8843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> mtu 1500
	status: active
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()

			ifaceName := "em0"
			if tt.goos == "netbsd" {
				ifaceName = "vioif0"
			}
			interfaces := map[string]any{ifaceName: map[string]any{"mtu": 1500}}
			run := func(name string, args ...string) string {
				if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
					return ""
				}
				if name == "dhcpleasectl" {
					return ""
				}
				if name == "ifconfig" && reflect.DeepEqual(args, []string{ifaceName}) {
					return tt.ifconfig
				}
				if name == "ifconfig" && reflect.DeepEqual(args, []string{"-m", ifaceName}) {
					return ""
				}
				t.Fatalf("unexpected command %q %#v", name, args)
				return ""
			}

			_, got := currentNetworkingData(tt.goos, interfaces, run)
			iface := got[ifaceName].(map[string]any)
			if got, want := iface["operational_state"], "active"; got != want {
				t.Fatalf("%s operational_state = %#v, want %#v", ifaceName, got, want)
			}
		})
	}
}

func TestCurrentNetworkingDataAddsFreeBSDSpeedDuplex(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{"em0": map[string]any{"mtu": 1500}}
	run := func(name string, args ...string) string {
		if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
			return ""
		}
		if name == "ifconfig" && reflect.DeepEqual(args, []string{"em0"}) {
			return "em0: flags=8843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> mtu 1500\n\tstatus: active\n"
		}
		if name == "ifconfig" && reflect.DeepEqual(args, []string{"-m", "em0"}) {
			return "em0: flags=8843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> mtu 1500\n\tmedia: Ethernet autoselect (1000baseT <full-duplex>)\n\tstatus: active\n"
		}
		t.Fatalf("unexpected command %q %#v", name, args)
		return ""
	}

	_, got := currentNetworkingData("freebsd", interfaces, run)
	em0 := got["em0"].(map[string]any)
	if em0["speed"] != 1000 {
		t.Fatalf("em0 speed = %#v, want 1000", em0["speed"])
	}
	if em0["duplex"] != "full" {
		t.Fatalf("em0 duplex = %#v, want full", em0["duplex"])
	}
}

func TestBSDMediaSpeedParsesCommonFreeBSDMediaTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		token string
		want  int
	}{
		{"10baseT/UTP", 10},
		{"100baseTX", 100},
		{"1000baseT", 1000},
		{"2500baseT", 2500},
		{"2.5Gbase-T", 2500},
		{"5000baseT", 5000},
		{"5Gbase-T", 5000},
		{"10Gbase-T", 10000},
		{"25GBase-SR", 25000},
		{"40GBase-CR4", 40000},
		{"100GBase-SR4", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			t.Parallel()

			if got := bsdMediaSpeed(tt.token); got != tt.want {
				t.Fatalf("bsdMediaSpeed(%q) = %d, want %d", tt.token, got, tt.want)
			}
		})
	}
}

func TestCurrentNetworkingDataAddsFreeBSDDHCPServer(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"em0": map[string]any{
			"bindings": []any{map[string]any{"address": "192.168.1.2", "netmask": "255.255.255.0", "network": "192.168.1.0"}},
		},
	}
	run := func(name string, args ...string) string {
		if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
			return "interface: em0\n"
		}
		if name == "ifconfig" {
			return ""
		}
		t.Fatalf("unexpected command %q %#v", name, args)
		return ""
	}
	readFile := func(path string) ([]byte, error) {
		if path != "/var/db/dhclient.leases.em0" {
			return nil, os.ErrNotExist
		}
		return []byte("lease {\n  option dhcp-server-identifier 192.168.1.1;\n}\n"), nil
	}

	primary, got := currentNetworkingData("freebsd", interfaces, run, readFile)
	if primary != "em0" {
		t.Fatalf("primary = %q, want em0", primary)
	}
	em0 := got["em0"].(map[string]any)
	if em0["dhcp"] != "192.168.1.1" {
		t.Fatalf("em0 dhcp = %#v, want 192.168.1.1", em0["dhcp"])
	}
	if dhcp := networkingDHCPFact(got, "192.168.1.2"); dhcp != "192.168.1.1" {
		t.Fatalf("networkingDHCPFact() = %q, want 192.168.1.1", dhcp)
	}
}

func TestCurrentNetworkingDataKeepsNetBSDDHCPAbsent(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{"vioif0": map[string]any{"bindings": []any{map[string]any{"address": "192.168.1.2"}}}}
	run := func(name string, args ...string) string {
		if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
			return "interface: vioif0\n"
		}
		if name == "ifconfig" {
			return "vioif0: flags=8843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> mtu 1500\n\tstatus: active\n"
		}
		t.Fatalf("unexpected command %q %#v", name, args)
		return ""
	}

	_, got := currentNetworkingData("netbsd", interfaces, run, func(string) ([]byte, error) {
		return []byte("option dhcp-server-identifier 192.168.1.1;\n"), nil
	})
	if dhcp := got["vioif0"].(map[string]any)["dhcp"]; dhcp != nil {
		t.Fatalf("vioif0 dhcp = %#v, want absent", dhcp)
	}
}

const dragonFlyIfconfigFixture = `vtnet0: flags=8843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> metric 0 mtu 1500
	options=2a<TXCSUM,VLAN_MTU,JUMBO_MTU>
	ether 52:54:00:12:34:56
	inet6 fe80::5054:ff:fe12:3456%vtnet0 prefixlen 64 scopeid 0x1
	inet 10.0.2.15 netmask 0xffffff00 broadcast 10.0.2.255
	media: Ethernet 1000baseT <full-duplex>
	status: active
lo0: flags=8049<UP,LOOPBACK,RUNNING,MULTICAST> metric 0 mtu 16384
	options=43<RXCSUM,TXCSUM,RSS>
	inet 127.0.0.1 netmask 0xff000000
	inet6 ::1 prefixlen 128
	inet6 fe80::1%lo0 prefixlen 64 scopeid 0x2
	groups: lo
`

func TestDragonFlyIPv4MaskParsesHexAndDottedMasks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  string
	}{
		{value: "", want: ""},
		{value: "0xffffff00", want: "255.255.255.0"},
		{value: "255.255.0.0", want: "255.255.0.0"},
		{value: "not-a-mask", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := netmaskString(dragonFlyIPv4Mask(tt.value))
			if got != tt.want {
				t.Fatalf("dragonFlyIPv4Mask(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestDragonFlyIPv4BindingBuildsNetworkFromOptionalMask(t *testing.T) {
	t.Parallel()

	binding, ok := dragonFlyIPv4Binding([]string{"inet", "10.0.2.15", "netmask", "0xffffff00", "broadcast", "10.0.2.255"})
	if !ok {
		t.Fatal("dragonFlyIPv4Binding(valid) ok = false, want true")
	}
	if binding["address"] != "10.0.2.15" || binding["netmask"] != "255.255.255.0" || binding["network"] != "10.0.2.0" {
		t.Fatalf("dragonFlyIPv4Binding(valid) = %#v", binding)
	}

	binding, ok = dragonFlyIPv4Binding([]string{"inet", "10.0.2.15"})
	if !ok {
		t.Fatal("dragonFlyIPv4Binding(no mask) ok = false, want true")
	}
	if _, exists := binding["netmask"]; exists {
		t.Fatalf("dragonFlyIPv4Binding(no mask) = %#v, want no netmask", binding)
	}
	if _, exists := binding["network"]; exists {
		t.Fatalf("dragonFlyIPv4Binding(no mask) = %#v, want no network", binding)
	}
	if binding, ok := dragonFlyIPv4Binding([]string{"inet", "not-an-ip"}); ok || binding != nil {
		t.Fatalf("dragonFlyIPv4Binding(invalid) = %#v, %v; want nil, false", binding, ok)
	}
}

func TestDragonFlyIPv6BindingBuildsNetworkFromOptionalPrefix(t *testing.T) {
	t.Parallel()

	binding, ok := dragonFlyIPv6Binding([]string{"inet6", "fe80::1%vtnet0", "prefixlen", "64"})
	if !ok {
		t.Fatal("dragonFlyIPv6Binding(valid) ok = false, want true")
	}
	for key, want := range map[string]any{
		"address": "fe80::1",
		"netmask": "ffff:ffff:ffff:ffff::",
		"network": "fe80::",
		"scope6":  "link",
	} {
		if binding[key] != want {
			t.Fatalf("dragonFlyIPv6Binding(valid)[%s] = %#v, want %#v", key, binding[key], want)
		}
	}

	binding, ok = dragonFlyIPv6Binding([]string{"inet6", "2001:db8::1", "prefixlen", "invalid"})
	if !ok {
		t.Fatal("dragonFlyIPv6Binding(invalid prefix) ok = false, want true")
	}
	if _, exists := binding["netmask"]; exists {
		t.Fatalf("dragonFlyIPv6Binding(invalid prefix) = %#v, want no netmask", binding)
	}
	if _, exists := binding["network"]; exists {
		t.Fatalf("dragonFlyIPv6Binding(invalid prefix) = %#v, want no network", binding)
	}
	if binding["scope6"] != "global" {
		t.Fatalf("dragonFlyIPv6Binding(invalid prefix) scope6 = %#v, want global", binding["scope6"])
	}

	if binding, ok := dragonFlyIPv6Binding([]string{"inet6", "10.0.2.15"}); ok || binding != nil {
		t.Fatalf("dragonFlyIPv6Binding(IPv4) = %#v, %v; want nil, false", binding, ok)
	}
}

func TestNetworkingInterfacesForPlatformDragonFlyFallsBackToIfconfigWhenGoHasNoInterfaces(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.host = &fakeHostOS{runOutput: dragonFlyIfconfigFixture}
	got := networkingInterfacesForPlatform(session, "dragonfly", func() ([]networkInterfaceSnapshot, error) {
		return nil, nil
	})

	vtnet0, ok := got["vtnet0"].(map[string]any)
	if !ok {
		t.Fatalf("vtnet0 = %#v, want interface map", got["vtnet0"])
	}
	if got, want := vtnet0["mtu"], 1500; got != want {
		t.Fatalf("vtnet0 mtu = %#v, want %#v", got, want)
	}
	if got, want := vtnet0["mac"], "52:54:00:12:34:56"; got != want {
		t.Fatalf("vtnet0 mac = %#v, want %#v", got, want)
	}
	bindings, ok := vtnet0["bindings"].([]any)
	if !ok || len(bindings) != 1 {
		t.Fatalf("vtnet0 bindings = %#v, want one IPv4 binding", vtnet0["bindings"])
	}
	if got, want := bindings[0].(map[string]any)["network"], "10.0.2.0"; got != want {
		t.Fatalf("vtnet0 IPv4 network = %#v, want %#v", got, want)
	}
	bindings6, ok := vtnet0["bindings6"].([]any)
	if !ok || len(bindings6) != 1 {
		t.Fatalf("vtnet0 bindings6 = %#v, want one IPv6 binding", vtnet0["bindings6"])
	}
	if got, want := bindings6[0].(map[string]any)["scope6"], "link"; got != want {
		t.Fatalf("vtnet0 IPv6 scope6 = %#v, want %#v", got, want)
	}
}

func TestNetworkingInterfacesForPlatformDragonFlyMergesIfconfigIntoPartialSnapshot(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.host = &fakeHostOS{runOutput: dragonFlyIfconfigFixture}
	got := networkingInterfacesForPlatform(session, "dragonfly", func() ([]networkInterfaceSnapshot, error) {
		return []networkInterfaceSnapshot{{Interface: net.Interface{Name: "vtnet0"}}}, nil
	})

	vtnet0, ok := got["vtnet0"].(map[string]any)
	if !ok {
		t.Fatalf("vtnet0 = %#v, want interface map", got["vtnet0"])
	}
	if got, want := vtnet0["mtu"], 1500; got != want {
		t.Fatalf("vtnet0 mtu = %#v, want %#v", got, want)
	}
	if got, want := vtnet0["mac"], "52:54:00:12:34:56"; got != want {
		t.Fatalf("vtnet0 mac = %#v, want %#v", got, want)
	}
	if bindings, ok := vtnet0["bindings"].([]any); !ok || len(bindings) != 1 {
		t.Fatalf("vtnet0 bindings = %#v, want one IPv4 binding", vtnet0["bindings"])
	}
}

func TestMergeMissingInterfaceFactsMergesBindingsByAddress(t *testing.T) {
	t.Parallel()

	values := map[string]any{
		"vtnet0": map[string]any{
			"bindings": []any{map[string]any{"address": "10.0.2.15"}},
		},
	}
	fallback := map[string]any{
		"vtnet0": map[string]any{
			"bindings": []any{
				map[string]any{"address": "10.0.2.15", "netmask": "255.255.255.0", "network": "10.0.2.0"},
				map[string]any{"address": "10.0.2.16", "netmask": "255.255.255.0", "network": "10.0.2.0"},
			},
		},
	}

	mergeMissingInterfaceFacts(values, fallback)

	bindings := values["vtnet0"].(map[string]any)["bindings"].([]any)
	if len(bindings) != 2 {
		t.Fatalf("bindings = %#v, want existing binding filled and fallback alias appended", bindings)
	}
	first := bindings[0].(map[string]any)
	if first["netmask"] != "255.255.255.0" || first["network"] != "10.0.2.0" {
		t.Fatalf("first binding = %#v, want netmask/network filled", first)
	}
}

func TestCurrentNetworkingDataDragonFlyExpandsIfconfigFallback(t *testing.T) {
	t.Parallel()

	interfaces := dragonFlyInterfacesFromIfconfig(dragonFlyIfconfigFixture)
	run := func(name string, args ...string) string {
		if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
			return "route to: default\ninterface: vtnet0\n"
		}
		if name == "ifconfig" {
			return dragonFlyIfconfigFixture
		}
		t.Fatalf("unexpected command %q %#v", name, args)
		return ""
	}
	readFile := func(path string) ([]byte, error) {
		if path != "/var/db/dhclient.leases.vtnet0" {
			return nil, os.ErrNotExist
		}
		return []byte("lease {\n  option dhcp-server-identifier 10.0.2.2;\n}\n"), nil
	}

	primary, got := currentNetworkingData("dragonfly", interfaces, run, readFile)
	if primary != "vtnet0" {
		t.Fatalf("primary = %q, want vtnet0", primary)
	}
	vtnet0 := got["vtnet0"].(map[string]any)
	for key, want := range map[string]any{
		"dhcp":              "10.0.2.2",
		"ip":                "10.0.2.15",
		"netmask":           "255.255.255.0",
		"network":           "10.0.2.0",
		"ip6":               "fe80::5054:ff:fe12:3456",
		"netmask6":          "ffff:ffff:ffff:ffff::",
		"network6":          "fe80::",
		"scope6":            "link",
		"operational_state": "active",
	} {
		if got := vtnet0[key]; got != want {
			t.Fatalf("vtnet0[%s] = %#v, want %#v", key, got, want)
		}
	}
}

func TestCurrentNetworkingDataAddsIllumosDHCPServer(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"vioif0": map[string]any{
			"bindings": []any{map[string]any{"address": "192.168.122.240", "netmask": "255.255.255.0", "network": "192.168.122.0"}},
		},
		"lo0": map[string]any{
			"bindings": []any{map[string]any{"address": "127.0.0.1", "netmask": "255.0.0.0", "network": "127.0.0.0"}},
		},
	}
	run := func(name string, args ...string) string {
		if name == "route" && reflect.DeepEqual(args, []string{"-n", "get", "default"}) {
			return "route to: default\ninterface: vioif0\n"
		}
		if name == "dhcpinfo" && reflect.DeepEqual(args, []string{"-i", "vioif0", "ServerID"}) {
			return "192.168.122.1\n"
		}
		if name == "dhcpinfo" {
			return ""
		}
		t.Fatalf("unexpected command %q %#v", name, args)
		return ""
	}

	primary, got := currentNetworkingData("illumos", interfaces, run)
	if primary != "vioif0" {
		t.Fatalf("primary = %q, want vioif0", primary)
	}
	vioif0 := got["vioif0"].(map[string]any)
	if got, want := vioif0["dhcp"], "192.168.122.1"; got != want {
		t.Fatalf("vioif0 dhcp = %#v, want %#v", got, want)
	}
	if got, want := vioif0["ip"], "192.168.122.240"; got != want {
		t.Fatalf("vioif0 ip = %#v, want %#v", got, want)
	}
	if dhcp := networkingDHCPFact(got, "192.168.122.240"); dhcp != "192.168.122.1" {
		t.Fatalf("networkingDHCPFact() = %q, want 192.168.122.1", dhcp)
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

func TestLinuxDHCPCDDHCPServer(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		"interface eth0",
		"static ip_address=192.0.2.10/24",
		"dhcp_server_identifier='10.32.10.163'",
	}, "\n")
	if got, want := linuxDHCPCDDHCPServer(content), "10.32.10.163"; got != want {
		t.Fatalf("linuxDHCPCDDHCPServer() = %q, want %q", got, want)
	}
	if got := linuxDHCPCDDHCPServer("interface eth0\nstatic routers=10.32.10.1\n"); got != "" {
		t.Fatalf("linuxDHCPCDDHCPServer(no server) = %q, want empty", got)
	}
}

func TestLinuxDHCPServerFromLeaseDirReadsMatchingLease(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "00-commented.lease"), `lease {
  # interface "eth0";
  option dhcp-server-identifier 10.66.66.66;
}`)
	writeFile(t, filepath.Join(dir, "dhclient.leases"), `lease {
  interface "eth0-backup";
  option dhcp-server-identifier 10.99.99.98;
}
lease {
  interface "eth0";
  option dhcp-server-identifier 10.32.10.163;
}
lease {
  interface "eth0.100";
  option dhcp-server-identifier 10.99.99.97;
}`)
	writeFile(t, filepath.Join(dir, "dhclient.en1.lease"), `lease {
  interface "en1";
  option dhcp-server-identifier 10.99.99.99;
}`)
	writeFile(t, filepath.Join(dir, "not-a-lease.txt"), `SERVER_ADDRESS=192.0.2.1`)

	if got, want := linuxDHCPServerFromLeaseDir(dir, "eth0"), "10.32.10.163"; got != want {
		t.Fatalf("linuxDHCPServerFromLeaseDir() = %q, want %q", got, want)
	}
	if got, want := linuxDHCPServerFromLeaseDir(dir, "eth0-backup"), "10.99.99.98"; got != want {
		t.Fatalf("linuxDHCPServerFromLeaseDir(eth0-backup) = %q, want %q", got, want)
	}
}

func TestLinuxDHCPServerFromLeaseDirUsesFilenameFallbackWhenInterfaceValueMalformed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dhclient.eth0.lease"), `lease {
  interface "broken
  option host-name "router";
  option dhcp-server-identifier 10.32.10.163;
}`)

	if got, want := linuxDHCPServerFromLeaseDir(dir, "eth0"), "10.32.10.163"; got != want {
		t.Fatalf("linuxDHCPServerFromLeaseDir() = %q, want filename fallback server %q", got, want)
	}
}

func TestLinuxDHCPServerFromLeaseDirDoesNotUseFilenameFallbackForUnrelatedExplicitInterface(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dhclient.eth0.lease"), `lease {
  interface "eth1";
  option dhcp-server-identifier 10.99.99.99;
}`)

	if got := linuxDHCPServerFromLeaseDir(dir, "eth0"); got != "" {
		t.Fatalf("linuxDHCPServerFromLeaseDir() = %q, want empty for explicit non-matching interface", got)
	}
}

func TestLinuxDHCPServerFromLeaseDirStopsAtMatchingLeaseWithoutServer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dhclient.eth0.lease"), `lease {
  interface "eth0";
  option host-name "dhcp-server-identifier 10.88.88.88";
  # option dhcp-server-identifier 10.99.99.99;
}`)
	writeFile(t, filepath.Join(dir, "zz-dhclient.eth0.lease"), `lease {
  interface "eth0";
  option dhcp-server-identifier 10.32.10.163;
}`)

	if got := linuxDHCPServerFromLeaseDir(dir, "eth0"); got != "" {
		t.Fatalf("linuxDHCPServerFromLeaseDir() = %q, want empty latest matching lease server", got)
	}
}

func TestLinuxDHClientDHCPServerForInterfaceFallsBackWhenInterfaceIsOutsideLeaseBlock(t *testing.T) {
	t.Parallel()

	content := `interface "eth0";
lease {
  option dhcp-server-identifier 10.32.10.163;
}`

	got, ok := linuxDHClientDHCPServerForInterface(content, "eth0")
	if !ok {
		t.Fatal("linuxDHClientDHCPServerForInterface() ok = false, want true")
	}
	if got != "10.32.10.163" {
		t.Fatalf("linuxDHClientDHCPServerForInterface() = %q, want 10.32.10.163", got)
	}

	content = `interface "eth0";
lease {
  option dhcp-server-identifier 10.32.10.163;
}
lease {
  interface "eth1";
  option dhcp-server-identifier 10.99.99.99;
}`
	got, ok = linuxDHClientDHCPServerForInterface(content, "eth0")
	if !ok {
		t.Fatal("linuxDHClientDHCPServerForInterface(mixed) ok = false, want true")
	}
	if got != "" {
		t.Fatalf("linuxDHClientDHCPServerForInterface(mixed) = %q, want empty ambiguous mixed lease", got)
	}

	content = `interface "eth0";
lease {
  option dhcp-server-identifier 10.32.10.163;
}
lease {
  option dhcp-server-identifier 10.99.99.99;
}`
	got, ok = linuxDHClientDHCPServerForInterface(content, "eth0")
	if !ok {
		t.Fatal("linuxDHClientDHCPServerForInterface(multiple unqualified) ok = false, want true")
	}
	if got != "10.99.99.99" {
		t.Fatalf("linuxDHClientDHCPServerForInterface(multiple unqualified) = %q, want latest matching lease", got)
	}
}

func TestLinuxDHClientDHCPServerForInterfaceIgnoresCommentAndQuotedBraces(t *testing.T) {
	t.Parallel()

	content := `# lease { ignored }
lease {
  # } ignored comment brace
  option host-name "edge-}router";
  interface "eth0";
  option dhcp-server-identifier 10.32.10.163;
}
lease {
  interface "eth1";
  option dhcp-server-identifier 10.99.99.99;
}`

	got, ok := linuxDHClientDHCPServerForInterface(content, "eth0")
	if !ok {
		t.Fatal("linuxDHClientDHCPServerForInterface() ok = false, want true")
	}
	if got != "10.32.10.163" {
		t.Fatalf("linuxDHClientDHCPServerForInterface() = %q, want 10.32.10.163", got)
	}
}

func TestLinuxDHClientDHCPServerForInterfaceSkipsHeaderAndInterfaceComments(t *testing.T) {
	t.Parallel()

	content := `lease # dhclient permits comments in whitespace
{
  interface # comment before value
  "eth0";
  option dhcp-server-identifier 10.32.10.163;
}
lease # another header comment
{
  interface "eth1";
  option dhcp-server-identifier 10.99.99.99;
}`

	got, ok := linuxDHClientDHCPServerForInterface(content, "eth0")
	if !ok {
		t.Fatal("linuxDHClientDHCPServerForInterface() ok = false, want true")
	}
	if got != "10.32.10.163" {
		t.Fatalf("linuxDHClientDHCPServerForInterface() = %q, want 10.32.10.163", got)
	}
	if blocks := linuxDHClientLeaseBlocks(content); len(blocks) != 2 {
		t.Fatalf("linuxDHClientLeaseBlocks() found %d blocks, want 2", len(blocks))
	}
}

func TestLinuxDHClientDHCPServerForInterfaceMatchesInlineInterfaceStatement(t *testing.T) {
	t.Parallel()

	content := `lease { interface "eth1"; option dhcp-server-identifier 10.99.99.99; }
lease { interface "eth0"; option dhcp-server-identifier 10.32.10.163; }`

	got, ok := linuxDHClientDHCPServerForInterface(content, "eth0")
	if !ok {
		t.Fatal("linuxDHClientDHCPServerForInterface() ok = false, want true")
	}
	if got != "10.32.10.163" {
		t.Fatalf("linuxDHClientDHCPServerForInterface() = %q, want 10.32.10.163", got)
	}
}

func TestLinuxDHClientDHCPServerForInterfaceLatestMatchingLeaseWithoutServerWins(t *testing.T) {
	t.Parallel()

	content := `lease {
  interface "eth0";
  option dhcp-server-identifier 10.32.10.163;
}
lease {
  interface "eth0";
  option host-name "dhcp-server-identifier 10.88.88.88";
  # option dhcp-server-identifier 10.99.99.99;
}`

	got, ok := linuxDHClientDHCPServerForInterface(content, "eth0")
	if !ok {
		t.Fatal("linuxDHClientDHCPServerForInterface() ok = false, want true")
	}
	if got != "" {
		t.Fatalf("linuxDHClientDHCPServerForInterface() = %q, want empty latest matching lease server", got)
	}
}

func TestLinuxDHClientDHCPServerForInterfaceResyncsAfterMalformedLeaseBlock(t *testing.T) {
	t.Parallel()

	content := `lease {
  interface "eth0";
  option dhcp-server-identifier 10.32.10.163;
lease {
  interface "eth1";
  option dhcp-server-identifier 10.99.99.99;
}`

	got, ok := linuxDHClientDHCPServerForInterface(content, "eth0")
	if !ok {
		t.Fatal("linuxDHClientDHCPServerForInterface() ok = false, want true")
	}
	if got != "" {
		t.Fatalf("linuxDHClientDHCPServerForInterface() = %q, want empty when only later valid block belongs to eth1", got)
	}

	blocks := linuxDHClientLeaseBlocks(content)
	if len(blocks) != 1 || !dhclientContentMatchesInterface(blocks[0], "eth1") {
		t.Fatalf("linuxDHClientLeaseBlocks() = %#v, want resynced eth1 block", blocks)
	}
}

func TestLinuxDHClientLeaseBlocksResyncsAcrossRepeatedMalformedBlocks(t *testing.T) {
	t.Parallel()

	var content strings.Builder
	for i := 0; i < 128; i++ {
		content.WriteString("lease {\n")
		content.WriteString("  interface \"stale\";\n")
	}
	content.WriteString(`lease {
  interface "eth0";
  option dhcp-server-identifier 10.32.10.163;
}`)

	blocks := linuxDHClientLeaseBlocks(content.String())
	if len(blocks) != 1 || !dhclientContentMatchesInterface(blocks[0], "eth0") {
		t.Fatalf("linuxDHClientLeaseBlocks() = %#v, want final eth0 block", blocks)
	}
}

func TestLinuxDHClientDHCPServerForInterfaceResyncsAfterUnterminatedQuotedString(t *testing.T) {
	t.Parallel()

	content := `lease {
  interface "eth0";
  option host-name "unterminated
lease {
  interface "eth1";
  option dhcp-server-identifier 10.99.99.99;
}`

	got, ok := linuxDHClientDHCPServerForInterface(content, "eth0")
	if !ok {
		t.Fatal("linuxDHClientDHCPServerForInterface() ok = false, want true")
	}
	if got != "" {
		t.Fatalf("linuxDHClientDHCPServerForInterface() = %q, want empty when only later valid block belongs to eth1", got)
	}

	blocks := linuxDHClientLeaseBlocks(content)
	if len(blocks) != 1 || !dhclientContentMatchesInterface(blocks[0], "eth1") {
		t.Fatalf("linuxDHClientLeaseBlocks() = %#v, want resynced eth1 block", blocks)
	}
}

func TestDHClientScannerIgnoresInterfaceTokensInCommentsAndStrings(t *testing.T) {
	t.Parallel()

	content := `# interface "commented0";
lease {
  option host-name "interface \"quoted0\"";
  interface "eth0";
}`

	got := dhclientInterfaceNames(content)
	want := []string{"eth0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dhclientInterfaceNames(%q) = %#v, want %#v", content, got, want)
	}
}

func TestDHClientScannerHandlesMalformedBlocksAndQuotedStrings(t *testing.T) {
	t.Parallel()

	if got := linuxDHClientLeaseBlocks(`lease { interface "eth0";`); len(got) != 0 {
		t.Fatalf("linuxDHClientLeaseBlocks(unclosed) = %#v, want empty", got)
	}
	if got := linuxDHClientLeaseBlocks(`release { interface "eth0"; }`); len(got) != 0 {
		t.Fatalf("linuxDHClientLeaseBlocks(release keyword) = %#v, want empty", got)
	}
	if value, next, ok := dhclientQuotedStringValue(`not quoted`, 0); ok || value != "" || next != 0 {
		t.Fatalf("dhclientQuotedStringValue(unquoted) = %q, %d, %v; want empty, 0, false", value, next, ok)
	}
	if value, _, ok := dhclientQuotedStringValue(`"eth\"0"`, 0); !ok || value != `eth"0` {
		t.Fatalf("dhclientQuotedStringValue(escaped) = %q, %v; want eth\"0, true", value, ok)
	}
	if value, _, ok := dhclientQuotedStringValue(`"unterminated`, 0); ok || value != "" {
		t.Fatalf("dhclientQuotedStringValue(unterminated) = %q, %v; want empty, false", value, ok)
	}
	for _, input := range []string{"\"eth\n0\"", "\"eth\\\n0\""} {
		if value, _, ok := dhclientQuotedStringValue(input, 0); ok || value != "" {
			t.Fatalf("dhclientQuotedStringValue(%q) = %q, %v; want empty, false", input, value, ok)
		}
	}
}

func TestLinuxDHClientDHCPServerUsesLastLeaseServer(t *testing.T) {
	t.Parallel()

	content := `lease {
  interface "eth0";
  option dhcp-server-identifier 10.32.10.163;
}
lease {
  interface "eth0";
  option dhcp-server-identifier 10.32.10.254;
}`

	if got, want := linuxDHClientDHCPServer(content), "10.32.10.254"; got != want {
		t.Fatalf("linuxDHClientDHCPServer() = %q, want %q", got, want)
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

func TestAddLinuxDHCPServersFromSnapshotsAddsInterfaceDHCP(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{
		files: map[string][]byte{
			"/run/systemd/netif/leases/7": []byte("SERVER_ADDRESS=10.16.122.163\n"),
			"/run/systemd/netif/leases/8": []byte("SERVER_ADDRESS=10.16.123.163\n"),
		},
	}
	s := NewSession()
	s.host = host
	values := map[string]any{
		"eth0": map[string]any{"bindings": []any{map[string]any{"address": "10.16.122.20"}}},
		"eth1": map[string]any{"bindings": []any{map[string]any{"address": "10.16.123.20"}}},
	}
	snapshots := []networkInterfaceSnapshot{
		{Interface: net.Interface{Name: "eth1", Index: 8}},
		{Interface: net.Interface{Name: "eth0", Index: 7}},
	}

	addLinuxDHCPServersFromSnapshots(s, values, snapshots)

	if got, want := values["eth0"].(map[string]any)["dhcp"], "10.16.122.163"; got != want {
		t.Fatalf("eth0 dhcp = %#v, want %q", got, want)
	}
	if got, want := values["eth1"].(map[string]any)["dhcp"], "10.16.123.163"; got != want {
		t.Fatalf("eth1 dhcp = %#v, want %q", got, want)
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

	if value, ok := networking["ip6"]; ok && value == "" {
		t.Fatalf("networking.ip6 = empty string in %#v, want omitted or populated", networking)
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
	ip6, _ := networking["ip6"].(string)
	if ip6 == "" {
		t.Skip("host has no primary IPv6 address")
	}

	for _, key := range []string{"netmask6", "network6", "scope6"} {
		value, _ := networking[key].(string)
		if value == "" {
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

func TestPrimaryIPv6BindingReturnsNilForEmptyMalformedOrMissingBinding(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"bad":     "ignored",
		"no-list": map[string]any{"bindings6": "ignored"},
		"en0": map[string]any{
			"bindings6": []any{"ignored", map[string]any{"address": "2001:db8::10"}},
		},
	}
	for _, primary := range []string{"", "2001:db8::20"} {
		if got := primaryIPv6Binding(interfaces, primary); got != nil {
			t.Fatalf("primaryIPv6Binding(%q) = %#v, want nil", primary, got)
		}
	}
}

func TestNetworkAddressReturnsMaskedAddress(t *testing.T) {
	t.Parallel()

	ip := &net.IPNet{IP: net.ParseIP("192.0.2.42"), Mask: net.CIDRMask(24, 32)}
	if got := networkAddress(ip); got != "192.0.2.0" {
		t.Fatalf("networkAddress(IPv4) = %q, want 192.0.2.0", got)
	}
	ip = &net.IPNet{IP: net.ParseIP("2001:db8::42"), Mask: net.CIDRMask(64, 128)}
	if got := networkAddress(ip); got != "2001:db8::" {
		t.Fatalf("networkAddress(IPv6) = %q, want 2001:db8::", got)
	}
	if got := networkAddress(nil); got != "" {
		t.Fatalf("networkAddress(nil) = %q, want empty", got)
	}
}

func TestParseInterfaceAddrAcceptsIPNetAndIPAddrOnly(t *testing.T) {
	t.Parallel()

	ipNet := &net.IPNet{IP: net.ParseIP("192.0.2.10"), Mask: net.CIDRMask(24, 32)}
	ip, parsedNet, ok := parseInterfaceAddr(ipNet)
	if !ok || !ip.Equal(ipNet.IP) || parsedNet == nil || !parsedNet.IP.Equal(ipNet.IP) || !reflect.DeepEqual(parsedNet.Mask, ipNet.Mask) {
		t.Fatalf("parseInterfaceAddr(IPNet) = %v, %#v, %v; want %v, equivalent net, true", ip, parsedNet, ok, ipNet.IP)
	}

	ipAddr := &net.IPAddr{IP: net.ParseIP("2001:db8::1")}
	ip, parsedNet, ok = parseInterfaceAddr(ipAddr)
	if !ok || !ip.Equal(ipAddr.IP) || parsedNet != nil {
		t.Fatalf("parseInterfaceAddr(IPAddr) = %v, %#v, %v; want %v, nil, true", ip, parsedNet, ok, ipAddr.IP)
	}

	ip, parsedNet, ok = parseInterfaceAddr(fakeAddr("192.0.2.10"))
	if ok || ip != nil || parsedNet != nil {
		t.Fatalf("parseInterfaceAddr(fakeAddr) = %v, %#v, %v; want nil, nil, false", ip, parsedNet, ok)
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

func TestPrimaryIPv6ScopeDefaultsToGlobalForRoutableAddressWithoutScope(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"bindings6": []any{
				map[string]any{"address": "2001:db8::10"},
			},
		},
	}

	if binding := primaryIPv6Binding(interfaces, "2001:db8::10"); binding == nil {
		t.Fatal("primaryIPv6Binding() = nil; test fixture must contain the primary address")
	}
	if got := primaryIPv6Scope(interfaces, "2001:db8::10"); got != "global" {
		t.Fatalf("primaryIPv6Scope() = %q, want global", got)
	}
}

func TestPrimaryIPv6ScopeOmitsEmptyPrimaryAddress(t *testing.T) {
	t.Parallel()

	if got := primaryIPv6Scope(map[string]any{"en0": map[string]any{}}, ""); got != "" {
		t.Fatalf("primaryIPv6Scope(empty) = %q, want empty", got)
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

func TestPrimaryIPv6AddressIgnoresMalformedBindings(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"en0": map[string]any{
			"bindings6": []any{
				"not a binding",
				map[string]any{"address": "not an ip"},
				map[string]any{"address": "2001:db8::10"},
			},
		},
	}

	if got := primaryIPv6Address(interfaces, "en0"); got != "2001:db8::10" {
		t.Fatalf("primaryIPv6Address(malformed bindings) = %q, want 2001:db8::10", got)
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
		{"site local", "fec0::1", "site"},
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

func TestIsIPv6SiteLocalRejectsInvalidAndNonSiteLocalAddresses(t *testing.T) {
	t.Parallel()

	for _, ip := range []net.IP{nil, net.ParseIP("192.0.2.10"), net.ParseIP("2001:db8::1"), net.ParseIP("fe80::1")} {
		if isIPv6SiteLocal(ip) {
			t.Fatalf("isIPv6SiteLocal(%v) = true, want false", ip)
		}
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

func TestPrimaryIPv4BindingReturnsNilForEmptyMalformedOrMissingBinding(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"bad":     "ignored",
		"no-list": map[string]any{"bindings": "ignored"},
		"eth0": map[string]any{
			"bindings": []any{"ignored", map[string]any{"address": "192.0.2.10"}},
		},
	}
	for _, primary := range []string{"", "192.0.2.20"} {
		if got := primaryIPv4Binding(interfaces, primary); got != nil {
			t.Fatalf("primaryIPv4Binding(%q) = %#v, want nil", primary, got)
		}
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

func TestPrimaryInterfaceFactReturnsNilForMissingInterface(t *testing.T) {
	interfaces := map[string]any{
		"eth0": "not-a-map",
	}

	for _, name := range []string{"", "eth0", "missing"} {
		if got := primaryInterfaceFact(interfaces, name, "mac"); got != nil {
			t.Fatalf("primaryInterfaceFact(%q) = %#v, want nil", name, got)
		}
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

func TestHostnameFactValuesOmitFQDNAndDomainWhenHostnameLookupFailed(t *testing.T) {
	t.Parallel()

	fqdnValue, domainValue := hostnameFactValues(nil, "foo.example.test", "example.test")
	if fqdnValue != nil || domainValue != nil {
		t.Fatalf("hostnameFactValues(nil) = %#v, %#v; want nil, nil", fqdnValue, domainValue)
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

func TestLinuxHostNameFromLookupsFallsBackWhenPrimaryLookupFails(t *testing.T) {
	hostname, value := linuxHostNameFromLookups(
		func() (string, error) { return "", errors.New("hostname unavailable") },
		func() string { return "kernel-host" },
		discardLog(),
	)

	if hostname != "kernel-host" || value != "kernel-host" {
		t.Fatalf("linuxHostNameFromLookups() = %q, %#v; want kernel-host", hostname, value)
	}
}

func TestLinuxHostNameFromLookupsReturnsNilWhenFallbackIsMissingOrUnusable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fallback func() string
	}{
		{name: "missing fallback"},
		{name: "empty fallback", fallback: func() string { return " " }},
		{name: "zero address fallback", fallback: func() string { return "0.0.0.0" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostname, value := linuxHostNameFromLookups(
				func() (string, error) { return "", nil },
				tt.fallback,
				discardLog(),
			)
			if hostname != "" || value != nil {
				t.Fatalf("linuxHostNameFromLookups() = %q, %#v; want empty nil", hostname, value)
			}
		})
	}
}

func TestLinuxHostNameFromLookupsPrefersUsablePrimaryLookup(t *testing.T) {
	fallbackCalled := false
	hostname, value := linuxHostNameFromLookups(
		func() (string, error) { return "socket-host", nil },
		func() string {
			fallbackCalled = true
			return "kernel-host"
		},
		discardLog(),
	)

	if hostname != "socket-host" || value != "socket-host" {
		t.Fatalf("linuxHostNameFromLookups() = %q, %#v; want socket-host", hostname, value)
	}
	if fallbackCalled {
		t.Fatal("linuxHostNameFromLookups() called fallback for usable primary hostname")
	}
}

func TestFQDNReturnsEmptyAndAlreadyQualifiedNames(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     string
	}{
		{name: "empty", hostname: "", want: ""},
		{name: "already qualified", hostname: "node.example.test", want: "node.example.test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := fqdn(tt.hostname); got != tt.want {
				t.Fatalf("fqdn(%q) = %q, want %q", tt.hostname, got, tt.want)
			}
		})
	}
}

func TestFQDNWithLookupUsesReverseLookupAndFallsBack(t *testing.T) {
	t.Parallel()

	got := fqdnWithLookup("node", func(host string) ([]string, error) {
		if host != "node" {
			t.Fatalf("lookup host = %q, want node", host)
		}
		return []string{"node.example.test."}, nil
	})
	if got != "node.example.test" {
		t.Fatalf("fqdnWithLookup() = %q, want node.example.test", got)
	}

	for _, tt := range []struct {
		name  string
		addrs []string
		err   error
	}{
		{name: "lookup error", err: os.ErrNotExist},
		{name: "empty lookup"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := fqdnWithLookup("node", func(string) ([]string, error) {
				return tt.addrs, tt.err
			})
			if got != "node" {
				t.Fatalf("fqdnWithLookup(%s) = %q, want node", tt.name, got)
			}
		})
	}
}

func TestHostNameForPlatformUsesLinuxFallbackOnlyOnLinux(t *testing.T) {
	hostname, value := hostNameForPlatform(
		"linux",
		func() (string, error) { return "", nil },
		func() string { return "kernel-host" },
		discardLog(),
	)
	if hostname != "kernel-host" || value != "kernel-host" {
		t.Fatalf("linux hostNameForPlatform() = %q, %#v, want kernel-host", hostname, value)
	}

	hostname, value = hostNameForPlatform(
		"darwin",
		func() (string, error) { return "", nil },
		func() string {
			t.Fatal("non-Linux platform used Linux hostname fallback")
			return "unused"
		},
		discardLog(),
	)
	if hostname != "" || value != "" {
		t.Fatalf("darwin hostNameForPlatform() = %q, %#v, want empty lookup result", hostname, value)
	}
}

func TestReadLinuxKernelHostnameUsesInjectedReader(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		if path != "/proc/sys/kernel/hostname" {
			t.Fatalf("path = %q, want kernel hostname path", path)
		}
		return []byte("kernel-host\n"), nil
	}

	if got := readLinuxKernelHostname(readFile); got != "kernel-host" {
		t.Fatalf("readLinuxKernelHostname() = %q, want kernel-host", got)
	}
}

func TestIPFromAddrAcceptsIPNetAndIPAddrOnly(t *testing.T) {
	ipNet := &net.IPNet{IP: net.ParseIP("192.0.2.10"), Mask: net.CIDRMask(24, 32)}
	if got, ok := ipFromAddr(ipNet); !ok || !got.Equal(ipNet.IP) {
		t.Fatalf("ipFromAddr(IPNet) = %v, %v, want %v, true", got, ok, ipNet.IP)
	}
	ipAddr := &net.IPAddr{IP: net.ParseIP("2001:db8::1")}
	if got, ok := ipFromAddr(ipAddr); !ok || !got.Equal(ipAddr.IP) {
		t.Fatalf("ipFromAddr(IPAddr) = %v, %v, want %v, true", got, ok, ipAddr.IP)
	}
	if got, ok := ipFromAddr(fakeAddr("192.0.2.10")); ok || got != nil {
		t.Fatalf("ipFromAddr(fakeAddr) = %v, %v, want nil, false", got, ok)
	}
}

func TestIPv6SelectionRankRejectsNonCandidates(t *testing.T) {
	for _, raw := range []string{"", "192.0.2.10", "::1", "::"} {
		if rank, ok := ipv6SelectionRank(net.ParseIP(raw)); ok {
			t.Fatalf("ipv6SelectionRank(%q) = %d, true; want false", raw, rank)
		}
	}
}

func TestPrimaryIPv6FromAddrsPrefersGlobalAndSkipsLinkLocal(t *testing.T) {
	addrs := []net.Addr{
		ipNetAddr("fe80::1", 64),
		ipNetAddr("fd00::10", 64),
		ipNetAddr("2001:db8::10", 64),
		ipNetAddr("192.0.2.10", 24),
	}

	if got := primaryIPv6FromAddrs(addrs); got != "2001:db8::10" {
		t.Fatalf("primaryIPv6FromAddrs() = %q, want global address", got)
	}
	if got := primaryIPv6FromAddrs([]net.Addr{ipNetAddr("fe80::1", 64)}); got != "" {
		t.Fatalf("primaryIPv6FromAddrs(link-local only) = %q, want empty", got)
	}
}

func ipNetAddr(raw string, bits int) net.Addr {
	ip := net.ParseIP(raw)
	maskBits := 128
	if ip.To4() != nil {
		maskBits = 32
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, maskBits)}
}

type fakeAddr string

func (a fakeAddr) Network() string { return "fake" }
func (a fakeAddr) String() string  { return string(a) }
