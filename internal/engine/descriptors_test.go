package engine

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCoreFactDescriptors_exactRows(t *testing.T) {
	rows := make([]string, 0, len(coreFactDescriptors))
	for _, descriptor := range coreFactDescriptors {
		rows = append(rows, fmt.Sprintf("%s|%s|%d|%s|%s",
			descriptor.root,
			descriptor.group,
			descriptor.groupOrder,
			descriptor.policy,
			strings.Join(descriptor.emittedRoots, ","),
		))
	}
	got := strings.Join(rows, "\n")
	want := strings.TrimSpace(`
facterversion||0|alwaysEager|facterversion
is_virtual||0|alwaysEager|is_virtual
path|path|5|alwaysEager|path
virtual||0|alwaysEager|virtual
networking|networking|2|gateable|networking
processors|processor|6|gateable|processors
memory|memory|1|gateable|memory
os|operating system|3|gateable|filesystems,kernel,os,system_profiler
dmi||0|gateable|dmi
disks||0|gateable|disks,mountpoints,partitions,zfs,zpool
ssh||0|gateable|ssh
identity||0|gateable|identity
system_uptime||0|gateable|load_averages,system_uptime
selinux||0|alwaysEager|os
fips_enabled||0|gateable|fips_enabled
timezone||0|gateable|timezone
augeas||0|gateable|augeas
xen||0|gateable|xen
packages|packages|4|gateable|packages
hypervisors||0|gateable|hypervisors
hypervisors||0|gateable|hypervisors
az_metadata||0|gateable|az_metadata,cloud
ec2_metadata||0|gateable|cloud,ec2_metadata,ec2_userdata
gce||0|gateable|cloud,gce`)
	if got != want {
		t.Fatalf("descriptor rows:\n%s\n\nwant:\n%s", got, want)
	}
}

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
		if descriptor.root != "selinux" && !slices.Contains(descriptor.emittedRoots, descriptor.root) {
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

func TestCoreFactDescriptors_eachAssemblyStaysWithinDeclaredRoots(t *testing.T) {
	host := &fakeHostOS{
		platform:        "linux",
		emptyRunDefault: true,
		files: map[string][]byte{
			"/etc/os-release": []byte("ID=test\n"),
		},
	}
	build := newCoreFactBuild(gatingProbeSession(host))
	for _, descriptor := range coreFactDescriptors {
		for _, fact := range descriptor.assemble(build) {
			root, _, _ := strings.Cut(fact.Name, ".")
			if !slices.Contains(descriptor.emittedRoots, root) {
				t.Fatalf("descriptor %q emitted undeclared root %q from fact %q; declared roots = %#v",
					descriptor.root, root, fact.Name, descriptor.emittedRoots)
			}
		}
	}
}

func assertDescriptorDeclaresFacts(t *testing.T, descriptorRoot string, facts []ResolvedFact) {
	t.Helper()
	for _, descriptor := range coreFactDescriptors {
		if descriptor.root != descriptorRoot {
			continue
		}
		for _, fact := range facts {
			root, _, _ := strings.Cut(fact.Name, ".")
			if !slices.Contains(descriptor.emittedRoots, root) {
				t.Fatalf("descriptor %q emitted undeclared root %q from positive fact %q; declared roots = %#v",
					descriptor.root, root, fact.Name, descriptor.emittedRoots)
			}
		}
		return
	}
	t.Fatalf("descriptor %q not found", descriptorRoot)
}
