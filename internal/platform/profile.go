package platform

// SupportTier classifies the policy status for a platform target.
type SupportTier string

const (
	SupportTierRelease      SupportTier = "release"
	SupportTierLabValidated SupportTier = "lab_validated"
)

// Target is one GOOS/GOARCH build or distribution tuple.
type Target struct {
	GOOS   string
	GOARCH string
}

// IdentityPolicy captures the default OS and kernel identity for a target.
type IdentityPolicy struct {
	OSFamily   string
	OSName     string
	KernelName string
}

// CapabilityPolicy captures coarse target-level fact applicability.
type CapabilityPolicy struct {
	Filesystems bool
	ZFS         bool
	OSRelease   bool
}

// GateMetadata names the native gate associated with a target without storing
// lab-specific hosts, credentials, or other local environment details.
type GateMetadata struct {
	GOOS        string
	Name        string
	CIJob       string
	LocalTarget string
	Script      string
	LabGuest    string
}

// Profile is the internal policy profile for one GOOS target.
type Profile struct {
	ID                  string
	Label               string
	SupportTier         SupportTier
	SchemaVisible       bool
	CompileTargets      []Target
	DistributionTargets []Target
	Gate                GateMetadata
	Identity            IdentityPolicy
	Capabilities        CapabilityPolicy
}

var profileOrder = []string{
	"linux",
	"darwin",
	"windows",
	"freebsd",
	"openbsd",
	"netbsd",
	"dragonfly",
	"illumos",
	"plan9",
}

