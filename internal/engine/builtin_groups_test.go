package engine

import (
	"slices"
	"testing"
)

func TestBuiltinFactGroups_keepStructuredRootsDropLegacyFlatNames(t *testing.T) {
	groups := map[string][]string{}
	for _, g := range BuiltinFactGroups() {
		groups[g.Name] = g.Facts
	}

	// The structured root of each group must remain so `--disable <group>`
	// still drops the subtree.
	wantRoot := map[string]string{
		"memory":           "memory",
		"networking":       "networking",
		"operating system": "os",
		"processor":        "processors",
		"path":             "path",
		// Facts-native group (ADR-0014): a deliberate parity divergence from
		// Ruby Facter's group list, so `--list-block-groups` names packages as
		// one disable unit.
		"packages": "packages",
	}
	for name, root := range wantRoot {
		facts, ok := groups[name]
		if !ok {
			t.Fatalf("BuiltinFactGroups() missing group %q", name)
		}
		if !slices.Contains(facts, root) {
			t.Fatalf("group %q = %#v, want structured root %q kept", name, facts, root)
		}
	}

	// The legacy flat names removed by ADR-0007 must no longer appear in any
	// group; they are no-ops today and only clutter the catalog.
	legacy := []string{
		"memoryfree", "memoryfree_mb", "memorysize", "memorysize_mb",
		"hostname", "ipaddress", "ipaddress6", "netmask", "domain", "fqdn",
		"operatingsystem", "osfamily", "operatingsystemrelease", "architecture",
		"processorcount", "physicalprocessorcount", "hardwareisa",
	}
	for _, g := range BuiltinFactGroups() {
		for _, fact := range g.Facts {
			if slices.Contains(legacy, fact) {
				t.Fatalf("group %q still lists legacy flat name %q", g.Name, fact)
			}
		}
	}
}
