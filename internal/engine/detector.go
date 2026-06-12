package engine

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnknownOS reports that a Ruby host_os value does not map to a Facter OS identifier.
var ErrUnknownOS = errors.New("unknown os")

// DetectOSIdentifier maps Ruby's RbConfig host_os value to Facter's OS identifier.
func DetectOSIdentifier(hostOS, linuxDistroID string) (string, error) {
	hostOS = strings.ToLower(strings.TrimSpace(hostOS))
	switch {
	case strings.Contains(hostOS, "darwin"):
		return "macosx", nil
	case strings.Contains(hostOS, "mingw") || strings.Contains(hostOS, "mswin") || strings.Contains(hostOS, "windows"):
		return "windows", nil
	case strings.Contains(hostOS, "linux"):
		linuxDistroID = strings.TrimSpace(linuxDistroID)
		if linuxDistroID != "" {
			return linuxDistroID, nil
		}
		return "linux", nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownOS, hostOS)
	}
}

// ConstructOSHierarchy returns the Ruby-compatible OS inheritance path for searchedOS.
func ConstructOSHierarchy(hierarchy []any, searchedOS string) []string {
	if searchedOS == "" {
		return []string{}
	}
	searched := capitalizeOSName(searchedOS)
	if hierarchy == nil {
		return []string{searched}
	}
	if path, ok := searchOSHierarchy(hierarchy, searched, nil); ok {
		return path
	}
	return []string{}
}

// DetectOSHierarchy returns Ruby's detected OS hierarchy for an identifier.
func DetectOSHierarchy(hierarchy []any, identifier, family string) []string {
	resolved := ConstructOSHierarchy(hierarchy, identifier)
	if len(resolved) > 0 {
		return resolved
	}

	debug("Could not detect hierarchy using os identifier: " + identifier + " , trying with family")
	for candidate := range strings.FieldsSeq(family) {
		resolved = ConstructOSHierarchy(hierarchy, candidate)
		if len(resolved) > 0 {
			return resolved
		}
	}

	debug("Could not detect hierarchy using family " + family + ", falling back to Linux")
	return ConstructOSHierarchy(hierarchy, "linux")
}

func searchOSHierarchy(nodes []any, searched string, path []string) ([]string, bool) {
	for _, node := range nodes {
		switch n := node.(type) {
		case string:
			if n == searched {
				return append(append([]string(nil), path...), n), true
			}
		case map[string]any:
			for key, value := range n {
				nextPath := append(append([]string(nil), path...), key)
				if key == searched {
					return nextPath, true
				}
				children, ok := value.([]any)
				if !ok {
					continue
				}
				if found, ok := searchOSHierarchy(children, searched, nextPath); ok {
					return found, true
				}
			}
		}
	}
	return nil, false
}

func capitalizeOSName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
}
