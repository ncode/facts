package engine

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCoreFactDescriptors_projectBuiltinGroups(t *testing.T) {
	want := []FactGroup{
		{Name: "memory", Facts: []string{"memory"}},
		{Name: "networking", Facts: []string{"networking"}},
		{Name: "operating system", Facts: []string{"os"}},
		{Name: "packages", Facts: []string{"packages"}},
		{Name: "path", Facts: []string{"path"}},
		{Name: "processor", Facts: []string{"processors"}},
	}
	if got := BuiltinFactGroups(); !reflect.DeepEqual(got, want) {
		t.Fatalf("BuiltinFactGroups() = %#v, want %#v", got, want)
	}
}

func TestBuildCoreFacts_inlineFactsKeepHeadOrder(t *testing.T) {
	facts := buildCoreFacts(NewSession(), map[string]bool{"packages": true})
	if len(facts) < 4 {
		t.Fatalf("buildCoreFacts() returned %d facts, want at least 4", len(facts))
	}
	got := []string{facts[0].Name, facts[1].Name, facts[2].Name, facts[3].Name}
	want := []string{"facterversion", "is_virtual", "path", "virtual"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("leading facts = %#v, want %#v", got, want)
	}
}

func TestCoreFactDescriptors_coverEmittedRoots(t *testing.T) {
	allowed := map[string]bool{}
	for _, descriptor := range coreFactDescriptors {
		if descriptor.root == "" {
			t.Fatalf("descriptor %#v has empty root", descriptor)
		}
		if len(descriptor.emittedRoots) == 0 {
			t.Fatalf("descriptor %q has no emitted roots", descriptor.root)
		}
		if descriptor.emitsUnder == "" && !slices.Contains(descriptor.emittedRoots, descriptor.root) {
			t.Fatalf("descriptor %q emitted roots = %#v, want root included", descriptor.root, descriptor.emittedRoots)
		}
		for _, root := range descriptor.emittedRoots {
			allowed[root] = true
		}
	}

	for _, fact := range buildCoreFacts(NewSession(), map[string]bool{"packages": true}) {
		root, _, _ := strings.Cut(fact.Name, ".")
		if !allowed[root] {
			t.Fatalf("buildCoreFacts emitted root %q from fact %q, missing from descriptor table", root, fact.Name)
		}
	}
}

func TestCoreFactDescriptors_standaloneGatesAreTableDriven(t *testing.T) {
	roots := standaloneCoreFactRoots()
	if len(roots) == 0 {
		t.Fatal("standaloneCoreFactRoots() = empty, want gated categories from descriptors")
	}
	for _, root := range roots {
		descriptor, ok := coreFactDescriptorByRoot(root, coreFactStandalone)
		if !ok {
			t.Fatalf("standalone root %q missing descriptor", root)
		}
		if !slices.Contains(descriptor.emittedRoots, root) {
			t.Fatalf("standalone descriptor %q emitted roots = %#v, want root included", root, descriptor.emittedRoots)
		}
	}
}

func coreFactDescriptorByRoot(root string, class coreFactGatingClass) (coreFactDescriptor, bool) {
	for _, descriptor := range coreFactDescriptors {
		if descriptor.root == root && descriptor.class == class {
			return descriptor, true
		}
	}
	return coreFactDescriptor{}, false
}
