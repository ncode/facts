package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func plan9Fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "plan9", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPlan9HostnameTrimsSysname(t *testing.T) {
	t.Parallel()

	if got := parsePlan9Sysname(plan9Fixture(t, "sysname")); got != "cirno" {
		t.Fatalf("parsePlan9Sysname() = %q, want cirno", got)
	}
	if got := parsePlan9Sysname("\n"); got != "" {
		t.Fatalf("parsePlan9Sysname(empty) = %q, want empty", got)
	}
}

func TestPlan9ArchitecturePrefersObjtypeEnvFile(t *testing.T) {
	t.Parallel()

	readFile := func(path string) ([]byte, error) {
		if path != "/env/objtype" {
			return nil, os.ErrNotExist
		}
		return []byte("amd64\x00"), nil
	}
	if got := plan9Architecture(readFile, "arm64"); got != "amd64" {
		t.Fatalf("plan9Architecture() = %q, want amd64", got)
	}
	if got := plan9Architecture(func(string) ([]byte, error) { return nil, os.ErrNotExist }, "arm64"); got != "arm64" {
		t.Fatalf("plan9Architecture(missing) = %q, want runtime fallback", got)
	}
}

func TestPlan9ProcessorISAPrefersCputypeThenObjtype(t *testing.T) {
	t.Parallel()

	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/env/cputype":
			return []byte("386\x00"), nil
		case "/env/objtype":
			return []byte("amd64\x00"), nil
		default:
			return nil, os.ErrNotExist
		}
	}
	if got := plan9ProcessorISA(readFile, "arm64"); got != "386" {
		t.Fatalf("plan9ProcessorISA() = %q, want cputype 386", got)
	}

	readFile = func(path string) ([]byte, error) {
		switch path {
		case "/env/cputype":
			return nil, os.ErrNotExist
		case "/env/objtype":
			return []byte("amd64\x00"), nil
		default:
			return nil, os.ErrNotExist
		}
	}
	if got := plan9ProcessorISA(readFile, "arm64"); got != "amd64" {
		t.Fatalf("plan9ProcessorISA(cputype missing) = %q, want objtype amd64", got)
	}
}

func TestParsePlan9SwapMemoryTotal(t *testing.T) {
	t.Parallel()

	if got := parsePlan9SwapMemoryTotal(plan9Fixture(t, "swap")); got != 1_067_843_584 {
		t.Fatalf("parsePlan9SwapMemoryTotal() = %d, want 1067843584", got)
	}
	if got := parsePlan9SwapMemoryTotal("4096 pagesize\n"); got != 0 {
		t.Fatalf("parsePlan9SwapMemoryTotal(no memory) = %d, want 0", got)
	}
}

func TestParsePlan9SysstatProcessorCount(t *testing.T) {
	t.Parallel()

	if got := parsePlan9SysstatProcessorCount(plan9Fixture(t, "sysstat")); got != 1 {
		t.Fatalf("parsePlan9SysstatProcessorCount() = %d, want 1", got)
	}
	if got := parsePlan9SysstatProcessorCount("0 1 2\n\n1 2 3\n"); got != 2 {
		t.Fatalf("parsePlan9SysstatProcessorCount(two lines) = %d, want 2", got)
	}
}

func TestParsePlan9ProcessorModels(t *testing.T) {
	t.Parallel()

	got := parsePlan9ProcessorModels(plan9Fixture(t, "cputype"), plan9Fixture(t, "archctl"), 1)
	want := []string{"Core 2/Xeon 3600"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePlan9ProcessorModels() = %#v, want %#v", got, want)
	}

	got = parsePlan9ProcessorModels("", plan9Fixture(t, "archctl"), 2)
	want = []string{"Core 2/Xeon 3600", "Core 2/Xeon 3600"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePlan9ProcessorModels(archctl) = %#v, want %#v", got, want)
	}

	got = parsePlan9ProcessorModels("", "cpu\n", 1)
	if got != nil {
		t.Fatalf("parsePlan9ProcessorModels(empty archctl model) = %#v, want nil", got)
	}

	got = parsePlan9ProcessorModels("386\n", "", 0)
	want = []string{"386"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePlan9ProcessorModels(default count) = %#v, want %#v", got, want)
	}
}