var profiles = map[string]Profile{
	"linux": {
		ID:            "linux",
		Label:         "Linux",
		SupportTier:   SupportTierRelease,
		SchemaVisible: true,
		CompileTargets: []Target{
			{GOOS: "linux", GOARCH: "amd64"},
			{GOOS: "linux", GOARCH: "arm64"},
		},
		DistributionTargets: []Target{
			{GOOS: "linux", GOARCH: "amd64"},
			{GOOS: "linux", GOARCH: "arm64"},
		},
		Gate: GateMetadata{
			GOOS:  "linux",
			Name:  "Linux GitHub-hosted runners and container workloads",
			CIJob: "go_tests, linux_container_tests, linux_distro_fact_tests",
		},
		Identity: IdentityPolicy{
			OSFamily:   "Linux",
			OSName:     "Linux",
			KernelName: "Linux",
		},
		Capabilities: CapabilityPolicy{
			Filesystems: true,
			OSRelease:   true,
		},
	},
	"darwin": {
		ID:            "darwin",
		Label:         "macOS / Darwin",
		SupportTier:   SupportTierRelease,
		SchemaVisible: true,
		CompileTargets: []Target{
			{GOOS: "darwin", GOARCH: "amd64"},
			{GOOS: "darwin", GOARCH: "arm64"},
		},
		DistributionTargets: []Target{
			{GOOS: "darwin", GOARCH: "amd64"},
			{GOOS: "darwin", GOARCH: "arm64"},
		},
		Gate: GateMetadata{
			GOOS:  "darwin",
			Name:  "macOS GitHub-hosted runners",
			CIJob: "go_tests",
		},
		Identity: IdentityPolicy{
			OSFamily:   "Darwin",
			OSName:     "Darwin",
			KernelName: "Darwin",
		},
		Capabilities: CapabilityPolicy{
			Filesystems: true,
			OSRelease:   true,
		},
	},
	"windows": {
		ID:            "windows",
		Label:         "Windows",
		SupportTier:   SupportTierRelease,
		SchemaVisible: true,
		CompileTargets: []Target{
			{GOOS: "windows", GOARCH: "amd64"},
			{GOOS: "windows", GOARCH: "arm64"},
		},
		DistributionTargets: []Target{
			{GOOS: "windows", GOARCH: "amd64"},
			{GOOS: "windows", GOARCH: "arm64"},
		},
		Gate: GateMetadata{
			GOOS:        "windows",
			Name:        "Windows release gate",
			CIJob:       "go_tests",
			Script:      "tools/windows-release-gate.ps1",
			LocalTarget: "go test ./... on Windows runner",
		},
		Identity: IdentityPolicy{
			OSFamily:   "windows",
			OSName:     "windows",
			KernelName: "windows",
		},
		Capabilities: CapabilityPolicy{
			OSRelease: true,
		},
	},
	"freebsd": {
		ID:            "freebsd",
		Label:         "FreeBSD",
		SupportTier:   SupportTierRelease,
		SchemaVisible: true,
		CompileTargets: []Target{
			{GOOS: "freebsd", GOARCH: "amd64"},
			{GOOS: "freebsd", GOARCH: "arm"},
			{GOOS: "freebsd", GOARCH: "arm64"},
		},
		DistributionTargets: []Target{
			{GOOS: "freebsd", GOARCH: "amd64"},
			{GOOS: "freebsd", GOARCH: "arm"},
			{GOOS: "freebsd", GOARCH: "arm64"},
		},
		Gate: GateMetadata{
			GOOS:        "freebsd",
			Name:        "FreeBSD release gate",
			CIJob:       "freebsd_tests",
			Script:      "tools/freebsd-release-gate.sh",
			LocalTarget: "lima-freebsd-smoke, local-freebsd-amd64-smoke",
			LabGuest:    "freebsd",
		},
		Identity: IdentityPolicy{
			OSFamily:   "FreeBSD",
			OSName:     "FreeBSD",
			KernelName: "FreeBSD",
		},
		Capabilities: CapabilityPolicy{
			ZFS:       true,
			OSRelease: true,
		},
	},
	"openbsd": {
		ID:            "openbsd",
		Label:         "OpenBSD",
		SupportTier:   SupportTierRelease,
		SchemaVisible: true,
		CompileTargets: []Target{
			{GOOS: "openbsd", GOARCH: "amd64"},
			{GOOS: "openbsd", GOARCH: "arm"},
			{GOOS: "openbsd", GOARCH: "arm64"},
		},
		DistributionTargets: []Target{
			{GOOS: "openbsd", GOARCH: "amd64"},
			{GOOS: "openbsd", GOARCH: "arm"},
			{GOOS: "openbsd", GOARCH: "arm64"},
		},
		Gate: GateMetadata{
			GOOS:        "openbsd",
			Name:        "OpenBSD release gate",
			CIJob:       "openbsd_tests",
			Script:      "tools/openbsd-release-gate.sh",
			LocalTarget: "local-openbsd-smoke, local-openbsd-amd64-smoke",
			LabGuest:    "openbsd",
		},
		Identity: IdentityPolicy{
			OSFamily:   "OpenBSD",
			OSName:     "OpenBSD",
			KernelName: "OpenBSD",
		},
		Capabilities: CapabilityPolicy{
			OSRelease: true,
		},
	},
	"netbsd": {
		ID:            "netbsd",
		Label:         "NetBSD",
		SupportTier:   SupportTierRelease,
		SchemaVisible: true,
		CompileTargets: []Target{
			{GOOS: "netbsd", GOARCH: "amd64"},
			{GOOS: "netbsd", GOARCH: "arm"},
			{GOOS: "netbsd", GOARCH: "arm64"},
		},
		DistributionTargets: []Target{
			{GOOS: "netbsd", GOARCH: "amd64"},
			{GOOS: "netbsd", GOARCH: "arm"},
			{GOOS: "netbsd", GOARCH: "arm64"},
		},
		Gate: GateMetadata{
			GOOS:        "netbsd",
			Name:        "NetBSD release gate",
			CIJob:       "netbsd_tests",
			Script:      "tools/netbsd-release-gate.sh",
			LocalTarget: "local-netbsd-smoke, local-netbsd-amd64-smoke",
			LabGuest:    "netbsd",
		},
		Identity: IdentityPolicy{
			OSFamily:   "NetBSD",
			OSName:     "NetBSD",
			KernelName: "NetBSD",
		},
		Capabilities: CapabilityPolicy{
			ZFS:       true,
			OSRelease: true,
		},
	},
	"dragonfly": {
		ID:            "dragonfly",
		Label:         "DragonFly BSD",
		SupportTier:   SupportTierRelease,
		SchemaVisible: true,
		CompileTargets: []Target{
			{GOOS: "dragonfly", GOARCH: "amd64"},
		},
		DistributionTargets: []Target{
			{GOOS: "dragonfly", GOARCH: "amd64"},
		},
		Gate: GateMetadata{
			GOOS:        "dragonfly",
			Name:        "DragonFly BSD release gate",
			CIJob:       "dragonfly_tests",
			Script:      "tools/dragonfly-release-gate.sh",
			LocalTarget: "local-dragonfly-amd64-smoke",
			LabGuest:    "dragonfly",
		},
		Identity: IdentityPolicy{
			OSFamily:   "DragonFly",
			OSName:     "DragonFly",
			KernelName: "DragonFly",
		},
		Capabilities: CapabilityPolicy{
			OSRelease: true,
		},
	},
	"illumos": {
		ID:            "illumos",
		Label:         "illumos",
		SupportTier:   SupportTierRelease,
		SchemaVisible: true,
		CompileTargets: []Target{
			{GOOS: "illumos", GOARCH: "amd64"},
		},
		DistributionTargets: []Target{
			{GOOS: "illumos", GOARCH: "amd64"},
		},
		Gate: GateMetadata{
			GOOS:        "illumos",
			Name:        "illumos release gate through OmniOS",
			CIJob:       "omnios_tests",
			Script:      "tools/illumos-release-gate.sh",
			LocalTarget: "local-illumos-amd64-smoke",
			LabGuest:    "omnios",
		},
		Identity: IdentityPolicy{
			OSFamily:   "illumos",
			OSName:     "illumos",
			KernelName: "SunOS",
		},
		Capabilities: CapabilityPolicy{
			ZFS:       true,
			OSRelease: true,
		},
	},
	"plan9": {
		ID:            "plan9",
		Label:         "Plan 9",
		SupportTier:   SupportTierLabValidated,
		SchemaVisible: true,
		CompileTargets: []Target{
			{GOOS: "plan9", GOARCH: "amd64"},
		},
		Gate: GateMetadata{
			GOOS:     "plan9",
			Name:     "Plan 9 lab release gate",
			Script:   "tools/plan9-release-gate.rc",
			LabGuest: "plan9",
		},
		Identity: IdentityPolicy{
			OSFamily:   "Plan 9",
			OSName:     "Plan 9",
			KernelName: "Plan 9",
		},
	},
}

