package engine

import (
	"strconv"
	"strings"
)

func fipsEnabled(path string, readFile fileReader) bool {
	data, err := readFile(path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

func currentFIPSEnabled(s *Session, linuxPath string) bool {
	if s.goos() == "windows" {
		return parseWindowsFIPSEnabled(s.commandOutput("reg", "query", `HKLM\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`, "/v", "Enabled"))
	}
	return fipsEnabled(linuxPath, s.readFile)
}

// fipsEnabledFacts resolves fips_enabled only on Linux and Windows, the
// platforms where Ruby Facter emits the fact; elsewhere the fact is absent
// instead of a placeholder false.
func fipsEnabledFacts(s *Session, linuxPath string) []ResolvedFact {
	goos := s.goos()
	if goos != "linux" && goos != "windows" {
		return nil
	}
	return []ResolvedFact{{Name: "fips_enabled", Value: currentFIPSEnabled(s, linuxPath)}}
}

func parseWindowsFIPSEnabled(input string) bool {
	for line := range strings.SplitSeq(input, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 || fields[0] != "Enabled" || fields[1] != "REG_DWORD" {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimPrefix(fields[2], "0x"), 16, 64)
		return err == nil && value != 0
	}
	return false
}

// fipsCoreFacts assembles the fips category fact (fips_enabled), emitted only on
// Linux and Windows.
func fipsCoreFacts(s *Session) []ResolvedFact {
	return fipsEnabledFacts(s, "/proc/sys/crypto/fips_enabled")
}