func TestCurrentPlan9ProcessorInfoReadsInjectedFiles(t *testing.T) {
	t.Parallel()

	files := map[string][]byte{
		"/dev/sysstat": []byte("0 1 2\n\n1 2 3\n"),
		"/dev/cputype": []byte("\n"),
		"/dev/archctl": []byte(plan9Fixture(t, "archctl")),
	}
	seen := map[string]bool{}
	got := currentPlan9ProcessorInfo(func(path string) ([]byte, error) {
		seen[path] = true
		data, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return data, nil
	})
	want := processorInfo{
		LogicalCount: 2,
		Models:       []string{"Core 2/Xeon 3600", "Core 2/Xeon 3600"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentPlan9ProcessorInfo() = %#v, want %#v", got, want)
	}
	for _, path := range []string{"/dev/sysstat", "/dev/cputype", "/dev/archctl"} {
		if !seen[path] {
			t.Fatalf("currentPlan9ProcessorInfo() did not read %s", path)
		}
	}
}

func TestParsePlan9IPIFCStatus(t *testing.T) {
	t.Parallel()

	got := parsePlan9IPIFCStatus(plan9Fixture(t, "ipifc_status"))
	want := map[string]any{
		"ether0": map[string]any{
			"bindings": []any{map[string]any{
				"address": "192.168.122.163",
				"netmask": "255.255.255.0",
				"network": "192.168.122.0",
			}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePlan9IPIFCStatus() = %#v, want %#v", got, want)
	}
}

func TestParsePlan9IPIFCStatusSkipsInvalidDevicePath(t *testing.T) {
	t.Parallel()

	got := parsePlan9IPIFCStatus("device .\n\t192.0.2.10 /120 192.0.2.0 0 0\n")
	if got != nil {
		t.Fatalf("parsePlan9IPIFCStatus(invalid device) = %#v, want nil", got)
	}
}

func TestPlan9IPv4MappedPrefixConversion(t *testing.T) {
	t.Parallel()

	if got := plan9IPv4Prefix(120); got != 24 {
		t.Fatalf("plan9IPv4Prefix(120) = %d, want 24", got)
	}
	if got := plan9IPv4Prefix(24); got != 24 {
		t.Fatalf("plan9IPv4Prefix(24) = %d, want 24", got)
	}
}

func TestParsePlan9MACAddress(t *testing.T) {
	t.Parallel()

	if got := parsePlan9MACAddress(plan9Fixture(t, "ether0_addr")); got != "52:54:00:76:cc:6d" {
		t.Fatalf("parsePlan9MACAddress() = %q, want colon-separated MAC", got)
	}
}

func TestParsePlan9PrimaryRouteIP(t *testing.T) {
	t.Parallel()

	if got := parsePlan9PrimaryRouteIP(plan9Fixture(t, "iproute")); got != "192.168.122.163" {
		t.Fatalf("parsePlan9PrimaryRouteIP() = %q, want 192.168.122.163", got)
	}
}

func TestParsePlan9PrimaryRouteIPPrefersHostRouteThenCandidate(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"0.0.0.0 0.0.0.0 /0 4 3 ifc 192.168.122.44 /120",
		"0.0.0.0 0.0.0.0 /0 4 3 ifc 192.168.122.55 /128",
	}, "\n")
	if got := parsePlan9PrimaryRouteIP(input); got != "192.168.122.55" {
		t.Fatalf("parsePlan9PrimaryRouteIP(host route) = %q, want 192.168.122.55", got)
	}

	input = strings.Join([]string{
		"0.0.0.0 0.0.0.0 /0 4 3 ifc 0.0.0.0 /128",
		"0.0.0.0 0.0.0.0 /0 4 3 ifc 192.168.122.44 /120",
	}, "\n")
	if got := parsePlan9PrimaryRouteIP(input); got != "192.168.122.44" {
		t.Fatalf("parsePlan9PrimaryRouteIP(candidate) = %q, want 192.168.122.44", got)
	}
}

func TestPlan9PrimaryInterfaceFallsBackWhenRouteDoesNotMatch(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"ether0": map[string]any{"bindings": []any{map[string]any{"address": "192.168.122.44"}}},
		"ether1": map[string]any{"bindings": []any{map[string]any{"address": "198.51.100.10"}}},
	}
	route := "0.0.0.0 0.0.0.0 /0 4 3 ifc 203.0.113.10 /128\n"

	if got := plan9PrimaryInterface(route, interfaces); got != "ether0" {
		t.Fatalf("plan9PrimaryInterface() = %q, want first non-ignored interface", got)
	}
}

