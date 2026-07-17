package engine

import "slices"

// DiscoveryDefaults captures ambient paths and ordered external-fact
// directories for one discovery or CLI invocation.
type DiscoveryDefaults struct {
	NativeConfigPath     string
	CompatibleConfigPath string
	CachePath            string
	ExternalFactDirs     []string
}

// CurrentDiscoveryDefaults returns the defaults for the current process.
func CurrentDiscoveryDefaults() DiscoveryDefaults {
	return DiscoveryDefaults{
		NativeConfigPath:     platformNativeDefaultConfigPath(),
		CompatibleConfigPath: platformDefaultConfigPath(),
		CachePath:            platformDefaultCachePath(),
		ExternalFactDirs:     CurrentDefaultExternalFactDirs(),
	}
}

func (d DiscoveryDefaults) clone() DiscoveryDefaults {
	d.ExternalFactDirs = slices.Clone(d.ExternalFactDirs)
	return d
}

func cloneDiscoveryDefaults(defaults *DiscoveryDefaults) *DiscoveryDefaults {
	if defaults == nil {
		return nil
	}
	cloned := defaults.clone()
	return &cloned
}
