package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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

func TestPlan9NetworkingCoreFactsEmitFirstSliceOnly(t *testing.T) {
	t.Parallel()

	files := map[string][]byte{
		"/dev/sysname":        []byte(plan9Fixture(t, "sysname")),
		"/net/ipifc/0/status": []byte(plan9Fixture(t, "ipifc_status")),
		"/net/ether0/addr":    []byte(plan9Fixture(t, "ether0_addr")),
		"/net/iproute":        []byte(plan9Fixture(t, "iproute")),
	}
	s := NewSession()
	s.host = &fakeHostOS{files: files}

	facts := plan9NetworkingCoreFactsWithGlob(s, func(pattern string) ([]string, error) {
		if pattern != "/net/ipifc/*/status" {
			t.Fatalf("glob pattern = %q, want /net/ipifc/*/status", pattern)
		}
		return []string{"/net/ipifc/0/status"}, nil
	})
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
