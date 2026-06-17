package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCoreFacts_includeIntegrationRootFactGroups(t *testing.T) {
	collection := Collection(CoreFacts(testSession))

	for _, name := range []string{"memory", "networking", "os", "path", "processors"} {
		if _, ok := collection[name]; !ok {
			t.Fatalf("CoreFacts(testSession) missing root fact group %q in %#v", name, collection)
		}
	}
}

func TestCoreFacts_includePathFromEnvironment(t *testing.T) {
	path := "/usr/bin:/etc:/usr/sbin:/usr/ucb:/usr/bin/X11:/sbin:/usr/java6/jre/bin:/usr/java6/bin"
	t.Setenv("PATH", path)
	collection := Collection(CoreFacts(NewSession()))

	if got := collection["path"]; got != path {
		t.Fatalf("path = %#v, want %#v", got, path)
	}
}

func TestCoreFacts_includeFacterVersion(t *testing.T) {
	collection := Collection(CoreFacts(testSession))

	if got := collection["facterversion"]; got != Version {
		t.Fatalf("facterversion = %#v, want %#v", got, Version)
	}
}

func TestCoreFacts_omitsRubyAndPuppetPackageVersionFacts(t *testing.T) {
	host := &fakeHostOS{
		runOutput: "3.3.0\narm64-darwin23\n/ignored\n/opt/puppetlabs/puppet/lib/ruby/site_ruby/3.3.0\n",
		files: map[string][]byte{
			"/opt/puppetlabs/puppet/VERSION": []byte("8.10.0\n"),
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	collection := Collection(CoreFacts(s))
	for _, name := range []string{"ruby", "aio_agent_version"} {
		if got, ok := collection[name]; ok {
			t.Fatalf("CoreFacts() %s = %#v, want absent", name, got)
		}
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
	"processor9", "scope6_", "selinux", "sp_", "windows_",
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
