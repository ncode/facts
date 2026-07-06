package engine

import "slices"

type coreFactGatingClass string

const (
	coreFactStandalone  coreFactGatingClass = "standalone"
	coreFactMultiOutput coreFactGatingClass = "multiOutput"
	coreFactSharedProbe coreFactGatingClass = "sharedProbe"
	coreFactInlineEager coreFactGatingClass = "inlineEager"
)

type coreFactDescriptor struct {
	root           string
	group          string
	groupOrder     int
	class          coreFactGatingClass
	assemble       func(*coreFactBuild) []ResolvedFact
	emittedRoots   []string
	probeConsumers []string
	emitsUnder     string
}

type coreFactBuild struct {
	s              *Session
	goos           string
	virtualization virtualization
	virtualFact    any
	isVirtualFact  any
	dmi            map[string]any
}

var coreFactDescriptors = []coreFactDescriptor{
	{
		root:         "facterversion",
		class:        coreFactInlineEager,
		assemble:     func(*coreFactBuild) []ResolvedFact { return []ResolvedFact{{Name: "facterversion", Value: Version}} },
		emittedRoots: []string{"facterversion"},
	},
	{
		root:  "is_virtual",
		class: coreFactInlineEager,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return []ResolvedFact{{Name: "is_virtual", Value: b.isVirtualFact}}
		},
		emittedRoots: []string{"is_virtual"},
	},
	{
		root:       "path",
		group:      "path",
		groupOrder: 5,
		class:      coreFactInlineEager,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return []ResolvedFact{{Name: "path", Value: currentPathEntries(b.goos, b.s.getenv)}}
		},
		emittedRoots: []string{"path"},
	},
	{
		root:  "virtual",
		class: coreFactInlineEager,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return []ResolvedFact{{Name: "virtual", Value: b.virtualFact}}
		},
		emittedRoots:   []string{"virtual"},
		probeConsumers: []string{"azure", "ec2", "gce", "hypervisors"},
	},
	{
		root:         "networking",
		group:        "networking",
		groupOrder:   2,
		class:        coreFactStandalone,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return networkingCoreFacts(b.s) },
		emittedRoots: []string{"networking"},
	},
	{
		root:         "processors",
		group:        "processor",
		groupOrder:   6,
		class:        coreFactStandalone,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return processorsCoreFacts(b.s) },
		emittedRoots: []string{"processors"},
	},
	{
		root:         "memory",
		group:        "memory",
		groupOrder:   1,
		class:        coreFactStandalone,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return memoryCoreFacts(b.s) },
		emittedRoots: []string{"memory"},
	},
	{
		root:         "os",
		group:        "operating system",
		groupOrder:   3,
		class:        coreFactMultiOutput,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return osCoreFacts(b.s) },
		emittedRoots: []string{"filesystems", "kernel", "os", "system_profiler"},
	},
	{
		root:           "dmi",
		class:          coreFactSharedProbe,
		assemble:       func(b *coreFactBuild) []ResolvedFact { return dmiCoreFacts(b.s) },
		emittedRoots:   []string{"dmi"},
		probeConsumers: []string{"gce"},
	},
	{
		root:         "disks",
		class:        coreFactMultiOutput,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return disksCoreFacts(b.s) },
		emittedRoots: []string{"disks", "mountpoints", "partitions", "zpool"},
	},
	{
		root:           "ssh",
		class:          coreFactStandalone,
		assemble:       func(b *coreFactBuild) []ResolvedFact { return sshCoreFacts(b.s) },
		emittedRoots:   []string{"ssh"},
		probeConsumers: []string{"identity"},
	},
	{
		root:           "identity",
		class:          coreFactSharedProbe,
		assemble:       func(b *coreFactBuild) []ResolvedFact { return identityCoreFacts(b.s) },
		emittedRoots:   []string{"identity"},
		probeConsumers: []string{"ssh"},
	},
	{
		root:         "system_uptime",
		class:        coreFactMultiOutput,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return uptimeCoreFacts(b.s) },
		emittedRoots: []string{"load_averages", "system_uptime"},
	},
	{
		root:         "selinux",
		class:        coreFactInlineEager,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return selinuxCoreFacts(b.s) },
		emittedRoots: []string{"os"},
		emitsUnder:   "os.selinux",
	},
	{
		root:         "fips_enabled",
		class:        coreFactStandalone,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return fipsCoreFacts(b.s) },
		emittedRoots: []string{"fips_enabled"},
	},
	{
		root:         "timezone",
		class:        coreFactStandalone,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return timezoneCoreFacts(b.s) },
		emittedRoots: []string{"timezone"},
	},
	{
		root:         "augeas",
		class:        coreFactStandalone,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return augeasCoreFacts(b.s) },
		emittedRoots: []string{"augeas"},
	},
	{
		root:         "xen",
		class:        coreFactStandalone,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return xenCoreFacts(b.s) },
		emittedRoots: []string{"xen"},
	},
	{
		root:         "packages",
		group:        "packages",
		groupOrder:   4,
		class:        coreFactStandalone,
		assemble:     func(b *coreFactBuild) []ResolvedFact { return packagesCoreFacts(b.s) },
		emittedRoots: []string{"packages"},
	},
	{
		root:           "hypervisors",
		class:          coreFactSharedProbe,
		assemble:       func(b *coreFactBuild) []ResolvedFact { return currentLinuxHypervisorFacts(b.s) },
		emittedRoots:   []string{"hypervisors"},
		probeConsumers: []string{"virtual"},
	},
	{
		root:           "hypervisors",
		class:          coreFactSharedProbe,
		assemble:       func(b *coreFactBuild) []ResolvedFact { return currentWindowsHypervisorFacts(b.s) },
		emittedRoots:   []string{"hypervisors"},
		probeConsumers: []string{"virtual"},
	},
	{
		root:  "az_metadata",
		class: coreFactSharedProbe,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return azureFacts(b.s.Context(), newAzureClient(azureMetadataBaseURL, nil), b.virtualization)
		},
		emittedRoots:   []string{"az_metadata", "cloud"},
		probeConsumers: []string{"virtual"},
	},
	{
		root:  "ec2_metadata",
		class: coreFactSharedProbe,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return ec2Facts(b.s, newEC2Client(ec2MetadataBaseURL, nil), b.virtualization)
		},
		emittedRoots:   []string{"cloud", "ec2_metadata", "ec2_userdata"},
		probeConsumers: []string{"virtual"},
	},
	{
		root:  "gce",
		class: coreFactSharedProbe,
		assemble: func(b *coreFactBuild) []ResolvedFact {
			return platformGCEFacts(b.s.Context(), b.goos, b.virtualization, dmiBIOSVendor(b.dmi), newGCEClient(gceMetadataBaseURL, nil))
		},
		emittedRoots:   []string{"cloud", "gce"},
		probeConsumers: []string{"dmi", "virtual"},
	},
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
		dmi:            s.cachedDMI(),
	}
}

func standaloneCoreFactRoots() []string {
	roots := make([]string, 0)
	for _, descriptor := range coreFactDescriptors {
		if descriptor.class == coreFactStandalone {
			roots = append(roots, descriptor.root)
		}
	}
	return roots
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
