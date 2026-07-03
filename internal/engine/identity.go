package engine

import (
	"os"
	osuser "os/user"
	"strconv"
	"strings"
)

type identityInfo struct {
	User       string
	UID        string
	GID        string
	Group      string
	Privileged *bool
}

func identityFact(s *Session) map[string]any {
	goos := s.goos()
	if goos == "windows" {
		return identityFactFromInfo(goos, currentWindowsIdentityInfo(s))
	}

	// The uid/gid/osuser syscalls stay outside the host seam: they have no
	// meaningful fake and describe the resolving process, not the probed host.
	privileged := os.Geteuid() == 0
	info := identityInfo{
		UID:        strconv.Itoa(os.Getuid()),
		GID:        strconv.Itoa(os.Getgid()),
		Privileged: &privileged,
	}
	current, err := osuser.Current()
	if err != nil {
		return identityFactFromInfo(goos, info)
	}
	info.UID = current.Uid
	info.GID = current.Gid
	info.User = current.Username
	if group, err := osuser.LookupGroupId(current.Gid); err == nil {
		info.Group = group.Name
	}
	return identityFactFromInfo(goos, info)
}

func currentWindowsIdentityInfo(s *Session) identityInfo {
	info := identityInfo{}
	info.User = strings.TrimSpace(s.commandOutput("whoami"))
	if info.User == "" {
		s.logr().Debug("failure resolving identity facts: ")
		return info
	}
	if privileged, ok := parseWindowsAdministratorGroups(s.commandOutput("whoami", "/groups")); ok {
		info.Privileged = &privileged
	}
	return info
}

func parseWindowsAdministratorGroups(output string) (bool, bool) {
	if strings.TrimSpace(output) == "" {
		return false, false
	}
	for line := range strings.SplitSeq(output, "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, `\administrators`) && !strings.Contains(lower, "s-1-5-32-544") {
			continue
		}
		return strings.Contains(lower, "enabled") && !strings.Contains(lower, "deny only"), true
	}
	return false, true
}

func identityFactFromInfo(goos string, info identityInfo) map[string]any {
	identity := make(map[string]any, 5)
	if info.Privileged != nil {
		identity["privileged"] = *info.Privileged
	}
	if info.User != "" {
		identity["user"] = info.User
	}
	if goos == "windows" {
		return identity
	}
	if info.UID != "" {
		identity["uid"] = numericIdentityValue(info.UID)
	}
	if info.GID != "" {
		identity["gid"] = numericIdentityValue(info.GID)
	}
	if info.Group != "" {
		identity["group"] = info.Group
	}
	return identity
}

func numericIdentityValue(value string) any {
	n, err := strconv.Atoi(value)
	if err != nil {
		return value
	}
	return n
}

// identityCoreFacts assembles the identity category fact (the current user,
// group, and privilege state) for the current host.
func identityCoreFacts(s *Session) []ResolvedFact {
	if s.goos() == "plan9" {
		return nil
	}
	return []ResolvedFact{
		{Name: "identity", Value: s.cachedIdentity()},
	}
}
