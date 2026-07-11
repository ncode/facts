package engine

import (
	"fmt"
	"strings"
)

func discoverFamily(id string) string {
	value := strings.ToLower(id)
	families := []struct {
		family string
		ids    []string
	}{
		// Ruby's OS hierarchy nests Ol, Amzn, and Meego under Rhel; rocky and
		// almalinux reach RedHat through os-release ID_LIKE in Ruby Facter.
		{family: "RedHat", ids: []string{"redhat", "rhel", "fedora", "centos", "scientific", "ascendos", "cloudlinux", "psbm", "oraclelinux", "ol", "ovs", "oel", "amazon", "amzn", "meego", "rocky", "almalinux", "xenserver", "xcp-ng", "virtuozzo", "photon", "mariner", "azurelinux"}},
		{family: "Suse", ids: []string{"sled", "sles", "opensuse", "suse"}},
		{family: "Debian", ids: []string{"debian", "ubuntu", "kde", "huaweios", "linuxmint", "devuan"}},
		{family: "Gentoo", ids: []string{"gentoo"}},
		{family: "Archlinux", ids: []string{"arch", "archlinux", "manjaro"}},
		{family: "Mandrake", ids: []string{"mandrake", "mandriva", "mageia"}},
	}

	for _, candidate := range families {
		for _, osID := range candidate.ids {
			if containsOSID(value, osID) {
				return candidate.family
			}
		}
	}
	return id
}

// usesRedHatReleaseDistro reports whether a distro's os.distro fields come
// from /etc/redhat-release. Oracle Linux and Amazon Linux are RedHat-family
// for os.family but their Ruby fact sets (facts/ol, facts/amzn) source
// os.distro from os-release and system-release, and Oracle's redhat-release
// carries Red Hat branding that must not leak into os.distro.
func usesRedHatReleaseDistro(id string) bool {
	switch strings.ToLower(id) {
	case "ol", "oel", "oraclelinux", "amzn", "amazon":
		return false
	}
	return discoverFamily(id) == "RedHat"
}

func containsOSID(value, id string) bool {
	// Short identifiers like "ol" must match exactly; substring matching
	// would misclassify unrelated IDs that merely contain the letters.
	if len(id) <= 3 {
		return value == id
	}
	return strings.Contains(value, id)
}

func releaseHashFromString(version string, includePatch bool) map[string]any {
	if version == "" {
		return nil
	}

	parts := strings.Split(version, ".")
	release := map[string]any{
		"full":  version,
		"major": parts[0],
	}
	if len(parts) > 1 && parts[1] != "" {
		release["minor"] = parts[1]
	}
	if includePatch && len(parts) > 2 && parts[2] != "" {
		release["patch"] = parts[2]
	}
	return release
}

func deepStringifyKeys(value any) any {
	switch v := value.(type) {
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			out[fmt.Sprint(key)] = deepStringifyKeys(child)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			out[key] = deepStringifyKeys(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = deepStringifyKeys(child)
		}
		return out
	default:
		return value
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
