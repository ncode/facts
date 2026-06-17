package facts

// Engine-row coverage backfill for the facter_test.go retirement (openspec
// change introduce-facts-library-api, task 5.3). Each test pins an
// engine-classified behavior from
// openspec/changes/introduce-facts-library-api/test-migration-map.md that no
// surviving test covered. Shared helpers (writeTestFile, hermeticSnapshot)
// live in engine_test.go.

import (
	"context"
	"errors"
	"testing"
)

// Pins TestValueAndFactDowncaseUserQueryLikeRubyAPI and
// TestFact_canonicalizesMixedCaseQueries: queries are matched
// case-insensitively against the canonical lowercase fact names.
func TestSnapshotValue_queriesAreCaseInsensitive(t *testing.T) {
	snap := hermeticSnapshot()

	for _, tc := range []struct{ canonical, mixed string }{
		{"os.name", "OS.NAME"},
		{"os.name", "Os.Name"},
		{"kernel", "KERNEL"},
	} {
		want, err := snap.Value(tc.canonical)
		if err != nil || want == nil {
			t.Fatalf("Value(%s) = %#v, %v, want resolved core fact", tc.canonical, want, err)
		}
		got, err := snap.Value(tc.mixed)
		if err != nil || got != want {
			t.Fatalf("Value(%s) = %#v, %v, want canonical %s value %#v", tc.mixed, got, err, tc.canonical, want)
		}
	}
}

// Pins TestValue_rejectsQueryWithNullByte: a query containing a null byte
// matches nothing and reports ErrFactNotFound (the Ruby API panicked; the new
// API rejects via the not-found error).
func TestSnapshotValue_nullByteQueryIsNotFound(t *testing.T) {
	snap := hermeticSnapshot()

	for _, query := range []string{"kernel\x00", "\x00", "os\x00.name"} {
		if _, err := snap.Value(query); !errors.Is(err, ErrFactNotFound) {
			t.Fatalf("Value(%q) err = %v, want ErrFactNotFound", query, err)
		}
	}
}

// Pins TestToHash_includesStandardCoreRootFacts: the canonical tree carries
// the standard core root facts. Legacy root aliases (hostname, fqdn,
// ipaddress, …) are removed entirely; only the structured networking.* facts
// carry those values (openspec change remove-legacy-facts).
func TestSnapshotTree_includesStandardCoreRootFacts(t *testing.T) {
	tree := hermeticSnapshot().Tree()

	names := []string{
		"dmi", "identity", "is_virtual",
		"kernel", "kernelmajversion", "kernelrelease", "kernelversion",
		"system_uptime", "timezone", "virtual",
	}
	for _, name := range names {
		if _, ok := tree[name]; !ok {
			t.Errorf("Tree() missing standard core root fact %q", name)
		}
	}

	networking, ok := tree["networking"].(map[string]any)
	if !ok {
		t.Fatalf("Tree()[networking] = %#v, want structured networking fact", tree["networking"])
	}
	if networking["hostname"] == nil {
		t.Fatalf("networking = %#v, want networking.hostname", networking)
	}
	for _, name := range []string{"hostname", "fqdn", "domain", "ipaddress", "ipaddress6", "macaddress", "interfaces", "netmask", "network"} {
		if got, ok := tree[name]; ok {
			t.Errorf("Tree()[%s] = %#v, want no legacy root alias", name, got)
		}
	}
}

// Legacy alias facts (Ruby Facter's --show-legacy layer) are not part of the
// Snapshot surface: no flat alias name or per-device alias family resolves in
// a default discovery (openspec change remove-legacy-facts).
func TestSnapshotTree_excludesLegacyAliasFacts(t *testing.T) {
	tree := hermeticSnapshot().Tree()

	for _, name := range []string{
		"architecture", "augeasversion", "dhcp_servers", "gid", "hardwareisa",
		"hardwaremodel", "id", "memoryfree", "memorysize", "memorysize_mb",
		"operatingsystem", "operatingsystemmajrelease",
		"operatingsystemrelease", "osfamily", "physicalprocessorcount",
		"processorcount", "productname", "rubyversion", "scope6",
		"serialnumber", "sshecdsakey", "sshed25519key", "sshfp_ecdsa",
		"sshfp_ed25519", "sshfp_rsa", "sshrsakey", "swapfree", "swapsize",
		"system32", "uptime", "uptime_days", "uptime_hours", "uptime_seconds",
		"uuid", "xendomains",
	} {
		if got, ok := tree[name]; ok {
			t.Errorf("Tree()[%s] = %#v, want no legacy alias fact", name, got)
		}
	}
	for name := range tree {
		for _, prefix := range []string{
			"blockdevice", "ipaddress_", "ipaddress6_", "lsb", "macaddress_",
			"macosx_", "mtu_", "netmask_", "netmask6_", "network_",
			"network6_", "scope6_", "selinux", "sp_", "windows_", "zfs_",
			"zpool_",
		} {
			if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
				t.Errorf("Tree()[%s] = %#v, want no legacy alias fact (prefix %s)", name, tree[name], prefix)
			}
		}
	}

	if _, err := hermeticSnapshot().Value("operatingsystem"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(operatingsystem) err = %v, want ErrFactNotFound", err)
	}
}

