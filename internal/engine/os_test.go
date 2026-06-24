package engine

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	targets "github.com/ncode/facts/internal/platform"
)

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

func TestCurrentWindowsKernelReportsStructuredComponents(t *testing.T) {
	t.Parallel()

	name, release, version, ok := currentWindowsKernel("OtherTypeDescription=\r\nProductType=1\r\nVersion=10.0.22631\r\n", discardLog())
	if !ok {
		t.Fatalf("currentWindowsKernel() ok = false, want true")
	}
	if name != "windows" || release != "10.0.22631" || version != "10.0.22631" {
		t.Fatalf("currentWindowsKernel() = (%q, %q, %q), want (windows, 10.0.22631, 10.0.22631)", name, release, version)
	}

	facts := kernelFacts(name, release, version)
	want := []ResolvedFact{
		{Name: "kernel.name", Value: "windows"},
		{Name: "kernel.release.full", Value: "10.0.22631"},
		{Name: "kernel.version.full", Value: "10.0.22631"},
		{Name: "kernel.release.major", Value: "10"},
		{Name: "kernel.release.minor", Value: "0"},
		{Name: "kernel.release.patch", Value: "22631"},
	}
	if !reflect.DeepEqual(facts, want) {
		t.Fatalf("kernelFacts() = %#v, want %#v", facts, want)
	}
}

func TestCurrentWindowsKernelLogsFailureLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	logger := captureLogger(&debugMessages, nil, nil)

	if _, _, _, ok := currentWindowsKernel("", logger); ok {
		t.Fatalf("currentWindowsKernel(empty) ok = true, want false")
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

func TestCurrentOSReleaseNetBSDUsesKernelReleaseMap(t *testing.T) {
	got := currentOSRelease(testSession, "netbsd", nil, func(name string, args ...string) string {
		if name != "uname" || !reflect.DeepEqual(args, []string{"-r"}) {
			t.Fatalf("command = %s %#v, want uname -r", name, args)
		}
		return "10.1\n"
	})

	want := map[string]any{"full": "10.1", "major": "10", "minor": "1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentOSRelease(testSession, netbsd) = %#v, want %#v", got, want)
	}
}

func TestCurrentOSReleaseDragonFlyUsesKernelReleaseMap(t *testing.T) {
	got := currentOSRelease(testSession, "dragonfly", nil, func(name string, args ...string) string {
		if name != "uname" || !reflect.DeepEqual(args, []string{"-r"}) {
			t.Fatalf("command = %s %#v, want uname -r", name, args)
		}
		return "6.4-RELEASE\n"
	})

	want := map[string]any{"full": "6.4-RELEASE", "major": "6", "minor": "4-RELEASE"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentOSRelease(testSession, dragonfly) = %#v, want %#v", got, want)
	}
}

func TestCurrentOSReleaseIllumosUsesEtcRelease(t *testing.T) {
	got := currentOSRelease(testSession, "illumos", func(path string) ([]byte, error) {
		if path != "/etc/release" {
			t.Fatalf("path = %q, want /etc/release", path)
		}
		return []byte("  OmniOS v11 r151058\n"), nil
	}, nil)

	want := map[string]any{"full": "r151058", "major": "151058"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentOSRelease(testSession, illumos) = %#v, want %#v", got, want)
	}
}

func TestCurrentOSReleaseIllumosScansPastBannerLines(t *testing.T) {
	got := currentOSRelease(testSession, "illumos", func(path string) ([]byte, error) {
		return []byte("\n  OpenIndiana Development\n  OpenIndiana Hipster r202510\n"), nil
	}, nil)

	want := map[string]any{"full": "r202510", "major": "202510"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentOSRelease(testSession, illumos) = %#v, want %#v", got, want)
	}
}

