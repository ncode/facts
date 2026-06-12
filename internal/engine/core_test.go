package engine

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCoreFacts_includeIntegrationRootFactGroups(t *testing.T) {
	collection := Collection(CoreFacts(testSession))

	for _, name := range []string{"memory", "networking", "os", "path", "processors"} {
		if _, ok := collection[name]; !ok {
			t.Fatalf("CoreFacts(testSession) missing root fact group %q in %#v", name, collection)
		}
	}
}

func TestCoreFacts_fipsEnabledOnlyOnLinuxAndWindows(t *testing.T) {
	collection := Collection(CoreFacts(testSession))

	value, ok := collection["fips_enabled"]
	switch runtime.GOOS {
	case "linux", "windows":
		if _, isBool := value.(bool); !ok || !isBool {
			t.Fatalf("fips_enabled = %#v, want bool", value)
		}
	default:
		if ok {
			t.Fatalf("fips_enabled = %#v, want fact omitted on %s", value, runtime.GOOS)
		}
	}
}

func TestFIPSEnabledFacts_omittedOutsideLinuxAndWindows(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		t.Fatalf("command = %s %#v, want no probe on a platform without the fact", name, args)
		return ""
	}
	for _, goos := range []string{"darwin", "freebsd", "openbsd", "netbsd", "solaris", "aix"} {
		if got := fipsEnabledFacts(goos, "/proc/sys/crypto/fips_enabled", run); got != nil {
			t.Fatalf("fipsEnabledFacts(%s) = %#v, want nil", goos, got)
		}
	}
}

func TestFIPSEnabledFacts_resolveOnLinuxAndWindows(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fips_enabled")
	if err := os.WriteFile(path, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "fips_enabled", Value: true}}
	if got := fipsEnabledFacts("linux", path, nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("fipsEnabledFacts(linux) = %#v, want %#v", got, want)
	}

	run := func(name string, args ...string) string {
		return strings.Join([]string{
			`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
			"    Enabled    REG_DWORD    0x0",
		}, "\n")
	}
	want = []ResolvedFact{{Name: "fips_enabled", Value: false}}
	if got := fipsEnabledFacts("windows", "", run); !reflect.DeepEqual(got, want) {
		t.Fatalf("fipsEnabledFacts(windows) = %#v, want %#v", got, want)
	}
}

func TestCoreFacts_includePathFromEnvironment(t *testing.T) {
	path := "/usr/bin:/etc:/usr/sbin:/usr/ucb:/usr/bin/X11:/sbin:/usr/java6/jre/bin:/usr/java6/bin"
	t.Setenv("PATH", path)
	collection := Collection(CoreFactsWithRuby(NewSession(), false))

	if got := collection["path"]; got != path {
		t.Fatalf("path = %#v, want %#v", got, path)
	}
}

func TestCoreFacts_includeFacterVersion(t *testing.T) {
	collection := Collection(CoreFactsWithRuby(testSession, false))

	if got := collection["facterversion"]; got != Version {
		t.Fatalf("facterversion = %#v, want %#v", got, Version)
	}
}

func TestCurrentFIPSEnabledReadsWindowsRegistry(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		if name != "reg" || !reflect.DeepEqual(args, []string{"query", `HKLM\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`, "/v", "Enabled"}) {
			t.Fatalf("command = %s %#v", name, args)
		}
		return strings.Join([]string{
			`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
			"    Enabled    REG_DWORD    0xff",
		}, "\n")
	}

	if !currentFIPSEnabled("windows", "", run) {
		t.Fatal("currentFIPSEnabled(windows) = false, want true")
	}
}

func TestParseWindowsFIPSEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name: "enabled decimal",
			input: strings.Join([]string{
				`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
				"    Enabled    REG_DWORD    0x1",
			}, "\n"),
			want: true,
		},
		{
			name: "enabled non-one",
			input: strings.Join([]string{
				`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
				"    Enabled    REG_DWORD    0xff",
			}, "\n"),
			want: true,
		},
		{
			name: "disabled zero",
			input: strings.Join([]string{
				`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
				"    Enabled    REG_DWORD    0x0",
			}, "\n"),
			want: false,
		},
		{
			name:  "missing",
			input: `HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseWindowsFIPSEnabled(tt.input); got != tt.want {
				t.Fatalf("parseWindowsFIPSEnabled() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestIdentityFactFromInfoWindowsOmitsPOSIXFields(t *testing.T) {
	t.Parallel()

	privileged := true
	got := identityFactFromInfo("windows", identityInfo{
		User:       `MG93C9IN9WKOITF\Administrator`,
		UID:        "S-1-5-21-uid",
		GID:        "S-1-5-21-gid",
		Group:      "Administrators",
		Privileged: &privileged,
	})

	want := map[string]any{
		"user":       `MG93C9IN9WKOITF\Administrator`,
		"privileged": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identityFactFromInfo(windows) = %#v, want %#v", got, want)
	}
}

func TestCurrentWindowsIdentityInfoUsesWhoamiCommands(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		switch {
		case name == "whoami" && len(args) == 0:
			return `MG93C9IN9WKOITF\Administrator`
		case name == "whoami" && reflect.DeepEqual(args, []string{"/groups"}):
			return strings.Join([]string{
				`Group Name                                 Type             SID          Attributes`,
				`========================================== ================ ============ ===============================================`,
				`BUILTIN\Administrators                    Alias            S-1-5-32-544 Mandatory group, Enabled by default, Enabled group`,
			}, "\n")
		default:
			t.Fatalf("run = %s %v, want whoami or whoami /groups", name, args)
			return ""
		}
	}

	got := currentWindowsIdentityInfo(run)
	if got.User != `MG93C9IN9WKOITF\Administrator` {
		t.Fatalf("User = %q, want administrator", got.User)
	}
	if got.Privileged == nil || !*got.Privileged {
		t.Fatalf("Privileged = %#v, want true", got.Privileged)
	}
}

func TestCurrentWindowsIdentityInfoLogsFailureWhenUserCannotResolveLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	SetDebugHandler(func(message string) { debugMessages = append(debugMessages, message) })
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := currentWindowsIdentityInfo(func(name string, args ...string) string {
		if name != "whoami" || len(args) != 0 {
			t.Fatalf("run = %s %v, want only whoami", name, args)
		}
		return ""
	})

	if got.User != "" {
		t.Fatalf("User = %q, want empty", got.User)
	}
	if got.Privileged != nil {
		t.Fatalf("Privileged = %#v, want nil", got.Privileged)
	}
	want := []string{"failure resolving identity facts: "}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestParseWindowsAdministratorGroupsDetectsDenyOnlyAdmin(t *testing.T) {
	t.Parallel()

	output := strings.Join([]string{
		`Group Name                                 Type             SID          Attributes`,
		`========================================== ================ ============ ===============================================`,
		`BUILTIN\Administrators                    Alias            S-1-5-32-544 Group used for deny only`,
	}, "\n")

	got, ok := parseWindowsAdministratorGroups(output)
	if !ok {
		t.Fatal("parseWindowsAdministratorGroups() ok = false, want true")
	}
	if got {
		t.Fatal("parseWindowsAdministratorGroups() = true, want false")
	}
}

func TestIdentityFactFromInfoPOSIXReturnsNumericUIDAndGID(t *testing.T) {
	t.Parallel()

	privileged := false
	got := identityFactFromInfo("linux", identityInfo{
		User:       "test1.test2",
		UID:        "501",
		GID:        "20",
		Group:      "staff",
		Privileged: &privileged,
	})

	want := map[string]any{
		"user":       "test1.test2",
		"uid":        501,
		"gid":        20,
		"group":      "staff",
		"privileged": false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identityFactFromInfo(posix) = %#v, want %#v", got, want)
	}
}

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

func TestNetworkingInterfacesWindowsKeepsAddresslessInterfaceLikeRubyResolver(t *testing.T) {
	t.Parallel()

	interfaces := networkingInterfacesFromSnapshots([]networkInterfaceSnapshot{
		{
			Interface: net.Interface{
				Name:  "Ethernet0",
				MTU:   1500,
				Flags: net.FlagUp,
			},
		},
	}, "windows")

	primary, got := currentNetworkingData("windows", interfaces, nil)

	if primary != "" {
		t.Fatalf("primary = %q, want empty for addressless interface", primary)
	}
	want := map[string]any{
		"Ethernet0": map[string]any{
			"dhcp": nil,
			"mtu":  1500,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentNetworkingData(windows) = %#v, want %#v", got, want)
	}
}

func TestNetworkingInterfacesIncludesAddresslessTunnelsLikeRubyResolver(t *testing.T) {
	t.Parallel()

	_, en0, err := net.ParseCIDR("192.168.1.20/24")
	if err != nil {
		t.Fatal(err)
	}
	en0.IP = net.ParseIP("192.168.1.20")

	got := networkingInterfacesFromSnapshots([]networkInterfaceSnapshot{
		{
			// macOS gif0: down, point-to-point tunnel without addresses.
			Interface: net.Interface{
				Name:  "gif0",
				MTU:   1280,
				Flags: net.FlagPointToPoint | net.FlagMulticast,
			},
		},
		{
			// macOS stf0: down 6to4 tunnel without addresses or flags.
			Interface: net.Interface{
				Name: "stf0",
				MTU:  1280,
			},
		},
		{
			Interface: net.Interface{
				Name:         "en0",
				MTU:          1500,
				Flags:        net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
				HardwareAddr: net.HardwareAddr{0x00, 0x50, 0x56, 0x9a, 0xf8, 0x6b},
			},
			Addrs: []net.Addr{en0},
		},
	}, "darwin")

	wantTunnel := map[string]any{"mtu": 1280}
	for _, name := range []string{"gif0", "stf0"} {
		if !reflect.DeepEqual(got[name], wantTunnel) {
			t.Errorf("networking.interfaces[%s] = %#v, want %#v", name, got[name], wantTunnel)
		}
	}
	wantEn0 := map[string]any{
		"mtu": 1500,
		"mac": "00:50:56:9a:f8:6b",
		"bindings": []any{map[string]any{
			"address": "192.168.1.20",
			"netmask": "255.255.255.0",
			"network": "192.168.1.0",
		}},
	}
	if !reflect.DeepEqual(got["en0"], wantEn0) {
		t.Errorf("networking.interfaces[en0] = %#v, want %#v", got["en0"], wantEn0)
	}
}

func TestNetworkingInterfacesWindowsLogsFailureLikeRubyResolver(t *testing.T) {
	var messages []string
	SetDebugHandler(func(message string) {
		messages = append(messages, message)
	})
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := networkingInterfacesForPlatform(testSession, "windows", func() ([]networkInterfaceSnapshot, error) {
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

func TestWindowsReleaseFinderMatchesRuby(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		consumer      bool
		description   string
		kernelVersion string
		version       string
		want          string
	}{
		{name: "missing version"},
		{name: "windows 10 consumer", consumer: true, kernelVersion: "10.0.123", version: "10.0", want: "10"},
		{name: "windows 11 consumer", consumer: true, kernelVersion: "10.0.22000", version: "10.0", want: "11"},
		{name: "windows server 2025", kernelVersion: "10.0.26100", version: "10.0", want: "2025"},
		{name: "windows server 2022", kernelVersion: "10.0.20348", version: "10.0", want: "2022"},
		{name: "windows server 2019", kernelVersion: "10.0.17623", version: "10.0", want: "2019"},
		{name: "windows server 2016", kernelVersion: "10.0.176", version: "10.0", want: "2016"},
		{name: "windows 8.1 consumer", consumer: true, version: "6.3", want: "8.1"},
		{name: "windows server 2012 r2", version: "6.3", want: "2012 R2"},
		{name: "windows xp consumer", consumer: true, version: "5.2", want: "XP"},
		{name: "windows server 2003", version: "5.2", want: "2003"},
		{name: "windows server 2003 r2", description: "R2", version: "5.2", want: "2003 R2"},
		{name: "unknown version falls back", description: "R2", version: "4.2", want: "4.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := windowsRelease(tt.version, tt.consumer, tt.description, tt.kernelVersion)
			if got != tt.want {
				t.Fatalf("windowsRelease() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentOSReleaseWindowsUsesKernelAndDescriptionData(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		if name != "wmic" {
			t.Fatalf("command = %q %v, want wmic", name, args)
		}
		return "OtherTypeDescription=\r\nProductType=1\r\nVersion=10.0.22631\r\n"
	}

	got := currentOSRelease(testSession, "windows", nil, run)
	want := map[string]any{"full": "11", "major": "11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentOSRelease(testSession, windows) = %#v, want %#v", got, want)
	}
}

func TestCurrentWindowsOSDescriptionMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  *windowsOSDescription
	}{
		{
			name:  "query returns no result",
			input: "",
		},
		{
			name:  "consumer release with empty description",
			input: "ProductType=1\r\nOtherTypeDescription=\r\n",
			want:  &windowsOSDescription{ConsumerRelease: true},
		},
		{
			name:  "missing product type keeps description and is not consumer",
			input: "ProductType=\r\nOtherTypeDescription=description\r\n",
			want:  &windowsOSDescription{Description: "description"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := currentWindowsOSDescription(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("currentWindowsOSDescription() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCurrentWindowsKernelFactsMatchRubyResolver(t *testing.T) {
	t.Parallel()

	got := currentWindowsKernelFacts("OtherTypeDescription=\r\nProductType=1\r\nVersion=10.0.22631\r\n")
	want := []ResolvedFact{
		{Name: "kernel", Value: "windows"},
		{Name: "kernelmajversion", Value: "10.0"},
		{Name: "kernelrelease", Value: "10.0.22631"},
		{Name: "kernelversion", Value: "10.0.22631"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentWindowsKernelFacts() = %#v, want %#v", got, want)
	}
}

func TestCurrentWindowsKernelFactsLogsFailureLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	SetDebugHandler(func(message string) { debugMessages = append(debugMessages, message) })
	t.Cleanup(func() { SetDebugHandler(nil) })

	if got := currentWindowsKernelFacts(""); got != nil {
		t.Fatalf("currentWindowsKernelFacts(empty) = %#v, want nil", got)
	}
	want := []string{"Calling Windows RtlGetVersion failed"}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestParseWindowsProductReleaseMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion`,
		`    EditionID    REG_SZ    ServerStandard`,
		`    InstallationType    REG_SZ    Server`,
		`    ProductName    REG_SZ    Windows Server 2022 Standard`,
		`    ReleaseId    REG_SZ    1809`,
		`    DisplayVersion    REG_SZ    21H2`,
	}, "\n")

	got := parseWindowsProductRelease(input)
	want := windowsProductRelease{
		EditionID:        "ServerStandard",
		InstallationType: "Server",
		ProductName:      "Windows Server 2022 Standard",
		ReleaseID:        "21H2",
		DisplayVersion:   "21H2",
	}
	if got != want {
		t.Fatalf("parseWindowsProductRelease() = %#v, want %#v", got, want)
	}
}

func TestParseWindowsProductReleaseFallsBackToReleaseID(t *testing.T) {
	t.Parallel()

	got := parseWindowsProductRelease("    ReleaseId    REG_SZ    1809\n")
	if got.ReleaseID != "1809" {
		t.Fatalf("ReleaseID = %q, want 1809", got.ReleaseID)
	}
	if got.DisplayVersion != "" {
		t.Fatalf("DisplayVersion = %q, want empty", got.DisplayVersion)
	}
}

func TestWindowsProductReleaseFactsReturnStructuredFacts(t *testing.T) {
	t.Parallel()

	core := windowsProductReleaseFacts(windowsProductRelease{
		EditionID:        "ServerStandard",
		InstallationType: "Server",
		ProductName:      "Windows Server 2022 Standard",
		ReleaseID:        "21H2",
		DisplayVersion:   "21H2",
	})

	if got := Collection(core); !reflect.DeepEqual(got, map[string]any{
		"os": map[string]any{"windows": map[string]any{
			"edition_id":        "ServerStandard",
			"installation_type": "Server",
			"product_name":      "Windows Server 2022 Standard",
			"release_id":        "21H2",
			"display_version":   "21H2",
		}},
	}) {
		t.Fatalf("core facts = %#v", got)
	}
}

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

	got := currentWindowsDMI("windows", run)
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
	SetDebugHandler(func(message string) { debugMessages = append(debugMessages, message) })
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := currentWindowsDMI("windows", func(string, ...string) string { return "" })
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

func TestParseWindowsProcessorsMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"Name=Pretty_Name",
		"Architecture=0",
		"NumberOfLogicalProcessors=2",
		"NumberOfCores=2",
	}, "\r\n")

	got := parseWindowsProcessors(input)
	want := processorInfo{
		ISA:            "x86",
		Models:         []string{"Pretty_Name"},
		LogicalCount:   2,
		PhysicalCount:  1,
		CoresPerSocket: 2,
		ThreadsPerCore: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWindowsProcessors() = %#v, want %#v", got, want)
	}
}

func TestParseWindowsProcessorsUsesRubyISATable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		architecture string
		want         string
	}{
		{architecture: "0", want: "x86"},
		{architecture: "1", want: "MIPS"},
		{architecture: "2", want: "Alpha"},
		{architecture: "3", want: "PowerPC"},
		{architecture: "5", want: "ARM"},
		{architecture: "6", want: "Itanium"},
		{architecture: "9", want: "x64"},
		{architecture: "12", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.architecture, func(t *testing.T) {
			input := strings.Join([]string{
				"Name=Pretty_Name",
				"Architecture=" + tt.architecture,
				"NumberOfLogicalProcessors=2",
				"NumberOfCores=2",
			}, "\r\n")

			got := parseWindowsProcessors(input)
			if got.ISA != tt.want {
				t.Fatalf("ISA = %q, want %q", got.ISA, tt.want)
			}
		})
	}
}

func TestParseWindowsProcessorsFallsBackWhenLogicalCountIsZero(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"Name=Pretty_Name",
		"Architecture=0",
		"NumberOfLogicalProcessors=0",
		"NumberOfCores=2",
		"",
		"Name=Awesome_Name",
		"Architecture=10",
		"NumberOfLogicalProcessors=0",
		"NumberOfCores=2",
	}, "\r\n")

	got := parseWindowsProcessors(input)
	want := processorInfo{
		ISA:            "x86",
		Models:         []string{"Pretty_Name", "Awesome_Name"},
		LogicalCount:   2,
		PhysicalCount:  2,
		CoresPerSocket: 2,
		ThreadsPerCore: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWindowsProcessors() = %#v, want %#v", got, want)
	}
}

func TestParseWindowsProcessorsLogsUnknownArchitectureLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	SetDebugHandler(func(message string) { debugMessages = append(debugMessages, message) })
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := parseWindowsProcessors("Name=Pretty_Name\r\nArchitecture=10\r\nNumberOfLogicalProcessors=2\r\nNumberOfCores=2\r\n")
	if got.ISA != "" {
		t.Fatalf("ISA = %q, want empty for unknown architecture", got.ISA)
	}
	want := []string{"Unable to determine processor type: unknown architecture"}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestCurrentWindowsProcessorsQueriesWMIC(t *testing.T) {
	t.Parallel()

	got := currentWindowsProcessors("windows", func(name string, args ...string) string {
		if name != "wmic" {
			t.Fatalf("command = %q %v, want wmic", name, args)
		}
		wantArgs := []string{"cpu", "get", "Name,Architecture,NumberOfLogicalProcessors,NumberOfCores", "/value"}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Fatalf("wmic args = %#v, want %#v", args, wantArgs)
		}
		return "Name=Pretty_Name\r\nArchitecture=0\r\nNumberOfLogicalProcessors=2\r\nNumberOfCores=2\r\n"
	})

	if got.LogicalCount != 2 || got.PhysicalCount != 1 || got.CoresPerSocket != 2 || got.ThreadsPerCore != 1 {
		t.Fatalf("currentWindowsProcessors() = %#v, want Ruby-compatible counts", got)
	}
	if got.ISA != "x86" {
		t.Fatalf("ISA = %q, want x86", got.ISA)
	}
	if !reflect.DeepEqual(got.Models, []string{"Pretty_Name"}) {
		t.Fatalf("Models = %#v, want Pretty_Name", got.Models)
	}
}

func TestCurrentWindowsProcessorsLogsNoResultDiagnosticsLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	SetDebugHandler(func(message string) { debugMessages = append(debugMessages, message) })
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := currentWindowsProcessors("windows", func(string, ...string) string { return "" })
	if !reflect.DeepEqual(got, processorInfo{}) {
		t.Fatalf("currentWindowsProcessors(empty WMI) = %#v, want empty processor info", got)
	}
	want := []string{"WMI query returned no resultsfor Win32_Processor with values Name, Architecture and NumberOfLogicalProcessors."}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestParseWindowsProcessorsOmitsUnknownISA(t *testing.T) {
	t.Parallel()

	got := parseWindowsProcessors("Name=Pretty_Name\r\nArchitecture=10\r\nNumberOfLogicalProcessors=2\r\nNumberOfCores=2\r\n")
	if got.ISA != "" {
		t.Fatalf("ISA = %q, want empty for unknown architecture", got.ISA)
	}
}

func TestCurrentWindowsProcessorsSkipsNonWindows(t *testing.T) {
	t.Parallel()

	got := currentWindowsProcessors("linux", func(string, ...string) string {
		t.Fatal("currentWindowsProcessors(non-windows) ran command")
		return ""
	})
	if !reflect.DeepEqual(got, processorInfo{}) {
		t.Fatalf("currentWindowsProcessors(linux) = %#v, want empty", got)
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

func TestCurrentWindowsSystem32MatchesRubyResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		goos       string
		systemRoot string
		isWOW64    bool
		wowOK      bool
		want       string
	}{
		{name: "wow64 process uses sysnative", goos: "windows", systemRoot: `C:\Windows`, isWOW64: true, wowOK: true, want: `C:\Windows\sysnative`},
		{name: "native process uses system32", goos: "windows", systemRoot: `C:\Windows`, wowOK: true, want: `C:\Windows\system32`},
		{name: "missing systemroot is empty", goos: "windows", wowOK: true},
		{name: "wow64 check failure is empty", goos: "windows", systemRoot: `C:\Windows`},
		{name: "non-windows is empty", goos: "linux", systemRoot: `C:\Windows`, wowOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := currentWindowsSystem32(tt.goos, tt.systemRoot, func() (bool, bool) {
				return tt.isWOW64, tt.wowOK
			})
			if got != tt.want {
				t.Fatalf("currentWindowsSystem32() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWindowsSystem32FactsReturnStructuredFacts(t *testing.T) {
	t.Parallel()

	core := windowsSystem32Facts(`C:\Windows\system32`)

	if got := Collection(core); !reflect.DeepEqual(got, map[string]any{
		"os": map[string]any{"windows": map[string]any{"system32": `C:\Windows\system32`}},
	}) {
		t.Fatalf("core facts = %#v", got)
	}
}

func TestWindowsTimezoneUsesAPICodepage(t *testing.T) {
	t.Parallel()

	got := currentWindowsTimezone("windows", "Central Europ\x82en Standard Time", "850", func() string {
		t.Fatal("registry codepage should not be used when API returns a value")
		return ""
	})

	if got != "Central Européen Standard Time" {
		t.Fatalf("currentWindowsTimezone() = %q, want %q", got, "Central Européen Standard Time")
	}
}

func TestWindowsTimezoneFallsBackToRegistryCodepage(t *testing.T) {
	t.Parallel()

	got := currentWindowsTimezone("windows", "Hora est\xa0ndar", "", func() string {
		return "850"
	})

	if got != "Hora estándar" {
		t.Fatalf("currentWindowsTimezone() = %q, want %q", got, "Hora estándar")
	}
}

func TestWindowsTimezoneKeepsOriginalForInvalidCodepage(t *testing.T) {
	t.Parallel()

	zone := "UTC"
	got := currentWindowsTimezone("windows", zone, "not-a-codepage", func() string { return "" })
	if got != zone {
		t.Fatalf("currentWindowsTimezone() = %q, want %q", got, zone)
	}
}

func TestCurrentWindowsTimezoneRunsOnlyOnWindows(t *testing.T) {
	t.Parallel()

	called := false
	got := currentWindowsTimezone("linux", "UTC", "850", func() string {
		called = true
		return "850"
	})
	if got != "" {
		t.Fatalf("currentWindowsTimezone(non-windows) = %q, want empty", got)
	}
	if called {
		t.Fatal("currentWindowsTimezone(non-windows) read registry codepage")
	}
}

func TestCurrentTimezoneLinuxMatchesRubyResolverFormat(t *testing.T) {
	t.Parallel()
	assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t, "linux")
}

func TestCurrentTimezoneDarwinMatchesRubyResolverFormat(t *testing.T) {
	t.Parallel()
	assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t, "darwin")
}

func TestCurrentTimezoneFreeBSDMatchesRubyResolverFormat(t *testing.T) {
	t.Parallel()
	assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t, "freebsd")
}

func assertCurrentTimezonePOSIXMatchesRubyResolverFormat(t *testing.T, goos string) {
	t.Helper()

	before := time.Now().Format("MST")
	got := currentTimezone(testSession, goos)
	after := time.Now().Format("MST")
	if got != before && got != after {
		t.Fatalf("currentTimezone(testSession, %s) = %q, want local timezone abbreviation %q or %q", goos, got, before, after)
	}
}

func TestParseWindowsMemoryMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"FreePhysicalMemory=1024",
		"TotalVisibleMemorySize=4096",
	}, "\n")

	got := parseWindowsMemory(input)
	want := windowsMemory{
		TotalBytes:     4096 * 1024,
		AvailableBytes: 1024 * 1024,
		UsedBytes:      3072 * 1024,
		Capacity:       "75.00%",
	}
	if got != want {
		t.Fatalf("parseWindowsMemory() = %#v, want %#v", got, want)
	}
}

func TestParseWindowsMemoryRejectsZeroValues(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"FreePhysicalMemory=1024\nTotalVisibleMemorySize=0\n",
		"FreePhysicalMemory=0\nTotalVisibleMemorySize=4096\n",
		"FreePhysicalMemory=bad\nTotalVisibleMemorySize=4096\n",
	} {
		if got := parseWindowsMemory(input); got != (windowsMemory{}) {
			t.Fatalf("parseWindowsMemory(%q) = %#v, want empty", input, got)
		}
	}
}

func TestParseWindowsMemoryLogsZeroValueDiagnosticLikeRubyResolver(t *testing.T) {
	var messages []string
	SetDebugHandler(func(message string) {
		messages = append(messages, message)
	})
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := parseWindowsMemory("FreePhysicalMemory=1024\nTotalVisibleMemorySize=0\n")
	if got != (windowsMemory{}) {
		t.Fatalf("parseWindowsMemory() = %#v, want empty", got)
	}

	wantMessages := []string{"Available or Total bytes are zero could not proceed further"}
	if !reflect.DeepEqual(messages, wantMessages) {
		t.Fatalf("debug messages = %#v, want %#v", messages, wantMessages)
	}
}

func TestParseWindowsMemoryLogsFailureDiagnosticLikeRubyResolver(t *testing.T) {
	var messages []string
	SetDebugHandler(func(message string) {
		messages = append(messages, message)
	})
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := parseWindowsMemory("")
	if got != (windowsMemory{}) {
		t.Fatalf("parseWindowsMemory() = %#v, want empty", got)
	}

	wantMessages := []string{"Resolving memory facts failed"}
	if !reflect.DeepEqual(messages, wantMessages) {
		t.Fatalf("debug messages = %#v, want %#v", messages, wantMessages)
	}
}

func TestCurrentWindowsMemoryRunsOnlyOnWindows(t *testing.T) {
	t.Parallel()

	called := false
	got := currentWindowsMemory("linux", func(name string, args ...string) string {
		called = true
		return ""
	})
	if got != (windowsMemory{}) {
		t.Fatalf("currentWindowsMemory(non-windows) = %#v, want empty", got)
	}
	if called {
		t.Fatal("currentWindowsMemory(non-windows) ran command")
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

func TestAddLinuxBondingSlaveMACsUsesPermanentHardwareAddress(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "proc/net/bonding/bond0"), strings.Join([]string{
		"Ethernet Channel Bonding Driver: v3.7.1 (April 27, 2011)",
		"",
		"Slave Interface: eth2",
		"Permanent HW addr: 08:00:27:29:dc:a5",
		"",
		"Slave Interface: eth3",
		"Permanent HW addr: 08:00:27:d5:44:7e",
	}, "\n"))
	interfaces := map[string]any{
		"bond0": map[string]any{"mac": "08:00:27:29:dc:a5"},
		"eth2":  map[string]any{"mac": "08:00:27:29:dc:a5"},
		"eth3":  map[string]any{"mac": "08:00:27:29:dc:a5"},
	}

	addLinuxBondingSlaveMACsFromRoot(root, interfaces)

	eth3 := interfaces["eth3"].(map[string]any)
	if got, want := eth3["mac"], "08:00:27:d5:44:7e"; got != want {
		t.Fatalf("eth3 mac = %#v, want %q", got, want)
	}
	if _, ok := interfaces["missing"]; ok {
		t.Fatalf("interfaces = %#v, want unknown bonding slaves ignored", interfaces)
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

func TestCurrentProcessorInfoWiresFreeBSDSysctlOutput(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	run := func(path string, args ...string) string {
		if path != "sysctl" || len(args) != 2 || args[0] != "-n" {
			t.Fatalf("run(%q, %#v), want sysctl -n <name>", path, args)
		}
		seen[args[1]] = true
		switch args[1] {
		case "hw.ncpu":
			return "2\n"
		case "hw.model":
			return "Intel(r) Xeon(r) Gold 6138 CPU @ 2.00GHz\n"
		case "hw.clockrate":
			return "2592\n"
		default:
			t.Fatalf("unexpected sysctl name %q", args[1])
			return ""
		}
	}

	got := currentProcessorInfo("freebsd", run)
	wantModels := []string{
		"Intel(r) Xeon(r) Gold 6138 CPU @ 2.00GHz",
		"Intel(r) Xeon(r) Gold 6138 CPU @ 2.00GHz",
	}
	if got.LogicalCount != 2 || got.SpeedHz != 2592000000 || !reflect.DeepEqual(got.Models, wantModels) {
		t.Fatalf("currentProcessorInfo() = %#v", got)
	}
	for _, name := range []string{"hw.ncpu", "hw.model", "hw.clockrate"} {
		if !seen[name] {
			t.Fatalf("currentProcessorInfo() did not query %s", name)
		}
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

func TestFIPSEnabledReadsProcFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "enabled", content: "1\n", want: true},
		{name: "disabled", content: "0\n", want: false},
		{name: "unexpected", content: "enabled\n", want: false},
		{name: "whitespace", content: " 1 \n", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "fips_enabled")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}

			if got := fipsEnabled(path); got != tt.want {
				t.Fatalf("fipsEnabled() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestFIPSEnabledMissingFileIsFalse(t *testing.T) {
	t.Parallel()

	if got := fipsEnabled(filepath.Join(t.TempDir(), "missing")); got {
		t.Fatalf("fipsEnabled() = %t, want false", got)
	}
}

func TestKernelVersionFactMatchesRubyPlatformBehavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		goos          string
		kernelRelease string
		unameVersion  string
		want          string
	}{
		{
			name:          "linux trims package suffix after semantic kernel version",
			goos:          "linux",
			kernelRelease: "4.11.5-19-generic",
			want:          "4.11.5",
		},
		{
			name:          "linux falls back to leading major digits",
			goos:          "linux",
			kernelRelease: "4test",
			want:          "4",
		},
		{
			name:          "darwin uses kernel release",
			goos:          "darwin",
			kernelRelease: "18.7.0",
			unameVersion:  "Darwin Kernel Version 18.7.0: root:xnu",
			want:          "18.7.0",
		},
		{
			name:          "bsd trims release to major minor",
			goos:          "freebsd",
			kernelRelease: "12.1-RELEASE-p3",
			want:          "12.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := kernelVersionFact(tt.goos, tt.kernelRelease, tt.unameVersion); got != tt.want {
				t.Fatalf("kernelVersionFact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAioAgentVersionReadsLeadingVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "three segments", content: "7.0.1\n", want: "7.0.1"},
		{name: "four segments", content: "7.0.1.8\n", want: "7.0.1.8"},
		{name: "dev build", content: "7.0.1.8.g12345678\n", want: "7.0.1.8"},
		{name: "numeric dev build suffix", content: "7.0.1.8.42 build metadata\n", want: "7.0.1.8"},
		{name: "suffix", content: "7.0.1-rc1\n", want: "7.0.1"},
		{name: "invalid", content: "puppet-agent 7.0.1\n", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "VERSION")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}

			if got := aioAgentVersion(path); got != tt.want {
				t.Fatalf("aioAgentVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAioAgentVersionMissingFileIsEmpty(t *testing.T) {
	t.Parallel()

	if got := aioAgentVersion(filepath.Join(t.TempDir(), "missing")); got != "" {
		t.Fatalf("aioAgentVersion() = %q, want empty", got)
	}
}

func TestCurrentAioAgentVersionWindowsUsesRegistryInstallDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("7.0.1.8.g12345678\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(name string, args ...string) string {
		if name != "reg" || len(args) != 4 || args[0] != "query" || args[1] != `HKLM\SOFTWARE\Puppet Labs\Puppet` || args[2] != "/v" {
			t.Fatalf("unexpected command: %s %v", name, args)
		}
		switch args[3] {
		case "RememberedInstallDir64":
			return "    RememberedInstallDir64    REG_SZ    " + dir + "\n"
		case "RememberedInstallDir":
			t.Fatal("did not expect 32-bit fallback when 64-bit path exists")
		}
		return ""
	}

	if got := currentAioAgentVersion("windows", run); got != "7.0.1.8" {
		t.Fatalf("currentAioAgentVersion() = %q, want %q", got, "7.0.1.8")
	}
}

func TestCurrentAioAgentVersionWindowsFallsBackTo32BitRegistry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("7.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(name string, args ...string) string {
		if args[3] == "RememberedInstallDir" {
			return "    RememberedInstallDir    REG_SZ    " + dir + "\n"
		}
		return ""
	}

	var messages []string
	SetDebugHandler(func(message string) {
		messages = append(messages, message)
	})
	t.Cleanup(func() { SetDebugHandler(nil) })

	if got := currentAioAgentVersion("windows", run); got != "7.0.1" {
		t.Fatalf("currentAioAgentVersion() = %q, want %q", got, "7.0.1")
	}

	wantMessages := []string{"Could not read Puppet AIO path from 64 bit registry"}
	if !reflect.DeepEqual(messages, wantMessages) {
		t.Fatalf("debug messages = %#v, want %#v", messages, wantMessages)
	}
}

func TestCurrentAioAgentVersionWindowsEmpty64BitRegistryDoesNotFallBack(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		if args[3] == "RememberedInstallDir64" {
			return "    RememberedInstallDir64    REG_SZ    \n"
		}
		t.Fatal("did not expect 32-bit fallback when 64-bit path is present but empty")
		return ""
	}

	if got := currentAioAgentVersion("windows", run); got != "" {
		t.Fatalf("currentAioAgentVersion() = %q, want empty", got)
	}
}

func TestCurrentAioAgentVersionWindowsLogsMissing32BitRegistryLikeRubyResolver(t *testing.T) {
	run := func(string, ...string) string {
		return ""
	}

	var messages []string
	SetDebugHandler(func(message string) {
		messages = append(messages, message)
	})
	t.Cleanup(func() { SetDebugHandler(nil) })

	if got := currentAioAgentVersion("windows", run); got != "" {
		t.Fatalf("currentAioAgentVersion() = %q, want empty", got)
	}

	wantMessages := []string{
		"Could not read Puppet AIO path from 64 bit registry",
		"Could not read Puppet AIO path from 32 bit registry",
	}
	if !reflect.DeepEqual(messages, wantMessages) {
		t.Fatalf("debug messages = %#v, want %#v", messages, wantMessages)
	}
}

func TestRubyFactsIncludeStructuredRuntimeFacts(t *testing.T) {
	t.Parallel()

	core := rubyFacts(rubyInfo{
		Version:  "3.3.0",
		Platform: "arm64-darwin23",
		Sitedir:  "/opt/puppetlabs/puppet/lib/ruby/site_ruby/3.3.0",
	})

	collection := Collection(core)
	ruby, ok := collection["ruby"].(map[string]any)
	if !ok {
		t.Fatalf("ruby fact = %#v, want structured ruby fact", collection["ruby"])
	}
	for _, tt := range []struct {
		name string
		want string
	}{
		{name: "version", want: "3.3.0"},
		{name: "platform", want: "arm64-darwin23"},
		{name: "sitedir", want: "/opt/puppetlabs/puppet/lib/ruby/site_ruby/3.3.0"},
	} {
		if got := ruby[tt.name]; got != tt.want {
			t.Fatalf("ruby.%s = %#v, want %#v", tt.name, got, tt.want)
		}
	}
}

func TestParseRubyInfoRequiresRubySitedirGuard(t *testing.T) {
	t.Parallel()

	info := parseRubyInfo("3.3.0\narm64-darwin23\n\n/opt/puppetlabs/puppet/lib/ruby/site_ruby/3.3.0\n")

	if info.Version != "3.3.0" {
		t.Fatalf("Version = %q, want %q", info.Version, "3.3.0")
	}
	if info.Platform != "arm64-darwin23" {
		t.Fatalf("Platform = %q, want %q", info.Platform, "arm64-darwin23")
	}
	if info.Sitedir != "" {
		t.Fatalf("Sitedir = %q, want empty when Ruby sitedir guard is missing", info.Sitedir)
	}
}

func TestSELinuxFactsReadsConfigAndMountpoint(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mountpoint := filepath.Join(dir, "selinux")
	if err := os.Mkdir(mountpoint, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "mounts"), "none "+mountpoint+" selinuxfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enforcing\nSELINUXTYPE=targeted\n")
	writeFile(t, filepath.Join(mountpoint, "enforce"), "1")
	writeFile(t, filepath.Join(mountpoint, "policyvers"), "33")

	core := selinuxFacts(filepath.Join(dir, "mounts"), filepath.Join(dir, "config"))
	collection := Collection(core)
	if got, want := collection["os"].(map[string]any)["selinux"], map[string]any{
		"config_mode":    "enforcing",
		"config_policy":  "targeted",
		"current_mode":   "enforcing",
		"enabled":        true,
		"enforced":       true,
		"policy_version": "33",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selinuxFacts() core = %#v, want %#v", got, want)
	}
}

func TestSELinuxFactsDisabledWithoutMountpointOrConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mounts"), "rootfs / rootfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enforcing\n")

	core := selinuxFacts(filepath.Join(dir, "mounts"), filepath.Join(dir, "config"))
	if got := Collection(core)["os"].(map[string]any)["selinux"].(map[string]any)["enabled"]; got != false {
		t.Fatalf("os.selinux.enabled = %#v, want false", got)
	}

	writeFile(t, filepath.Join(dir, "mounts"), "none /sys/fs/selinux selinuxfs rw 0 0\n")
	core = selinuxFacts(filepath.Join(dir, "mounts"), filepath.Join(dir, "missing-config"))
	if got := Collection(core)["os"].(map[string]any)["selinux"].(map[string]any)["enabled"]; got != false {
		t.Fatalf("os.selinux.enabled = %#v, want false without config", got)
	}
}

func TestSELinuxFactsForPlatform_omittedOutsideLinux(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mounts"), "none /sys/fs/selinux selinuxfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enforcing\nSELINUXTYPE=targeted\n")

	for _, goos := range []string{"darwin", "freebsd", "openbsd", "windows"} {
		if got := selinuxFactsForPlatform(goos, filepath.Join(dir, "mounts"), filepath.Join(dir, "config")); got != nil {
			t.Fatalf("selinuxFactsForPlatform(%s) = %#v, want nil", goos, got)
		}
	}
}

func TestSELinuxFactsForPlatform_resolvesOnLinux(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mounts"), "rootfs / rootfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enforcing\n")

	core := selinuxFactsForPlatform("linux", filepath.Join(dir, "mounts"), filepath.Join(dir, "config"))
	if got := Collection(core)["os"].(map[string]any)["selinux"].(map[string]any)["enabled"]; got != false {
		t.Fatalf("os.selinux.enabled = %#v, want false on Linux without selinuxfs", got)
	}
}

func TestSELinuxFactsKeepsMissingPolicyVersionNil(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mountpoint := filepath.Join(dir, "selinux")
	writeFile(t, filepath.Join(dir, "mounts"), "none "+mountpoint+" selinuxfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enabled\nSELINUXTYPE=targeted\n")
	writeFile(t, filepath.Join(mountpoint, "enforce"), "")

	core := selinuxFacts(filepath.Join(dir, "mounts"), filepath.Join(dir, "config"))

	if got := Collection(core)["os"].(map[string]any)["selinux"].(map[string]any)["policy_version"]; got != nil {
		t.Fatalf("os.selinux.policy_version = %#v, want nil", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
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

func TestPartitionsFactAddsFirstMountpointForDevice(t *testing.T) {
	partitions := map[string]any{
		"/dev/sda2": map[string]any{
			"filesystem": "btrfs",
			"size":       "13.09 GiB",
			"size_bytes": 14050918400,
			"uuid":       "bbc18fba-8191-48c8-b8bd-30373654bb3e",
		},
	}
	mountpoints := map[string]any{
		"/": map[string]any{
			"device": "/dev/sda2",
		},
		"/boot/grub2/x86_64-efi": map[string]any{
			"device": "/dev/sda2",
		},
	}

	got := partitionsFact(partitions, mountpoints)
	want := map[string]any{
		"/dev/sda2": map[string]any{
			"filesystem": "btrfs",
			"mount":      "/",
			"size":       "13.09 GiB",
			"size_bytes": 14050918400,
			"uuid":       "bbc18fba-8191-48c8-b8bd-30373654bb3e",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionsFact() = %#v, want %#v", got, want)
	}
}

func TestPartitionsFactWithMountEntriesUsesResolverOrderForDuplicateDeviceLikeRuby(t *testing.T) {
	partitions := map[string]any{
		"/dev/sda2": map[string]any{
			"filesystem": "btrfs",
			"size":       "13.09 GiB",
			"size_bytes": 14050918400,
		},
	}
	mountEntries := []mountEntry{
		{Device: "/dev/sda2", Path: "/z-first", Filesystem: "btrfs"},
		{Device: "/dev/sda2", Path: "/a-second", Filesystem: "btrfs"},
	}
	mountpoints := map[string]any{
		"/a-second": map[string]any{"device": "/dev/sda2"},
		"/z-first":  map[string]any{"device": "/dev/sda2"},
	}

	got := partitionsFactWithMountEntries(partitions, mountEntries, mountpoints)
	partition, ok := got["/dev/sda2"].(map[string]any)
	if !ok {
		t.Fatalf("partitionsFactWithMountEntries() = %#v, want /dev/sda2 partition", got)
	}
	if partition["mount"] != "/z-first" {
		t.Fatalf("partition mount = %#v, want first resolver mountpoint /z-first", partition["mount"])
	}
}

func TestPartitionsFactReturnsPartitionsWithoutMountpoints(t *testing.T) {
	partitions := map[string]any{
		"/dev/sda1": map[string]any{"filesystem": "ext3"},
	}

	got := partitionsFact(partitions, nil)
	if !reflect.DeepEqual(got, partitions) {
		t.Fatalf("partitionsFact() = %#v, want %#v", got, partitions)
	}
}

func TestPartitionsFactReturnsNilForEmptyPartitions(t *testing.T) {
	if got := partitionsFact(map[string]any{}, map[string]any{"/": map[string]any{"device": "/dev/sda1"}}); got != nil {
		t.Fatalf("partitionsFact() = %#v, want nil", got)
	}
}

func TestDiscoverPartitionsReadsSysfsPartitionEntries(t *testing.T) {
	root := t.TempDir()
	partitionDir := filepath.Join(root, "sda1")
	diskDir := filepath.Join(root, "sda")
	if err := os.Mkdir(partitionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(diskDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partitionDir, "partition"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partitionDir, "size"), []byte("4096\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(diskDir, "size"), []byte("8192\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := discoverPartitions(root)
	want := map[string]any{
		"/dev/sda1": map[string]any{"size": "2.00 MiB", "size_bytes": 2097152},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverPartitions() = %#v, want %#v", got, want)
	}
}

func TestCurrentLinuxPartitionsAddsLSBLKParttypeLikeRubyResolver(t *testing.T) {
	root := t.TempDir()
	partitionDir := filepath.Join(root, "sda1")
	if err := os.Mkdir(partitionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(partitionDir, "partition"), "1\n")
	writeFile(t, filepath.Join(partitionDir, "size"), "234\n")

	run := func(name string, args ...string) string {
		if name != "lsblk" {
			t.Fatalf("run(%q, %#v), want lsblk", name, args)
		}
		switch strings.Join(args, " ") {
		case "--version":
			return "lsblk from util-linux 2.25\n"
		case "-p -P -o NAME,FSTYPE,UUID,LABEL,PARTUUID,PARTLABEL,PARTTYPE":
			return `NAME="/dev/sda1" FSTYPE="ext3" UUID="88077904-4fd4-476f-9af2-0f7a806ca25e" LABEL="/boot" PARTUUID="00061fe0-01" PARTLABEL="" PARTTYPE="21686148-6449-6E6F-744E-656564454649"` + "\n"
		default:
			t.Fatalf("unexpected lsblk args %#v", args)
			return ""
		}
	}

	got := currentLinuxPartitions(root, run)
	want := map[string]any{
		"/dev/sda1": map[string]any{
			"filesystem": "ext3",
			"label":      "/boot",
			"parttype":   "21686148-6449-6E6F-744E-656564454649",
			"partuuid":   "00061fe0-01",
			"size":       "117.00 KiB",
			"size_bytes": 119808,
			"uuid":       "88077904-4fd4-476f-9af2-0f7a806ca25e",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxPartitions() = %#v, want %#v", got, want)
	}
}

func TestDiscoverPartitionsHandlesDMAndLoopDevicesLikeRubyResolver(t *testing.T) {
	root := t.TempDir()
	dmDir := filepath.Join(root, "dm-0")
	loopDir := filepath.Join(root, "loop0")
	if err := os.MkdirAll(filepath.Join(dmDir, "dm"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(loopDir, "loop"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dmDir, "dm", "name"), "VolGroup00-LogVol00\n")
	writeFile(t, filepath.Join(dmDir, "size"), "201213\n")
	writeFile(t, filepath.Join(loopDir, "loop", "backing_file"), "some_path\n")
	writeFile(t, filepath.Join(loopDir, "size"), "234\n")

	got := discoverPartitions(root)
	want := map[string]any{
		"/dev/mapper/VolGroup00-LogVol00": map[string]any{"size": "98.25 MiB", "size_bytes": 103021056},
		"/dev/loop0":                      map[string]any{"backing_file": "some_path", "size": "117.00 KiB", "size_bytes": 119808},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverPartitions() = %#v, want %#v", got, want)
	}
}

func TestParseFreeBSDGeomPartitions_returnsRubyCompatiblePartitionFacts(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("testdata", "kern.geom.confxml"))
	if err != nil {
		t.Fatal(err)
	}

	got := parseFreeBSDGeomPartitions(string(input))
	want := map[string]any{
		"ada0p1": map[string]any{
			"partlabel":  "gptboot0",
			"partuuid":   "503d3458-c135-11e8-bd11-7d7cd061b26f",
			"size":       "512.00 KiB",
			"size_bytes": 524288,
		},
		"ada0p2": map[string]any{
			"partlabel":  "swap0",
			"partuuid":   "5048d40d-c135-11e8-bd11-7d7cd061b26f",
			"size":       "2.00 GiB",
			"size_bytes": 2147483648,
		},
		"ada0p3": map[string]any{
			"partlabel":  "zfs0",
			"partuuid":   "504f1547-c135-11e8-bd11-7d7cd061b26f",
			"size":       "474.94 GiB",
			"size_bytes": 509961306112,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFreeBSDGeomPartitions() = %#v, want %#v", got, want)
	}
}

func TestParseFreeBSDGeomDisks_returnsRubyCompatibleDiskFacts(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("testdata", "kern.geom.confxml"))
	if err != nil {
		t.Fatal(err)
	}

	got := parseFreeBSDGeomDisks(string(input))
	want := map[string]any{
		"ada0": map[string]any{
			"model":         "Samsung SSD 850 PRO 512GB",
			"serial_number": "S250NXAG959927J",
			"size":          "476.94 GiB",
			"size_bytes":    512110190592,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFreeBSDGeomDisks() = %#v, want %#v", got, want)
	}
}

func TestParseSSHHostPublicKeyBuildsStructuredFacts(t *testing.T) {
	entry, ok := parseSSHHostPublicKey("ssh-rsa YWJj root@example")
	if !ok {
		t.Fatal("parseSSHHostPublicKey() ok = false, want true")
	}
	if got, want := entry.Name, "rsa"; got != want {
		t.Fatalf("entry.Name = %q, want %q", got, want)
	}
	if got, want := entry.Type, "ssh-rsa"; got != want {
		t.Fatalf("entry.Type = %q, want %q", got, want)
	}
	if got, want := entry.Key, "YWJj"; got != want {
		t.Fatalf("entry.Key = %q, want %q", got, want)
	}
	if got, want := entry.SHA1, "SSHFP 1 1 a9993e364706816aba3e25717850c26c9cd0d89d"; got != want {
		t.Fatalf("entry.SHA1 = %q, want %q", got, want)
	}
	if got, want := entry.SHA256, "SSHFP 1 2 ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"; got != want {
		t.Fatalf("entry.SHA256 = %q, want %q", got, want)
	}

	collection := Collection(sshFacts([]sshHostKey{entry}))
	ssh, ok := collection["ssh"].(map[string]any)
	if !ok {
		t.Fatalf("ssh fact = %#v, want map", collection["ssh"])
	}
	rsa, ok := ssh["rsa"].(map[string]any)
	if !ok {
		t.Fatalf("ssh.rsa = %#v, want map", ssh["rsa"])
	}
	fingerprints, ok := rsa["fingerprints"].(map[string]any)
	if !ok {
		t.Fatalf("ssh.rsa.fingerprints = %#v, want map", rsa["fingerprints"])
	}
	if fingerprints["sha1"] != entry.SHA1 || fingerprints["sha256"] != entry.SHA256 {
		t.Fatalf("fingerprints = %#v, want sha1/sha256", fingerprints)
	}
	for _, name := range []string{"sshrsakey", "sshfp_rsa", "sshfp_rsa_algorithm"} {
		if got, ok := collection[name]; ok {
			t.Fatalf("%s = %#v, want no legacy alias fact", name, got)
		}
	}
}

func TestParseSSHHostPublicKeyIgnoresNonBase64CharactersForFingerprints(t *testing.T) {
	entry, ok := parseSSHHostPublicKey("ssh-rsa -_YWJj root@example")
	if !ok {
		t.Fatal("parseSSHHostPublicKey() ok = false, want true")
	}

	if got, want := entry.Key, "-_YWJj"; got != want {
		t.Fatalf("entry.Key = %q, want original key %q", got, want)
	}
	if got, want := entry.SHA1, "SSHFP 1 1 a9993e364706816aba3e25717850c26c9cd0d89d"; got != want {
		t.Fatalf("entry.SHA1 = %q, want %q", got, want)
	}
	if got, want := entry.SHA256, "SSHFP 1 2 ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"; got != want {
		t.Fatalf("entry.SHA256 = %q, want %q", got, want)
	}
}

func TestDiscoverSSHHostKeysLinuxSearchesRubyPathsAndOrder(t *testing.T) {
	t.Parallel()
	assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t, "linux")
}

func TestDiscoverSSHHostKeysDarwinSearchesRubyPathsAndOrder(t *testing.T) {
	t.Parallel()
	assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t, "darwin")
}

func TestDiscoverSSHHostKeysFreeBSDSearchesRubyPathsAndOrder(t *testing.T) {
	t.Parallel()
	assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t, "freebsd")
}

func assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t *testing.T, goos string) {
	t.Helper()

	readFile := func(path string) ([]byte, error) {
		switch path {
		case filepath.Join("/etc", "ssh_host_rsa_key.pub"):
			return []byte("ssh-rsa YWJj root@example"), nil
		case filepath.Join("/etc", "ssh_host_ecdsa_key.pub"):
			return []byte("ecdsa-sha2-nistp256 ZGVm root@example"), nil
		case filepath.Join("/etc/opt/ssh", "ssh_host_ed25519_key.pub"):
			return []byte("ssh-ed25519 Z2hp root@example"), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	got := discoverSSHHostKeysForPlatform(goos, "", readFile)
	want := []struct {
		name string
		typ  string
		key  string
	}{
		{name: "rsa", typ: "ssh-rsa", key: "YWJj"},
		{name: "ecdsa", typ: "ecdsa-sha2-nistp256", key: "ZGVm"},
		{name: "ed25519", typ: "ssh-ed25519", key: "Z2hp"},
	}
	if len(got) != len(want) {
		t.Fatalf("discoverSSHHostKeysForPlatform(%s) returned %d keys, want %d: %#v", goos, len(got), len(want), got)
	}
	for i, wantKey := range want {
		if got[i].Name != wantKey.name || got[i].Type != wantKey.typ || got[i].Key != wantKey.key {
			t.Fatalf("key %d = %#v, want name=%q type=%q key=%q", i, got[i], wantKey.name, wantKey.typ, wantKey.key)
		}
		if got[i].SHA1 == "" || got[i].SHA256 == "" {
			t.Fatalf("key %d = %#v, want populated SSHFP fingerprints", i, got[i])
		}
	}
}

func TestDiscoverSSHHostKeysWindowsReadsProgramDataSSH(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		switch path {
		case filepath.Join(`C:\ProgramData`, "ssh", "ssh_host_rsa_key.pub"):
			return []byte("ssh-rsa YWJj root@example"), nil
		case filepath.Join(`C:\ProgramData`, "ssh", "ssh_host_ecdsa_key.pub"):
			return []byte("ecdsa-sha2-nistp256 ZGVm root@example"), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	keys := discoverSSHHostKeysForPlatform("windows", `C:\ProgramData`, readFile)

	if len(keys) != 2 {
		t.Fatalf("discoverSSHHostKeysForPlatform() returned %d keys, want 2: %#v", len(keys), keys)
	}
	if keys[0].Name != "rsa" || keys[1].Name != "ecdsa" {
		t.Fatalf("key order = %#v, want rsa then ecdsa", keys)
	}
}

func TestSSHFactsWindowsUnprivilegedSkipsDiscovery(t *testing.T) {
	t.Parallel()

	called := false
	facts := sshFactsForPlatformWithPrivilege("windows", false, func() []sshHostKey {
		called = true
		return []sshHostKey{{Name: "rsa", Type: "ssh-rsa", Key: "YWJj"}}
	})

	if called {
		t.Fatal("unprivileged Windows SSH fact collection discovered host keys")
	}
	collection := Collection(facts)
	if got := collection["ssh"]; got != nil {
		t.Fatalf("ssh = %#v, want nil", got)
	}
	for _, name := range []string{"sshrsakey", "sshfp_rsa"} {
		if got, ok := collection[name]; ok {
			t.Fatalf("%s = %#v, want no legacy alias fact", name, got)
		}
	}
}

func TestSSHFactsOpenBSDEmptyResolverReturnsEmptyStructuredFact(t *testing.T) {
	t.Parallel()

	collection := Collection(sshFactsForPlatform("openbsd", nil))
	got, ok := collection["ssh"].(map[string]any)
	if !ok {
		t.Fatalf("ssh = %#v, want empty map", collection["ssh"])
	}
	if len(got) != 0 {
		t.Fatalf("ssh = %#v, want empty map", got)
	}
}

func TestSSHFactsOpenBSDReturnsStructuredFacts(t *testing.T) {
	t.Parallel()

	keys := []sshHostKey{
		{Name: "ecdsa", Type: "ecdsa", Key: "test", SHA1: "sha11", SHA256: "sha2561"},
		{Name: "rsa", Type: "rsa", Key: "test", SHA1: "sha12", SHA256: "sha2562"},
	}
	collection := Collection(sshFactsForPlatform("openbsd", keys))

	ssh, ok := collection["ssh"].(map[string]any)
	if !ok {
		t.Fatalf("ssh = %#v, want map", collection["ssh"])
	}
	for _, key := range keys {
		entry, ok := ssh[key.Name].(map[string]any)
		if !ok {
			t.Fatalf("ssh.%s = %#v, want map", key.Name, ssh[key.Name])
		}
		fingerprints, ok := entry["fingerprints"].(map[string]any)
		if !ok {
			t.Fatalf("ssh.%s.fingerprints = %#v, want map", key.Name, entry["fingerprints"])
		}
		if fingerprints["sha1"] != key.SHA1 || fingerprints["sha256"] != key.SHA256 {
			t.Fatalf("ssh.%s.fingerprints = %#v, want sha1/sha256", key.Name, fingerprints)
		}
		if got := entry["key"]; got != key.Key {
			t.Fatalf("ssh.%s.key = %#v, want %q", key.Name, got, key.Key)
		}
		if got := entry["type"]; got != key.Type {
			t.Fatalf("ssh.%s.type = %#v, want %q", key.Name, got, key.Type)
		}
		for _, name := range []string{"ssh" + key.Name + "key", "sshfp_" + key.Name} {
			if got, ok := collection[name]; ok {
				t.Fatalf("%s = %#v, want no legacy alias fact", name, got)
			}
		}
	}
}

func TestParseSSHHostPublicKeyRejectsUnknownOrInvalidKeys(t *testing.T) {
	for _, line := range []string{
		"ssh-unknown YWJj root@example",
		"ssh-rsa not-base64 root@example",
		"ssh-rsa -- root@example",
		"ssh-rsa",
		"",
	} {
		t.Run(strings.ReplaceAll(line, " ", "_"), func(t *testing.T) {
			if entry, ok := parseSSHHostPublicKey(line); ok {
				t.Fatalf("parseSSHHostPublicKey(%q) = %#v, true; want false", line, entry)
			}
		})
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

	gotFQDN, gotDomain := currentHostnameFQDNAndDomain("darwin", "foo", "foo", resolvConfPath)
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
	SetDebugHandler(func(message string) { debugMessages = append(debugMessages, message) })
	t.Cleanup(func() { SetDebugHandler(nil) })

	hostname, value := hostNameFromLookup(func() (string, error) {
		return "", errors.New("hostname unavailable")
	})

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
	)

	if hostname != "kernel-host" {
		t.Fatalf("hostname = %q, want kernel-host", hostname)
	}
	if value != "kernel-host" {
		t.Fatalf("hostname fact value = %#v, want kernel-host", value)
	}
}

// legacyAliasNames is the Ruby Facter legacy classification (Ruby 4.10.0
// --show-legacy output minus its default output): flat aliases of structured
// facts that must never resolve in Facts.
var legacyAliasNames = []string{
	"architecture", "augeasversion", "dhcp_servers", "domain", "fqdn", "gid",
	"hardwareisa", "hardwaremodel", "hostname", "id", "interfaces",
	"ipaddress", "ipaddress6", "macaddress", "macosx_buildversion",
	"macosx_productname", "macosx_productversion",
	"macosx_productversion_major", "macosx_productversion_minor",
	"macosx_productversion_patch", "manufacturer", "memoryfree",
	"memoryfree_mb", "memorysize", "memorysize_mb", "netmask", "netmask6",
	"network", "network6", "operatingsystem", "operatingsystemmajrelease",
	"operatingsystemrelease", "osfamily", "physicalprocessorcount",
	"processorcount", "productname", "rubyplatform", "rubysitedir",
	"rubyversion", "scope6", "serialnumber", "sshecdsakey", "sshed25519key",
	"sshfp_ecdsa", "sshfp_ed25519", "sshfp_rsa", "sshrsakey", "swapencrypted",
	"swapfree", "swapfree_mb", "swapsize", "swapsize_mb", "system32",
	"uptime", "uptime_days", "uptime_hours", "uptime_seconds", "uuid",
	"xendomains",
}

// legacyAliasPrefixes covers the per-device legacy alias families
// (ipaddress_en0, mtu_lo0, processor0, sp_boot_mode, blockdevice_sda_model, …).
var legacyAliasPrefixes = []string{
	"blockdevice", "bios_", "board", "chassis", "ipaddress_", "ipaddress6_",
	"lsb", "macaddress_", "mtu_", "netmask_", "netmask6_", "network_",
	"network6_", "processor0", "processor1", "processor2", "processor3",
	"processor4", "processor5", "processor6", "processor7", "processor8",
	"processor9", "scope6_", "selinux", "sp_", "windows_", "zfs_", "zpool_",
}

func assertNoLegacyAliases(t *testing.T, collection map[string]any) {
	t.Helper()
	for _, name := range legacyAliasNames {
		if got, ok := collection[name]; ok {
			t.Errorf("%s = %#v, want no legacy alias fact", name, got)
		}
	}
	for name := range collection {
		for _, prefix := range legacyAliasPrefixes {
			if strings.HasPrefix(name, prefix) {
				t.Errorf("%s = %#v, want no legacy alias fact (prefix %s)", name, collection[name], prefix)
			}
		}
	}
}

func TestCoreFacts_excludeAllLegacyAliases(t *testing.T) {
	assertNoLegacyAliases(t, Collection(CoreFacts(testSession)))
}

func TestArchitectureName_matchesRubyFacterUnameCompatibility(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		machine string
		want    string
	}{
		{name: "linux amd64 uname", goos: "linux", machine: "x86_64", want: "x86_64"},
		{name: "linux i686 normalized", goos: "linux", machine: "i686", want: "i386"},
		{name: "macos arm", goos: "darwin", machine: "arm64", want: "arm64"},
		{name: "missing machine falls back", goos: "linux", machine: "", want: runtime.GOARCH},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := architectureName(tt.goos, tt.machine); got != tt.want {
				t.Fatalf("architectureName(%q, %q) = %q, want %q", tt.goos, tt.machine, got, tt.want)
			}
		})
	}
}

func TestWindowsHardwareArchitecture_matchesRubyResolver(t *testing.T) {
	tests := []struct {
		name             string
		processor        string
		level            int
		wantHardware     string
		wantArchitecture string
	}{
		{name: "amd64", processor: "AMD64", wantHardware: "x86_64", wantArchitecture: "x64"},
		{name: "arm", processor: "ARM", wantHardware: "arm", wantArchitecture: "arm"},
		{name: "ia64", processor: "IA64", wantHardware: "ia64", wantArchitecture: "ia64"},
		{name: "intel level below 5", processor: "INTEL", level: 4, wantHardware: "i486", wantArchitecture: "x86"},
		{name: "intel level above 5", processor: "INTEL", level: 8, wantHardware: "i686", wantArchitecture: "x86"},
		{name: "unknown", wantHardware: "unknown", wantArchitecture: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hardware, architecture := windowsHardwareArchitecture(tt.processor, tt.level)
			if hardware != tt.wantHardware || architecture != tt.wantArchitecture {
				t.Fatalf("windowsHardwareArchitecture(%q, %d) = %q, %q, want %q, %q", tt.processor, tt.level, hardware, architecture, tt.wantHardware, tt.wantArchitecture)
			}
		})
	}
}

func TestWindowsOSNameFamilyHardwareAndArchitectureMatchRubyFacts(t *testing.T) {
	hardware, architecture := windowsHardwareArchitecture("AMD64", 0)

	if got := osName("windows", linuxDistro{}); got != "windows" {
		t.Fatalf("osName(windows) = %q, want windows", got)
	}
	if got := osFamily("windows", linuxDistro{}); got != "windows" {
		t.Fatalf("osFamily(windows) = %q, want windows", got)
	}
	if hardware != "x86_64" {
		t.Fatalf("hardware = %q, want x86_64", hardware)
	}
	if architecture != "x64" {
		t.Fatalf("architecture = %q, want x64", architecture)
	}
	if got := architectureName("windows", hardware); got != architecture {
		t.Fatalf("architectureName(windows, %q) = %q, want %q", hardware, got, architecture)
	}
}

func TestCurrentProcessorISAUsesOpenBSDUnameProcessor(t *testing.T) {
	got := currentProcessorISA(testSession, "openbsd", "amd64", func(name string, args ...string) string {
		if name != "uname" || !reflect.DeepEqual(args, []string{"-p"}) {
			t.Fatalf("command = %s %#v, want uname -p", name, args)
		}
		return "i386\n"
	})

	if got != "i386" {
		t.Fatalf("currentProcessorISA(testSession, openbsd) = %q, want i386", got)
	}
}

func TestCurrentOSReleaseOpenBSDUsesKernelReleaseMap(t *testing.T) {
	got := currentOSRelease(testSession, "openbsd", nil, func(name string, args ...string) string {
		if name != "uname" || !reflect.DeepEqual(args, []string{"-r"}) {
			t.Fatalf("command = %s %#v, want uname -r", name, args)
		}
		return "7.2\n"
	})

	want := map[string]any{"full": "7.2", "major": "7", "minor": "2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentOSRelease(testSession, openbsd) = %#v, want %#v", got, want)
	}
}

func TestCoreFacts_processorSpeedOmittedWhenProbeYieldsNothing(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	processors, ok := collection["processors"].(map[string]any)
	if !ok {
		t.Fatalf("processors fact = %#v, want map", collection["processors"])
	}
	speed, ok := processors["speed"]
	if !ok {
		return // unresolvable speed (e.g. Apple Silicon) is absent, matching Ruby
	}
	if got, isString := speed.(string); !isString || got == "" {
		t.Fatalf("processors.speed = %#v, want non-empty string when present", speed)
	}
}

func TestCurrentProcessorInfoDarwinMatchesRubyMacOSResolver(t *testing.T) {
	output := strings.Join([]string{
		"hw.logicalcpu_max: 4",
		"hw.physicalcpu_max: 1",
		"machdep.cpu.brand_string: Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
		"hw.cpufrequency_max: 2300000000",
		"machdep.cpu.core_count: 4",
		"machdep.cpu.thread_count: 4",
	}, "\n")
	got := currentProcessorInfo("darwin", func(name string, args ...string) string {
		wantArgs := []string{
			"hw.logicalcpu_max",
			"hw.physicalcpu_max",
			"machdep.cpu.brand_string",
			"hw.cpufrequency_max",
			"machdep.cpu.core_count",
			"machdep.cpu.thread_count",
		}
		if name != "sysctl" || !reflect.DeepEqual(args, wantArgs) {
			t.Fatalf("command = %s %#v, want sysctl %#v", name, args, wantArgs)
		}
		return output
	})

	want := processorInfo{
		SpeedHz:        2_300_000_000,
		LogicalCount:   4,
		PhysicalCount:  1,
		CoresPerSocket: 4,
		ThreadsPerCore: 1,
		Models: []string{
			"Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
			"Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
			"Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
			"Intel(R) Xeon(R) CPU E5-2697 v4 @ 2.30GHz",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentProcessorInfo(darwin) = %#v, want %#v", got, want)
	}
}

func TestParseDarwinProcessorsKeepsRubyCoreCountAsCoresPerSocket(t *testing.T) {
	input := strings.Join([]string{
		"hw.logicalcpu_max: 16",
		"hw.physicalcpu_max: 2",
		"machdep.cpu.brand_string: Apple CPU",
		"hw.cpufrequency_max: 0",
		"machdep.cpu.core_count: 8",
		"machdep.cpu.thread_count: 16",
	}, "\n")

	got := parseDarwinProcessors(input)
	if got.CoresPerSocket != 8 {
		t.Fatalf("CoresPerSocket = %d, want raw Ruby core_count 8", got.CoresPerSocket)
	}
	if got.ThreadsPerCore != 2 {
		t.Fatalf("ThreadsPerCore = %d, want 2", got.ThreadsPerCore)
	}
}

func TestCoreFacts_includeProcessorTopology(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	processors, ok := collection["processors"].(map[string]any)
	if !ok {
		t.Fatalf("processors fact = %#v, want map", collection["processors"])
	}

	for _, key := range []string{"cores", "threads"} {
		value, ok := processors[key].(int)
		if !ok || value <= 0 {
			t.Fatalf("processors.%s = %#v, want positive int", key, processors[key])
		}
	}
}

func TestCoreFacts_includeOSHardware(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	osFact, ok := collection["os"].(map[string]any)
	if !ok {
		t.Fatalf("os fact = %#v, want map", collection["os"])
	}
	if got, ok := osFact["hardware"].(string); !ok || got == "" {
		t.Fatalf("os.hardware = %#v, want hardware model", osFact["hardware"])
	}
	if got := collection["hardwaremodel"]; got != nil {
		t.Fatalf("hardwaremodel = %#v, want no legacy alias in core collection", got)
	}
}

func TestAugeasFacts_returnsStructuredVersion(t *testing.T) {
	got := augeasFacts("augparse 1.14.1 <http://augeas.net/>")
	want := []ResolvedFact{
		{Name: "augeas.version", Value: "1.14.1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("augeasFacts() = %#v, want %#v", got, want)
	}
}

func TestAugeasFacts_skipsMissingVersion(t *testing.T) {
	if got := augeasFacts(""); got != nil {
		t.Fatalf("augeasFacts() = %#v, want nil", got)
	}
}

func TestAugeasVersionFacts_omittedWhenAugparseUnavailable(t *testing.T) {
	t.Parallel()

	if got := augeasVersionFacts(""); got != nil {
		t.Fatalf("augeasVersionFacts(\"\") = %#v, want nil", got)
	}
	want := []ResolvedFact{{Name: "augeas.version", Value: "1.14.1"}}
	if got := augeasVersionFacts("1.14.1"); !reflect.DeepEqual(got, want) {
		t.Fatalf("augeasVersionFacts(1.14.1) = %#v, want %#v", got, want)
	}
}

func TestDisksFacts_omittedWhenNoDevicesEnumerate(t *testing.T) {
	t.Parallel()

	if got := disksFacts(nil); got != nil {
		t.Fatalf("disksFacts(nil) = %#v, want nil", got)
	}
	if got := disksFacts(map[string]any{}); got != nil {
		t.Fatalf("disksFacts(empty) = %#v, want nil", got)
	}

	disks := map[string]any{"sda": map[string]any{"size": "8.00 GiB"}}
	want := []ResolvedFact{{Name: "disks", Value: disks}}
	if got := disksFacts(disks); !reflect.DeepEqual(got, want) {
		t.Fatalf("disksFacts() = %#v, want %#v", got, want)
	}
}

func TestPartitionsFacts_omittedWhenNoDevicesEnumerate(t *testing.T) {
	t.Parallel()

	if got := partitionsFacts(nil); got != nil {
		t.Fatalf("partitionsFacts(nil) = %#v, want nil", got)
	}
	if got := partitionsFacts(map[string]any{}); got != nil {
		t.Fatalf("partitionsFacts(empty) = %#v, want nil", got)
	}

	partitions := map[string]any{"/dev/sda1": map[string]any{"size": "8.00 GiB"}}
	want := []ResolvedFact{{Name: "partitions", Value: partitions}}
	if got := partitionsFacts(partitions); !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionsFacts() = %#v, want %#v", got, want)
	}
}

func TestProcessorSpeedFacts_omittedWhenProbeYieldsNothing(t *testing.T) {
	t.Parallel()

	if got := processorSpeedFacts(""); got != nil {
		t.Fatalf("processorSpeedFacts(\"\") = %#v, want nil", got)
	}
	want := []ResolvedFact{{Name: "processors.speed", Value: "2.30 GHz"}}
	if got := processorSpeedFacts("2.30 GHz"); !reflect.DeepEqual(got, want) {
		t.Fatalf("processorSpeedFacts() = %#v, want %#v", got, want)
	}
}

func TestFilesystemsFacts_omittedWhenUnresolved(t *testing.T) {
	t.Parallel()

	if got := filesystemsFacts(nil); got != nil {
		t.Fatalf("filesystemsFacts(nil) = %#v, want nil", got)
	}
	if got := filesystemsFacts(""); got != nil {
		t.Fatalf("filesystemsFacts(\"\") = %#v, want nil", got)
	}
	want := []ResolvedFact{{Name: "filesystems", Value: "apfs,autofs,devfs"}}
	if got := filesystemsFacts("apfs,autofs,devfs"); !reflect.DeepEqual(got, want) {
		t.Fatalf("filesystemsFacts() = %#v, want %#v", got, want)
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

func TestRubyFacts_omittedWithoutRubyRuntime(t *testing.T) {
	t.Parallel()

	if got := rubyFacts(rubyInfo{}); got != nil {
		t.Fatalf("rubyFacts(zero) = %#v, want nil", got)
	}
}

// Sweep: a default host discovery emits no top-level fact whose value is an
// empty string or empty map — unresolvable facts are absent, never
// placeholders (openspec change omit-not-applicable-facts).
func TestCoreFacts_defaultDiscoveryHasNoEmptyTopLevelFacts(t *testing.T) {
	collection := Collection(CoreFacts(testSession))

	for name, value := range collection {
		switch v := value.(type) {
		case string:
			if v == "" {
				t.Errorf("fact %q = empty string, want fact omitted", name)
			}
		case map[string]any:
			if len(v) == 0 {
				t.Errorf("fact %q = empty map, want fact omitted", name)
			}
		}
	}
}

// Platform-inapplicable facts are absent from the host collection, matching
// the platforms where Ruby Facter resolves them.
func TestCoreFacts_omitPlatformInapplicableFacts(t *testing.T) {
	collection := Collection(CoreFacts(testSession))

	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		if value, ok := collection["fips_enabled"]; ok {
			t.Errorf("fips_enabled = %#v, want fact omitted on %s", value, runtime.GOOS)
		}
	}
	if runtime.GOOS != "linux" {
		if osFact, ok := collection["os"].(map[string]any); ok {
			if value, ok := osFact["selinux"]; ok {
				t.Errorf("os.selinux = %#v, want fact omitted on %s", value, runtime.GOOS)
			}
		}
	}
}

func TestCurrentAugeasVersion_prefersPuppetAgentAugparse(t *testing.T) {
	var gotName string
	var gotArgs []string

	got := currentAugeasVersion(
		func(path string) bool { return path == "/opt/puppetlabs/puppet/bin/augparse" },
		func(name string, args ...string) string {
			gotName = name
			gotArgs = args
			return "augparse 1.12.0 <http://augeas.net/>"
		},
	)

	if got != "1.12.0" {
		t.Fatalf("currentAugeasVersion() = %q, want 1.12.0", got)
	}
	if gotName != "/opt/puppetlabs/puppet/bin/augparse" {
		t.Fatalf("augparse command = %q, want puppet-agent augparse", gotName)
	}
	if !reflect.DeepEqual(gotArgs, []string{"--version"}) {
		t.Fatalf("augparse args = %#v, want --version", gotArgs)
	}
}

func TestCurrentAugeasVersion_usesPathAugparseWhenPuppetAgentAugparseIsAbsent(t *testing.T) {
	var gotName string

	got := currentAugeasVersion(
		func(string) bool { return false },
		func(name string, args ...string) string {
			gotName = name
			return "augparse 1.14.1 <http://augeas.net/>"
		},
	)

	if got != "1.14.1" {
		t.Fatalf("currentAugeasVersion() = %q, want 1.14.1", got)
	}
	if gotName != "augparse" {
		t.Fatalf("augparse command = %q, want path augparse", gotName)
	}
}

func TestParseAugeasVersion(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "augparse", out: "augparse 1.14.1 <http://augeas.net/>", want: "1.14.1"},
		{name: "package suffix", out: "augparse 1.12.0-2ubuntu1", want: "1.12.0"},
		{name: "no version", out: "augparse unknown", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAugeasVersion(tt.out); got != tt.want {
				t.Fatalf("parseAugeasVersion(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

func TestCoreFacts_includeFilesystems(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("filesystems resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession))
	if got, ok := collection["filesystems"].(string); !ok || got == "" {
		t.Fatalf("filesystems = %#v, want non-empty comma-separated string", collection["filesystems"])
	}
}

func TestDisksFact_readsLinuxSysfsBlockDevices(t *testing.T) {
	dir := t.TempDir()
	disk := filepath.Join(dir, "sda")
	for _, subdir := range []string{"device", "queue"} {
		if err := os.MkdirAll(filepath.Join(disk, subdir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"device/model":     "FastDisk\n",
		"device/vendor":    "Acme\n",
		"queue/rotational": "0\n",
		"size":             "2048\n",
	}
	for name, value := range files {
		if err := os.WriteFile(filepath.Join(disk, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got := disksFact(dir)
	want := map[string]any{
		"sda": map[string]any{
			"model":      "FastDisk",
			"vendor":     "Acme",
			"type":       "ssd",
			"size":       "1.00 MiB",
			"size_bytes": 1_048_576,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("disksFact() = %#v, want %#v", got, want)
	}
}

func TestCurrentLinuxDisksAddsSerialAndWWNLikeRubyResolver(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"sda", "sr0"} {
		disk := filepath.Join(dir, name)
		for _, subdir := range []string{"device", "queue"} {
			if err := os.MkdirAll(filepath.Join(disk, subdir), 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
	files := map[string]string{
		"sda/device/model":     "model2\n",
		"sda/device/vendor":    "vendor2\n",
		"sda/queue/rotational": "1\n",
		"sda/size":             "231\n",
		"sr0/device/model":     "model1\n",
		"sr0/device/vendor":    "vendor1\n",
		"sr0/queue/rotational": "0\n",
		"sr0/size":             "12\n",
	}
	for name, value := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) string {
		if name != "lsblk" || len(args) != 4 || args[0] != "-dn" || args[1] != "-o" {
			t.Fatalf("run(%q, %#v), want lsblk -dn -o <field> /dev/<disk>", name, args)
		}
		switch strings.Join(args[2:], " ") {
		case "serial /dev/sda":
			return "B2EI34F1AL\n"
		case "wwn /dev/sda":
			return "29429191.0\n"
		case "serial /dev/sr0", "wwn /dev/sr0":
			return ""
		default:
			t.Fatalf("unexpected lsblk args %#v", args)
			return ""
		}
	}

	got := currentLinuxDisks(dir, run)
	want := map[string]any{
		"sda": map[string]any{
			"model":      "model2",
			"serial":     "B2EI34F1AL",
			"size":       "115.50 KiB",
			"size_bytes": 118_272,
			"type":       "hdd",
			"vendor":     "vendor2",
			"wwn":        "29429191.0",
		},
		"sr0": map[string]any{
			"model":      "model1",
			"size":       "6.00 KiB",
			"size_bytes": 6144,
			"type":       "ssd",
			"vendor":     "vendor1",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxDisks() = %#v, want %#v", got, want)
	}
}

func TestXenFacts_includePrivilegedDomains(t *testing.T) {
	got := Collection(xenFacts("xen0", []string{"win", "linux"}))

	wantXen := map[string]any{"domains": []string{"win", "linux"}}
	if !reflect.DeepEqual(got["xen"], wantXen) {
		t.Fatalf("xen = %#v, want %#v", got["xen"], wantXen)
	}
}

func TestXenFacts_skipUnprivilegedXen(t *testing.T) {
	got := xenFacts("xenu", []string{"win"})

	if len(got) != 1 || got[0].Name != "xen" || got[0].Value != nil {
		t.Fatalf("xenFacts(xenu) = %#v, want nil xen fact only", got)
	}
}

func TestDetectXenVMFromSignalsMatchesRubyResolver(t *testing.T) {
	tests := []struct {
		name      string
		evtchn    bool
		procXen   bool
		xvda1     bool
		xvda1Link bool
		want      string
	}{
		{name: "privileged evtchn", evtchn: true, procXen: true, xvda1: true, want: "xen0"},
		{name: "proc xen unprivileged", procXen: true, want: "xenu"},
		{name: "xvda unprivileged", xvda1: true, want: "xenu"},
		{name: "xvda symlink ignored", xvda1: true, xvda1Link: true, want: ""},
		{name: "not xen", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectXenVMFromSignals(tt.evtchn, tt.procXen, tt.xvda1, tt.xvda1Link); got != tt.want {
				t.Fatalf("detectXenVMFromSignals() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectXenCommandMatchesRubyResolver(t *testing.T) {
	tests := []struct {
		name   string
		exists map[string]bool
		want   string
	}{
		{
			name: "both stacks prefer xen toolstack",
			exists: map[string]bool{
				"/usr/lib/xen-common/bin/xen-toolstack": true,
				"/usr/sbin/xl":                          true,
				"/usr/sbin/xm":                          true,
			},
			want: "/usr/lib/xen-common/bin/xen-toolstack",
		},
		{name: "xl first", exists: map[string]bool{"/usr/sbin/xl": true}, want: "/usr/sbin/xl"},
		{name: "xm fallback", exists: map[string]bool{"/usr/sbin/xm": true}, want: "/usr/sbin/xm"},
		{name: "no command", exists: map[string]bool{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectXenCommand(func(path string) bool { return tt.exists[path] })
			if got != tt.want {
				t.Fatalf("selectXenCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseLinuxFilesystems_sortsAndSkipsPseudoEntries(t *testing.T) {
	input := "nodev\tsysfs\nnodev\tproc\next4\nfuseblk\nxfs\n"

	if got, want := parseLinuxFilesystems(input), "ext4,xfs"; got != want {
		t.Fatalf("parseLinuxFilesystems() = %q, want %q", got, want)
	}
}

func TestCurrentLinuxFilesystemsUnreadableProcMatchesRubyResolver(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		if path != "/proc/filesystems" {
			t.Fatalf("path = %q, want /proc/filesystems", path)
		}
		return nil, os.ErrPermission
	}

	if got := currentFilesystems("linux", readFile, nil); got != nil {
		t.Fatalf("currentFilesystems(linux) = %#v, want nil", got)
	}
}

func TestParseDarwinFilesystems_sortsUniqueFilesystemTypes(t *testing.T) {
	input := "/dev/disk3s1 on / (apfs, local, read-only)\nmap auto_home on /System/Volumes/Data/home (autofs, automounted)\n/dev/disk3s2 on /System/Volumes/Preboot (apfs, local)\n"

	if got, want := parseDarwinFilesystems(input), "apfs,autofs"; got != want {
		t.Fatalf("parseDarwinFilesystems() = %q, want %q", got, want)
	}
}

func TestParseDarwinFilesystems_matchesRubyMacOSFixture(t *testing.T) {
	input := strings.Join([]string{
		"/dev/disk1s5 on / (apfs, local, read-only, journaled)",
		"devfs on /dev (devfs, local, nobrowse)",
		"/dev/disk1s1 on /System/Volumes/Data (apfs, local, journaled, nobrowse)",
		"/dev/disk1s4 on /private/var/vm (apfs, local, journaled, nobrowse)",
		"map auto_home on /System/Volumes/Data/home (autofs, automounted, nobrowse)",
		".host:/VMware Shared Folders on /Volumes/VMware Shared Folders (vmhgfs)",
	}, "\n")

	if got, want := parseDarwinFilesystems(input), "apfs,autofs,devfs,vmhgfs"; got != want {
		t.Fatalf("parseDarwinFilesystems() = %q, want %q", got, want)
	}
}

func TestParseLinuxOSRelease_keepsGenericMajorReleaseAsFullVersion(t *testing.T) {
	got := parseLinuxOSRelease("ID=ubuntu\nVERSION_ID=10.9\n")

	want := map[string]any{"full": "10.9", "major": "10.9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxOSRelease() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxOSRelease_splitsDebianMajorAndMinorRelease(t *testing.T) {
	got := parseLinuxOSRelease("ID=debian\nVERSION_ID=10.02\n")

	want := map[string]any{"full": "10.02", "major": "10", "minor": "2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxOSRelease() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxOSRelease_padsDebianVersionIDLikeRubyResolver(t *testing.T) {
	got := parseLinuxOSRelease("ID=debian\nVERSION_ID=10\n")

	want := map[string]any{"full": "10.0", "major": "10", "minor": "0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxOSRelease() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxDistroOSRelease_trimsDebianMinorLeadingZero(t *testing.T) {
	got := parseLinuxDistroOSRelease("ID=debian\nVERSION_ID=10.02\n")

	want := map[string]any{"full": "10.02", "major": "10", "minor": "2"}
	if !reflect.DeepEqual(got.Release, want) {
		t.Fatalf("parseLinuxDistroOSRelease().Release = %#v, want %#v", got.Release, want)
	}
}

func TestCurrentOSRelease_prefersDistroSpecificReleaseFiles(t *testing.T) {
	tests := []struct {
		name         string
		osRelease    string
		specificPath string
		specificBody string
		want         map[string]any
	}{
		{
			name:         "mageia",
			osRelease:    "ID=mageia\nVERSION_ID=9\n",
			specificPath: "/etc/mageia-release",
			specificBody: "Mageia release 19.4\n",
			want:         map[string]any{"full": "19.4", "major": "19", "minor": "4"},
		},
		{
			name:         "openwrt",
			osRelease:    "ID=openwrt\nVERSION_ID=23.05.3\n",
			specificPath: "/etc/openwrt_version",
			specificBody: "19.07.10\n",
			want:         map[string]any{"full": "19.07.10", "major": "19", "minor": "07.10"},
		},
		{
			name:         "gentoo",
			osRelease:    "ID=gentoo\nVERSION_ID=2.8\n",
			specificPath: "/etc/gentoo-release",
			specificBody: "Gentoo Base System release 2007.0\n",
			want:         map[string]any{"full": "2007.0", "major": "2007", "minor": "0"},
		},
		{
			name:         "alpine",
			osRelease:    "ID=alpine\nVERSION_ID=3\n",
			specificPath: "/etc/alpine-release",
			specificBody: "3.13.0\n",
			want:         map[string]any{"full": "3.13.0", "major": "3", "minor": "13"},
		},
		{
			name:         "slackware",
			osRelease:    "ID=slackware\nVERSION_ID=15.0\n",
			specificPath: "/etc/slackware-version",
			specificBody: "Slackware 19.4\n",
			want:         map[string]any{"full": "19.4", "major": "19", "minor": "4"},
		},
		{
			name:         "amazon linux",
			osRelease:    "ID=amzn\nVERSION_ID=2\n",
			specificPath: "/etc/system-release",
			specificBody: "Amazon Linux 2\n",
			want:         map[string]any{"full": "2", "major": "2"},
		},
		{
			name:         "photon",
			osRelease:    "ID=photon\nVERSION_ID=5.0\n",
			specificPath: "/etc/lsb-release",
			specificBody: "DISTRIB_RELEASE=\"19.4\"\n",
			want:         map[string]any{"full": "19.4", "major": "19", "minor": "4"},
		},
		{
			name:         "mariner",
			osRelease:    "ID=mariner\nVERSION_ID=2.0\n",
			specificPath: "/etc/mariner-release",
			specificBody: "CBL-Mariner 2.0.20220824\n",
			want:         map[string]any{"full": "2.0.20220824", "major": "2", "minor": "0"},
		},
		{
			name:         "azurelinux",
			osRelease:    "ID=azurelinux\nVERSION_ID=3.0\n",
			specificPath: "/etc/azurelinux-release",
			specificBody: "AZURELINUX_BUILD_NUMBER=3.0.20240401\n",
			want:         map[string]any{"full": "3.0.20240401", "major": "3", "minor": "0"},
		},
		{
			name:         "linuxmint",
			osRelease:    "ID=linuxmint\nVERSION_ID=21.3\n",
			specificPath: "/etc/linuxmint/info",
			specificBody: "RELEASE=19.4\n",
			want:         map[string]any{"full": "19", "major": "19"},
		},
		{
			name:         "devuan",
			osRelease:    "ID=devuan\nVERSION_ID=beowulf\n",
			specificPath: "/etc/devuan_version",
			specificBody: "2.13.0\n",
			want:         map[string]any{"full": "2.13.0", "major": "2", "minor": "13"},
		},
		{
			name:         "meego",
			osRelease:    "ID=meego\nVERSION_ID=beowulf\n",
			specificPath: "/etc/meego-release",
			specificBody: "2.13.0\n",
			want:         map[string]any{"full": "2.13.0", "major": "2", "minor": "13"},
		},
		{
			name:         "ovs",
			osRelease:    "ID=ovs\nVERSION_ID=beowulf\n",
			specificPath: "/etc/ovs-release",
			specificBody: "Open vSwitch release 2.13.0\n",
			want:         map[string]any{"full": "2.13.0", "major": "2", "minor": "13"},
		},
		{
			name:         "eos",
			osRelease:    "ID=eos\nVERSION_ID=4.31.2F\n",
			specificPath: "/etc/Eos-release",
			specificBody: "Arista 4.31.2F\n",
			want:         map[string]any{"full": "4.31.2F", "major": "4", "minor": "31"},
		},
		{
			name:         "oel",
			osRelease:    "ID=oel\nVERSION_ID=beowulf\n",
			specificPath: "/etc/enterprise-release",
			specificBody: "Oracle Linux release 10.5 (something)\n",
			want:         map[string]any{"full": "10.5", "major": "10", "minor": "5"},
		},
		{
			name:         "ol",
			osRelease:    "ID=ol\nVERSION_ID=beowulf\n",
			specificPath: "/etc/oracle-release",
			specificBody: "Oracle Linux release 9.4\n",
			want:         map[string]any{"full": "9.4", "major": "9", "minor": "4"},
		},
		{
			name:         "debian",
			osRelease:    "ID=debian\nVERSION_ID=12\n",
			specificPath: "/etc/debian_version",
			specificBody: "testing/release\n",
			want:         map[string]any{"full": "testing/release", "major": "testing/release"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{
				"/etc/os-release": tt.osRelease,
				tt.specificPath:   tt.specificBody,
			}
			readFile := func(path string) ([]byte, error) {
				value, ok := files[path]
				if !ok {
					return nil, os.ErrNotExist
				}
				return []byte(value), nil
			}

			got := currentOSRelease(testSession, "linux", readFile, func(string, ...string) string { return "" })
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("currentOSRelease(testSession) = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCurrentOSRelease_marinerAndAzureLinuxFallbackSplitOSReleaseVersion(t *testing.T) {
	tests := []struct {
		name      string
		osRelease string
		want      map[string]any
	}{
		{
			name:      "mariner",
			osRelease: "ID=mariner\nVERSION_ID=2.0.20220824\n",
			want:      map[string]any{"full": "2.0.20220824", "major": "2", "minor": "0"},
		},
		{
			name:      "azurelinux",
			osRelease: "ID=azurelinux\nVERSION_ID=3.0.20240401\n",
			want:      map[string]any{"full": "3.0.20240401", "major": "3", "minor": "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readFile := func(path string) ([]byte, error) {
				if path != "/etc/os-release" {
					return nil, os.ErrNotExist
				}
				return []byte(tt.osRelease), nil
			}

			got := currentOSRelease(testSession, "linux", readFile, func(string, ...string) string { return "" })
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("currentOSRelease(testSession) = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCurrentOSRelease_linuxmintFallbackSplitsOSReleaseVersionLikeRubyFact(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		if path != "/etc/os-release" {
			return nil, os.ErrNotExist
		}
		return []byte("ID=linuxmint\nVERSION_ID=19.4\n"), nil
	}

	got := currentOSRelease(testSession, "linux", readFile, func(string, ...string) string { return "" })
	want := map[string]any{"full": "19.4", "major": "19", "minor": "4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentOSRelease(testSession) = %#v, want %#v", got, want)
	}
}

func TestCurrentOSRelease_gentooAndMageiaFallbackSplitOSReleaseVersion(t *testing.T) {
	tests := []struct {
		name      string
		osRelease string
		want      map[string]any
	}{
		{
			name:      "gentoo",
			osRelease: "ID=gentoo\nVERSION_ID=2007.0\n",
			want:      map[string]any{"full": "2007.0", "major": "2007", "minor": "0"},
		},
		{
			name:      "mageia",
			osRelease: "ID=mageia\nVERSION_ID=19.4\n",
			want:      map[string]any{"full": "19.4", "major": "19", "minor": "4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readFile := func(path string) ([]byte, error) {
				if path != "/etc/os-release" {
					return nil, os.ErrNotExist
				}
				return []byte(tt.osRelease), nil
			}

			got := currentOSRelease(testSession, "linux", readFile, func(string, ...string) string { return "" })
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("currentOSRelease(testSession) = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCurrentOSRelease_usesAmazonLinux2023RPMVersion(t *testing.T) {
	files := map[string]string{
		"/etc/os-release":     "ID=amzn\nVERSION_ID=2023\n",
		"/etc/system-release": "Amazon Linux 2023\n",
	}
	readFile := func(path string) ([]byte, error) {
		value, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(value), nil
	}
	run := func(name string, args ...string) string {
		if name != "rpm" || !reflect.DeepEqual(args, []string{"-q", "--qf", "%{NAME}\n%{VERSION}\n%{RELEASE}\n%{VENDOR}", "-f", "/etc/os-release"}) {
			t.Fatalf("run(%q, %#v), want rpm os-release package query", name, args)
		}
		return "system-release\n2023.1.20230912\n1.amzn2023\nAmazon Linux"
	}

	got := currentOSRelease(testSession, "linux", readFile, run)
	want := map[string]any{"full": "2023.1.20230912", "major": "2023", "minor": "1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentOSRelease(testSession) = %#v, want %#v", got, want)
	}
}

func TestReleaseFromFirstLineMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "release value", input: "Oracle Linux release 10.5 (something)", want: "10.5"},
		{name: "rawhide", input: "a bunch of data and there is Rawhide", want: "Rawhide"},
		{name: "amazon linux", input: "some other data and Amazon Linux 15 and that's it", want: "15"},
		{name: "missing", input: "Oracle Linux Server", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := releaseFromFirstLine(tt.input); got != tt.want {
				t.Fatalf("releaseFromFirstLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRedHatRelease_matchesRubyResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  linuxDistro
	}{
		{
			name:  "enterprise linux",
			input: "Red Hat Enterprise Linux release 8.0 (Ootpa)\n",
			want: linuxDistro{
				Name:         "RedHat",
				ID:           "RedHatEnterprise",
				Description:  "Red Hat Enterprise Linux release 8.0 (Ootpa)",
				Codename:     "Ootpa",
				Release:      map[string]any{"full": "8.0", "major": "8", "minor": "0"},
				ReleaseKnown: true,
			},
		},
		{
			name:  "centos linux",
			input: "CentOS Linux release 7.2.1511 (Core)\n",
			want: linuxDistro{
				Name:         "CentOS",
				ID:           "CentOS",
				Description:  "CentOS Linux release 7.2.1511 (Core)",
				Codename:     "Core",
				Release:      map[string]any{"full": "7.2.1511", "major": "7", "minor": "2"},
				ReleaseKnown: true,
			},
		},
		{
			name:  "oracle vm without codename",
			input: "Oracle VM server release 3.4.4\n",
			want: linuxDistro{
				Name:         "OracleVM",
				ID:           "OracleVMserver",
				Description:  "Oracle VM server release 3.4.4",
				Release:      map[string]any{"full": "3.4.4", "major": "3", "minor": "4"},
				ReleaseKnown: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseRedHatRelease(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseRedHatRelease() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCurrentLinuxDistro_usesRedHatReleaseForRHELDistroFields(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"/etc/os-release":     "NAME=\"CentOS Linux\"\nID=centos\nVERSION_ID=7.2.1511\n",
		"/etc/redhat-release": "CentOS Linux release 7.2.1511 (Core)\n",
	}
	readFile := func(path string) ([]byte, error) {
		value, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(value), nil
	}
	lookPath := func(string) (string, error) { return "", os.ErrNotExist }

	got := currentLinuxDistro("linux", lookPath, func(string, ...string) string { return "" }, readFile)
	want := linuxDistro{
		Name:         "CentOS",
		ID:           "CentOS",
		Description:  "CentOS Linux release 7.2.1511 (Core)",
		Codename:     "Core",
		Release:      map[string]any{"full": "7.2.1511", "major": "7", "minor": "2"},
		ReleaseKnown: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxDistro() = %#v, want %#v", got, want)
	}
}

func TestCurrentLinuxDistroRHELPrefersRedHatReleaseOverLSB(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"/etc/os-release":     "NAME=\"Red Hat Enterprise Linux\"\nID=rhel\nVERSION_ID=8.0\n",
		"/etc/redhat-release": "Red Hat Enterprise Linux release 8.0 (Ootpa)\n",
	}
	readFile := func(path string) ([]byte, error) {
		value, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(value), nil
	}
	lookPath := func(name string) (string, error) {
		if name != "lsb_release" {
			return "", os.ErrNotExist
		}
		return "/usr/bin/lsb_release", nil
	}
	run := func(name string, args ...string) string {
		if name != "lsb_release" || !reflect.DeepEqual(args, []string{"-a"}) {
			t.Fatalf("run(%q, %#v), want lsb_release -a", name, args)
		}
		return "Distributor ID:\trhel-lsb\nDescription:\tLSB supplied description\nRelease:\t8.0-lsb\nCodename:\tlsb-code\n"
	}

	got := currentLinuxDistro("linux", lookPath, run, readFile)
	core := linuxDistroFacts(got)

	coreCollection := Collection(core)
	osFact, ok := coreCollection["os"].(map[string]any)
	if !ok {
		t.Fatalf("core distro facts = %#v, want os fact", coreCollection)
	}
	distroFact, ok := osFact["distro"].(map[string]any)
	if !ok {
		t.Fatalf("os fact = %#v, want distro map", osFact)
	}
	if distroFact["id"] != "RedHatEnterprise" || distroFact["description"] != "Red Hat Enterprise Linux release 8.0 (Ootpa)" || distroFact["codename"] != "Ootpa" {
		t.Fatalf("os.distro = %#v, want RedHatRelease id, description, and codename", distroFact)
	}
	if !reflect.DeepEqual(distroFact["release"], map[string]any{"full": "8.0", "major": "8", "minor": "0"}) {
		t.Fatalf("os.distro.release = %#v, want RedHatRelease 8.0 map", distroFact["release"])
	}
}

func TestCurrentLinuxDistro_usesSuseReleaseWhenOSReleaseIsMissing(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"/etc/SuSE-release": "openSUSE 11.1 (i586)\nVERSION = 11.1\n",
	}
	readFile := func(path string) ([]byte, error) {
		value, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(value), nil
	}
	lookPath := func(string) (string, error) { return "", os.ErrNotExist }

	got := currentLinuxDistro("linux", lookPath, func(string, ...string) string { return "" }, readFile)
	want := linuxDistro{
		Name:         "openSUSE",
		ID:           "opensuse",
		Release:      map[string]any{"full": "11.1", "major": "11", "minor": "1"},
		ReleaseKnown: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxDistro() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxDistroOSRelease_mapsSLESDistroID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "sles 12",
			in:   "ID=sles\nVERSION_ID=12.1\n",
			want: "SUSE LINUX",
		},
		{
			name: "sles 15",
			in:   "ID=sles\nVERSION_ID=15\n",
			want: "SUSE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseLinuxDistroOSRelease(tt.in)
			if got.ID != tt.want {
				t.Fatalf("parseLinuxDistroOSRelease().ID = %q, want %q", got.ID, tt.want)
			}
		})
	}
}

func TestOSFamily_mapsSLESLikeRubyFact(t *testing.T) {
	t.Parallel()

	distro := parseLinuxDistroOSRelease("ID=sles\nVERSION_ID=15\n")
	if got := osFamily("linux", distro); got != "Suse" {
		t.Fatalf("osFamily(linux, sles) = %q, want Suse", got)
	}
}

func TestOSFamily_mapsBSDLikeRubyFact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos string
		want string
	}{
		{goos: "freebsd", want: "FreeBSD"},
		{goos: "netbsd", want: "NetBSD"},
		{goos: "openbsd", want: "OpenBSD"},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()

			if got := osFamily(tt.goos, linuxDistro{}); got != tt.want {
				t.Fatalf("osFamily(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestParseLinuxDistroOSRelease_mapsMissingSLESCodenameToNA(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("ID=sles\nVERSION_ID=15\n")
	if got.Codename != "n/a" {
		t.Fatalf("parseLinuxDistroOSRelease().Codename = %q, want n/a", got.Codename)
	}
}

func TestParseLinuxDistroOSRelease_mapsSLESDescriptionFromPrettyName(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("ID=sles\nVERSION_ID=15\nPRETTY_NAME=\"SUSE Linux Enterprise Server 15\"\n")
	if got.Description != "SUSE Linux Enterprise Server 15" {
		t.Fatalf("parseLinuxDistroOSRelease().Description = %q, want SUSE Linux Enterprise Server 15", got.Description)
	}

	core := linuxDistroFacts(got)
	collection := Collection(core)
	os, ok := collection["os"].(map[string]any)
	if !ok {
		t.Fatalf("os fact = %#v, want map", collection["os"])
	}
	distro, ok := os["distro"].(map[string]any)
	if !ok {
		t.Fatalf("os.distro = %#v, want map", os["distro"])
	}
	if distro["description"] != "SUSE Linux Enterprise Server 15" {
		t.Fatalf("os.distro.description = %#v, want SUSE Linux Enterprise Server 15", distro["description"])
	}
}

func TestParseLinuxDistroOSRelease_normalizesSLESNameAndSAPID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		id   string
	}{
		{name: "sles", in: "NAME=\"SLES\"\nID=sles\nVERSION_ID=15\n", id: "SUSE"},
		{name: "sles sap", in: "NAME=\"SLES_SAP\"\nID=sles_sap\nVERSION_ID=15\n", id: "SUSE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseLinuxDistroOSRelease(tt.in)
			if got.Name != "SLES" {
				t.Fatalf("parseLinuxDistroOSRelease().Name = %q, want SLES", got.Name)
			}
			if got.ID != tt.id {
				t.Fatalf("parseLinuxDistroOSRelease().ID = %q, want %q", got.ID, tt.id)
			}
		})
	}
}

func TestParseLinuxDistroOSRelease_normalizesArchLinuxName(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("NAME=\"Arch Linux\"\nID=arch\nPRETTY_NAME=\"Arch Linux\"\n")
	if got.Name != "Archlinux" {
		t.Fatalf("parseLinuxDistroOSRelease().Name = %q, want Archlinux", got.Name)
	}
	if name := osName("linux", got); name != "Archlinux" {
		t.Fatalf("osName(linux, arch) = %q, want Archlinux", name)
	}
}

func TestParseLinuxDistroOSRelease_normalizesManjaroLinuxName(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("NAME=\"Manjaro Linux\"\nID=manjaro\nPRETTY_NAME=\"Manjaro Linux\"\n")
	if got.Name != "Manjarolinux" {
		t.Fatalf("parseLinuxDistroOSRelease().Name = %q, want Manjarolinux", got.Name)
	}
	if name := osName("linux", got); name != "Manjarolinux" {
		t.Fatalf("osName(linux, manjaro) = %q, want Manjarolinux", name)
	}
}

func TestParseLinuxDistroOSRelease_normalizesOracleLinuxName(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("NAME=\"Oracle Linux Server\"\nID=ol\nPRETTY_NAME=\"Oracle Linux Server\"\n")
	if got.Name != "OracleLinux" {
		t.Fatalf("parseLinuxDistroOSRelease().Name = %q, want OracleLinux", got.Name)
	}
	if name := osName("linux", got); name != "OracleLinux" {
		t.Fatalf("osName(linux, ol) = %q, want OracleLinux", name)
	}
}

func TestParseLinuxDistroOSRelease_normalizesAzureLinuxName(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("NAME=\"Microsoft Azure Linux\"\nID=azurelinux\nPRETTY_NAME=\"Microsoft Azure Linux\"\n")
	if got.Name != "AzureLinux" {
		t.Fatalf("parseLinuxDistroOSRelease().Name = %q, want AzureLinux", got.Name)
	}
	if name := osName("linux", got); name != "AzureLinux" {
		t.Fatalf("osName(linux, azurelinux) = %q, want AzureLinux", name)
	}
}

func TestParseLinuxDistroOSRelease_normalizesMarinerNameLikeRubyResolver(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("NAME=\"Common Base Linux Mariner\"\nID=mariner\nPRETTY_NAME=\"Common Base Linux Mariner\"\n")
	if got.Name != "Mariner" {
		t.Fatalf("parseLinuxDistroOSRelease().Name = %q, want Mariner", got.Name)
	}
	if name := osName("linux", got); name != "Mariner" {
		t.Fatalf("osName(linux, mariner) = %q, want Mariner", name)
	}
}

func TestParseLinuxDistroOSRelease_appendsLinuxToVirtuozzoName(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("NAME=\"Virtuozzo\"\nID=virtuozzo\nPRETTY_NAME=\"Virtuozzo\"\n")
	if got.Name != "VirtuozzoLinux" {
		t.Fatalf("parseLinuxDistroOSRelease().Name = %q, want VirtuozzoLinux", got.Name)
	}
	if name := osName("linux", got); name != "VirtuozzoLinux" {
		t.Fatalf("osName(linux, virtuozzo) = %q, want VirtuozzoLinux", name)
	}
}

func TestParseLinuxDistroOSRelease_mapsSLESReleaseMinorToNil(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("ID=sles\nVERSION_ID=15\n")
	want := map[string]any{"full": "15", "major": "15", "minor": nil}
	if !reflect.DeepEqual(got.Release, want) {
		t.Fatalf("parseLinuxDistroOSRelease().Release = %#v, want %#v", got.Release, want)
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
			},
		},
	}
	if !reflect.DeepEqual(collection, want) {
		t.Fatalf("openBSDDMIFacts() = %#v, want %#v", collection, want)
	}
}

func TestParseFreeBSDOSRelease_matchesRubyFreeBSDFact(t *testing.T) {
	tests := []struct {
		name              string
		installedUserland string
		want              map[string]any
	}{
		{
			name:              "RELEASE patchlevel",
			installedUserland: "12.1-RELEASE-p3",
			want: map[string]any{
				"full":       "12.1-RELEASE-p3",
				"major":      "12",
				"minor":      "1",
				"branch":     "RELEASE-p3",
				"patchlevel": "3",
			},
		},
		{
			name:              "STABLE",
			installedUserland: "12.1-STABLE",
			want: map[string]any{
				"full":   "12.1-STABLE",
				"major":  "12",
				"minor":  "1",
				"branch": "STABLE",
			},
		},
		{
			name:              "CURRENT",
			installedUserland: "13-CURRENT",
			want: map[string]any{
				"full":   "13-CURRENT",
				"major":  "13",
				"branch": "CURRENT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFreeBSDOSRelease(tt.installedUserland)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseFreeBSDOSRelease() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseFreeBSDVersions_returnsKernelAndUserlandValues(t *testing.T) {
	got := parseFreeBSDVersions("13.0-CURRENT\n", "12.1-RELEASE-p3\n12.0-STABLE\n")
	want := freeBSDVersions{
		InstalledKernel:   "13.0-CURRENT",
		RunningKernel:     "12.1-RELEASE-p3",
		InstalledUserland: "12.0-STABLE",
	}
	if got != want {
		t.Fatalf("parseFreeBSDVersions() = %#v, want %#v", got, want)
	}
}

func TestCurrentOSRelease_mapsDarwinKernelReleaseLikeRubyFact(t *testing.T) {
	got := currentOSRelease(testSession, "darwin", nil, func(name string, args ...string) string {
		if name != "uname" || !reflect.DeepEqual(args, []string{"-r"}) {
			t.Fatalf("run(%q, %#v), want uname -r", name, args)
		}
		return "10.9\n"
	})

	want := map[string]any{"full": "10.9", "major": "10", "minor": "9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentOSRelease(testSession, darwin) = %#v, want %#v", got, want)
	}
}

func TestKernelMajorVersionFact_matchesLinuxRubyFact(t *testing.T) {
	tests := []struct {
		name          string
		kernelRelease string
		want          string
	}{
		{name: "dot separated", kernelRelease: "4.15", want: "4.15"},
		{name: "no dot delimiter", kernelRelease: "4test", want: "4test"},
		{name: "package suffix", kernelRelease: "4.15.0-109-generic", want: "4.15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kernelMajorVersionFact("linux", tt.kernelRelease, nil); got != tt.want {
				t.Fatalf("kernelMajorVersionFact(linux, %q) = %q, want %q", tt.kernelRelease, got, tt.want)
			}
		})
	}
}

func TestKernelMajorVersionFact_matchesBSDRubyFact(t *testing.T) {
	if got := kernelMajorVersionFact("freebsd", "12.1-RELEASE-p3", nil); got != "12" {
		t.Fatalf("kernelMajorVersionFact(freebsd) = %q, want 12", got)
	}
}

func TestKernelMajorVersionFact_matchesDarwinRubyFact(t *testing.T) {
	if got := kernelMajorVersionFact("darwin", "18.7.0", nil); got != "18.7" {
		t.Fatalf("kernelMajorVersionFact(darwin) = %q, want 18.7", got)
	}
}

func TestParseLinuxOSRelease(t *testing.T) {
	got := parseLinuxOSRelease("NAME=Ubuntu\nVERSION_ID=\"24.04\"\n")
	want := map[string]any{"full": "24.04", "major": "24.04"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxOSRelease() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxOSRelease_withoutMinor(t *testing.T) {
	got := parseLinuxOSRelease("ID=amzn\nVERSION_ID=\"2023\"\n")
	want := map[string]any{"full": "2023", "major": "2023"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxOSRelease() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxOSRelease_missingVersionReturnsNil(t *testing.T) {
	if got := parseLinuxOSRelease("ID=photon\n"); got != nil {
		t.Fatalf("parseLinuxOSRelease() = %#v, want nil", got)
	}
}

func TestMacOSVersionFacts_splitsVersion10MajorLikeRubyFacter(t *testing.T) {
	core := macOSVersionFacts("10.15.7", "")

	wantCore := []ResolvedFact{{Name: "os.macosx.version", Value: map[string]any{"full": "10.15.7", "major": "10.15", "minor": "7"}}}
	if !reflect.DeepEqual(core, wantCore) {
		t.Fatalf("macOSVersionFacts() core = %#v, want %#v", core, wantCore)
	}
}

func TestMacOSVersionFacts_includesPatchAndExtraForModernVersions(t *testing.T) {
	core := macOSVersionFacts("14.5.1", "Beta")

	wantVersion := map[string]any{"full": "14.5.1", "major": "14", "minor": "5", "patch": "1", "extra": "Beta"}
	if !reflect.DeepEqual(core, []ResolvedFact{{Name: "os.macosx.version", Value: wantVersion}}) {
		t.Fatalf("macOSVersionFacts() core = %#v, want version %#v", core, wantVersion)
	}
}

func TestMacOSStringFact_returnsCoreFact(t *testing.T) {
	core := macOSStringFact("os.macosx.product", "macOS")

	wantCore := []ResolvedFact{{Name: "os.macosx.product", Value: "macOS"}}
	if !reflect.DeepEqual(core, wantCore) {
		t.Fatalf("macOSStringFact() core = %#v, want %#v", core, wantCore)
	}
}

func TestMacOSStringFact_skipsEmptyValues(t *testing.T) {
	core := macOSStringFact("os.macosx.build", "")

	if core != nil {
		t.Fatalf("macOSStringFact() = %#v, want nil facts", core)
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

func TestCurrentMacOSModelUsesSysctlHWModel(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		if name != "sysctl" || !reflect.DeepEqual(args, []string{"-n", "hw.model"}) {
			t.Fatalf("command = %s %#v, want sysctl -n hw.model", name, args)
		}
		return "MacBookPro11,4\n"
	}

	if got := currentMacOSModel("darwin", run); got != "MacBookPro11,4" {
		t.Fatalf("currentMacOSModel() = %q, want MacBookPro11,4", got)
	}
}

func TestParseSwVers(t *testing.T) {
	got := parseSwVers("ProductName:\t\tmacOS\nProductVersion:\t14.5.1\nProductVersionExtra:\tBeta\nBuildVersion:\t\t23F79\n")
	want := macOSInfo{ProductName: "macOS", ProductVersion: "14.5.1", ProductVersionExtra: "Beta", BuildVersion: "23F79"}

	if got != want {
		t.Fatalf("parseSwVers() = %#v, want %#v", got, want)
	}
}

func TestCurrentMacOSInfoUsesSwVersCommand(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		if name != "sw_vers" || len(args) != 0 {
			t.Fatalf("command = %s %#v, want sw_vers", name, args)
		}
		return "ProductName:\tmacOS\nProductVersion:\t13.3.1\nProductVersionExtra:\t(a)\nBuildVersion:\t22E772610a\n"
	}

	got := currentMacOSInfo("darwin", run)
	want := macOSInfo{ProductName: "macOS", ProductVersion: "13.3.1", ProductVersionExtra: "(a)", BuildVersion: "22E772610a"}
	if got != want {
		t.Fatalf("currentMacOSInfo() = %#v, want %#v", got, want)
	}
}

func TestCoreFacts_includeMacOSReleaseKernelHardwareAndIdentity(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skipf("macOS host fact integration runs only on darwin, not %s", runtime.GOOS)
	}
	collection := Collection(CoreFactsWithRuby(NewSession(), false))
	osFact, ok := collection["os"].(map[string]any)
	if !ok {
		t.Fatalf("os = %#v, want map", collection["os"])
	}
	for _, tt := range []struct {
		key  string
		want string
	}{
		{key: "name", want: "Darwin"},
		{key: "family", want: "Darwin"},
	} {
		if got := osFact[tt.key]; got != tt.want {
			t.Fatalf("os.%s = %#v, want %q", tt.key, got, tt.want)
		}
	}
	for _, key := range []string{"architecture", "hardware"} {
		if got, ok := osFact[key].(string); !ok || got == "" {
			t.Fatalf("os.%s = %#v, want non-empty string", key, osFact[key])
		}
	}
	release, ok := osFact["release"].(map[string]any)
	if !ok || release["full"] == "" || release["major"] == "" {
		t.Fatalf("os.release = %#v, want full and major values", osFact["release"])
	}
	macosx, ok := osFact["macosx"].(map[string]any)
	if !ok {
		t.Fatalf("os.macosx = %#v, want map", osFact["macosx"])
	}
	for _, key := range []string{"product", "build"} {
		if got, ok := macosx[key].(string); !ok || got == "" {
			t.Fatalf("os.macosx.%s = %#v, want non-empty string", key, macosx[key])
		}
	}
	version, ok := macosx["version"].(map[string]any)
	if !ok || version["full"] == "" || version["major"] == "" {
		t.Fatalf("os.macosx.version = %#v, want full and major values", macosx["version"])
	}

	dmi, ok := collection["dmi"].(map[string]any)
	if !ok {
		t.Fatalf("dmi = %#v, want map", collection["dmi"])
	}
	product, ok := dmi["product"].(map[string]any)
	if !ok || product["name"] == "" {
		t.Fatalf("dmi.product = %#v, want product.name", dmi["product"])
	}
	identity, ok := collection["identity"].(map[string]any)
	if !ok {
		t.Fatalf("identity = %#v, want map", collection["identity"])
	}
	for _, key := range []string{"user", "uid", "gid", "group", "privileged"} {
		if _, ok := identity[key]; !ok {
			t.Fatalf("identity = %#v, want key %q", identity, key)
		}
	}
	for _, key := range []string{"kernel", "kernelrelease", "kernelversion", "kernelmajversion", "facterversion"} {
		if got, ok := collection[key].(string); !ok || got == "" {
			t.Fatalf("%s = %#v, want non-empty string", key, collection[key])
		}
	}
	for _, key := range []string{"operatingsystem", "osfamily", "operatingsystemrelease", "architecture"} {
		if got, ok := collection[key]; ok {
			t.Fatalf("%s = %#v, want no legacy alias fact", key, got)
		}
	}
}

func TestParseMacOSSystemProfilerHardware(t *testing.T) {
	input := `Hardware:

    Hardware Overview:

      Model Name: MacBook Pro
      Model Identifier: Mac14,6
      Processor Name: Apple M2 Max
      Processor Speed: 3.68 GHz
      Number of Processors: 1
      Total Number of Cores: 12
      L2 Cache (per Core): 4 MB
      L3 Cache: 24 MB
      Memory: 32 GB
      System Firmware Version: 11881.121.1
      SMC Version (system): 1.16f8
      Serial Number (system): C02TEST1234
      Hardware UUID: 11111111-2222-3333-4444-555555555555
      Subsystem Vendor ID: 0x106b
`

	got := parseMacOSSystemProfilerHardware(input)
	want := macOSSystemProfilerHardware{
		ModelName:          "MacBook Pro",
		ModelIdentifier:    "Mac14,6",
		ProcessorName:      "Apple M2 Max",
		ProcessorSpeed:     "3.68 GHz",
		NumberOfProcessors: "1",
		TotalCores:         "12",
		L2CachePerCore:     "4 MB",
		L3Cache:            "24 MB",
		Memory:             "32 GB",
		BootROMVersion:     "11881.121.1",
		SMCVersion:         "1.16f8",
		SerialNumber:       "C02TEST1234",
		HardwareUUID:       "11111111-2222-3333-4444-555555555555",
		SubsystemVendorID:  "0x106b",
	}
	if got != want {
		t.Fatalf("parseMacOSSystemProfilerHardware() = %#v, want %#v", got, want)
	}
}

func TestParseMacOSSystemProfilerSoftware(t *testing.T) {
	input := `Software:

    System Software Overview:

      System Version: macOS 14.5.1 (23F79)
      Kernel Version: Darwin 23.5.0
      Boot Volume: Macintosh HD
      Boot Mode: Normal
      Computer Name: build-host
      User Name: ncode (ncode)
      Secure Virtual Memory: Enabled
      Time since boot: 3 days, 4 hours, 5 minutes
`

	got := parseMacOSSystemProfilerSoftware(input)
	want := macOSSystemProfilerSoftware{
		SystemVersion:       "macOS 14.5.1 (23F79)",
		KernelVersion:       "Darwin 23.5.0",
		BootVolume:          "Macintosh HD",
		BootMode:            "Normal",
		ComputerName:        "build-host",
		UserName:            "ncode (ncode)",
		SecureVirtualMemory: "Enabled",
		TimeSinceBoot:       "3 days, 4 hours, 5 minutes",
	}
	if got != want {
		t.Fatalf("parseMacOSSystemProfilerSoftware() = %#v, want %#v", got, want)
	}
}

func TestParseMacOSSystemProfilerEthernet(t *testing.T) {
	input := `Ethernet Cards:

    ethernet:

      Type: Ethernet Controller
      Bus: PCI
      Vendor ID: 0x8086
      Device ID: 0x100f
      Subsystem Vendor ID: 0x1ab8
      Subsystem ID: 0x0400
      Revision ID: 0x0000
      BSD name: en0
      Kext name: AppleIntel8254XEthernet.kext
      Location: /System/Library/Extensions/IONetworkingFamily.kext/Contents/PlugIns/AppleIntel8254XEthernet.kext
      Version: 3.1.5
`

	got := parseMacOSSystemProfilerEthernet(input)
	want := macOSSystemProfilerEthernet{
		Type:              "Ethernet Controller",
		Bus:               "PCI",
		VendorID:          "0x8086",
		DeviceID:          "0x100f",
		SubsystemVendorID: "0x1ab8",
		SubsystemID:       "0x0400",
		RevisionID:        "0x0000",
		BSDName:           "en0",
		KextName:          "AppleIntel8254XEthernet.kext",
		Location:          "/System/Library/Extensions/IONetworkingFamily.kext/Contents/PlugIns/AppleIntel8254XEthernet.kext",
		Version:           "3.1.5",
	}
	if got != want {
		t.Fatalf("parseMacOSSystemProfilerEthernet() = %#v, want %#v", got, want)
	}
}

func TestParseMacOSSystemProfilerEthernetIgnoresMalformedKeyValueLinesLikeRubyExecutor(t *testing.T) {
	input := "Vendor ID:0x8086\nDevice ID: 0x100f\n"

	got := parseMacOSSystemProfilerEthernet(input)
	if got.VendorID != "" {
		t.Fatalf("VendorID = %q, want empty", got.VendorID)
	}
	if got.DeviceID != "0x100f" {
		t.Fatalf("DeviceID = %q, want 0x100f", got.DeviceID)
	}
}

func TestCurrentMacOSSystemProfilerEthernetUsesCommand(t *testing.T) {
	var calledName string
	var calledArgs []string
	run := func(name string, args ...string) string {
		calledName = name
		calledArgs = append([]string(nil), args...)
		return "Vendor ID: 0x8086\n"
	}

	got := currentMacOSSystemProfilerEthernet("darwin", run)
	if calledName != "system_profiler" || len(calledArgs) != 1 || calledArgs[0] != "SPEthernetDataType" {
		t.Fatalf("command = %q %#v, want system_profiler SPEthernetDataType", calledName, calledArgs)
	}
	if got.VendorID != "0x8086" {
		t.Fatalf("currentMacOSSystemProfilerEthernet().VendorID = %q, want 0x8086", got.VendorID)
	}
}

func TestMacOSSystemProfilerEthernetFactsIncludeRubyResolverFields(t *testing.T) {
	facts := macOSSystemProfilerEthernetFacts(macOSSystemProfilerEthernet{
		Type:              "Ethernet Controller",
		Bus:               "PCI",
		VendorID:          "0x8086",
		DeviceID:          "0x100f",
		SubsystemVendorID: "0x1ab8",
		SubsystemID:       "0x0400",
		RevisionID:        "0x0000",
		BSDName:           "en0",
		KextName:          "AppleIntel8254XEthernet.kext",
		Location:          "/System/Library/Extensions/IONetworkingFamily.kext/Contents/PlugIns/AppleIntel8254XEthernet.kext",
		Version:           "3.1.5",
	})

	collection := Collection(facts)
	systemProfiler, ok := collection["system_profiler"].(map[string]any)
	if !ok {
		t.Fatalf("system_profiler = %#v, want map", collection["system_profiler"])
	}
	for _, tt := range []struct {
		name string
		want string
	}{
		{name: "type", want: "Ethernet Controller"},
		{name: "bus", want: "PCI"},
		{name: "vendor_id", want: "0x8086"},
		{name: "device_id", want: "0x100f"},
		{name: "subsystem_vendor_id", want: "0x1ab8"},
		{name: "subsystem_id", want: "0x0400"},
		{name: "revision_id", want: "0x0000"},
		{name: "bsd_name", want: "en0"},
		{name: "kext_name", want: "AppleIntel8254XEthernet.kext"},
		{name: "location", want: "/System/Library/Extensions/IONetworkingFamily.kext/Contents/PlugIns/AppleIntel8254XEthernet.kext"},
		{name: "version", want: "3.1.5"},
	} {
		if got := systemProfiler[tt.name]; got != tt.want {
			t.Fatalf("system_profiler.%s = %#v, want %#v", tt.name, got, tt.want)
		}
	}
}

func TestMacOSSystemProfilerEthernetFactsOmitEmptyFields(t *testing.T) {
	facts := macOSSystemProfilerEthernetFacts(macOSSystemProfilerEthernet{})
	if len(facts) != 0 {
		t.Fatalf("macOSSystemProfilerEthernetFacts(empty) = %#v, want no facts", facts)
	}

	facts = macOSSystemProfilerEthernetFacts(macOSSystemProfilerEthernet{
		VendorID: "0x8086",
	})
	if len(facts) != 1 || facts[0] != (ResolvedFact{Name: "system_profiler.vendor_id", Value: "0x8086"}) {
		t.Fatalf("facts = %#v, want only system_profiler.vendor_id", facts)
	}
}

func TestMacOSSystemProfilerFactsIncludesHardwareFacts(t *testing.T) {
	facts := macOSSystemProfilerFacts(macOSSystemProfilerHardware{
		ModelName:          "MacBook Pro",
		ModelIdentifier:    "Mac14,6",
		ProcessorName:      "Apple M2 Max",
		ProcessorSpeed:     "3.68 GHz",
		NumberOfProcessors: "1",
		TotalCores:         "12",
		L2CachePerCore:     "4 MB",
		L3Cache:            "24 MB",
		Memory:             "32 GB",
		BootROMVersion:     "11881.121.1",
		SMCVersion:         "1.16f8",
		SerialNumber:       "C02TEST1234",
		HardwareUUID:       "11111111-2222-3333-4444-555555555555",
		SubsystemVendorID:  "0x106b",
	})

	collection := Collection(facts)
	systemProfiler, ok := collection["system_profiler"].(map[string]any)
	if !ok {
		t.Fatalf("system_profiler = %#v, want map", collection["system_profiler"])
	}
	for _, tt := range []struct {
		name string
		want string
	}{
		{name: "model_name", want: "MacBook Pro"},
		{name: "model_identifier", want: "Mac14,6"},
		{name: "processor_name", want: "Apple M2 Max"},
		{name: "processor_speed", want: "3.68 GHz"},
		{name: "processors", want: "1"},
		{name: "cores", want: "12"},
		{name: "l2_cache_per_core", want: "4 MB"},
		{name: "l3_cache", want: "24 MB"},
		{name: "memory", want: "32 GB"},
		{name: "boot_rom_version", want: "11881.121.1"},
		{name: "smc_version", want: "1.16f8"},
		{name: "serial_number", want: "C02TEST1234"},
		{name: "hardware_uuid", want: "11111111-2222-3333-4444-555555555555"},
		{name: "subsystem_vendor_id", want: "0x106b"},
	} {
		if got := systemProfiler[tt.name]; got != tt.want {
			t.Fatalf("system_profiler.%s = %#v, want %#v", tt.name, got, tt.want)
		}
	}

}

func TestMacOSSystemProfilerFactsOmitEmptyHardwareFields(t *testing.T) {
	facts := macOSSystemProfilerFacts(macOSSystemProfilerHardware{})
	if len(facts) != 0 {
		t.Fatalf("macOSSystemProfilerFacts(empty) = %#v, want no facts", facts)
	}

	facts = macOSSystemProfilerFacts(macOSSystemProfilerHardware{
		ModelName: "MacBook Pro",
	})
	if len(facts) != 1 || facts[0] != (ResolvedFact{Name: "system_profiler.model_name", Value: "MacBook Pro"}) {
		t.Fatalf("facts = %#v, want only system_profiler.model_name", facts)
	}
}

func TestMacOSSystemProfilerSoftwareFactsIncludeRubyResolverFields(t *testing.T) {
	facts := macOSSystemProfilerSoftwareFacts(macOSSystemProfilerSoftware{
		SystemVersion:       "macOS 14.5.1 (23F79)",
		KernelVersion:       "Darwin 23.5.0",
		BootVolume:          "Macintosh HD",
		BootMode:            "Normal",
		ComputerName:        "build-host",
		UserName:            "ncode (ncode)",
		SecureVirtualMemory: "Enabled",
		TimeSinceBoot:       "3 days, 4 hours, 5 minutes",
	})

	collection := Collection(facts)
	systemProfiler, ok := collection["system_profiler"].(map[string]any)
	if !ok {
		t.Fatalf("system_profiler = %#v, want map", collection["system_profiler"])
	}
	for _, tt := range []struct {
		name string
		want string
	}{
		{name: "system_version", want: "macOS 14.5.1 (23F79)"},
		{name: "kernel_version", want: "Darwin 23.5.0"},
		{name: "boot_volume", want: "Macintosh HD"},
		{name: "boot_mode", want: "Normal"},
		{name: "computer_name", want: "build-host"},
		{name: "username", want: "ncode (ncode)"},
		{name: "secure_virtual_memory", want: "Enabled"},
		{name: "uptime", want: "3 days, 4 hours, 5 minutes"},
	} {
		if got := systemProfiler[tt.name]; got != tt.want {
			t.Fatalf("system_profiler.%s = %#v, want %#v", tt.name, got, tt.want)
		}
	}

}

func TestMacOSSystemProfilerSoftwareFactsOmitEmptyFields(t *testing.T) {
	facts := macOSSystemProfilerSoftwareFacts(macOSSystemProfilerSoftware{})
	if len(facts) != 0 {
		t.Fatalf("macOSSystemProfilerSoftwareFacts(empty) = %#v, want no facts", facts)
	}

	facts = macOSSystemProfilerSoftwareFacts(macOSSystemProfilerSoftware{
		SystemVersion: "macOS 14.5.1 (23F79)",
	})
	if len(facts) != 1 || facts[0] != (ResolvedFact{Name: "system_profiler.system_version", Value: "macOS 14.5.1 (23F79)"}) {
		t.Fatalf("facts = %#v, want only system_profiler.system_version", facts)
	}
}

func TestParseLSBRelease(t *testing.T) {
	input := "LSB Version:\t:core-4.1-amd64:core-4.1-noarch:cxx-4.1-amd64\nDistributor ID:\tUbuntu\nDescription:\tUbuntu 24.04.2 LTS\nRelease:\t24.04\nCodename:\tnoble\n"

	got := parseLSBRelease(input)
	want := linuxDistro{
		Name:          "Ubuntu",
		ID:            "Ubuntu",
		Description:   "Ubuntu 24.04.2 LTS",
		Codename:      "noble",
		Specification: ":core-4.1-amd64:core-4.1-noarch:cxx-4.1-amd64",
		Release: map[string]any{
			"full":  "24.04",
			"major": "24.04",
		},
		ReleaseKnown: true,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLSBRelease() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxDistroOSRelease(t *testing.T) {
	input := "ID=photon\nPRETTY_NAME=\"VMware Photon OS/Linux 5.0\"\nVERSION_ID=5.0\nVERSION_CODENAME=photon\n"

	got := parseLinuxDistroOSRelease(input)
	want := linuxDistro{
		ID:          "photon",
		Description: "VMware Photon OS/Linux 5.0",
		Codename:    "photon",
		Release: map[string]any{
			"full":  "5.0",
			"major": "5",
			"minor": "0",
		},
		ReleaseKnown: true,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxDistroOSRelease() = %#v, want %#v", got, want)
	}
}

func TestLinuxDistroFacts_includeCoreDistroFacts(t *testing.T) {
	distro := linuxDistro{
		Name:          "Ubuntu",
		ID:            "Ubuntu",
		Description:   "Ubuntu 24.04.2 LTS",
		Codename:      "noble",
		Specification: ":core-4.1-amd64:core-4.1-noarch:cxx-4.1-amd64",
		Release: map[string]any{
			"full":  "24.04",
			"major": "24",
			"minor": "04",
		},
	}

	core := linuxDistroFacts(distro)
	coreCollection := Collection(core)
	osFact, ok := coreCollection["os"].(map[string]any)
	if !ok {
		t.Fatalf("core distro facts = %#v, want os fact", coreCollection)
	}
	distroFact, ok := osFact["distro"].(map[string]any)
	if !ok {
		t.Fatalf("os fact = %#v, want distro map", osFact)
	}
	if distroFact["id"] != "Ubuntu" || distroFact["description"] != "Ubuntu 24.04.2 LTS" || distroFact["codename"] != "noble" || distroFact["specification"] != distro.Specification {
		t.Fatalf("os.distro = %#v, want id, description, codename, specification", distroFact)
	}
	if !reflect.DeepEqual(distroFact["release"], distro.Release) {
		t.Fatalf("os.distro.release = %#v, want %#v", distroFact["release"], distro.Release)
	}

}

func TestLinuxDistroFactsDevuanReturnsNilDistroReleaseWithoutLSBRelease(t *testing.T) {
	distro := parseLinuxDistroOSRelease("ID=devuan\nVERSION_ID=beowulf\n")

	core := linuxDistroFacts(distro)

	for _, fact := range core {
		if fact.Name == "os.distro.release" {
			if fact.Value != nil {
				t.Fatalf("os.distro.release = %#v, want nil", fact.Value)
			}
			return
		}
	}
	t.Fatalf("core facts = %#v, want os.distro.release nil fact", core)
}

func TestParseLinuxDistroOSRelease_readsDistributionName(t *testing.T) {
	input := "NAME=\"Ubuntu\"\nID=ubuntu\nPRETTY_NAME=\"Ubuntu 24.04.2 LTS\"\nVERSION_ID=\"24.04\"\n"

	got := parseLinuxDistroOSRelease(input)

	if got.Name != "Ubuntu" {
		t.Fatalf("Name = %q, want Ubuntu", got.Name)
	}
	if got.ID != "ubuntu" {
		t.Fatalf("ID = %q, want ubuntu", got.ID)
	}
}

func TestParseLinuxDistroOSRelease_matchesRubyUbuntuFixture(t *testing.T) {
	input := "NAME=\"Ubuntu Linux\"\nVERSION=\"18.04.1 LTS (Bionic Beaver)\"\nID=\nID_LIKE=debian\nPRETTY_NAME=\"Ubuntu 18.04.1 LTS\"\nVERSION_ID=\"18.04\"\nVERSION_CODENAME=bionic\nUBUNTU_CODENAME=bionic\n"

	got := parseLinuxDistroOSRelease(input)

	if got.Name != "Ubuntu" {
		t.Fatalf("Name = %q, want Ubuntu", got.Name)
	}
	if got.ID != "" {
		t.Fatalf("ID = %q, want explicit empty ID", got.ID)
	}
}

func TestParseLinuxDistroOSRelease_extractsDebianCodenameFromVersionWhenVersionCodenameMissing(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("NAME=\"Debian GNU/Linux\"\nID=debian\nVERSION=\"9 (stretch)\"\nVERSION_ID=9\n")
	if got.Codename != "stretch" {
		t.Fatalf("parseLinuxDistroOSRelease().Codename = %q, want stretch", got.Codename)
	}
}

func TestParseLinuxDistroOSRelease_usesRubyDefaultFirstWordName(t *testing.T) {
	t.Parallel()

	got := parseLinuxDistroOSRelease("NAME=\"Debian GNU/Linux\"\nID=debian\nPRETTY_NAME=\"Debian GNU/Linux 10 (buster)\"\nVERSION_ID=10\n")
	if got.Name != "Debian" {
		t.Fatalf("parseLinuxDistroOSRelease().Name = %q, want Debian", got.Name)
	}
	if name := osName("linux", got); name != "Debian" {
		t.Fatalf("osName(linux, debian) = %q, want Debian", name)
	}
}

func TestParseLinuxDistroOSRelease_normalizesOpenSUSELeapID(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"ID=opensuse-leap\n", "ID=\"Opensuse-Leap\"\n"} {
		got := parseLinuxDistroOSRelease(input)
		if got.ID != "opensuse" {
			t.Fatalf("ID = %q, want opensuse for input %q", got.ID, input)
		}
	}
}

func TestParseLinuxDistroOSRelease_keepsUbuntuMajorReleaseAsFullVersion(t *testing.T) {
	input := "NAME=Ubuntu\nID=ubuntu\nVERSION_ID=18.04\n"

	got := parseLinuxDistroOSRelease(input)

	want := map[string]any{"full": "18.04", "major": "18.04"}
	if !reflect.DeepEqual(got.Release, want) {
		t.Fatalf("Release = %#v, want %#v", got.Release, want)
	}
}

func TestParseLinuxDistroOSRelease_unescapesQuotedValues(t *testing.T) {
	input := "NAME=\"Example\\\"Linux\"\nID=example\nPRETTY_NAME=\"Example\\\\Linux\"\nVERSION_ID=\"1.2\"\n"

	got := parseLinuxDistroOSRelease(input)

	if got.Name != `Example"Linux` {
		t.Fatalf("Name = %q, want escaped quote to be unescaped", got.Name)
	}
	if got.Description != `Example\Linux` {
		t.Fatalf("Description = %q, want escaped backslash to be unescaped", got.Description)
	}
}

func TestCurrentLinuxDistro_usesAmazonLinux2023RPMVersionWithPatch(t *testing.T) {
	files := map[string]string{
		"/etc/os-release": "ID=amzn\nVERSION_ID=2023\n",
	}
	readFile := func(path string) ([]byte, error) {
		value, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(value), nil
	}
	run := func(name string, args ...string) string {
		if name != "rpm" || !reflect.DeepEqual(args, []string{"-q", "--qf", "%{NAME}\n%{VERSION}\n%{RELEASE}\n%{VENDOR}", "-f", "/etc/os-release"}) {
			t.Fatalf("run(%q, %#v), want rpm os-release package query", name, args)
		}
		return "system-release\n2023.1.20230912\n1.amzn2023\nAmazon Linux"
	}

	got := currentLinuxDistro("linux", func(string) (string, error) { return "", os.ErrNotExist }, run, readFile)
	want := linuxDistro{
		ID:           "amzn",
		Release:      map[string]any{"full": "2023.1.20230912", "major": "2023", "minor": "1", "patch": "20230912"},
		ReleaseKnown: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxDistro() = %#v, want %#v", got, want)
	}
}

func TestCurrentLinuxDistro_usesAmazonSystemReleaseForDistroFields(t *testing.T) {
	files := map[string]string{
		"/etc/os-release":     "ID=amzn\nVERSION_ID=2\n",
		"/etc/system-release": "Amazon Linux release 2 (2017.12) LTS Release Candidate\n",
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
		ID:           "Amazon",
		Description:  "Amazon Linux release 2 (2017.12) LTS Release Candidate",
		Codename:     "2017.12",
		Release:      map[string]any{"full": "2", "major": "2"},
		ReleaseKnown: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxDistro() = %#v, want %#v", got, want)
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

func TestOSName_usesLinuxDistributionName(t *testing.T) {
	distro := linuxDistro{Name: "Ubuntu", ID: "ubuntu"}

	if got := osName("linux", distro); got != "Ubuntu" {
		t.Fatalf("osName(linux) = %q, want Ubuntu", got)
	}
}

func TestOSName_mapsLinuxMintIDLikeRubyFact(t *testing.T) {
	t.Parallel()

	distro := linuxDistro{ID: "linuxmint"}

	if got := osName("linux", distro); got != "Linuxmint" {
		t.Fatalf("osName(linux) = %q, want Linuxmint", got)
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

func TestCoreFacts_includeRealSystemMemoryTotal(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("memory total resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession))
	memory, ok := collection["memory"].(map[string]any)
	if !ok {
		t.Fatalf("memory fact = %#v, want map", collection["memory"])
	}
	system, ok := memory["system"].(map[string]any)
	if !ok {
		t.Fatalf("memory.system fact = %#v, want map", memory["system"])
	}
	totalBytes, ok := system["total_bytes"].(int)
	if !ok {
		t.Fatalf("memory.system.total_bytes = %#v, want int", system["total_bytes"])
	}
	if totalBytes <= 0 {
		t.Fatalf("memory.system.total_bytes = %d, want positive physical memory", totalBytes)
	}
}

func TestCoreFacts_includeMemorySystemTotal(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("memory total resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession))
	memory, ok := collection["memory"].(map[string]any)
	if !ok {
		t.Fatalf("memory fact = %#v, want map", collection["memory"])
	}
	system, ok := memory["system"].(map[string]any)
	if !ok {
		t.Fatalf("memory.system fact = %#v, want map", memory["system"])
	}
	total, ok := system["total"].(string)
	if !ok || total == "" {
		t.Fatalf("memory.system.total = %#v, want human-readable string", system["total"])
	}
}

func TestCoreFacts_includeMemoryUsageAndSwap(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	memory, ok := collection["memory"].(map[string]any)
	if !ok {
		t.Fatalf("memory fact = %#v, want map", collection["memory"])
	}
	system, ok := memory["system"].(map[string]any)
	if !ok {
		t.Fatalf("memory.system fact = %#v, want map", memory["system"])
	}
	for _, key := range []string{"available", "available_bytes", "capacity", "total", "total_bytes", "used", "used_bytes"} {
		if _, ok := system[key]; !ok {
			t.Fatalf("memory.system = %#v, want key %q", system, key)
		}
	}
	swap, ok := memory["swap"].(map[string]any)
	if !ok {
		// No swap on this host: Ruby Facter omits the swap subtree entirely,
		// so memory must carry no swap key at all (not even encrypted).
		if value, exists := memory["swap"]; exists {
			t.Fatalf("memory.swap = %#v, want subtree omitted on a host without swap", value)
		}
		return
	}
	for _, key := range []string{"available", "available_bytes", "capacity", "total", "total_bytes", "used", "used_bytes"} {
		if _, ok := swap[key]; !ok {
			t.Fatalf("memory.swap = %#v, want key %q", swap, key)
		}
	}
	totalBytes, ok := swap["total_bytes"].(int)
	if !ok || totalBytes <= 0 {
		t.Fatalf("memory.swap.total_bytes = %#v, want positive total when the subtree is present", swap["total_bytes"])
	}
	if _, ok := swap["encrypted"]; !ok {
		t.Fatalf("memory.swap = %#v, want encrypted key", swap)
	}
}

func TestMemorySwapValuesIncludesUsageFieldsWhenSwapExists(t *testing.T) {
	got := memorySwapValues(1024, 512)

	if got.available == nil || got.availableBytes == nil || got.capacity == nil || got.total == nil || got.totalBytes == nil || got.used == nil || got.usedBytes == nil {
		t.Fatalf("memorySwapValues() = %#v, want populated usage fields", got)
	}
}

func TestMemorySwapValuesNoSwapMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	meminfo := "MemTotal: 4037608 kB\nMemAvailable: 3665024 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n"
	got := memorySwapValues(
		parseLinuxMeminfoBytes(meminfo, "SwapTotal"),
		parseLinuxMeminfoBytes(meminfo, "SwapFree"))

	if got.total != nil {
		t.Fatalf("swap total = %#v, want nil", got.total)
	}
	if got.totalBytes != nil {
		t.Fatalf("swap total_bytes = %#v, want nil", got.totalBytes)
	}
	if got.available != nil {
		t.Fatalf("swap available = %#v, want nil", got.available)
	}
	if got.availableBytes != nil {
		t.Fatalf("swap available_bytes = %#v, want nil", got.availableBytes)
	}
	if got.capacity != nil {
		t.Fatalf("swap capacity = %#v, want nil", got.capacity)
	}
	if got.used != nil {
		t.Fatalf("swap used = %#v, want nil", got.used)
	}
	if got.usedBytes != nil {
		t.Fatalf("swap used_bytes = %#v, want nil", got.usedBytes)
	}
}

func TestParseLinuxMeminfoBytes(t *testing.T) {
	input := "MemTotal:       16384000 kB\nMemAvailable:    4096000 kB\nSwapTotal:       1048576 kB\nSwapFree:         262144 kB\n"
	tests := []struct {
		name string
		key  string
		want int
	}{
		{name: "mem total", key: "MemTotal", want: 16_777_216_000},
		{name: "mem available", key: "MemAvailable", want: 4_194_304_000},
		{name: "swap total", key: "SwapTotal", want: 1_073_741_824},
		{name: "swap free", key: "SwapFree", want: 268_435_456},
		{name: "missing", key: "Buffers", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLinuxMeminfoBytes(input, tt.key); got != tt.want {
				t.Fatalf("parseLinuxMeminfoBytes(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

func TestParseLinuxMeminfoBytesFallsBackToFreeBuffersCachedForAvailableMemory(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"MemTotal:        4036680 kB",
		"MemFree:         3547792 kB",
		"Buffers:            4288 kB",
		"Cached:           298624 kB",
		"SwapCached:            0 kB",
	}, "\n")

	if got, want := parseLinuxMeminfoBytes(input, "MemAvailable"), 3_943_120_896; got != want {
		t.Fatalf("parseLinuxMeminfoBytes(MemAvailable fallback) = %d, want %d", got, want)
	}
}

func TestParseDarwinVMStatAvailableBytes(t *testing.T) {
	input := `Mach Virtual Memory Statistics: (page size of 4096 bytes)
Pages free:                             1364873.
Pages active:                           2653007.
Pages inactive:                         1583485.
Pages speculative:                      1061442.
Pages wired down:                        986842.
`

	if got, want := parseDarwinVMStatAvailableBytes(input), 5_590_519_808; got != want {
		t.Fatalf("parseDarwinVMStatAvailableBytes() = %d, want %d", got, want)
	}
}

func TestParseDarwinSwapUsage(t *testing.T) {
	got := parseDarwinSwapUsage("total = 3072.00M  used = 1422.75M  free = 1649.25M  (encrypted)")
	want := darwinSwapUsage{
		TotalBytes:     3_221_225_472,
		UsedBytes:      1_491_861_504,
		AvailableBytes: 1_729_363_968,
		Encrypted:      true,
	}

	if got != want {
		t.Fatalf("parseDarwinSwapUsage() = %#v, want %#v", got, want)
	}
}

func TestCurrentDarwinSwapUsageMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	got := currentDarwinSwapUsage("darwin", func(name string, args ...string) string {
		if name != "sysctl" || !reflect.DeepEqual(args, []string{"-n", "vm.swapusage"}) {
			t.Fatalf("run = %s %v, want sysctl -n vm.swapusage", name, args)
		}
		return "total = 3072.00M  used = 1422.75M  free = 1649.25M  (encrypted)"
	})

	want := darwinSwapUsage{
		TotalBytes:     3_221_225_472,
		UsedBytes:      1_491_861_504,
		AvailableBytes: 1_729_363_968,
		Encrypted:      true,
	}
	if got != want {
		t.Fatalf("currentDarwinSwapUsage() = %#v, want %#v", got, want)
	}
}

func TestCoreFacts_includeSystemUptime(t *testing.T) {
	collection := Collection(CoreFacts(testSession))
	systemUptime, ok := collection["system_uptime"].(map[string]any)
	if !ok {
		t.Fatalf("system_uptime fact = %#v, want map", collection["system_uptime"])
	}

	for _, key := range []string{"days", "hours", "seconds", "uptime"} {
		if systemUptime[key] == nil {
			t.Fatalf("system_uptime = %#v, want key %q", systemUptime, key)
		}
	}
	if collection["uptime_hours"] != nil {
		t.Fatalf("uptime_hours = %#v, want legacy alias hidden from core collection", collection["uptime_hours"])
	}
}

func TestUptimeString_matchesRubyFacterFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		seconds int
		want    string
	}{
		{name: "less than one minute", seconds: 20, want: "0:00 hours"},
		{name: "minutes", seconds: 620, want: "0:10 hours"},
		{name: "hours", seconds: 11_420, want: "3:10 hours"},
		{name: "one day", seconds: 97_820, want: "1 day"},
		{name: "multiple days", seconds: 184_220, want: "2 days"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := uptimeString(uptimeInfo{Duration: time.Duration(tt.seconds) * time.Second, Known: true})
			if got != tt.want {
				t.Fatalf("uptimeString(%d seconds) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestUptimeStringReturnsUnknownWhenSecondsAreUnknown(t *testing.T) {
	t.Parallel()

	if got := uptimeString(uptimeInfo{}); got != "unknown" {
		t.Fatalf("uptimeString(unknown) = %q, want unknown", got)
	}
}

func TestParseUptimeCommandSeconds_matchesRubyFacterUptimeParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "days hours and minutes", input: "10:00AM up 2 days, 1:00, 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "days and hours", input: "10:00AM up 2 days, 1 hr(s), 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "days and minutes", input: "10:00AM up 2 days, 60 min(s), 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "days", input: "10:00AM up 2 days, 1 user, load average: 1.00, 0.75, 0.66", want: 172_800},
		{name: "hours and minutes", input: "10:00AM up 49:00, 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "hours", input: "10:00AM up 49 hr(s), 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "minutes", input: "10:00AM up 2940 mins, 1 user, load average: 1.00, 0.75, 0.66", want: 176_400},
		{name: "invalid", input: "running for 2 days", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseUptimeCommandSeconds(tt.input); got != tt.want {
				t.Fatalf("parseUptimeCommandSeconds(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDockerElapsedTimeSecondsMatchesRubyResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "less than one minute", input: "20", want: 20},
		{name: "minutes", input: "10:20", want: 620},
		{name: "hours", input: "3:10:20", want: 11_420},
		{name: "one day", input: "1-3:10:20", want: 97_820},
		{name: "multiple days", input: "2-3:10:20", want: 184_220},
		{name: "invalid", input: "not uptime", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseDockerElapsedTimeSeconds(tt.input); got != tt.want {
				t.Fatalf("parseDockerElapsedTimeSeconds(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCurrentUptimeFallsBackToKernelBoottime(t *testing.T) {
	t.Parallel()

	readFile := func(path string) ([]byte, error) {
		if path != "/proc/uptime" {
			t.Fatalf("readFile path = %q, want /proc/uptime", path)
		}
		return nil, os.ErrNotExist
	}
	run := func(name string, args ...string) string {
		if name != "sysctl" || !reflect.DeepEqual(args, []string{"-n", "kern.boottime"}) {
			t.Fatalf("run = %s %v, want sysctl -n kern.boottime", name, args)
		}
		return "{ sec = 60, usec = 0 } Tue Oct 10 10:59:00 2019"
	}
	now := func() time.Time { return time.Unix(120, 0) }

	got := currentUptime(testSession, "freebsd", readFile, run, now)
	if got != time.Minute {
		t.Fatalf("currentUptime(testSession) = %s, want 1m0s", got)
	}
}

func TestCurrentLinuxUptimeUsesDockerPIDOneElapsedTime(t *testing.T) {
	t.Parallel()

	got := currentLinuxUptimeInfo(func(path string) ([]byte, error) {
		t.Fatalf("currentLinuxUptimeInfo() read %q, want Docker ps only", path)
		return nil, os.ErrNotExist
	}, func(name string, args ...string) string {
		if name != "ps" || !reflect.DeepEqual(args, []string{"-o", "etime=", "-p", "1"}) {
			t.Fatalf("run = %s %v, want ps -o etime= -p 1", name, args)
		}
		return "1-3:10:20"
	}, time.Now, true)

	if !got.Known || got.Duration != 97_820*time.Second {
		t.Fatalf("currentLinuxUptimeInfo() = %#v, want known 97820s", got)
	}
}

func TestCurrentLinuxUptimeFallsBackWhenDockerElapsedTimeInvalid(t *testing.T) {
	t.Parallel()

	got := currentLinuxUptimeInfo(func(path string) ([]byte, error) {
		if path != "/proc/uptime" {
			t.Fatalf("readFile path = %q, want /proc/uptime", path)
		}
		return []byte("60.00 10.00"), nil
	}, func(name string, args ...string) string {
		if name != "ps" {
			t.Fatalf("run = %s %v, want ps for Docker fallback", name, args)
		}
		return "invalid"
	}, time.Now, true)

	if !got.Known || got.Duration != time.Minute {
		t.Fatalf("currentLinuxUptimeInfo() = %#v, want known 1m", got)
	}
}

func TestCurrentUptimeInfoMarksMissingSourcesUnknown(t *testing.T) {
	t.Parallel()

	readFile := func(path string) ([]byte, error) {
		if path != "/proc/uptime" {
			t.Fatalf("readFile path = %q, want /proc/uptime", path)
		}
		return nil, os.ErrNotExist
	}
	run := func(name string, args ...string) string {
		switch name {
		case "sysctl":
			return ""
		case "uptime":
			return "running for a while"
		default:
			t.Fatalf("unexpected command %q %v", name, args)
			return ""
		}
	}

	got := currentUptimeInfo(testSession, "freebsd", readFile, run, time.Now)
	if got.Known || got.Duration != 0 {
		t.Fatalf("currentUptimeInfo(testSession) = %#v, want unknown zero duration", got)
	}
}

func TestCurrentUptimeUsesWindowsWMITimes(t *testing.T) {
	t.Parallel()

	got := currentUptime(testSession, "windows", func(path string) ([]byte, error) {
		t.Fatalf("currentUptime(testSession, windows) read %q, want WMI only", path)
		return nil, os.ErrNotExist
	}, func(name string, args ...string) string {
		if name != "wmic" {
			t.Fatalf("command = %q %v, want wmic", name, args)
		}
		wantArgs := []string{"os", "get", "LocalDateTime,LastBootUpTime", "/value"}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Fatalf("wmic args = %#v, want %#v", args, wantArgs)
		}
		return "LocalDateTime=20010203045006+0700\r\nLastBootUpTime=20010203030506+0700\r\n"
	}, time.Now)

	if got != 105*time.Minute {
		t.Fatalf("currentUptime(testSession, windows) = %s, want 1h45m0s", got)
	}
}

func TestCurrentUptimeReturnsZeroForInvalidWindowsWMITimes(t *testing.T) {
	t.Parallel()

	got := currentUptime(testSession, "windows", func(string) ([]byte, error) {
		t.Fatal("currentUptime(testSession, windows) read file, want WMI only")
		return nil, os.ErrNotExist
	}, func(string, ...string) string {
		return "LocalDateTime=20010201110506+0700\r\nLastBootUpTime=20010201120506+0700\r\n"
	}, time.Now)

	if got != 0 {
		t.Fatalf("currentUptime(testSession, windows invalid times) = %s, want 0", got)
	}
}

func TestCurrentWindowsUptimeInfoMarksInvalidWMITimesUnknown(t *testing.T) {
	t.Parallel()

	got := currentUptimeInfo(testSession, "windows", func(string) ([]byte, error) {
		t.Fatal("currentUptimeInfo(testSession, windows) read file, want WMI only")
		return nil, os.ErrNotExist
	}, func(string, ...string) string {
		return "LocalDateTime=20010201110506+0700\r\nLastBootUpTime=20010201120506+0700\r\n"
	}, time.Now)

	if got.Known || got.Duration != 0 {
		t.Fatalf("currentUptimeInfo(testSession, windows invalid times) = %#v, want unknown zero duration", got)
	}
}

func TestCurrentWindowsUptimeInfoLogsNoResultDiagnosticsLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	SetDebugHandler(func(message string) { debugMessages = append(debugMessages, message) })
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := currentUptimeInfo(testSession, "windows", func(string) ([]byte, error) {
		t.Fatal("currentUptimeInfo(testSession, windows) read file, want WMI only")
		return nil, os.ErrNotExist
	}, func(string, ...string) string {
		return ""
	}, time.Now)

	if got.Known || got.Duration != 0 {
		t.Fatalf("currentUptimeInfo(testSession, windows empty WMI) = %#v, want unknown zero duration", got)
	}
	want := []string{
		"WMI query returned no resultsfor Win32_OperatingSystem with values LocalDateTime and LastBootUpTime.",
		"Unable to determine system uptime!",
	}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestCurrentWindowsUptimeInfoLogsInvalidDurationLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	SetDebugHandler(func(message string) { debugMessages = append(debugMessages, message) })
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := currentUptimeInfo(testSession, "windows", func(string) ([]byte, error) {
		t.Fatal("currentUptimeInfo(testSession, windows) read file, want WMI only")
		return nil, os.ErrNotExist
	}, func(string, ...string) string {
		return "LocalDateTime=20010201110506+0700\r\nLastBootUpTime=20010201120506+0700\r\n"
	}, time.Now)

	if got.Known || got.Duration != 0 {
		t.Fatalf("currentUptimeInfo(testSession, windows invalid times) = %#v, want unknown zero duration", got)
	}
	want := []string{"Unable to determine system uptime!"}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestParseWindowsWMITimeAcceptsCIMDatetimeOffsetMinutes(t *testing.T) {
	t.Parallel()

	got, ok := parseWindowsWMITime("20010203040506.123456+060")
	if !ok {
		t.Fatal("parseWindowsWMITime() ok = false, want true")
	}
	want := time.Date(2001, 2, 3, 4, 5, 6, 0, time.FixedZone("", 60*60))
	if !got.Equal(want) {
		t.Fatalf("parseWindowsWMITime() = %s, want %s", got, want)
	}
}

func TestCoreFacts_includeLoadAverages(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("load averages resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession))
	loadAverages, ok := collection["load_averages"].(map[string]any)
	if !ok {
		t.Fatalf("load_averages fact = %#v, want map", collection["load_averages"])
	}

	for _, key := range []string{"1m", "5m", "15m"} {
		value, ok := loadAverages[key].(float64)
		if !ok || value < 0 {
			t.Fatalf("load_averages[%q] = %#v, want non-negative float64", key, loadAverages[key])
		}
	}
}

func TestCoreFacts_includeRootMountpoint(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("mountpoints resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession))
	mountpoints, ok := collection["mountpoints"].(map[string]any)
	if !ok {
		t.Fatalf("mountpoints fact = %#v, want map", collection["mountpoints"])
	}
	root, ok := mountpoints["/"].(map[string]any)
	if !ok {
		t.Fatalf("mountpoints[/] = %#v, want root mountpoint", mountpoints["/"])
	}

	for _, key := range []string{"available", "available_bytes", "capacity", "size", "size_bytes", "used", "used_bytes"} {
		if root[key] == nil {
			t.Fatalf("mountpoints[/] = %#v, want key %q", root, key)
		}
	}
	if got := ValueForQuery(ResolvedFact{Name: "mountpoints", Value: mountpoints, UserQuery: "mountpoints./.available.something"}); got != nil {
		t.Fatalf("mountpoints./.available.something = %#v, want nil", got)
	}
}

func TestMountpointsFactIncludesDeviceFilesystemAndOptions(t *testing.T) {
	entries := []mountEntry{
		{Device: "/dev/disk1", Path: "/", Filesystem: "apfs", Options: []string{"rw", "local"}},
		{Device: "proc", Path: "/proc", Filesystem: "proc", Options: []string{"rw"}},
		{Device: "tmpfs", Path: "/proc/acpi", Filesystem: "tmpfs", Options: []string{"rw"}},
		{Device: "auto_home", Path: "/home", Filesystem: "autofs", Options: []string{"automounted"}},
	}
	stats := func(path string) (mountStat, bool) {
		if path != "/" && path != "/proc/acpi" {
			return mountStat{}, false
		}
		return mountStat{SizeBytes: 100, AvailableBytes: 25, UsedBytes: 75}, true
	}

	got := mountpointsFact(entries, stats)
	want := map[string]any{
		"/": map[string]any{
			"available":       "25 bytes",
			"available_bytes": 25,
			"capacity":        "75.00%",
			"device":          "/dev/disk1",
			"filesystem":      "apfs",
			"options":         []string{"rw", "local"},
			"size":            "100 bytes",
			"size_bytes":      100,
			"used":            "75 bytes",
			"used_bytes":      75,
		},
		"/proc/acpi": map[string]any{
			"available":       "25 bytes",
			"available_bytes": 25,
			"capacity":        "75.00%",
			"device":          "tmpfs",
			"filesystem":      "tmpfs",
			"options":         []string{"rw"},
			"size":            "100 bytes",
			"size_bytes":      100,
			"used":            "75 bytes",
			"used_bytes":      75,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mountpointsFact() = %#v, want %#v", got, want)
	}
}

func TestMountpointsFactCapacityMatchesRubyFilesystemHelper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stats mountStat
		want  string
	}{
		{
			name:  "full",
			stats: mountStat{SizeBytes: 100, UsedBytes: 100},
			want:  "100%",
		},
		{
			name:  "empty",
			stats: mountStat{SizeBytes: 100, UsedBytes: 0},
			want:  "0%",
		},
		{
			name:  "partial",
			stats: mountStat{SizeBytes: 10_000, UsedBytes: 421},
			want:  "4.21%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mountpointsFact([]mountEntry{{Path: "/data"}}, func(string) (mountStat, bool) {
				return tt.stats, true
			})
			mountpoint := got["/data"].(map[string]any)
			if mountpoint["capacity"] != tt.want {
				t.Fatalf("capacity = %#v, want %#v", mountpoint["capacity"], tt.want)
			}
		})
	}
}

func TestResolveLinuxRootMountDeviceMatchesRubyResolver(t *testing.T) {
	tests := []struct {
		name    string
		cmdline string
		blkid   string
		want    string
	}{
		{
			name:    "device path",
			cmdline: "console=ttyAMA0 root=/dev/mmcblk0p2 rootfstype=ext4",
			want:    "/dev/mmcblk0p2",
		},
		{
			name:    "missing cmdline root",
			cmdline: "",
			want:    "",
		},
		{
			name:    "partuuid maps through blkid",
			cmdline: "console=tty0 root=PARTUUID=a2f52878-01 rw",
			blkid:   `/dev/xvda1: UUID="f3d" PARTUUID="a2f52878-01"`,
			want:    "/dev/xvda1",
		},
		{
			name:    "partuuid remains when blkid cannot map",
			cmdline: "console=tty0 root=PARTUUID=a2f52878-01 rw",
			blkid:   "blkid: command not found",
			want:    "PARTUUID=a2f52878-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readFile := func(path string) ([]byte, error) {
				if path != "/proc/cmdline" {
					t.Fatalf("readFile path = %q, want /proc/cmdline", path)
				}
				return []byte(tt.cmdline), nil
			}
			run := func(name string, args ...string) string {
				if name != "blkid" || len(args) != 0 {
					t.Fatalf("run = %q %#v, want blkid", name, args)
				}
				return tt.blkid
			}

			got := resolveLinuxRootMountDevice(readFile, run)
			if got != tt.want {
				t.Fatalf("resolveLinuxRootMountDevice() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLinuxMountEntriesReplaceDevRootLikeRubyResolver(t *testing.T) {
	entries := []mountEntry{{Device: "/dev/root", Path: "/", Filesystem: "ext4", Options: []string{"rw", "noatime"}}}
	readFile := func(path string) ([]byte, error) {
		return []byte("console=ttyAMA0 root=/dev/mmcblk0p2 rootfstype=ext4"), nil
	}
	run := func(name string, args ...string) string { return "" }

	got := linuxMountEntriesWithRootDevice(entries, readFile, run)
	if got[0].Device != "/dev/mmcblk0p2" {
		t.Fatalf("device = %q, want /dev/mmcblk0p2", got[0].Device)
	}
}

func TestDarwinMountpointsFactUsesZeroDefaultsWhenStatFails(t *testing.T) {
	entries := []mountEntry{{Device: "/dev/root", Path: "/", Filesystem: "ext4", Options: []string{"rw", "noatime"}}}
	stats := func(string) (mountStat, bool) { return mountStat{}, false }

	got := darwinMountpointsFact(entries, stats)
	want := map[string]any{
		"/": map[string]any{
			"available":       "0 bytes",
			"available_bytes": 0,
			"capacity":        "100%",
			"device":          "/dev/root",
			"filesystem":      "ext4",
			"options":         []string{"rw", "noatime"},
			"size":            "0 bytes",
			"size_bytes":      0,
			"used":            "0 bytes",
			"used_bytes":      0,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("darwinMountpointsFact() = %#v, want %#v", got, want)
	}
}

func TestFreeBSDMountpointsFactParsesMountOutput(t *testing.T) {
	mountOutput := `/dev/ada0p2 on / (ufs, local, journaled soft-updates)
devfs on /dev (devfs)
tmpfs on /tmp/example path (tmpfs, local, nosuid)
`
	stats := func(path string) (mountStat, bool) {
		if path != "/" {
			return mountStat{}, false
		}
		return mountStat{SizeBytes: 466_449_743_872, AvailableBytes: 67_979_685_888, UsedBytes: 374_704_357_376}, true
	}

	got := mountpointsFact(parseFreeBSDMountEntries(mountOutput), stats)
	want := map[string]any{
		"/": map[string]any{
			"available":       "63.31 GiB",
			"available_bytes": 67_979_685_888,
			"capacity":        "80.33%",
			"device":          "/dev/ada0p2",
			"filesystem":      "ufs",
			"options":         []string{"local", "journaled soft-updates"},
			"size":            "434.42 GiB",
			"size_bytes":      466_449_743_872,
			"used":            "348.97 GiB",
			"used_bytes":      374_704_357_376,
		},
		"/dev": map[string]any{
			"device":     "devfs",
			"filesystem": "devfs",
		},
		"/tmp/example path": map[string]any{
			"device":     "tmpfs",
			"filesystem": "tmpfs",
			"options":    []string{"local", "nosuid"},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mountpointsFact(parseFreeBSDMountEntries()) = %#v, want %#v", got, want)
	}
}

func TestOpenBSDMountpointsFactParsesMountAndDFOutput(t *testing.T) {
	mountOutput := `/dev/sd0a on / type ffs (local)
/dev/sd0d on /usr type ffs (local, nodev)
/dev/sd0e on /usr/local type ffs (local, nodev, wxallowed)
`
	dfOutput := `Filesystem   512-blocks       Used   Available Capacity Mounted on
/dev/sd0a       2018844     404488     1513416    21%   /
/dev/sd0d       2018844    1595216      322688    83%   /usr
/dev/sd0e       6082908    3477752     2301012    60%   /usr/local
`

	got := openBSDMountpointsFact(mountOutput, dfOutput)
	want := map[string]any{
		"/": map[string]any{
			"available":       "738.97 MiB",
			"available_bytes": 774_868_992,
			"capacity":        "20.04%",
			"device":          "/dev/sd0a",
			"filesystem":      "ffs",
			"options":         []string{"local"},
			"size":            "985.76 MiB",
			"size_bytes":      1_033_648_128,
			"used":            "197.50 MiB",
			"used_bytes":      207_097_856,
		},
		"/usr": map[string]any{
			"available":       "157.56 MiB",
			"available_bytes": 165_216_256,
			"capacity":        "79.02%",
			"device":          "/dev/sd0d",
			"filesystem":      "ffs",
			"options":         []string{"local", "nodev"},
			"size":            "985.76 MiB",
			"size_bytes":      1_033_648_128,
			"used":            "778.91 MiB",
			"used_bytes":      816_750_592,
		},
		"/usr/local": map[string]any{
			"available":       "1.10 GiB",
			"available_bytes": 1_178_118_144,
			"capacity":        "57.17%",
			"device":          "/dev/sd0e",
			"filesystem":      "ffs",
			"options":         []string{"local", "nodev", "wxallowed"},
			"size":            "2.90 GiB",
			"size_bytes":      3_114_448_896,
			"used":            "1.66 GiB",
			"used_bytes":      1_780_609_024,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("openBSDMountpointsFact() = %#v, want %#v", got, want)
	}
}

func TestParseDarwinMountEntries(t *testing.T) {
	input := "/dev/disk3s1s1 on / (apfs, sealed, local, read-only, journaled)\nmap auto_home on /System/Volumes/Data/home (autofs, automounted, nobrowse)\nserver:/Shared\\040Data on /Volumes/Shared\\040Data (nfs, nodev, nosuid)\n"

	got := parseDarwinMountEntries(input)
	want := []mountEntry{
		{Device: "/dev/disk3s1s1", Path: "/", Filesystem: "apfs", Options: []string{"sealed", "local", "readonly", "journaled"}},
		{Device: "map auto_home", Path: "/System/Volumes/Data/home", Filesystem: "autofs", Options: []string{"automounted", "nobrowse"}},
		{Device: "server:/Shared Data", Path: "/Volumes/Shared Data", Filesystem: "nfs", Options: []string{"nodev", "nosuid"}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDarwinMountEntries() = %#v, want %#v", got, want)
	}
}

func TestParseDarwinMountEntriesNormalizesRubyOptionAliases(t *testing.T) {
	input := "/dev/disk3s1 on / (apfs, read-only, asynchronous, synchronous, quotas, rootfs, defwrite, nodev)\n"

	got := parseDarwinMountEntries(input)
	want := []mountEntry{{
		Device:     "/dev/disk3s1",
		Path:       "/",
		Filesystem: "apfs",
		Options:    []string{"readonly", "async", "noasync", "quota", "root", "deferwrites", "nodev"},
	}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDarwinMountEntries() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxLoadAverages(t *testing.T) {
	got := parseLoadAverages("0.00 0.03 0.03 1/1153 20372")
	want := map[string]any{"1m": 0.00, "5m": 0.03, "15m": 0.03}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestParseMacOSLoadAverages(t *testing.T) {
	got := parseLoadAverages("{ 2.10 2.20 2.30 }")
	want := map[string]any{"1m": 2.10, "5m": 2.20, "15m": 2.30}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestCurrentLoadAverages_wiresBSDVMLoadavg(t *testing.T) {
	for _, goos := range []string{"freebsd", "openbsd"} {
		t.Run(goos, func(t *testing.T) {
			got := currentLoadAverages(goos, nil, func(path string, args ...string) string {
				if path != "sysctl" || !reflect.DeepEqual(args, []string{"-n", "vm.loadavg"}) {
					t.Fatalf("run(%q, %#v), want sysctl -n vm.loadavg", path, args)
				}
				return "{ 0.01 0.02 0.03 }"
			})
			want := map[string]any{"1m": 0.01, "5m": 0.02, "15m": 0.03}

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("currentLoadAverages() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestCurrentLoadAverages_wiresDarwinVMLoadavg(t *testing.T) {
	got := currentLoadAverages("darwin", nil, func(path string, args ...string) string {
		if path != "sysctl" || !reflect.DeepEqual(args, []string{"-n", "vm.loadavg"}) {
			t.Fatalf("run(%q, %#v), want sysctl -n vm.loadavg", path, args)
		}
		return "{ 0.00 0.03 0.03 }"
	})
	want := map[string]any{"1m": 0.00, "5m": 0.03, "15m": 0.03}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestCurrentLoadAverages_linuxUnreadableProcLoadavgMatchesRubyResolver(t *testing.T) {
	got := currentLoadAverages("linux", func(path string) ([]byte, error) {
		if path != "/proc/loadavg" {
			t.Fatalf("readFile(%q), want /proc/loadavg", path)
		}
		return nil, os.ErrPermission
	}, nil)
	want := map[string]any{"1m": nil, "5m": nil, "15m": nil}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestParseLoadAveragesInvalidInput(t *testing.T) {
	got := parseLoadAverages("not enough")
	want := map[string]any{"1m": nil, "5m": nil, "15m": nil}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLoadAverages() = %#v, want %#v", got, want)
	}
}

func TestBytesToMB(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  any
	}{
		{"bytes", 256_586_343, 244.6998052597046},
		{"string bytes", "2343455", 2.2348928451538086},
		{"string bytes with suffix", "2343455abc", 2.2348928451538086},
		{"non numeric string", "not-a-number", 0.0},
		{"zero", 0, 0.0},
		{"nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bytesToMB(tt.value); got != tt.want {
				t.Fatalf("bytesToMB(%#v) = %#v, want %#v", tt.value, got, tt.want)
			}
		})
	}
}

func TestBytesToHumanReadable(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  any
	}{
		{"nil", nil, nil},
		{"zero", 0, "0 bytes"},
		{"rounds to next unit", 1_048_575.7, "1.00 MiB"},
		{"bytes", 1023, "1023 bytes"},
		{"kibibyte", 1024, "1.00 KiB"},
		{"too large", "3296472651763232323235", "3296472651763232323235 bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bytesToHumanReadable(tt.value); got != tt.want {
				t.Fatalf("bytesToHumanReadable(%#v) = %#v, want %#v", tt.value, got, tt.want)
			}
		})
	}
}

func TestHertzToHumanReadable(t *testing.T) {
	tests := []struct {
		name string
		hz   any
		want string
	}{
		{"zero", 0, ""},
		{"non numeric string", "test", ""},
		{"gigahertz", 2_300_000_000, "2.30 GHz"},
		{"megahertz", 800_000_000, "800.00 MHz"},
		{"hertz", 42, "42.00 Hz"},
		{"string kilohertz", "2400", "2.40 kHz"},
		{"rounds down", 2_363_000_000, "2.36 GHz"},
		{"rounds half up", 2_365_000_000, "2.37 GHz"},
		{"rounds up", 2_367_000_000, "2.37 GHz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hertzToHumanReadable(tt.hz); got != tt.want {
				t.Fatalf("hertzToHumanReadable(%#v) = %q, want %q", tt.hz, got, tt.want)
			}
		})
	}
}

func TestParseLinuxProcessorSpeed(t *testing.T) {
	input := "processor\t: 0\nmodel name\t: CPU\ncpu MHz\t\t: 1800.000\n"

	if got, want := parseLinuxProcessorSpeed(input), "1.80 GHz"; got != want {
		t.Fatalf("parseLinuxProcessorSpeed() = %q, want %q", got, want)
	}
}

func TestParseLinuxProcessorModels(t *testing.T) {
	input := "processor\t: 0\nmodel name\t: Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz\n" +
		"processor\t: 1\nmodel name\t: Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz\n"

	got := parseLinuxProcessorModels(input)
	want := []string{
		"Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz",
		"Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxProcessorModels() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxProcessorModelsMatchesRubyPowerPCCPUKey(t *testing.T) {
	input := "processor\t: 0\ncpu\t\t: POWER8 (raw), altivec supported\n" +
		"processor\t: 1\ncpu\t\t: POWER8 (raw), altivec supported\n"

	got := parseLinuxProcessorModels(input)
	want := []string{
		"POWER8 (raw), altivec supported",
		"POWER8 (raw), altivec supported",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxProcessorModels() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxProcessorTopology(t *testing.T) {
	input := "processor\t: 0\nsiblings\t: 8\ncpu cores\t: 4\n"

	cores, threads := parseLinuxProcessorTopology(input)
	if cores != 4 || threads != 2 {
		t.Fatalf("parseLinuxProcessorTopology() = %d, %d, want 4, 2", cores, threads)
	}
}

func TestParseLinuxProcessorTopologyMatchesRubyLscpuResolver(t *testing.T) {
	input := "'Thread(s) per core': 2\n'Cores(s) per socket': 1\n"

	cores, threads := parseLinuxProcessorTopology(input)
	if cores != 1 || threads != 2 {
		t.Fatalf("parseLinuxProcessorTopology() = %d, %d, want 1, 2", cores, threads)
	}
}

func TestLinuxProcessorPhysicalCountFallsBackToSysfsPackageIDsLikeRuby(t *testing.T) {
	sysCPU := t.TempDir()
	for cpu, packageID := range map[string]string{"cpu0": "0", "cpu1": "1"} {
		topology := filepath.Join(sysCPU, cpu, "topology")
		if err := os.MkdirAll(topology, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", topology, err)
		}
		if err := os.WriteFile(filepath.Join(topology, "physical_package_id"), []byte(packageID), 0o644); err != nil {
			t.Fatalf("WriteFile package id: %v", err)
		}
	}
	if err := os.Mkdir(filepath.Join(sysCPU, "cpuindex"), 0o755); err != nil {
		t.Fatalf("Mkdir cpuindex: %v", err)
	}

	cpuinfo := "processor\t: 0\nmodel name\t: CPU\nprocessor\t: 1\nmodel name\t: CPU\n"
	if got, want := linuxProcessorPhysicalCount(cpuinfo, sysCPU), 2; got != want {
		t.Fatalf("linuxProcessorPhysicalCount() = %d, want %d", got, want)
	}
}

func TestParseFreeBSDMemory_returnsRubyCompatibleSystemAndSwapFacts(t *testing.T) {
	sysctlValues := map[string]int{
		"vm.stats.vm.v_page_size":    4096,
		"vm.stats.vm.v_page_count":   4_161_024,
		"vm.stats.vm.v_active_count": 2_335_139,
		"vm.stats.vm.v_wire_count":   1_167_569,
	}
	swapinfo := strings.Join([]string{
		"Device          1K-blocks     Used    Avail Capacity",
		"/dev/ada0p2.eli   2097152        0  2097152     0%",
		"/dev/ada1p2.eli   2097152        0  2097152     0%",
	}, "\n")

	got := parseFreeBSDMemory(sysctlValues, swapinfo)
	want := freeBSDMemoryInfo{
		System: map[string]any{
			"available_bytes": 2_696_462_336,
			"capacity":        "84.18%",
			"total_bytes":     17_043_554_304,
			"used_bytes":      14_347_091_968,
		},
		Swap: map[string]any{
			"available_bytes": 4_294_967_296,
			"capacity":        "0%",
			"encrypted":       true,
			"total_bytes":     4_294_967_296,
			"used_bytes":      0,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFreeBSDMemory() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxProcessorExtensions_derivesX86Levels(t *testing.T) {
	input := "flags : fpu cx8 cmov mmx fxsr sse2 syscall lm cx16 lahf_lm popcnt sse4_1 sse4_2 ssse3 abm avx avx2 bmi1 bmi2 f16c fma movbe xsave\n"

	got := parseLinuxProcessorExtensions(input, "x86_64")
	want := []string{"x86_64", "x86_64-v1", "x86_64-v2", "x86_64-v3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxProcessorExtensions() = %#v, want %#v", got, want)
	}
}
