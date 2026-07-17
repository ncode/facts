package engine

import "slices"

type coreFactSchedulingPolicy string

const (
	coreFactGateable    coreFactSchedulingPolicy = "gateable"
	coreFactAlwaysEager coreFactSchedulingPolicy = "alwaysEager"
)

type coreFactDescriptor struct {
	root         string
	group        string
	groupOrder   int
	policy       coreFactSchedulingPolicy
	assemble     func(*coreFactBuild) []ResolvedFact
	emittedRoots []string
}

type coreFactBuild struct {
	s              *Session
	goos           string
	virtualization virtualization
	virtualFact    any
	isVirtualFact  any
}

var coreFactDescriptors = []coreFactDescriptor{
	{
		root:         "facterversion",
		policy:       coreFactAlwaysEager,
		assemble:     func(*coreFactBuild) []ResolvedFact { return []ResolvedFact{{Name: "facterversion", Value: Version}} },
		emittedRoots: []string{"facterversion"},
	},
	{
		root:   "is_virtual",
		policy: coreFactAlwaysEager,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return []ResolvedFact{{Name: "is_virtual", Value: b.isVirtualFact}}
		},
		emittedRoots: []string{"is_virtual"},
	},
	{
		root:       "path",
		group:      "path",
		groupOrder: 5,
		policy:     coreFactAlwaysEager,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return []ResolvedFact{{Name: "path", Value: currentPathEntries(b.goos, b.s.getenv)}}
		},
		emittedRoots: []string{"path"},
	},
	{
		root:   "virtual",
		policy: coreFactAlwaysEager,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return []ResolvedFact{{Name: "virtual", Value: b.virtualFact}}
		},
		emittedRoots: []string{"virtual"},
	},
	{
		root:         "networking",
		group:        "networking",
		groupOrder:   2,
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return networkingCoreFacts(b.s) },
		emittedRoots: []string{"networking"},
	},
	{
		root:         "processors",
		group:        "processor",
		groupOrder:   6,
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return processorsCoreFacts(b.s) },
		emittedRoots: []string{"processors"},
	},
	{
		root:         "memory",
		group:        "memory",
		groupOrder:   1,
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return memoryCoreFacts(b.s) },
		emittedRoots: []string{"memory"},
	},
	{
		root:         "os",
		group:        "operating system",
		groupOrder:   3,
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return osCoreFacts(b.s) },
		emittedRoots: []string{"filesystems", "kernel", "os", "system_profiler"},
	},
	{
		root:         "dmi",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return dmiCoreFacts(b.s) },
		emittedRoots: []string{"dmi"},
	},
	{
		root:         "disks",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return disksCoreFacts(b.s) },
		emittedRoots: []string{"disks", "mountpoints", "partitions", "zfs", "zpool"},
	},
	{
		root:         "ssh",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return sshCoreFacts(b.s) },
		emittedRoots: []string{"ssh"},
	},
	{
		root:         "identity",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return identityCoreFacts(b.s) },
		emittedRoots: []string{"identity"},
	},
	{
		root:         "system_uptime",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return uptimeCoreFacts(b.s) },
		emittedRoots: []string{"load_averages", "system_uptime"},
	},
	{
		root:         "selinux",
		policy:       coreFactAlwaysEager,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return selinuxCoreFacts(b.s) },
		emittedRoots: []string{"os"},
	},
	{
		root:         "fips_enabled",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return fipsCoreFacts(b.s) },
		emittedRoots: []string{"fips_enabled"},
	},
	{
		root:         "timezone",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return timezoneCoreFacts(b.s) },
		emittedRoots: []string{"timezone"},
	},
	{
		root:         "augeas",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return augeasCoreFacts(b.s) },
		emittedRoots: []string{"augeas"},
	},
	{
		root:         "xen",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return xenCoreFacts(b.s) },
		emittedRoots: []string{"xen"},
	},
	{
		root:         "packages",
		group:        "packages",
		groupOrder:   4,
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return packagesCoreFacts(b.s) },
		emittedRoots: []string{"packages"},
	},
	{
		root:         "hypervisors",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return currentLinuxHypervisorFacts(b.s) },
		emittedRoots: []string{"hypervisors"},
	},
	{
		root:         "hypervisors",
		policy:       coreFactGateable,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return currentWindowsHypervisorFacts(b.s) },
		emittedRoots: []string{"hypervisors"},
	},
	{
		root:   "az_metadata",
		policy: coreFactGateable,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return azureFacts(b.s.Context(), newAzureClient(azureMetadataBaseURL, nil), b.virtualization)
		},
		emittedRoots: []string{"az_metadata", "cloud"},
	},
	{
		root:   "ec2_metadata",
		policy: coreFactGateable,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return ec2Facts(b.s, newEC2Client(ec2MetadataBaseURL, nil), b.virtualization)
		},
		emittedRoots: []string{"cloud", "ec2_metadata", "ec2_userdata"},
	},
	{
		root:   "gce",
		policy: coreFactGateable,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return platformGCEFacts(b.s.Context(), b.goos, b.virtualization, dmiBIOSVendor(b.s.cachedDMI()), newGCEClient(gceMetadataBaseURL, nil))
		},
		emittedRoots: []string{"cloud", "gce"},
	},
}

func (d coreFactDescriptor) shouldRun(disabled map[string]bool) bool {
	if d.policy == coreFactAlwaysEager {
		return true
	}
	for _, root := range d.emittedRoots {
		if !disabled[root] {
			return true
		}
	}
	return false
}

func newCoreFactBuild(s *Session) *coreFactBuild {
	goos := s.goos()
	virtualization := detectVirtualization(s)
	virtualFact, isVirtualFact := virtualizationFactValues(virtualization)
	return &coreFactBuild{
		s:              s,
		goos:           goos,
		virtualization: virtualization,
		virtualFact:    virtualFact,
		isVirtualFact:  isVirtualFact,
	}
}

func builtinFactGroupsFromDescriptors() []FactGroup {
	groups := make([]FactGroup, 0)
	for _, descriptor := range coreFactDescriptors {
		if descriptor.group == "" {
			continue
		}
		groups = append(groups, FactGroup{
			Name:  descriptor.group,
			Facts: []string{descriptor.root},
		})
	}
	slices.SortFunc(groups, func(a, b FactGroup) int {
		return descriptorGroupOrder(a.Name) - descriptorGroupOrder(b.Name)
	})
	return groups
}

func descriptorGroupOrder(name string) int {
	for _, descriptor := range coreFactDescriptors {
		if descriptor.group == name {
			return descriptor.groupOrder
		}
	}
	return 0
}