func TestCurrentPlan9InterfacesMergesStatusAndMAC(t *testing.T) {
	t.Parallel()

	files := map[string][]byte{
		"/net/ipifc/0/status": []byte(plan9Fixture(t, "ipifc_status")),
		"/net/ether0/addr":    []byte(plan9Fixture(t, "ether0_addr")),
	}
	got := currentPlan9Interfaces(func(path string) ([]byte, error) {
		data, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return data, nil
	}, func(pattern string) ([]string, error) {
		if pattern != "/net/ipifc/*/status" {
			t.Fatalf("glob pattern = %q, want /net/ipifc/*/status", pattern)
		}
		return []string{"/net/ipifc/0/status"}, nil
	})

	want := map[string]any{
		"ether0": map[string]any{
			"mac": "52:54:00:76:cc:6d",
			"bindings": []any{map[string]any{
				"address": "192.168.122.163",
				"netmask": "255.255.255.0",
				"network": "192.168.122.0",
			}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentPlan9Interfaces() = %#v, want %#v", got, want)
	}
}

func TestPlan9NetworkingCoreFactsUseInjectedHostFiles(t *testing.T) {
	t.Parallel()

	s := NewSession()
	host := &fakeHostOS{
		files: map[string][]byte{
			"/dev/sysname":        []byte(plan9Fixture(t, "sysname")),
			"/net/ipifc/0/status": []byte(plan9Fixture(t, "ipifc_status")),
			"/net/ether0/addr":    []byte(plan9Fixture(t, "ether0_addr")),
			"/net/iproute":        []byte(plan9Fixture(t, "iproute")),
		},
		globs: map[string][]string{
			"/net/ipifc/*/status": {"/net/ipifc/0/status"},
		},
	}
	s.host = host
	facts := Collection(plan9NetworkingCoreFacts(s))
	if want := []string{"/net/ipifc/*/status"}; !reflect.DeepEqual(host.globCalls, want) {
		t.Fatalf("glob calls = %#v, want %#v", host.globCalls, want)
	}
	networking, ok := facts["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking = %#v, want map", facts["networking"])
	}

	for key, want := range map[string]any{
		"hostname": "cirno",
		"primary":  "ether0",
		"ip":       "192.168.122.163",
		"mac":      "52:54:00:76:cc:6d",
		"netmask":  "255.255.255.0",
		"network":  "192.168.122.0",
	} {
		if got := networking[key]; got != want {
			t.Fatalf("networking.%s = %#v, want %#v", key, got, want)
		}
	}
	interfaces, ok := networking["interfaces"].(map[string]any)
	if !ok {
		t.Fatalf("networking.interfaces = %#v, want map", networking["interfaces"])
	}
	ether0, ok := interfaces["ether0"].(map[string]any)
	if !ok {
		t.Fatalf("networking.interfaces.ether0 = %#v, want map", interfaces["ether0"])
	}
	if got := ether0["ip"]; got != "192.168.122.163" {
		t.Fatalf("ether0.ip = %#v, want 192.168.122.163", got)
	}
}

func TestPlan9NetworkingCoreFactsUsesSessionGlob(t *testing.T) {
	t.Parallel()

	s := NewSession()
	s.host = &fakeHostOS{
		files: map[string][]byte{
			"/dev/sysname":        []byte(plan9Fixture(t, "sysname")),
			"/net/ipifc/0/status": []byte(plan9Fixture(t, "ipifc_status")),
			"/net/ether0/addr":    []byte(plan9Fixture(t, "ether0_addr")),
			"/net/iproute":        []byte(plan9Fixture(t, "iproute")),
		},
		globs: map[string][]string{
			"/net/ipifc/*/status": {"/net/ipifc/0/status"},
		},
	}

	facts := Collection(plan9NetworkingCoreFacts(s))
	networking, ok := facts["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking = %#v, want map", facts["networking"])
	}
	if got := networking["ip"]; got != "192.168.122.163" {
		t.Fatalf("networking.ip = %#v, want 192.168.122.163", got)
	}
}

func TestNetworkingCoreFactsUsesSessionPlatformForPlan9(t *testing.T) {
	t.Parallel()

	s := NewSession()
	s.host = &fakeHostOS{
		platform: "plan9",
		files: map[string][]byte{
			"/dev/sysname":        []byte(plan9Fixture(t, "sysname")),
			"/net/ipifc/0/status": []byte(plan9Fixture(t, "ipifc_status")),
			"/net/ether0/addr":    []byte(plan9Fixture(t, "ether0_addr")),
			"/net/iproute":        []byte(plan9Fixture(t, "iproute")),
		},
		globs: map[string][]string{
			"/net/ipifc/*/status": {"/net/ipifc/0/status"},
		},
	}

	facts := Collection(networkingCoreFacts(s))
	networking, ok := facts["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking = %#v, want map", facts["networking"])
	}
	if got := networking["ip"]; got != "192.168.122.163" {
		t.Fatalf("networking.ip = %#v, want Plan 9 fixture IP", got)
	}
}

