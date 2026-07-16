package engine

import (
	"context"
	"strings"
	"testing"
)

// rootNames returns the set of distinct top-level (pre-dot) fact roots present
// in facts.
func rootNames(facts []ResolvedFact) map[string]bool {
	roots := map[string]bool{}
	for _, f := range facts {
		root, _, _ := strings.Cut(f.Name, ".")
		roots[root] = true
	}
	return roots
}

// hasRoot reports whether any fact in facts has the given top-level root.
func hasRoot(facts []ResolvedFact, root string) bool {
	return rootNames(facts)[root]
}

func TestBuildCoreFacts_multiOutputResolverRunsForKeptSibling(t *testing.T) {
	// Disabling only the multi-output root `os` must not gate osCoreFacts because
	// its kernel/filesystems siblings remain enabled. buildCoreFacts still emits
	// os.* because pruning happens later in discovery.
	facts := buildCoreFacts(NewSession(), map[string]bool{"os": true})
	if !hasRoot(facts, "os") {
		t.Fatal("buildCoreFacts(disabled=os) dropped os.*; resolver must run for a kept sibling")
	}
}

func TestBuildCoreFacts_multiOutputSubfactDisableRunsResolverThenPrunes(t *testing.T) {
	disabled := map[string]bool{"os.release": true}
	facts := buildCoreFacts(NewSession(), disabled)
	// The resolver still runs (os.name present) and still emits os.release;
	// only the later FilterDisabledFacts prunes the disabled sub-fact.
	var sawName, sawRelease bool
	for _, f := range facts {
		switch {
		case f.Name == "os.name":
			sawName = true
		case f.Name == "os.release" || strings.HasPrefix(f.Name, "os.release."):
			sawRelease = true
		}
	}
	if !sawName {
		t.Fatal("buildCoreFacts(disabled=os.release) dropped os.name; sub-fact disable must not gate the resolver")
	}
	if !sawRelease {
		t.Fatal("buildCoreFacts(disabled=os.release) did not emit os.release before pruning; expected resolve-then-prune")
	}
	filtered := FilterDisabledFacts(facts, disabled)
	for _, f := range filtered {
		if f.Name == "os.release" || strings.HasPrefix(f.Name, "os.release.") {
			t.Fatalf("FilterDisabledFacts kept disabled sub-fact %q", f.Name)
		}
	}
	if !hasRoot(filtered, "os") {
		t.Fatal("FilterDisabledFacts dropped the whole os root when only os.release was disabled")
	}
}

func TestBuildCoreFacts_keptCategoriesUnaffectedByGate(t *testing.T) {
	// Gating timezone must leave the other categories' roots untouched.
	baseline := buildCoreFacts(NewSession(), nil)
	gated := buildCoreFacts(NewSession(), map[string]bool{"timezone": true})
	for root := range rootNames(baseline) {
		if root == "timezone" {
			continue
		}
		if !hasRoot(gated, root) {
			t.Fatalf("gating timezone dropped unrelated root %q", root)
		}
	}
}

// newGatingProbeHost builds a fake Linux host so the gated resolvers that branch
// on s.goos() take a path reading a distinctive marker file; the readFile/run
// spies then record whether each resolver actually ran. A Linux fake is used
// regardless of the test host so the s.goos()-keyed categories
// (networking/processors/memory/xen/fips_enabled) are observable on darwin,
// linux, and Windows CI alike.
func newGatingProbeHost() *fakeHostOS {
	return &fakeHostOS{platform: "linux"}
}

func gatingProbeSession(host *fakeHostOS) *Session {
	s := NewSessionContext(context.Background())
	s.host = host
	return s
}

func hostRanCommand(h *fakeHostOS, substr string) bool {
	for _, call := range h.runCalls {
		if strings.Contains(call.name, substr) {
			return true
		}
	}
	return false
}

func hostReadFileMatching(h *fakeHostOS, substr string) bool {
	for _, path := range h.readFileCalls {
		if strings.Contains(path, substr) {
			return true
		}
	}
	return false
}

func hostReadFileCount(h *fakeHostOS, substr string) int {
	count := 0
	for _, path := range h.readFileCalls {
		if strings.Contains(path, substr) {
			count++
		}
	}
	return count
}

func hostReadDirCount(h *fakeHostOS, substr string) int {
	count := 0
	for _, path := range h.readDirCalls {
		if strings.Contains(path, substr) {
			count++
		}
	}
	return count
}

func hostRunCount(h *fakeHostOS, name string) int {
	count := 0
	for _, call := range h.runCalls {
		if call.name == name {
			count++
		}
	}
	return count
}