// Pins the surviving halves of TestAdd_resolvesProgrammaticCustomFactLazily,
// TestValue_reusesResolvedProgrammaticCustomFact, and
// TestValue_missingFactDoesNotResolveUnrelatedProgrammaticCustomFacts: each
// WithFact resolver runs exactly once per Discover and the resolved value is
// fixed in the Snapshot. DIVERGENCE from the global API: resolution is eager
// — every registered resolver runs during Discover, even when no query names
// it (the old lazy/on-demand behavior died with the Ruby API).
func TestWithFact_resolverRunsExactlyOncePerDiscover(t *testing.T) {
	calls := 0
	eng, err := New(WithFact("counted_fact", func(context.Context) (any, error) {
		calls++
		return calls, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	first, err := eng.Discover(context.Background(), "os.family")
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if calls != 1 {
		t.Fatalf("resolver ran %d times after one Discover with an unrelated query, want eager resolution exactly once", calls)
	}
	for range 2 {
		if got, err := first.Value("counted_fact"); err != nil || got != 1 {
			t.Fatalf("Value(counted_fact) = %#v, %v, want first-run value 1", got, err)
		}
	}
	if calls != 1 {
		t.Fatalf("resolver ran %d times after Snapshot lookups, want lookups served from the Snapshot", calls)
	}

	second, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if calls != 2 {
		t.Fatalf("resolver ran %d times after two Discovers, want once per Discover", calls)
	}
	if got, _ := second.Value("counted_fact"); got != 2 {
		t.Fatalf("second Value(counted_fact) = %#v, want fresh value 2", got)
	}
	if got, _ := first.Value("counted_fact"); got != 1 {
		t.Fatalf("first Value(counted_fact) = %#v after rediscovery, want immutable 1", got)
	}
}

// Pins the falsey half of TestValue_reusesResolvedProgrammaticCustomFact and
// TestValue_digsProgrammaticCustomMapWithStringifiedKeys: a false value is
// found (not missing), and dotted digging into a map with non-string keys
// stringifies the keys.
func TestWithFact_falseValuesAreFoundAndNonStringMapKeysDig(t *testing.T) {
	eng, err := New(
		WithFact("flag_fact", func(context.Context) (any, error) { return false, nil }),
		WithFact("mixed_map", func(context.Context) (any, error) {
			return map[any]any{"role": "web", 1: "one"}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}

	if got, err := snap.Value("flag_fact"); err != nil || got != false {
		t.Fatalf("Value(flag_fact) = %#v, %v, want found false value", got, err)
	}
	if got, err := snap.Value("mixed_map.role"); err != nil || got != "web" {
		t.Fatalf("Value(mixed_map.role) = %#v, %v, want web", got, err)
	}
	if got, err := snap.Value("mixed_map.1"); err != nil || got != "one" {
		t.Fatalf("Value(mixed_map.1) = %#v, %v, want stringified integer key dig", got, err)
	}
	if _, err := snap.Value("mixed_map.2"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(mixed_map.2) err = %v, want ErrFactNotFound", err)
	}
}

// Pins TestResolve_environmentExternalFactsOverrideExternalFactFiles:
// FACTER_* environment facts win precedence over file-based external facts of
// the same name.
func TestWithSystemDefaults_environmentFactsOverrideExternalFactFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "site.txt", "env_precedence_probe=from-file\n")
	t.Setenv("FACTER_env_precedence_probe", "from-env")

	eng, err := New(WithExternalDirs(dir), WithSystemDefaults())
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background(), "env_precedence_probe")
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if got, err := snap.Value("env_precedence_probe"); err != nil || got != "from-env" {
		t.Fatalf("Value(env_precedence_probe) = %#v, %v, want environment fact overriding external file", got, err)
	}
}

// Pins TestToHash_omitsProgrammaticCustomNilFacts/TestToHash_omitsCustomNilFacts
// and TestToHash_includesRegisteredCustomAndExternalFacts: the canonical tree
// includes registered and external (.txt) facts, while nil-valued facts are
// omitted from the tree and iteration yet stay queryable as resolved-nil.
func TestSnapshotTree_includesRegisteredAndExternalFactsAndOmitsNilFacts(t *testing.T) {
	externalDir := t.TempDir()
	writeTestFile(t, externalDir, "site.txt", "site_location=lab\n")

	eng, err := New(
		WithFact("site_role", func(context.Context) (any, error) { return "web", nil }),
		WithExternalDirs(externalDir),
		WithFact("nil_fact", func(context.Context) (any, error) { return nil, nil }),
	)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}

	tree := snap.Tree()
	if got := tree["site_role"]; got != "web" {
		t.Fatalf("Tree()[site_role] = %#v, want registered fact in tree", got)
	}
	if got := tree["site_location"]; got != "lab" {
		t.Fatalf("Tree()[site_location] = %#v, want registered external fact in tree", got)
	}
	if _, ok := tree["nil_fact"]; ok {
		t.Fatal("Tree() includes nil_fact, want nil-valued facts omitted from the tree")
	}
	for name := range snap.All() {
		if name == "nil_fact" {
			t.Fatal("All() yielded nil_fact, want nil-valued facts omitted from iteration")
		}
	}
	if value, err := snap.Value("nil_fact"); err != nil || value != nil {
		t.Fatalf("Value(nil_fact) = %#v, %v, want resolved-nil fact still queryable", value, err)
	}
}