// Lookup returns the target profile for goos.
func Lookup(goos string) (Profile, bool) {
	profile, ok := profiles[goos]
	if !ok {
		return Profile{}, false
	}
	return copyProfile(profile), true
}

// Profiles returns all target profiles in stable policy order.
func Profiles() []Profile {
	out := make([]Profile, 0, len(profileOrder))
	for _, id := range profileOrder {
		profile, ok := profiles[id]
		if !ok {
			panic("internal/platform: profileOrder references unknown profile " + id)
		}
		out = append(out, copyProfile(profile))
	}
	return out
}

// SchemaVisibleProfiles returns profiles accepted by the facts schema.
func SchemaVisibleProfiles() []Profile {
	profiles := Profiles()
	out := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.SchemaVisible {
			out = append(out, profile)
		}
	}
	return out
}

// CompileTargets returns the cross-compile target set in stable order.
func CompileTargets() []Target {
	var out []Target
	for _, profile := range Profiles() {
		out = append(out, profile.CompileTargets...)
	}
	return out
}

// DistributionTargets returns the published artifact target set in stable order.
func DistributionTargets() []Target {
	var out []Target
	for _, profile := range Profiles() {
		out = append(out, profile.DistributionTargets...)
	}
	return out
}

// NativeGates returns target gate metadata in stable target order.
func NativeGates() []GateMetadata {
	var out []GateMetadata
	for _, profile := range Profiles() {
		if profile.Gate.Name != "" {
			out = append(out, profile.Gate)
		}
	}
	return out
}

func copyProfile(profile Profile) Profile {
	profile.CompileTargets = append([]Target(nil), profile.CompileTargets...)
	profile.DistributionTargets = append([]Target(nil), profile.DistributionTargets...)
	return profile
}