func TestResolveCoreFactDescriptors_probeCountsEveryCatalogRow(t *testing.T) {
	for i, descriptor := range coreFactDescriptors {
		t.Run(descriptor.root, func(t *testing.T) {
			calls := 0
			observed := descriptor
			observed.assemble = func(*coreFactBuild) []ResolvedFact {
				calls++
				return nil
			}

			allDisabled := make(map[string]bool, len(descriptor.emittedRoots))
			for _, root := range descriptor.emittedRoots {
				allDisabled[root] = true
			}
			resolveCoreFactDescriptors(&coreFactBuild{}, allDisabled, []coreFactDescriptor{observed})
			want := 0
			if descriptor.policy == coreFactAlwaysEager {
				want = 1
			}
			if calls != want {
				t.Fatalf("catalog row %d all-disabled resolver calls = %d, want %d", i, calls, want)
			}

			if descriptor.policy != coreFactGateable {
				return
			}
			calls = 0
			delete(allDisabled, descriptor.emittedRoots[0])
			resolveCoreFactDescriptors(&coreFactBuild{}, allDisabled, []coreFactDescriptor{observed})
			if calls != 1 {
				t.Fatalf("catalog row %d one-kept resolver calls = %d, want 1", i, calls)
			}
		})
	}
}

func TestBuildCoreFacts_DMIProbeRunsOnlyForKeptDMIOrGCE(t *testing.T) {
	tests := []struct {
		name     string
		disabled map[string]bool
		want     int
	}{
		{
			name:     "both consumers disabled",
			disabled: map[string]bool{"cloud": true, "dmi": true, "gce": true, "packages": true},
			want:     0,
		},
		{
			name:     "dmi kept",
			disabled: map[string]bool{"cloud": true, "gce": true, "packages": true},
			want:     1,
		},
		{
			name:     "gce kept",
			disabled: map[string]bool{"dmi": true, "packages": true},
			want:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := &fakeHostOS{
				platform:        "linux",
				emptyRunDefault: true,
				files: map[string][]byte{
					"/sys/class/dmi/id/board_vendor": []byte("Example"),
				},
			}
			buildCoreFacts(gatingProbeSession(host), tt.disabled)
			if got := hostReadFileCount(host, "/sys/class/dmi/id/board_vendor"); got != tt.want {
				t.Fatalf("DMI probe count = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildCoreFacts_multiOutputProbesRunOnlyForKeptRoots(t *testing.T) {
	tests := []struct {
		name   string
		roots  []string
		probes func(*fakeHostOS) int
	}{
		{
			name:  "operating system",
			roots: []string{"filesystems", "kernel", "os", "system_profiler"},
			probes: func(host *fakeHostOS) int {
				return hostReadFileCount(host, "/etc/os-release")
			},
		},
		{
			name:  "disks",
			roots: []string{"disks", "mountpoints", "partitions", "zfs", "zpool"},
			probes: func(host *fakeHostOS) int {
				return hostReadDirCount(host, "/sys/block")
			},
		},
		{
			name:  "uptime",
			roots: []string{"load_averages", "system_uptime"},
			probes: func(host *fakeHostOS) int {
				return hostReadFileCount(host, "/proc/uptime")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allDisabled := map[string]bool{"packages": true}
			for _, root := range tt.roots {
				allDisabled[root] = true
			}

			host := &fakeHostOS{platform: "linux", emptyRunDefault: true}
			buildCoreFacts(gatingProbeSession(host), allDisabled)
			if got := tt.probes(host); got != 0 {
				t.Fatalf("all emitted roots disabled: probe count = %d, want 0", got)
			}

			oneKept := make(map[string]bool, len(allDisabled)-1)
			for root := range allDisabled {
				oneKept[root] = true
			}
			delete(oneKept, tt.roots[0])
			host = &fakeHostOS{platform: "linux", emptyRunDefault: true}
			buildCoreFacts(gatingProbeSession(host), oneKept)
			if got := tt.probes(host); got == 0 {
				t.Fatal("one emitted root kept: probe count = 0, want resolver to run")
			}
		})
	}
}

func TestBuildCoreFacts_identityProbeRunsOnlyForKeptIdentityOrSSH(t *testing.T) {
	tests := []struct {
		name     string
		disabled map[string]bool
		want     int
	}{
		{name: "both consumers disabled", disabled: map[string]bool{"identity": true, "packages": true, "ssh": true}, want: 0},
		{name: "identity kept", disabled: map[string]bool{"packages": true, "ssh": true}, want: 1},
		{name: "ssh kept", disabled: map[string]bool{"identity": true, "packages": true}, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := &fakeHostOS{platform: "windows", emptyRunDefault: true}
			buildCoreFacts(gatingProbeSession(host), tt.disabled)
			if got := hostRunCount(host, "whoami"); got != tt.want {
				t.Fatalf("identity probe count = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildCoreFacts_cloudProvidersRunForSharedCloudRoot(t *testing.T) {
	tests := []struct {
		name         string
		providerRoot string
		disabled     map[string]bool
	}{
		{name: "azure", providerRoot: "az_metadata", disabled: map[string]bool{"az_metadata": true}},
		{name: "ec2", providerRoot: "ec2_metadata", disabled: map[string]bool{"ec2_metadata": true, "ec2_userdata": true}},
		{name: "gce", providerRoot: "gce", disabled: map[string]bool{"gce": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keptCloud := make(map[string]bool, len(tt.disabled)+1)
			for root := range tt.disabled {
				keptCloud[root] = true
			}
			keptCloud["packages"] = true
			facts := buildCoreFacts(gatingProbeSession(&fakeHostOS{platform: "linux", emptyRunDefault: true}), keptCloud)
			if !hasRoot(facts, tt.providerRoot) {
				t.Fatalf("cloud kept: resolver did not emit %q", tt.providerRoot)
			}

			allDisabled := make(map[string]bool, len(keptCloud)+1)
			for root := range keptCloud {
				allDisabled[root] = true
			}
			allDisabled["cloud"] = true
			facts = buildCoreFacts(gatingProbeSession(&fakeHostOS{platform: "linux", emptyRunDefault: true}), allDisabled)
			if hasRoot(facts, tt.providerRoot) {
				t.Fatalf("all emitted roots disabled: resolver still emitted %q", tt.providerRoot)
			}
		})
	}
}

// TestBuildCoreFacts_resolutionGatingSkipsProbeWork proves the gate skips real
// work, not just output: each gated category's distinctive host probe must run
// when the category is enabled and must NOT run when it is disabled. A display
// filter (resolve-then-hide) would satisfy the root-absence tests above while
// still invoking the probe — these probe-call assertions are what distinguish
// resolution-gating from output filtering (ADR-0015).
func TestBuildCoreFacts_resolutionGatingSkipsProbeWork(t *testing.T) {
	markers := []struct {
		category string
		probed   func(h *fakeHostOS) bool
	}{
		{
			category: "networking",
			probed:   func(h *fakeHostOS) bool { return hostReadFileMatching(h, "/etc/resolv.conf") },
		},
		{
			category: "processors",
			probed:   func(h *fakeHostOS) bool { return hostReadFileMatching(h, "/proc/cpuinfo") },
		},
		{
			category: "memory",
			probed:   func(h *fakeHostOS) bool { return hostReadFileMatching(h, "/proc/meminfo") },
		},
		{
			category: "xen",
			probed:   func(h *fakeHostOS) bool { return hostReadFileMatching(h, "/proc/xen/capabilities") },
		},
		{
			// augeas runs `augparse --version` on every platform (no goos branch),
			// so it is observable through the run spy regardless of the test host.
			category: "augeas",
			probed:   func(h *fakeHostOS) bool { return hostRanCommand(h, "augparse") },
		},
		{
			// ssh reads ssh_host_*_key.pub via readFile; the path set keys off
			// s.goos(), so the fake Linux host drives the unix path on any test host.
			category: "ssh",
			probed:   func(h *fakeHostOS) bool { return hostReadFileMatching(h, "ssh_host_rsa_key.pub") },
		},
		{
			category: "fips_enabled",
			probed:   func(h *fakeHostOS) bool { return hostReadFileMatching(h, "/proc/sys/crypto/fips_enabled") },
		},
		{
			category: "packages",
			probed:   func(h *fakeHostOS) bool { return hostReadFileMatching(h, "/var/lib/dpkg/status") },
		},
		// timezone is intentionally omitted: on every non-Windows host it derives
		// the zone from Go's time.Now().Format("MST") with no host probe at all,
		// so no recorded seam exists to observe. Its gating is covered by the
		// root-absence test above; a probe-work proof is unreachable.
	}

	for _, m := range markers {
		t.Run(m.category, func(t *testing.T) {
			enabled := newGatingProbeHost()
			buildCoreFacts(gatingProbeSession(enabled), nil)
			if !m.probed(enabled) {
				t.Fatalf("%s enabled: its probe did not run, so the test cannot prove gating (reads=%v runs=%v)",
					m.category, enabled.readFileCalls, enabled.runCalls)
			}

			disabled := newGatingProbeHost()
			buildCoreFacts(gatingProbeSession(disabled), map[string]bool{m.category: true})
			if m.probed(disabled) {
				t.Fatalf("%s disabled: its probe STILL ran — gating only filtered output instead of skipping work", m.category)
			}
		})
	}
}
