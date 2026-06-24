package engine

import (
	"path/filepath"
	"runtime"
	"strings"
)

// selinuxFactsForPlatform resolves os.selinux only on Linux, the only
// platform where Ruby Facter emits SELinux data; elsewhere the fact is
// absent.
func selinuxFactsForPlatform(goos, mountsPath, configPath string, readFile fileReader) []ResolvedFact {
	if goos != "linux" {
		return nil
	}
	return selinuxFacts(mountsPath, configPath, readFile)
}

func selinuxFacts(mountsPath, configPath string, readFile fileReader) []ResolvedFact {
	mountpoint := selinuxMountpoint(mountsPath, readFile)
	configMode, configPolicy, hasConfig := readSELinuxConfig(configPath, readFile)
	enabled := mountpoint != ""
	values := map[string]any{"enabled": enabled}
	if enabled {
		if hasConfig {
			if configMode != "" {
				values["config_mode"] = configMode
			}
			if configPolicy != "" {
				values["config_policy"] = configPolicy
			}
		}
		values["policy_version"] = readOptionalText(filepath.Join(mountpoint, "policyvers"), readFile)
		if enforced, ok := readSELinuxEnforce(filepath.Join(mountpoint, "enforce"), readFile); ok {
			values["enforced"] = enforced
			if enforced {
				values["current_mode"] = "enforcing"
			} else {
				values["current_mode"] = "permissive"
			}
		}
	}

	coreNames := map[string]string{
		"config_mode":    "os.selinux.config_mode",
		"config_policy":  "os.selinux.config_policy",
		"current_mode":   "os.selinux.current_mode",
		"enabled":        "os.selinux.enabled",
		"enforced":       "os.selinux.enforced",
		"policy_version": "os.selinux.policy_version",
	}
	keys := []string{"config_mode", "config_policy", "current_mode", "enabled", "enforced", "policy_version"}
	core := make([]ResolvedFact, 0, len(values))
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		core = append(core, ResolvedFact{Name: coreNames[key], Value: value})
	}
	return core
}

func selinuxMountpoint(path string, readFile fileReader) string {
	for line := range strings.SplitSeq(readText(path, readFile), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "selinuxfs" {
			return fields[1]
		}
	}
	return ""
}

func readSELinuxConfig(path string, readFile fileReader) (mode, policy string, ok bool) {
	data, err := readFile(path)
	if err != nil || len(data) == 0 {
		return "", "", false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if value, found := strings.CutPrefix(line, "SELINUX="); found {
			mode = strings.TrimSpace(value)
		}
		if value, found := strings.CutPrefix(line, "SELINUXTYPE="); found {
			policy = strings.TrimSpace(value)
		}
	}
	return mode, policy, true
}

func readSELinuxEnforce(path string, readFile fileReader) (bool, bool) {
	data, err := readFile(path)
	if err != nil {
		return false, false
	}
	switch strings.TrimSpace(string(data)) {
	case "1":
		return true, true
	case "0":
		return false, true
	default:
		return false, false
	}
}

// selinuxCoreFacts assembles the selinux category facts (os.selinux), emitted
// only on Linux.
func selinuxCoreFacts(s *Session) []ResolvedFact {
	return selinuxFactsForPlatform(runtime.GOOS, "/proc/self/mounts", "/etc/selinux/config", s.readFile)
}