func TestCurrentOSReleaseIllumosFallsBackToKernelRelease(t *testing.T) {
	host := &fakeHostOS{runOutput: "5.11\n"}
	session := NewSession()
	session.host = host

	got := currentOSRelease(session, "illumos", func(path string) ([]byte, error) {
		return []byte("OpenIndiana Hipster\n"), nil
	}, session.commandOutput)

	if got != "5.11" {
		t.Fatalf("currentOSRelease(session, illumos) = %#v, want %q", got, "5.11")
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

func TestParseLinuxOSRelease_keepsGenericMajorReleaseAsFullVersion(t *testing.T) {
	got := parseLinuxOSRelease("ID=ubuntu\nVERSION_ID=10.9\n")

	want := map[string]any{"full": "10.9", "major": "10.9"}
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

func TestParseLinuxOSRelease_splitsNixOSVersionID(t *testing.T) {
	got := parseLinuxOSRelease("ID=nixos\nVERSION_ID=26.05\n")

	want := map[string]any{"full": "26.05", "major": "26", "minor": "05"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxOSRelease() = %#v, want %#v", got, want)
	}
}

func TestParseLinuxOSRelease_splitsRockyAndAlmaVersionID(t *testing.T) {
	tests := []struct {
		id string
	}{
		{id: "rocky"},
		{id: "almalinux"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := parseLinuxOSRelease("ID=" + tt.id + "\nVERSION_ID=9.8\n")

			want := map[string]any{"full": "9.8", "major": "9", "minor": "8"}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("parseLinuxOSRelease() = %#v, want %#v", got, want)
			}
		})
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
		{goos: "dragonfly", want: "DragonFly"},
		{goos: "illumos", want: "illumos"},
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

func TestOSName_mapsBSDLikeRubyFact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos string
		want string
	}{
		{goos: "freebsd", want: "FreeBSD"},
		{goos: "netbsd", want: "NetBSD"},
		{goos: "openbsd", want: "OpenBSD"},
		{goos: "dragonfly", want: "DragonFly"},
		{goos: "illumos", want: "OmniOS"},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()

			distro := linuxDistro{}
			if tt.goos == "illumos" {
				distro.Name = "OmniOS"
			}
			if got := osName(tt.goos, distro); got != tt.want {
				t.Fatalf("osName(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestKernelName_mapsBSDLikeRubyFact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos string
		want string
	}{
		{goos: "freebsd", want: "FreeBSD"},
		{goos: "netbsd", want: "NetBSD"},
		{goos: "openbsd", want: "OpenBSD"},
		{goos: "dragonfly", want: "DragonFly"},
		{goos: "illumos", want: "SunOS"},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()

			if got := kernelName(tt.goos); got != tt.want {
				t.Fatalf("kernelName(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestOSIdentityHelpersUseTargetProfileDefaults(t *testing.T) {
	t.Parallel()

	for _, profile := range targets.Profiles() {
		if profile.ID == "linux" {
			continue
		}
		t.Run(profile.ID, func(t *testing.T) {
			t.Parallel()

			if got := osFamily(profile.ID, linuxDistro{}); got != profile.Identity.OSFamily {
				t.Fatalf("osFamily(%q) = %q, want profile OS family %q", profile.ID, got, profile.Identity.OSFamily)
			}
			if got := osName(profile.ID, linuxDistro{}); got != profile.Identity.OSName {
				t.Fatalf("osName(%q) = %q, want profile OS name %q", profile.ID, got, profile.Identity.OSName)
			}
			if got := kernelName(profile.ID); got != profile.Identity.KernelName {
				t.Fatalf("kernelName(%q) = %q, want profile kernel name %q", profile.ID, got, profile.Identity.KernelName)
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

	got := parseLinuxDistroOSRelease("NAME=\"Arch Linux\"\nID=arch\nBUILD_ID=rolling\nPRETTY_NAME=\"Arch Linux\"\n")
	if got.Name != "Archlinux" {
		t.Fatalf("parseLinuxDistroOSRelease().Name = %q, want Archlinux", got.Name)
	}
	if len(got.Release) != 0 || got.ReleaseKnown {
		t.Fatalf("parseLinuxDistroOSRelease().Release = %#v, ReleaseKnown = %v, want no release from BUILD_ID", got.Release, got.ReleaseKnown)
	}
	if name := osName("linux", got); name != "Archlinux" {
		t.Fatalf("osName(linux, arch) = %q, want Archlinux", name)
	}
}

func TestCurrentOSRelease_omitsArchRollingBuildID(t *testing.T) {
	t.Parallel()

	readFile := func(path string) ([]byte, error) {
		if path != "/etc/os-release" {
			return nil, os.ErrNotExist
		}
		return []byte("NAME=\"Arch Linux\"\nPRETTY_NAME=\"Arch Linux\"\nID=arch\nBUILD_ID=rolling\n"), nil
	}

	got := currentOSRelease(testSession, "linux", readFile, func(string, ...string) string { return "" })
	if got != nil {
		t.Fatalf("currentOSRelease(testSession, arch) = %#v, want nil because BUILD_ID is not a release", got)
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

func TestKernelReleaseComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		kernelRelease string
		major         string
		minor         string
		patch         string
		patchOK       bool
	}{
		{name: "darwin three components", kernelRelease: "25.5.0", major: "25", minor: "5", patch: "0", patchOK: true},
		{name: "linux package suffix", kernelRelease: "4.15.0-109-generic", major: "4", minor: "15", patch: "0", patchOK: true},
		{name: "bsd release suffix without patch", kernelRelease: "12.1-RELEASE-p3", major: "12", minor: "1", patchOK: false},
		{name: "two components only", kernelRelease: "7.9", major: "7", minor: "9", patchOK: false},
		{name: "no dot delimiter", kernelRelease: "4test", major: "4", patchOK: false},
		{name: "non numeric", kernelRelease: "generic", patchOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			major, minor, patch, patchOK := kernelReleaseComponents(tt.kernelRelease)
			if major != tt.major || minor != tt.minor || patch != tt.patch || patchOK != tt.patchOK {
				t.Fatalf("kernelReleaseComponents(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
					tt.kernelRelease, major, minor, patch, patchOK, tt.major, tt.minor, tt.patch, tt.patchOK)
			}
		})
	}
}

func TestKernelFacts_structuredShape(t *testing.T) {
	t.Parallel()

	got := kernelFacts("Darwin", "25.5.0", "25.5.0")
	want := []ResolvedFact{
		{Name: "kernel.name", Value: "Darwin"},
		{Name: "kernel.release.full", Value: "25.5.0"},
		{Name: "kernel.version.full", Value: "25.5.0"},
		{Name: "kernel.release.major", Value: "25"},
		{Name: "kernel.release.minor", Value: "5"},
		{Name: "kernel.release.patch", Value: "0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kernelFacts() = %#v, want %#v", got, want)
	}
}

func TestKernelFacts_omitsPatchWhenAbsent(t *testing.T) {
	t.Parallel()

	got := kernelFacts("FreeBSD", "12.1-RELEASE-p3", "12.1")
	for _, fact := range got {
		if fact.Name == "kernel.release.patch" {
			t.Fatalf("kernelFacts(%q) emitted kernel.release.patch = %#v, want it omitted", "12.1-RELEASE-p3", fact.Value)
		}
	}
	want := []ResolvedFact{
		{Name: "kernel.name", Value: "FreeBSD"},
		{Name: "kernel.release.full", Value: "12.1-RELEASE-p3"},
		{Name: "kernel.version.full", Value: "12.1"},
		{Name: "kernel.release.major", Value: "12"},
		{Name: "kernel.release.minor", Value: "1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kernelFacts() = %#v, want %#v", got, want)
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
