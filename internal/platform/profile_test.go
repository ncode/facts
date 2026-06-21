package platform

import (
	"reflect"
	"testing"
)

func TestProfilesExposeExpectedTargetIDs(t *testing.T) {
	got := profileIDs(Profiles())
	want := []string{"linux", "darwin", "windows", "freebsd", "openbsd", "netbsd", "dragonfly", "illumos", "plan9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Profiles() IDs = %#v, want %#v", got, want)
	}
}

func TestTargetSetsRemainDistinct(t *testing.T) {
	compileTargets := CompileTargets()
	distributionTargets := DistributionTargets()

	if !containsTarget(compileTargets, Target{GOOS: "plan9", GOARCH: "amd64"}) {
		t.Fatalf("CompileTargets() = %#v, want plan9/amd64", compileTargets)
	}
	if containsTarget(distributionTargets, Target{GOOS: "plan9", GOARCH: "amd64"}) {
		t.Fatalf("DistributionTargets() = %#v, want plan9/amd64 excluded until artifact promotion", distributionTargets)
	}
	if !containsTarget(distributionTargets, Target{GOOS: "illumos", GOARCH: "amd64"}) {
		t.Fatalf("DistributionTargets() = %#v, want illumos/amd64", distributionTargets)
	}

	schemaIDs := profileIDs(SchemaVisibleProfiles())
	if !reflect.DeepEqual(schemaIDs, []string{"linux", "darwin", "windows", "freebsd", "openbsd", "netbsd", "dragonfly", "illumos", "plan9"}) {
		t.Fatalf("SchemaVisibleProfiles() IDs = %#v, want all schema-visible targets", schemaIDs)
	}
	if reflect.DeepEqual(compileTargets, distributionTargets) {
		t.Fatalf("compile and distribution target sets unexpectedly match: %#v", compileTargets)
	}
}

func TestUnsupportedNamesRemainExcluded(t *testing.T) {
	for _, id := range []string{"solaris", "aix"} {
		if _, ok := Lookup(id); ok {
			t.Fatalf("Lookup(%q) found unsupported target", id)
		}
		if containsGOOS(CompileTargets(), id) {
			t.Fatalf("CompileTargets() includes unsupported GOOS %q", id)
		}
		if containsGOOS(DistributionTargets(), id) {
			t.Fatalf("DistributionTargets() includes unsupported GOOS %q", id)
		}
		for _, profile := range SchemaVisibleProfiles() {
			if profile.ID == id {
				t.Fatalf("SchemaVisibleProfiles() includes unsupported GOOS %q", id)
			}
		}
	}
}

func TestCapabilityPolicyCapturesLowRiskGates(t *testing.T) {
	tests := []struct {
		goos             string
		filesystems      bool
		zfs              bool
		operatingRelease bool
	}{
		{goos: "linux", filesystems: true, operatingRelease: true},
		{goos: "darwin", filesystems: true, operatingRelease: true},
		{goos: "freebsd", zfs: true, operatingRelease: true},
		{goos: "netbsd", zfs: true, operatingRelease: true},
		{goos: "illumos", zfs: true, operatingRelease: true},
		{goos: "plan9", operatingRelease: false},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			profile, ok := Lookup(tt.goos)
			if !ok {
				t.Fatalf("Lookup(%q) did not find profile", tt.goos)
			}
			if profile.Capabilities.Filesystems != tt.filesystems {
				t.Fatalf("%s filesystems capability = %v, want %v", tt.goos, profile.Capabilities.Filesystems, tt.filesystems)
			}
			if profile.Capabilities.ZFS != tt.zfs {
				t.Fatalf("%s ZFS capability = %v, want %v", tt.goos, profile.Capabilities.ZFS, tt.zfs)
			}
			if profile.Capabilities.OSRelease != tt.operatingRelease {
				t.Fatalf("%s OSRelease capability = %v, want %v", tt.goos, profile.Capabilities.OSRelease, tt.operatingRelease)
			}
		})
	}
}

func TestNativeGateMetadataUsesTargetIDs(t *testing.T) {
	for _, gate := range NativeGates() {
		if _, ok := Lookup(gate.GOOS); !ok {
			t.Fatalf("NativeGates() includes unknown GOOS %q", gate.GOOS)
		}
	}
}

func profileIDs(profiles []Profile) []string {
	ids := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
	}
	return ids
}

func containsTarget(targets []Target, want Target) bool {
	for _, target := range targets {
		if target == want {
			return true
		}
	}
	return false
}

func containsGOOS(targets []Target, goos string) bool {
	for _, target := range targets {
		if target.GOOS == goos {
			return true
		}
	}
	return false
}
