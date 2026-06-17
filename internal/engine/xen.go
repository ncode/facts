package engine

import (
	"runtime"
	"strings"
)

func currentXenFacts(s *Session) []ResolvedFact {
	vm := detectXenVM(s)
	var domains []string
	if vm == "xen0" {
		domains = detectXenDomains(s)
	}
	return xenFacts(vm, domains)
}

func xenFacts(vm string, domains []string) []ResolvedFact {
	if vm != "xen0" {
		return []ResolvedFact{{Name: "xen", Value: nil}}
	}
	if domains == nil {
		domains = []string{}
	}
	return []ResolvedFact{
		{Name: "xen", Value: map[string]any{"domains": domains}},
	}
}

func detectXenVM(s *Session) string {
	if runtime.GOOS != "linux" {
		return ""
	}
	if strings.Contains(readFileString("/proc/xen/capabilities", s.readFile), "control_d") {
		return "xen0"
	}
	return detectXenVMFromSignals(fileExistsWithHost(s.host, "/dev/xen/evtchn"), dirExistsWithHost(s.host, "/proc/xen"), fileExistsWithHost(s.host, "/dev/xvda1"), isSymlink("/dev/xvda1", s.lstat))
}

func detectXenVMFromSignals(evtchn, procXen, xvda1, xvda1Symlink bool) string {
	if evtchn {
		return "xen0"
	}
	if procXen || (xvda1 && !xvda1Symlink) {
		return "xenu"
	}
	return ""
}

func detectXenDomains(s *Session) []string {
	bin := selectXenCommand(fileExists)
	if bin == "" {
		return nil
	}
	out := s.commandOutput(bin, "list")
	if out == "" {
		return nil
	}
	return parseXenDomains(out)
}

func selectXenCommand(exists func(string) bool) string {
	const toolstack = "/usr/lib/xen-common/bin/xen-toolstack"
	commands := []string{"/usr/sbin/xl", "/usr/sbin/xm"}

	stacks := 0
	for _, command := range commands {
		if exists(command) {
			stacks++
		}
	}
	if stacks > 1 && exists(toolstack) {
		return toolstack
	}
	for _, command := range commands {
		if exists(command) {
			return command
		}
	}
	return ""
}

func parseXenDomains(out string) []string {
	domains := make([]string, 0)
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "Name" || fields[0] == "Domain-0" {
			continue
		}
		domains = append(domains, fields[0])
	}
	return domains
}

// xenCoreFacts assembles the xen category facts (the privileged-domain list),
// emitted only on Xen dom0 hosts.
func xenCoreFacts(s *Session) []ResolvedFact {
	return currentXenFacts(s)
}
