package engine

import (
	"context"
	"runtime"
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

// gatedSingleOutputCategories pairs each resolution-gated core category with the
// top-level fact name that gates it (ADR-0015).
var gatedSingleOutputCategories = []string{
	"networking", "processors", "memory", "ssh",
	"timezone", "fips_enabled", "augeas", "xen", "packages",
}

func TestBuildCoreFacts_resolutionGatesSingleOutputCategories(t *testing.T) {
	// buildCoreFacts performs no filtering, so a fact root that is absent here
	// can only be absent because its resolver was skipped — exactly the
	// resolution-gating contract.
	baseline := buildCoreFacts(NewSession(), nil)
	for _, fact := range gatedSingleOutputCategories {
		if !hasRoot(baseline, fact) {
			// Not every category resolves on this host (e.g. augeas/xen). Only
			// assert gating for the ones that do produce output by default.
			continue
		}
		gated := buildCoreFacts(NewSession(), map[string]bool{fact: true})
		if hasRoot(gated, fact) {
			t.Fatalf("buildCoreFacts(disabled=%q) still emitted %q root; resolver was not gated", fact, fact)
		}
	}
}

func TestBuildCoreFacts_multiOutputCategoryStaysEager(t *testing.T) {
	// Disabling the multi-output root `os` must NOT gate osCoreFacts: it stays
	// eager (resolve-then-prune) so its sibling kernel/filesystems outputs are
	// not collateral. buildCoreFacts still emits os.* because it never prunes.
	facts := buildCoreFacts(NewSession(), map[string]bool{"os": true})
	if !hasRoot(facts, "os") {
		t.Fatal("buildCoreFacts(disabled=os) dropped os.*; multi-output category must stay eager")
	}
}

func TestBuildCoreFacts_multiOutputSubfactStaysEagerThenPruned(t *testing.T) {
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
// (networking/processors/memory/xen) are observable on darwin, linux, and
// Windows CI alike.
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

// TestBuildCoreFacts_resolutionGatingSkipsProbeWork proves the gate skips real
// work, not just output: each gated category's distinctive host probe must run
// when the category is enabled and must NOT run when it is disabled. A display
// filter (resolve-then-hide) would satisfy the root-absence tests above while
// still invoking the probe — these probe-call assertions are what distinguish
// resolution-gating from output filtering (ADR-0015).
func TestBuildCoreFacts_resolutionGatingSkipsProbeWork(t *testing.T) {
	markers := []struct {
		category   string
		probed     func(h *fakeHostOS) bool
		skip       func() bool
		skipReason string
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
			// ssh reads ssh_host_*_key.pub via readFile on every non-Windows host;
			// the path set keys off runtime.GOOS, which the fake cannot override,
			// so the Windows path (programdata + a different seam) is skipped.
			category:   "ssh",
			probed:     func(h *fakeHostOS) bool { return hostReadFileMatching(h, "ssh_host_rsa_key.pub") },
			skip:       func() bool { return runtime.GOOS == "windows" },
			skipReason: "ssh host-key paths key off runtime.GOOS; the Windows path uses a different seam",
		},
		{
			// fips_enabled reads /proc/sys/crypto/fips_enabled only when
			// runtime.GOOS is linux (Windows uses a reg query); elsewhere the
			// resolver returns nil before any host call, so there is nothing to
			// observe on this host.
			category:   "fips_enabled",
			probed:     func(h *fakeHostOS) bool { return hostReadFileMatching(h, "/proc/sys/crypto/fips_enabled") },
			skip:       func() bool { return runtime.GOOS != "linux" },
			skipReason: "fips_enabled probes only on linux/windows; runtime.GOOS gates the probe away here",
		},
		// timezone is intentionally omitted: on every non-Windows host it derives
		// the zone from Go's time.Now().Format("MST") with no host probe at all,
		// so no recorded seam exists to observe. Its gating is covered by the
		// root-absence test above; a probe-work proof is unreachable.
	}

	for _, m := range markers {
		t.Run(m.category, func(t *testing.T) {
			if m.skip != nil && m.skip() {
				t.Skipf("%s probe unobservable on %s: %s", m.category, runtime.GOOS, m.skipReason)
			}

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