func TestPlan9MemoryCoreFactsEmitOnlyTotal(t *testing.T) {
	t.Parallel()

	got := plan9MemoryCoreFacts(1_067_843_584)
	want := []ResolvedFact{
		{Name: "memory.system.total", Value: "1018.38 MiB"},
		{Name: "memory.system.total_bytes", Value: 1_067_843_584},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan9MemoryCoreFacts() = %#v, want %#v", got, want)
	}
	if got := plan9MemoryCoreFacts(0); got != nil {
		t.Fatalf("plan9MemoryCoreFacts(0) = %#v, want nil", got)
	}
}

func TestPlan9ProcessorsCoreFactsEmitOnlyFirstSlice(t *testing.T) {
	t.Parallel()

	info := processorInfo{LogicalCount: 1, Models: []string{"Core 2/Xeon 3600"}}
	got := plan9ProcessorsCoreFacts(info, "amd64")
	want := []ResolvedFact{
		{Name: "processors.count", Value: 1},
		{Name: "processors.isa", Value: "amd64"},
		{Name: "processors.models", Value: []string{"Core 2/Xeon 3600"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan9ProcessorsCoreFacts() = %#v, want %#v", got, want)
	}
}

func TestPlan9UptimeCoreFactsEmitsRubyCompatibleShape(t *testing.T) {
	t.Parallel()

	got := plan9UptimeCoreFacts(uptimeInfo{Duration: 49*time.Hour + 3*time.Minute + 2*time.Second, Known: true})
	want := []ResolvedFact{
		{Name: "system_uptime.days", Value: int64(2)},
		{Name: "system_uptime.hours", Value: int64(49)},
		{Name: "system_uptime.seconds", Value: int64(176582)},
		{Name: "system_uptime.uptime", Value: "2 days"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan9UptimeCoreFacts() = %#v, want %#v", got, want)
	}
	got = plan9UptimeCoreFacts(uptimeInfo{Duration: 100_000*time.Hour + 5*time.Second, Known: true})
	want = []ResolvedFact{
		{Name: "system_uptime.days", Value: int64(4166)},
		{Name: "system_uptime.hours", Value: int64(100000)},
		{Name: "system_uptime.seconds", Value: int64(360000005)},
		{Name: "system_uptime.uptime", Value: "4166 days"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan9UptimeCoreFacts(large duration) = %#v, want %#v", got, want)
	}
	if got := plan9UptimeCoreFacts(uptimeInfo{}); got != nil {
		t.Fatalf("plan9UptimeCoreFacts(unknown) = %#v, want nil", got)
	}
}

func TestCurrentPlan9UptimeParsesCommandOutput(t *testing.T) {
	t.Parallel()

	got := currentPlan9Uptime(func(name string, args ...string) string {
		if name != "uptime" || len(args) != 0 {
			t.Fatalf("run(%q, %#v), want uptime", name, args)
		}
		return "cirno up 1 day, 01:02:03\n"
	})
	want := uptimeInfo{Duration: 90_123 * time.Second, Known: true}
	if got != want {
		t.Fatalf("currentPlan9Uptime() = %#v, want %#v", got, want)
	}
	got = currentPlan9Uptime(func(string, ...string) string { return "not uptime output\n" })
	if got != (uptimeInfo{}) {
		t.Fatalf("currentPlan9Uptime(invalid) = %#v, want unknown", got)
	}
}

func TestPlan9NetworkingCoreFactsEmitFirstSliceOnly(t *testing.T) {
	t.Parallel()

	files := map[string][]byte{
		"/dev/sysname":        []byte(plan9Fixture(t, "sysname")),
		"/net/ipifc/0/status": []byte(plan9Fixture(t, "ipifc_status")),
		"/net/ether0/addr":    []byte(plan9Fixture(t, "ether0_addr")),
		"/net/iproute":        []byte(plan9Fixture(t, "iproute")),
	}
	s := NewSession()
	s.host = &fakeHostOS{
		files: files,
		globs: map[string][]string{
			"/net/ipifc/*/status": {"/net/ipifc/0/status"},
		},
	}

	facts := plan9NetworkingCoreFacts(s)
	got := Collection(facts)
	networking, ok := got["networking"].(map[string]any)
	if !ok {
		t.Fatalf("networking facts = %#v, want map", got["networking"])
	}

	for key, want := range map[string]any{
		"hostname": "cirno",
		"primary":  "ether0",
		"ip":       "192.168.122.163",
		"netmask":  "255.255.255.0",
		"network":  "192.168.122.0",
		"mac":      "52:54:00:76:cc:6d",
	} {
		if networking[key] != want {
			t.Fatalf("networking.%s = %#v, want %#v in %#v", key, networking[key], want, networking)
		}
	}
	for _, key := range []string{"dhcp", "mtu", "fqdn", "domain", "ip6"} {
		if value, ok := networking[key]; ok {
			t.Fatalf("networking.%s = %#v, want omitted", key, value)
		}
	}
}
